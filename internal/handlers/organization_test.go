package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
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
