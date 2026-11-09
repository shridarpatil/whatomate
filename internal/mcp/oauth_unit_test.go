package mcp

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func pkcePair(verifier string) (string, string) {
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestVerifyPKCE_S256(t *testing.T) {
	v, c := pkcePair(strings.Repeat("a", 43) + "-._~")
	assert.True(t, verifyPKCE(v, c))
	assert.False(t, verifyPKCE(v+"x", c))
	assert.False(t, verifyPKCE("short", c))
	assert.False(t, verifyPKCE(v, c[:len(c)-1]+"A"))
	assert.False(t, verifyPKCE(c, c), "plain method must not pass")
}

func TestValidRedirectURI(t *testing.T) {
	for _, ok := range []string{
		"https://claude.ai/api/mcp/auth_callback", "http://localhost:6274/oauth/callback",
		"http://127.0.0.1:33418/callback", "http://[::1]:8080/cb",
	} {
		assert.True(t, validRedirectURI(ok), ok)
	}
	for _, bad := range []string{
		"", "http://evil.example.com/cb", "https://claude.ai/cb#frag", "javascript:alert(1)",
		"ftp://x/y", "https://user:pw@claude.ai/cb", "/relative",
	} {
		assert.False(t, validRedirectURI(bad), bad)
	}
}

func TestHandleMCP_Unauthorized(t *testing.T) {
	s := newUnitServer(t)
	r := newRequest("POST", "/mcp", []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	require.NoError(t, s.HandleMCP(r))
	assert.Equal(t, fasthttp.StatusUnauthorized, r.RequestCtx.Response.StatusCode())
	challenge := string(r.RequestCtx.Response.Header.Peek("WWW-Authenticate"))
	assert.Equal(t, `Bearer resource_metadata="http://wa.example.com/.well-known/oauth-protected-resource"`, challenge)

	// Wrong token shape is rejected before any lookup and flagged invalid_token.
	r = newRequest("POST", "/mcp", []byte(`{}`))
	r.RequestCtx.Request.Header.Set("Authorization", "Bearer not-ours")
	require.NoError(t, s.HandleMCP(r))
	assert.Equal(t, fasthttp.StatusUnauthorized, r.RequestCtx.Response.StatusCode())
	assert.Contains(t, string(r.RequestCtx.Response.Header.Peek("WWW-Authenticate")), `error="invalid_token"`)
}

func TestPublicEndpoints_DisabledReturn404(t *testing.T) {
	s := newUnitServer(t)
	s.app.Config.MCP.Enabled = false
	for _, h := range []func(*Server, string) int{
		func(s *Server, _ string) int {
			r := newRequest("POST", "/mcp", []byte(`{}`))
			_ = s.HandleMCP(r)
			return r.RequestCtx.Response.StatusCode()
		},
		func(s *Server, _ string) int {
			r := newRequest("GET", "/mcp", nil)
			_ = s.MethodNotAllowed(r)
			return r.RequestCtx.Response.StatusCode()
		},
		func(s *Server, _ string) int {
			r := newRequest("GET", "/x", nil)
			_ = s.ProtectedResourceMetadata(r)
			return r.RequestCtx.Response.StatusCode()
		},
		func(s *Server, _ string) int {
			r := newRequest("GET", "/x", nil)
			_ = s.AuthorizationServerMetadata(r)
			return r.RequestCtx.Response.StatusCode()
		},
		func(s *Server, _ string) int {
			r := newRequest("POST", "/x", []byte(`{}`))
			_ = s.Register(r)
			return r.RequestCtx.Response.StatusCode()
		},
		func(s *Server, _ string) int {
			r := newRequest("GET", "/x", nil)
			_ = s.Authorize(r)
			return r.RequestCtx.Response.StatusCode()
		},
		func(s *Server, _ string) int {
			r := newRequest("POST", "/x", nil)
			_ = s.Token(r)
			return r.RequestCtx.Response.StatusCode()
		},
	} {
		assert.Equal(t, fasthttp.StatusNotFound, h(s, ""))
	}
}

func TestMetadataDocuments(t *testing.T) {
	s := newUnitServer(t)
	s.app.Config.Server.PublicURL = "https://wa.example.com/"
	s.app.Config.Server.BasePath = "/whatomate/"

	r := newRequest("GET", "/.well-known/oauth-protected-resource", nil)
	require.NoError(t, s.ProtectedResourceMetadata(r))
	var prm map[string]any
	require.NoError(t, json.Unmarshal(r.RequestCtx.Response.Body(), &prm))
	assert.Equal(t, "https://wa.example.com/whatomate/mcp", prm["resource"])
	assert.Equal(t, []any{"https://wa.example.com/whatomate"}, prm["authorization_servers"])

	r = newRequest("GET", "/.well-known/oauth-authorization-server", nil)
	require.NoError(t, s.AuthorizationServerMetadata(r))
	var asm map[string]any
	require.NoError(t, json.Unmarshal(r.RequestCtx.Response.Body(), &asm))
	assert.Equal(t, "https://wa.example.com/whatomate", asm["issuer"])
	assert.Equal(t, "https://wa.example.com/whatomate/oauth/token", asm["token_endpoint"])
	assert.Equal(t, []any{"S256"}, asm["code_challenge_methods_supported"])
	assert.Equal(t, []any{"none"}, asm["token_endpoint_auth_methods_supported"])

	// Derived from forwarded headers when public_url is unset.
	s.app.Config.Server.PublicURL = ""
	r = newRequest("GET", "/x", nil)
	r.RequestCtx.Request.Header.Set("X-Forwarded-Proto", "https, http")
	r.RequestCtx.Request.Header.Set("X-Forwarded-Host", "proxy.example.com")
	assert.Equal(t, "https://proxy.example.com/whatomate", s.baseURL(r.RequestCtx))
}

func TestToken_UnsupportedGrantAndCORSPaths(t *testing.T) {
	s := newUnitServer(t)
	r := newRequest("POST", "/oauth/token", nil)
	r.RequestCtx.Request.Header.SetContentType("application/x-www-form-urlencoded")
	r.RequestCtx.Request.SetBodyString("grant_type=password&username=a&password=b")
	require.NoError(t, s.Token(r))
	assert.Equal(t, fasthttp.StatusBadRequest, r.RequestCtx.Response.StatusCode())
	assert.Contains(t, string(r.RequestCtx.Response.Body()), "unsupported_grant_type")

	for _, p := range []string{"/.well-known/oauth-authorization-server", "/oauth/token", "/oauth/register", "/mcp"} {
		assert.True(t, IsPublicCORSPath(p), p)
	}
	for _, p := range []string{"/oauth/authorize", "/api/oauth/consent", "/api/me", "/mcpx"} {
		assert.False(t, IsPublicCORSPath(p), p)
	}
}
