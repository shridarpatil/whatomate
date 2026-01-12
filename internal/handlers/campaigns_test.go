package handlers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// MockQueue implements queue.Queue for testing
type MockQueue struct {
	EnqueuedJobs []*queue.RecipientJob
	EnqueueErr   error
}

func (m *MockQueue) EnqueueRecipient(ctx context.Context, job *queue.RecipientJob) error {
	if m.EnqueueErr != nil {
		return m.EnqueueErr
	}
	m.EnqueuedJobs = append(m.EnqueuedJobs, job)
	return nil
}

func (m *MockQueue) EnqueueRecipients(ctx context.Context, jobs []*queue.RecipientJob) error {
	if m.EnqueueErr != nil {
		return m.EnqueueErr
	}
	m.EnqueuedJobs = append(m.EnqueuedJobs, jobs...)
	return nil
}

func (m *MockQueue) Close() error {
	return nil
}

// campaignTestApp creates an App instance for campaign testing.
func campaignTestApp(t *testing.T) (*handlers.App, *MockQueue) {
	t.Helper()

	db := testutil.SetupTestDB(t)
	log := testutil.NopLogger()
	mockQueue := &MockQueue{}

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:            testJWTSecret,
			AccessExpiryMins:  15,
			RefreshExpiryDays: 7,
		},
	}

	return &handlers.App{
		Config: cfg,
		DB:     db,
		Log:    log,
		Queue:  mockQueue,
	}, mockQueue
}

// createTestTemplate creates a test template in the database.
func createTestTemplate(t *testing.T, app *handlers.App, orgID uuid.UUID, accountName string) *models.Template {
	t.Helper()

	template := &models.Template{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: accountName,
		Name:            "test-template-" + uuid.New().String()[:8],
		MetaTemplateID:  "meta-" + uuid.New().String()[:8],
		Category:        "MARKETING",
		Language:        "en",
		Status:          string(models.TemplateStatusApproved),
		BodyContent:     "Hello {{1}}",
	}
	require.NoError(t, app.DB.Create(template).Error)
	return template
}

// createTestWhatsAppAccount creates a test WhatsApp account in the database.
func createTestWhatsAppAccount(t *testing.T, app *handlers.App, orgID uuid.UUID, name string) *models.WhatsAppAccount {
	t.Helper()

	account := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     orgID,
		Name:               name,
		PhoneID:            "phone-" + uuid.New().String()[:8],
		BusinessID:         "business-" + uuid.New().String()[:8],
		AccessToken:        "test-token",
		WebhookVerifyToken: "webhook-token",
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, app.DB.Create(account).Error)
	return account
}

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

// setAuthContext sets organization and user ID in request context.
func setAuthContext(req *fastglue.Request, orgID, userID uuid.UUID) {
	req.RequestCtx.SetUserValue("organization_id", orgID)
	req.RequestCtx.SetUserValue("user_id", userID)
}

// --- ListCampaigns Tests ---

func TestApp_ListCampaigns_Success(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("list-campaigns"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "test-account")
	template := createTestTemplate(t, app, org.ID, account.Name)

	// Create multiple campaigns
	createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusCompleted)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)

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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("list-filter"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "test-account-filter")
	template := createTestTemplate(t, app, org.ID, account.Name)

	createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusCompleted)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
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
	app, _ := campaignTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context set

	err := app.ListCampaigns(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- CreateCampaign Tests ---

func TestApp_CreateCampaign_Success(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-campaign"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "create-account")
	template := createTestTemplate(t, app, org.ID, account.Name)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Test Campaign",
		"whatsapp_account": account.Name,
		"template_id":      template.ID.String(),
	})
	setAuthContext(req, org.ID, user.ID)

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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-scheduled"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "scheduled-account")
	template := createTestTemplate(t, app, org.ID, account.Name)

	scheduledAt := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Scheduled Campaign",
		"whatsapp_account": account.Name,
		"template_id":      template.ID.String(),
		"scheduled_at":     scheduledAt,
	})
	setAuthContext(req, org.ID, user.ID)

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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("invalid-template"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "invalid-template-account")

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Test Campaign",
		"whatsapp_account": account.Name,
		"template_id":      "not-a-valid-uuid",
	})
	setAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateCampaign_TemplateNotFound(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("template-not-found"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "no-template-account")

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Test Campaign",
		"whatsapp_account": account.Name,
		"template_id":      uuid.New().String(),
	})
	setAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateCampaign_AccountNotFound(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("account-not-found"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "temp-account-for-template")
	template := createTestTemplate(t, app, org.ID, account.Name)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Test Campaign",
		"whatsapp_account": "nonexistent-account",
		"template_id":      template.ID.String(),
	})
	setAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateCampaign_InvalidRequestBody(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("invalid-body"), "password", models.RoleAdmin, true)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")
	setAuthContext(req, org.ID, user.ID)

	err := app.CreateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- GetCampaign Tests ---

func TestApp_GetCampaign_Success(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("get-campaign"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "get-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("get-not-found"), "password", models.RoleAdmin, true)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetCampaign_InvalidID(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("get-invalid-id"), "password", models.RoleAdmin, true)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- UpdateCampaign Tests ---

func TestApp_UpdateCampaign_Success(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("update-campaign"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "update-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":             "Updated Campaign Name",
		"whatsapp_account": account.Name,
		"template_id":      template.ID.String(),
	})
	setAuthContext(req, org.ID, user.ID)
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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("update-not-draft"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "update-not-draft-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Updated Name",
	})
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.UpdateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateCampaign_NotFound(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("update-not-found"), "password", models.RoleAdmin, true)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Updated Name",
	})
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- DeleteCampaign Tests ---

func TestApp_DeleteCampaign_Success(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("delete-campaign"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "delete-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("delete-with-recipients"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "delete-recipients-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)
	createTestRecipient(t, app, campaign.ID, "+0987654321", models.MessageStatusPending)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("delete-running"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "delete-running-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.DeleteCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteCampaign_NotFound(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("delete-not-found"), "password", models.RoleAdmin, true)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- StartCampaign Tests ---

func TestApp_StartCampaign_Success(t *testing.T) {
	app, mockQueue := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("start-campaign"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "start-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)
	createTestRecipient(t, app, campaign.ID, "+0987654321", models.MessageStatusPending)

	req := testutil.NewJSONRequest(t, nil)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.StartCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify jobs were enqueued
	assert.Len(t, mockQueue.EnqueuedJobs, 2)

	// Verify campaign status changed
	var updated models.BulkMessageCampaign
	app.DB.Where("id = ?", campaign.ID).First(&updated)
	assert.Equal(t, models.CampaignStatusProcessing, updated.Status)
	assert.NotNil(t, updated.StartedAt)
}

func TestApp_StartCampaign_NoPendingRecipients(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("start-no-recipients"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "start-no-recipients-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	// No recipients added

	req := testutil.NewJSONRequest(t, nil)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.StartCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_StartCampaign_InvalidStatus(t *testing.T) {
	statuses := []models.CampaignStatus{models.CampaignStatusProcessing, models.CampaignStatusCompleted, models.CampaignStatusCancelled}

	for _, status := range statuses {
		t.Run("status_"+string(status), func(t *testing.T) {
			app, _ := campaignTestApp(t)
			org := createTestOrganization(t, app)
			user := createTestUser(t, app, org.ID, uniqueEmail("start-invalid-"+string(status)), "password", models.RoleAdmin, true)
			account := createTestWhatsAppAccount(t, app, org.ID, "start-invalid-"+string(status))
			template := createTestTemplate(t, app, org.ID, account.Name)
			campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, status)
			createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)

			req := testutil.NewJSONRequest(t, nil)
			setAuthContext(req, org.ID, user.ID)
			testutil.SetPathParam(req, "id", campaign.ID.String())

			err := app.StartCampaign(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
		})
	}
}

func TestApp_StartCampaign_CanResumePaused(t *testing.T) {
	app, mockQueue := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("resume-paused"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "resume-paused-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusPaused)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)

	req := testutil.NewJSONRequest(t, nil)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.StartCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	assert.Len(t, mockQueue.EnqueuedJobs, 1)
}

// --- PauseCampaign Tests ---

func TestApp_PauseCampaign_Success(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("pause-campaign"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "pause-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewJSONRequest(t, nil)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.PauseCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var updated models.BulkMessageCampaign
	app.DB.Where("id = ?", campaign.ID).First(&updated)
	assert.Equal(t, models.CampaignStatusPaused, updated.Status)
}

func TestApp_PauseCampaign_NotRunning(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("pause-not-running"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "pause-not-running-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewJSONRequest(t, nil)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.PauseCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- CancelCampaign Tests ---

func TestApp_CancelCampaign_Success(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("cancel-campaign"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "cancel-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewJSONRequest(t, nil)
	setAuthContext(req, org.ID, user.ID)
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
			app, _ := campaignTestApp(t)
			org := createTestOrganization(t, app)
			user := createTestUser(t, app, org.ID, uniqueEmail("cancel-finished-"+string(status)), "password", models.RoleAdmin, true)
			account := createTestWhatsAppAccount(t, app, org.ID, "cancel-finished-"+string(status))
			template := createTestTemplate(t, app, org.ID, account.Name)
			campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, status)

			req := testutil.NewJSONRequest(t, nil)
			setAuthContext(req, org.ID, user.ID)
			testutil.SetPathParam(req, "id", campaign.ID.String())

			err := app.CancelCampaign(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
		})
	}
}

// --- ImportRecipients Tests ---

func TestApp_ImportRecipients_Success(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("import-recipients"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "import-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"recipients": []map[string]interface{}{
			{"phone_number": "+1234567890", "recipient_name": "John Doe"},
			{"phone_number": "+0987654321", "recipient_name": "Jane Doe"},
		},
	})
	setAuthContext(req, org.ID, user.ID)
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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("import-with-params"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "import-params-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
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
	setAuthContext(req, org.ID, user.ID)
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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("import-not-draft"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "import-not-draft-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusProcessing)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"recipients": []map[string]interface{}{
			{"phone_number": "+1234567890"},
		},
	})
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.ImportRecipients(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- GetCampaignRecipients Tests ---

func TestApp_GetCampaignRecipients_Success(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("get-recipients"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "get-recipients-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusPending)
	createTestRecipient(t, app, campaign.ID, "+0987654321", models.MessageStatusSent)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("get-recipients-not-found"), "password", models.RoleAdmin, true)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetCampaignRecipients(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- RetryFailed Tests ---

func TestApp_RetryFailed_Success(t *testing.T) {
	app, mockQueue := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("retry-failed"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "retry-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusCompleted)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusSent)
	createTestRecipient(t, app, campaign.ID, "+0987654321", models.MessageStatusFailed)

	req := testutil.NewJSONRequest(t, nil)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.RetryFailed(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify only failed recipients were enqueued
	assert.Len(t, mockQueue.EnqueuedJobs, 1)
	assert.Equal(t, "+0987654321", mockQueue.EnqueuedJobs[0].PhoneNumber)

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
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("retry-no-failed"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "retry-no-failed-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusCompleted)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusSent)

	req := testutil.NewJSONRequest(t, nil)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.RetryFailed(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_RetryFailed_InvalidStatus(t *testing.T) {
	app, _ := campaignTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("retry-invalid-status"), "password", models.RoleAdmin, true)
	account := createTestWhatsAppAccount(t, app, org.ID, "retry-invalid-account")
	template := createTestTemplate(t, app, org.ID, account.Name)
	campaign := createTestCampaign(t, app, org.ID, template.ID, user.ID, account.Name, models.CampaignStatusDraft)
	createTestRecipient(t, app, campaign.ID, "+1234567890", models.MessageStatusFailed)

	req := testutil.NewJSONRequest(t, nil)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", campaign.ID.String())

	err := app.RetryFailed(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- Cross-Organization Tests ---

func TestApp_Campaign_CrossOrgIsolation(t *testing.T) {
	app, _ := campaignTestApp(t)

	// Create two organizations
	org1 := createTestOrganization(t, app)
	org2 := createTestOrganization(t, app)

	user1 := createTestUser(t, app, org1.ID, uniqueEmail("cross-org-1"), "password", models.RoleAdmin, true)
	user2 := createTestUser(t, app, org2.ID, uniqueEmail("cross-org-2"), "password", models.RoleAdmin, true)

	account1 := createTestWhatsAppAccount(t, app, org1.ID, "cross-org-account-1")
	template1 := createTestTemplate(t, app, org1.ID, account1.Name)
	campaign1 := createTestCampaign(t, app, org1.ID, template1.ID, user1.ID, account1.Name, models.CampaignStatusDraft)

	// User from org2 tries to access org1's campaign
	req := testutil.NewGETRequest(t)
	setAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", campaign1.ID.String())

	err := app.GetCampaign(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}
