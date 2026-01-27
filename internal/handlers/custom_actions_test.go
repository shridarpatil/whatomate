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

// createTestCustomAction creates a custom action directly in the database.
func createTestCustomAction(t *testing.T, app *handlers.App, orgID uuid.UUID, name string, actionType models.ActionType, config map[string]interface{}, isActive bool, displayOrder int) *models.CustomAction {
	t.Helper()

	action := &models.CustomAction{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           name,
		Icon:           "zap",
		ActionType:     actionType,
		Config:         models.JSONB(config),
		IsActive:       isActive,
		DisplayOrder:   displayOrder,
	}
	require.NoError(t, app.DB.Create(action).Error)
	return action
}

// --- ListCustomActions Tests ---

func TestApp_ListCustomActions(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		createTestCustomAction(t, app, org.ID, "Action A", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook"}, true, 2)
		createTestCustomAction(t, app, org.ID, "Action B", models.ActionTypeURL,
			map[string]interface{}{"url": "https://example.com"}, true, 1)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListCustomActions(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				CustomActions []handlers.CustomActionResponse `json:"custom_actions"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.CustomActions, 2)
		// Ordered by display_order ASC
		assert.Equal(t, "Action B", resp.Data.CustomActions[0].Name)
		assert.Equal(t, "Action A", resp.Data.CustomActions[1].Name)
	})

	t.Run("EmptyList", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListCustomActions(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				CustomActions []handlers.CustomActionResponse `json:"custom_actions"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Data.CustomActions)
	})
}

// --- GetCustomAction Tests ---

func TestApp_GetCustomAction(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "My Webhook", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook", "method": "POST"}, true, 0)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.GetCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CustomActionResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, action.ID, resp.Data.ID)
		assert.Equal(t, "My Webhook", resp.Data.Name)
		assert.Equal(t, models.ActionTypeWebhook, resp.Data.ActionType)
		assert.Equal(t, "zap", resp.Data.Icon)
		assert.True(t, resp.Data.IsActive)
	})

	t.Run("NotFound", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.GetCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// --- CreateCustomAction Tests ---

func TestApp_CreateCustomAction(t *testing.T) {
	t.Parallel()

	t.Run("Success_Webhook", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":        "Send to CRM",
			"icon":        "send",
			"action_type": "webhook",
			"config": map[string]any{
				"url":    "https://crm.example.com/api/webhook",
				"method": "POST",
			},
			"is_active":     true,
			"display_order": 1,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CustomActionResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Send to CRM", resp.Data.Name)
		assert.Equal(t, "send", resp.Data.Icon)
		assert.Equal(t, models.ActionTypeWebhook, resp.Data.ActionType)
		assert.True(t, resp.Data.IsActive)
		assert.Equal(t, 1, resp.Data.DisplayOrder)
		assert.NotEqual(t, uuid.Nil, resp.Data.ID)
		assert.NotEmpty(t, resp.Data.CreatedAt)

		// Verify persisted in DB
		var count int64
		app.DB.Model(&models.CustomAction{}).Where("id = ?", resp.Data.ID).Count(&count)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Success_URL", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":        "Open Profile",
			"action_type": "url",
			"config": map[string]any{
				"url":             "https://crm.example.com/contact/{{contact.id}}",
				"open_in_new_tab": true,
			},
			"is_active": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CustomActionResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, models.ActionTypeURL, resp.Data.ActionType)
	})

	t.Run("Success_JavaScript", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":        "Copy Phone",
			"action_type": "javascript",
			"config": map[string]any{
				"code": "navigator.clipboard.writeText(context.contact.phone_number)",
			},
			"is_active": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CustomActionResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, models.ActionTypeJavascript, resp.Data.ActionType)
	})

	t.Run("ValidationError_MissingName", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"action_type": "webhook",
			"config": map[string]any{
				"url": "https://example.com/hook",
			},
			"is_active": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("ValidationError_MissingActionType", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "No Type",
			"config": map[string]any{
				"url": "https://example.com",
			},
			"is_active": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("ValidationError_InvalidActionType", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":        "Bad Type",
			"action_type": "invalid_type",
			"config": map[string]any{
				"url": "https://example.com",
			},
			"is_active": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("ValidationError_WebhookMissingURL", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":        "No URL Webhook",
			"action_type": "webhook",
			"config":      map[string]any{},
			"is_active":   true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("ValidationError_URLMissingURL", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":        "No URL Action",
			"action_type": "url",
			"config":      map[string]any{},
			"is_active":   true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("ValidationError_JavaScriptMissingCode", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":        "No Code JS",
			"action_type": "javascript",
			"config":      map[string]any{},
			"is_active":   true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("Unauthorized", func(t *testing.T) {
		app := newTestApp(t)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":        "Test",
			"action_type": "webhook",
			"config": map[string]any{
				"url": "https://example.com/hook",
			},
		})
		// No auth context

		err := app.CreateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})
}

// --- UpdateCustomAction Tests ---

func TestApp_UpdateCustomAction(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "Original Name", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":          "Updated Name",
			"icon":          "star",
			"is_active":     false,
			"display_order": 5,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.UpdateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CustomActionResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, action.ID, resp.Data.ID)
		assert.Equal(t, "Updated Name", resp.Data.Name)
		assert.Equal(t, "star", resp.Data.Icon)
		assert.False(t, resp.Data.IsActive)
		assert.Equal(t, 5, resp.Data.DisplayOrder)
	})

	t.Run("Success_UpdateConfig", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "Webhook Action", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://old.example.com/hook"}, true, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"config": map[string]any{
				"url":    "https://new.example.com/hook",
				"method": "PUT",
			},
			"is_active": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.UpdateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.CustomActionResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "https://new.example.com/hook", resp.Data.Config["url"])
	})

	t.Run("NotFound", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":      "Updated",
			"is_active": true,
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.UpdateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("CrossOrgIsolation", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

		action := createTestCustomAction(t, app, org1.ID, "Org1 Action", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

		// User from org2 tries to update org1's action
		req := testutil.NewJSONRequest(t, map[string]any{
			"name":      "Hijacked",
			"is_active": true,
		})
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.UpdateCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})
}

// --- DeleteCustomAction Tests ---

func TestApp_DeleteCustomAction(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "To Delete", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.DeleteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Equal(t, "deleted", resp.Data.Status)

		// Verify removed from DB
		var count int64
		app.DB.Model(&models.CustomAction{}).Where("id = ?", action.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("NotFound", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.DeleteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("CrossOrgIsolation", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

		action := createTestCustomAction(t, app, org1.ID, "Org1 Action", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

		// User from org2 tries to delete org1's action
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.DeleteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Action should still exist
		var count int64
		app.DB.Model(&models.CustomAction{}).Where("id = ?", action.ID).Count(&count)
		assert.Equal(t, int64(1), count)
	})
}
