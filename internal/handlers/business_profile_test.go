package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/logf"
	"gorm.io/gorm"
)

// fakeProfileServer simulates Meta's whatsapp_business_profile endpoint.
// GetBusinessProfile = GET; UpdateBusinessProfile = POST.
type fakeProfileServer struct {
	server   *httptest.Server
	getFn    func(w http.ResponseWriter, r *http.Request)
	postFn   func(w http.ResponseWriter, r *http.Request)
	LastBody map[string]any
}

func newFakeProfileServer(t *testing.T) *fakeProfileServer {
	t.Helper()
	f := &fakeProfileServer{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/whatsapp_business_profile") {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if f.getFn != nil {
				f.getFn(w, r)
				return
			}
			// Default: a populated profile.
			_, _ = w.Write([]byte(`{"data":[{"messaging_product":"whatsapp","about":"hello","description":"a desc","email":"x@example.com","vertical":"RETAIL","websites":["https://example.com"],"profile_picture_url":"https://cdn/p.jpg","address":"1 Main St"}]}`))
		case http.MethodPost:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.LastBody = body
			if f.postFn != nil {
				f.postFn(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

// newAppForProfile creates an app pointed at a profile-server-shaped fake Meta endpoint.
func newAppForProfile(t *testing.T, fakeURL string) *handlers.App {
	t.Helper()
	app := newTestApp(t)
	app.WhatsApp = whatsapp.NewWithBaseURL(logf.New(logf.Opts{Level: logf.ErrorLevel}), fakeURL)
	app.Config = &config.Config{
		JWT:      config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
		WhatsApp: config.WhatsAppConfig{BaseURL: fakeURL, APIVersion: "v18.0"},
	}
	return app
}

func mkAccountForProfile(t *testing.T, db *gorm.DB, orgID uuid.UUID) *models.WhatsAppAccount {
	t.Helper()
	acc := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     orgID,
		Name:               "p-acc-" + uuid.New().String()[:6],
		PhoneID:            "phone-" + uuid.New().String()[:8],
		BusinessID:         "biz-" + uuid.New().String()[:8],
		AccessToken:        "tok",
		WebhookVerifyToken: "vt-" + uuid.New().String()[:8],
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, db.Create(acc).Error)
	return acc
}

// --- GetBusinessProfile ---

func TestApp_GetBusinessProfile_Success(t *testing.T) {
	meta := newFakeProfileServer(t)
	app := newAppForProfile(t, meta.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := mkAccountForProfile(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "id", acc.ID.String())

	require.NoError(t, app.GetBusinessProfile(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data whatsapp.BusinessProfile `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "hello", resp.Data.About)
	assert.Equal(t, "x@example.com", resp.Data.Email)
}

func TestApp_GetBusinessProfile_AccountNotFound(t *testing.T) {
	meta := newFakeProfileServer(t)
	app := newAppForProfile(t, meta.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "id", uuid.New().String())

	require.NoError(t, app.GetBusinessProfile(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetBusinessProfile_CrossOrgIsolation(t *testing.T) {
	meta := newFakeProfileServer(t)
	app := newAppForProfile(t, meta.server.URL)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	acc := mkAccountForProfile(t, app.DB, orgA.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, orgB.ID, uuid.New())
	testutil.SetPathParam(req, "id", acc.ID.String())

	require.NoError(t, app.GetBusinessProfile(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req),
		"cross-org access must look like not-found")
}

func TestApp_GetBusinessProfile_MetaAPIErrorBubbles(t *testing.T) {
	meta := newFakeProfileServer(t)
	meta.getFn = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad token","code":190}}`))
	}
	app := newAppForProfile(t, meta.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := mkAccountForProfile(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "id", acc.ID.String())

	require.NoError(t, app.GetBusinessProfile(req))
	assert.Equal(t, fasthttp.StatusInternalServerError, testutil.GetResponseStatusCode(req))
}

func TestApp_GetBusinessProfile_Unauthorized(t *testing.T) {
	meta := newFakeProfileServer(t)
	app := newAppForProfile(t, meta.server.URL)

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "id", uuid.New().String())

	require.NoError(t, app.GetBusinessProfile(req))
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- UpdateBusinessProfile ---

func TestApp_UpdateBusinessProfile_Success(t *testing.T) {
	meta := newFakeProfileServer(t)
	app := newAppForProfile(t, meta.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := mkAccountForProfile(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"about":       "new about",
		"description": "new desc",
		"email":       "new@example.com",
		"vertical":    "EDU",
		"websites":    []string{"https://new.example.com"},
		"address":     "2 New Rd",
	})
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "id", acc.ID.String())

	require.NoError(t, app.UpdateBusinessProfile(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Meta received the update payload with messaging_product set by the client.
	require.NotNil(t, meta.LastBody)
	assert.Equal(t, "whatsapp", meta.LastBody["messaging_product"])
	assert.Equal(t, "new about", meta.LastBody["about"])
	assert.Equal(t, "new desc", meta.LastBody["description"])
	assert.Equal(t, "new@example.com", meta.LastBody["email"])

	// Handler does a re-fetch (GET) and returns the refreshed profile from the fake.
	var resp struct {
		Data whatsapp.BusinessProfile `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "hello", resp.Data.About) // fake's default GET response
}

func TestApp_UpdateBusinessProfile_RefetchFailureStillReportsSuccess(t *testing.T) {
	meta := newFakeProfileServer(t)
	// Update succeeds, but the post-update re-fetch returns an error.
	meta.getFn = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"transient","code":1}}`))
	}
	app := newAppForProfile(t, meta.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := mkAccountForProfile(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"about": "x"})
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "id", acc.ID.String())

	require.NoError(t, app.UpdateBusinessProfile(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Contains(t, resp.Data["message"], "Profile updated successfully",
		"when re-fetch fails the handler should still report success with a message")
}

func TestApp_UpdateBusinessProfile_MetaUpdateFails(t *testing.T) {
	meta := newFakeProfileServer(t)
	meta.postFn = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid email","code":100}}`))
	}
	app := newAppForProfile(t, meta.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := mkAccountForProfile(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"email": "bad"})
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "id", acc.ID.String())

	require.NoError(t, app.UpdateBusinessProfile(req))
	assert.Equal(t, fasthttp.StatusInternalServerError, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateBusinessProfile_CrossOrgIsolation(t *testing.T) {
	meta := newFakeProfileServer(t)
	app := newAppForProfile(t, meta.server.URL)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	acc := mkAccountForProfile(t, app.DB, orgA.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"about": "should-not-apply"})
	testutil.SetAuthContext(req, orgB.ID, uuid.New())
	testutil.SetPathParam(req, "id", acc.ID.String())

	require.NoError(t, app.UpdateBusinessProfile(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	assert.Nil(t, meta.LastBody, "Meta must not be called when the account doesn't belong to the requesting org")
}

func TestApp_UpdateBusinessProfile_InvalidJSONBody(t *testing.T) {
	meta := newFakeProfileServer(t)
	app := newAppForProfile(t, meta.server.URL)
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := mkAccountForProfile(t, app.DB, org.ID)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.SetMethod("PUT")
	req.RequestCtx.Request.SetBody([]byte("not json"))
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "id", acc.ID.String())

	require.NoError(t, app.UpdateBusinessProfile(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}
