package whatsapp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ExchangeCodeForToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		code            string
		appID           string
		appSecret       string
		apiVersion      string
		serverResponse  func(t *testing.T, w http.ResponseWriter, r *http.Request)
		wantToken       string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:       "successful token exchange",
			code:       "test_auth_code_123",
			appID:      "123456",
			appSecret:  "secret123",
			apiVersion: "v21.0",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/oauth/access_token")
				assert.Contains(t, r.URL.RawQuery, "client_id=123456")
				assert.Contains(t, r.URL.RawQuery, "client_secret=secret123")
				assert.Contains(t, r.URL.RawQuery, "code=test_auth_code_123")

				// Return success
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"access_token": "EAABwzLixnjYBO1234567890",
					"token_type":   "bearer",
				})
			},
			wantToken: "EAABwzLixnjYBO1234567890",
			wantErr:   false,
		},
		{
			name:       "invalid authorization code",
			code:       "invalid_code",
			appID:      "123456",
			appSecret:  "secret123",
			apiVersion: "v21.0",
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
						Message: "Invalid authorization code",
						Code:    100,
					},
				})
			},
			wantErr:         true,
			wantErrContains: "Invalid authorization code",
		},
		{
			name:       "empty access token in response",
			code:       "test_code",
			appID:      "123456",
			appSecret:  "secret123",
			apiVersion: "v21.0",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"access_token": "", // Empty!
				})
			},
			wantErr:         true,
			wantErrContains: "no access token",
		},
		{
			name:       "expired code",
			code:       "expired_code",
			appID:      "123456",
			appSecret:  "secret123",
			apiVersion: "v21.0",
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
						Message: "Code has expired",
						Code:    100,
					},
				})
			},
			wantErr:         true,
			wantErrContains: "Code has expired",
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
			client := whatsapp.NewWithBaseURL(log, server.URL)

			ctx := testutil.TestContext(t)

			token, err := client.ExchangeCodeForToken(ctx, tt.code, tt.appID, tt.appSecret, tt.apiVersion)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestClient_GetPhoneNumberInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		phoneID         string
		serverResponse  func(t *testing.T, w http.ResponseWriter, r *http.Request)
		wantInfo        *whatsapp.PhoneNumberInfo
		wantErr         bool
		wantErrContains string
	}{
		{
			name:    "successful fetch",
			phoneID: "123456789",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "123456789")
				assert.Contains(t, r.URL.RawQuery, "fields=verified_name,display_phone_number,quality_rating")
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"verified_name":        "Test Business",
					"display_phone_number": "+1234567890",
					"quality_rating":       "GREEN",
				})
			},
			wantInfo: &whatsapp.PhoneNumberInfo{
				VerifiedName:       "Test Business",
				DisplayPhoneNumber: "+1234567890",
				QualityRating:      "GREEN",
			},
			wantErr: false,
		},
		{
			name:    "phone not found",
			phoneID: "nonexistent",
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
						Message: "Phone number not found",
						Code:    100,
					},
				})
			},
			wantErr:         true,
			wantErrContains: "Phone number not found",
		},
		{
			name:    "invalid access token",
			phoneID: "123456789",
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
						Message: "Invalid OAuth access token",
						Code:    190,
					},
				})
			},
			wantErr:         true,
			wantErrContains: "Invalid OAuth access token",
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
			client := whatsapp.NewWithBaseURL(log, server.URL)

			ctx := testutil.TestContext(t)

			info, err := client.GetPhoneNumberInfo(ctx, tt.phoneID, "test-token", "v21.0")

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantInfo, info)
		})
	}
}

func TestClient_RegisterPhoneNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		phoneID         string
		pin             string
		serverResponse  func(t *testing.T, w http.ResponseWriter, r *http.Request)
		wantErr         bool
		wantErrContains string
	}{
		{
			name:    "successful registration",
			phoneID: "123456789",
			pin:     "123456",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Contains(t, r.URL.Path, "123456789/register")
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

				var body map[string]string
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, "whatsapp", body["messaging_product"])
				assert.Equal(t, "123456", body["pin"])

				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			},
			wantErr: false,
		},
		{
			name:    "PIN already exists",
			phoneID: "123456789",
			pin:     "654321",
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
						Message: "Two-step verification is already enabled for this phone number",
						Code:    33,
					},
				})
			},
			wantErr:         true,
			wantErrContains: "Two-step verification is already enabled",
		},
		{
			name:    "invalid PIN format",
			phoneID: "123456789",
			pin:     "abc",
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
						Message: "PIN must be 6 digits",
						Code:    100,
					},
				})
			},
			wantErr:         true,
			wantErrContains: "PIN must be 6 digits",
		},
		{
			name:    "phone not verified",
			phoneID: "123456789",
			pin:     "123456",
			serverResponse: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
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
						Message: "Phone number must be verified before registration",
						Code:    368,
					},
				})
			},
			wantErr:         true,
			wantErrContains: "Phone number must be verified before registration",
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
			client := whatsapp.NewWithBaseURL(log, server.URL)

			ctx := testutil.TestContext(t)

			err := client.RegisterPhoneNumber(ctx, tt.phoneID, tt.pin, "test-token", "v21.0")

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}
