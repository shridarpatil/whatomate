package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	// GORM skips zero-value bools on INSERT, so the DB default (true) takes effect.
	// Explicitly UPDATE the column when the caller wants isActive=false.
	if !isActive {
		require.NoError(t, app.DB.Model(action).Update("is_active", false).Error)
		action.IsActive = false
	}
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
				"code": "return { clipboard: contact.phone_number, toast: { message: 'Copied!', type: 'success' } }",
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

// --- ListCustomActions Cross-Org Isolation ---

func TestApp_ListCustomActions_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	createTestCustomAction(t, app, org1.ID, "Org1 Action", models.ActionTypeWebhook,
		map[string]interface{}{"url": "https://example.com/hook1"}, true, 0)
	createTestCustomAction(t, app, org2.ID, "Org2 Action", models.ActionTypeURL,
		map[string]interface{}{"url": "https://example.com/page"}, true, 0)

	// User1 should only see org1's action
	req1 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req1, org1.ID, user1.ID)

	err := app.ListCustomActions(req1)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req1))

	var resp1 struct {
		Data struct {
			CustomActions []handlers.CustomActionResponse `json:"custom_actions"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req1), &resp1)
	require.NoError(t, err)
	assert.Len(t, resp1.Data.CustomActions, 1)
	assert.Equal(t, "Org1 Action", resp1.Data.CustomActions[0].Name)

	// User2 should only see org2's action
	req2 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req2, org2.ID, user2.ID)

	err = app.ListCustomActions(req2)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))

	var resp2 struct {
		Data struct {
			CustomActions []handlers.CustomActionResponse `json:"custom_actions"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req2), &resp2)
	require.NoError(t, err)
	assert.Len(t, resp2.Data.CustomActions, 1)
	assert.Equal(t, "Org2 Action", resp2.Data.CustomActions[0].Name)
}

// --- GetCustomAction Cross-Org Isolation ---

func TestApp_GetCustomAction_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	action := createTestCustomAction(t, app, org1.ID, "Org1 Secret Action", models.ActionTypeWebhook,
		map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

	// User from org2 tries to get org1's action
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", action.ID.String())

	err := app.GetCustomAction(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- GetCustomAction Invalid ID ---

func TestApp_GetCustomAction_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-valid-uuid")

	err := app.GetCustomAction(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- UpdateCustomAction with Invalid ActionType ---

func TestApp_UpdateCustomAction_InvalidActionType(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	action := createTestCustomAction(t, app, org.ID, "Test Action", models.ActionTypeWebhook,
		map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

	req := testutil.NewJSONRequest(t, map[string]any{
		"action_type": "invalid_type",
		"is_active":   true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", action.ID.String())

	err := app.UpdateCustomAction(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- UpdateCustomAction with Invalid Config for current action type ---

func TestApp_UpdateCustomAction_InvalidConfig(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	action := createTestCustomAction(t, app, org.ID, "Webhook Action", models.ActionTypeWebhook,
		map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

	// Try to update config without required url for webhook type
	req := testutil.NewJSONRequest(t, map[string]any{
		"config":    map[string]any{"method": "PUT"},
		"is_active": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", action.ID.String())

	err := app.UpdateCustomAction(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- ExecuteCustomAction Tests ---

func TestApp_ExecuteCustomAction(t *testing.T) {
	t.Parallel()

	t.Run("Success_Webhook", func(t *testing.T) {
		t.Parallel()

		// Create a mock webhook server
		var receivedBody []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			receivedBody = buf
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		}))
		defer server.Close()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "CRM Webhook", models.ActionTypeWebhook,
			map[string]interface{}{"url": server.URL, "method": "POST"}, true, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": contact.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.ActionResult `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Data.Success)
		assert.Contains(t, resp.Data.Message, "successfully")
		assert.NotNil(t, resp.Data.Toast)
		assert.Equal(t, "success", resp.Data.Toast.Type)

		// Verify webhook received data
		assert.NotEmpty(t, receivedBody)
	})

	t.Run("Success_URL", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "Open CRM", models.ActionTypeURL,
			map[string]interface{}{
				"url":             "https://crm.example.com/contact/{{contact.id}}",
				"open_in_new_tab": true,
			}, true, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": contact.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.ActionResult `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Data.Success)
		assert.Equal(t, "Opening URL", resp.Data.Message)
		assert.NotEmpty(t, resp.Data.RedirectURL)
		assert.Contains(t, resp.Data.RedirectURL, "/api/custom-actions/redirect/")
	})

	t.Run("Success_JavaScript", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "Copy Phone", models.ActionTypeJavascript,
			map[string]interface{}{
				"code": "return { clipboard: contact.phone_number, toast: { message: 'Copied!', type: 'success' } }",
			}, true, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": contact.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.ActionResult `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Data.Success)
		assert.Equal(t, "JavaScript action executed", resp.Data.Message)
		assert.NotEmpty(t, resp.Data.Clipboard)
		assert.NotNil(t, resp.Data.Toast)
		assert.Equal(t, "success", resp.Data.Toast.Type)
	})

	t.Run("InactiveAction", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		// Create an inactive action
		action := createTestCustomAction(t, app, org.ID, "Disabled Action", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook"}, false, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": contact.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": uuid.New().String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("InvalidContactID", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "Test Action", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": "not-a-valid-uuid",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	})

	t.Run("ContactNotFound", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "Test Action", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": uuid.New().String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("CrossOrgIsolation", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))

		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID)
		contact2 := testutil.CreateTestContact(t, app.DB, org2.ID)

		// Create action in org1
		action := createTestCustomAction(t, app, org1.ID, "Org1 Action", models.ActionTypeWebhook,
			map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

		// User from org2 tries to execute org1's action
		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": contact2.ID.String(),
		})
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("Unauthorized", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": uuid.New().String(),
		})
		// No auth context
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
	})

	t.Run("WebhookServerError", func(t *testing.T) {
		t.Parallel()

		// Create a mock server that returns 500
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"error":"internal server error"}`)
		}))
		defer server.Close()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "Failing Webhook", models.ActionTypeWebhook,
			map[string]interface{}{"url": server.URL, "method": "POST"}, true, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": contact.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.ActionResult `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		// Webhook returns non-2xx but handler still returns 200 with success=false
		assert.False(t, resp.Data.Success)
		assert.Contains(t, resp.Data.Message, "500")
		assert.NotNil(t, resp.Data.Toast)
		assert.Equal(t, "error", resp.Data.Toast.Type)
	})

	t.Run("WebhookWithVariableReplacement", func(t *testing.T) {
		t.Parallel()

		var receivedURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedURL = r.URL.String()
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		}))
		defer server.Close()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		// Use a URL with variable template
		action := createTestCustomAction(t, app, org.ID, "Variable Webhook", models.ActionTypeWebhook,
			map[string]interface{}{
				"url":    server.URL + "/contact/{{contact.id}}",
				"method": "GET",
			}, true, 0)

		req := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": contact.ID.String(),
		})
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", action.ID.String())

		err := app.ExecuteCustomAction(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		// Verify the variable was replaced in the URL
		assert.Contains(t, receivedURL, contact.ID.String())
	})
}

// --- CustomActionRedirect Tests ---

func TestApp_CustomActionRedirect(t *testing.T) {
	t.Parallel()

	t.Run("Success_ViaExecuteURL", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "Open URL", models.ActionTypeURL,
			map[string]interface{}{
				"url": "https://crm.example.com/contact/view",
			}, true, 0)

		// Execute the URL action to create a redirect token
		execReq := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": contact.ID.String(),
		})
		testutil.SetAuthContext(execReq, org.ID, user.ID)
		testutil.SetPathParam(execReq, "id", action.ID.String())

		err := app.ExecuteCustomAction(execReq)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(execReq))

		var execResp struct {
			Data handlers.ActionResult `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(execReq), &execResp)
		require.NoError(t, err)
		require.NotEmpty(t, execResp.Data.RedirectURL)

		// Extract the token from the redirect URL
		// Format: /api/custom-actions/redirect/<token>
		tokenStart := len("/api/custom-actions/redirect/")
		require.Greater(t, len(execResp.Data.RedirectURL), tokenStart)
		token := execResp.Data.RedirectURL[tokenStart:]

		// Now use the token with CustomActionRedirect
		redirectReq := testutil.NewGETRequest(t)
		testutil.SetPathParam(redirectReq, "token", token)

		err = app.CustomActionRedirect(redirectReq)
		require.NoError(t, err)

		// Should redirect (302)
		assert.Equal(t, fasthttp.StatusFound, redirectReq.RequestCtx.Response.StatusCode())
		location := string(redirectReq.RequestCtx.Response.Header.Peek("Location"))
		assert.Equal(t, "https://crm.example.com/contact/view", location)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t)

		req := testutil.NewGETRequest(t)
		testutil.SetPathParam(req, "token", "nonexistent-token-12345")

		err := app.CustomActionRedirect(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("TokenIsOneTimeUse", func(t *testing.T) {
		t.Parallel()

		app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
		org := testutil.CreateTestOrganization(t, app.DB)
		user := testutil.CreateTestUser(t, app.DB, org.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)

		action := createTestCustomAction(t, app, org.ID, "One-Time URL", models.ActionTypeURL,
			map[string]interface{}{
				"url": "https://crm.example.com/secret",
			}, true, 0)

		// Execute to get redirect token
		execReq := testutil.NewJSONRequest(t, map[string]any{
			"contact_id": contact.ID.String(),
		})
		testutil.SetAuthContext(execReq, org.ID, user.ID)
		testutil.SetPathParam(execReq, "id", action.ID.String())

		err := app.ExecuteCustomAction(execReq)
		require.NoError(t, err)

		var execResp struct {
			Data handlers.ActionResult `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(execReq), &execResp)
		require.NoError(t, err)
		token := execResp.Data.RedirectURL[len("/api/custom-actions/redirect/"):]

		// First use should succeed
		req1 := testutil.NewGETRequest(t)
		testutil.SetPathParam(req1, "token", token)
		err = app.CustomActionRedirect(req1)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusFound, req1.RequestCtx.Response.StatusCode())

		// Second use should fail (token consumed)
		req2 := testutil.NewGETRequest(t)
		testutil.SetPathParam(req2, "token", token)
		err = app.CustomActionRedirect(req2)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req2))
	})
}

// --- CreateCustomAction with missing config ---

func TestApp_CreateCustomAction_MissingConfig(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// Config is nil / not provided
	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "No Config",
		"action_type": "webhook",
		"is_active":   true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCustomAction(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- CreateCustomAction duplicate name (should succeed - names are not unique) ---

func TestApp_CreateCustomAction_DuplicateName(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	createTestCustomAction(t, app, org.ID, "Same Name", models.ActionTypeWebhook,
		map[string]interface{}{"url": "https://example.com/hook1"}, true, 0)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":        "Same Name",
		"action_type": "webhook",
		"config": map[string]any{
			"url": "https://example.com/hook2",
		},
		"is_active": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateCustomAction(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Should now have 2 actions with the same name
	var count int64
	app.DB.Model(&models.CustomAction{}).Where("organization_id = ? AND name = ?", org.ID, "Same Name").Count(&count)
	assert.Equal(t, int64(2), count)
}

// --- UpdateCustomAction change action type and config ---

func TestApp_UpdateCustomAction_ChangeActionType(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	action := createTestCustomAction(t, app, org.ID, "Convert Action", models.ActionTypeWebhook,
		map[string]interface{}{"url": "https://example.com/hook"}, true, 0)

	// Change from webhook to javascript
	req := testutil.NewJSONRequest(t, map[string]any{
		"action_type": "javascript",
		"config": map[string]any{
			"code": "console.log('hello')",
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
	assert.Equal(t, models.ActionTypeJavascript, resp.Data.ActionType)
	assert.Equal(t, "console.log('hello')", resp.Data.Config["code"])
}

// --- ListCustomActions Unauthorized ---

func TestApp_ListCustomActions_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context

	err := app.ListCustomActions(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- GetCustomAction Unauthorized ---

func TestApp_GetCustomAction_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetCustomAction(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- DeleteCustomAction Unauthorized ---

func TestApp_DeleteCustomAction_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteCustomAction(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- UpdateCustomAction Unauthorized ---

func TestApp_UpdateCustomAction_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":      "Updated",
		"is_active": true,
	})
	// No auth context
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateCustomAction(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}
