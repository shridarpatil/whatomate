package mcp

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/middleware"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/valyala/fasthttp"
)

// Caller kinds.
const (
	CallerAPIKey = "api_key"
	CallerOAuth  = "oauth"
)

// Access token prefixes.
const (
	accessTokenPrefix  = "wmcp_at_"
	refreshTokenPrefix = "wmcp_rt_"
	authCodePrefix     = "wmcp_ac_"
	clientIDPrefix     = "wmcp_c_"
)

var (
	errNoCredentials = errors.New("missing credentials")
	errInvalidToken  = errors.New("invalid or expired access token")
	errInvalidAPIKey = errors.New("invalid API key")
)

// Caller is the authenticated identity of one /mcp request.
type Caller struct {
	Kind   string
	UserID uuid.UUID
	OrgID  uuid.UUID // uuid.Nil for an API key without an organisation
	APIKey string    // forwarded as X-API-Key on dispatch (api_key callers)
	Grant  *models.MCPGrant
	User   *models.User // user resolved into Grant's org (oauth callers)
}

// authenticate resolves the caller of an /mcp request. A nil Caller means the
// request must be answered with 401; the error says why.
func (s *Server) authenticate(ctx *fasthttp.RequestCtx) (*Caller, error) {
	if key := strings.TrimSpace(string(ctx.Request.Header.Peek("X-API-Key"))); key != "" {
		id, ok := middleware.ValidateAPIKey(s.app.DB, key)
		if !ok {
			return nil, errInvalidAPIKey
		}
		c := &Caller{Kind: CallerAPIKey, UserID: id.UserID, APIKey: key}
		if id.OrganizationID != nil {
			c.OrgID = *id.OrganizationID
		}
		return c, nil
	}

	auth := strings.TrimSpace(string(ctx.Request.Header.Peek("Authorization")))
	if auth == "" {
		return nil, errNoCredentials
	}
	scheme, token, ok := strings.Cut(auth, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return nil, errInvalidToken
	}
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, accessTokenPrefix) {
		return nil, errInvalidToken
	}
	grant, err := s.lookupAccessToken(token)
	if err != nil {
		return nil, errInvalidToken
	}
	user, err := s.checkGrant(grant)
	if err != nil {
		return nil, errInvalidToken
	}
	return &Caller{
		Kind:   CallerOAuth,
		UserID: grant.UserID,
		OrgID:  grant.OrganizationID,
		Grant:  grant,
		User:   user,
	}, nil
}

// writeUnauthorized answers 401 with the RFC 9728 resource_metadata pointer so
// MCP clients can discover the authorization server.
func (s *Server) writeUnauthorized(ctx *fasthttp.RequestCtx, cause error) {
	challenge := fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`, s.baseURL(ctx))
	desc := "Authentication required: connect with OAuth or send an X-API-Key header"
	if errors.Is(cause, errInvalidToken) {
		challenge += `, error="invalid_token", error_description="The access token is invalid, expired or revoked"`
		desc = cause.Error()
	} else if errors.Is(cause, errInvalidAPIKey) {
		desc = cause.Error()
	}
	ctx.Response.Header.Set("WWW-Authenticate", challenge)
	ctx.SetStatusCode(fasthttp.StatusUnauthorized)
	ctx.SetContentType("application/json")
	body, _ := marshalNoEscape(map[string]string{"error": "unauthorized", "error_description": desc})
	ctx.SetBody(body)
}
