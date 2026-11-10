package whatsapp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_RequestSMBAppDataSync(t *testing.T) {
	t.Parallel()

	for _, syncType := range []string{whatsapp.SMBSyncTypeContacts, whatsapp.SMBSyncTypeHistory} {
		t.Run(syncType, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/v24.0/123456/smb_app_data", r.URL.Path)
				assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
				var body map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, "whatsapp", body["messaging_product"])
				assert.Equal(t, syncType, body["sync_type"])
				_ = json.NewEncoder(w).Encode(map[string]string{
					"messaging_product": "whatsapp",
					"request_id":        "req-" + syncType,
				})
			}))
			defer server.Close()

			client := whatsapp.NewWithBaseURL(testutil.NopLogger(), server.URL)
			id, err := client.RequestSMBAppDataSync(context.Background(), &whatsapp.Account{
				PhoneID: "123456", APIVersion: "v24.0", AccessToken: "tok",
			}, syncType)
			require.NoError(t, err)
			assert.Equal(t, "req-"+syncType, id)
		})
	}
}

func TestClient_RequestSMBAppDataSync_InvalidType(t *testing.T) {
	t.Parallel()
	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), "http://127.0.0.1:1")
	_, err := client.RequestSMBAppDataSync(context.Background(), &whatsapp.Account{PhoneID: "1"}, "bogus")
	assert.Error(t, err)
}

func TestClient_RequestSMBAppDataSync_MetaError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Sync window expired","code":100}}`))
	}))
	defer server.Close()

	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), server.URL)
	_, err := client.RequestSMBAppDataSync(context.Background(), &whatsapp.Account{
		PhoneID: "1", APIVersion: "v24.0", AccessToken: "tok",
	}, whatsapp.SMBSyncTypeHistory)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Sync window expired")
}
