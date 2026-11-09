package mcp

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var allowedMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

// basePathOf normalises server.base_path to "" or "/segment[/segment]".
func basePathOf(raw string) string {
	p := strings.TrimSpace(raw)
	p = strings.ReplaceAll(p, "\\", "/")
	p = "/" + strings.Trim(p, "/")
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if p == "/" {
		return ""
	}
	return p
}

// resolveCallPath validates a tool-supplied method + path and returns the
// catalog entry it hits and the server-internal path to dispatch to.
//
// Rules: method must be GET/POST/PUT/PATCH/DELETE; the path must be an /api
// path (a leading server.base_path is stripped), contain no query string,
// fragment, "..", "." or encoded slashes; it must not be an excluded route and
// must match a catalog entry.
func resolveCallPath(method, rawPath, basePath string) (*Entry, string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if !allowedMethods[method] {
		return nil, "", fmt.Errorf("method must be one of GET, POST, PUT, PATCH, DELETE")
	}
	p := strings.TrimSpace(rawPath)
	if p == "" {
		return nil, "", errors.New("path is required, e.g. /api/contacts")
	}
	if strings.ContainsAny(p, "?#") {
		return nil, "", errors.New("put query parameters in the `query` argument, not in the path")
	}
	if strings.Contains(p, "\\") {
		return nil, "", errors.New("path must not contain backslashes")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if bp := basePathOf(basePath); bp != "" && strings.HasPrefix(p, bp+"/api/") {
		p = strings.TrimPrefix(p, bp)
	}
	if !strings.HasPrefix(p, "/api/") {
		return nil, "", errors.New("only /api/... paths can be called")
	}
	if strings.Contains(p, "//") {
		return nil, "", errors.New("path must not contain empty segments")
	}
	for _, seg := range strings.Split(strings.Trim(p, "/"), "/") {
		dec, err := url.PathUnescape(seg)
		if err != nil {
			return nil, "", fmt.Errorf("invalid path segment %q", seg)
		}
		if dec == "." || dec == ".." || strings.ContainsAny(dec, "/\\") {
			return nil, "", fmt.Errorf("path segment %q is not allowed", seg)
		}
	}
	p = strings.TrimSuffix(p, "/")
	if IsExcluded(method, p) {
		return nil, "", fmt.Errorf("%s %s is not available to AI apps (session plumbing or human-only)", method, p)
	}
	e, _, ok := Match(method, p)
	if !ok {
		return nil, "", fmt.Errorf("%s %s is not a known Whatomate API endpoint; use whatomate_list_areas and whatomate_describe_area to find the right path", method, p)
	}
	return e, p, nil
}
