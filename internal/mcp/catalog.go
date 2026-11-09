// Package mcp exposes Whatomate to MCP clients (Claude) as a remote MCP server.
//
// Every authenticated /api route the web UI can call is described by an Entry in
// the endpoint catalog (catalog_<area>.go files). Tool calls are dispatched
// in-process through the normal HTTP handler chain, so permissions, org scoping,
// validation and audit logging behave exactly as they do for the browser.
package mcp

import (
	"sort"
	"strings"
)

// Param describes a path or query parameter.
type Param struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // string | integer | boolean | number
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// MultipartSpec describes a multipart/form-data endpoint.
type MultipartSpec struct {
	FileField   string  `json:"file_field"`             // form field holding the file, e.g. "file"
	Accept      string  `json:"accept,omitempty"`       // e.g. "text/csv", "image/*"
	ExtraFields []Param `json:"extra_fields,omitempty"` // other form fields
}

// Entry is one catalogued API endpoint.
type Entry struct {
	Area        string         `json:"area"`
	Method      string         `json:"method"` // GET POST PUT PATCH DELETE
	Path        string         `json:"path"`   // route template exactly as registered, e.g. /api/contacts/{id}
	Summary     string         `json:"summary"`
	Permission  string         `json:"permission,omitempty"` // e.g. "contacts:write"; "" = any logged-in user; "super_admin"
	Query       []Param        `json:"query,omitempty"`
	Body        map[string]any `json:"body,omitempty"` // JSON schema of the JSON request body
	Multipart   *MultipartSpec `json:"multipart,omitempty"`
	SideEffects string         `json:"side_effects,omitempty"` // plain words, e.g. "sends WhatsApp messages"
	Tool        string         `json:"tool,omitempty"`         // featured tool name, e.g. "whatomate_send_text"
}

// Area groups endpoints for discovery.
type Area struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

var (
	areas   = map[string]Area{}
	entries []Entry
)

// registerArea and register are called from init() in catalog_<area>.go files.
func registerArea(a Area) { areas[a.Key] = a }

func register(es ...Entry) { entries = append(entries, es...) }

// Areas returns all areas sorted by key.
func Areas() []Area {
	out := make([]Area, 0, len(areas))
	for _, a := range areas {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Entries returns every catalogued endpoint.
func Entries() []Entry { return entries }

// EntriesForArea returns the endpoints in one area.
func EntriesForArea(key string) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.Area == key {
			out = append(out, e)
		}
	}
	return out
}

// Match finds the catalog entry for a concrete method + path
// (e.g. GET /api/contacts/3f6c…) and returns it with the extracted path params.
func Match(method, path string) (*Entry, map[string]string, bool) {
	method = strings.ToUpper(method)
	segs := splitPath(path)
	var best *Entry
	var bestParams map[string]string
	bestLiterals := -1
	// When several templates match (/api/x/import vs /api/x/{id}), the one with
	// the most literal segments wins, mirroring the router.
	for i := range entries {
		e := &entries[i]
		if e.Method != method {
			continue
		}
		tpl := splitPath(e.Path)
		params, ok := matchTemplate(tpl, segs)
		if !ok {
			continue
		}
		if lit := len(tpl) - len(params); lit > bestLiterals {
			best, bestParams, bestLiterals = e, params, lit
		}
	}
	return best, bestParams, best != nil
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func matchTemplate(tpl, segs []string) (map[string]string, bool) {
	if len(tpl) != len(segs) {
		return nil, false
	}
	params := map[string]string{}
	// Prefer literal segments: a literal mismatch fails; {x} matches any non-empty segment.
	for i, t := range tpl {
		if strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}") {
			if segs[i] == "" {
				return nil, false
			}
			params[strings.Trim(t, "{}")] = segs[i]
			continue
		}
		if t != segs[i] {
			return nil, false
		}
	}
	return params, true
}
