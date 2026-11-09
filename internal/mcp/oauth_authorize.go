package mcp

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/nikyjain/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// PKCE code_challenge: base64url of a SHA-256 digest is 43 chars; RFC 7636
// allows 43-128 characters from the unreserved set.
var codeChallengeRe = regexp.MustCompile(`^[A-Za-z0-9\-._~]{43,128}$`)

// authParams are the authorization request parameters, carried unchanged from
// /oauth/authorize through the consent page to POST /api/oauth/consent.
type authParams struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	ResponseType        string `json:"response_type"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Scope               string `json:"scope"`
	Resource            string `json:"resource"`
}

// authFailure describes a rejected authorization request. Redirectable
// failures are reported to the client's redirect_uri; others are shown to the
// user because the redirect target cannot be trusted.
type authFailure struct {
	Redirectable bool
	Code         string
	Description  string
}

// validateAuthParams checks client, exact redirect URI, response type, PKCE
// S256 and the optional resource indicator.
func (s *Server) validateAuthParams(p authParams) (*models.MCPOAuthClient, *authFailure) {
	client, ok := s.findClient(p.ClientID)
	if !ok {
		return nil, &authFailure{false, "invalid_client", "Unknown client_id. Remove the connector and add it again."}
	}
	if !client.HasRedirectURI(p.RedirectURI) {
		return nil, &authFailure{false, "invalid_request", "redirect_uri does not match the registered redirect URIs."}
	}
	if p.ResponseType != "code" {
		return client, &authFailure{true, "unsupported_response_type", "response_type must be code"}
	}
	if p.CodeChallengeMethod != "S256" || !codeChallengeRe.MatchString(p.CodeChallenge) {
		return client, &authFailure{true, "invalid_request", "PKCE with code_challenge_method S256 is required"}
	}
	if p.Resource != "" {
		u, err := url.Parse(p.Resource)
		if err != nil || u.Host == "" || !strings.HasSuffix(strings.TrimRight(u.Path, "/"), "/mcp") {
			return client, &authFailure{true, "invalid_target", "resource must be this server's /mcp URL"}
		}
	}
	return client, nil
}

// errorRedirect builds redirect_uri?error=...&state=...
func errorRedirect(redirectURI, code, desc, state string) string {
	q := url.Values{}
	q.Set("error", code)
	if desc != "" {
		q.Set("error_description", desc)
	}
	if state != "" {
		q.Set("state", state)
	}
	return appendQuery(redirectURI, q)
}

func appendQuery(base string, q url.Values) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + q.Encode()
}

func authParamsFromArgs(a *fasthttp.Args) authParams {
	return authParams{
		ClientID:            string(a.Peek("client_id")),
		RedirectURI:         string(a.Peek("redirect_uri")),
		ResponseType:        string(a.Peek("response_type")),
		State:               string(a.Peek("state")),
		CodeChallenge:       string(a.Peek("code_challenge")),
		CodeChallengeMethod: string(a.Peek("code_challenge_method")),
		Scope:               string(a.Peek("scope")),
		Resource:            string(a.Peek("resource")),
	}
}

// Authorize serves GET /oauth/authorize: it validates the request and sends
// the browser to the frontend consent page with the original parameters.
func (s *Server) Authorize(r *fastglue.Request) error {
	if !s.Enabled() {
		return notFound(r)
	}
	ctx := r.RequestCtx
	p := authParamsFromArgs(ctx.QueryArgs())
	if _, fail := s.validateAuthParams(p); fail != nil {
		if fail.Redirectable {
			ctx.Redirect(errorRedirect(p.RedirectURI, fail.Code, fail.Description, p.State), fasthttp.StatusFound)
			return nil
		}
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetContentType("text/html; charset=utf-8")
		ctx.SetBodyString(fmt.Sprintf("<!doctype html><title>Cannot connect</title><h1>Cannot connect this app</h1><p>%s</p>",
			html.EscapeString(fail.Description)))
		return nil
	}
	consent := basePathOf(s.app.Config.Server.BasePath) + "/oauth/consent?" + string(ctx.QueryArgs().QueryString())
	ctx.Response.Header.Set("Location", consent)
	ctx.Response.Header.Set("Cache-Control", "no-store")
	ctx.SetStatusCode(fasthttp.StatusFound)
	return nil
}
