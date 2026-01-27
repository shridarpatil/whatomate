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
