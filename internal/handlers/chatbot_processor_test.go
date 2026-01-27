package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRasaServer creates a mock Rasa server for testing.
type mockRasaServer struct {
	server       *httptest.Server
	responses    []map[string]interface{}
	receivedReqs []map[string]string
	returnError  bool
	errorStatus  int
	errorMessage string
}

func newMockRasaServer(responses []map[string]interface{}) *mockRasaServer {
	m := &mockRasaServer{
		responses:    responses,
		receivedReqs: make([]map[string]string, 0),
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.returnError {
			w.WriteHeader(m.errorStatus)
			_, _ = w.Write([]byte(m.errorMessage))
			return
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		m.receivedReqs = append(m.receivedReqs, req)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(m.responses)
	}))

	return m
}

func (m *mockRasaServer) close() {
	m.server.Close()
}

// processorTestAppMinimal creates a minimal App for testing functions that don't need DB/Redis.
// This is useful for testing HTTP client functions like generateRasaResponse.
func processorTestAppMinimal(t *testing.T) *handlers.App {
	t.Helper()

	log := testutil.NopLogger()
	cfg := &config.Config{}

	return &handlers.App{
		Config: cfg,
		Log:    log,
	}
}

func TestGenerateRasaResponse_Success(t *testing.T) {
	// Create mock Rasa server
	mockRasa := newMockRasaServer([]map[string]interface{}{
		{"recipient_id": "1234567890", "text": "Hello! How can I help you?"},
	})
	defer mockRasa.close()

	app := processorTestAppMinimal(t)

	settings := &models.ChatbotSettings{
		AI: models.AIConfig{
			Enabled:   true,
			Provider:  models.AIProviderRasa,
			ServerURL: mockRasa.server.URL,
			APIKey:    "NO-KEY",
		},
	}

	session := &models.ChatbotSession{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		PhoneNumber: "1234567890",
	}

	response, err := handlers.GenerateRasaResponseForTest(app, settings, session, "Hi there!")
	require.NoError(t, err)
	assert.Equal(t, "Hello! How can I help you?", response)

	// Verify request was sent correctly
	require.Len(t, mockRasa.receivedReqs, 1)
	assert.Equal(t, "1234567890", mockRasa.receivedReqs[0]["sender"])
	assert.Equal(t, "Hi there!", mockRasa.receivedReqs[0]["message"])
}

func TestGenerateRasaResponse_MultipleMessages(t *testing.T) {
	// Create mock Rasa server returning multiple messages
	mockRasa := newMockRasaServer([]map[string]interface{}{
		{"recipient_id": "1234567890", "text": "First response."},
		{"recipient_id": "1234567890", "text": "Second response."},
		{"recipient_id": "1234567890", "text": "Third response."},
	})
	defer mockRasa.close()

	app := processorTestAppMinimal(t)

	settings := &models.ChatbotSettings{
		AI: models.AIConfig{
			Enabled:   true,
			Provider:  models.AIProviderRasa,
			ServerURL: mockRasa.server.URL,
			APIKey:    "NO-KEY",
		},
	}

	session := &models.ChatbotSession{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		PhoneNumber: "1234567890",
	}

	response, err := handlers.GenerateRasaResponseForTest(app, settings, session, "Tell me more")
	require.NoError(t, err)
	assert.Equal(t, "First response.\n\nSecond response.\n\nThird response.", response)
}

func TestGenerateRasaResponse_MissingServerURL(t *testing.T) {
	app := processorTestAppMinimal(t)

	settings := &models.ChatbotSettings{
		AI: models.AIConfig{
			Enabled:   true,
			Provider:  models.AIProviderRasa,
			ServerURL: "", // Missing URL
			APIKey:    "NO-KEY",
		},
	}

	session := &models.ChatbotSession{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		PhoneNumber: "1234567890",
	}

	_, err := handlers.GenerateRasaResponseForTest(app, settings, session, "Hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server URL is not configured")
}

func TestGenerateRasaResponse_ServerError(t *testing.T) {
	mockRasa := newMockRasaServer(nil)
	mockRasa.returnError = true
	mockRasa.errorStatus = 500
	mockRasa.errorMessage = "Internal Server Error"
	defer mockRasa.close()

	app := processorTestAppMinimal(t)

	settings := &models.ChatbotSettings{
		AI: models.AIConfig{
			Enabled:   true,
			Provider:  models.AIProviderRasa,
			ServerURL: mockRasa.server.URL,
			APIKey:    "NO-KEY",
		},
	}

	session := &models.ChatbotSession{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		PhoneNumber: "1234567890",
	}

	_, err := handlers.GenerateRasaResponseForTest(app, settings, session, "Hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Rasa API error (status 500)")
}

func TestGenerateRasaResponse_EmptyResponse(t *testing.T) {
	mockRasa := newMockRasaServer([]map[string]interface{}{})
	defer mockRasa.close()

	app := processorTestAppMinimal(t)

	settings := &models.ChatbotSettings{
		AI: models.AIConfig{
			Enabled:   true,
			Provider:  models.AIProviderRasa,
			ServerURL: mockRasa.server.URL,
			APIKey:    "NO-KEY",
		},
	}

	session := &models.ChatbotSession{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		PhoneNumber: "1234567890",
	}

	_, err := handlers.GenerateRasaResponseForTest(app, settings, session, "Hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no response from Rasa")
}

func TestGenerateRasaResponse_NoTextInResponse(t *testing.T) {
	mockRasa := newMockRasaServer([]map[string]interface{}{
		{"recipient_id": "1234567890", "image": "http://example.com/image.png"},
	})
	defer mockRasa.close()

	app := processorTestAppMinimal(t)

	settings := &models.ChatbotSettings{
		AI: models.AIConfig{
			Enabled:   true,
			Provider:  models.AIProviderRasa,
			ServerURL: mockRasa.server.URL,
			APIKey:    "NO-KEY",
		},
	}

	session := &models.ChatbotSession{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		PhoneNumber: "1234567890",
	}

	_, err := handlers.GenerateRasaResponseForTest(app, settings, session, "Hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no text response from Rasa")
}

func TestGenerateRasaResponse_WithAuthToken(t *testing.T) {
	var receivedAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"recipient_id": "1234567890", "text": "Authenticated response"},
		})
	}))
	defer server.Close()

	app := processorTestAppMinimal(t)

	settings := &models.ChatbotSettings{
		AI: models.AIConfig{
			Enabled:   true,
			Provider:  models.AIProviderRasa,
			ServerURL: server.URL,
			APIKey:    "my-secret-token",
		},
	}

	session := &models.ChatbotSession{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		PhoneNumber: "1234567890",
	}

	response, err := handlers.GenerateRasaResponseForTest(app, settings, session, "Hello")
	require.NoError(t, err)
	assert.Equal(t, "Authenticated response", response)
	assert.Equal(t, "Bearer my-secret-token", receivedAuthHeader)
}

func TestGenerateRasaResponse_NoAuthHeaderWhenNoKey(t *testing.T) {
	var receivedAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"recipient_id": "1234567890", "text": "No auth response"},
		})
	}))
	defer server.Close()

	app := processorTestAppMinimal(t)

	settings := &models.ChatbotSettings{
		AI: models.AIConfig{
			Enabled:   true,
			Provider:  models.AIProviderRasa,
			ServerURL: server.URL,
			APIKey:    "", // Empty key
		},
	}

	session := &models.ChatbotSession{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		PhoneNumber: "1234567890",
	}

	response, err := handlers.GenerateRasaResponseForTest(app, settings, session, "Hello")
	require.NoError(t, err)
	assert.Equal(t, "No auth response", response)
	assert.Empty(t, receivedAuthHeader, "Should not send auth header when API key is empty")
}
