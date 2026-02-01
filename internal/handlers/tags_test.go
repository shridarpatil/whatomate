package handlers_test

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// createTestTag creates a tag directly in the database for testing.
func createTestTag(t *testing.T, app *handlers.App, orgID uuid.UUID, name, color string) *models.Tag {
	t.Helper()

	tag := &models.Tag{
		OrganizationID: orgID,
		Name:           name,
		Color:          color,
	}
	require.NoError(t, app.DB.Create(tag).Error)
	return tag
}

// --- ListTags Tests ---

func TestApp_ListTags(t *testing.T) {
	t.Parallel()

	t.Run("success with results", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "VIP", "blue")
		createTestTag(t, app, org.ID, "Premium", "purple")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListTags(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Tags []handlers.TagResponse `json:"tags"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Tags, 2)
	})

	t.Run("empty list", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListTags(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Tags []handlers.TagResponse `json:"tags"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Data.Tags)
	})

	t.Run("sorted alphabetically", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "Zebra", "gray")
		createTestTag(t, app, org.ID, "Alpha", "blue")
		createTestTag(t, app, org.ID, "Middle", "green")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListTags(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Tags []handlers.TagResponse `json:"tags"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		require.Len(t, resp.Data.Tags, 3)
		assert.Equal(t, "Alpha", resp.Data.Tags[0].Name)
		assert.Equal(t, "Middle", resp.Data.Tags[1].Name)
		assert.Equal(t, "Zebra", resp.Data.Tags[2].Name)
	})

	t.Run("isolates by organization", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		role1 := testutil.CreateAdminRole(t, app.DB, org1.ID)
		role2 := testutil.CreateAdminRole(t, app.DB, org2.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&role1.ID))
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithRoleID(&role2.ID))

		createTestTag(t, app, org1.ID, "Org1Tag", "blue")
		createTestTag(t, app, org2.ID, "Org2Tag", "red")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)

		err := app.ListTags(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Tags []handlers.TagResponse `json:"tags"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.Tags, 1)
		assert.Equal(t, "Org1Tag", resp.Data.Tags[0].Name)
		_ = user2
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewGETRequest(t)
		// No auth context

		err := app.ListTags(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- CreateTag Tests ---

func TestApp_CreateTag(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "New Lead",
			"color": "green",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.TagResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "New Lead", resp.Data.Name)
		assert.Equal(t, "green", resp.Data.Color)
		assert.NotEmpty(t, resp.Data.CreatedAt)
	})

	t.Run("success with default color", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "No Color Tag",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.TagResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "No Color Tag", resp.Data.Name)
		assert.Equal(t, "", resp.Data.Color) // Empty color is allowed
	})

	t.Run("validation error missing name", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{
			"color": "blue",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("validation error name too long", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		longName := ""
		for range 51 {
			longName += "a"
		}

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": longName,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("validation error invalid color", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "Invalid Color Tag",
			"color": "pink", // Invalid color
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("duplicate name conflict", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "Duplicate", "blue")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "Duplicate",
			"color": "red",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
	})

	t.Run("same name different orgs allowed", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		role1 := testutil.CreateAdminRole(t, app.DB, org1.ID)
		role2 := testutil.CreateAdminRole(t, app.DB, org2.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&role1.ID))
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithRoleID(&role2.ID))

		createTestTag(t, app, org1.ID, "SharedName", "blue")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "SharedName",
			"color": "green",
		})
		testutil.SetAuthContext(req, org2.ID, user2.ID)

		err := app.CreateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
		_ = user1
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "Test",
			"color": "blue",
		})
		// No auth context

		err := app.CreateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- UpdateTag Tests ---

func TestApp_UpdateTag(t *testing.T) {
	t.Parallel()

	t.Run("success update color only", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "ColorChange", "blue")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "ColorChange",
			"color": "red",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", "ColorChange")

		err := app.UpdateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.TagResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "ColorChange", resp.Data.Name)
		assert.Equal(t, "red", resp.Data.Color)
	})

	t.Run("success rename tag", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "OldName", "blue")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "NewName",
			"color": "blue",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", "OldName")

		err := app.UpdateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.TagResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "NewName", resp.Data.Name)

		// Verify old tag no longer exists
		var count int64
		app.DB.Model(&models.Tag{}).Where("organization_id = ? AND name = ?", org.ID, "OldName").Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("rename preserves color if not specified", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "KeepColor", "purple")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "KeepColorRenamed",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", "KeepColor")

		err := app.UpdateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.TagResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "KeepColorRenamed", resp.Data.Name)
		assert.Equal(t, "purple", resp.Data.Color)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "UpdatedName",
			"color": "blue",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", "NonExistent")

		err := app.UpdateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("rename to existing name conflict", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "ExistingTag", "blue")
		createTestTag(t, app, org.ID, "TagToRename", "green")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "ExistingTag",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", "TagToRename")

		err := app.UpdateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid color", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "InvalidColorUpdate", "blue")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "InvalidColorUpdate",
			"color": "orange", // Invalid
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", "InvalidColorUpdate")

		err := app.UpdateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		role1 := testutil.CreateAdminRole(t, app.DB, org1.ID)
		role2 := testutil.CreateAdminRole(t, app.DB, org2.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&role1.ID))
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithRoleID(&role2.ID))

		createTestTag(t, app, org1.ID, "Org1OnlyTag", "blue")

		// User from org2 tries to update org1's tag
		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "Hijacked",
			"color": "red",
		})
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "name", "Org1OnlyTag")

		err := app.UpdateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Verify original is unchanged
		var tag models.Tag
		require.NoError(t, app.DB.Where("organization_id = ? AND name = ?", org1.ID, "Org1OnlyTag").First(&tag).Error)
		assert.Equal(t, "blue", tag.Color)
		_ = user1
	})

	t.Run("url encoded name", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "Tag With Spaces", "blue")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "Tag With Spaces",
			"color": "green",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", url.PathEscape("Tag With Spaces"))

		err := app.UpdateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.TagResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "green", resp.Data.Color)
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":  "Updated",
			"color": "blue",
		})
		testutil.SetPathParam(req, "name", "SomeTag")

		err := app.UpdateTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- DeleteTag Tests ---

func TestApp_DeleteTag(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "DeleteMe", "red")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", "DeleteMe")

		err := app.DeleteTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Tag deleted", resp.Data.Message)

		// Verify it's deleted
		var count int64
		app.DB.Model(&models.Tag{}).Where("organization_id = ? AND name = ?", org.ID, "DeleteMe").Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", "NonExistent")

		err := app.DeleteTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		role1 := testutil.CreateAdminRole(t, app.DB, org1.ID)
		role2 := testutil.CreateAdminRole(t, app.DB, org2.ID)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&role1.ID))
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithRoleID(&role2.ID))

		createTestTag(t, app, org1.ID, "ProtectedTag", "blue")

		// User from org2 tries to delete org1's tag
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "name", "ProtectedTag")

		err := app.DeleteTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Verify it still exists
		var count int64
		app.DB.Model(&models.Tag{}).Where("organization_id = ? AND name = ?", org1.ID, "ProtectedTag").Count(&count)
		assert.Equal(t, int64(1), count)
		_ = user1
	})

	t.Run("double delete", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "DeleteTwice", "gray")

		// First delete
		req1 := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req1, org.ID, user.ID)
		testutil.SetPathParam(req1, "name", "DeleteTwice")

		err := app.DeleteTag(req1)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req1))

		// Second delete should return not found
		req2 := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req2, org.ID, user.ID)
		testutil.SetPathParam(req2, "name", "DeleteTwice")

		err = app.DeleteTag(req2)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req2))
	})

	t.Run("url encoded name", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		role := testutil.CreateAdminRole(t, app.DB, org.ID)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

		createTestTag(t, app, org.ID, "Delete With Spaces", "yellow")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "name", url.PathEscape("Delete With Spaces"))

		err := app.DeleteTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewGETRequest(t)
		testutil.SetPathParam(req, "name", "SomeTag")

		err := app.DeleteTag(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- Tag Full Lifecycle Test ---

func TestApp_Tag_FullLifecycle(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	// 1. Create
	createReq := testutil.NewJSONRequest(t, map[string]any{
		"name":  "Lifecycle Tag",
		"color": "blue",
	})
	testutil.SetAuthContext(createReq, org.ID, user.ID)

	err := app.CreateTag(createReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(createReq))

	var createResp struct {
		Data handlers.TagResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(createReq), &createResp)
	require.NoError(t, err)
	assert.Equal(t, "Lifecycle Tag", createResp.Data.Name)
	assert.Equal(t, "blue", createResp.Data.Color)

	// 2. List and verify
	listReq := testutil.NewGETRequest(t)
	testutil.SetAuthContext(listReq, org.ID, user.ID)

	err = app.ListTags(listReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(listReq))

	var listResp struct {
		Data struct {
			Tags []handlers.TagResponse `json:"tags"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(listReq), &listResp)
	require.NoError(t, err)
	require.Len(t, listResp.Data.Tags, 1)
	assert.Equal(t, "Lifecycle Tag", listResp.Data.Tags[0].Name)

	// 3. Update color
	updateReq := testutil.NewJSONRequest(t, map[string]any{
		"name":  "Lifecycle Tag",
		"color": "green",
	})
	testutil.SetAuthContext(updateReq, org.ID, user.ID)
	testutil.SetPathParam(updateReq, "name", "Lifecycle Tag")

	err = app.UpdateTag(updateReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(updateReq))

	var updateResp struct {
		Data handlers.TagResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(updateReq), &updateResp)
	require.NoError(t, err)
	assert.Equal(t, "green", updateResp.Data.Color)

	// 4. Rename
	renameReq := testutil.NewJSONRequest(t, map[string]any{
		"name":  "Lifecycle Tag Renamed",
		"color": "green",
	})
	testutil.SetAuthContext(renameReq, org.ID, user.ID)
	testutil.SetPathParam(renameReq, "name", "Lifecycle Tag")

	err = app.UpdateTag(renameReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(renameReq))

	var renameResp struct {
		Data handlers.TagResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(renameReq), &renameResp)
	require.NoError(t, err)
	assert.Equal(t, "Lifecycle Tag Renamed", renameResp.Data.Name)

	// 5. Delete
	deleteReq := testutil.NewGETRequest(t)
	testutil.SetAuthContext(deleteReq, org.ID, user.ID)
	testutil.SetPathParam(deleteReq, "name", "Lifecycle Tag Renamed")

	err = app.DeleteTag(deleteReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(deleteReq))

	// 6. Verify gone
	listReq2 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(listReq2, org.ID, user.ID)

	err = app.ListTags(listReq2)
	require.NoError(t, err)

	var listResp2 struct {
		Data struct {
			Tags []handlers.TagResponse `json:"tags"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(listReq2), &listResp2)
	require.NoError(t, err)
	assert.Empty(t, listResp2.Data.Tags)
}

// --- Valid Colors Test ---

func TestApp_CreateTag_AllValidColors(t *testing.T) {
	t.Parallel()

	validColors := []string{"blue", "red", "green", "yellow", "purple", "gray"}

	for _, color := range validColors {
		t.Run(color, func(t *testing.T) {
			t.Parallel()

			app := newTestApp(t)
			org := testutil.CreateTestOrganization(t, app.DB)
			role := testutil.CreateAdminRole(t, app.DB, org.ID)
			user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

			req := testutil.NewJSONRequest(t, map[string]any{
				"name":  "Tag " + color,
				"color": color,
			})
			testutil.SetAuthContext(req, org.ID, user.ID)

			err := app.CreateTag(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

			var resp struct {
				Data handlers.TagResponse `json:"data"`
			}
			err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
			require.NoError(t, err)
			assert.Equal(t, color, resp.Data.Color)
		})
	}
}
