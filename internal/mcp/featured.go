package mcp

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// featuredTool is a typed tool generated from a catalog entry with Tool set.
type featuredTool struct {
	def   toolDef
	entry Entry
}

// pathParams returns the {name} placeholders of a route template in order.
func pathParams(tpl string) []string {
	var out []string
	for _, seg := range splitPath(tpl) {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			out = append(out, strings.Trim(seg, "{}"))
		}
	}
	return out
}

func paramSchema(p Param) map[string]any {
	t := p.Type
	if t == "" {
		t = "string"
	}
	s := map[string]any{"type": t}
	if p.Description != "" {
		s["description"] = p.Description
	}
	if len(p.Enum) > 0 {
		s["enum"] = p.Enum
	}
	return s
}

// featuredTools builds one tool per catalog entry whose Tool field is set
// (multipart entries are reached via whatomate_upload instead). Input schema =
// path params + query params + a "body" property holding the entry's body.
func featuredTools() []featuredTool {
	var out []featuredTool
	seen := map[string]bool{}
	for _, e := range Entries() {
		if e.Tool == "" || e.Multipart != nil || seen[e.Tool] {
			continue
		}
		seen[e.Tool] = true
		props := map[string]any{}
		var required []string
		for _, name := range pathParams(e.Path) {
			props[name] = map[string]any{"type": "string", "description": "Path parameter " + name}
			required = append(required, name)
		}
		for _, q := range e.Query {
			props[q.Name] = paramSchema(q)
			if q.Required {
				required = append(required, q.Name)
			}
		}
		if e.Body != nil {
			props["body"] = e.Body
			switch req := e.Body["required"].(type) {
			case []string:
				if len(req) > 0 {
					required = append(required, "body")
				}
			case []any:
				if len(req) > 0 {
					required = append(required, "body")
				}
			}
		}
		desc := e.Summary + fmt.Sprintf(" (%s %s", e.Method, e.Path)
		if e.Permission != "" {
			desc += ", needs " + e.Permission
		}
		desc += ")"
		if e.SideEffects != "" {
			desc += ". Side effects: " + e.SideEffects
		}
		ann := readOnly(e.Summary)
		if e.Method != "GET" {
			ann = destructive(e.Summary)
		}
		out = append(out, featuredTool{
			def: toolDef{
				Name:        e.Tool,
				Description: desc,
				InputSchema: objectSchema(props, required...),
				Annotations: ann,
			},
			entry: e,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].def.Name < out[j].def.Name })
	return out
}

func findFeatured(name string) (*featuredTool, bool) {
	for _, ft := range featuredTools() {
		if ft.def.Name == name {
			ft := ft
			return &ft, true
		}
	}
	return nil, false
}

// buildFeaturedCall turns featured-tool arguments into method, path, query, body.
func buildFeaturedCall(e Entry, args map[string]any) (string, url.Values, any, error) {
	path := e.Path
	for _, name := range pathParams(e.Path) {
		v, ok := args[name]
		s := scalarString(v)
		if !ok || s == "" {
			return "", nil, nil, fmt.Errorf("missing required argument %q", name)
		}
		path = strings.Replace(path, "{"+name+"}", url.PathEscape(s), 1)
	}
	query := url.Values{}
	for _, q := range e.Query {
		if v, ok := args[q.Name]; ok && v != nil {
			addQueryValue(query, q.Name, v)
		}
	}
	return path, query, args["body"], nil
}
