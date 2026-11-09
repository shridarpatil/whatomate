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
	// The router decodes the path before routing, so exclusions are checked on
	// both the raw and the decoded form (/api/%61uth/login is /api/auth/login).
	dec := decodedPath(p)
	if IsExcluded(method, p) || IsExcluded(method, dec) {
		return nil, "", fmt.Errorf("%s %s is not available to AI apps (session plumbing or human-only)", method, p)
	}
	e, _, ok := Match(method, dec)
	if !ok {
		return nil, "", fmt.Errorf("%s %s is not a known Whatomate API endpoint; use whatomate_list_areas and whatomate_describe_area to find the right path", method, p)
	}
	if err := checkLiteralSegments(e.Path, p); err != nil {
		return nil, "", err
	}
	return e, p, nil
}

// decodedPath returns p with every segment percent-decoded. Callers have
// already rejected segments that fail to decode or decode to a slash.
func decodedPath(p string) string {
	segs := splitPath(p)
	for i, seg := range segs {
		if d, err := url.PathUnescape(seg); err == nil {
			segs[i] = d
		}
	}
	return "/" + strings.Join(segs, "/")
}

// checkLiteralSegments allows percent-encoding only inside {param} values:
// every literal segment of the matched template must appear unencoded.
func checkLiteralSegments(tpl, raw string) error {
	tsegs, rsegs := splitPath(tpl), splitPath(raw)
	if len(tsegs) != len(rsegs) {
		return fmt.Errorf("path %s does not match %s", raw, tpl)
	}
	for i, t := range tsegs {
		if strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}") {
			continue
		}
		if rsegs[i] != t {
			return fmt.Errorf("path segment %q must be written as %q; percent-encoding is only allowed inside parameter values", rsegs[i], t)
		}
	}
	return nil
}
