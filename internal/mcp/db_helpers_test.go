package mcp

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/config"
	"github.com/nikyjain/whatomate/internal/handlers"
	"github.com/nikyjain/whatomate/internal/middleware"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/test/testutil"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

const testRedirectURI = "https://claude.ai/api/mcp/auth_callback"

// newDBServer builds a Server on the shared test DB + Redis with a mini router
// (the real /api auth hook plus the handlers the tests call). Skips when
// TEST_DATABASE_URL / TEST_REDIS_URL are not set.
func newDBServer(t *testing.T) *Server {
	t.Helper()
	db := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set, skipping test")
	}
	app := &handlers.App{
		Config: &config.Config{
			App: config.AppConfig{EncryptionKey: "test-encryption-key-for-handlers-longer-than-32-chars"},
			JWT: config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
			MCP: config.MCPConfig{Enabled: true},
		},
		DB: db, Redis: rdb, Log: testutil.NopLogger(),
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
	s := New(app)
	// The shared Redis keeps rate-limit counters across runs; tests that need
	// the registration limiter install their own (see TestRegister_RateLimited).
	s.registerLimit = nil
	g := fastglue.NewGlue()
	g.Before(func(r *fastglue.Request) *fastglue.Request {
		if strings.HasPrefix(string(r.RequestCtx.Path()), "/api") {
			return middleware.AuthWithDB(app.Config.JWT.Secret, app.DB)(r)
		}
		return r
	})
	g.GET("/api/me", app.GetCurrentUser)
	g.GET("/api/me/organizations", app.ListMyOrganizations)
	g.GET("/api/contacts", app.ListContacts)
	g.GET("/api/users", app.ListUsers)
	s.SetHandler(g.Handler())
	useTestCatalog(t)
	return s
}

// orgFixture is an organisation with a user (role: contacts:read only) and
// one contact.
type orgFixture struct {
	Org     *models.Organization
	Role    *models.CustomRole
	User    *models.User
	Contact *models.Contact
}

func randomPhone() string { return fmt.Sprintf("9199%08d", rand.IntN(100000000)) }

func newOrgFixture(t *testing.T, s *Server) *orgFixture {
	t.Helper()
	db := s.app.DB
	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "mcp-reader-"+uuid.NewString()[:8], []string{"contacts:read"})
	user := testutil.CreateTestUser(t, db, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContactWith(t, db, org.ID, testutil.WithPhoneNumber(randomPhone()))
	return &orgFixture{Org: org, Role: role, User: user, Contact: contact}
}

// addMembership makes user a member of org with role.
func addMembership(t *testing.T, s *Server, userID, orgID uuid.UUID, roleID *uuid.UUID) {
	t.Helper()
	require.NoError(t, s.app.DB.Create(&models.UserOrganization{
		BaseModel: models.BaseModel{ID: uuid.New()}, UserID: userID, OrganizationID: orgID, RoleID: roleID,
	}).Error)
}

func registerClient(t *testing.T, s *Server) string {
	t.Helper()
	r := newRequest("POST", "/oauth/register", []byte(`{"client_name":"Claude","redirect_uris":["`+testRedirectURI+`"]}`))
	require.NoError(t, s.Register(r))
	require.Equal(t, fasthttp.StatusCreated, r.RequestCtx.Response.StatusCode(), string(r.RequestCtx.Response.Body()))
	var out map[string]any
	require.NoError(t, json.Unmarshal(r.RequestCtx.Response.Body(), &out))
	return out["client_id"].(string)
}

// consent posts an approval as a browser-session user and returns the code.
func consent(t *testing.T, s *Server, userID, orgID uuid.UUID, clientID, challenge string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"client_id": clientID, "redirect_uri": testRedirectURI, "state": "st8",
		"code_challenge": challenge, "code_challenge_method": "S256",
		"organization_id": orgID.String(), "approve": true,
	})
	r := newRequest("POST", "/api/oauth/consent", body)
	r.RequestCtx.SetUserValue("user_id", userID)
	require.NoError(t, s.Consent(r))
	require.Equal(t, 200, r.RequestCtx.Response.StatusCode(), string(r.RequestCtx.Response.Body()))
	var env struct {
		Data struct {
			RedirectURL string `json:"redirect_url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(r.RequestCtx.Response.Body(), &env))
	u, err := url.Parse(env.Data.RedirectURL)
	require.NoError(t, err)
	require.Equal(t, "st8", u.Query().Get("state"))
	return u.Query().Get("code")
}

// tokenRequest posts a form to /oauth/token and returns status + JSON body.
func tokenRequest(t *testing.T, s *Server, form url.Values) (int, map[string]any) {
	t.Helper()
	r := newRequest("POST", "/oauth/token", nil)
	r.RequestCtx.Request.Header.SetContentType("application/x-www-form-urlencoded")
	r.RequestCtx.Request.SetBodyString(form.Encode())
	require.NoError(t, s.Token(r))
	var out map[string]any
	require.NoError(t, json.Unmarshal(r.RequestCtx.Response.Body(), &out))
	return r.RequestCtx.Response.StatusCode(), out
}

// connect runs the full OAuth flow and returns access + refresh tokens.
func connect(t *testing.T, s *Server, userID, orgID uuid.UUID) (string, string) {
	t.Helper()
	clientID := registerClient(t, s)
	verifier, challenge := pkcePair("v-" + strings.Repeat("x", 50) + uuid.NewString())
	code := consent(t, s, userID, orgID, clientID, challenge)
	status, out := tokenRequest(t, s, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID},
		"redirect_uri": {testRedirectURI}, "code_verifier": {verifier},
	})
	require.Equal(t, 200, status, out)
	return out["access_token"].(string), out["refresh_token"].(string)
}

// mcpCall posts one JSON-RPC message to /mcp with the given auth header.
func mcpCall(t *testing.T, s *Server, header, value, msg string) (int, map[string]any) {
	t.Helper()
	r := newRequest("POST", "/mcp", []byte(msg))
	r.RequestCtx.Request.Header.Set(header, value)
	require.NoError(t, s.HandleMCP(r))
	var out map[string]any
	if b := r.RequestCtx.Response.Body(); len(b) > 0 {
		require.NoError(t, json.Unmarshal(b, &out), string(b))
	}
	return r.RequestCtx.Response.StatusCode(), out
}

func callToolMsg(name, args string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`
}
