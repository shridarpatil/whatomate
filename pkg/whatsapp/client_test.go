package whatsapp_test

import (
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

// testAccount returns a test WhatsApp account configured to use the test server.
func testAccount(serverURL string) *whatsapp.Account {
	return &whatsapp.Account{
		PhoneID:     "123456789",
		BusinessID:  "987654321",
		APIVersion:  "v21.0",
		AccessToken: "test-access-token",
	}
}

func TestClient_SendTextMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		phone           string
		text            string
		serverResponse  func(t *testing.T, w http.ResponseWriter, r *http.Request)
		wantMessageID   string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:  "successful send",
			phone: "1234567890",
			text:  "Hello, World!",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Contains(t, r.URL.Path, "/messages")

				// Verify headers
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))

				// Verify body
				var body map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&body)
				require.NoError(t, err)
				assert.Equal(t, "whatsapp", body["messaging_product"])
				assert.Equal(t, "1234567890", body["to"])
				assert.Equal(t, "text", body["type"])

				textContent := body["text"].(map[string]interface{})
				assert.Equal(t, "Hello, World!", textContent["body"])

				// Return success
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"messages": []map[string]string{{"id": "wamid.test123"}},
				})
			},
			wantMessageID: "wamid.test123",
			wantErr:       false,
		},
		{
			name:  "API error - invalid phone",
			phone: "invalid",
			text:  "Hello",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
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
						Message: "Invalid phone number format",
						Code:    100,
					},
				})
			},
			wantErr:         true,
			wantErrContains: "Invalid phone number format",
		},
		{
			name:  "API error - unauthorized",
			phone: "1234567890",
			text:  "Hello",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
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
			},
			wantErr:         true,
			wantErrContains: "Invalid access token",
		},
		{
			name:  "empty message ID in response",
			phone: "1234567890",
			text:  "Hello",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"messages": []map[string]string{}, // Empty
				})
			},
			wantErr:         true,
			wantErrContains: "no message ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.serverResponse(t, w, r)
			}))
			defer server.Close()

			// Create client with custom HTTP client that redirects to test server
			log := testutil.NopLogger()
			client := whatsapp.NewWithTimeout(log, 5*time.Second)

			// Override HTTP client to use test server
			client.HTTPClient = &http.Client{
				Transport: &testServerTransport{serverURL: server.URL},
			}

			account := testAccount(server.URL)
			ctx := testutil.TestContext(t)

			msgID, err := client.SendTextMessage(ctx, account, tt.phone, tt.text)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantMessageID, msgID)
		})
	}
}

func TestClient_GetMediaURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		mediaID         string
		serverResponse  func(t *testing.T, w http.ResponseWriter, r *http.Request)
		wantURL         string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:    "successful get",
			mediaID: "media123",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "media123")

				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(whatsapp.MediaURLResponse{
					URL:      "https://cdn.whatsapp.net/media/test123",
					MimeType: "image/jpeg",
				})
			},
			wantURL: "https://cdn.whatsapp.net/media/test123",
			wantErr: false,
		},
		{
			name:    "media not found",
			mediaID: "nonexistent",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
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
						Message: "Media not found",
						Code:    100,
					},
				})
			},
			wantErr:         true,
			wantErrContains: "Media not found",
		},
		{
			name:    "empty URL in response",
			mediaID: "media123",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(whatsapp.MediaURLResponse{
					URL: "", // Empty URL
				})
			},
			wantErr:         true,
			wantErrContains: "no URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.serverResponse(t, w, r)
			}))
			defer server.Close()

			log := testutil.NopLogger()
			client := whatsapp.NewWithTimeout(log, 5*time.Second)
			client.HTTPClient = &http.Client{
				Transport: &testServerTransport{serverURL: server.URL},
			}

			account := testAccount(server.URL)
			ctx := testutil.TestContext(t)

			url, err := client.GetMediaURL(ctx, tt.mediaID, account)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, url)
		})
	}
}

func TestClient_DownloadMedia(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverResponse func(t *testing.T, w http.ResponseWriter, r *http.Request)
		wantData       []byte
		wantErr        bool
	}{
		{
			name: "successful download",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("fake image data"))
			},
			wantData: []byte("fake image data"),
			wantErr:  false,
		},
		{
			name: "download failed",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.serverResponse(t, w, r)
			}))
			defer server.Close()

			log := testutil.NopLogger()
			client := whatsapp.NewWithTimeout(log, 5*time.Second)

			ctx := testutil.TestContext(t)

			data, err := client.DownloadMedia(ctx, server.URL+"/media/test", "test-access-token")

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantData, data)
		})
	}
}

func TestClient_MarkMessageRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		messageID      string
		serverResponse func(t *testing.T, w http.ResponseWriter, r *http.Request)
		wantErr        bool
	}{
		{
			name:      "successful mark read",
			messageID: "wamid.test123",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)

				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, "read", body["status"])
				assert.Equal(t, "wamid.test123", body["message_id"])

				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			},
			wantErr: false,
		},
		{
			name:      "message not found",
			messageID: "wamid.invalid",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.serverResponse(t, w, r)
			}))
			defer server.Close()

			log := testutil.NopLogger()
			client := whatsapp.NewWithTimeout(log, 5*time.Second)
			client.HTTPClient = &http.Client{
				Transport: &testServerTransport{serverURL: server.URL},
			}

			account := testAccount(server.URL)
			ctx := testutil.TestContext(t)

			err := client.MarkMessageRead(ctx, account, tt.messageID)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestClient_SendImageMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		assert.Equal(t, "image", body["type"])
		image := body["image"].(map[string]interface{})
		assert.Equal(t, "media123", image["id"])
		assert.Equal(t, "Test caption", image["caption"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"messages": []map[string]string{{"id": "wamid.img123"}},
		})
	}))
	defer server.Close()

	log := testutil.NopLogger()
	client := whatsapp.NewWithTimeout(log, 5*time.Second)
	client.HTTPClient = &http.Client{
		Transport: &testServerTransport{serverURL: server.URL},
	}

	account := testAccount(server.URL)
	ctx := testutil.TestContext(t)

	msgID, err := client.SendImageMessage(ctx, account, "1234567890", "media123", "Test caption")

	require.NoError(t, err)
	assert.Equal(t, "wamid.img123", msgID)
}

func TestClient_SendDocumentMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		assert.Equal(t, "document", body["type"])
		doc := body["document"].(map[string]interface{})
		assert.Equal(t, "media456", doc["id"])
		assert.Equal(t, "report.pdf", doc["filename"])
		assert.Equal(t, "Monthly report", doc["caption"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"messages": []map[string]string{{"id": "wamid.doc123"}},
		})
	}))
	defer server.Close()

	log := testutil.NopLogger()
	client := whatsapp.NewWithTimeout(log, 5*time.Second)
	client.HTTPClient = &http.Client{
		Transport: &testServerTransport{serverURL: server.URL},
	}

	account := testAccount(server.URL)
	ctx := testutil.TestContext(t)

	msgID, err := client.SendDocumentMessage(ctx, account, "1234567890", "media456", "report.pdf", "Monthly report")

	require.NoError(t, err)
	assert.Equal(t, "wamid.doc123", msgID)
}

// testServerTransport redirects all requests to the test server
type testServerTransport struct {
	serverURL string
}

func (t *testServerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the host with test server
	testReq := req.Clone(req.Context())
	testReq.URL.Scheme = "http"
	testReq.URL.Host = t.serverURL[7:] // Remove "http://"
	return http.DefaultTransport.RoundTrip(testReq)
}
