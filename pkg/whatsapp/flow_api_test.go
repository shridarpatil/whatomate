package whatsapp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CreateFlow ---

func TestClient_CreateFlow_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/flows")

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Test Flow", body["name"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "flow-123"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	id, err := client.CreateFlow(context.Background(), account, "Test Flow", []string{"OTHER"})
	require.NoError(t, err)
	assert.Equal(t, "flow-123", id)
}

func TestClient_CreateFlow_APIError(t *testing.T) {
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
				Message: "Invalid flow name",
				Code:    100,
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	_, err := client.CreateFlow(context.Background(), account, "", nil)
	require.Error(t, err)
}

// --- PublishFlow ---

func TestClient_PublishFlow_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/publish")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.PublishFlow(context.Background(), account, "flow-123")
	require.NoError(t, err)
}

func TestClient_PublishFlow_Failure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": false})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.PublishFlow(context.Background(), account, "flow-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to publish")
}

// --- DeprecateFlow ---

func TestClient_DeprecateFlow_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/deprecate")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.DeprecateFlow(context.Background(), account, "flow-123")
	require.NoError(t, err)
}

func TestClient_DeprecateFlow_Failure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": false})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.DeprecateFlow(context.Background(), account, "flow-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deprecate")
}

// --- DeleteFlow ---

func TestClient_DeleteFlow_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.DeleteFlow(context.Background(), account, "flow-123")
	require.NoError(t, err)
}

func TestClient_DeleteFlow_APIError(t *testing.T) {
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
				Message: "Flow not found",
				Code:    100,
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	err := client.DeleteFlow(context.Background(), account, "nonexistent")
	require.Error(t, err)
}

// --- GetFlow ---

func TestClient_GetFlow_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         "flow-123",
			"name":       "Test Flow",
			"status":     "DRAFT",
			"categories": []string{"OTHER"},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	flow, err := client.GetFlow(context.Background(), account, "flow-123")
	require.NoError(t, err)
	assert.Equal(t, "flow-123", flow.ID)
	assert.Equal(t, "Test Flow", flow.Name)
	assert.Equal(t, "DRAFT", flow.Status)
}

// --- ListFlows ---

func TestClient_ListFlows_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/flows")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "f1", "name": "Flow 1", "status": "DRAFT"},
				{"id": "f2", "name": "Flow 2", "status": "PUBLISHED"},
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	flows, err := client.ListFlows(context.Background(), account)
	require.NoError(t, err)
	require.Len(t, flows, 2)
	assert.Equal(t, "Flow 1", flows[0].Name)
	assert.Equal(t, "PUBLISHED", flows[1].Status)
}

func TestClient_ListFlows_Empty(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	flows, err := client.ListFlows(context.Background(), account)
	require.NoError(t, err)
	assert.Empty(t, flows)
}

// --- UpdateFlowJSON ---

func TestClient_UpdateFlowJSON_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/assets")
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	flowJSON := &whatsapp.FlowJSON{
		Version: "3.0",
		Screens: []interface{}{
			map[string]interface{}{
				"id":    "WELCOME",
				"title": "Welcome",
			},
		},
	}

	err := client.UpdateFlowJSON(context.Background(), account, "flow-123", flowJSON)
	require.NoError(t, err)
}

func TestClient_UpdateFlowJSON_ValidationError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":           false,
			"validation_errors": "Invalid screen layout",
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	account := testAccount(server.URL)

	flowJSON := &whatsapp.FlowJSON{Version: "3.0", Screens: []interface{}{}}

	err := client.UpdateFlowJSON(context.Background(), account, "flow-123", flowJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation errors")
}
