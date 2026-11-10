package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// smbMetaServer mocks the Meta endpoints used when onboarding a coexistence
// number and records the smb_app_data sync types it receives.
func smbMetaServer(t *testing.T, phoneID, wabaID string) (*httptest.Server, func() []string, func() bool) {
	t.Helper()
	var mu sync.Mutex
	var syncTypes []string
	registered := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/oauth/access_token"):
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "smb_token"})
		case strings.HasSuffix(path, phoneID+"/smb_app_data"):
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			syncTypes = append(syncTypes, body["sync_type"])
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]string{
				"messaging_product": "whatsapp",
				"request_id":        "req-" + body["sync_type"],
			})
		case strings.HasSuffix(path, phoneID+"/register"):
			mu.Lock()
			registered = true
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		case strings.Contains(path, phoneID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"verified_name":        fmt.Sprintf("SMB Business %d", time.Now().UnixNano()),
				"display_phone_number": "+15550001111",
				"is_on_biz_app":        true,
				"platform_type":        "CLOUD_API",
			})
		case strings.Contains(path, wabaID):
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	return server,
		func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), syncTypes...) },
		func() bool { mu.Lock(); defer mu.Unlock(); return registered }
}

func TestApp_ExchangeToken_Coexistence_RequestsSync(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	phoneID := fmt.Sprintf("323456789%d", time.Now().UnixNano()%1000000)
	wabaID := fmt.Sprintf("787654321%d", time.Now().UnixNano()%1000000)
	server, syncTypes, registered := smbMetaServer(t, phoneID, wabaID)
	app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, server.URL)

	req := testutil.NewJSONRequest(t, map[string]any{
		"code":     "smb_code",
		"phone_id": phoneID,
		"waba_id":  wabaID,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.ExchangeToken(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Account         map[string]any            `json:"account"`
			CoexistenceSync map[string]map[string]any `json:"coexistence_sync"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))

	assert.Equal(t, "active", resp.Data.Account["status"])
	assert.Equal(t, true, resp.Data.Account["is_smb"])
	assert.False(t, registered(), "coexistence numbers must not be re-registered")
	assert.Equal(t, []string{whatsapp.SMBSyncTypeContacts, whatsapp.SMBSyncTypeHistory}, syncTypes())
	assert.Equal(t, "req-history", resp.Data.CoexistenceSync["history"]["request_id"])
	assert.Equal(t, true, resp.Data.CoexistenceSync["smb_app_state_sync"]["success"])
}

func TestApp_SyncCoexistenceData(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	server, syncTypes, _ := smbMetaServer(t, account.PhoneID, account.BusinessID)
	app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, server.URL)

	// Not a coexistence number: rejected.
	req := testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", account.ID.String())
	require.NoError(t, app.SyncCoexistenceData(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	assert.Empty(t, syncTypes())

	// Coexistence number: both syncs requested.
	require.NoError(t, app.DB.Model(&models.WhatsAppAccount{}).Where("id = ?", account.ID).Update("is_smb", true).Error)

	req = testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", account.ID.String())
	require.NoError(t, app.SyncCoexistenceData(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	assert.Equal(t, []string{whatsapp.SMBSyncTypeContacts, whatsapp.SMBSyncTypeHistory}, syncTypes())
}
