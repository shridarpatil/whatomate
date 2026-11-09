package mcp

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func callArgs(t *testing.T, s *Server, c *Caller, tool, args string) (string, bool) {
	t.Helper()
	_, resp := rpc(t, s, c, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+tool+`","arguments":`+args+`}}`)
	return toolText(t, resp)
}

// Fix 1: an AI app must never change the caller's own password or email.
func TestSelfCredentialChange_Blocked(t *testing.T) {
	withTestEntries(t, []Area{testArea},
		Entry{Area: "zz_test", Method: "PUT", Path: "/api/users/{id}", Summary: "Edit user", Tool: "whatomate_update_user",
			Body: map[string]any{"type": "object"}},
		Entry{Area: "zz_test", Method: "POST", Path: "/api/users/{id}/avatar", Summary: "Upload avatar",
			Multipart: &MultipartSpec{FileField: "file"}},
	)
	s := newUnitServer(t)
	f := &fakeHandler{body: `{"status":"success","data":{}}`}
	s.SetHandler(f.handle)
	self := uuid.New()
	c := &Caller{Kind: CallerAPIKey, APIKey: "whm_x", UserID: self}
	other := uuid.New()

	blocked := []string{
		`{"method":"PUT","path":"/api/users/` + self.String() + `","body":{"password":"n3w-secret"}}`,
		`{"method":"PUT","path":"/api/users/` + self.String() + `","body":{"Email":"evil@example.com"}}`,
		`{"method":"PUT","path":"/api/users/` + strings.ToUpper(self.String()) + `","body":{"full_name":"x","PASSWORD":"y"}}`,
		`{"method":"PUT","path":"/api/users/` + strings.Replace(self.String(), "-", "%2D", 1) + `","body":{"email":"a@b.c"}}`,
	}
	for _, args := range blocked {
		text, isErr := callArgs(t, s, c, "whatomate_call", args)
		assert.True(t, isErr, args)
		assert.Contains(t, text, "changing your own password or email is not available to AI apps")
	}
	text, isErr := callArgs(t, s, c, "whatomate_update_user", `{"id":"`+self.String()+`","body":{"password":"x"}}`)
	assert.True(t, isErr, text)
	assert.Contains(t, text, "under your profile")
	assert.Equal(t, 0, f.calls, "blocked calls must never reach the handler")

	// Editing your own name, or another user's password, still goes through.
	_, isErr = callArgs(t, s, c, "whatomate_call", `{"method":"PUT","path":"/api/users/`+self.String()+`","body":{"full_name":"Asha"}}`)
	assert.False(t, isErr)
	_, isErr = callArgs(t, s, c, "whatomate_call", `{"method":"PUT","path":"/api/users/`+other.String()+`","body":{"password":"temp-pass","email":"o@x.y"}}`)
	assert.False(t, isErr)
	assert.Equal(t, 2, f.calls)

	// Form fields of an upload are checked too.
	b64 := base64.StdEncoding.EncodeToString([]byte("img"))
	assert.Error(t, guardSelfCredentialChange(c, "PUT", &Entry{Path: selfUserRoute}, "/api/users/"+self.String(), []string{"password"}))
	assert.NoError(t, guardSelfCredentialChange(c, "POST", &Entry{Path: selfUserRoute}, "/api/users/"+self.String(), []string{"password"}))
	_, isErr = callArgs(t, s, c, "whatomate_upload", `{"path":"/api/users/`+self.String()+`/avatar","file_base64":"`+b64+`","filename":"a.png","fields":{"email":"x"}}`)
	assert.False(t, isErr, "only PUT/PATCH /api/users/{id} is guarded")
}

// Fix 2: API keys cannot be minted or re-enabled through the connector.
func TestAPIKeyMinting_Excluded(t *testing.T) {
	assert.True(t, IsExcluded("POST", "/api/api-keys"))
	assert.True(t, IsExcluded("PUT", "/api/api-keys/{id}"))
	assert.True(t, IsExcluded("put", "/api/api-keys/"+uuid.NewString()))
	assert.False(t, IsExcluded("GET", "/api/api-keys"))
	assert.False(t, IsExcluded("GET", "/api/api-keys/{id}"))
	assert.False(t, IsExcluded("DELETE", "/api/api-keys/{id}"))
	for _, e := range Entries() {
		if e.Path == "/api/api-keys" || e.Path == "/api/api-keys/{id}" {
			assert.NotContains(t, []string{"POST /api/api-keys", "PUT /api/api-keys/{id}"}, e.Method+" "+e.Path)
		}
	}
	_, _, err := resolveCallPath("POST", "/api/api-keys", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
	_, _, err = resolveCallPath("DELETE", "/api/api-keys/"+uuid.NewString(), "")
	assert.NoError(t, err)
}

// Fix 7: percent-encoding is only allowed inside parameter values.
func TestResolveCallPath_EncodedLiterals(t *testing.T) {
	useTestCatalog(t)
	for _, p := range []string{"/api/%61uth/login", "/api/oauth/%63onsent", "/api/me/%70assword", "/api/custom-actions/%72edirect/x", "/api/%63ontacts/abc"} {
		_, _, err := resolveCallPath("POST", p, "")
		assert.Error(t, err, p)
		_, _, err = resolveCallPath("GET", p, "")
		assert.Error(t, err, p)
	}
	_, _, err := resolveCallPath("GET", "/api/%63ontacts/abc", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "percent-encoding is only allowed inside parameter values")
	// Encoding inside a parameter value is fine.
	e, p, err := resolveCallPath("GET", "/api/contacts/a%20b", "")
	require.NoError(t, err)
	assert.Equal(t, "/api/contacts/{id}", e.Path)
	assert.Equal(t, "/api/contacts/a%20b", p)
}

// Fix 8: responses are capped at 2 MB; oversized uploads are refused early.
func TestDispatch_TruncatesLargeResponses(t *testing.T) {
	useTestCatalog(t)
	s := newUnitServer(t)
	f := &fakeHandler{contentType: "text/csv", body: strings.Repeat("a", maxResponseBytes+10)}
	s.SetHandler(f.handle)
	resp, err := s.dispatch(nil, apiKeyCaller, apiRequest{Method: "GET", Path: "/api/contacts"})
	require.NoError(t, err)
	assert.True(t, resp.Truncated)
	assert.Len(t, resp.Body, maxResponseBytes)
	text, isErr := callArgs(t, s, apiKeyCaller, "whatomate_call", `{"method":"GET","path":"/api/contacts"}`)
	assert.False(t, isErr)
	assert.True(t, strings.HasSuffix(text, "response truncated at 2 MB]"))

	f.body = "small"
	resp, err = s.dispatch(nil, apiKeyCaller, apiRequest{Method: "GET", Path: "/api/contacts"})
	require.NoError(t, err)
	assert.False(t, resp.Truncated)
}

func TestUpload_RejectsOversizedBase64BeforeDecoding(t *testing.T) {
	useTestCatalog(t)
	s := newUnitServer(t)
	f := &fakeHandler{body: `{"status":"success","data":{}}`}
	s.SetHandler(f.handle)
	assert.Equal(t, base64.StdEncoding.EncodedLen(maxUploadBytes), maxUploadBase64Len)
	assert.Equal(t, 4, base64PayloadLen("data:text/csv;base64, QU\nJD\t"))
	tooBig := strings.Repeat("A", maxUploadBase64Len+4)
	text, isErr := callArgs(t, s, apiKeyCaller, "whatomate_upload", `{"path":"/api/contacts/import","file_base64":"`+tooBig+`","filename":"a.csv"}`)
	assert.True(t, isErr)
	assert.Contains(t, text, "larger than 16 MB")
	assert.Equal(t, 0, f.calls)
}

// Fix 6: production without server.public_url warns at startup.
func TestPublicURLWarning(t *testing.T) {
	cfg := &config.Config{MCP: config.MCPConfig{Enabled: true}, App: config.AppConfig{Environment: "production"}}
	assert.Contains(t, PublicURLWarning(cfg), "server.public_url")
	cfg.Server.PublicURL = "https://wa.example.com"
	assert.Empty(t, PublicURLWarning(cfg))
	cfg.Server.PublicURL, cfg.App.Environment = "", "development"
	assert.Empty(t, PublicURLWarning(cfg))
	cfg.App.Environment, cfg.MCP.Enabled = "production", false
	assert.Empty(t, PublicURLWarning(cfg))
	assert.Empty(t, PublicURLWarning(nil))
}

// Fix 3: only the hosted Claude callbacks and loopback URIs are "verified".
func TestVerifiedRedirectURI(t *testing.T) {
	for _, u := range []string{
		"https://claude.ai/api/mcp/auth_callback", "https://claude.com/api/mcp/auth_callback",
		"http://localhost:33418/callback", "http://127.0.0.1:5000/cb", "http://localhost/cb",
	} {
		assert.True(t, isVerifiedRedirectURI(u), u)
	}
	for _, u := range []string{
		"https://claude.ai/api/mcp/auth_callback/x", "https://claude.ai.evil.com/api/mcp/auth_callback",
		"https://evil.example/cb", "http://localhost.evil.com/cb", "http://127.0.0.1@evil.com/cb",
		"https://localhost:3000/cb", "http://[::1]:80/cb", "",
	} {
		assert.False(t, isVerifiedRedirectURI(u), u)
	}
	assert.Equal(t, "https://evil.example", redirectHost("https://Evil.Example/cb?x=1"))
	assert.Equal(t, "http://127.0.0.1:5000", redirectHost("http://127.0.0.1:5000/cb"))
	assert.Equal(t, "", redirectHost("not a url"))
}
