package mcp

// Small JSON-schema and parameter builders shared by the messaging, contacts,
// campaigns, RSVP, templates, tags, canned responses and import/export catalog
// files. Prefixed "mx" so they never collide with helpers in other areas.

func mxObj(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func mxStr(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func mxEnum(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

func mxInt(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func mxBool(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func mxArr(desc string, items map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": items}
}

func mxStrArr(desc string) map[string]any {
	return mxArr(desc, map[string]any{"type": "string"})
}

// mxStrMap is an object whose values are strings (e.g. template placeholder values).
func mxStrMap(desc string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          desc,
		"additionalProperties": map[string]any{"type": "string"},
	}
}

// mxFreeObj is an object with arbitrary keys.
func mxFreeObj(desc string) map[string]any {
	return map[string]any{"type": "object", "description": desc, "additionalProperties": true}
}

func mxQ(name, typ, desc string) Param {
	return Param{Name: name, Type: typ, Description: desc}
}

func mxQEnum(name, desc string, values ...string) Param {
	return Param{Name: name, Type: "string", Description: desc, Enum: values}
}

// mxPaging returns the standard page/limit query params (default limit 50, max 100).
func mxPaging() []Param {
	return []Param{
		mxQ("page", "integer", "Page number, starting at 1"),
		mxQ("limit", "integer", "Items per page (default 50, max 100)"),
	}
}

func mxWithPaging(ps ...Param) []Param {
	return append(mxPaging(), ps...)
}
