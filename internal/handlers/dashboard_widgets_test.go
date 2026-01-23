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
)

// widgetTestApp creates an App instance for widget testing.
func widgetTestApp(t *testing.T) *handlers.App {
	t.Helper()

	db := testutil.SetupTestDB(t)
	log := testutil.NopLogger()

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
	}
}

// getOrCreateAnalyticsPermissions gets existing analytics permissions or creates them.
func getOrCreateAnalyticsPermissions(t *testing.T, app *handlers.App) []models.Permission {
	t.Helper()

	// First try to get existing analytics permissions
	var existingPerms []models.Permission
	if err := app.DB.Where("resource = ?", "analytics").Order("action").Find(&existingPerms).Error; err == nil && len(existingPerms) >= 3 {
		return existingPerms
	}

	// Create analytics permissions if they don't exist
	permissions := []models.Permission{
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "analytics", Action: "read", Description: "View analytics dashboard"},
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "analytics", Action: "write", Description: "Create and edit dashboard widgets"},
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "analytics", Action: "delete", Description: "Delete dashboard widgets"},
	}

	for i := range permissions {
		// Check if permission already exists
		var existing models.Permission
		if err := app.DB.Where("resource = ? AND action = ?", permissions[i].Resource, permissions[i].Action).First(&existing).Error; err == nil {
			permissions[i] = existing
		} else {
			require.NoError(t, app.DB.Create(&permissions[i]).Error)
		}
	}

	return permissions
}

// createAnalyticsRole creates a role with analytics permissions.
func createAnalyticsRole(t *testing.T, app *handlers.App, orgID uuid.UUID, name string, permissions []models.Permission) *models.CustomRole {
	t.Helper()

	role := &models.CustomRole{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           name,
		Description:    "Role with analytics permissions",
		IsSystem:       false,
		IsDefault:      false,
		Permissions:    permissions,
	}
	require.NoError(t, app.DB.Create(role).Error)
	return role
}

// createTestWidget creates a test dashboard widget in the database.
func createTestWidget(t *testing.T, app *handlers.App, orgID uuid.UUID, userID *uuid.UUID, name string, isShared, isDefault bool) *models.DashboardWidget {
	t.Helper()

	widget := &models.DashboardWidget{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		UserID:         userID,
		Name:           name,
		Description:    "Test widget description",
		DataSource:     "messages",
		Metric:         "count",
		DisplayType:    "number",
		ShowChange:     true,
		Color:          "blue",
		Size:           "small",
		DisplayOrder:   1,
		IsShared:       isShared,
		IsDefault:      isDefault,
	}
	require.NoError(t, app.DB.Create(widget).Error)
	return widget
}

// --- ListDashboardWidgets Tests ---

func TestApp_ListDashboardWidgets_Success(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("list-widgets"), "password", &role.ID, true)

	// Create multiple widgets
	createTestWidget(t, app, org.ID, &user.ID, "Widget 1", true, false)
	createTestWidget(t, app, org.ID, &user.ID, "Widget 2", true, false)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)

	err := app.ListDashboardWidgets(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Widgets []handlers.WidgetResponse `json:"widgets"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Widgets, 2)
}

func TestApp_ListDashboardWidgets_NoPermission(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	// User without analytics permission
	user := createTestUser(t, app, org.ID, uniqueEmail("list-no-perm"), "password", nil, true)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)

	err := app.ListDashboardWidgets(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_ListDashboardWidgets_FiltersByOrganization(t *testing.T) {
	app := widgetTestApp(t)

	// Create two organizations
	org1 := createTestOrganization(t, app)
	org2 := createTestOrganization(t, app)

	perms := getOrCreateAnalyticsPermissions(t, app)
	role1 := createAnalyticsRole(t, app, org1.ID, "Analytics User 1", perms)
	role2 := createAnalyticsRole(t, app, org2.ID, "Analytics User 2", perms)

	user1 := createTestUser(t, app, org1.ID, uniqueEmail("list-org1"), "password", &role1.ID, true)
	user2 := createTestUser(t, app, org2.ID, uniqueEmail("list-org2"), "password", &role2.ID, true)

	// Create widgets for each org
	createTestWidget(t, app, org1.ID, &user1.ID, "Org1 Widget", true, false)
	createTestWidget(t, app, org2.ID, &user2.ID, "Org2 Widget", true, false)

	// User from org1 should only see org1's widgets
	req := testutil.NewGETRequest(t)
	setAuthContext(req, org1.ID, user1.ID)

	err := app.ListDashboardWidgets(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Widgets []handlers.WidgetResponse `json:"widgets"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Widgets, 1)
	assert.Equal(t, "Org1 Widget", resp.Data.Widgets[0].Name)
}

func TestApp_ListDashboardWidgets_Unauthorized(t *testing.T) {
	app := widgetTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context set

	err := app.ListDashboardWidgets(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- GetDashboardWidget Tests ---

func TestApp_GetDashboardWidget_Success(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("get-widget"), "password", &role.ID, true)
	widget := createTestWidget(t, app, org.ID, &user.ID, "Test Widget", true, false)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.GetDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.WidgetResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, widget.ID, resp.Data.ID)
	assert.Equal(t, "Test Widget", resp.Data.Name)
}

func TestApp_GetDashboardWidget_NoPermission(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	owner := createTestUser(t, app, org.ID, uniqueEmail("owner-get"), "password", &role.ID, true)
	// User without analytics permission
	otherUser := createTestUser(t, app, org.ID, uniqueEmail("no-perm-get"), "password", nil, true)

	widget := createTestWidget(t, app, org.ID, &owner.ID, "Test Widget", true, false)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.GetDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_GetDashboardWidget_NotFound(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("get-not-found"), "password", &role.ID, true)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetDashboardWidget_InvalidID(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("get-invalid-id"), "password", &role.ID, true)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- CreateDashboardWidget Tests ---

func TestApp_CreateDashboardWidget_Success(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-widget"), "password", &role.ID, true)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "New Widget",
		"description": "A test widget",
		"data_source": "messages",
		"metric":      "count",
		"color":       "green",
	})
	setAuthContext(req, org.ID, user.ID)

	err := app.CreateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.WidgetResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "New Widget", resp.Data.Name)
	assert.Equal(t, "messages", resp.Data.DataSource)
	assert.Equal(t, "green", resp.Data.Color)
}

func TestApp_CreateDashboardWidget_NoPermission(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	// User without analytics write permission (only read)
	readOnlyPerms := getOrCreateAnalyticsPermissions(t, app)
	readOnlyRole := createAnalyticsRole(t, app, org.ID, "Read Only", readOnlyPerms[:1]) // Only read permission
	user := createTestUser(t, app, org.ID, uniqueEmail("create-no-perm"), "password", &readOnlyRole.ID, true)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "New Widget",
		"data_source": "messages",
		"metric":      "count",
	})
	setAuthContext(req, org.ID, user.ID)

	err := app.CreateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateDashboardWidget_WithFilters(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-with-filters"), "password", &role.ID, true)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "Filtered Widget",
		"data_source": "messages",
		"metric":      "count",
		"filters": []map[string]any{
			{
				"field":    "direction",
				"operator": "equals",
				"value":    "inbound",
			},
		},
	})
	setAuthContext(req, org.ID, user.ID)

	err := app.CreateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.WidgetResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Filters, 1)
}

func TestApp_CreateDashboardWidget_InvalidDataSource(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-invalid-source"), "password", &role.ID, true)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "Invalid Widget",
		"data_source": "invalid_source",
		"metric":      "count",
	})
	setAuthContext(req, org.ID, user.ID)

	err := app.CreateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateDashboardWidget_MissingName(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-missing-name"), "password", &role.ID, true)

	req := testutil.NewJSONRequest(t, map[string]any{
		"data_source": "messages",
		"metric":      "count",
	})
	setAuthContext(req, org.ID, user.ID)

	err := app.CreateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateDashboardWidget_Unauthorized(t *testing.T) {
	app := widgetTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "Widget",
		"data_source": "messages",
		"metric":      "count",
	})
	// No auth context

	err := app.CreateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- UpdateDashboardWidget Tests ---

func TestApp_UpdateDashboardWidget_Success(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("update-widget"), "password", &role.ID, true)
	widget := createTestWidget(t, app, org.ID, &user.ID, "Original Name", true, false)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "Updated Name",
		"description": "Updated description",
		"color":       "red",
	})
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.UpdateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.WidgetResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", resp.Data.Name)
	assert.Equal(t, "red", resp.Data.Color)
}

func TestApp_UpdateDashboardWidget_NoPermission(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	owner := createTestUser(t, app, org.ID, uniqueEmail("owner-update"), "password", &role.ID, true)
	// User without analytics write permission
	readOnlyRole := createAnalyticsRole(t, app, org.ID, "Read Only", perms[:1])
	otherUser := createTestUser(t, app, org.ID, uniqueEmail("no-perm-update"), "password", &readOnlyRole.ID, true)

	widget := createTestWidget(t, app, org.ID, &owner.ID, "Test Widget", true, false)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Updated Name",
	})
	setAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.UpdateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateDashboardWidget_OnlyOwnerCanEdit(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	owner := createTestUser(t, app, org.ID, uniqueEmail("owner-only"), "password", &role.ID, true)
	otherUser := createTestUser(t, app, org.ID, uniqueEmail("other-user"), "password", &role.ID, true)

	// Create widget owned by 'owner'
	widget := createTestWidget(t, app, org.ID, &owner.ID, "Owner Widget", true, false)

	// Other user (with write permission) should NOT be able to edit
	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Attempted Update",
	})
	setAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.UpdateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateDashboardWidget_NotFound(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("update-not-found"), "password", &role.ID, true)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Updated",
	})
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- DeleteDashboardWidget Tests ---

func TestApp_DeleteDashboardWidget_Success(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("delete-widget"), "password", &role.ID, true)
	widget := createTestWidget(t, app, org.ID, &user.ID, "To Delete", true, false)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.DeleteDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify widget is deleted
	var count int64
	app.DB.Model(&models.DashboardWidget{}).Where("id = ?", widget.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteDashboardWidget_NoPermission(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	owner := createTestUser(t, app, org.ID, uniqueEmail("owner-del"), "password", &role.ID, true)
	// User without analytics delete permission (only read and write)
	limitedRole := createAnalyticsRole(t, app, org.ID, "Limited", perms[:2])
	otherUser := createTestUser(t, app, org.ID, uniqueEmail("no-del-perm"), "password", &limitedRole.ID, true)

	widget := createTestWidget(t, app, org.ID, &owner.ID, "Test Widget", true, false)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.DeleteDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteDashboardWidget_OnlyOwnerCanDelete(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	owner := createTestUser(t, app, org.ID, uniqueEmail("owner-del-only"), "password", &role.ID, true)
	otherUser := createTestUser(t, app, org.ID, uniqueEmail("other-del-only"), "password", &role.ID, true)

	// Create widget owned by 'owner'
	widget := createTestWidget(t, app, org.ID, &owner.ID, "Owner Widget", true, false)

	// Other user (with delete permission) should NOT be able to delete someone else's widget
	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.DeleteDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))

	// Widget should still exist
	var count int64
	app.DB.Model(&models.DashboardWidget{}).Where("id = ?", widget.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestApp_DeleteDashboardWidget_NotFound(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("delete-not-found"), "password", &role.ID, true)

	req := testutil.NewGETRequest(t)
	setAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- ReorderDashboardWidgets Tests ---

func TestApp_ReorderDashboardWidgets_Success(t *testing.T) {
	app := widgetTestApp(t)
	org := createTestOrganization(t, app)
	perms := getOrCreateAnalyticsPermissions(t, app)
	role := createAnalyticsRole(t, app, org.ID, "Analytics User", perms)
	user := createTestUser(t, app, org.ID, uniqueEmail("reorder-widgets"), "password", &role.ID, true)

	widget1 := createTestWidget(t, app, org.ID, &user.ID, "Widget 1", true, false)
	widget2 := createTestWidget(t, app, org.ID, &user.ID, "Widget 2", true, false)
	widget3 := createTestWidget(t, app, org.ID, &user.ID, "Widget 3", true, false)

	// Reorder: widget3 first, widget1 second, widget2 third
	req := testutil.NewJSONRequest(t, map[string]any{
		"widget_ids": []string{widget3.ID.String(), widget1.ID.String(), widget2.ID.String()},
	})
	setAuthContext(req, org.ID, user.ID)

	err := app.ReorderDashboardWidgets(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify order
	var widgets []models.DashboardWidget
	app.DB.Where("organization_id = ?", org.ID).Order("display_order").Find(&widgets)
	assert.Equal(t, widget3.ID, widgets[0].ID)
	assert.Equal(t, widget1.ID, widgets[1].ID)
	assert.Equal(t, widget2.ID, widgets[2].ID)
}

func TestApp_ReorderDashboardWidgets_Unauthorized(t *testing.T) {
	app := widgetTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"widget_ids": []string{uuid.New().String()},
	})
	// No auth context

	err := app.ReorderDashboardWidgets(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- Cross-Organization Isolation Tests ---

func TestApp_DashboardWidget_CrossOrgIsolation(t *testing.T) {
	app := widgetTestApp(t)

	org1 := createTestOrganization(t, app)
	org2 := createTestOrganization(t, app)

	perms := getOrCreateAnalyticsPermissions(t, app)
	role1 := createAnalyticsRole(t, app, org1.ID, "Analytics User 1", perms)
	role2 := createAnalyticsRole(t, app, org2.ID, "Analytics User 2", perms)

	user1 := createTestUser(t, app, org1.ID, uniqueEmail("cross-widget-1"), "password", &role1.ID, true)
	user2 := createTestUser(t, app, org2.ID, uniqueEmail("cross-widget-2"), "password", &role2.ID, true)

	// Create widget in org1
	widget1 := createTestWidget(t, app, org1.ID, &user1.ID, "Org1 Widget", true, false)

	// User from org2 tries to access org1's widget
	req := testutil.NewGETRequest(t)
	setAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", widget1.ID.String())

	err := app.GetDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_DashboardWidget_CrossOrg_CannotDelete(t *testing.T) {
	app := widgetTestApp(t)

	org1 := createTestOrganization(t, app)
	org2 := createTestOrganization(t, app)

	perms := getOrCreateAnalyticsPermissions(t, app)
	role1 := createAnalyticsRole(t, app, org1.ID, "Analytics User 1", perms)
	role2 := createAnalyticsRole(t, app, org2.ID, "Analytics User 2", perms)

	user1 := createTestUser(t, app, org1.ID, uniqueEmail("cross-del-1"), "password", &role1.ID, true)
	user2 := createTestUser(t, app, org2.ID, uniqueEmail("cross-del-2"), "password", &role2.ID, true)

	// Create widget in org1
	widget1 := createTestWidget(t, app, org1.ID, &user1.ID, "Org1 Widget", true, false)

	// User from org2 tries to delete org1's widget
	req := testutil.NewGETRequest(t)
	setAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", widget1.ID.String())

	err := app.DeleteDashboardWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	// Widget should still exist
	var count int64
	app.DB.Model(&models.DashboardWidget{}).Where("id = ?", widget1.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}
