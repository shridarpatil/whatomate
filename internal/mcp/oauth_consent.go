package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/handlers"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// authCode is what a one-time authorization code is bound to (stored in Redis).
type authCode struct {
	ClientID      string    `json:"client_id"`
	RedirectURI   string    `json:"redirect_uri"`
	CodeChallenge string    `json:"code_challenge"`
	UserID        uuid.UUID `json:"user_id"`
	OrgID         uuid.UUID `json:"organization_id"`
	Resource      string    `json:"resource,omitempty"`
	Scope         string    `json:"scope,omitempty"`
}

func authCodeKey(code string) string { return "mcp:oauth:code:" + hashToken(code) }

type consentRequest struct {
	authParams
	OrganizationID string `json:"organization_id"`
	Approve        bool   `json:"approve"`
}

// sessionUser returns the user of a browser-session request. API-key and
// connector-dispatched requests are refused: consent must be given by a person.
func sessionUser(r *fastglue.Request) (uuid.UUID, bool) {
	h := &r.RequestCtx.Request.Header
	if len(h.Peek("X-API-Key")) > 0 || strings.EqualFold(string(h.Peek("X-Whatomate-Client")), "mcp") {
		return uuid.Nil, false
	}
	id, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// Consent serves POST /api/oauth/consent (browser session only). On approval
// it stores a 5-minute single-use code and returns the redirect URL.
func (s *Server) Consent(r *fastglue.Request) error {
	if !s.Enabled() {
		return notFound(r)
	}
	userID, ok := sessionUser(r)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Connecting an AI app must be approved by a signed-in person", nil, "")
	}
	var req consentRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}
	req.ResponseType = "code"
	if _, fail := s.validateAuthParams(req.authParams); fail != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, fail.Description, nil, "")
	}
	if !req.Approve {
		return r.SendEnvelope(map[string]string{
			"redirect_url": errorRedirect(req.RedirectURI, "access_denied", "The user denied access", req.State),
		})
	}
	orgID, err := uuid.Parse(req.OrganizationID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "organization_id is required", nil, "")
	}
	if _, err := s.app.ResolveOrgUser(userID, orgID); err != nil {
		if errors.Is(err, handlers.ErrNotOrgMember) || errors.Is(err, handlers.ErrOrgNotFound) {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You are not a member of this organization", nil, "")
		}
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Your account cannot be connected", nil, "")
	}

	code, err := newSecret(authCodePrefix)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create authorization code", nil, "")
	}
	payload, _ := json.Marshal(authCode{
		ClientID: req.ClientID, RedirectURI: req.RedirectURI, CodeChallenge: req.CodeChallenge,
		UserID: userID, OrgID: orgID, Resource: req.Resource, Scope: req.Scope,
	})
	rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.app.Redis.Set(rctx, authCodeKey(code), payload, authCodeTTL).Err(); err != nil {
		s.app.Log.Error("Failed to store MCP authorization code", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create authorization code", nil, "")
	}
	q := url.Values{}
	q.Set("code", code)
	if req.State != "" {
		q.Set("state", req.State)
	}
	return r.SendEnvelope(map[string]string{"redirect_url": appendQuery(req.RedirectURI, q)})
}

// consumeAuthCode returns the code's binding and deletes it; a code can be
// redeemed once (only one DEL can succeed).
func (s *Server) consumeAuthCode(code string) (*authCode, bool) {
	if !strings.HasPrefix(code, authCodePrefix) {
		return nil, false
	}
	rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := authCodeKey(code)
	raw, err := s.app.Redis.Get(rctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	if n, err := s.app.Redis.Del(rctx, key).Result(); err != nil || n != 1 {
		return nil, false
	}
	var ac authCode
	if err := json.Unmarshal(raw, &ac); err != nil {
		return nil, false
	}
	return &ac, true
}

// ClientInfo serves GET /api/oauth/client?client_id=&redirect_uri= for the
// consent page: the registered app name and whether the redirect URI matches.
func (s *Server) ClientInfo(r *fastglue.Request) error {
	if !s.Enabled() {
		return notFound(r)
	}
	if _, ok := sessionUser(r); !ok {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Browser session required", nil, "")
	}
	args := r.RequestCtx.QueryArgs()
	client, ok := s.findClient(string(args.Peek("client_id")))
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Unknown app. Remove the connector and add it again.", nil, "")
	}
	return r.SendEnvelope(map[string]any{
		"client_id":          client.ClientID,
		"client_name":        client.ClientName,
		"redirect_uri_valid": client.HasRedirectURI(string(args.Peek("redirect_uri"))),
	})
}
