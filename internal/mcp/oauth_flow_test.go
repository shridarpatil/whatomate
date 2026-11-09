package mcp

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/middleware"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestAuthorize_ValidatesClientRedirectAndPKCE(t *testing.T) {
	s := newDBServer(t)
	s.app.Config.Server.BasePath = "/whatomate"
	clientID := registerClient(t, s)
	_, challenge := pkcePair(strings.Repeat("k", 60))
	q := url.Values{"client_id": {clientID}, "redirect_uri": {testRedirectURI}, "response_type": {"code"},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"abc"}}

	r := newRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	require.NoError(t, s.Authorize(r))
	assert.Equal(t, fasthttp.StatusFound, r.RequestCtx.Response.StatusCode())
	loc := string(r.RequestCtx.Response.Header.Peek("Location"))
	assert.True(t, strings.HasPrefix(loc, "/whatomate/oauth/consent?"), loc)
	assert.Contains(t, loc, "state=abc")

	// Redirect URI must match exactly: no redirect to an unregistered target.
	bad := url.Values{}
	for k, v := range q {
		bad[k] = v
	}
	bad.Set("redirect_uri", testRedirectURI+"/x")
	r = newRequest("GET", "/oauth/authorize?"+bad.Encode(), nil)
	require.NoError(t, s.Authorize(r))
	assert.Equal(t, fasthttp.StatusBadRequest, r.RequestCtx.Response.StatusCode())
	assert.Empty(t, r.RequestCtx.Response.Header.Peek("Location"))

	// Missing PKCE / plain method is reported to the (valid) redirect URI.
	noPKCE := url.Values{}
	for k, v := range q {
		noPKCE[k] = v
	}
	noPKCE.Set("code_challenge_method", "plain")
	r = newRequest("GET", "/oauth/authorize?"+noPKCE.Encode(), nil)
	require.NoError(t, s.Authorize(r))
	assert.Equal(t, fasthttp.StatusFound, r.RequestCtx.Response.StatusCode())
	loc = string(r.RequestCtx.Response.Header.Peek("Location"))
	assert.True(t, strings.HasPrefix(loc, testRedirectURI+"?"), loc)
	assert.Contains(t, loc, "error=invalid_request")

	// Registration refuses non-loopback http redirect URIs.
	r = newRequest("POST", "/oauth/register", []byte(`{"client_name":"x","redirect_uris":["http://evil.example.com/cb"]}`))
	require.NoError(t, s.Register(r))
	assert.Equal(t, fasthttp.StatusBadRequest, r.RequestCtx.Response.StatusCode())
}

func TestRegister_RateLimited(t *testing.T) {
	s := newDBServer(t)
	s.registerLimit = middleware.RateLimit(middleware.RateLimitOpts{
		Redis: s.app.Redis, Log: s.app.Log, Max: 1, Window: time.Minute, KeyPrefix: "mcp_register_test_" + uuid.NewString(),
	})
	registerClient(t, s)
	r := newRequest("POST", "/oauth/register", []byte(`{"client_name":"x","redirect_uris":["`+testRedirectURI+`"]}`))
	require.NoError(t, s.Register(r))
	assert.Equal(t, fasthttp.StatusTooManyRequests, r.RequestCtx.Response.StatusCode())
}

func TestConsent_RequiresHumanSessionAndMembership(t *testing.T) {
	s := newDBServer(t)
	a, b := newOrgFixture(t, s), newOrgFixture(t, s)
	clientID := registerClient(t, s)
	_, challenge := pkcePair(strings.Repeat("c", 60))
	post := func(userID uuid.UUID, orgID uuid.UUID, approve bool, header string) (int, string) {
		body, _ := json.Marshal(map[string]any{"client_id": clientID, "redirect_uri": testRedirectURI, "state": "s",
			"code_challenge": challenge, "code_challenge_method": "S256", "organization_id": orgID.String(), "approve": approve})
		r := newRequest("POST", "/api/oauth/consent", body)
		r.RequestCtx.SetUserValue("user_id", userID)
		if header != "" {
			r.RequestCtx.Request.Header.Set(header, "mcp")
		}
		require.NoError(t, s.Consent(r))
		return r.RequestCtx.Response.StatusCode(), string(r.RequestCtx.Response.Body())
	}

	status, body := post(a.User.ID, a.Org.ID, false, "")
	assert.Equal(t, 200, status)
	assert.Contains(t, body, "error=access_denied")

	status, _ = post(a.User.ID, b.Org.ID, true, "")
	assert.Equal(t, fasthttp.StatusForbidden, status, "non-member org must be refused")

	status, _ = post(a.User.ID, a.Org.ID, true, "X-API-Key")
	assert.Equal(t, fasthttp.StatusForbidden, status, "API-key callers cannot consent")
	status, _ = post(a.User.ID, a.Org.ID, true, "X-Whatomate-Client")
	assert.Equal(t, fasthttp.StatusForbidden, status, "connector-dispatched calls cannot consent")

	// Client info for the consent page.
	r := newRequest("GET", "/api/oauth/client?client_id="+clientID+"&redirect_uri="+url.QueryEscape(testRedirectURI), nil)
	r.RequestCtx.SetUserValue("user_id", a.User.ID)
	require.NoError(t, s.ClientInfo(r))
	assert.Contains(t, string(r.RequestCtx.Response.Body()), `"client_name":"Claude"`)
	assert.Contains(t, string(r.RequestCtx.Response.Body()), `"redirect_uri_valid":true`)
}

func TestTokenExchange_PKCEAndSingleUseCode(t *testing.T) {
	s := newDBServer(t)
	a := newOrgFixture(t, s)
	clientID := registerClient(t, s)
	verifier, challenge := pkcePair(strings.Repeat("p", 60))
	exchange := func(code, verifier, redirect string) (int, map[string]any) {
		return tokenRequest(t, s, url.Values{"grant_type": {"authorization_code"}, "code": {code},
			"client_id": {clientID}, "redirect_uri": {redirect}, "code_verifier": {verifier}})
	}

	code := consent(t, s, a.User.ID, a.Org.ID, clientID, challenge)
	status, out := exchange(code, strings.Repeat("q", 60), testRedirectURI)
	assert.Equal(t, 400, status)
	assert.Equal(t, "invalid_grant", out["error"])

	code = consent(t, s, a.User.ID, a.Org.ID, clientID, challenge)
	status, _ = exchange(code, verifier, testRedirectURI+"x")
	assert.Equal(t, 400, status, "redirect_uri must match")

	code = consent(t, s, a.User.ID, a.Org.ID, clientID, challenge)
	status, out = exchange(code, verifier, testRedirectURI)
	require.Equal(t, 200, status, out)
	assert.True(t, strings.HasPrefix(out["access_token"].(string), accessTokenPrefix))
	assert.True(t, strings.HasPrefix(out["refresh_token"].(string), refreshTokenPrefix))
	assert.Equal(t, "Bearer", out["token_type"])

	status, out = exchange(code, verifier, testRedirectURI)
	assert.Equal(t, 400, status, "a code can be redeemed once")
	assert.Equal(t, "invalid_grant", out["error"])

	// Tokens are stored hashed only.
	var count int64
	s.app.DB.Model(&models.MCPToken{}).Where("token_hash = ?", out["access_token"]).Count(&count)
	assert.Zero(t, count)
}

func TestRefreshRotation_ReuseRevokesGrant(t *testing.T) {
	s := newDBServer(t)
	a := newOrgFixture(t, s)
	access1, refresh1 := connect(t, s, a.User.ID, a.Org.ID)

	status, _ := mcpCall(t, s, "Authorization", "Bearer "+access1, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	require.Equal(t, 200, status)

	status, out := tokenRequest(t, s, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh1}})
	require.Equal(t, 200, status, out)
	access2, refresh2 := out["access_token"].(string), out["refresh_token"].(string)
	assert.NotEqual(t, refresh1, refresh2)

	status, _ = mcpCall(t, s, "Authorization", "Bearer "+access1, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assert.Equal(t, 401, status, "rotated-out access token stops working")
	status, _ = mcpCall(t, s, "Authorization", "Bearer "+access2, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assert.Equal(t, 200, status)

	// Replaying the old refresh token revokes the whole grant.
	status, out = tokenRequest(t, s, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh1}})
	assert.Equal(t, 400, status)
	assert.Equal(t, "invalid_grant", out["error"])
	status, _ = mcpCall(t, s, "Authorization", "Bearer "+access2, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assert.Equal(t, 401, status)
	status, _ = tokenRequest(t, s, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh2}})
	assert.Equal(t, 400, status)
}
