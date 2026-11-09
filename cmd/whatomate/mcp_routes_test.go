package main

import (
	"strings"
	"testing"

	"github.com/nikyjain/whatomate/internal/config"
	"github.com/nikyjain/whatomate/internal/handlers"
	"github.com/nikyjain/whatomate/internal/mcp"
	"github.com/nikyjain/whatomate/internal/middleware"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"github.com/zerodha/logf"
)

// newRoutedHandler wires the real setupRoutes (no DB needed for these paths).
func newRoutedHandler(t *testing.T, enabled bool) fasthttp.RequestHandler {
	t.Helper()
	cfg := &config.Config{MCP: config.MCPConfig{Enabled: enabled}}
	cfg.JWT.Secret = "test-secret-key-must-be-at-least-32-chars"
	lo := logf.New(logf.Opts{Level: logf.ErrorLevel})
	app := &handlers.App{Config: cfg, Log: lo}
	srv := mcp.New(app)
	g := fastglue.NewGlue()
	g.Before(middleware.CSRFProtection())
	setupRoutes(g, app, lo, "", nil, cfg, srv)
	h := middleware.ReportAPIErrors(corsWrapper(g.Handler(), map[string]bool{"https://app.example.com": true}), "")
	srv.SetHandler(h)
	return h
}

func serve(h fasthttp.RequestHandler, method, uri, origin string) *fasthttp.Response {
	var ctx fasthttp.RequestCtx
	var req fasthttp.Request
	req.Header.SetMethod(method)
	req.SetRequestURI(uri)
	req.Header.SetHost("wa.example.com")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	ctx.Init(&req, nil, nil)
	h(&ctx)
	var resp fasthttp.Response
	ctx.Response.CopyTo(&resp)
	return &resp
}

func TestMCPRoutes_PublicAndUnauthenticated(t *testing.T) {
	h := newRoutedHandler(t, true)

	resp := serve(h, "GET", "/.well-known/oauth-protected-resource", "")
	if resp.StatusCode() != 200 || !strings.Contains(string(resp.Body()), `"resource":"http://wa.example.com/mcp"`) {
		t.Fatalf("protected resource metadata: %d %s", resp.StatusCode(), resp.Body())
	}
	for _, p := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-authorization-server", "/.well-known/oauth-authorization-server/x"} {
		if resp := serve(h, "GET", p, ""); resp.StatusCode() != 200 {
			t.Errorf("%s: %d", p, resp.StatusCode())
		}
	}

	resp = serve(h, "POST", "/mcp", "")
	if resp.StatusCode() != 401 || !strings.Contains(string(resp.Header.Peek("WWW-Authenticate")), "resource_metadata=") {
		t.Fatalf("/mcp without credentials: %d %q", resp.StatusCode(), resp.Header.Peek("WWW-Authenticate"))
	}
	if resp := serve(h, "GET", "/mcp", ""); resp.StatusCode() != 405 {
		t.Errorf("GET /mcp: %d", resp.StatusCode())
	}
	if resp := serve(h, "DELETE", "/mcp", ""); resp.StatusCode() != 405 {
		t.Errorf("DELETE /mcp: %d", resp.StatusCode())
	}

	// The new /api endpoints still sit behind the normal /api auth hook.
	for _, p := range []string{"/api/mcp/info", "/api/mcp/grants", "/api/oauth/client"} {
		if resp := serve(h, "GET", p, ""); resp.StatusCode() != 401 {
			t.Errorf("%s without session: %d", p, resp.StatusCode())
		}
	}

	// Permissive CORS for discovery/token, normal CORS elsewhere.
	resp = serve(h, "OPTIONS", "/oauth/token", "https://claude.ai")
	if resp.StatusCode() != 204 || string(resp.Header.Peek("Access-Control-Allow-Origin")) != "*" {
		t.Errorf("token preflight: %d %q", resp.StatusCode(), resp.Header.Peek("Access-Control-Allow-Origin"))
	}
	resp = serve(h, "OPTIONS", "/api/contacts", "https://claude.ai")
	if got := string(resp.Header.Peek("Access-Control-Allow-Origin")); got != "" {
		t.Errorf("/api CORS must stay restricted, got %q", got)
	}
}

func TestMCPRoutes_DisabledReturn404(t *testing.T) {
	h := newRoutedHandler(t, false)
	for _, c := range [][2]string{
		{"POST", "/mcp"}, {"GET", "/.well-known/oauth-protected-resource"},
		{"GET", "/.well-known/oauth-authorization-server"}, {"GET", "/oauth/authorize"},
		{"POST", "/oauth/token"}, {"POST", "/oauth/register"},
	} {
		if resp := serve(h, c[0], c[1], ""); resp.StatusCode() != 404 {
			t.Errorf("%s %s with mcp disabled: %d", c[0], c[1], resp.StatusCode())
		}
	}
}
