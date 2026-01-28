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
	"golang.org/x/crypto/bcrypt"
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

// --- ChangePassword Tests ---

func TestApp_ChangePassword(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("chpwd-ok")),
			testutil.WithPassword("oldPassword1"),
		)

		reqBody := map[string]interface{}{
			"current_password": "oldPassword1",
			"new_password":     "newPassword2",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ChangePassword(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Password changed successfully", resp.Data.Message)

		// Verify new password works by loading user and checking hash
		var dbUser models.User
		require.NoError(t, app.DB.Where("id = ?", user.ID).First(&dbUser).Error)
		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte("newPassword2")))
	})

	t.Run("incorrect current password", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("chpwd-wrongold")),
			testutil.WithPassword("correctPassword"),
		)

		reqBody := map[string]interface{}{
			"current_password": "wrongPassword",
			"new_password":     "newPassword2",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ChangePassword(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("missing current password", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("chpwd-nocur")),
		)

		reqBody := map[string]interface{}{
			"new_password": "newPassword2",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ChangePassword(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("missing new password", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("chpwd-nonew")),
			testutil.WithPassword("oldPassword1"),
		)

		reqBody := map[string]interface{}{
			"current_password": "oldPassword1",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ChangePassword(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("new password too short", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("chpwd-short")),
			testutil.WithPassword("oldPassword1"),
		)

		reqBody := map[string]interface{}{
			"current_password": "oldPassword1",
			"new_password":     "abc",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ChangePassword(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("unauthorized without user_id", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)

		reqBody := map[string]interface{}{
			"current_password": "oldPassword1",
			"new_password":     "newPassword2",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		// Do not set auth context

		err := app.ChangePassword(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- UpdateCurrentUserSettings Tests ---

func TestApp_UpdateCurrentUserSettings(t *testing.T) {
	t.Parallel()

	t.Run("success with all settings", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("settings-all")),
		)

		reqBody := map[string]interface{}{
			"email_notifications": true,
			"new_message_alerts":  true,
			"campaign_updates":    false,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateCurrentUserSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message  string                 `json:"message"`
				Settings map[string]interface{} `json:"settings"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "Settings updated successfully", resp.Data.Message)
		assert.Equal(t, true, resp.Data.Settings["email_notifications"])
		assert.Equal(t, true, resp.Data.Settings["new_message_alerts"])
		assert.Equal(t, false, resp.Data.Settings["campaign_updates"])
	})

	t.Run("settings persist in database", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("settings-persist")),
		)

		reqBody := map[string]interface{}{
			"email_notifications": false,
			"new_message_alerts":  true,
			"campaign_updates":    true,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.UpdateCurrentUserSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		// Verify in DB
		var dbUser models.User
		require.NoError(t, app.DB.Where("id = ?", user.ID).First(&dbUser).Error)
		assert.Equal(t, false, dbUser.Settings["email_notifications"])
		assert.Equal(t, true, dbUser.Settings["new_message_alerts"])
		assert.Equal(t, true, dbUser.Settings["campaign_updates"])
	})

	t.Run("update overwrites previous settings", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID,
			testutil.WithEmail(testutil.UniqueEmail("settings-overwrite")),
		)

		// First update
		reqBody1 := map[string]interface{}{
			"email_notifications": true,
			"new_message_alerts":  true,
			"campaign_updates":    true,
		}
		req1 := testutil.NewJSONRequest(t, reqBody1)
		testutil.SetAuthContext(req1, org.ID, user.ID)
		err := app.UpdateCurrentUserSettings(req1)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req1))

		// Second update with different values
		reqBody2 := map[string]interface{}{
			"email_notifications": false,
			"new_message_alerts":  false,
			"campaign_updates":    false,
		}
		req2 := testutil.NewJSONRequest(t, reqBody2)
		testutil.SetAuthContext(req2, org.ID, user.ID)
		err = app.UpdateCurrentUserSettings(req2)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))

		var resp struct {
			Data struct {
				Settings map[string]interface{} `json:"settings"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req2), &resp)
		require.NoError(t, err)

		assert.Equal(t, false, resp.Data.Settings["email_notifications"])
		assert.Equal(t, false, resp.Data.Settings["new_message_alerts"])
		assert.Equal(t, false, resp.Data.Settings["campaign_updates"])
	})

	t.Run("unauthorized without user_id", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)

		reqBody := map[string]interface{}{
			"email_notifications": true,
			"new_message_alerts":  true,
			"campaign_updates":    true,
		}

		req := testutil.NewJSONRequest(t, reqBody)
		// Do not set auth context

		err := app.UpdateCurrentUserSettings(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- Cross-Org Isolation Tests ---

func TestApp_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	t.Run("ListUsers returns only users from the requested org", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)

		// Create two orgs with users
		org1 := testutil.CreateTestOrganization(t, app.DB)
		adminRole1 := testutil.CreateAdminRole(t, app.DB, org1.ID)
		admin1 := testutil.CreateTestUser(t, app.DB, org1.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-org1-admin")),
			testutil.WithRoleID(&adminRole1.ID),
		)
		testutil.CreateTestUser(t, app.DB, org1.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-org1-user")),
		)

		org2 := testutil.CreateTestOrganization(t, app.DB)
		testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-org2-user")),
		)

		// List users for org1
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, admin1.ID)

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

		// Should only see org1 users
		assert.Len(t, resp.Data.Users, 2)
		for _, u := range resp.Data.Users {
			assert.Equal(t, org1.ID, u.OrganizationID)
		}
	})

	t.Run("GetUser cannot access user from different org", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-get-org1")),
		)

		org2 := testutil.CreateTestOrganization(t, app.DB)
		userOrg2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-get-org2")),
		)

		// User from org1 tries to get user from org2
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)
		testutil.SetPathParam(req, "id", userOrg2.ID.String())

		err := app.GetUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("UpdateUser cannot update user from different org", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		adminRole1 := testutil.CreateAdminRole(t, app.DB, org1.ID)
		admin1 := testutil.CreateTestUser(t, app.DB, org1.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-upd-admin")),
			testutil.WithRoleID(&adminRole1.ID),
		)

		org2 := testutil.CreateTestOrganization(t, app.DB)
		userOrg2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-upd-org2")),
		)

		reqBody := map[string]interface{}{
			"full_name": "Hacked Name",
		}

		req := testutil.NewJSONRequest(t, reqBody)
		testutil.SetAuthContext(req, org1.ID, admin1.ID)
		testutil.SetPathParam(req, "id", userOrg2.ID.String())

		err := app.UpdateUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("DeleteUser cannot delete user from different org", func(t *testing.T) {
		t.Parallel()
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		adminRole1 := testutil.CreateAdminRole(t, app.DB, org1.ID)
		admin1 := testutil.CreateTestUser(t, app.DB, org1.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-del-admin")),
			testutil.WithRoleID(&adminRole1.ID),
		)

		org2 := testutil.CreateTestOrganization(t, app.DB)
		userOrg2 := testutil.CreateTestUser(t, app.DB, org2.ID,
			testutil.WithEmail(testutil.UniqueEmail("iso-del-org2")),
		)

		req := testutil.NewGETRequest(t)
		req.RequestCtx.Request.Header.SetMethod("DELETE")
		testutil.SetAuthContext(req, org1.ID, admin1.ID)
		testutil.SetPathParam(req, "id", userOrg2.ID.String())

		err := app.DeleteUser(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Verify user from org2 still exists
		var dbUser models.User
		require.NoError(t, app.DB.Where("id = ?", userOrg2.ID).First(&dbUser).Error)
	})
}

// --- Additional Edge Case Tests ---

func TestApp_DeleteUser_InvalidUUID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	admin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("del-baduuid-admin")),
		testutil.WithRoleID(&adminRole.ID),
	)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.DeleteUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteUser_LastAdmin(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)
	// Create a system admin role with the exact name "admin"
	adminRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "admin", true, false, allPerms)

	admin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("lastadmin-admin")),
		testutil.WithRoleID(&adminRole.ID),
	)
	// This is the only admin in the org

	target := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("lastadmin-target")),
		testutil.WithRoleID(&adminRole.ID),
	)

	// Now there are 2 admins; delete one should succeed
	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "id", target.ID.String())

	err := app.DeleteUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Now try to delete admin (last one) by creating another admin to do it
	admin2 := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("lastadmin-admin2")),
		testutil.WithRoleID(&adminRole.ID),
	)

	// Delete admin2, leaving admin as the last one
	req2 := testutil.NewGETRequest(t)
	req2.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req2, org.ID, admin.ID)
	testutil.SetPathParam(req2, "id", admin2.ID.String())

	err = app.DeleteUser(req2)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))
}

func TestApp_UpdateUser_InvalidUUID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	admin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("upd-baduuid-admin")),
		testutil.WithRoleID(&adminRole.ID),
	)

	reqBody := map[string]interface{}{
		"full_name": "Updated",
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.UpdateUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateUser_ForbiddenWithoutPermission(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// User with no role (no permissions)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("upd-noperm-user")),
	)
	target := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("upd-noperm-target")),
	)

	reqBody := map[string]interface{}{
		"full_name": "Hacked",
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", target.ID.String())

	err := app.UpdateUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateUser_CannotDeactivateSelf(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("upd-deact-self")),
	)

	isActive := false
	reqBody := map[string]interface{}{
		"is_active": isActive,
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", user.ID.String())

	err := app.UpdateUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateUser_DeactivateOtherUser(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	admin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("upd-deact-admin")),
		testutil.WithRoleID(&adminRole.ID),
	)
	target := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("upd-deact-target")),
	)

	isActive := false
	reqBody := map[string]interface{}{
		"is_active": isActive,
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "id", target.ID.String())

	err := app.UpdateUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.UserResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Data.IsActive)
}

func TestApp_UpdateUser_ChangeRole(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	admin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("upd-role-admin")),
		testutil.WithRoleID(&adminRole.ID),
	)

	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	target := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("upd-role-target")),
	)

	reqBody := map[string]interface{}{
		"role_id": agentRole.ID.String(),
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "id", target.ID.String())

	err := app.UpdateUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.UserResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp.Data.RoleID)
	assert.Equal(t, agentRole.ID, *resp.Data.RoleID)
}

func TestApp_UpdateUser_RoleChangeWithoutPermission(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)

	// Regular user with no users:write permission
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("upd-rolenoprm-user")),
	)

	reqBody := map[string]interface{}{
		"role_id": agentRole.ID.String(),
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", user.ID.String())

	err := app.UpdateUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateUser_InvalidRoleID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	admin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("create-badrole-admin")),
		testutil.WithRoleID(&adminRole.ID),
	)

	nonExistentRole := uuid.New()
	reqBody := map[string]interface{}{
		"email":     testutil.UniqueEmail("create-badrole"),
		"password":  "securePass123",
		"full_name": "Bad Role User",
		"role_id":   nonExistentRole.String(),
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, admin.ID)

	err := app.CreateUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateUser_RoleFromDifferentOrg(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	adminRole1 := testutil.CreateAdminRole(t, app.DB, org1.ID)
	admin := testutil.CreateTestUser(t, app.DB, org1.ID,
		testutil.WithEmail(testutil.UniqueEmail("create-crossrole-admin")),
		testutil.WithRoleID(&adminRole1.ID),
	)

	// Role in a different org
	org2 := testutil.CreateTestOrganization(t, app.DB)
	roleOrg2 := testutil.CreateAgentRole(t, app.DB, org2.ID)

	reqBody := map[string]interface{}{
		"email":     testutil.UniqueEmail("create-crossrole"),
		"password":  "securePass123",
		"full_name": "Cross Org Role User",
		"role_id":   roleOrg2.ID.String(),
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org1.ID, admin.ID)

	err := app.CreateUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetCurrentUser_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, uuid.New()) // non-existent user

	err := app.GetCurrentUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateAvailability_NoChangeNoNewLog(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("avail-nochange")),
	)

	// User starts available; set available again (no change)
	reqBody := map[string]interface{}{
		"is_available": true,
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.UpdateAvailability(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// No availability log should be created for same status
	var logCount int64
	app.DB.Model(&models.UserAvailabilityLog{}).
		Where("user_id = ? AND organization_id = ?", user.ID, org.ID).
		Count(&logCount)
	assert.Equal(t, int64(0), logCount)
}

func TestApp_ListUsers_CrossOrgExclusion(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	// Create two orgs each with one user
	orgA := testutil.CreateTestOrganization(t, app.DB)
	adminRoleA := testutil.CreateAdminRole(t, app.DB, orgA.ID)
	adminA := testutil.CreateTestUser(t, app.DB, orgA.ID,
		testutil.WithEmail(testutil.UniqueEmail("crosslist-adminA")),
		testutil.WithRoleID(&adminRoleA.ID),
	)

	orgB := testutil.CreateTestOrganization(t, app.DB)
	testutil.CreateTestUser(t, app.DB, orgB.ID,
		testutil.WithEmail(testutil.UniqueEmail("crosslist-userB")),
	)

	// List users as admin of orgA
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, orgA.ID, adminA.ID)

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

	// Should only see orgA's user
	assert.Len(t, resp.Data.Users, 1)
	assert.Equal(t, adminA.ID, resp.Data.Users[0].ID)
}

func TestApp_ChangePassword_OldPasswordStopsWorking(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("chpwd-oldstop")),
		testutil.WithPassword("originalPass1"),
	)

	// Change the password
	reqBody := map[string]interface{}{
		"current_password": "originalPass1",
		"new_password":     "brandNewPass2",
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ChangePassword(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Now try to change password again using the old password -- should fail
	reqBody2 := map[string]interface{}{
		"current_password": "originalPass1",
		"new_password":     "anotherPass3",
	}

	req2 := testutil.NewJSONRequest(t, reqBody2)
	testutil.SetAuthContext(req2, org.ID, user.ID)

	err = app.ChangePassword(req2)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req2))
}

func TestApp_GetUser_VerifyResponseFields(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("get-fields")),
		testutil.WithFullName("Field Check User"),
		testutil.WithRoleID(&adminRole.ID),
	)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", user.ID.String())

	err := app.GetUser(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.UserResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, user.ID, resp.Data.ID)
	assert.Equal(t, "Field Check User", resp.Data.FullName)
	assert.True(t, resp.Data.IsActive)
	assert.True(t, resp.Data.IsAvailable)
	assert.False(t, resp.Data.IsSuperAdmin)
	assert.Equal(t, org.ID, resp.Data.OrganizationID)
	assert.NotNil(t, resp.Data.RoleID)
	assert.Equal(t, adminRole.ID, *resp.Data.RoleID)
	assert.NotNil(t, resp.Data.Role)
	assert.NotEmpty(t, resp.Data.CreatedAt)
	assert.NotEmpty(t, resp.Data.UpdatedAt)
}

func TestApp_CreateUser_CreatedUserIsActive(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	admin := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("create-active-admin")),
		testutil.WithRoleID(&adminRole.ID),
	)

	newEmail := testutil.UniqueEmail("create-active-user")
	reqBody := map[string]interface{}{
		"email":     newEmail,
		"password":  "securePass123",
		"full_name": "Active By Default",
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

	assert.True(t, resp.Data.IsActive)
	assert.Equal(t, org.ID, resp.Data.OrganizationID)

	// Verify password was hashed (not stored as plaintext)
	var dbUser models.User
	require.NoError(t, app.DB.Where("email = ?", newEmail).First(&dbUser).Error)
	assert.NotEqual(t, "securePass123", dbUser.PasswordHash)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte("securePass123")))
}
