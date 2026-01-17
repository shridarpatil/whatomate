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

// createTestRole creates a test role with the given permissions
func createTestRole(t *testing.T, app *handlers.App, orgID uuid.UUID, name string, isSystem, isDefault bool, permissions []models.Permission) *models.CustomRole {
	t.Helper()

	role := &models.CustomRole{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           name,
		Description:    "Test role: " + name,
		IsSystem:       isSystem,
		IsDefault:      isDefault,
		Permissions:    permissions,
	}
	require.NoError(t, app.DB.Create(role).Error)
	return role
}

// getOrCreateTestPermissions gets existing permissions or creates them if needed
func getOrCreateTestPermissions(t *testing.T, app *handlers.App) []models.Permission {
	t.Helper()

	// First try to get existing permissions
	var existingPerms []models.Permission
	if err := app.DB.Order("resource, action").Find(&existingPerms).Error; err == nil && len(existingPerms) > 0 {
		return existingPerms
	}

	// If no permissions exist, create test ones
	permissions := []models.Permission{
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "users", Action: "read", Description: "View users"},
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "users", Action: "write", Description: "Create/edit users"},
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "users", Action: "delete", Description: "Delete users"},
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "contacts", Action: "read", Description: "View contacts"},
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "contacts", Action: "write", Description: "Create/edit contacts"},
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "messages", Action: "read", Description: "View messages"},
		{BaseModel: models.BaseModel{ID: uuid.New()}, Resource: "messages", Action: "write", Description: "Send messages"},
	}

	for i := range permissions {
		require.NoError(t, app.DB.Create(&permissions[i]).Error)
	}

	return permissions
}

func TestApp_ListRoles_Success(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	permissions := getOrCreateTestPermissions(t, app)

	// Create some roles
	adminRole := createTestRole(t, app, org.ID, "Admin", true, false, permissions)
	agentRole := createTestRole(t, app, org.ID, "Agent", false, true, permissions[:3])

	// Create a user to make the request
	user := createTestUser(t, app, org.ID, uniqueEmail("list-roles"), "password123", &adminRole.ID, true)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)

	err := app.ListRoles(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Roles []handlers.RoleResponse `json:"roles"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Len(t, resp.Data.Roles, 2)

	// Check that roles are sorted (system first, then by name)
	assert.Equal(t, adminRole.Name, resp.Data.Roles[0].Name)
	assert.True(t, resp.Data.Roles[0].IsSystem)
	assert.Equal(t, agentRole.Name, resp.Data.Roles[1].Name)
	assert.True(t, resp.Data.Roles[1].IsDefault)
}

func TestApp_GetRole_Success(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	permissions := getOrCreateTestPermissions(t, app)

	role := createTestRole(t, app, org.ID, "Test Role", false, false, permissions[:2])
	user := createTestUser(t, app, org.ID, uniqueEmail("get-role"), "password123", &role.ID, true)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("id", role.ID.String())

	err := app.GetRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string                `json:"status"`
		Data   handlers.RoleResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, role.ID, resp.Data.ID)
	assert.Equal(t, role.Name, resp.Data.Name)
	assert.Len(t, resp.Data.Permissions, 2)
}

func TestApp_GetRole_NotFound(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("get-role-404"), "password123", nil, true)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("id", uuid.New().String())

	err := app.GetRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateRole_Success(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	permissions := getOrCreateTestPermissions(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-role"), "password123", nil, true)

	reqBody := handlers.RoleRequest{
		Name:        "New Role",
		Description: "A new custom role",
		IsDefault:   false,
		Permissions: []string{"users:read", "users:write"},
	}

	req := testutil.NewJSONRequest(t, reqBody)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)

	err := app.CreateRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string                `json:"status"`
		Data   handlers.RoleResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, "New Role", resp.Data.Name)
	assert.Equal(t, "A new custom role", resp.Data.Description)
	assert.False(t, resp.Data.IsSystem)
	assert.Len(t, resp.Data.Permissions, 2)

	// Verify permissions were assigned correctly
	var dbRole models.CustomRole
	require.NoError(t, app.DB.Preload("Permissions").First(&dbRole, "id = ?", resp.Data.ID).Error)
	assert.Len(t, dbRole.Permissions, 2)

	// Clean up permissions for next test
	_ = permissions
}

func TestApp_CreateRole_DuplicateName(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	_ = getOrCreateTestPermissions(t, app)

	createTestRole(t, app, org.ID, "Existing Role", false, false, nil)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-dup-role"), "password123", nil, true)

	reqBody := handlers.RoleRequest{
		Name:        "Existing Role",
		Description: "Trying to create duplicate",
		Permissions: []string{},
	}

	req := testutil.NewJSONRequest(t, reqBody)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)

	err := app.CreateRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateRole_MissingName(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-no-name"), "password123", nil, true)

	reqBody := handlers.RoleRequest{
		Name:        "",
		Description: "Role without name",
		Permissions: []string{},
	}

	req := testutil.NewJSONRequest(t, reqBody)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)

	err := app.CreateRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateRole_WithDefaultFlag(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	_ = getOrCreateTestPermissions(t, app)

	// Create an existing default role
	existingDefault := createTestRole(t, app, org.ID, "Old Default", false, true, nil)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-default"), "password123", nil, true)

	reqBody := handlers.RoleRequest{
		Name:        "New Default Role",
		Description: "This will be the new default",
		IsDefault:   true,
		Permissions: []string{},
	}

	req := testutil.NewJSONRequest(t, reqBody)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)

	err := app.CreateRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify the old default was unset
	var oldDefault models.CustomRole
	require.NoError(t, app.DB.First(&oldDefault, "id = ?", existingDefault.ID).Error)
	assert.False(t, oldDefault.IsDefault)
}

func TestApp_UpdateRole_Success(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	permissions := getOrCreateTestPermissions(t, app)

	role := createTestRole(t, app, org.ID, "Editable Role", false, false, permissions[:1])
	user := createTestUser(t, app, org.ID, uniqueEmail("update-role"), "password123", nil, true)

	reqBody := handlers.RoleRequest{
		Name:        "Updated Role Name",
		Description: "Updated description",
		Permissions: []string{"users:read", "users:write", "contacts:read"},
	}

	req := testutil.NewJSONRequest(t, reqBody)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("id", role.ID.String())

	err := app.UpdateRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string                `json:"status"`
		Data   handlers.RoleResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "Updated Role Name", resp.Data.Name)
	assert.Equal(t, "Updated description", resp.Data.Description)
	assert.Len(t, resp.Data.Permissions, 3)
}

func TestApp_UpdateRole_SystemRoleOnlyDescription(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	permissions := getOrCreateTestPermissions(t, app)

	// Create a system role
	systemRole := createTestRole(t, app, org.ID, "System Admin", true, false, permissions)
	user := createTestUser(t, app, org.ID, uniqueEmail("update-sys-role"), "password123", nil, true)

	reqBody := handlers.RoleRequest{
		Name:        "Changed Name",        // Should be ignored for system roles
		Description: "Updated description", // Only this should be updated
		Permissions: []string{"users:read"}, // Should be ignored for system roles
	}

	req := testutil.NewJSONRequest(t, reqBody)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("id", systemRole.ID.String())

	err := app.UpdateRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string                `json:"status"`
		Data   handlers.RoleResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	// Name should not change for system roles
	assert.Equal(t, "System Admin", resp.Data.Name)
	assert.Equal(t, "Updated description", resp.Data.Description)
	// Permissions should remain the same
	assert.Len(t, resp.Data.Permissions, len(permissions))
}

func TestApp_UpdateRole_NotFound(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("update-404"), "password123", nil, true)

	reqBody := handlers.RoleRequest{
		Name: "Updated Name",
	}

	req := testutil.NewJSONRequest(t, reqBody)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("id", uuid.New().String())

	err := app.UpdateRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteRole_Success(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)

	role := createTestRole(t, app, org.ID, "Deletable Role", false, false, nil)
	user := createTestUser(t, app, org.ID, uniqueEmail("delete-role"), "password123", nil, true)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("id", role.ID.String())

	err := app.DeleteRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify role was deleted
	var dbRole models.CustomRole
	err = app.DB.First(&dbRole, "id = ?", role.ID).Error
	assert.Error(t, err) // Should be not found
}

func TestApp_DeleteRole_SystemRole(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)

	systemRole := createTestRole(t, app, org.ID, "System Role", true, false, nil)
	user := createTestUser(t, app, org.ID, uniqueEmail("delete-sys"), "password123", nil, true)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("id", systemRole.ID.String())

	err := app.DeleteRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	// Verify role still exists
	var dbRole models.CustomRole
	require.NoError(t, app.DB.First(&dbRole, "id = ?", systemRole.ID).Error)
}

func TestApp_DeleteRole_WithAssignedUsers(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)

	role := createTestRole(t, app, org.ID, "Role With Users", false, false, nil)
	// Create a user with this role
	createTestUser(t, app, org.ID, uniqueEmail("assigned-user"), "password123", &role.ID, true)
	adminUser := createTestUser(t, app, org.ID, uniqueEmail("delete-used-role"), "password123", nil, true)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	req.RequestCtx.SetUserValue("user_id", adminUser.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("id", role.ID.String())

	err := app.DeleteRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_ListPermissions_Success(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	permissions := getOrCreateTestPermissions(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("list-perms"), "password123", nil, true)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)

	err := app.ListPermissions(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Permissions []handlers.PermissionResponse `json:"permissions"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.GreaterOrEqual(t, len(resp.Data.Permissions), len(permissions))

	// Verify permission format
	for _, perm := range resp.Data.Permissions {
		assert.NotEmpty(t, perm.Resource)
		assert.NotEmpty(t, perm.Action)
		assert.Equal(t, perm.Resource+":"+perm.Action, perm.Key)
	}
}
