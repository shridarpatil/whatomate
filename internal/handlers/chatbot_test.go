package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// chatbotTestApp creates an App instance for chatbot testing.
func chatbotTestApp(t *testing.T) *handlers.App {
	t.Helper()

	db := testutil.SetupTestDB(t)
	redis := testutil.SetupTestRedis(t)
	log := testutil.NopLogger()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:            "test-secret-key-must-be-at-least-32-chars",
			AccessExpiryMins:  15,
			RefreshExpiryDays: 7,
		},
	}

	return &handlers.App{
		Config: cfg,
		DB:     db,
		Redis:  redis,
		Log:    log,
	}
}

// createChatbotTestOrg creates a test organization for chatbot tests.
func createChatbotTestOrg(t *testing.T, app *handlers.App) *models.Organization {
	t.Helper()

	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test Org " + uuid.New().String()[:8],
		Slug:      "test-org-" + uuid.New().String()[:8],
	}
	require.NoError(t, app.DB.Create(org).Error)
	return org
}

func TestUpdateChatbotSettings_RasaProvider_SetsDefaultAPIKey(t *testing.T) {
	app := chatbotTestApp(t)
	org := createChatbotTestOrg(t, app)

	// Create request with RASA provider but no API key
	reqBody := map[string]interface{}{
		"ai_enabled":    true,
		"ai_provider":   "rasa",
		"ai_server_url": "http://localhost:5005/webhooks/rest/webhook",
	}
	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody(jsonBody)
	ctx.SetUserValue("organization_id", org.ID)

	req := &fastglue.Request{RequestCtx: ctx}

	// Call handler
	err = app.UpdateChatbotSettings(req)
	require.NoError(t, err)

	// Verify settings were saved with NO-KEY
	var settings models.ChatbotSettings
	err = app.DB.Where("organization_id = ?", org.ID).First(&settings).Error
	require.NoError(t, err)

	assert.Equal(t, models.AIProviderRasa, settings.AI.Provider)
	assert.Equal(t, "NO-KEY", settings.AI.APIKey, "RASA provider should have NO-KEY as default API key")
	assert.Equal(t, "http://localhost:5005/webhooks/rest/webhook", settings.AI.ServerURL)
	assert.True(t, settings.AI.Enabled)
}

func TestUpdateChatbotSettings_RasaProvider_PreservesExplicitAPIKey(t *testing.T) {
	app := chatbotTestApp(t)
	org := createChatbotTestOrg(t, app)

	// Create request with RASA provider AND explicit API key
	reqBody := map[string]interface{}{
		"ai_enabled":    true,
		"ai_provider":   "rasa",
		"ai_api_key":    "my-custom-rasa-token",
		"ai_server_url": "http://localhost:5005/webhooks/rest/webhook",
	}
	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody(jsonBody)
	ctx.SetUserValue("organization_id", org.ID)

	req := &fastglue.Request{RequestCtx: ctx}

	err = app.UpdateChatbotSettings(req)
	require.NoError(t, err)

	// Verify explicit API key was preserved
	var settings models.ChatbotSettings
	err = app.DB.Where("organization_id = ?", org.ID).First(&settings).Error
	require.NoError(t, err)

	assert.Equal(t, models.AIProviderRasa, settings.AI.Provider)
	assert.Equal(t, "my-custom-rasa-token", settings.AI.APIKey, "Explicit API key should be preserved")
}
