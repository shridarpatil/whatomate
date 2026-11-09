package mcp

import (
	"sync"
	"time"

	"github.com/nikyjain/whatomate/internal/handlers"
	"github.com/nikyjain/whatomate/internal/middleware"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// ServerVersion is reported in the MCP initialize result. main sets it to the
// build version.
var ServerVersion = "dev"

const (
	// maxResultChars caps the text of one tool result.
	maxResultChars = 60000
	// maxUploadBytes caps a decoded whatomate_upload file.
	maxUploadBytes = 16 << 20
	// internalTokenTTL is the lifetime of the app JWT minted per dispatched call.
	internalTokenTTL = 5 * time.Minute
	accessTokenTTL   = time.Hour
	refreshTokenTTL  = 30 * 24 * time.Hour
	authCodeTTL      = 5 * time.Minute
)

// Server is the remote MCP server plus its OAuth 2.1 authorization server.
type Server struct {
	app *handlers.App

	mu      sync.RWMutex
	handler fasthttp.RequestHandler

	registerLimit fastglue.FastMiddleware
}

// New creates the MCP server. Call SetHandler with the final server handler
// once routes are set up, before serving traffic.
func New(app *handlers.App) *Server {
	s := &Server{app: app}
	if app.Redis != nil {
		s.registerLimit = middleware.RateLimit(middleware.RateLimitOpts{
			Redis:      app.Redis,
			Log:        app.Log,
			Max:        20,
			Window:     time.Hour,
			KeyPrefix:  "mcp_register",
			TrustProxy: app.Config.RateLimit.TrustProxy,
		})
	}
	return s
}

// SetHandler stores the fully wrapped fasthttp handler the HTTP server uses, so
// tool calls run through exactly the same middleware and routes as the browser.
func (s *Server) SetHandler(h fasthttp.RequestHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = h
}

func (s *Server) rootHandler() fasthttp.RequestHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handler
}

// Enabled reports whether the connector is switched on (mcp.enabled).
func (s *Server) Enabled() bool {
	return s.app.Config != nil && s.app.Config.MCP.Enabled
}

// notFound answers a disabled public endpoint.
func notFound(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Not found", nil, "")
}

// MethodNotAllowed answers GET/DELETE /mcp: this server is stateless and offers
// no SSE stream or session to delete.
func (s *Server) MethodNotAllowed(r *fastglue.Request) error {
	if !s.Enabled() {
		return notFound(r)
	}
	r.RequestCtx.Response.Header.Set("Allow", "POST")
	return r.SendErrorEnvelope(fasthttp.StatusMethodNotAllowed, "Method not allowed; use POST", nil, "")
}

// HandleMCP serves POST /mcp (JSON-RPC 2.0).
func (s *Server) HandleMCP(r *fastglue.Request) error {
	if !s.Enabled() {
		return notFound(r)
	}
	ctx := r.RequestCtx
	caller, authErr := s.authenticate(ctx)
	if caller == nil {
		s.writeUnauthorized(ctx, authErr)
		return nil
	}
	if caller.Grant != nil {
		s.touchGrant(caller.Grant)
	}

	status, body := s.handleRPC(ctx, caller, ctx.PostBody())
	ctx.SetStatusCode(status)
	if body != nil {
		ctx.SetContentType("application/json")
		ctx.SetBody(body)
	}
	return nil
}
