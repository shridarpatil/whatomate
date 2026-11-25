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

func TestApp_ListRoles_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	permissions := testutil.GetOrCreateTestPermissions(t, app.DB)

	// Create some roles
	adminRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Admin", true, false, permissions)
	agentRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Agent", false, true, permissions[:3])

	// Create a user to make the request
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-roles")), testutil.WithRoleID(&adminRole.ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	permissions := testutil.GetOrCreateTestPermissions(t, app.DB)

	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Test Role", false, false, permissions[:2])
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-role")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("get-role-404")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("user_id", user.ID)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("id", uuid.New().String())

	err := app.GetRole(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateRole_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	permissions := testutil.GetOrCreateTestPermissions(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-role")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	_ = testutil.GetOrCreateTestPermissions(t, app.DB)

	testutil.CreateTestRoleExact(t, app.DB, org.ID, "Existing Role", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-dup-role")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-no-name")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	_ = testutil.GetOrCreateTestPermissions(t, app.DB)

	// Create an existing default role
	existingDefault := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Old Default", false, true, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-default")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	permissions := testutil.GetOrCreateTestPermissions(t, app.DB)

	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Editable Role", false, false, permissions[:1])
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-role")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	permissions := testutil.GetOrCreateTestPermissions(t, app.DB)

	// Create a system role
	systemRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "System Admin", true, false, permissions)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-sys-role")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

	reqBody := handlers.RoleRequest{
		Name:        "Changed Name",         // Should be ignored for system roles
		Description: "Updated description",  // Only this should be updated
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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("update-404")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Deletable Role", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-role")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	systemRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "System Role", true, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-sys")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Role With Users", false, false, nil)
	// Create a user with this role
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("assigned-user")), testutil.WithRoleID(&role.ID))
	adminUser := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-used-role")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	permissions := testutil.GetOrCreateTestPermissions(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-perms")), testutil.WithRoleID(&testutil.CreateAdminRole(t, app.DB, org.ID).ID))

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

// --- Permission checks ---

func rolesTestUserWithPerms(t *testing.T, app *handlers.App, orgID uuid.UUID, keys ...string) *models.User {
	t.Helper()
	role := testutil.CreateTestRoleWithKeys(t, app.DB, orgID, "roles-perm", keys)
	return testutil.CreateTestUser(t, app.DB, orgID, testutil.WithRoleID(&role.ID))
}

func TestApp_Roles_RequirePermissions(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// Agent-like user: no roles or users permissions.
	user := rolesTestUserWithPerms(t, app, org.ID, "chat:read", "contacts:read")

	t.Run("list roles forbidden", func(t *testing.T) {
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		require.NoError(t, app.ListRoles(req))
		assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	})

	t.Run("list permissions forbidden", func(t *testing.T) {
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		require.NoError(t, app.ListPermissions(req))
		assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	})

	t.Run("create role forbidden", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, handlers.RoleRequest{Name: "Sneaky", Permissions: []string{"users:read"}})
		testutil.SetAuthContext(req, org.ID, user.ID)
		require.NoError(t, app.CreateRole(req))
		assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))

		var count int64
		app.DB.Model(&models.CustomRole{}).Where("organization_id = ? AND name = ?", org.ID, "Sneaky").Count(&count)
		assert.Zero(t, count)
	})

	t.Run("delete role forbidden", func(t *testing.T) {
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Keep Me", false, false, nil)
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", role.ID.String())
		require.NoError(t, app.DeleteRole(req))
		assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	})
}

func TestApp_Roles_ReadPermissionAllowsRead(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := rolesTestUserWithPerms(t, app, org.ID, "roles:read")
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Readable", false, false, nil)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", role.ID.String())
	require.NoError(t, app.GetRole(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	req = testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.ListPermissions(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// roles:read alone does not allow creating roles.
	req = testutil.NewJSONRequest(t, handlers.RoleRequest{Name: "Nope"})
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.CreateRole(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// The Users page lists roles for its role pickers, so users:read is enough to
// list roles, but not to open a role or see the permission catalog.
func TestApp_ListRoles_UsersReadAllowed(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := rolesTestUserWithPerms(t, app, org.ID, "users:read")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.ListRoles(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Hidden", false, false, nil)
	req = testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", role.ID.String())
	require.NoError(t, app.GetRole(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}
