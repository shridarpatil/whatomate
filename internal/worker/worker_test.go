package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/config"
	"github.com/nikyjain/whatomate/internal/crypto"
	"github.com/nikyjain/whatomate/internal/contactutil"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/internal/queue"
	"github.com/nikyjain/whatomate/internal/templateutil"
	"github.com/nikyjain/whatomate/pkg/whatsapp"
	"github.com/nikyjain/whatomate/test/testutil"
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

// getOrCreateTestPermissions gets existing permissions or creates them for testing.
func getOrCreateTestPermissions(t *testing.T, w *Worker) []models.Permission {
	t.Helper()

	var existingPerms []models.Permission
	if err := w.DB.Order("resource, action").Find(&existingPerms).Error; err == nil && len(existingPerms) > 0 {
		return existingPerms
	}

	// Create all default permissions if none exist
	perms := models.DefaultPermissions()
	for i := range perms {
		perms[i].ID = uuid.New()
	}
	require.NoError(t, w.DB.Create(&perms).Error)
	return perms
}

// createTestRole creates an admin role with all permissions for testing.
func createTestRole(t *testing.T, w *Worker, orgID uuid.UUID) *models.CustomRole {
	t.Helper()

	// Get or create permissions
	perms := getOrCreateTestPermissions(t, w)

	role := &models.CustomRole{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           "admin_" + uuid.New().String()[:8],
		Description:    "Test admin role",
		IsSystem:       false,
		IsDefault:      false,
		Permissions:    perms,
	}
	require.NoError(t, w.DB.Create(role).Error)
	return role
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

	// Create role for user
	role := createTestRole(t, w, org.ID)

	// Create user for CreatedBy foreign key
	user := &models.User{
		OrganizationID: org.ID,
		Email:          "test-" + uniqueID + "@example.com",
		PasswordHash:   "hashed",
		FullName:       "Test User",
		RoleID:         &role.ID,
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

func createRSVPReminderDeliveryForRecipient(
	t *testing.T,
	w *Worker,
	org *models.Organization,
	account *models.WhatsAppAccount,
	template *models.Template,
	campaign *models.BulkMessageCampaign,
	recipient *models.BulkMessageRecipient,
	startedAt *time.Time,
) (*models.RSVPReminderSchedule, *models.RSVPReminderDelivery) {
	t.Helper()
	contact := testutil.CreateTestContactWith(t, w.DB, org.ID,
		testutil.WithContactAccount(account.Name),
		testutil.WithPhoneNumber(recipient.PhoneNumber),
	)
	event := &models.RSVPEvent{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		Name:            "Worker RSVP",
		Status:          models.RSVPEventStatusActive,
		AccessMode:      models.RSVPAccessModeGuestList,
		WhatsAppAccount: account.Name,
		CreatedBy:       campaign.CreatedBy,
	}
	require.NoError(t, w.DB.Create(event).Error)
	response := &models.RSVPResponse{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		RSVPEventID:    event.ID,
		OrganizationID: org.ID,
		ContactID:      contact.ID,
		PhoneNumber:    contact.PhoneNumber,
		Attendance:     models.RSVPAttendancePending,
		Source:         models.RSVPGuestSourceContactSelection,
		RSVPStartedAt:  startedAt,
	}
	require.NoError(t, w.DB.Create(response).Error)
	require.NoError(t, w.DB.Model(campaign).Updates(map[string]interface{}{
		"source_type": models.CampaignSourceRSVPReminder,
		"source_id":   event.ID,
	}).Error)
	schedule := &models.RSVPReminderSchedule{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		RSVPEventID:    event.ID,
		OrganizationID: org.ID,
		ScheduledAt:    time.Now().UTC(),
		TemplateID:     template.ID,
		Status:         models.RSVPReminderScheduleProcessing,
		CreatedBy:      campaign.CreatedBy,
		CampaignID:     &campaign.ID,
	}
	require.NoError(t, w.DB.Create(schedule).Error)
	delivery := &models.RSVPReminderDelivery{
		BaseModel:           models.BaseModel{ID: uuid.New()},
		RSVPEventID:         event.ID,
		OrganizationID:      org.ID,
		RSVPResponseID:      response.ID,
		ScheduleID:          &schedule.ID,
		DeliveryType:        models.RSVPReminderDeliveryScheduled,
		Status:              models.RSVPReminderDeliveryQueued,
		AttemptedAt:         time.Now().UTC(),
		CampaignID:          &campaign.ID,
		CampaignRecipientID: &recipient.ID,
	}
	require.NoError(t, w.DB.Create(delivery).Error)
	return schedule, delivery
}

func TestWorker_RSVPReminderOutcomeCompletesLinkedSchedule(t *testing.T) {
	w := testWorker(t)
	org, account, template, campaign, recipient := createTestCampaignData(t, w)
	schedule, delivery := createRSVPReminderDeliveryForRecipient(t, w, org, account, template, campaign, recipient, nil)

	w.updateRecipientStatus(recipient.ID, models.MessageStatusSent, "wamid.reminder", "")
	w.incrementCampaignCount(campaign.ID, "sent_count")
	w.checkCampaignCompletion(context.Background(), campaign.ID, org.ID)

	require.NoError(t, w.DB.First(delivery, delivery.ID).Error)
	assert.Equal(t, models.RSVPReminderDeliverySent, delivery.Status)
	assert.Equal(t, "wamid.reminder", delivery.MessageID)
	require.NoError(t, w.DB.First(schedule, schedule.ID).Error)
	assert.Equal(t, models.RSVPReminderScheduleCompleted, schedule.Status)
	assert.Equal(t, 1, schedule.SentCount)
	assert.Zero(t, schedule.FailedCount)
	require.NotNil(t, schedule.ProcessedAt)
	require.NoError(t, w.DB.First(campaign, campaign.ID).Error)
	assert.Equal(t, models.CampaignStatusCompleted, campaign.Status)
}

func TestWorker_RSVPReminderSkipsGuestWhoStartedAfterQueueing(t *testing.T) {
	w := testWorker(t)
	org, account, template, campaign, recipient := createTestCampaignData(t, w)
	startedAt := time.Now().UTC()
	schedule, delivery := createRSVPReminderDeliveryForRecipient(t, w, org, account, template, campaign, recipient, &startedAt)
	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
	}

	assert.True(t, w.skipIneligibleRSVPReminder(context.Background(), job))

	require.NoError(t, w.DB.First(recipient, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, recipient.Status)
	require.NoError(t, w.DB.First(delivery, delivery.ID).Error)
	assert.Equal(t, models.RSVPReminderDeliverySkipped, delivery.Status)
	require.NoError(t, w.DB.First(schedule, schedule.ID).Error)
	assert.Equal(t, models.RSVPReminderScheduleCompleted, schedule.Status)
	assert.Zero(t, schedule.SentCount)
	assert.Zero(t, schedule.FailedCount)
	require.NoError(t, w.DB.First(campaign, campaign.ID).Error)
	assert.Equal(t, 1, campaign.FailedCount)
	assert.Equal(t, models.CampaignStatusCompleted, campaign.Status)
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

	// Create role for user
	role := createTestRole(t, w, org.ID)

	user := &models.User{
		OrganizationID: org.ID,
		Email:          "test-" + uniqueID + "@example.com",
		PasswordHash:   "hashed",
		FullName:       "Test User",
		RoleID:         &role.ID,
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
	require.NoError(t, w.DB.Model(campaign).Updates(map[string]any{
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
	require.NoError(t, w.DB.Model(campaign).Updates(map[string]any{
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
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"messages": []map[string]any{
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

	msgID, err := w.sendTemplateMessage(context.Background(), account, template, recipient, "", "")
	require.NoError(t, err)
	assert.Equal(t, "wamid.test123", msgID)

	// Verify request structure
	templateData := capturedBody["template"].(map[string]any)
	assert.Equal(t, "test_template", templateData["name"])
	assert.Equal(t, "en", templateData["language"].(map[string]any)["code"])

	components := templateData["components"].([]any)
	require.Len(t, components, 1)

	bodyComponent := components[0].(map[string]any)
	assert.Equal(t, "body", bodyComponent["type"])

	params := bodyComponent["parameters"].([]any)
	require.Len(t, params, 2)
	assert.Equal(t, "Hello", params[0].(map[string]any)["text"])
	assert.Equal(t, "World", params[1].(map[string]any)["text"])
}

// When the recipient has explicit HeaderParams, the worker must use them
// for the TEXT-header component instead of falling back to TemplateParams.
// This protects positional templates where header {{1}} and body {{1}}
// would otherwise share the same value.
func TestWorker_sendTemplateMessage_HeaderParamsTakePrecedence(t *testing.T) {
	w := testWorker(t)

	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"messages": []map[string]any{{"id": "wamid.hp-explicit"}},
		})
	}))
	defer server.Close()
	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	account := &models.WhatsAppAccount{PhoneID: "123", BusinessID: "456", AccessToken: "token", APIVersion: "v21.0"}
	template := &models.Template{
		Name:          "tpl_explicit_hp",
		Language:      "en",
		HeaderType:    "TEXT",
		HeaderContent: "Code {{1}}",
		BodyContent:   "Use {{1}} to redeem",
	}
	recipient := &models.BulkMessageRecipient{
		PhoneNumber:    "1234567890",
		HeaderParams:   models.JSONB{"1": "HEADER-VAL"},
		TemplateParams: models.JSONB{"1": "BODY-VAL"},
	}

	_, err := w.sendTemplateMessage(context.Background(), account, template, recipient, "", "")
	require.NoError(t, err)

	components := capturedBody["template"].(map[string]any)["components"].([]any)
	var headerComp, bodyComp map[string]any
	for _, c := range components {
		m := c.(map[string]any)
		switch m["type"] {
		case "header":
			headerComp = m
		case "body":
			bodyComp = m
		}
	}
	require.NotNil(t, headerComp)
	require.NotNil(t, bodyComp)
	assert.Equal(t, "HEADER-VAL", headerComp["parameters"].([]any)[0].(map[string]any)["text"])
	assert.Equal(t, "BODY-VAL", bodyComp["parameters"].([]any)[0].(map[string]any)["text"],
		"body {{1}} must keep its own value when HeaderParams is set")
}

// Legacy recipient rows persisted before HeaderParams existed only have
// TemplateParams. For NAMED templates the var name is unique across
// components, so the worker falls back to TemplateParams[name].
func TestWorker_sendTemplateMessage_HeaderParamsFallbackToTemplateParams(t *testing.T) {
	w := testWorker(t)

	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"messages": []map[string]any{{"id": "wamid.hp-fallback"}},
		})
	}))
	defer server.Close()
	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	account := &models.WhatsAppAccount{PhoneID: "123", BusinessID: "456", AccessToken: "token", APIVersion: "v21.0"}
	template := &models.Template{
		Name:          "tpl_fallback_hp",
		Language:      "en",
		HeaderType:    "TEXT",
		HeaderContent: "Our {{season}} sale",
		BodyContent:   "Hi {{name}}",
	}
	recipient := &models.BulkMessageRecipient{
		PhoneNumber: "1234567890",
		// No HeaderParams — legacy row. season lives in TemplateParams.
		TemplateParams: models.JSONB{"season": "Summer", "name": "Alex"},
	}

	_, err := w.sendTemplateMessage(context.Background(), account, template, recipient, "", "")
	require.NoError(t, err)

	components := capturedBody["template"].(map[string]any)["components"].([]any)
	var headerComp map[string]any
	for _, c := range components {
		if m := c.(map[string]any); m["type"] == "header" {
			headerComp = m
			break
		}
	}
	require.NotNil(t, headerComp, "header component must still be emitted via fallback")
	params := headerComp["parameters"].([]any)[0].(map[string]any)
	assert.Equal(t, "Summer", params["text"])
	assert.Equal(t, "season", params["parameter_name"])
}

func TestWorker_sendTemplateMessage_NoParams(t *testing.T) {
	w := testWorker(t)

	// Create mock server
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"messages": []map[string]any{
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

	msgID, err := w.sendTemplateMessage(context.Background(), account, template, recipient, "", "")
	require.NoError(t, err)
	assert.Equal(t, "wamid.test456", msgID)

	// Verify no components when no params
	templateData := capturedBody["template"].(map[string]any)
	components, hasComponents := templateData["components"]
	if hasComponents {
		assert.Empty(t, components)
	}
}

func TestWorker_sendTemplateMessage_WithDynamicURLButton(t *testing.T) {
	w := testWorker(t)

	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"messages": []map[string]interface{}{
				{"id": "wamid.test789"},
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
		Name:        "url_button_template",
		Language:    "en",
		BodyContent: "Track your order",
		Buttons: models.JSONBArray{
			map[string]interface{}{
				"type": "URL",
				"text": "Track",
				"url":  "https://example.com/{{tracking_code}}",
			},
		},
	}

	recipient := &models.BulkMessageRecipient{
		PhoneNumber: "1234567890",
		TemplateParams: models.JSONB{
			"tracking_code": "TRACK-123",
		},
	}

	msgID, err := w.sendTemplateMessage(context.Background(), account, template, recipient, "", "")
	require.NoError(t, err)
	assert.Equal(t, "wamid.test789", msgID)

	templateData := capturedBody["template"].(map[string]interface{})
	components := templateData["components"].([]interface{})
	// The body "Track your order" declares no variables, so Meta must receive
	// no body component (an extra body parameter is rejected with #132000);
	// only the dynamic URL button component is sent.
	require.Len(t, components, 1)

	buttonComponent := components[0].(map[string]interface{})
	assert.Equal(t, "button", buttonComponent["type"])
	assert.Equal(t, "url", buttonComponent["sub_type"])
	assert.Equal(t, "0", buttonComponent["index"])
	params := buttonComponent["parameters"].([]interface{})
	require.Len(t, params, 1)
	assert.Equal(t, "TRACK-123", params[0].(map[string]interface{})["text"])
}

func TestWorker_sendTemplateMessage_WithHeaderMediaURL(t *testing.T) {
	w := testWorker(t)

	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"messages": []map[string]interface{}{
				{"id": "wamid.header-link"},
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
		Name:        "header_link_template",
		Language:    "en",
		HeaderType:  "IMAGE",
		BodyContent: "Hello {{1}}",
	}

	recipient := &models.BulkMessageRecipient{
		PhoneNumber: "1234567890",
		TemplateParams: models.JSONB{
			"1": "Alice",
		},
	}

	msgID, err := w.sendTemplateMessage(context.Background(), account, template, recipient, "", "https://cdn.example.com/banner.jpg")
	require.NoError(t, err)
	assert.Equal(t, "wamid.header-link", msgID)

	templateData := capturedBody["template"].(map[string]interface{})
	components := templateData["components"].([]interface{})
	require.Len(t, components, 2)

	headerComponent := components[0].(map[string]interface{})
	assert.Equal(t, "header", headerComponent["type"])
	params := headerComponent["parameters"].([]interface{})
	require.Len(t, params, 1)
	imageParam := params[0].(map[string]interface{})
	assert.Equal(t, "image", imageParam["type"])
	assert.Equal(t, "https://cdn.example.com/banner.jpg", imageParam["image"].(map[string]interface{})["link"])
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

	campaign.HeaderMediaID = "media-123"
	campaign.HeaderMediaLocalPath = "/media/documents/invoice.pdf"
	campaign.HeaderMediaMimeType = "application/pdf"
	campaign.HeaderMediaFilename = "invoice.pdf"
	require.NoError(t, w.DB.Save(campaign).Error)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"messages": []map[string]any{
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
	assert.Equal(t, "/media/documents/invoice.pdf", message.MediaURL)
	assert.Equal(t, "application/pdf", message.MediaMimeType)
	assert.Equal(t, "invoice.pdf", message.MediaFilename)

	// Verify contact last_message_at and preview updated
	var updatedContact models.Contact
	require.NoError(t, w.DB.Where("organization_id = ? AND phone_number = ?", org.ID, contactutil.CanonicalPhone(recipient.PhoneNumber)).First(&updatedContact).Error)
	assert.NotNil(t, updatedContact.LastMessageAt)
	assert.NotEmpty(t, updatedContact.LastMessagePreview)
}

func TestWorker_HandleRecipientJob_WhatsAppError(t *testing.T) {
	w := testWorker(t)
	org, account, _, campaign, recipient := createTestCampaignData(t, w)

	// Create mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"error": map[string]any{
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

func TestWorker_HandleRecipientJob_UndeliverableMarksContactInactive(t *testing.T) {
	w := testWorker(t)
	org, account, _, campaign, recipient := createTestCampaignData(t, w)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Message Undeliverable.",
				"code":    131026,
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

	var contact models.Contact
	require.NoError(t, w.DB.Where("organization_id = ? AND phone_number = ?", org.ID, contactutil.CanonicalPhone(recipient.PhoneNumber)).First(&contact).Error)
	assert.False(t, contact.IsActive)
}

func TestWorker_HandleRecipientJob_SkipsInactiveContact(t *testing.T) {
	w := testWorker(t)
	org, account, _, campaign, recipient := createTestCampaignData(t, w)

	// Contact.IsActive has a gorm default of true, so a zero-value false is
	// dropped on INSERT; create the contact first, then deactivate it.
	inactiveContact := &models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
		ProfileName:    "Inactive Contact",
	}
	require.NoError(t, w.DB.Create(inactiveContact).Error)
	require.NoError(t, w.DB.Model(inactiveContact).Update("is_active", false).Error)

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		called = true
		rw.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(rw).Encode(map[string]interface{}{
			"messages": []map[string]interface{}{{"id": "wamid.should-not-send"}},
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
	assert.False(t, called)

	var updatedRecipient models.BulkMessageRecipient
	require.NoError(t, w.DB.First(&updatedRecipient, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, updatedRecipient.Status)
	assert.Equal(t, "Contact is inactive", updatedRecipient.ErrorMessage)

	var messageCount int64
	require.NoError(t, w.DB.Model(&models.Message{}).Where("contact_id IN (SELECT id FROM contacts WHERE organization_id = ? AND phone_number = ?)", org.ID, contactutil.CanonicalPhone(recipient.PhoneNumber)).Count(&messageCount).Error)
	assert.EqualValues(t, 0, messageCount)
}

func TestWorker_HandleRecipientJob_CreatesContact(t *testing.T) {
	w := testWorker(t)
	org, account, _, campaign, recipient := createTestCampaignData(t, w)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"messages": []map[string]any{
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
	require.NoError(t, w.DB.Where("organization_id = ? AND phone_number = ?", org.ID, contactutil.CanonicalPhone(newPhone)).First(&contact).Error)
	assert.Equal(t, "New Contact", contact.ProfileName)
}

func TestWorker_HandleRecipientJob_CampaignCompletion(t *testing.T) {
	w := testWorker(t)
	org, account, _, campaign, recipient := createTestCampaignData(t, w)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"messages": []map[string]any{
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
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"messages": []map[string]any{
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

func TestWorker_DecryptAccountSecrets_WithEncryptionKey(t *testing.T) {
	w := &Worker{
		Config: &config.Config{
			App: config.AppConfig{EncryptionKey: "test-secret-key-for-aes256"},
		},
	}

	// Encrypt a token
	plainToken := "EAAI2ZCP4ZAMv8BQtest"
	plainSecret := "app-secret-123"
	encToken, err := crypto.Encrypt(plainToken, w.Config.App.EncryptionKey)
	require.NoError(t, err)
	encSecret, err := crypto.Encrypt(plainSecret, w.Config.App.EncryptionKey)
	require.NoError(t, err)

	// Verify they are actually encrypted
	assert.True(t, crypto.IsEncrypted(encToken))
	assert.True(t, crypto.IsEncrypted(encSecret))

	account := &models.WhatsAppAccount{
		AccessToken: encToken,
		AppSecret:   encSecret,
	}

	w.decryptAccountSecrets(account)

	assert.Equal(t, plainToken, account.AccessToken)
	assert.Equal(t, plainSecret, account.AppSecret)
}

func TestWorker_DecryptAccountSecrets_NilConfig(t *testing.T) {
	w := &Worker{}

	account := &models.WhatsAppAccount{
		AccessToken: "plain-token",
		AppSecret:   "plain-secret",
	}

	w.decryptAccountSecrets(account)

	// Should remain unchanged (no-op)
	assert.Equal(t, "plain-token", account.AccessToken)
	assert.Equal(t, "plain-secret", account.AppSecret)
}

func TestWorker_HandleRecipientJob_WithEncryptedToken(t *testing.T) {
	w := testWorker(t)

	encKey := "test-encryption-key-for-aes"
	w.Config = &config.Config{
		App: config.AppConfig{EncryptionKey: encKey},
	}

	org, account, template, campaign, recipient := createTestCampaignData(t, w)

	// Encrypt the token in the DB (simulating production)
	encToken, err := crypto.Encrypt("test-token", encKey)
	require.NoError(t, err)
	require.NoError(t, w.DB.Model(account).Updates(map[string]any{
		"access_token": encToken,
		"api_version":  "v21.0",
	}).Error)

	// Create mock server that verifies the decrypted token arrives
	var capturedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"messages": []map[string]any{
				{"id": "wamid.encrypted123"},
			},
		})
	}))
	defer server.Close()

	w.WhatsApp = whatsapp.NewWithBaseURL(w.Log, server.URL)

	job := &queue.RecipientJob{
		CampaignID:     campaign.ID,
		RecipientID:    recipient.ID,
		OrganizationID: org.ID,
		PhoneNumber:    recipient.PhoneNumber,
		RecipientName:  recipient.RecipientName,
		TemplateParams: recipient.TemplateParams,
	}

	err = w.HandleRecipientJob(context.Background(), job)
	require.NoError(t, err)

	// Verify the decrypted token was sent to Meta API (not the encrypted one)
	assert.Equal(t, "Bearer test-token", capturedAuth)
	assert.NotContains(t, capturedAuth, "enc:")

	// Verify recipient marked as sent
	var updatedRecipient models.BulkMessageRecipient
	require.NoError(t, w.DB.First(&updatedRecipient, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusSent, updatedRecipient.Status)
	assert.Equal(t, "wamid.encrypted123", updatedRecipient.WhatsAppMessageID)

	// Verify message record created
	var message models.Message
	require.NoError(t, w.DB.Where("template_name = ?", template.Name).Order("created_at desc").First(&message).Error)
	assert.Equal(t, models.MessageStatusSent, message.Status)
}

// Unit tests for parameter resolution functions (no database required)

func TestResolveTemplateParams_NamedParams(t *testing.T) {
	bodyContent := "Hello {{name}}, your order {{order_id}} is ready!"
	params := models.JSONB{
		"name":     "John",
		"order_id": "ORD-123",
	}

	result := templateutil.ResolveParams(bodyContent, params)

	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveTemplateParams_PositionalParams(t *testing.T) {
	bodyContent := "Hello {{1}}, your order {{2}} is ready!"
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := templateutil.ResolveParams(bodyContent, params)

	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveTemplateParams_FallbackToPositional(t *testing.T) {
	// Named params in template, but user provides positional params
	bodyContent := "Hello {{name}}, your order {{order_id}} is ready!"
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := templateutil.ResolveParams(bodyContent, params)

	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveTemplateParams_MixedParams(t *testing.T) {
	// User provides some named, some positional
	bodyContent := "Hello {{name}}, your order {{order_id}} is ready!"
	params := models.JSONB{
		"name": "John",
		"2":    "ORD-123", // Positional fallback for second param
	}

	result := templateutil.ResolveParams(bodyContent, params)

	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveTemplateParams_NoParams(t *testing.T) {
	// Template without any parameters
	bodyContent := "Hello, your order is ready!"
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := templateutil.ResolveParams(bodyContent, params)

	assert.Nil(t, result)
}

func TestResolveTemplateParams_EmptyParams(t *testing.T) {
	bodyContent := "Hello {{name}}!"
	params := models.JSONB{}

	result := templateutil.ResolveParams(bodyContent, params)

	assert.Nil(t, result)
}

func TestReplaceTemplateContent_NamedParams(t *testing.T) {
	bodyContent := "Hello {{name}}, your order {{order_id}} is ready!"
	content := "Hello {{name}}, your order {{order_id}} is ready!"
	params := models.JSONB{
		"name":     "John",
		"order_id": "ORD-123",
	}

	result := templateutil.ReplaceWithJSONBParams(bodyContent, content, params)

	assert.Equal(t, "Hello John, your order ORD-123 is ready!", result)
}

func TestReplaceTemplateContent_PositionalParams(t *testing.T) {
	bodyContent := "Hello {{1}}, your order {{2}} is ready!"
	content := "Hello {{1}}, your order {{2}} is ready!"
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := templateutil.ReplaceWithJSONBParams(bodyContent, content, params)

	assert.Equal(t, "Hello John, your order ORD-123 is ready!", result)
}

func TestReplaceTemplateContent_NamedParamsWithPositionalInput(t *testing.T) {
	// Template has named placeholders but user provides positional params
	bodyContent := "Hello {{name}}, your order {{order_id}} is ready!"
	content := "Hello {{name}}, your order {{order_id}} is ready!"
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := templateutil.ReplaceWithJSONBParams(bodyContent, content, params)

	assert.Equal(t, "Hello John, your order ORD-123 is ready!", result)
}

func TestReplaceTemplateContent_NoParams(t *testing.T) {
	// Template without any parameters
	bodyContent := "Hello, your order is ready!"
	content := "Hello, your order is ready!"
	params := models.JSONB{
		"1": "John",
		"2": "ORD-123",
	}

	result := templateutil.ReplaceWithJSONBParams(bodyContent, content, params)

	assert.Equal(t, "Hello, your order is ready!", result)
}
