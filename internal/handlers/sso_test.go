package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// fakeOAuthProvider stands up an httptest server that simulates a "custom" OIDC
// provider — auth, token, and userinfo endpoints. Tests can override email/name
// returned by userinfo via UserEmail/UserName fields.
type fakeOAuthProvider struct {
	server    *httptest.Server
	UserID    string
	UserEmail string
	UserName  string

	// State capture for assertions.
	LastTokenCode string
}

func newFakeOAuth(t *testing.T) *fakeOAuthProvider {
	t.Helper()
	f := &fakeOAuthProvider{
		UserID:    "ext-user-1",
		UserEmail: "user@example.com",
		UserName:  "External User",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		// Real OAuth providers redirect back to the app's callback. Tests just
		// assert that this URL was reachable; we don't follow the redirect here.
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.LastTokenCode = r.Form.Get("code")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub":   f.UserID,
			"email": f.UserEmail,
			"name":  f.UserName,
		})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeOAuthProvider) AuthURL() string     { return f.server.URL + "/auth" }
func (f *fakeOAuthProvider) TokenURL() string    { return f.server.URL + "/token" }
func (f *fakeOAuthProvider) UserInfoURL() string { return f.server.URL + "/userinfo" }

// newSSOApp returns an app suitable for SSO tests — config with empty encryption
// key (so client secret stored as plaintext) and a real HTTP client.
func newSSOApp(t *testing.T) *handlers.App {
	t.Helper()
	app := newTestApp(t)
	app.Config = &config.Config{
		App:    config.AppConfig{Environment: "development"},
		JWT:    config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
		Server: config.ServerConfig{},
	}
	return app
}

func createCustomSSOProvider(t *testing.T, app *handlers.App, orgID uuid.UUID, fake *fakeOAuthProvider, opts ...func(*models.SSOProvider)) *models.SSOProvider {
	t.Helper()
	p := &models.SSOProvider{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		Provider:        "custom",
		ClientID:        "client-id-1",
		ClientSecret:    "client-secret-1",
		IsEnabled:       true,
		AllowAutoCreate: true,
		DefaultRoleName: "agent",
		AuthURL:         fake.AuthURL(),
		TokenURL:        fake.TokenURL(),
		UserInfoURL:     fake.UserInfoURL(),
	}
	for _, o := range opts {
		o(p)
	}
	require.NoError(t, app.DB.Create(p).Error)
	return p
}

// --- GetPublicSSOProviders ---

func TestApp_GetPublicSSOProviders_DedupsByType(t *testing.T) {
	app := newSSOApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	// Two different orgs both have google enabled — should appear once.
	require.NoError(t, app.DB.Create(&models.SSOProvider{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org1.ID, Provider: "google", ClientID: "g1", ClientSecret: "s1", IsEnabled: true,
	}).Error)
	require.NoError(t, app.DB.Create(&models.SSOProvider{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org2.ID, Provider: "google", ClientID: "g2", ClientSecret: "s2", IsEnabled: true,
	}).Error)
	require.NoError(t, app.DB.Create(&models.SSOProvider{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org1.ID, Provider: "github", ClientID: "gh1", ClientSecret: "s3", IsEnabled: true,
	}).Error)
	// Disabled — must NOT appear.
	require.NoError(t, app.DB.Create(&models.SSOProvider{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org1.ID, Provider: "microsoft", ClientID: "ms1", ClientSecret: "s4", IsEnabled: false,
	}).Error)

	req := testutil.NewGETRequest(t)
	require.NoError(t, app.GetPublicSSOProviders(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data []handlers.SSOProviderPublic `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))

	got := make(map[string]bool)
	for _, p := range resp.Data {
		got[p.Provider] = true
	}
	assert.True(t, got["google"])
	assert.True(t, got["github"])
	assert.False(t, got["microsoft"], "disabled provider must not be exposed")
	assert.Len(t, resp.Data, 2, "two providers across 3 enabled rows")
}

// --- GetSSOSettings (admin) ---

func TestApp_GetSSOSettings_HidesSecretButReportsHasSecret(t *testing.T) {
	app := newSSOApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.DB.Create(&models.SSOProvider{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, Provider: "google",
		ClientID: "id", ClientSecret: "the-secret", IsEnabled: true,
	}).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, uuid.New())

	require.NoError(t, app.GetSSOSettings(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data []handlers.SSOProviderResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "id", resp.Data[0].ClientID)
	assert.True(t, resp.Data[0].HasSecret)
	// Marshal whole response to ensure no field carries the secret value.
	raw := string(testutil.GetResponseBody(req))
	assert.NotContains(t, raw, "the-secret", "client secret must never appear in admin response")
}

// --- UpdateSSOProvider ---

func TestApp_UpdateSSOProvider_CreateCustomRequiresURLs(t *testing.T) {
	app := newSSOApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	req := testutil.NewJSONRequest(t, map[string]any{
		"client_id":     "id",
		"client_secret": "secret",
		"is_enabled":    true,
	})
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "provider", "custom")

	require.NoError(t, app.UpdateSSOProvider(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "auth_url, token_url, and user_info_url")
}

func TestApp_UpdateSSOProvider_InvalidProviderRejected(t *testing.T) {
	app := newSSOApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	req := testutil.NewJSONRequest(t, map[string]any{
		"client_id":     "id",
		"client_secret": "s",
	})
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "provider", "okta") // not in allowlist

	require.NoError(t, app.UpdateSSOProvider(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid provider")
}

func TestApp_UpdateSSOProvider_EncryptsClientSecret(t *testing.T) {
	app := newSSOApp(t)
	app.Config.App.EncryptionKey = "this-is-a-32-character-test-key-XX"
	org := testutil.CreateTestOrganization(t, app.DB)

	req := testutil.NewJSONRequest(t, map[string]any{
		"client_id":     "id",
		"client_secret": "PLAIN-SSO-SECRET",
		"is_enabled":    true,
	})
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "provider", "google")

	require.NoError(t, app.UpdateSSOProvider(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var stored models.SSOProvider
	require.NoError(t, app.DB.Where("organization_id = ? AND provider = ?", org.ID, "google").First(&stored).Error)
	assert.NotEqual(t, "PLAIN-SSO-SECRET", stored.ClientSecret, "client secret must be encrypted at rest")
	assert.True(t, strings.HasPrefix(stored.ClientSecret, "enc:"), "stored secret should carry the enc: prefix")
}

func TestApp_UpdateSSOProvider_OmittingSecretLeavesUnchanged(t *testing.T) {
	app := newSSOApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// Pre-existing.
	original := &models.SSOProvider{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, Provider: "google",
		ClientID: "old-id", ClientSecret: "ORIGINAL-SECRET", IsEnabled: false, DefaultRoleName: "agent",
	}
	require.NoError(t, app.DB.Create(original).Error)

	// Update without supplying client_secret.
	req := testutil.NewJSONRequest(t, map[string]any{
		"client_id":  "new-id",
		"is_enabled": true,
	})
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "provider", "google")

	require.NoError(t, app.UpdateSSOProvider(req))

	var stored models.SSOProvider
	require.NoError(t, app.DB.Where("id = ?", original.ID).First(&stored).Error)
	assert.Equal(t, "new-id", stored.ClientID)
	assert.True(t, stored.IsEnabled)
	assert.Equal(t, "ORIGINAL-SECRET", stored.ClientSecret, "missing client_secret in body must not wipe the existing one")
}

// --- DeleteSSOProvider ---

func TestApp_DeleteSSOProvider_Success(t *testing.T) {
	app := newSSOApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.DB.Create(&models.SSOProvider{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, Provider: "google",
		ClientID: "id", ClientSecret: "s", IsEnabled: true,
	}).Error)

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "provider", "google")

	require.NoError(t, app.DeleteSSOProvider(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var count int64
	app.DB.Model(&models.SSOProvider{}).Where("organization_id = ? AND provider = ?", org.ID, "google").Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteSSOProvider_NotFound(t *testing.T) {
	app := newSSOApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, org.ID, uuid.New())
	testutil.SetPathParam(req, "provider", "google")

	require.NoError(t, app.DeleteSSOProvider(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "SSO provider not found")
}

func TestApp_DeleteSSOProvider_CrossOrgIsolation(t *testing.T) {
	app := newSSOApp(t)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	rec := &models.SSOProvider{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: orgA.ID, Provider: "google",
		ClientID: "id", ClientSecret: "s", IsEnabled: true,
	}
	require.NoError(t, app.DB.Create(rec).Error)

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, orgB.ID, uuid.New())
	testutil.SetPathParam(req, "provider", "google")

	require.NoError(t, app.DeleteSSOProvider(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	// Other org's record is intact.
	var count int64
	app.DB.Model(&models.SSOProvider{}).Where("id = ?", rec.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

// --- InitSSO ---

func TestApp_InitSSO_InvalidProviderRejected(t *testing.T) {
	app := newSSOApp(t)

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "unknown")

	require.NoError(t, app.InitSSO(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid SSO provider")
}

func TestApp_InitSSO_NoConfigReturns404(t *testing.T) {
	app := newSSOApp(t)
	// Other tests in this package create enabled providers; clear them so the
	// "not configured" path is actually exercised.
	require.NoError(t, app.DB.Exec("DELETE FROM sso_providers").Error)

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "google")

	require.NoError(t, app.InitSSO(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "not configured")
}

func TestApp_InitSSO_StoresStateInRedisAndRedirects(t *testing.T) {
	app := newSSOApp(t)
	fake := newFakeOAuth(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	createCustomSSOProvider(t, app, org.ID, fake)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.SetRequestURI("http://example.test/api/auth/sso/custom/init")
	req.RequestCtx.Request.SetHost("example.test")
	testutil.SetPathParam(req, "provider", "custom")

	require.NoError(t, app.InitSSO(req))
	require.Equal(t, fasthttp.StatusTemporaryRedirect, testutil.GetResponseStatusCode(req))

	loc := string(req.RequestCtx.Response.Header.Peek("Location"))
	require.NotEmpty(t, loc)
	parsed, err := url.Parse(loc)
	require.NoError(t, err)
	state := parsed.Query().Get("state")
	require.NotEmpty(t, state, "auth URL must include state= query")

	// Verify Redis stored state matches.
	got, err := app.Redis.Get(context.Background(), "sso:state:"+state).Bytes()
	require.NoError(t, err)
	var stored handlers.SSOState
	require.NoError(t, json.Unmarshal(got, &stored))
	assert.Equal(t, "custom", stored.Provider)
	assert.Equal(t, org.ID.String(), stored.OrgID)
	assert.True(t, stored.ExpiresAt.After(time.Now()))
}

// --- CallbackSSO: state validation ---

func TestApp_CallbackSSO_MissingCodeOrStateRedirectsWithError(t *testing.T) {
	app := newSSOApp(t)

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "custom")

	require.NoError(t, app.CallbackSSO(req))
	assert.Equal(t, fasthttp.StatusTemporaryRedirect, testutil.GetResponseStatusCode(req))
	loc := string(req.RequestCtx.Response.Header.Peek("Location"))
	assert.Contains(t, loc, "sso_error=")
	assert.Contains(t, loc, "Invalid+callback+parameters")
}

func TestApp_CallbackSSO_OAuthErrorParamRedirects(t *testing.T) {
	app := newSSOApp(t)

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "custom")
	testutil.SetQueryParam(req, "error", "access_denied")
	testutil.SetQueryParam(req, "error_description", "User declined")

	require.NoError(t, app.CallbackSSO(req))
	assert.Equal(t, fasthttp.StatusTemporaryRedirect, testutil.GetResponseStatusCode(req))
	loc := string(req.RequestCtx.Response.Header.Peek("Location"))
	assert.Contains(t, loc, "User+declined")
}

func TestApp_CallbackSSO_UnknownStateNonceFails(t *testing.T) {
	app := newSSOApp(t)

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "custom")
	testutil.SetQueryParam(req, "code", "any")
	testutil.SetQueryParam(req, "state", "never-stored")

	require.NoError(t, app.CallbackSSO(req))
	loc := string(req.RequestCtx.Response.Header.Peek("Location"))
	assert.Contains(t, loc, "Invalid+or+expired+state")
}

func TestApp_CallbackSSO_StateProviderMismatchRejected(t *testing.T) {
	app := newSSOApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	// Plant state claiming provider=google, but request comes in for provider=custom.
	nonce := "test-nonce-1"
	state := handlers.SSOState{
		OrgID: org.ID.String(), Provider: "google", Nonce: nonce, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	stored, _ := json.Marshal(state)
	require.NoError(t, app.Redis.Set(context.Background(), "sso:state:"+nonce, stored, 5*time.Minute).Err())

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "custom")
	testutil.SetQueryParam(req, "code", "x")
	testutil.SetQueryParam(req, "state", nonce)

	require.NoError(t, app.CallbackSSO(req))
	loc := string(req.RequestCtx.Response.Header.Peek("Location"))
	assert.Contains(t, loc, "Invalid+or+expired+state")
}

func TestApp_CallbackSSO_StateIsSingleUse(t *testing.T) {
	app := newSSOApp(t)
	fake := newFakeOAuth(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "agent", true, true, nil)
	_ = role
	createCustomSSOProvider(t, app, org.ID, fake)

	nonce := "single-use-nonce"
	state := handlers.SSOState{
		OrgID: org.ID.String(), Provider: "custom", Nonce: nonce, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	stored, _ := json.Marshal(state)
	require.NoError(t, app.Redis.Set(context.Background(), "sso:state:"+nonce, stored, 5*time.Minute).Err())

	// First callback consumes the state and (likely) succeeds.
	req1 := testutil.NewGETRequest(t)
	testutil.SetPathParam(req1, "provider", "custom")
	testutil.SetQueryParam(req1, "code", "code-1")
	testutil.SetQueryParam(req1, "state", nonce)
	require.NoError(t, app.CallbackSSO(req1))

	// Replay: same nonce should fail with state error.
	req2 := testutil.NewGETRequest(t)
	testutil.SetPathParam(req2, "provider", "custom")
	testutil.SetQueryParam(req2, "code", "code-1")
	testutil.SetQueryParam(req2, "state", nonce)
	require.NoError(t, app.CallbackSSO(req2))
	loc := string(req2.RequestCtx.Response.Header.Peek("Location"))
	assert.Contains(t, loc, "Invalid+or+expired+state", "state must be deleted on first use")
}

// --- CallbackSSO: full happy path with custom provider ---

func TestApp_CallbackSSO_CustomProvider_ExistingUser_LoginSuccess(t *testing.T) {
	app := newSSOApp(t)
	fake := newFakeOAuth(t)
	email := testutil.UniqueEmail("sso-existing")
	fake.UserEmail = email
	org := testutil.CreateTestOrganization(t, app.DB)
	createCustomSSOProvider(t, app, org.ID, fake, func(p *models.SSOProvider) {
		p.AllowAutoCreate = false
	})

	// Pre-create the user matching the userinfo response.
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email))

	nonce := "good-nonce"
	state := handlers.SSOState{
		OrgID: org.ID.String(), Provider: "custom", Nonce: nonce, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	stored, _ := json.Marshal(state)
	require.NoError(t, app.Redis.Set(context.Background(), "sso:state:"+nonce, stored, 5*time.Minute).Err())

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "custom")
	testutil.SetQueryParam(req, "code", "auth-code-xyz")
	testutil.SetQueryParam(req, "state", nonce)

	require.NoError(t, app.CallbackSSO(req))
	assert.Equal(t, fasthttp.StatusTemporaryRedirect, testutil.GetResponseStatusCode(req))

	// Auth cookies set on the response.
	assert.NotEmpty(t, testutil.GetResponseCookie(req, "whm_access"))
	assert.NotEmpty(t, testutil.GetResponseCookie(req, "whm_refresh"))

	// Token endpoint received our code.
	assert.Equal(t, "auth-code-xyz", fake.LastTokenCode)

	// Redirect target is the frontend SSO callback (no token in URL).
	loc := string(req.RequestCtx.Response.Header.Peek("Location"))
	assert.Contains(t, loc, "/auth/sso/callback")
	assert.NotContains(t, loc, "access_token", "token must not be exposed in URL")

	// Existing user got SSO fields populated.
	var refreshed models.User
	require.NoError(t, app.DB.Where("id = ?", user.ID).First(&refreshed).Error)
	assert.Equal(t, "custom", refreshed.SSOProvider)
}

func TestApp_CallbackSSO_AutoCreateDisabledRejectsNewUser(t *testing.T) {
	app := newSSOApp(t)
	fake := newFakeOAuth(t)
	fake.UserEmail = "newcomer@example.com"
	org := testutil.CreateTestOrganization(t, app.DB)
	createCustomSSOProvider(t, app, org.ID, fake, func(p *models.SSOProvider) {
		p.AllowAutoCreate = false
	})

	nonce := "no-auto-nonce"
	state := handlers.SSOState{
		OrgID: org.ID.String(), Provider: "custom", Nonce: nonce, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	stored, _ := json.Marshal(state)
	require.NoError(t, app.Redis.Set(context.Background(), "sso:state:"+nonce, stored, 5*time.Minute).Err())

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "custom")
	testutil.SetQueryParam(req, "code", "c")
	testutil.SetQueryParam(req, "state", nonce)

	require.NoError(t, app.CallbackSSO(req))
	loc := string(req.RequestCtx.Response.Header.Peek("Location"))
	assert.Contains(t, loc, "User+not+found")

	// And no user was created.
	var count int64
	app.DB.Model(&models.User{}).Where("email = ?", "newcomer@example.com").Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_CallbackSSO_AutoCreateEnabledCreatesUserWithDefaultRole(t *testing.T) {
	app := newSSOApp(t)
	fake := newFakeOAuth(t)
	fake.UserEmail = "auto@example.com"
	fake.UserName = "Auto Created"
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "agent", true, true, nil)
	createCustomSSOProvider(t, app, org.ID, fake) // AllowAutoCreate=true by default

	nonce := "auto-nonce"
	state := handlers.SSOState{
		OrgID: org.ID.String(), Provider: "custom", Nonce: nonce, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	stored, _ := json.Marshal(state)
	require.NoError(t, app.Redis.Set(context.Background(), "sso:state:"+nonce, stored, 5*time.Minute).Err())

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "custom")
	testutil.SetQueryParam(req, "code", "c")
	testutil.SetQueryParam(req, "state", nonce)

	require.NoError(t, app.CallbackSSO(req))
	require.Equal(t, fasthttp.StatusTemporaryRedirect, testutil.GetResponseStatusCode(req))
	assert.NotEmpty(t, testutil.GetResponseCookie(req, "whm_access"))

	var created models.User
	require.NoError(t, app.DB.Where("email = ?", "auto@example.com").First(&created).Error)
	assert.Equal(t, "Auto Created", created.FullName)
	require.NotNil(t, created.RoleID)
	assert.Equal(t, role.ID, *created.RoleID)
	assert.True(t, created.IsActive)

	// UserOrganization entry must also exist.
	var uoCount int64
	app.DB.Model(&models.UserOrganization{}).Where("user_id = ? AND organization_id = ?", created.ID, org.ID).Count(&uoCount)
	assert.Equal(t, int64(1), uoCount)
}

func TestApp_CallbackSSO_DomainRestrictionRejectsOutsideEmail(t *testing.T) {
	app := newSSOApp(t)
	fake := newFakeOAuth(t)
	fake.UserEmail = "outsider@notallowed.com"
	org := testutil.CreateTestOrganization(t, app.DB)
	createCustomSSOProvider(t, app, org.ID, fake, func(p *models.SSOProvider) {
		p.AllowedDomains = "example.com,corp.example.com"
	})

	nonce := "domain-nonce"
	state := handlers.SSOState{
		OrgID: org.ID.String(), Provider: "custom", Nonce: nonce, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	stored, _ := json.Marshal(state)
	require.NoError(t, app.Redis.Set(context.Background(), "sso:state:"+nonce, stored, 5*time.Minute).Err())

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "custom")
	testutil.SetQueryParam(req, "code", "c")
	testutil.SetQueryParam(req, "state", nonce)

	require.NoError(t, app.CallbackSSO(req))
	loc := string(req.RequestCtx.Response.Header.Peek("Location"))
	assert.Contains(t, loc, "Email+domain+not+allowed")
}

func TestApp_CallbackSSO_DisabledExistingUserRejected(t *testing.T) {
	app := newSSOApp(t)
	fake := newFakeOAuth(t)
	email := testutil.UniqueEmail("sso-disabled")
	fake.UserEmail = email
	org := testutil.CreateTestOrganization(t, app.DB)
	createCustomSSOProvider(t, app, org.ID, fake, func(p *models.SSOProvider) {
		p.AllowAutoCreate = false
	})

	// Create existing user, then disable.
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email))
	require.NoError(t, app.DB.Model(user).Update("is_active", false).Error)

	nonce := "disabled-user-nonce"
	state := handlers.SSOState{
		OrgID: org.ID.String(), Provider: "custom", Nonce: nonce, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	stored, _ := json.Marshal(state)
	require.NoError(t, app.Redis.Set(context.Background(), "sso:state:"+nonce, stored, 5*time.Minute).Err())

	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "provider", "custom")
	testutil.SetQueryParam(req, "code", "c")
	testutil.SetQueryParam(req, "state", nonce)

	require.NoError(t, app.CallbackSSO(req))
	loc := string(req.RequestCtx.Response.Header.Peek("Location"))
	assert.Contains(t, loc, "Account+is+disabled")
	// No cookies set when account is disabled.
	assert.Empty(t, testutil.GetResponseCookie(req, "whm_access"))
}
