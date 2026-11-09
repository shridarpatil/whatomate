package mcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCallPath_Accepts(t *testing.T) {
	useTestCatalog(t)
	e, p, err := resolveCallPath("get", "/api/contacts/abc-123", "")
	require.NoError(t, err)
	assert.Equal(t, "/api/contacts/{id}", e.Path)
	assert.Equal(t, "/api/contacts/abc-123", p)

	// The literal route wins over the {id} template.
	e, _, err = resolveCallPath("POST", "/api/contacts/import", "")
	require.NoError(t, err)
	assert.Equal(t, "/api/contacts/import", e.Path)

	// A leading base path is stripped; a missing leading slash is tolerated.
	_, p, err = resolveCallPath("GET", "/whatomate/api/contacts", "/whatomate/")
	require.NoError(t, err)
	assert.Equal(t, "/api/contacts", p)
	_, p, err = resolveCallPath("GET", "api/contacts/", "")
	require.NoError(t, err)
	assert.Equal(t, "/api/contacts", p)
}

func TestResolveCallPath_Rejects(t *testing.T) {
	useTestCatalog(t)
	cases := map[string][2]string{
		"traversal":         {"GET", "/api/contacts/../users"},
		"encoded traversal": {"GET", "/api/contacts/%2e%2e"},
		"encoded slash":     {"GET", "/api/contacts/a%2Fb"},
		"backslash":         {"GET", "/api/contacts\\x"},
		"non-api":           {"GET", "/health"},
		"mcp itself":        {"POST", "/mcp"},
		"query in path":     {"GET", "/api/contacts?search=x"},
		"unknown":           {"GET", "/api/does-not-exist"},
		"wrong method":      {"DELETE", "/api/contacts"},
		"bad method":        {"TRACE", "/api/contacts"},
		"login":             {"POST", "/api/auth/login"},
		"switch-org":        {"POST", "/api/auth/switch-org"},
		"password":          {"PUT", "/api/me/password"},
		"consent":           {"POST", "/api/oauth/consent"},
		"webhook":           {"POST", "/api/webhook"},
		"empty segment":     {"GET", "/api//contacts"},
	}
	for name, c := range cases {
		_, _, err := resolveCallPath(c[0], c[1], "")
		assert.Error(t, err, name)
	}
	// Excluded routes stay excluded even if a catalog entry existed for them.
	withTestEntries(t, nil, Entry{Area: "zz_test", Method: "POST", Path: "/api/auth/login"})
	_, _, err := resolveCallPath("POST", "/api/auth/login", "")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not available"))
}

func TestIsExcluded(t *testing.T) {
	excluded := []string{
		"/api/auth/login", "/api/auth/sso/{provider}/init", "/api/webhook", "/api/webhook/leadgen",
		"/api/custom-actions/redirect", "/api/custom-actions/redirect/{token}", "/api/me/password",
		"/api/embedded-signup/config", "/api/oauth/consent", "/api/oauth/client",
	}
	for _, p := range excluded {
		assert.True(t, IsExcluded("GET", p), p)
	}
	for _, p := range []string{"/api/me", "/api/contacts", "/api/custom-actions", "/api/webhooks", "/api/mcp/grants"} {
		assert.False(t, IsExcluded("GET", p), p)
	}
}
