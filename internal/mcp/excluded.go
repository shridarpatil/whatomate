package mcp

import "strings"

// ExcludedRoute is an /api route deliberately NOT reachable through the MCP
// connector (session/infra plumbing, or unsafe for an agent).
//
// Method "*" matches any method. A Path ending in "/*" matches the path itself
// and everything below it; otherwise Path is a route template matched segment
// by segment ({x} matches any one segment).
type ExcludedRoute struct {
	Method string
	Path   string
	Reason string
}

const apiKeysOutliveReason = "API keys outlive the AI connection; create or re-enable keys in Whatomate"

// ExcludedRoutes is the explicit exclusion list checked by the catalog parity
// test: every /api route registered in cmd/whatomate/main.go must be either
// catalogued or listed here.
var ExcludedRoutes = []ExcludedRoute{
	{"*", "/api/auth/*", "login, register, refresh, logout, switch-org, ws-token and SSO are browser-session plumbing"},
	{"*", "/api/webhook", "Meta WhatsApp callback"},
	{"*", "/api/webhook/*", "Meta callbacks (incl. Lead Ads leadgen)"},
	{"*", "/api/custom-actions/redirect/*", "one-time browser redirect token"},
	{"*", "/api/me/password", "an AI app must never change the user's password"},
	{"*", "/api/embedded-signup/config", "public Embedded Signup bootstrap, no data"},
	{"*", "/api/oauth/*", "MCP OAuth consent stays human-only"},
	{"POST", "/api/api-keys", apiKeysOutliveReason},
	{"PUT", "/api/api-keys/{id}", apiKeysOutliveReason},
	{"*", "/public/*", "public media links, not API"},
	{"*", "/ws", "WebSocket"},
	{"*", "/health", "infrastructure probe"},
	{"*", "/ready", "infrastructure probe"},
}

// IsExcluded reports whether method + path (concrete path or route template)
// falls under ExcludedRoutes.
func IsExcluded(method, path string) bool {
	method = strings.ToUpper(method)
	for _, ex := range ExcludedRoutes {
		if ex.Method != "*" && !strings.EqualFold(ex.Method, method) {
			continue
		}
		if excludedPathMatches(ex.Path, path) {
			return true
		}
	}
	return false
}

func excludedPathMatches(pattern, path string) bool {
	path = "/" + strings.Trim(path, "/")
	if prefix, ok := strings.CutSuffix(pattern, "/*"); ok {
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	_, ok := matchTemplate(splitPath(pattern), splitPath(path))
	return ok
}
