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

// getAnalyticsPermissions returns analytics permissions from the full permission set.
func getAnalyticsPermissions(t *testing.T, app *handlers.App) []models.Permission {
	t.Helper()

	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)

	var analyticsPerms []models.Permission
	for _, p := range allPerms {
		if p.Resource == "analytics" {
			analyticsPerms = append(analyticsPerms, p)
		}
	}
	require.NotEmpty(t, analyticsPerms, "expected analytics permissions in default set")
	return analyticsPerms
}

// createTestWidget creates a test dashboard widget in the database.
func createTestWidget(t *testing.T, app *handlers.App, orgID uuid.UUID, userID *uuid.UUID, name string, isShared, isDefault bool) *models.Widget {
	t.Helper()

	widget := &models.Widget{
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

func TestApp_ListWidgets_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-widgets")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	// Create multiple widgets
	createTestWidget(t, app, org.ID, &user.ID, "Widget 1", true, false)
	createTestWidget(t, app, org.ID, &user.ID, "Widget 2", true, false)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListWidgets(req)
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

func TestApp_ListWidgets_NoPermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// User without analytics permission
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-no-perm")), testutil.WithPassword("password"))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListWidgets(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_ListWidgets_FiltersByOrganization(t *testing.T) {
	app := newTestApp(t)

	// Create two organizations
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)

	perms := getAnalyticsPermissions(t, app)
	role1 := testutil.CreateTestRoleExact(t, app.DB, org1.ID, "Analytics User 1", false, false, perms)
	role2 := testutil.CreateTestRoleExact(t, app.DB, org2.ID, "Analytics User 2", false, false, perms)

	user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("list-org1")), testutil.WithPassword("password"), testutil.WithRoleID(&role1.ID))
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("list-org2")), testutil.WithPassword("password"), testutil.WithRoleID(&role2.ID))

	// Create widgets for each org
	createTestWidget(t, app, org1.ID, &user1.ID, "Org1 Widget", true, false)
	createTestWidget(t, app, org2.ID, &user2.ID, "Org2 Widget", true, false)

	// User from org1 should only see org1's widgets
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org1.ID, user1.ID)

	err := app.ListWidgets(req)
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

func TestApp_ListWidgets_Unauthorized(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context set

	err := app.ListWidgets(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- GetDashboardWidget Tests ---

func TestApp_GetWidget_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-widget")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))
	widget := createTestWidget(t, app, org.ID, &user.ID, "Test Widget", true, false)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.GetWidget(req)
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

func TestApp_GetWidget_NoPermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("owner-get")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))
	// User without analytics permission
	otherUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("no-perm-get")), testutil.WithPassword("password"))

	widget := createTestWidget(t, app, org.ID, &owner.ID, "Test Widget", true, false)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.GetWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_GetWidget_NotFound(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-not-found")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetWidget_InvalidID(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-invalid-id")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- CreateDashboardWidget Tests ---

func TestApp_CreateWidget_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-widget")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "New Widget",
		"description": "A test widget",
		"data_source": "messages",
		"metric":      "count",
		"color":       "green",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWidget(req)
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

func TestApp_CreateWidget_NoPermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// User without analytics write permission (only read)
	readOnlyPerms := getAnalyticsPermissions(t, app)
	readOnlyRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Read Only", false, false, readOnlyPerms[:1]) // Only read permission
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-no-perm")), testutil.WithPassword("password"), testutil.WithRoleID(&readOnlyRole.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "New Widget",
		"data_source": "messages",
		"metric":      "count",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateWidget_WithFilters(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-with-filters")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

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
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.WidgetResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Filters, 1)
}

func TestApp_CreateWidget_InvalidDataSource(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-invalid-source")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "Invalid Widget",
		"data_source": "invalid_source",
		"metric":      "count",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateWidget_MissingName(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-missing-name")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"data_source": "messages",
		"metric":      "count",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateWidget_Unauthorized(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "Widget",
		"data_source": "messages",
		"metric":      "count",
	})
	// No auth context

	err := app.CreateWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- UpdateDashboardWidget Tests ---

func TestApp_UpdateWidget_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-widget")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))
	widget := createTestWidget(t, app, org.ID, &user.ID, "Original Name", true, false)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "Updated Name",
		"description": "Updated description",
		"color":       "red",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.UpdateWidget(req)
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

func TestApp_UpdateWidget_NoPermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("owner-update")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))
	// User without analytics write permission
	readOnlyRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Read Only", false, false, perms[:1])
	otherUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("no-perm-update")), testutil.WithPassword("password"), testutil.WithRoleID(&readOnlyRole.ID))

	widget := createTestWidget(t, app, org.ID, &owner.ID, "Test Widget", true, false)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Updated Name",
	})
	testutil.SetAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.UpdateWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateWidget_OnlyOwnerCanEdit(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("owner-only")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))
	otherUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("other-user")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	// Create widget owned by 'owner'
	widget := createTestWidget(t, app, org.ID, &owner.ID, "Owner Widget", true, false)

	// Other user (with write permission) should NOT be able to edit
	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Attempted Update",
	})
	testutil.SetAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.UpdateWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateWidget_NotFound(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-not-found")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Updated",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- DeleteDashboardWidget Tests ---

func TestApp_DeleteWidget_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-widget")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))
	widget := createTestWidget(t, app, org.ID, &user.ID, "To Delete", true, false)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.DeleteWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify widget is deleted
	var count int64
	app.DB.Model(&models.Widget{}).Where("id = ?", widget.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteWidget_NoPermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("owner-del")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))
	// User without analytics delete permission (only read and write)
	limitedRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Limited", false, false, perms[:2])
	otherUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("no-del-perm")), testutil.WithPassword("password"), testutil.WithRoleID(&limitedRole.ID))

	widget := createTestWidget(t, app, org.ID, &owner.ID, "Test Widget", true, false)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.DeleteWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteWidget_OnlyOwnerCanDelete(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("owner-del-only")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))
	otherUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("other-del-only")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	// Create widget owned by 'owner'
	widget := createTestWidget(t, app, org.ID, &owner.ID, "Owner Widget", true, false)

	// Other user (with delete permission) should NOT be able to delete someone else's widget
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, otherUser.ID)
	testutil.SetPathParam(req, "id", widget.ID.String())

	err := app.DeleteWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))

	// Widget should still exist
	var count int64
	app.DB.Model(&models.Widget{}).Where("id = ?", widget.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestApp_DeleteWidget_NotFound(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics User", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-not-found")), testutil.WithPassword("password"), testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- Cross-Organization Isolation Tests ---

func TestApp_Widget_CrossOrgIsolation(t *testing.T) {
	app := newTestApp(t)

	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)

	perms := getAnalyticsPermissions(t, app)
	role1 := testutil.CreateTestRoleExact(t, app.DB, org1.ID, "Analytics User 1", false, false, perms)
	role2 := testutil.CreateTestRoleExact(t, app.DB, org2.ID, "Analytics User 2", false, false, perms)

	user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("cross-widget-1")), testutil.WithPassword("password"), testutil.WithRoleID(&role1.ID))
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("cross-widget-2")), testutil.WithPassword("password"), testutil.WithRoleID(&role2.ID))

	// Create widget in org1
	widget1 := createTestWidget(t, app, org1.ID, &user1.ID, "Org1 Widget", true, false)

	// User from org2 tries to access org1's widget
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", widget1.ID.String())

	err := app.GetWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_Widget_CrossOrg_CannotDelete(t *testing.T) {
	app := newTestApp(t)

	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)

	perms := getAnalyticsPermissions(t, app)
	role1 := testutil.CreateTestRoleExact(t, app.DB, org1.ID, "Analytics User 1", false, false, perms)
	role2 := testutil.CreateTestRoleExact(t, app.DB, org2.ID, "Analytics User 2", false, false, perms)

	user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("cross-del-1")), testutil.WithPassword("password"), testutil.WithRoleID(&role1.ID))
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("cross-del-2")), testutil.WithPassword("password"), testutil.WithRoleID(&role2.ID))

	// Create widget in org1
	widget1 := createTestWidget(t, app, org1.ID, &user1.ID, "Org1 Widget", true, false)

	// User from org2 tries to delete org1's widget
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", widget1.ID.String())

	err := app.DeleteWidget(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	// Widget should still exist
	var count int64
	app.DB.Model(&models.Widget{}).Where("id = ?", widget1.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}
