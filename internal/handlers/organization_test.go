package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/calling"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// --- GetOrganizationSettings Tests ---

func TestApp_GetOrganizationSettings_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-settings")))

	// Set organization settings
	org.Settings = models.JSONB{
		"mask_phone_numbers": true,
		"timezone":           "Asia/Kolkata",
		"date_format":        "DD/MM/YYYY",
	}
	require.NoError(t, app.DB.Save(org).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Settings handlers.OrganizationSettings `json:"settings"`
			Name     string                        `json:"name"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, true, resp.Data.Settings.MaskPhoneNumbers)
	assert.Equal(t, "Asia/Kolkata", resp.Data.Settings.Timezone)
	assert.Equal(t, "DD/MM/YYYY", resp.Data.Settings.DateFormat)
	assert.Equal(t, org.Name, resp.Data.Name)
}

func TestApp_GetOrganizationSettings_Defaults(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-settings-defaults")))

	// Organization with nil settings should return defaults
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Settings handlers.OrganizationSettings `json:"settings"`
			Name     string                        `json:"name"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, false, resp.Data.Settings.MaskPhoneNumbers)
	assert.Equal(t, "UTC", resp.Data.Settings.Timezone)
	assert.Equal(t, "YYYY-MM-DD", resp.Data.Settings.DateFormat)
}

func TestApp_GetOrganizationSettings_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context set

	err := app.GetOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- UpdateOrganizationSettings Tests ---

func TestApp_UpdateOrganizationSettings_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-settings")))

	maskEnabled := true
	timezone := "America/New_York"
	dateFormat := "MM/DD/YYYY"
	newName := "Updated Organization"

	req := testutil.NewJSONRequest(t, map[string]any{
		"mask_phone_numbers": maskEnabled,
		"timezone":           timezone,
		"date_format":        dateFormat,
		"name":               newName,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.UpdateOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Settings updated successfully", resp.Data.Message)

	// Verify the settings were actually persisted
	var updatedOrg models.Organization
	require.NoError(t, app.DB.Where("id = ?", org.ID).First(&updatedOrg).Error)

	assert.Equal(t, newName, updatedOrg.Name)
	assert.Equal(t, true, updatedOrg.Settings["mask_phone_numbers"])
	assert.Equal(t, "America/New_York", updatedOrg.Settings["timezone"])
	assert.Equal(t, "MM/DD/YYYY", updatedOrg.Settings["date_format"])
}

func TestApp_UpdateOrganizationSettings_PartialUpdate(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("partial-update")))

	// Set initial settings
	org.Settings = models.JSONB{
		"mask_phone_numbers": false,
		"timezone":           "UTC",
		"date_format":        "YYYY-MM-DD",
	}
	require.NoError(t, app.DB.Save(org).Error)
	originalName := org.Name

	// Only update timezone (partial update)
	req := testutil.NewJSONRequest(t, map[string]any{
		"timezone": "Europe/London",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.UpdateOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify only timezone changed, other fields remain the same
	var updatedOrg models.Organization
	require.NoError(t, app.DB.Where("id = ?", org.ID).First(&updatedOrg).Error)

	assert.Equal(t, originalName, updatedOrg.Name)
	assert.Equal(t, false, updatedOrg.Settings["mask_phone_numbers"])
	assert.Equal(t, "Europe/London", updatedOrg.Settings["timezone"])
	assert.Equal(t, "YYYY-MM-DD", updatedOrg.Settings["date_format"])
}

func TestApp_UpdateOrganizationSettings_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"timezone": "UTC",
	})
	// No auth context set

	err := app.UpdateOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateOrganizationSettings_EmptyNameIgnored(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("empty-name")))
	originalName := org.Name

	// Send an empty name -- should be ignored
	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.UpdateOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify name was not changed
	var updatedOrg models.Organization
	require.NoError(t, app.DB.Where("id = ?", org.ID).First(&updatedOrg).Error)
	assert.Equal(t, originalName, updatedOrg.Name)
}

func TestApp_UpdateOrganizationSettings_InvalidJSON(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("invalid-json")))

	// Create a request with invalid JSON body
	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.SetBody([]byte(`{invalid json`))
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.UpdateOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- GetCurrentOrganization Tests ---

func TestApp_GetCurrentOrganization_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-current-org")))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetCurrentOrganization(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.OrganizationResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, org.ID, resp.Data.ID)
	assert.Equal(t, org.Name, resp.Data.Name)
	assert.Equal(t, org.Slug, resp.Data.Slug)
	assert.NotEmpty(t, resp.Data.CreatedAt)
}

func TestApp_GetCurrentOrganization_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context set

	err := app.GetCurrentOrganization(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

func TestApp_GetCurrentOrganization_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-org-404")))

	// Set auth context with a non-existent organization ID
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, uuid.New(), user.ID)

	err := app.GetCurrentOrganization(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- Calling Config in Organization Settings Tests ---

func TestApp_GetOrganizationSettings_CallingDefaults(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	// Set global calling config defaults
	app.Config.Calling.MaxCallDuration = 3600
	app.Config.Calling.TransferTimeoutSecs = 60

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("calling-defaults")))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Settings handlers.OrganizationSettings `json:"settings"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	// Calling should be disabled by default, with global config fallbacks
	assert.Equal(t, false, resp.Data.Settings.CallingEnabled)
	assert.Equal(t, 3600, resp.Data.Settings.MaxCallDuration)
	assert.Equal(t, 60, resp.Data.Settings.TransferTimeoutSecs)
}

func TestApp_GetOrganizationSettings_CallingOverrides(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.Config.Calling.MaxCallDuration = 3600
	app.Config.Calling.TransferTimeoutSecs = 60

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("calling-overrides")))

	// Set org-level calling overrides
	org.Settings = models.JSONB{
		"calling_enabled":       true,
		"max_call_duration":     float64(1800),
		"transfer_timeout_secs": float64(90),
	}
	require.NoError(t, app.DB.Save(org).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Settings handlers.OrganizationSettings `json:"settings"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, true, resp.Data.Settings.CallingEnabled)
	assert.Equal(t, 1800, resp.Data.Settings.MaxCallDuration)
	assert.Equal(t, 90, resp.Data.Settings.TransferTimeoutSecs)
}

func TestApp_UpdateOrganizationSettings_CallingFields(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-calling")))

	// Enable calling with custom values
	req := testutil.NewJSONRequest(t, map[string]any{
		"calling_enabled":       true,
		"max_call_duration":     1800,
		"transfer_timeout_secs": 90,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.UpdateOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify persisted in DB
	var updatedOrg models.Organization
	require.NoError(t, app.DB.Where("id = ?", org.ID).First(&updatedOrg).Error)

	assert.Equal(t, true, updatedOrg.Settings["calling_enabled"])
	// JSON numbers are float64
	assert.Equal(t, float64(1800), updatedOrg.Settings["max_call_duration"])
	assert.Equal(t, float64(90), updatedOrg.Settings["transfer_timeout_secs"])
}

func TestApp_UpdateOrganizationSettings_CallingPartialUpdate(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("calling-partial")))

	// Set initial calling settings
	org.Settings = models.JSONB{
		"calling_enabled":       true,
		"max_call_duration":     float64(3600),
		"transfer_timeout_secs": float64(60),
	}
	require.NoError(t, app.DB.Save(org).Error)

	// Only update transfer timeout
	req := testutil.NewJSONRequest(t, map[string]any{
		"transfer_timeout_secs": 120,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.UpdateOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify only transfer_timeout_secs changed
	var updatedOrg models.Organization
	require.NoError(t, app.DB.Where("id = ?", org.ID).First(&updatedOrg).Error)

	assert.Equal(t, true, updatedOrg.Settings["calling_enabled"])
	assert.Equal(t, float64(3600), updatedOrg.Settings["max_call_duration"])
	assert.Equal(t, float64(120), updatedOrg.Settings["transfer_timeout_secs"])
}

func TestApp_UpdateOrganizationSettings_CallingDisable(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("calling-disable")))

	// Start with calling enabled
	org.Settings = models.JSONB{
		"calling_enabled": true,
	}
	require.NoError(t, app.DB.Save(org).Error)

	// Disable calling
	req := testutil.NewJSONRequest(t, map[string]any{
		"calling_enabled": false,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.UpdateOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var updatedOrg models.Organization
	require.NoError(t, app.DB.Where("id = ?", org.ID).First(&updatedOrg).Error)
	assert.Equal(t, false, updatedOrg.Settings["calling_enabled"])
}

func TestApp_UpdateOrganizationSettings_CallingZeroDurationIgnored(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("calling-zero")))

	// Set initial calling settings
	org.Settings = models.JSONB{
		"max_call_duration": float64(3600),
	}
	require.NoError(t, app.DB.Save(org).Error)

	// Send zero/negative values — should be ignored (guard: > 0)
	req := testutil.NewJSONRequest(t, map[string]any{
		"max_call_duration":     0,
		"transfer_timeout_secs": -1,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.UpdateOrganizationSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var updatedOrg models.Organization
	require.NoError(t, app.DB.Where("id = ?", org.ID).First(&updatedOrg).Error)

	// Original value should be preserved
	assert.Equal(t, float64(3600), updatedOrg.Settings["max_call_duration"])
	// transfer_timeout_secs should not have been set
	_, exists := updatedOrg.Settings["transfer_timeout_secs"]
	assert.False(t, exists)
}

// --- IsCallingEnabledForOrg Tests ---

func TestApp_IsCallingEnabledForOrg_NoCallManager(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.CallManager = nil // no infrastructure

	org := testutil.CreateTestOrganization(t, app.DB)
	org.Settings = models.JSONB{"calling_enabled": true}
	require.NoError(t, app.DB.Save(org).Error)

	// Even with org setting enabled, returns false without CallManager
	assert.False(t, app.IsCallingEnabledForOrg(org.ID))
}

func TestApp_IsCallingEnabledForOrg_DisabledByDefault(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	// Simulate CallManager being set (non-nil pointer)
	app.CallManager = &calling.Manager{}

	org := testutil.CreateTestOrganization(t, app.DB)
	// No settings — calling_enabled defaults to false

	assert.False(t, app.IsCallingEnabledForOrg(org.ID))
}

func TestApp_IsCallingEnabledForOrg_Enabled(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.CallManager = &calling.Manager{}

	org := testutil.CreateTestOrganization(t, app.DB)
	org.Settings = models.JSONB{"calling_enabled": true}
	require.NoError(t, app.DB.Save(org).Error)

	assert.True(t, app.IsCallingEnabledForOrg(org.ID))
}

func TestApp_IsCallingEnabledForOrg_ExplicitlyDisabled(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.CallManager = &calling.Manager{}

	org := testutil.CreateTestOrganization(t, app.DB)
	org.Settings = models.JSONB{"calling_enabled": false}
	require.NoError(t, app.DB.Save(org).Error)

	assert.False(t, app.IsCallingEnabledForOrg(org.ID))
}

func TestApp_IsCallingEnabledForOrg_NonExistentOrg(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.CallManager = &calling.Manager{}

	assert.False(t, app.IsCallingEnabledForOrg(uuid.New()))
}

func TestApp_IsCallingEnabledForOrg_OrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.CallManager = &calling.Manager{}

	org1 := testutil.CreateTestOrganization(t, app.DB)
	org1.Settings = models.JSONB{"calling_enabled": true}
	require.NoError(t, app.DB.Save(org1).Error)

	org2 := testutil.CreateTestOrganization(t, app.DB)
	// org2 has no calling_enabled setting

	assert.True(t, app.IsCallingEnabledForOrg(org1.ID))
	assert.False(t, app.IsCallingEnabledForOrg(org2.ID))
}

// --- GetOrgCallingConfig Tests ---

func TestApp_GetOrgCallingConfig_GlobalDefaults(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.Config.Calling.MaxCallDuration = 3600
	app.Config.Calling.TransferTimeoutSecs = 60

	org := testutil.CreateTestOrganization(t, app.DB)
	// No org-level overrides

	maxDuration, transferTimeout := app.GetOrgCallingConfig(org.ID)
	assert.Equal(t, 3600, maxDuration)
	assert.Equal(t, 60, transferTimeout)
}

func TestApp_GetOrgCallingConfig_OrgOverrides(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.Config.Calling.MaxCallDuration = 3600
	app.Config.Calling.TransferTimeoutSecs = 60

	org := testutil.CreateTestOrganization(t, app.DB)
	org.Settings = models.JSONB{
		"max_call_duration":     float64(1800),
		"transfer_timeout_secs": float64(90),
	}
	require.NoError(t, app.DB.Save(org).Error)

	maxDuration, transferTimeout := app.GetOrgCallingConfig(org.ID)
	assert.Equal(t, 1800, maxDuration)
	assert.Equal(t, 90, transferTimeout)
}

func TestApp_GetOrgCallingConfig_PartialOverride(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.Config.Calling.MaxCallDuration = 3600
	app.Config.Calling.TransferTimeoutSecs = 60

	org := testutil.CreateTestOrganization(t, app.DB)
	// Only override max_call_duration
	org.Settings = models.JSONB{
		"max_call_duration": float64(900),
	}
	require.NoError(t, app.DB.Save(org).Error)

	maxDuration, transferTimeout := app.GetOrgCallingConfig(org.ID)
	assert.Equal(t, 900, maxDuration)
	assert.Equal(t, 60, transferTimeout) // falls back to global
}

func TestApp_GetOrgCallingConfig_NonExistentOrg(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	app.Config.Calling.MaxCallDuration = 3600
	app.Config.Calling.TransferTimeoutSecs = 60

	// Non-existent org should return global defaults
	maxDuration, transferTimeout := app.GetOrgCallingConfig(uuid.New())
	assert.Equal(t, 3600, maxDuration)
	assert.Equal(t, 60, transferTimeout)
}
