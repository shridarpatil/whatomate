package mcp

import (
	"strings"

	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// publicCORSPaths get permissive CORS (any origin, no credentials): MCP
// clients may fetch discovery documents and the token/registration endpoints
// from a browser.
var publicCORSPaths = []string{"/.well-known/", "/oauth/token", "/oauth/register", "/mcp"}

// IsPublicCORSPath reports whether path (server-internal) gets permissive CORS.
func IsPublicCORSPath(path string) bool {
	for _, p := range publicCORSPaths {
		if path == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(path, p)) {
			return true
		}
	}
	return false
}

func firstHeaderValue(v string) string {
	if i := strings.Index(v, ","); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// baseURL is the public base of this installation including server.base_path:
// server.public_url when set, else scheme/host from X-Forwarded-Proto /
// X-Forwarded-Host / Host.
func (s *Server) baseURL(ctx *fasthttp.RequestCtx) string {
	bp := basePathOf(s.app.Config.Server.BasePath)
	if pub := strings.TrimRight(strings.TrimSpace(s.app.Config.Server.PublicURL), "/"); pub != "" {
		return pub + bp
	}
	scheme := firstHeaderValue(string(ctx.Request.Header.Peek("X-Forwarded-Proto")))
	host := firstHeaderValue(string(ctx.Request.Header.Peek("X-Forwarded-Host")))
	if scheme == "" {
		scheme = "http"
		if ctx.IsTLS() {
			scheme = "https"
		}
	}
	if host == "" {
		host = string(ctx.Host())
	}
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host + bp
}

// writeJSON writes a raw (non-envelope) JSON document, as OAuth requires.
func writeJSON(ctx *fasthttp.RequestCtx, status int, v any) error {
	b, err := marshalNoEscape(v)
	if err != nil {
		return err
	}
	ctx.Response.Header.Set("Cache-Control", "no-store")
	ctx.SetStatusCode(status)
	ctx.SetContentType("application/json")
	ctx.SetBody(b)
	return nil
}

// ProtectedResourceMetadata serves RFC 9728 metadata for the /mcp resource.
func (s *Server) ProtectedResourceMetadata(r *fastglue.Request) error {
	if !s.Enabled() {
		return notFound(r)
	}
	base := s.baseURL(r.RequestCtx)
	return writeJSON(r.RequestCtx, fasthttp.StatusOK, map[string]any{
		"resource":                 base + "/mcp",
		"authorization_servers":    []string{base},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Whatomate",
	})
}

// AuthorizationServerMetadata serves RFC 8414 metadata.
func (s *Server) AuthorizationServerMetadata(r *fastglue.Request) error {
	if !s.Enabled() {
		return notFound(r)
	}
	base := s.baseURL(r.RequestCtx)
	return writeJSON(r.RequestCtx, fasthttp.StatusOK, map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/oauth/authorize",
		"token_endpoint":                        base + "/oauth/token",
		"registration_endpoint":                 base + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"service_documentation":                 base + "/settings/ai-connector",
	})
}
