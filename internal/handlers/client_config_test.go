package handlers_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestGetEmbeddedSignupConfig(t *testing.T) {
	t.Parallel()

	// Setup test app with mock config
	app := &handlers.App{
		Config: &config.Config{
			WhatsApp: config.WhatsAppConfig{
				AppID:      "test-app-id-123",
				ConfigID:   "test-config-id-456",
				APIVersion: "v21.0",
			},
		},
	}

	// Create test request
	req := testutil.NewGETRequest(t)

	// Call handler
	err := app.GetEmbeddedSignupConfig(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Parse response body
	var resp struct {
		Data struct {
			WhatsAppAppID      string `json:"whatsapp_app_id"`
			WhatsAppConfigID   string `json:"whatsapp_config_id"`
			WhatsAppAPIVersion string `json:"whatsapp_api_version"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "test-app-id-123", resp.Data.WhatsAppAppID)
	assert.Equal(t, "test-config-id-456", resp.Data.WhatsAppConfigID)
	assert.Equal(t, "v21.0", resp.Data.WhatsAppAPIVersion)
}

func TestGetEmbeddedSignupConfig_EmptyValues(t *testing.T) {
	t.Parallel()

	// Setup test app with empty config
	app := &handlers.App{
		Config: &config.Config{
			WhatsApp: config.WhatsAppConfig{
				AppID:    "",
				ConfigID: "",
			},
		},
	}

	// Create test request
	req := testutil.NewGETRequest(t)

	// Call handler
	err := app.GetEmbeddedSignupConfig(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	body := string(testutil.GetResponseBody(req))
	// Verify that empty fields are omitted from JSON due to omitempty
	assert.False(t, strings.Contains(body, "whatsapp_app_id"))
	assert.False(t, strings.Contains(body, "whatsapp_config_id"))

	// Parse response body - should still default to empty strings in the struct
	var resp struct {
		Data struct {
			WhatsAppAppID      string `json:"whatsapp_app_id"`
			WhatsAppConfigID   string `json:"whatsapp_config_id"`
			WhatsAppAPIVersion string `json:"whatsapp_api_version"`
		} `json:"data"`
	}
	err = json.Unmarshal([]byte(body), &resp)
	require.NoError(t, err)
	assert.Equal(t, "", resp.Data.WhatsAppAppID)
	assert.Equal(t, "", resp.Data.WhatsAppConfigID)
}
