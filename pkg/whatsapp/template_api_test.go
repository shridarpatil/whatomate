package whatsapp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, server *httptest.Server) *whatsapp.Client {
	t.Helper()
	log := testutil.NopLogger()
	client := whatsapp.NewWithTimeout(log, 5*time.Second)
	client.HTTPClient = &http.Client{
		Transport: &testServerTransport{serverURL: server.URL},
	}
	return client
}

// --- SubmitTemplate ---

func TestClient_SubmitTemplate_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/message_templates")
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))

		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "hello_world", body["name"])
		assert.Equal(t, "en", body["language"])
		assert.Equal(t, "MARKETING", body["category"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "tmpl-123"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)
	ctx := context.Background()

	tmpl := &whatsapp.TemplateSubmission{
		Name:        "hello_world",
		Language:    "en",
		Category:    "MARKETING",
		BodyContent: "Hello! Welcome to our service.",
	}

	id, err := client.SubmitTemplate(ctx, account, tmpl)
	require.NoError(t, err)
	assert.Equal(t, "tmpl-123", id)
}

func TestClient_SubmitTemplate_WithVariables(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		components := body["components"].([]interface{})
		// Should have BODY component
		var bodyComp map[string]interface{}
		for _, c := range components {
			comp := c.(map[string]interface{})
			if comp["type"] == "BODY" {
				bodyComp = comp
			}
		}
		require.NotNil(t, bodyComp)
		assert.Equal(t, "Hello {{1}}! Your order {{2}} is ready.", bodyComp["text"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "tmpl-456"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)
	ctx := context.Background()

	tmpl := &whatsapp.TemplateSubmission{
		Name:        "order_ready",
		Language:    "en",
		Category:    "UTILITY",
		BodyContent: "Hello {{1}}! Your order {{2}} is ready.",
		SampleValues: []interface{}{
			map[string]interface{}{"component": "body", "index": 1, "value": "John"},
			map[string]interface{}{"component": "body", "index": 2, "value": "ORD-123"},
		},
	}

	id, err := client.SubmitTemplate(ctx, account, tmpl)
	require.NoError(t, err)
	assert.Equal(t, "tmpl-456", id)
}

func TestClient_SubmitTemplate_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(whatsapp.MetaAPIError{
			Error: struct {
				Message      string `json:"message"`
				Type         string `json:"type"`
				Code         int    `json:"code"`
				ErrorSubcode int    `json:"error_subcode"`
				ErrorUserMsg string `json:"error_user_msg"`
				ErrorData    struct {
					Details string `json:"details"`
				} `json:"error_data"`
				FBTraceID string `json:"fbtrace_id"`
			}{
				Message: "Invalid template name",
				Code:    100,
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)
	ctx := context.Background()

	tmpl := &whatsapp.TemplateSubmission{
		Name:        "",
		Language:    "en",
		Category:    "MARKETING",
		BodyContent: "Hello",
	}

	_, err := client.SubmitTemplate(ctx, account, tmpl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid template name")
}

func TestClient_SubmitTemplate_MissingVariableSamples(t *testing.T) {
	t.Parallel()

	// SubmitTemplate should fail if body has variables but no samples
	log := testutil.NopLogger()
	client := whatsapp.NewWithTimeout(log, 5*time.Second)

	account := &whatsapp.Account{
		PhoneID:     "123",
		BusinessID:  "456",
		APIVersion:  "v21.0",
		AccessToken: "token",
	}

	tmpl := &whatsapp.TemplateSubmission{
		Name:         "test",
		Language:     "en",
		Category:     "UTILITY",
		BodyContent:  "Hello {{1}}!",
		SampleValues: []interface{}{}, // Empty samples
	}

	_, err := client.SubmitTemplate(context.Background(), account, tmpl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sample values are required")
}

// --- FetchTemplates ---

func TestClient_FetchTemplates_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/message_templates")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "1", "name": "hello", "language": "en", "category": "MARKETING", "status": "APPROVED"},
				{"id": "2", "name": "goodbye", "language": "en", "category": "UTILITY", "status": "PENDING"},
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	templates, err := client.FetchTemplates(context.Background(), account)
	require.NoError(t, err)
	require.Len(t, templates, 2)
	assert.Equal(t, "hello", templates[0].Name)
	assert.Equal(t, "APPROVED", templates[0].Status)
	assert.Equal(t, "goodbye", templates[1].Name)
}

func TestClient_FetchTemplates_Empty(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	templates, err := client.FetchTemplates(context.Background(), account)
	require.NoError(t, err)
	assert.Empty(t, templates)
}

func TestClient_FetchTemplates_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(whatsapp.MetaAPIError{
			Error: struct {
				Message      string `json:"message"`
				Type         string `json:"type"`
				Code         int    `json:"code"`
				ErrorSubcode int    `json:"error_subcode"`
				ErrorUserMsg string `json:"error_user_msg"`
				ErrorData    struct {
					Details string `json:"details"`
				} `json:"error_data"`
				FBTraceID string `json:"fbtrace_id"`
			}{
				Message: "Invalid access token",
				Code:    190,
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	_, err := client.FetchTemplates(context.Background(), account)
	require.Error(t, err)
}

// --- DeleteTemplate ---

func TestClient_DeleteTemplate_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Contains(t, r.URL.Path, "/message_templates")
		assert.Contains(t, r.URL.RawQuery, "name=hello_world")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.DeleteTemplate(context.Background(), account, "hello_world")
	require.NoError(t, err)
}

func TestClient_DeleteTemplate_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(whatsapp.MetaAPIError{
			Error: struct {
				Message      string `json:"message"`
				Type         string `json:"type"`
				Code         int    `json:"code"`
				ErrorSubcode int    `json:"error_subcode"`
				ErrorUserMsg string `json:"error_user_msg"`
				ErrorData    struct {
					Details string `json:"details"`
				} `json:"error_data"`
				FBTraceID string `json:"fbtrace_id"`
			}{
				Message: "Template not found",
				Code:    100,
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.DeleteTemplate(context.Background(), account, "nonexistent")
	require.Error(t, err)
}
