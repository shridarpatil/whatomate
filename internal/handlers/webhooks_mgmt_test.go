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

// createTestWebhook is a test helper that inserts a Webhook directly into the DB.
func createTestWebhook(t *testing.T, app *handlers.App, orgID uuid.UUID, name, url string, events []string) *models.Webhook {
	t.Helper()
	wh := &models.Webhook{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           name,
		URL:            url,
		Events:         events,
		Headers:        models.JSONB{"X-Custom": "value"},
		Secret:         "test-secret",
		IsActive:       true,
	}
	require.NoError(t, app.DB.Create(wh).Error)
	return wh
}

// --- ListWebhooks Tests ---

func TestApp_ListWebhooks_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	wh1 := createTestWebhook(t, app, org.ID, "Webhook A", "https://example.com/a", []string{"message.incoming"})
	wh2 := createTestWebhook(t, app, org.ID, "Webhook B", "https://example.com/b", []string{"message.sent", "contact.created"})

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListWebhooks(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Webhooks       []handlers.WebhookResponse `json:"webhooks"`
			AvailableEvents []map[string]string        `json:"available_events"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Webhooks, 2)

	// Ordered by created_at DESC, so wh2 first
	assert.Equal(t, wh2.ID, resp.Data.Webhooks[0].ID)
	assert.Equal(t, wh1.ID, resp.Data.Webhooks[1].ID)

	// Verify available_events is returned
	assert.NotEmpty(t, resp.Data.AvailableEvents)
}

func TestApp_ListWebhooks_Empty(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListWebhooks(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Webhooks []handlers.WebhookResponse `json:"webhooks"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Webhooks, 0)
}

func TestApp_ListWebhooks_OrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	createTestWebhook(t, app, org1.ID, "Org1 Hook", "https://example.com/org1", []string{"message.incoming"})
	createTestWebhook(t, app, org1.ID, "Org1 Hook 2", "https://example.com/org1b", []string{"message.sent"})
	createTestWebhook(t, app, org2.ID, "Org2 Hook", "https://example.com/org2", []string{"contact.created"})

	// org1 should see 2
	req1 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req1, org1.ID, user1.ID)
	err := app.ListWebhooks(req1)
	require.NoError(t, err)

	var resp1 struct {
		Data struct {
			Webhooks []handlers.WebhookResponse `json:"webhooks"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req1), &resp1)
	require.NoError(t, err)
	assert.Len(t, resp1.Data.Webhooks, 2)

	// org2 should see 1
	req2 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req2, org2.ID, user2.ID)
	err = app.ListWebhooks(req2)
	require.NoError(t, err)

	var resp2 struct {
		Data struct {
			Webhooks []handlers.WebhookResponse `json:"webhooks"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req2), &resp2)
	require.NoError(t, err)
	assert.Len(t, resp2.Data.Webhooks, 1)
}

func TestApp_ListWebhooks_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context

	err := app.ListWebhooks(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- GetWebhook Tests ---

func TestApp_GetWebhook_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	wh := createTestWebhook(t, app, org.ID, "My Hook", "https://example.com/hook", []string{"message.incoming", "message.sent"})

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.GetWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.WebhookResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, wh.ID, resp.Data.ID)
	assert.Equal(t, "My Hook", resp.Data.Name)
	assert.Equal(t, "https://example.com/hook", resp.Data.URL)
	assert.ElementsMatch(t, []string{"message.incoming", "message.sent"}, resp.Data.Events)
	assert.True(t, resp.Data.IsActive)
	assert.True(t, resp.Data.HasSecret) // webhook has a secret
	assert.Equal(t, "value", resp.Data.Headers["X-Custom"])
}

func TestApp_GetWebhook_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetWebhook_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetWebhook_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	wh := createTestWebhook(t, app, org1.ID, "Org1 Only", "https://example.com/private", []string{"message.incoming"})

	// User from org2 tries to access org1's webhook
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.GetWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- CreateWebhook Tests ---

func TestApp_CreateWebhook_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":      "Production Hook",
		"url":       "https://api.example.com/webhook",
		"events":    []string{"message.incoming", "contact.created"},
		"headers":   map[string]string{"Authorization": "Bearer tok123"},
		"secret":    "my-secret",
		"is_active": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.WebhookResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Production Hook", resp.Data.Name)
	assert.Equal(t, "https://api.example.com/webhook", resp.Data.URL)
	assert.ElementsMatch(t, []string{"message.incoming", "contact.created"}, resp.Data.Events)
	assert.Equal(t, "Bearer tok123", resp.Data.Headers["Authorization"])
	assert.True(t, resp.Data.IsActive)
	assert.True(t, resp.Data.HasSecret)
	assert.NotEqual(t, uuid.Nil, resp.Data.ID)

	// Verify persisted in database
	var dbWebhook models.Webhook
	require.NoError(t, app.DB.Where("id = ?", resp.Data.ID).First(&dbWebhook).Error)
	assert.Equal(t, "Production Hook", dbWebhook.Name)
	assert.Equal(t, org.ID, dbWebhook.OrganizationID)
	assert.Equal(t, "my-secret", dbWebhook.Secret)
}

func TestApp_CreateWebhook_MissingName(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"url":    "https://example.com/hook",
		"events": []string{"message.incoming"},
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateWebhook_MissingURL(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":   "My Hook",
		"events": []string{"message.incoming"},
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateWebhook_MissingEvents(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "My Hook",
		"url":  "https://example.com/hook",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateWebhook_EmptyEvents(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":   "My Hook",
		"url":    "https://example.com/hook",
		"events": []string{},
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_CreateWebhook_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":   "Hook",
		"url":    "https://example.com",
		"events": []string{"message.incoming"},
	})
	// No auth context

	err := app.CreateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- UpdateWebhook Tests ---

func TestApp_UpdateWebhook_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	wh := createTestWebhook(t, app, org.ID, "Old Name", "https://old.example.com", []string{"message.incoming"})

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":      "Updated Name",
		"url":       "https://new.example.com/hook",
		"events":    []string{"message.sent", "contact.created"},
		"headers":   map[string]string{"X-New-Header": "new-value"},
		"is_active": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.UpdateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.WebhookResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, wh.ID, resp.Data.ID)
	assert.Equal(t, "Updated Name", resp.Data.Name)
	assert.Equal(t, "https://new.example.com/hook", resp.Data.URL)
	assert.ElementsMatch(t, []string{"message.sent", "contact.created"}, resp.Data.Events)
	assert.Equal(t, "new-value", resp.Data.Headers["X-New-Header"])
	assert.True(t, resp.Data.IsActive)

	// Verify persisted
	var updated models.Webhook
	require.NoError(t, app.DB.Where("id = ?", wh.ID).First(&updated).Error)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "https://new.example.com/hook", updated.URL)
}

func TestApp_UpdateWebhook_PartialUpdate(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	wh := createTestWebhook(t, app, org.ID, "Original", "https://original.example.com", []string{"message.incoming"})

	// Only update the name
	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":      "Only Name Changed",
		"is_active": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.UpdateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.WebhookResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Only Name Changed", resp.Data.Name)
	// Original values preserved
	assert.Equal(t, "https://original.example.com", resp.Data.URL)
	assert.ElementsMatch(t, []string{"message.incoming"}, resp.Data.Events)
}

func TestApp_UpdateWebhook_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":      "Updated",
		"is_active": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateWebhook_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":      "Updated",
		"is_active": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.UpdateWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- DeleteWebhook Tests ---

func TestApp_DeleteWebhook_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	wh := createTestWebhook(t, app, org.ID, "To Delete", "https://example.com/delete", []string{"message.incoming"})

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.DeleteWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Webhook deleted successfully", resp.Data.Message)

	// Verify soft-deleted
	var count int64
	app.DB.Model(&models.Webhook{}).Where("id = ?", wh.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteWebhook_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteWebhook_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.DeleteWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteWebhook_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	wh := createTestWebhook(t, app, org1.ID, "Org1 Hook", "https://example.com/org1", []string{"message.incoming"})

	// User from org2 tries to delete org1's webhook
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.DeleteWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	// Verify the webhook still exists in org1
	var count int64
	app.DB.Model(&models.Webhook{}).Where("id = ?", wh.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

// --- TestWebhook Tests ---

func TestApp_TestWebhook_Success(t *testing.T) {
	t.Parallel()

	// Start a mock HTTP server that accepts webhook posts
	var receivedBody []byte
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		receivedBody = buf
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	wh := createTestWebhook(t, app, org.ID, "Test Hook", server.URL, []string{"message.incoming"})

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.TestWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Test webhook sent successfully", resp.Data.Message)

	// Verify the mock server received the request
	require.NotNil(t, receivedHeaders)
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.Equal(t, "Whatomate-Webhook/1.0", receivedHeaders.Get("User-Agent"))
	// Custom header from webhook config
	assert.Equal(t, "value", receivedHeaders.Get("X-Custom"))
	// HMAC signature should be set since webhook has a secret
	assert.NotEmpty(t, receivedHeaders.Get("X-Webhook-Signature"))

	// Verify payload contains test event
	require.NotEmpty(t, receivedBody)
	var payload map[string]interface{}
	err = json.Unmarshal(receivedBody, &payload)
	require.NoError(t, err)
	assert.Equal(t, "test", payload["event"])
}

func TestApp_TestWebhook_ServerError(t *testing.T) {
	t.Parallel()

	// Mock server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, "Internal Server Error")
	}))
	defer server.Close()

	app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	wh := createTestWebhook(t, app, org.ID, "Failing Hook", server.URL, []string{"message.incoming"})

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.TestWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadGateway, testutil.GetResponseStatusCode(req))
}

func TestApp_TestWebhook_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, withHTTPClient(&http.Client{Timeout: 5 * time.Second}))
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.TestWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_TestWebhook_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, nil)
	// No auth context

	err := app.TestWebhook(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- webhookToResponse Tests ---

func TestWebhookToResponse_HasSecretTrue(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// Create a webhook with a secret
	wh := createTestWebhook(t, app, org.ID, "Secret Hook", "https://example.com/secret", []string{"message.incoming"})

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.GetWebhook(req)
	require.NoError(t, err)

	var resp struct {
		Data handlers.WebhookResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Data.HasSecret, "webhook with secret should have has_secret=true")
}

func TestWebhookToResponse_HasSecretFalse(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// Create a webhook without a secret
	wh := &models.Webhook{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "No Secret Hook",
		URL:            "https://example.com/nosecret",
		Events:         []string{"message.incoming"},
		Headers:        models.JSONB{},
		Secret:         "",
		IsActive:       true,
	}
	require.NoError(t, app.DB.Create(wh).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", wh.ID.String())

	err := app.GetWebhook(req)
	require.NoError(t, err)

	var resp struct {
		Data handlers.WebhookResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Data.HasSecret, "webhook without secret should have has_secret=false")
}
