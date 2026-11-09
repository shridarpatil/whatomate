package mcp

import (
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/valyala/fasthttp"
)

// apiRequest is one in-process call into the /api router.
type apiRequest struct {
	Method      string
	Path        string // server-internal path, e.g. /api/contacts/{uuid}
	Query       url.Values
	Body        []byte
	ContentType string
}

// apiResponse is the captured response of an in-process call.
type apiResponse struct {
	Status      int
	ContentType string
	Body        []byte
}

var loopbackAddr = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}

// forwardedHeaders are copied from the /mcp request so handlers that build
// public links from the request (media URLs etc.) see the real host.
var forwardedHeaders = []string{"X-Forwarded-Proto", "X-Forwarded-Host"}

// dispatch runs req through the same fasthttp handler the HTTP server uses,
// authenticated as the caller: a freshly minted 5-minute app JWT for OAuth
// grants (same claims as a browser session in that org), or the caller's own
// X-API-Key. Rate limiting, permissions, org scoping and audit logging apply
// exactly as for the browser.
func (s *Server) dispatch(origin *fasthttp.RequestCtx, c *Caller, req apiRequest) (resp *apiResponse, err error) {
	h := s.rootHandler()
	if h == nil {
		return nil, errors.New("connector is not ready (no HTTP handler)")
	}

	var r fasthttp.Request
	uri := req.Path
	if len(req.Query) > 0 {
		uri += "?" + req.Query.Encode()
	}
	r.SetRequestURI(uri)
	r.Header.SetMethod(req.Method)
	if origin != nil {
		r.Header.SetHost(string(origin.Host()))
		for _, name := range forwardedHeaders {
			if v := origin.Request.Header.Peek(name); len(v) > 0 {
				r.Header.SetBytesV(name, v)
			}
		}
	} else {
		r.Header.SetHost("localhost")
	}
	r.Header.Set("Accept", "application/json")
	r.Header.Set("X-Whatomate-Client", "mcp")

	switch c.Kind {
	case CallerAPIKey:
		r.Header.Set("X-API-Key", c.APIKey)
	case CallerOAuth:
		if c.User == nil {
			return nil, errors.New("grant has no resolved user")
		}
		token, err := s.app.MintAccessToken(c.User, internalTokenTTL)
		if err != nil {
			return nil, fmt.Errorf("failed to mint internal token: %w", err)
		}
		r.Header.Set("Authorization", "Bearer "+token)
	default:
		return nil, errors.New("unknown caller kind")
	}

	if len(req.Body) > 0 {
		ct := req.ContentType
		if ct == "" {
			ct = "application/json"
		}
		r.Header.SetContentType(ct)
		r.SetBody(req.Body)
	}

	var fctx fasthttp.RequestCtx
	fctx.Init(&r, loopbackAddr, nil)

	defer func() {
		if rec := recover(); rec != nil {
			resp, err = nil, fmt.Errorf("internal error while handling %s %s", req.Method, req.Path)
			s.app.Log.Error("Panic during MCP dispatch", "error", rec, "method", req.Method, "path", req.Path)
		}
	}()
	h(&fctx)

	body := fctx.Response.Body()
	out := &apiResponse{
		Status:      fctx.Response.StatusCode(),
		ContentType: string(fctx.Response.Header.ContentType()),
		Body:        append([]byte(nil), body...),
	}
	return out, nil
}
