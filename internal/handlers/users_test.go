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

// --- ListUsers Tests ---

func TestApp_ListUsers(t *testing.T) {
	t.Parallel()

	t.Run("success with multiple users", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("list-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)
		testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("list-user2")),
			testutil.WithFullName("Second User"),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, admin.ID)

		err := app.ListUsers(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Status string `json:"status"`
			Data   struct {
				Users []handlers.UserResponse `json:"users"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "success", resp.Status)
		assert.Len(t, resp.Data.Users, 2)
	})

	t.Run("empty list for new org", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		// Create a user in a different org so the admin has permissions
		otherOrg := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, otherOrg.ID)
		admin := testutil.CreateTestUser(t, app.DB, otherOrg.ID,
			testutil.WithEmail(testutil.UniqueEmail("list-empty-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)

		req := testutil.NewGETRequest(t)
		// Query for org that has no users, but auth as the admin from otherOrg
		testutil.SetAuthContext(req, org.ID, admin.ID)

		err := app.ListUsers(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Users []handlers.UserResponse `json:"users"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Data.Users)
	})

	t.Run("forbidden without users:read permission", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		// User with no role (no permissions)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("list-noperm")),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListUsers(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	})
}

// --- GetUser Tests ---

func TestApp_GetUser(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		targetEmail := testutil.UniqueEmail("get-target")
		target := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(targetEmail),
			testutil.WithFullName("Target User"),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, target.ID)
		testutil.SetPathParam(req, "id", target.ID.String())

		err := app.GetUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Status string                 `json:"status"`
			Data   handlers.UserResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "success", resp.Status)
		assert.Equal(t, target.ID, resp.Data.ID)
		assert.Equal(t, targetEmail, resp.Data.Email)
		assert.Equal(t, "Target User", resp.Data.FullName)
		assert.True(t, resp.Data.IsActive)
		assert.Equal(t, org.ID, resp.Data.OrganizationID)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, uuid.New())
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid uuid", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, uuid.New())
		testutil.SetPathParam(req, "id", "not-a-uuid")

		err := app.GetUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})
}

// --- CreateUser Tests ---

func TestApp_CreateUser(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("create-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)

		newEmail := testutil.UniqueEmail("create-new")
		reqBody := map[string]interface{}{
			"email":     newEmail,
			"password":  "securePass123",
			"full_name": "New User",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, admin.ID)

		err := app.CreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Status string                 `json:"status"`
			Data   handlers.UserResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "success", resp.Status)
		assert.Equal(t, newEmail, resp.Data.Email)
		assert.Equal(t, "New User", resp.Data.FullName)
		assert.True(t, resp.Data.IsActive)
		assert.Equal(t, org.ID, resp.Data.OrganizationID)
	})

	t.Run("success with role_id", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("create-withrole-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)

		newEmail := testutil.UniqueEmail("create-withrole")
		reqBody := map[string]interface{}{
			"email":     newEmail,
			"password":  "securePass123",
			"full_name": "Agent User",
			"role_id":   agentRole.ID.String(),
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, admin.ID)

		err := app.CreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.UserResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, newEmail, resp.Data.Email)
		assert.NotNil(t, resp.Data.RoleID)
		assert.Equal(t, agentRole.ID, *resp.Data.RoleID)
	})

	t.Run("duplicate email", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		existingEmail := testutil.UniqueEmail("create-dup")
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(existingEmail),
			testutil.WithRoleID(&adminRole.ID),
		)

		reqBody := map[string]interface{}{
			"email":     existingEmail,
			"password":  "securePass123",
			"full_name": "Duplicate User",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, admin.ID)

		err := app.CreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
	})

	t.Run("missing required fields", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("create-missing-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)

		// Missing password and full_name
		reqBody := map[string]interface{}{
			"email": testutil.UniqueEmail("create-missing"),
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, admin.ID)

		err := app.CreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("missing email", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("create-noemail-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)

		reqBody := map[string]interface{}{
			"password":  "securePass123",
			"full_name": "No Email User",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, admin.ID)

		err := app.CreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("forbidden without users:write permission", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		// User with no role (no permissions)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("create-noperm")),
		)

		reqBody := map[string]interface{}{
			"email":     testutil.UniqueEmail("create-noperm-new"),
			"password":  "securePass123",
			"full_name": "No Perm User",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	})
}

// --- UpdateUser Tests ---

func TestApp_UpdateUser(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("update-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)

		target := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("update-target")),
			testutil.WithFullName("Original Name"),
		)

		updatedName := "Updated Name"
		reqBody := map[string]interface{}{
			"full_name": updatedName,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, admin.ID)
		testutil.SetPathParam(req, "id", target.ID.String())

		err := app.UpdateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Status string                 `json:"status"`
			Data   handlers.UserResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "success", resp.Status)
		assert.Equal(t, updatedName, resp.Data.FullName)
		assert.Equal(t, target.ID, resp.Data.ID)
	})

	t.Run("self update allowed", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("selfupdate")),
			testutil.WithFullName("Old Name"),
		)

		reqBody := map[string]interface{}{
			"full_name": "Self Updated Name",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", user.ID.String())

		err := app.UpdateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.UserResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "Self Updated Name", resp.Data.FullName)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("update-404-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)

		reqBody := map[string]interface{}{
			"full_name": "Ghost",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, admin.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.UpdateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("update email", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("update-email-old")),
		)

		newEmail := testutil.UniqueEmail("update-email-new")
		reqBody := map[string]interface{}{
			"email": newEmail,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", user.ID.String())

		err := app.UpdateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.UserResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, newEmail, resp.Data.Email)
	})

	t.Run("duplicate email conflict", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		existingEmail := testutil.UniqueEmail("update-dup-existing")
		testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(existingEmail),
		)

		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("update-dup-user")),
		)

		reqBody := map[string]interface{}{
			"email": existingEmail,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", user.ID.String())

		err := app.UpdateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
	})
}

// --- DeleteUser Tests ---

func TestApp_DeleteUser(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("delete-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)

		target := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("delete-target")),
		)

		req := testutil.NewGETRequest(t)
		req.RequestCtx.Request.Header.SetMethod("DELETE")
		testutil.SetAuthContext(req, org.ID, admin.ID)
		testutil.SetPathParam(req, "id", target.ID.String())

		err := app.DeleteUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		// Verify user was soft-deleted
		var deletedUser models.User
		result := app.DB.Unscoped().Where("id = ?", target.ID).First(&deletedUser)
		require.NoError(t, result.Error)
		assert.True(t, deletedUser.DeletedAt.Valid)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("delete-404-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)

		req := testutil.NewGETRequest(t)
		req.RequestCtx.Request.Header.SetMethod("DELETE")
		testutil.SetAuthContext(req, org.ID, admin.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.DeleteUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("prevent self-delete", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		admin := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("delete-self-admin")),
			testutil.WithRoleID(&adminRole.ID),
		)

		req := testutil.NewGETRequest(t)
		req.RequestCtx.Request.Header.SetMethod("DELETE")
		testutil.SetAuthContext(req, org.ID, admin.ID)
		testutil.SetPathParam(req, "id", admin.ID.String())

		err := app.DeleteUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

		// Verify user still exists
		var user models.User
		require.NoError(t, app.DB.Where("id = ?", admin.ID).First(&user).Error)
	})

	t.Run("forbidden without users:delete permission", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		// User with no role (no permissions)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("delete-noperm")),
		)
		target := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("delete-noperm-target")),
		)

		req := testutil.NewGETRequest(t)
		req.RequestCtx.Request.Header.SetMethod("DELETE")
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", target.ID.String())

		err := app.DeleteUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	})
}

// --- GetCurrentUser Tests ---

func TestApp_GetCurrentUser(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		email := testutil.UniqueEmail("current-user")
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(email),
			testutil.WithFullName("Current User"),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.GetCurrentUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Status string                 `json:"status"`
			Data   handlers.UserResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "success", resp.Status)
		assert.Equal(t, user.ID, resp.Data.ID)
		assert.Equal(t, email, resp.Data.Email)
		assert.Equal(t, "Current User", resp.Data.FullName)
		assert.True(t, resp.Data.IsActive)
		assert.Equal(t, org.ID, resp.Data.OrganizationID)
	})

	t.Run("success with role info", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("current-with-role")),
			testutil.WithRoleID(&adminRole.ID),
		)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.GetCurrentUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.UserResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.NotNil(t, resp.Data.Role)
		assert.NotNil(t, resp.Data.RoleID)
		assert.Equal(t, adminRole.ID, *resp.Data.RoleID)
	})

	t.Run("unauthorized without user_id", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewGETRequest(t)
		// Do not set auth context -- no user_id

		err := app.GetCurrentUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- UpdateAvailability Tests ---

func TestApp_UpdateAvailability(t *testing.T) {
	t.Parallel()

	t.Run("toggle to unavailable", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("avail-off")),
		)

		// User starts as available (default from CreateTestUser)
		assert.True(t, user.IsAvailable)

		reqBody := map[string]interface{}{
			"is_available": false,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateAvailability(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message        string `json:"message"`
				IsAvailable    bool   `json:"is_available"`
				Status         string `json:"status"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "Availability updated successfully", resp.Data.Message)
		assert.False(t, resp.Data.IsAvailable)
		assert.Equal(t, "away", resp.Data.Status)

		// Verify in DB
		var dbUser models.User
		require.NoError(t, app.DB.Where("id = ?", user.ID).First(&dbUser).Error)
		assert.False(t, dbUser.IsAvailable)
	})

	t.Run("toggle to available", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("avail-on")),
		)

		// First set to unavailable
		require.NoError(t, app.DB.Model(user).Update("is_available", false).Error)

		reqBody := map[string]interface{}{
			"is_available": true,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateAvailability(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message     string `json:"message"`
				IsAvailable bool   `json:"is_available"`
				Status      string `json:"status"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "Availability updated successfully", resp.Data.Message)
		assert.True(t, resp.Data.IsAvailable)
		assert.Equal(t, "available", resp.Data.Status)

		// Verify in DB
		var dbUser models.User
		require.NoError(t, app.DB.Where("id = ?", user.ID).First(&dbUser).Error)
		assert.True(t, dbUser.IsAvailable)
	})

	t.Run("creates availability log on status change", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("avail-log")),
		)

		reqBody := map[string]interface{}{
			"is_available": false,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateAvailability(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		// Verify availability log was created
		var logCount int64
		app.DB.Model(&models.UserAvailabilityLog{}).
			Where("user_id = ? AND organization_id = ?", user.ID, org.ID).
			Count(&logCount)
		assert.Equal(t, int64(1), logCount)
	})

	t.Run("unauthorized without user_id", func(t *testing.T) {
		app := newTestApp(t)

		reqBody := map[string]interface{}{
			"is_available": false,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		// Do not set auth context

		err := app.UpdateAvailability(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}
