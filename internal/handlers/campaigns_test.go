package handlers_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// createTestCampaign creates a test campaign in the database.
func createTestCampaign(t *testing.T, app *handlers.App, orgID, templateID, userID uuid.UUID, whatsappAccount string, status models.CampaignStatus) *models.BulkMessageCampaign {
	t.Helper()

	campaign := &models.BulkMessageCampaign{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		Name:            "Test Campaign " + uuid.New().String()[:8],
		WhatsAppAccount: whatsappAccount,
		TemplateID:      templateID,
		Status:          status,
		CreatedBy:       userID,
	}
	require.NoError(t, app.DB.Create(campaign).Error)
	return campaign
}

// createTestRecipient creates a test recipient for a campaign.
func createTestRecipient(t *testing.T, app *handlers.App, campaignID uuid.UUID, phone string, status models.MessageStatus) *models.BulkMessageRecipient {
	t.Helper()

	recipient := &models.BulkMessageRecipient{
		BaseModel:     models.BaseModel{ID: uuid.New()},
		CampaignID:    campaignID,
		PhoneNumber:   phone,
		RecipientName: "Test Recipient",
		Status:        status,
	}
	require.NoError(t, app.DB.Create(recipient).Error)
	return recipient
}

// --- ListCampaigns Tests ---

func TestApp_ListCampaigns_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-campaigns")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("test-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)

	// Create multiple campaigns
	createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusCompleted)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListCampaigns(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Campaigns []handlers.CampaignResponse `json:"campaigns"`
			Total     int                         `json:"total"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Data.Total)
	assert.Len(t, resp.Data.Campaigns, 2)
}

func TestApp_ListCampaigns_FilterByStatus(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-filter")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("test-account-filter"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)

	createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusCompleted)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "status", models.CampaignStatusDraft)

	err := app.ListCampaigns(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Campaigns []handlers.CampaignResponse `json:"campaigns"`
			Total     int                         `json:"total"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Data.Total)
	assert.Equal(t, models.CampaignStatusDraft, resp.Data.Campaigns[0].Status)
}

func TestApp_ListCampaigns_Unauthorized(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))

	req := testutil.NewGETRequest(t)
	// No auth context set

	err := app.ListCampaigns(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- CreateCampaign Tests ---

func TestApp_CreateCampaign_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-campaign")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("create-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Test Campaign",
		"whatsapp_account": account.Name,
		"template_id":      template.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CampaignResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Test Campaign", resp.Data.Name)
	assert.Equal(t, models.CampaignStatusDraft, resp.Data.Status)
	assert.Equal(t, template.ID, resp.Data.TemplateID)
}

func TestApp_CreateCampaign_WithScheduledAt(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-scheduled")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("scheduled-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)

	scheduledAt := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Scheduled Campaign",
		"whatsapp_account": account.Name,
		"template_id":      template.ID.String(),
		"scheduled_at":     scheduledAt,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CampaignResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Data.ScheduledAt)
}

func TestApp_CreateCampaign_InvalidTemplateID(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("invalid-template")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("invalid-template-account"))

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Test Campaign",
		"whatsapp_account": account.Name,
		"template_id":      "not-a-valid-uuid",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateCampaign_TemplateNotFound(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("template-not-found")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("no-template-account"))

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Test Campaign",
		"whatsapp_account": account.Name,
		"template_id":      uuid.New().String(),
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateCampaign_AccountNotFound(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("account-not-found")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("temp-account-for-template"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Test Campaign",
		"whatsapp_account": "nonexistent-account",
		"template_id":      template.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateCampaign_InvalidRequestBody(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("invalid-body")), testutil.WithPassword("password"))

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- GetCampaign Tests ---

func TestApp_GetCampaign_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-campaign")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("get-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.GetCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CampaignResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, campaign.ID, resp.Data.ID)
	assert.Equal(t, campaign.Name, resp.Data.Name)
}

func TestApp_GetCampaign_NotFound(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-not-found")), testutil.WithPassword("password"))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetCampaign_InvalidID(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-invalid-id")), testutil.WithPassword("password"))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- UpdateCampaign Tests ---

func TestApp_UpdateCampaign_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-campaign")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("update-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Updated Campaign Name",
		"whatsapp_account": account.Name,
		"template_id":      template.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.UpdateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CampaignResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Updated Campaign Name", resp.Data.Name)
}

func TestApp_UpdateCampaign_NotDraft(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-not-draft")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("update-not-draft-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Updated Name",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.UpdateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateCampaign_NotFound(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-not-found")), testutil.WithPassword("password"))

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Updated Name",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- DeleteCampaign Tests ---

func TestApp_DeleteCampaign_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-campaign")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("delete-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.DeleteCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify campaign is deleted
	var count int64
	app.DB.Model(&models.BulkMessageCampaign{}).Where("id = ?", campaign.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteCampaign_WithRecipients(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-with-recipients")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("delete-recipients-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)
	createTestRecipient(t, app, campaign.ID, "+0987654321", models.MessageStatusPending)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.DeleteCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify recipients are also deleted
	var count int64
	app.DB.Model(&models.BulkMessageRecipient{}).Where("campaign_id = ?", campaign.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteCampaign_RunningCampaign(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-running")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("delete-running-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.DeleteCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteCampaign_NotFound(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-not-found")), testutil.WithPassword("password"))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- StartCampaign Tests ---

func TestApp_StartCampaign_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("start-campaign")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("start-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)
	createTestRecipient(t, app, campaign.ID, "+0987654321", models.MessageStatusPending)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.StartCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify jobs were enqueued
	assert.Len(t, mockQueue.Jobs, 2)

	// Verify campaign status changed
	var updated models.BulkMessageCampaign
	app.DB.Where("id = ?", campaign.ID).First(&updated)
	assert.Equal(t, models.CampaignStatusProcessing, updated.Status)
	assert.NotNil(t, updated.StartedAt)
}

func TestApp_StartCampaign_NoPendingRecipients(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("start-no-recipients")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("start-no-recipients-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	// No recipients added

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.StartCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_StartCampaign_InvalidStatus(t *testing.T) {
	statuses := []models.CampaignStatus{models.CampaignStatusProcessing, models.CampaignStatusCompleted, models.CampaignStatusCancelled}

	for _, status := range statuses {
		t.Run("status_"+string(status), func(t *testing.T) {
			mockQueue := testutil.NewMockQueue()
			app := newTestApp(t, withQueue(mockQueue))
			org := testutil.CreateTestOrganization(t, app.DB)
			user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("start-invalid-"+string(status))), testutil.WithPassword("password"))
			account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("start-invalid-"+string(status)))
			template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
			campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, status)
			createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)

			req := testutil.NewJSONRequest(t, nil)
			testutil.SetAuthContext(req, org.ID, user.ID)
			testutil.SetPathParam(req, "id", campaign.ID.String())

			err := app.StartCampaign(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
		})
	}
}

func TestApp_StartCampaign_CanResumePaused(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("resume-paused")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("resume-paused-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusPaused)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.StartCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	assert.Len(t, mockQueue.Jobs, 1)
}

// --- PauseCampaign Tests ---

func TestApp_PauseCampaign_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("pause-campaign")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("pause-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.PauseCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var updated models.BulkMessageCampaign
	app.DB.Where("id = ?", campaign.ID).First(&updated)
	assert.Equal(t, models.CampaignStatusPaused, updated.Status)
}

func TestApp_PauseCampaign_NotRunning(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("pause-not-running")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("pause-not-running-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.PauseCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- CancelCampaign Tests ---

func TestApp_CancelCampaign_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("cancel-campaign")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("cancel-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.CancelCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var updated models.BulkMessageCampaign
	app.DB.Where("id = ?", campaign.ID).First(&updated)
	assert.Equal(t, models.CampaignStatusCancelled, updated.Status)
}

func TestApp_CancelCampaign_AlreadyFinished(t *testing.T) {
	finishedStatuses := []models.CampaignStatus{models.CampaignStatusCompleted, models.CampaignStatusCancelled}

	for _, status := range finishedStatuses {
		t.Run("status_"+string(status), func(t *testing.T) {
			mockQueue := testutil.NewMockQueue()
			app := newTestApp(t, withQueue(mockQueue))
			org := testutil.CreateTestOrganization(t, app.DB)
			user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("cancel-finished-"+string(status))), testutil.WithPassword("password"))
			account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("cancel-finished-"+string(status)))
			template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
			campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, status)

			req := testutil.NewJSONRequest(t, nil)
			testutil.SetAuthContext(req, org.ID, user.ID)
			testutil.SetPathParam(req, "id", campaign.ID.String())

			err := app.CancelCampaign(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
		})
	}
}

// --- ImportRecipients Tests ---

func TestApp_ImportRecipients_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("import-recipients")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("import-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"recipients": []map[string]interface{}{
			{"phone_number": "+1234567890", "recipient_name": "John Doe"},
			{"phone_number": "+0987654321", "recipient_name": "Jane Doe"},
		},
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.ImportRecipients(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message         string `json:"message"`
			AddedCount      int    `json:"added_count"`
			TotalRecipients int64  `json:"total_recipients"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Data.AddedCount)
	assert.Equal(t, int64(2), resp.Data.TotalRecipients)
}

func TestApp_ImportRecipients_WithTemplateParams(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("import-with-params")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("import-params-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"recipients": []map[string]interface{}{
			{
				"phone_number":    "+1234567890",
				"recipient_name":  "John Doe",
				"template_params": map[string]interface{}{"1": "John", "2": "Welcome"},
			},
		},
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.ImportRecipients(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify recipient has template params
	var recipient models.BulkMessageRecipient
	app.DB.Where("campaign_id = ?", campaign.ID).First(&recipient)
	assert.NotNil(t, recipient.TemplateParams)
}

func TestApp_ImportRecipients_NotDraft(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("import-not-draft")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("import-not-draft-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"recipients": []map[string]interface{}{
			{"phone_number": "+1234567890"},
		},
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.ImportRecipients(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- GetCampaignRecipients Tests ---

func TestApp_GetCampaignRecipients_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-recipients")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("get-recipients-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)
	createTestRecipient(t, app, campaign.ID, "+0987654321", models.MessageStatusSent)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.GetCampaignRecipients(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Recipients []models.BulkMessageRecipient `json:"recipients"`
			Total      int                           `json:"total"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Data.Total)
}

func TestApp_GetCampaignRecipients_CampaignNotFound(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-recipients-not-found")), testutil.WithPassword("password"))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetCampaignRecipients(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- RetryFailed Tests ---

func TestApp_RetryFailed_Success(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("retry-failed")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("retry-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusCompleted)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusSent)
	createTestRecipient(t, app, campaign.ID, "+0987654321", models.MessageStatusFailed)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.RetryFailed(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify only failed recipients were enqueued
	assert.Len(t, mockQueue.Jobs, 1)
	assert.Equal(t, "+0987654321", mockQueue.Jobs[0].PhoneNumber)

	var resp struct {
		Data struct {
			RetryCount int    `json:"retry_count"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Data.RetryCount)
	assert.Equal(t, string(models.CampaignStatusProcessing), resp.Data.Status)
}

func TestApp_RetryFailed_NoFailedRecipients(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("retry-no-failed")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("retry-no-failed-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusCompleted)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusSent)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.RetryFailed(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_RetryFailed_InvalidStatus(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("retry-invalid-status")), testutil.WithPassword("password"))
	account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, testutil.WithAccountName("retry-invalid-account"))
	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusFailed)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.RetryFailed(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- Cross-Organization Tests ---

func TestApp_Campaign_CrossOrgIsolation(t *testing.T) {
	mockQueue := testutil.NewMockQueue()
	app := newTestApp(t, withQueue(mockQueue))

	// Create two organizations
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)

	user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("cross-org-1")), testutil.WithPassword("password"))
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("cross-org-2")), testutil.WithPassword("password"))

	account1 := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org1.ID, testutil.WithAccountName("cross-org-account-1"))
	template1 := testutil.CreateTestTemplate(t, app.DB, org1.ID, account1.Name)
	campaign1 := createTestCampaign(t, app, org1.ID, template1.ID, user1.ID, account1.Name, models.CampaignStatusDraft)

	// User from org2 tries to access org1's campaign
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", campaign1.ID.String())

	err := app.GetCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}
