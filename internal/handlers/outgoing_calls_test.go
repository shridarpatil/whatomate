package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestApp_GetCallPermission(t *testing.T) {
	tests := []struct {
		name           string
		callingEnabled bool
		metaStatus     int
		metaResponse   string
		wantStatus     string
		wantRequests   int64
	}{
		{
			name:         "disabled account skips Meta",
			metaStatus:   http.StatusBadRequest,
			metaResponse: `{"error":{"message":"Authentication Error","code":190}}`,
			wantStatus:   "unknown",
			wantRequests: 0,
		},
		{
			name:           "Meta authentication error returns unknown",
			callingEnabled: true,
			metaStatus:     http.StatusBadRequest,
			metaResponse:   `{"error":{"message":"Authentication Error","code":190}}`,
			wantStatus:     "unknown",
			wantRequests:   1,
		},
		{
			name:           "enabled account preserves permission status",
			callingEnabled: true,
			metaStatus:     http.StatusOK,
			metaResponse:   `{"permission":{"status":"temporary"}}`,
			wantStatus:     "temporary",
			wantRequests:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp(t)
			org := testutil.CreateTestOrganization(t, app.DB)
			role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "call-reader", []string{"outgoing_calls:read"})
			user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
			account := testutil.CreateTestWhatsAppAccountWith(t, app.DB, org.ID, func(account *models.WhatsAppAccount) {
				account.BusinessCallingEnabled = tt.callingEnabled
			})
			contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
				testutil.WithContactAccount(account.Name),
				testutil.WithPhoneNumber("15551234567"),
			)

			var requests atomic.Int64
			meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/"+account.APIVersion+"/"+account.PhoneID+"/call_permissions", r.URL.Path)
				assert.Equal(t, contact.PhoneNumber, r.URL.Query().Get("user_wa_id"))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.metaStatus)
				_, _ = w.Write([]byte(tt.metaResponse))
			}))
			t.Cleanup(meta.Close)
			app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, meta.URL)

			req := testutil.NewGETRequest(t)
			testutil.SetAuthContext(req, org.ID, user.ID)
			testutil.SetPathParam(req, "contactId", contact.ID.String())
			testutil.SetQueryParam(req, "whatsapp_account", account.Name)

			require.NoError(t, app.GetCallPermission(req))
			require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
			var response struct {
				Status string `json:"status"`
				Data   struct {
					Status string `json:"status"`
				} `json:"data"`
			}
			testutil.ParseJSONResponse(t, req, &response)
			assert.Equal(t, "success", response.Status)
			assert.Equal(t, tt.wantStatus, response.Data.Status)
			assert.Equal(t, tt.wantRequests, requests.Load())
		})
	}
}
