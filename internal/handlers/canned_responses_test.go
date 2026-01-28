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
	"github.com/zerodha/fastglue"
)

// createTestCannedResponse creates a canned response directly in the database for testing.
func createTestCannedResponse(t *testing.T, app *handlers.App, orgID, userID uuid.UUID, name, shortcut, content, category string) *models.CannedResponse {
	t.Helper()

	cr := &models.CannedResponse{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           name,
		Shortcut:       shortcut,
		Content:        content,
		Category:       category,
		IsActive:       true,
		CreatedByID:    userID,
	}
	require.NoError(t, app.DB.Create(cr).Error)
	return cr
}

// --- ListCannedResponses Tests ---

func TestApp_ListCannedResponses(t *testing.T) {
	t.Parallel()

	t.Run("success with results", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		createTestCannedResponse(t, app, org.ID, user.ID, "Greeting", "/greet", "Hello! How can I help?", "general")
		createTestCannedResponse(t, app, org.ID, user.ID, "Farewell", "/bye", "Thank you, goodbye!", "general")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListCannedResponses(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.CannedResponses, 2)
	})

	t.Run("empty list", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListCannedResponses(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Data.CannedResponses)
	})

	t.Run("filters by category", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		createTestCannedResponse(t, app, org.ID, user.ID, "Sales Intro", "/sales", "Welcome to sales!", "sales")
		createTestCannedResponse(t, app, org.ID, user.ID, "Support Intro", "/support", "How can we help?", "support")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetQueryParam(req, "category", "sales")

		err := app.ListCannedResponses(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.CannedResponses, 1)
		assert.Equal(t, "Sales Intro", resp.Data.CannedResponses[0].Name)
	})

	t.Run("filters by search", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		createTestCannedResponse(t, app, org.ID, user.ID, "Hello World", "/hello", "Hello there!", "general")
		createTestCannedResponse(t, app, org.ID, user.ID, "Goodbye", "/goodbye", "See you later!", "general")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetQueryParam(req, "search", "Hello")

		err := app.ListCannedResponses(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.CannedResponses, 1)
		assert.Equal(t, "Hello World", resp.Data.CannedResponses[0].Name)
	})

	t.Run("filters active only", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		activeCR := createTestCannedResponse(t, app, org.ID, user.ID, "Active One", "/active", "Active content", "general")
		inactiveCR := createTestCannedResponse(t, app, org.ID, user.ID, "Inactive One", "/inactive", "Inactive content", "general")
		// Mark one as inactive
		require.NoError(t, app.DB.Model(inactiveCR).Update("is_active", false).Error)
		_ = activeCR

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetQueryParam(req, "active_only", "true")

		err := app.ListCannedResponses(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.CannedResponses, 1)
		assert.Equal(t, "Active One", resp.Data.CannedResponses[0].Name)
	})

	t.Run("isolates by organization", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

		createTestCannedResponse(t, app, org1.ID, user1.ID, "Org1 Response", "/org1", "Org1 content", "general")
		createTestCannedResponse(t, app, org2.ID, user2.ID, "Org2 Response", "/org2", "Org2 content", "general")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org1.ID, user1.ID)

		err := app.ListCannedResponses(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.CannedResponses, 1)
		assert.Equal(t, "Org1 Response", resp.Data.CannedResponses[0].Name)
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewGETRequest(t)
		// No auth context

		err := app.ListCannedResponses(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- CreateCannedResponse Tests ---

func TestApp_CreateCannedResponse(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":     "Welcome Message",
			"shortcut": "/welcome",
			"content":  "Welcome to our support!",
			"category": "onboarding",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CannedResponseResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Welcome Message", resp.Data.Name)
		assert.Equal(t, "/welcome", resp.Data.Shortcut)
		assert.Equal(t, "Welcome to our support!", resp.Data.Content)
		assert.Equal(t, "onboarding", resp.Data.Category)
		assert.True(t, resp.Data.IsActive)
		assert.Equal(t, 0, resp.Data.UsageCount)
		assert.NotEqual(t, uuid.Nil, resp.Data.ID)
	})

	t.Run("validation error missing name", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"content": "Some content",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("validation error missing content", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "No Content Response",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("validation error missing both name and content", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"shortcut": "/empty",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("duplicate name conflict", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		createTestCannedResponse(t, app, org.ID, user.ID, "Duplicate Name", "/dup", "First content", "general")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":    "Duplicate Name",
			"content": "Second content",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":    "Test",
			"content": "Content",
		})
		// No auth context

		err := app.CreateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- GetCannedResponse Tests ---

func TestApp_GetCannedResponse(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		cr := createTestCannedResponse(t, app, org.ID, user.ID, "Get Me", "/getme", "Get this response", "support")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", cr.ID.String())

		err := app.GetCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CannedResponseResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, cr.ID, resp.Data.ID)
		assert.Equal(t, "Get Me", resp.Data.Name)
		assert.Equal(t, "/getme", resp.Data.Shortcut)
		assert.Equal(t, "Get this response", resp.Data.Content)
		assert.Equal(t, "support", resp.Data.Category)
		assert.True(t, resp.Data.IsActive)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid id", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "not-a-uuid")

		err := app.GetCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

		cr := createTestCannedResponse(t, app, org1.ID, user1.ID, "Org1 Only", "/org1only", "Secret content", "general")

		// User from org2 tries to access org1's canned response
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", cr.ID.String())

		err := app.GetCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewGETRequest(t)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- UpdateCannedResponse Tests ---

func TestApp_UpdateCannedResponse(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		cr := createTestCannedResponse(t, app, org.ID, user.ID, "Original Name", "/orig", "Original content", "general")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":      "Updated Name",
			"shortcut":  "/updated",
			"content":   "Updated content",
			"category":  "updated-category",
			"is_active": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", cr.ID.String())

		err := app.UpdateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CannedResponseResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, cr.ID, resp.Data.ID)
		assert.Equal(t, "Updated Name", resp.Data.Name)
		assert.Equal(t, "/updated", resp.Data.Shortcut)
		assert.Equal(t, "Updated content", resp.Data.Content)
		assert.Equal(t, "updated-category", resp.Data.Category)
		assert.True(t, resp.Data.IsActive)
	})

	t.Run("partial update", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		cr := createTestCannedResponse(t, app, org.ID, user.ID, "Keep Name", "/keep", "Keep content", "keep-cat")

		req := testutil.NewJSONRequest(t, map[string]any{
			"content":  "Only content changed",
			"is_active": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", cr.ID.String())

		err := app.UpdateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CannedResponseResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		// Name should remain unchanged since empty string is not sent
		assert.Equal(t, "Keep Name", resp.Data.Name)
		assert.Equal(t, "Only content changed", resp.Data.Content)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":    "Updated",
			"content": "Updated content",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.UpdateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid id", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":    "Updated",
			"content": "Content",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "bad-uuid")

		err := app.UpdateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

		cr := createTestCannedResponse(t, app, org1.ID, user1.ID, "Org1 CR", "/org1cr", "Org1 content", "general")

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":    "Hijacked",
			"content": "Hijacked content",
		})
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", cr.ID.String())

		err := app.UpdateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Verify the original is unchanged
		var original models.CannedResponse
		require.NoError(t, app.DB.First(&original, "id = ?", cr.ID).Error)
		assert.Equal(t, "Org1 CR", original.Name)
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":    "Updated",
			"content": "Content",
		})
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.UpdateCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- DeleteCannedResponse Tests ---

func TestApp_DeleteCannedResponse(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		cr := createTestCannedResponse(t, app, org.ID, user.ID, "Delete Me", "/delme", "To be deleted", "general")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", cr.ID.String())

		err := app.DeleteCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Canned response deleted", resp.Data.Message)

		// Verify it is deleted (soft-deleted via GORM)
		var count int64
		app.DB.Model(&models.CannedResponse{}).Where("id = ?", cr.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.DeleteCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid id", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "invalid-uuid")

		err := app.DeleteCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)
		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

		cr := createTestCannedResponse(t, app, org1.ID, user1.ID, "Cannot Delete", "/nodelete", "Protected content", "general")

		// User from org2 tries to delete org1's canned response
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", cr.ID.String())

		err := app.DeleteCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Verify it still exists
		var count int64
		app.DB.Model(&models.CannedResponse{}).Where("id = ?", cr.ID).Count(&count)
		assert.Equal(t, int64(1), count)
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewGETRequest(t)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.DeleteCannedResponse(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- CreateCannedResponse Additional Tests ---

func TestApp_CreateCannedResponse_DuplicateShortcut(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	createTestCannedResponse(t, app, org.ID, user.ID, "First", "/dup-shortcut", "First content", "general")

	// Creating a second canned response with the same shortcut but different name should succeed,
	// since the handler only checks for duplicate names, not shortcuts.
	req := testutil.NewJSONRequest(t, map[string]any{
		"name":     "Second",
		"shortcut": "/dup-shortcut",
		"content":  "Second content",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCannedResponse(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Second", resp.Data.Name)
	assert.Equal(t, "/dup-shortcut", resp.Data.Shortcut)
}

func TestApp_CreateCannedResponse_SameNameDifferentOrgs(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	// Create a canned response in org1
	createTestCannedResponse(t, app, org1.ID, user1.ID, "Shared Name", "/sn1", "Org1 content", "general")

	// Creating the same name in org2 should succeed (name uniqueness is per-org)
	req := testutil.NewJSONRequest(t, map[string]any{
		"name":    "Shared Name",
		"content": "Org2 content",
	})
	testutil.SetAuthContext(req, org2.ID, user2.ID)

	err := app.CreateCannedResponse(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Shared Name", resp.Data.Name)
}

func TestApp_CreateCannedResponse_InvalidJSON(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// Send raw invalid JSON body
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody([]byte(`{invalid json`))
	req := &fastglue.Request{RequestCtx: ctx}
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCannedResponse(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateCannedResponse_WithAllOptionalFields(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":     "Full Response",
		"shortcut": "/full",
		"content":  "Full content with all fields",
		"category": "premium",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCannedResponse(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Full Response", resp.Data.Name)
	assert.Equal(t, "/full", resp.Data.Shortcut)
	assert.Equal(t, "Full content with all fields", resp.Data.Content)
	assert.Equal(t, "premium", resp.Data.Category)
	assert.True(t, resp.Data.IsActive)
	assert.Equal(t, 0, resp.Data.UsageCount)
	assert.NotEmpty(t, resp.Data.CreatedAt)
	assert.NotEmpty(t, resp.Data.UpdatedAt)
}

func TestApp_CreateCannedResponse_WithoutShortcutOrCategory(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// Shortcut and category are optional
	req := testutil.NewJSONRequest(t, map[string]any{
		"name":    "Minimal Response",
		"content": "Just name and content",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCannedResponse(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Minimal Response", resp.Data.Name)
	assert.Equal(t, "", resp.Data.Shortcut)
	assert.Equal(t, "", resp.Data.Category)
	assert.Equal(t, "Just name and content", resp.Data.Content)
}

// --- ListCannedResponses Additional Tests ---

func TestApp_ListCannedResponses_SearchByShortcut(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	createTestCannedResponse(t, app, org.ID, user.ID, "Alpha", "/alpha-cmd", "Alpha content", "general")
	createTestCannedResponse(t, app, org.ID, user.ID, "Beta", "/beta-cmd", "Beta content", "general")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "search", "/alpha")

	err := app.ListCannedResponses(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.CannedResponses, 1)
	assert.Equal(t, "Alpha", resp.Data.CannedResponses[0].Name)
}

func TestApp_ListCannedResponses_OrderedByUsageCount(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	crLow := createTestCannedResponse(t, app, org.ID, user.ID, "Low Usage", "/low", "Low usage content", "general")
	crHigh := createTestCannedResponse(t, app, org.ID, user.ID, "High Usage", "/high", "High usage content", "general")

	// Set usage counts: high=10, low=1
	require.NoError(t, app.DB.Model(crHigh).Update("usage_count", 10).Error)
	require.NoError(t, app.DB.Model(crLow).Update("usage_count", 1).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListCannedResponses(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Data.CannedResponses, 2)
	// Higher usage count should come first
	assert.Equal(t, "High Usage", resp.Data.CannedResponses[0].Name)
	assert.Equal(t, "Low Usage", resp.Data.CannedResponses[1].Name)
}

func TestApp_ListCannedResponses_SearchByContent(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	createTestCannedResponse(t, app, org.ID, user.ID, "Promo", "/promo", "Special discount offer!", "sales")
	createTestCannedResponse(t, app, org.ID, user.ID, "Normal", "/normal", "Regular response", "general")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "search", "discount")

	err := app.ListCannedResponses(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.CannedResponses, 1)
	assert.Equal(t, "Promo", resp.Data.CannedResponses[0].Name)
}

func TestApp_ListCannedResponses_CombinedCategoryAndSearch(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	createTestCannedResponse(t, app, org.ID, user.ID, "Sales Hello", "/sh", "Hello from sales", "sales")
	createTestCannedResponse(t, app, org.ID, user.ID, "Support Hello", "/sph", "Hello from support", "support")
	createTestCannedResponse(t, app, org.ID, user.ID, "Sales Bye", "/sb", "Bye from sales", "sales")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "category", "sales")
	testutil.SetQueryParam(req, "search", "Hello")

	err := app.ListCannedResponses(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.CannedResponses, 1)
	assert.Equal(t, "Sales Hello", resp.Data.CannedResponses[0].Name)
}

// --- UpdateCannedResponse Additional Tests ---

func TestApp_UpdateCannedResponse_DeactivateResponse(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	cr := createTestCannedResponse(t, app, org.ID, user.ID, "To Deactivate", "/deact", "Will be deactivated", "general")
	assert.True(t, cr.IsActive)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":      "To Deactivate",
		"content":   "Will be deactivated",
		"is_active": false,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", cr.ID.String())

	err := app.UpdateCannedResponse(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Data.IsActive)

	// Verify in DB
	var updated models.CannedResponse
	require.NoError(t, app.DB.First(&updated, "id = ?", cr.ID).Error)
	assert.False(t, updated.IsActive)
}

func TestApp_UpdateCannedResponse_ClearShortcutAndCategory(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	cr := createTestCannedResponse(t, app, org.ID, user.ID, "With Shortcut", "/shortcut", "Has shortcut", "support")

	// Update with empty shortcut and category to clear them
	req := testutil.NewJSONRequest(t, map[string]any{
		"name":      "With Shortcut",
		"content":   "Has shortcut",
		"shortcut":  "",
		"category":  "",
		"is_active": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", cr.ID.String())

	err := app.UpdateCannedResponse(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "", resp.Data.Shortcut)
	assert.Equal(t, "", resp.Data.Category)
}

func TestApp_UpdateCannedResponse_PreservesUsageCount(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	cr := createTestCannedResponse(t, app, org.ID, user.ID, "Count Preserver", "/countpres", "Usage should stay", "general")
	// Set a usage count
	require.NoError(t, app.DB.Model(cr).Update("usage_count", 42).Error)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":      "Count Preserver Updated",
		"content":   "Updated but usage stays",
		"is_active": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", cr.ID.String())

	err := app.UpdateCannedResponse(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Count Preserver Updated", resp.Data.Name)
	assert.Equal(t, 42, resp.Data.UsageCount)
}

// --- DeleteCannedResponse Additional Tests ---

func TestApp_DeleteCannedResponse_DoubleDelete(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	cr := createTestCannedResponse(t, app, org.ID, user.ID, "Delete Twice", "/del2x", "Double delete test", "general")

	// First delete should succeed
	req1 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req1, org.ID, user.ID)
	testutil.SetPathParam(req1, "id", cr.ID.String())

	err := app.DeleteCannedResponse(req1)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req1))

	// Second delete should return not found
	req2 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req2, org.ID, user.ID)
	testutil.SetPathParam(req2, "id", cr.ID.String())

	err = app.DeleteCannedResponse(req2)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req2))
}

func TestApp_DeleteCannedResponse_VerifyNotListedAfterDelete(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	cr := createTestCannedResponse(t, app, org.ID, user.ID, "Will Vanish", "/vanish", "Gone after delete", "general")
	createTestCannedResponse(t, app, org.ID, user.ID, "Will Stay", "/stay", "Remains after delete", "general")

	// Delete the first one
	delReq := testutil.NewGETRequest(t)
	testutil.SetAuthContext(delReq, org.ID, user.ID)
	testutil.SetPathParam(delReq, "id", cr.ID.String())

	err := app.DeleteCannedResponse(delReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(delReq))

	// List should only return the remaining one
	listReq := testutil.NewGETRequest(t)
	testutil.SetAuthContext(listReq, org.ID, user.ID)

	err = app.ListCannedResponses(listReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(listReq))

	var resp struct {
		Data struct {
			CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(listReq), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.CannedResponses, 1)
	assert.Equal(t, "Will Stay", resp.Data.CannedResponses[0].Name)
}

// --- IncrementCannedResponseUsage Additional Tests ---

func TestApp_IncrementCannedResponseUsage_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	cr := createTestCannedResponse(t, app, org1.ID, user1.ID, "Org1 Usage", "/org1usage", "Org1 only", "general")

	// User from org2 tries to increment org1's canned response usage
	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", cr.ID.String())

	err := app.IncrementCannedResponseUsage(req)
	require.NoError(t, err)
	// The handler uses UpdateColumn which succeeds even if no rows matched,
	// but the WHERE clause filters by org, so no row gets updated.
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify usage count was NOT incremented for org1's response
	var updated models.CannedResponse
	require.NoError(t, app.DB.First(&updated, "id = ?", cr.ID).Error)
	assert.Equal(t, 0, updated.UsageCount)
}

func TestApp_IncrementCannedResponseUsage_ReflectedInGet(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	cr := createTestCannedResponse(t, app, org.ID, user.ID, "Get After Increment", "/getinc", "Check via get", "general")

	// Increment usage
	incReq := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(incReq, org.ID, user.ID)
	testutil.SetPathParam(incReq, "id", cr.ID.String())

	err := app.IncrementCannedResponseUsage(incReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(incReq))

	// Get the canned response and verify usage_count is reflected
	getReq := testutil.NewGETRequest(t)
	testutil.SetAuthContext(getReq, org.ID, user.ID)
	testutil.SetPathParam(getReq, "id", cr.ID.String())

	err = app.GetCannedResponse(getReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(getReq))

	var resp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(getReq), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Data.UsageCount)
}

// --- CRUD Lifecycle Test ---

func TestApp_CannedResponse_FullLifecycle(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// 1. Create
	createReq := testutil.NewJSONRequest(t, map[string]any{
		"name":     "Lifecycle Response",
		"shortcut": "/lifecycle",
		"content":  "Original lifecycle content",
		"category": "lifecycle",
	})
	testutil.SetAuthContext(createReq, org.ID, user.ID)

	err := app.CreateCannedResponse(createReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(createReq))

	var createResp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(createReq), &createResp)
	require.NoError(t, err)
	crID := createResp.Data.ID
	assert.NotEqual(t, uuid.Nil, crID)

	// 2. Get
	getReq := testutil.NewGETRequest(t)
	testutil.SetAuthContext(getReq, org.ID, user.ID)
	testutil.SetPathParam(getReq, "id", crID.String())

	err = app.GetCannedResponse(getReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(getReq))

	var getResp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(getReq), &getResp)
	require.NoError(t, err)
	assert.Equal(t, "Lifecycle Response", getResp.Data.Name)
	assert.Equal(t, "Original lifecycle content", getResp.Data.Content)

	// 3. Update
	updateReq := testutil.NewJSONRequest(t, map[string]any{
		"name":      "Lifecycle Response Updated",
		"shortcut":  "/lifecycle-v2",
		"content":   "Updated lifecycle content",
		"category":  "lifecycle-v2",
		"is_active": true,
	})
	testutil.SetAuthContext(updateReq, org.ID, user.ID)
	testutil.SetPathParam(updateReq, "id", crID.String())

	err = app.UpdateCannedResponse(updateReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(updateReq))

	var updateResp struct {
		Data handlers.CannedResponseResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(updateReq), &updateResp)
	require.NoError(t, err)
	assert.Equal(t, "Lifecycle Response Updated", updateResp.Data.Name)
	assert.Equal(t, "Updated lifecycle content", updateResp.Data.Content)
	assert.Equal(t, "/lifecycle-v2", updateResp.Data.Shortcut)

	// 4. Increment usage
	incReq := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(incReq, org.ID, user.ID)
	testutil.SetPathParam(incReq, "id", crID.String())

	err = app.IncrementCannedResponseUsage(incReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(incReq))

	// 5. Verify in list
	listReq := testutil.NewGETRequest(t)
	testutil.SetAuthContext(listReq, org.ID, user.ID)

	err = app.ListCannedResponses(listReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(listReq))

	var listResp struct {
		Data struct {
			CannedResponses []handlers.CannedResponseResponse `json:"canned_responses"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(listReq), &listResp)
	require.NoError(t, err)
	require.Len(t, listResp.Data.CannedResponses, 1)
	assert.Equal(t, "Lifecycle Response Updated", listResp.Data.CannedResponses[0].Name)
	assert.Equal(t, 1, listResp.Data.CannedResponses[0].UsageCount)

	// 6. Delete
	delReq := testutil.NewGETRequest(t)
	testutil.SetAuthContext(delReq, org.ID, user.ID)
	testutil.SetPathParam(delReq, "id", crID.String())

	err = app.DeleteCannedResponse(delReq)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(delReq))

	// 7. Verify gone
	getReq2 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(getReq2, org.ID, user.ID)
	testutil.SetPathParam(getReq2, "id", crID.String())

	err = app.GetCannedResponse(getReq2)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(getReq2))
}

// --- IncrementCannedResponseUsage Tests ---

func TestApp_IncrementCannedResponseUsage(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		cr := createTestCannedResponse(t, app, org.ID, user.ID, "Usage Counter", "/usage", "Count me", "general")
		assert.Equal(t, 0, cr.UsageCount)

		req := testutil.NewJSONRequest(t, nil)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", cr.ID.String())

		err := app.IncrementCannedResponseUsage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Usage incremented", resp.Data.Message)

		// Verify count incremented in DB
		var updated models.CannedResponse
		require.NoError(t, app.DB.First(&updated, "id = ?", cr.ID).Error)
		assert.Equal(t, 1, updated.UsageCount)
	})

	t.Run("increments multiple times", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		cr := createTestCannedResponse(t, app, org.ID, user.ID, "Multi Usage", "/multi", "Count multiple", "general")

		// Increment 3 times
		for i := 0; i < 3; i++ {
			req := testutil.NewJSONRequest(t, nil)
			testutil.SetAuthContext(req, org.ID, user.ID)
			testutil.SetPathParam(req, "id", cr.ID.String())

			err := app.IncrementCannedResponseUsage(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
		}

		var updated models.CannedResponse
		require.NoError(t, app.DB.First(&updated, "id = ?", cr.ID).Error)
		assert.Equal(t, 3, updated.UsageCount)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, nil)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.IncrementCannedResponseUsage(req)
		require.NoError(t, err)
		// The handler uses UpdateColumn which succeeds even if no rows matched,
		// so this returns 200 with success message
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	})

	t.Run("invalid id", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, nil)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "not-a-uuid")

		err := app.IncrementCannedResponseUsage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewJSONRequest(t, nil)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.IncrementCannedResponseUsage(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}
