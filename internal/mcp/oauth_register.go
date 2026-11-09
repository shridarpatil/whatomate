package mcp

import (
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/nikyjain/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

const (
	maxRedirectURIs   = 10
	maxClientNameLen  = 200
	maxRedirectURILen = 2000
)

// oauthError writes an RFC 6749 style error document.
func oauthError(ctx *fasthttp.RequestCtx, status int, code, desc string) error {
	return writeJSON(ctx, status, map[string]string{"error": code, "error_description": desc})
}

// validRedirectURI accepts https URIs anywhere, and http only on loopback
// (localhost / 127.0.0.1 / [::1], any port). Fragments are never allowed.
func validRedirectURI(raw string) bool {
	if raw == "" || len(raw) > maxRedirectURILen {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || strings.Contains(raw, "#") || u.Host == "" || u.User != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	default:
		return false
	}
}

type registerRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

// Register implements RFC 7591 dynamic client registration for public clients.
func (s *Server) Register(r *fastglue.Request) error {
	if !s.Enabled() {
		return notFound(r)
	}
	ctx := r.RequestCtx
	// The global cap runs first and fails closed, so the per-IP limiter below
	// (which fails open) only ever runs while Redis is answering.
	if status, desc := s.checkGlobalRegisterLimit(); status != 0 {
		return oauthError(ctx, status, "temporarily_unavailable", desc)
	}
	if s.registerLimit != nil && s.registerLimit(r) == nil {
		return nil // rate limited; 429 already written
	}
	var req registerRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_client_metadata", "body must be a JSON client metadata document")
	}
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > maxRedirectURIs {
		return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_redirect_uri", "1 to 10 redirect_uris are required")
	}
	for _, u := range req.RedirectURIs {
		if !validRedirectURI(u) {
			return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_redirect_uri",
				"redirect URIs must be https, or http on localhost/127.0.0.1, without a fragment")
		}
	}
	if m := req.TokenEndpointAuthMethod; m != "" && m != "none" {
		return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_client_metadata", "only public clients (token_endpoint_auth_method \"none\") are supported")
	}
	name := strings.TrimSpace(req.ClientName)
	if name == "" {
		name = "AI app"
	}
	if r := []rune(name); len(r) > maxClientNameLen {
		name = string(r[:maxClientNameLen])
	}

	clientID, err := newSecret(clientIDPrefix)
	if err != nil {
		return oauthError(ctx, fasthttp.StatusInternalServerError, "server_error", "failed to create client")
	}
	clientID = clientID[:len(clientIDPrefix)+32] // 16 random bytes are plenty for an identifier
	client := models.MCPOAuthClient{ClientID: clientID, ClientName: name, RedirectURIs: models.StringArray(req.RedirectURIs)}
	if err := s.app.DB.Create(&client).Error; err != nil {
		s.app.Log.Error("Failed to register MCP OAuth client", "error", err)
		return oauthError(ctx, fasthttp.StatusInternalServerError, "server_error", "failed to create client")
	}
	s.pruneUnusedClients()
	return writeJSON(ctx, fasthttp.StatusCreated, map[string]any{
		"client_id":                  client.ClientID,
		"client_name":                client.ClientName,
		"redirect_uris":              req.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"client_id_issued_at":        time.Now().Unix(),
	})
}

// findClient loads a registered client by client_id.
func (s *Server) findClient(clientID string) (*models.MCPOAuthClient, bool) {
	if clientID == "" || len(clientID) > 64 {
		return nil, false
	}
	var c models.MCPOAuthClient
	if err := s.app.DB.Where("client_id = ?", clientID).First(&c).Error; err != nil {
		return nil, false
	}
	return &c, true
}
