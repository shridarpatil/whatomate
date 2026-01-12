package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWorker(t *testing.T) *Worker {
	t.Helper()
	db := testutil.SetupTestDB(t)
	log := testutil.NopLogger()

	w := &Worker{
		DB:       db,
		Log:      log,
		WhatsApp: whatsapp.New(log),
	}

	// Set up Publisher if Redis is available
	if rdb := testutil.SetupTestRedis(t); rdb != nil {
		w.Redis = rdb
		w.Publisher = queue.NewPublisher(rdb, log)
	}

	return w
}

func createTestCampaignData(t *testing.T, w *Worker) (*models.Organization, *models.WhatsAppAccount, *models.Template, *models.BulkMessageCampaign, *models.BulkMessageRecipient) {
	t.Helper()

	uniqueID := uuid.New().String()[:8]

	// Create organization
	org := &models.Organization{
		Name: "Test Org " + uniqueID,
		Slug: "test-org-" + uniqueID,
	}
	require.NoError(t, w.DB.Create(org).Error)

	// Create user for CreatedBy foreign key
	user := &models.User{
		OrganizationID: org.ID,
		Email:          "test-" + uniqueID + "@example.com",
		PasswordHash:   "hashed",
		FullName:       "Test User",
		Role:           models.RoleAdmin,
		IsActive:       true,
	}
	require.NoError(t, w.DB.Create(user).Error)

	// Create WhatsApp account with unique name
	accountName := "test-account-" + uniqueID
	account := &models.WhatsAppAccount{
		OrganizationID: org.ID,
		Name:           accountName,
		PhoneID:        "phone-" + uniqueID,
		BusinessID:     "business-" + uniqueID,
		AccessToken:    "test-token",
	}
	require.NoError(t, w.DB.Create(account).Error)

	// Create template
	template := &models.Template{
		OrganizationID:  org.ID,
		WhatsAppAccount: accountName,
		Name:            "test_template_" + uniqueID,
		Language:        "en",
		Category:        "MARKETING",
		Status:          "APPROVED",
		BodyContent:     "Hello {{1}}, your order {{2}} is ready!",
	}
	require.NoError(t, w.DB.Create(template).Error)

	// Create campaign with CreatedBy
	campaign := &models.BulkMessageCampaign{
		OrganizationID:  org.ID,
		Name:            "Test Campaign " + uniqueID,
		WhatsAppAccount: accountName,
		TemplateID:      template.ID,
		Status:          models.CampaignStatusProcessing,
		TotalRecipients: 1,
		CreatedBy:       user.ID,
	}
	require.NoError(t, w.DB.Create(campaign).Error)

	// Create recipient
	recipient := &models.BulkMessageRecipient{
		CampaignID:    campaign.ID,
		PhoneNumber:   "1112223333",
		RecipientName: "Test User",
		Status:        models.MessageStatusPending,
		TemplateParams: models.JSONB{
			"1": "John",
			"2": "ORD-123",
		},
	}
	require.NoError(t, w.DB.Create(recipient).Error)

	// Reload campaign with template
	require.NoError(t, w.DB.Preload("Template").First(campaign, campaign.ID).Error)

	return org, account, template, campaign, recipient
}

func TestWorker_HandleRecipientJob_CampaignPaused(t *testing.T) {
	w := testWorker(t)
	org, _, _, campaign, recipient := createTestCampaignData(t, w)

	// Pause the campaign
	require.NoError(t, w.DB.Model(campaign).Update("status", models.CampaignStatusPaused).Error)

	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
		RecipientName:  recipient.RecipientName,
	}

	err := w.HandleRecipientJob(context.Background(), job)
	require.NoError(t, err)

	// Recipient status should remain pending (job was skipped)
	var updatedRecipient models.BulkMessageRecipient
	require.NoError(t, w.DB.First(&updatedRecipient, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusPending, updatedRecipient.Status)
}

func TestWorker_HandleRecipientJob_CampaignCancelled(t *testing.T) {
	w := testWorker(t)
	org, _, _, campaign, recipient := createTestCampaignData(t, w)

	// Cancel the campaign
	require.NoError(t, w.DB.Model(campaign).Update("status", models.CampaignStatusCancelled).Error)

	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
		RecipientName:  recipient.RecipientName,
	}

	err := w.HandleRecipientJob(context.Background(), job)
	require.NoError(t, err)

	// Recipient status should remain pending (job was skipped)
	var updatedRecipient models.BulkMessageRecipient
	require.NoError(t, w.DB.First(&updatedRecipient, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusPending, updatedRecipient.Status)
}

func TestWorker_HandleRecipientJob_AccountNotFound(t *testing.T) {
	w := testWorker(t)
	org, _, _, campaign, recipient := createTestCampaignData(t, w)

	// Change campaign to use non-existent account
	campaign.WhatsAppAccount = "non-existent-account"
	require.NoError(t, w.DB.Save(campaign).Error)

	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
		RecipientName:  recipient.RecipientName,
	}

	err := w.HandleRecipientJob(context.Background(), job)
	require.NoError(t, err)

	// Verify recipient marked as failed
	var updatedRecipient models.BulkMessageRecipient
	require.NoError(t, w.DB.First(&updatedRecipient, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, updatedRecipient.Status)
	assert.Contains(t, updatedRecipient.ErrorMessage, "WhatsApp account not found")
}

func TestWorker_HandleRecipientJob_CampaignNotFound(t *testing.T) {
	w := testWorker(t)

	job := &queue.RecipientJob{
		CampaignID:     uuid.New(), // Non-existent campaign
		RecipientID:    uuid.New(),
		OrganizationID: uuid.New(),
		PhoneNumber:    "1234567890",
		RecipientName:  "Test",
	}

	err := w.HandleRecipientJob(context.Background(), job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load campaign")
}

func TestWorker_getOrCreateContact_CreatesNewContact(t *testing.T) {
	w := testWorker(t)

	// Create organization
	org := &models.Organization{
		Name: "Test Org " + uuid.New().String()[:8],
		Slug: "test-org-" + uuid.New().String()[:8],
	}
	require.NoError(t, w.DB.Create(org).Error)

	contact, err := w.getOrCreateContact(org.ID, "1234567890", "Test User")
	require.NoError(t, err)
	require.NotNil(t, contact)

	assert.Equal(t, "1234567890", contact.PhoneNumber)
	assert.Equal(t, "Test User", contact.ProfileName)
	assert.Equal(t, org.ID, contact.OrganizationID)
}

func TestWorker_getOrCreateContact_NormalizesPhoneNumber(t *testing.T) {
	w := testWorker(t)

	// Create organization
	org := &models.Organization{
		Name: "Test Org " + uuid.New().String()[:8],
		Slug: "test-org-" + uuid.New().String()[:8],
	}
	require.NoError(t, w.DB.Create(org).Error)

	// Test with + prefix - should strip it
	contact1, err := w.getOrCreateContact(org.ID, "+1234567890", "Test User")
	require.NoError(t, err)
	assert.Equal(t, "1234567890", contact1.PhoneNumber)

	// Test finding existing contact with normalized number
	contact2, err := w.getOrCreateContact(org.ID, "+1234567890", "Different Name")
	require.NoError(t, err)
	assert.Equal(t, contact1.ID, contact2.ID) // Should return same contact
}

func TestWorker_getOrCreateContact_FindsExistingContact(t *testing.T) {
	w := testWorker(t)

	// Create organization
	org := &models.Organization{
		Name: "Test Org " + uuid.New().String()[:8],
		Slug: "test-org-" + uuid.New().String()[:8],
	}
	require.NoError(t, w.DB.Create(org).Error)

	// Pre-create a contact
	existingContact := &models.Contact{
		OrganizationID: org.ID,
		PhoneNumber:    "5556667777",
		ProfileName:    "Existing Contact",
	}
	require.NoError(t, w.DB.Create(existingContact).Error)

	// Should find existing contact
	contact, err := w.getOrCreateContact(org.ID, "5556667777", "Different Name")
	require.NoError(t, err)
	assert.Equal(t, existingContact.ID, contact.ID)
	assert.Equal(t, "Existing Contact", contact.ProfileName) // Name shouldn't change
}

func TestWorker_getOrCreateContact_FindsWithPlusPrefix(t *testing.T) {
	w := testWorker(t)

	// Create organization
	org := &models.Organization{
		Name: "Test Org " + uuid.New().String()[:8],
		Slug: "test-org-" + uuid.New().String()[:8],
	}
	require.NoError(t, w.DB.Create(org).Error)

	// Create contact with + prefix
	existingContact := &models.Contact{
		OrganizationID: org.ID,
		PhoneNumber:    "+9876543210",
		ProfileName:    "Plus Prefix Contact",
	}
	require.NoError(t, w.DB.Create(existingContact).Error)

	// Search without + prefix should find it
	contact, err := w.getOrCreateContact(org.ID, "9876543210", "Test")
	require.NoError(t, err)
	assert.Equal(t, existingContact.ID, contact.ID)
}

// createMinimalCampaignData creates the minimum data needed for campaign tests
// Returns org, user, template, and campaign
func createMinimalCampaignData(t *testing.T, w *Worker, status models.CampaignStatus) (*models.Organization, *models.User, *models.Template, *models.BulkMessageCampaign) {
	t.Helper()
	uniqueID := uuid.New().String()[:8]

	org := &models.Organization{
		Name: "Test Org " + uniqueID,
		Slug: "test-org-" + uniqueID,
	}
	require.NoError(t, w.DB.Create(org).Error)

	user := &models.User{
		OrganizationID: org.ID,
		Email:          "test-" + uniqueID + "@example.com",
		PasswordHash:   "hashed",
		FullName:       "Test User",
		Role:           models.RoleAdmin,
		IsActive:       true,
	}
	require.NoError(t, w.DB.Create(user).Error)

	accountName := "test-account-" + uniqueID
	account := &models.WhatsAppAccount{
		OrganizationID: org.ID,
		Name:           accountName,
		PhoneID:        "phone-" + uniqueID,
		BusinessID:     "business-" + uniqueID,
		AccessToken:    "test-token",
	}
	require.NoError(t, w.DB.Create(account).Error)

	template := &models.Template{
		OrganizationID:  org.ID,
		WhatsAppAccount: accountName,
		Name:            "test_template_" + uniqueID,
		Language:        "en",
		Category:        string(models.TemplateCategoryMarketing),
		Status:          string(models.TemplateStatusApproved),
		BodyContent:     "Hello {{1}}!",
	}
	require.NoError(t, w.DB.Create(template).Error)

	campaign := &models.BulkMessageCampaign{
		OrganizationID:  org.ID,
		Name:            "Test Campaign " + uniqueID,
		WhatsAppAccount: accountName,
		TemplateID:      template.ID,
		Status:          status,
		CreatedBy:       user.ID,
	}
	require.NoError(t, w.DB.Create(campaign).Error)

	return org, user, template, campaign
}

func TestWorker_updateRecipientStatus_Sent(t *testing.T) {
	w := testWorker(t)

	// Create campaign data with proper foreign keys
	_, _, _, campaign := createMinimalCampaignData(t, w, models.CampaignStatusProcessing)

	recipient := &models.BulkMessageRecipient{
		CampaignID:  campaign.ID,
		PhoneNumber: "1234567890",
		Status:      models.MessageStatusPending,
	}
	require.NoError(t, w.DB.Create(recipient).Error)

	// Test updating to sent status
	w.updateRecipientStatus(recipient.ID, models.MessageStatusSent, "wamid.123", "")

	var updated models.BulkMessageRecipient
	require.NoError(t, w.DB.First(&updated, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusSent, updated.Status)
	assert.Equal(t, "wamid.123", updated.WhatsAppMessageID)
	assert.NotNil(t, updated.SentAt)
}

func TestWorker_updateRecipientStatus_Failed(t *testing.T) {
	w := testWorker(t)

	// Create campaign data with proper foreign keys
	_, _, _, campaign := createMinimalCampaignData(t, w, models.CampaignStatusProcessing)

	recipient := &models.BulkMessageRecipient{
		CampaignID:  campaign.ID,
		PhoneNumber: "9876543210",
		Status:      models.MessageStatusPending,
	}
	require.NoError(t, w.DB.Create(recipient).Error)

	w.updateRecipientStatus(recipient.ID, models.MessageStatusFailed, "", "API error")

	var updated models.BulkMessageRecipient
	require.NoError(t, w.DB.First(&updated, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, updated.Status)
	assert.Equal(t, "API error", updated.ErrorMessage)
}

func TestWorker_incrementCampaignCount(t *testing.T) {
	w := testWorker(t)

	// Create campaign data with proper foreign keys
	_, _, _, campaign := createMinimalCampaignData(t, w, models.CampaignStatusProcessing)

	// Increment sent count multiple times
	w.incrementCampaignCount(campaign.ID, "sent_count")
	w.incrementCampaignCount(campaign.ID, "sent_count")
	w.incrementCampaignCount(campaign.ID, "failed_count")

	var updated models.BulkMessageCampaign
	require.NoError(t, w.DB.First(&updated, campaign.ID).Error)
	assert.Equal(t, 2, updated.SentCount)
	assert.Equal(t, 1, updated.FailedCount)
}

func TestWorker_checkCampaignCompletion_CompletesWhenAllProcessed(t *testing.T) {
	w := testWorker(t)

	// Create campaign data with proper foreign keys
	org, _, _, campaign := createMinimalCampaignData(t, w, models.CampaignStatusProcessing)

	// Update campaign counts for this test
	require.NoError(t, w.DB.Model(campaign).Updates(map[string]interface{}{
		"total_recipients": 2,
		"sent_count":       2,
	}).Error)

	// Create recipients that are already processed (not pending)
	recipient1 := &models.BulkMessageRecipient{
		CampaignID:  campaign.ID,
		PhoneNumber: "1111111111",
		Status:      models.MessageStatusSent,
	}
	recipient2 := &models.BulkMessageRecipient{
		CampaignID:  campaign.ID,
		PhoneNumber: "2222222222",
		Status:      models.MessageStatusSent,
	}
	require.NoError(t, w.DB.Create(recipient1).Error)
	require.NoError(t, w.DB.Create(recipient2).Error)

	// Check completion - should complete since no pending recipients
	w.checkCampaignCompletion(context.Background(), campaign.ID, org.ID)

	var updated models.BulkMessageCampaign
	require.NoError(t, w.DB.First(&updated, campaign.ID).Error)
	assert.Equal(t, models.CampaignStatusCompleted, updated.Status)
	assert.NotNil(t, updated.CompletedAt)
}

func TestWorker_checkCampaignCompletion_DoesNotCompleteWithPending(t *testing.T) {
	w := testWorker(t)

	// Create campaign data with proper foreign keys
	org, _, _, campaign := createMinimalCampaignData(t, w, models.CampaignStatusProcessing)

	// Update campaign counts for this test
	require.NoError(t, w.DB.Model(campaign).Updates(map[string]interface{}{
		"total_recipients": 2,
		"sent_count":       1,
	}).Error)

	// Create one processed and one pending recipient
	recipient1 := &models.BulkMessageRecipient{
		CampaignID:  campaign.ID,
		PhoneNumber: "1111111111",
		Status:      models.MessageStatusSent,
	}
	recipient2 := &models.BulkMessageRecipient{
		CampaignID:  campaign.ID,
		PhoneNumber: "2222222222",
		Status:      models.MessageStatusPending,
	}
	require.NoError(t, w.DB.Create(recipient1).Error)
	require.NoError(t, w.DB.Create(recipient2).Error)

	// Check completion - should NOT complete since there's a pending recipient
	w.checkCampaignCompletion(context.Background(), campaign.ID, org.ID)

	var updated models.BulkMessageCampaign
	require.NoError(t, w.DB.First(&updated, campaign.ID).Error)
	assert.Equal(t, models.CampaignStatusProcessing, updated.Status)
	assert.Nil(t, updated.CompletedAt)
}

func TestWorker_checkCampaignCompletion_NotProcessingStatus(t *testing.T) {
	w := testWorker(t)

	// Create campaign data with proper foreign keys - status is paused
	org, _, _, campaign := createMinimalCampaignData(t, w, models.CampaignStatusPaused)

	// Should not change status since it's not models.CampaignStatusProcessing
	w.checkCampaignCompletion(context.Background(), campaign.ID, org.ID)

	var updated models.BulkMessageCampaign
	require.NoError(t, w.DB.First(&updated, campaign.ID).Error)
	assert.Equal(t, models.CampaignStatusPaused, updated.Status)
}

func TestWorker_sendTemplateMessage_BuildsComponents(t *testing.T) {
	w := testWorker(t)

	// Create mock server
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"messages": []map[string]interface{}{
				{"id": "wamid.test123"},
			},
		})
	}))
	defer server.Close()

	// Create WhatsApp client pointing to mock server
	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	account := &models.WhatsAppAccount{
		PhoneID:     "123",
		BusinessID:  "456",
		AccessToken: "token",
		APIVersion:  "v21.0",
	}

	template := &models.Template{
		Name:        "test_template",
		Language:    "en",
		BodyContent: "Hello {{1}}, welcome to {{2}}!",
	}

	recipient := &models.BulkMessageRecipient{
		PhoneNumber: "1234567890",
		TemplateParams: models.JSONB{
			"1": "Hello",
			"2": "World",
		},
	}

	msgID, err := w.sendTemplateMessage(context.Background(), account, template, recipient, "")
	require.NoError(t, err)
	assert.Equal(t, "wamid.test123", msgID)

	// Verify request structure
	templateData := capturedBody["template"].(map[string]interface{})
	assert.Equal(t, "test_template", templateData["name"])
	assert.Equal(t, "en", templateData["language"].(map[string]interface{})["code"])

	components := templateData["components"].([]interface{})
	require.Len(t, components, 1)

	bodyComponent := components[0].(map[string]interface{})
	assert.Equal(t, "body", bodyComponent["type"])

	params := bodyComponent["parameters"].([]interface{})
	require.Len(t, params, 2)
	assert.Equal(t, "Hello", params[0].(map[string]interface{})["text"])
	assert.Equal(t, "World", params[1].(map[string]interface{})["text"])
}

func TestWorker_sendTemplateMessage_NoParams(t *testing.T) {
	w := testWorker(t)

	// Create mock server
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"messages": []map[string]interface{}{
				{"id": "wamid.test456"},
			},
		})
	}))
	defer server.Close()

	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	account := &models.WhatsAppAccount{
		PhoneID:     "123",
		BusinessID:  "456",
		AccessToken: "token",
		APIVersion:  "v21.0",
	}

	template := &models.Template{
		Name:     "simple_template",
		Language: "en",
	}

	recipient := &models.BulkMessageRecipient{
		PhoneNumber:    "1234567890",
		TemplateParams: nil, // No params
	}

	msgID, err := w.sendTemplateMessage(context.Background(), account, template, recipient, "")
	require.NoError(t, err)
	assert.Equal(t, "wamid.test456", msgID)

	// Verify no components when no params
	templateData := capturedBody["template"].(map[string]interface{})
	components, hasComponents := templateData["components"]
	if hasComponents {
		assert.Empty(t, components)
	}
}

func TestWorker_Close_NilConsumer(t *testing.T) {
	w := &Worker{
		Consumer: nil, // No consumer
	}

	err := w.Close()
	assert.NoError(t, err)
}

func TestWorker_HandleRecipientJob_Success(t *testing.T) {
	w := testWorker(t)
	org, account, template, campaign, recipient := createTestCampaignData(t, w)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"messages": []map[string]interface{}{
				{"id": "wamid.success123"},
			},
		})
	}))
	defer server.Close()

	// Update account's API version for URL building
	require.NoError(t, w.DB.Model(account).Update("api_version", "v21.0").Error)

	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
		RecipientName:  recipient.RecipientName,
		TemplateParams: recipient.TemplateParams,
	}

	err := w.HandleRecipientJob(context.Background(), job)
	require.NoError(t, err)

	// Verify recipient status updated
	var updatedRecipient models.BulkMessageRecipient
	require.NoError(t, w.DB.First(&updatedRecipient, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusSent, updatedRecipient.Status)
	assert.Equal(t, "wamid.success123", updatedRecipient.WhatsAppMessageID)

	// Verify campaign count incremented
	var updatedCampaign models.BulkMessageCampaign
	require.NoError(t, w.DB.First(&updatedCampaign, campaign.ID).Error)
	assert.Equal(t, 1, updatedCampaign.SentCount)

	// Verify message record created
	var message models.Message
	require.NoError(t, w.DB.Where("template_name = ?", template.Name).First(&message).Error)
	assert.Equal(t, models.MessageStatusSent, message.Status)
	assert.Equal(t, models.DirectionOutgoing, message.Direction)
	assert.Equal(t, models.MessageTypeTemplate, message.MessageType)
}

func TestWorker_HandleRecipientJob_WhatsAppError(t *testing.T) {
	w := testWorker(t)
	org, account, _, campaign, recipient := createTestCampaignData(t, w)

	// Create mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid phone number",
				"code":    100,
			},
		})
	}))
	defer server.Close()

	require.NoError(t, w.DB.Model(account).Update("api_version", "v21.0").Error)
	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
		RecipientName:  recipient.RecipientName,
		TemplateParams: recipient.TemplateParams,
	}

	err := w.HandleRecipientJob(context.Background(), job)
	require.NoError(t, err) // Job handler returns nil to not retry

	// Verify recipient marked as failed
	var updatedRecipient models.BulkMessageRecipient
	require.NoError(t, w.DB.First(&updatedRecipient, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, updatedRecipient.Status)
	assert.NotEmpty(t, updatedRecipient.ErrorMessage)

	// Verify campaign failed count incremented
	var updatedCampaign models.BulkMessageCampaign
	require.NoError(t, w.DB.First(&updatedCampaign, campaign.ID).Error)
	assert.Equal(t, 1, updatedCampaign.FailedCount)
}

func TestWorker_HandleRecipientJob_CreatesContact(t *testing.T) {
	w := testWorker(t)
	org, account, _, campaign, recipient := createTestCampaignData(t, w)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"messages": []map[string]interface{}{
				{"id": "wamid.contact123"},
			},
		})
	}))
	defer server.Close()

	require.NoError(t, w.DB.Model(account).Update("api_version", "v21.0").Error)
	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	// Use a new phone number that doesn't have a contact
	newPhone := "9998887777"
	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    newPhone,
		RecipientName:  "New Contact",
		TemplateParams: recipient.TemplateParams,
	}

	err := w.HandleRecipientJob(context.Background(), job)
	require.NoError(t, err)

	// Verify contact was created
	var contact models.Contact
	require.NoError(t, w.DB.Where("organization_id = ? AND phone_number = ?", org.ID, newPhone).First(&contact).Error)
	assert.Equal(t, "New Contact", contact.ProfileName)
}

func TestWorker_HandleRecipientJob_CampaignCompletion(t *testing.T) {
	w := testWorker(t)
	org, account, _, campaign, recipient := createTestCampaignData(t, w)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"messages": []map[string]interface{}{
				{"id": "wamid.complete123"},
			},
		})
	}))
	defer server.Close()

	require.NoError(t, w.DB.Model(account).Update("api_version", "v21.0").Error)
	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
		RecipientName:  recipient.RecipientName,
		TemplateParams: recipient.TemplateParams,
	}

	err := w.HandleRecipientJob(context.Background(), job)
	require.NoError(t, err)

	// Verify campaign is marked as completed (all recipients processed)
	var updatedCampaign models.BulkMessageCampaign
	require.NoError(t, w.DB.First(&updatedCampaign, campaign.ID).Error)
	assert.Equal(t, models.CampaignStatusCompleted, updatedCampaign.Status)
	assert.NotNil(t, updatedCampaign.CompletedAt)
}

func TestWorker_HandleRecipientJob_TemplateParamSubstitution(t *testing.T) {
	w := testWorker(t)
	org, account, template, campaign, recipient := createTestCampaignData(t, w)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"messages": []map[string]interface{}{
				{"id": "wamid.subst123"},
			},
		})
	}))
	defer server.Close()

	require.NoError(t, w.DB.Model(account).Update("api_version", "v21.0").Error)
	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
		RecipientName:  recipient.RecipientName,
		TemplateParams: models.JSONB{
			"1": "Alice",
			"2": "ORD-456",
		},
	}

	err := w.HandleRecipientJob(context.Background(), job)
	require.NoError(t, err)

	// Verify message content has substituted params
	var message models.Message
	require.NoError(t, w.DB.Where("template_name = ?", template.Name).Order("created_at desc").First(&message).Error)
	assert.Contains(t, message.Content, "Alice")
	assert.Contains(t, message.Content, "ORD-456")
	assert.NotContains(t, message.Content, "{{1}}")
	assert.NotContains(t, message.Content, "{{2}}")
}

// Unit tests for parameter resolution functions (no database required)

func TestResolveTemplateParams_NamedParams(t *testing.T) {
	template := &models.Template{
		BodyContent: "Hello {{name}}, your order {{order_id}} is ready!",
	}
	params := models.JSONB{
		"name":     "John",
		"order_id": "ORD-123",
	}

	result := resolveTemplateParams(template, params)

	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveTemplateParams_PositionalParams(t *testing.T) {
	template := &models.Template{
		BodyContent: "Hello {{1}}, your order {{2}} is ready!",
	}
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := resolveTemplateParams(template, params)

	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveTemplateParams_FallbackToPositional(t *testing.T) {
	// Named params in template, but user provides positional params
	template := &models.Template{
		BodyContent: "Hello {{name}}, your order {{order_id}} is ready!",
	}
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := resolveTemplateParams(template, params)

	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveTemplateParams_MixedParams(t *testing.T) {
	// User provides some named, some positional
	template := &models.Template{
		BodyContent: "Hello {{name}}, your order {{order_id}} is ready!",
	}
	params := models.JSONB{
		"name": "John",
		"2":    "ORD-123", // Positional fallback for second param
	}

	result := resolveTemplateParams(template, params)

	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveTemplateParams_NoParams(t *testing.T) {
	// Template without any parameters
	template := &models.Template{
		BodyContent: "Hello, your order is ready!",
	}
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := resolveTemplateParams(template, params)

	assert.Nil(t, result)
}

func TestResolveTemplateParams_EmptyParams(t *testing.T) {
	template := &models.Template{
		BodyContent: "Hello {{name}}!",
	}
	params := models.JSONB{}

	result := resolveTemplateParams(template, params)

	assert.Nil(t, result)
}

func TestReplaceTemplateContent_NamedParams(t *testing.T) {
	template := &models.Template{
		BodyContent: "Hello {{name}}, your order {{order_id}} is ready!",
	}
	content := "Hello {{name}}, your order {{order_id}} is ready!"
	params := models.JSONB{
		"name":     "John",
		"order_id": "ORD-123",
	}

	result := replaceTemplateContent(template, content, params)

	assert.Equal(t, "Hello John, your order ORD-123 is ready!", result)
}

func TestReplaceTemplateContent_PositionalParams(t *testing.T) {
	template := &models.Template{
		BodyContent: "Hello {{1}}, your order {{2}} is ready!",
	}
	content := "Hello {{1}}, your order {{2}} is ready!"
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := replaceTemplateContent(template, content, params)

	assert.Equal(t, "Hello John, your order ORD-123 is ready!", result)
}

func TestReplaceTemplateContent_NamedParamsWithPositionalInput(t *testing.T) {
	// Template has named placeholders but user provides positional params
	template := &models.Template{
		BodyContent: "Hello {{name}}, your order {{order_id}} is ready!",
	}
	content := "Hello {{name}}, your order {{order_id}} is ready!"
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := replaceTemplateContent(template, content, params)

	assert.Equal(t, "Hello John, your order ORD-123 is ready!", result)
}

func TestReplaceTemplateContent_NoParams(t *testing.T) {
	// Template without any parameters
	template := &models.Template{
		BodyContent: "Hello, your order is ready!",
	}
	content := "Hello, your order is ready!"
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := replaceTemplateContent(template, content, params)

	assert.Equal(t, "Hello, your order is ready!", result)
}
