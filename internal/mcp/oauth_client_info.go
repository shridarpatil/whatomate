package mcp

import (
	"net"
	"net/url"
	"strings"

	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// VerifiedClaudeRedirectURIs are the exact redirect URIs of the hosted Claude
// apps. A consent request whose redirect_uri is one of these, or a loopback
// http URI (Claude Code / Claude Desktop), is shown as a recognised Claude
// app; anything else gets a phishing warning on the consent page.
var VerifiedClaudeRedirectURIs = []string{
	"https://claude.ai/api/mcp/auth_callback",
	"https://claude.com/api/mcp/auth_callback",
}

// isVerifiedRedirectURI reports whether raw is a recognised Claude redirect.
func isVerifiedRedirectURI(raw string) bool {
	for _, v := range VerifiedClaudeRedirectURIs {
		if raw == v {
			return true
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Fragment != "" {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1"
}

// redirectHost is the scheme://host[:port] the consent page names as the
// place access will be sent to.
func redirectHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(hostPort(u))
}

func hostPort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return net.JoinHostPort(u.Hostname(), p)
	}
	return u.Hostname()
}

// ClientInfo serves GET /api/oauth/client?client_id=&redirect_uri= for the
// consent page: the registered app name, whether the redirect URI matches,
// where access will be sent and whether that is a recognised Claude app.
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
	redirectURI := string(args.Peek("redirect_uri"))
	return r.SendEnvelope(map[string]any{
		"client_id":          client.ClientID,
		"client_name":        client.ClientName,
		"redirect_uri_valid": client.HasRedirectURI(redirectURI),
		"redirect_host":      redirectHost(redirectURI),
		"verified":           isVerifiedRedirectURI(redirectURI),
	})
}
