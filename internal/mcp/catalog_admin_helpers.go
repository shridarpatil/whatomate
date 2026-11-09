package mcp

// Schema helpers shared by the users / roles / organisations / settings /
// analytics / calls / admin catalog files. The "adm" prefix keeps them from
// colliding with helpers in other catalog_<area>.go files.

func admObj(props map[string]any, required ...string) map[string]any {
	o := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		o["required"] = required
	}
	return o
}

func admStr(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func admEnum(desc string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": vals}
}

func admBool(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func admInt(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func admArr(items map[string]any, desc string) map[string]any {
	return map[string]any{"type": "array", "items": items, "description": desc}
}

// admPaging is the standard page/limit pair read by parsePagination.
func admPaging() []Param {
	return []Param{
		{Name: "page", Type: "integer", Description: "Page number, starting at 1 (default 1)."},
		{Name: "limit", Type: "integer", Description: "Rows per page (default 50, max 100)."},
	}
}

// admDateRange is the from/to pair used by analytics endpoints (YYYY-MM-DD).
func admDateRange() []Param {
	return []Param{
		{Name: "from", Type: "string", Description: "Start date, ISO format YYYY-MM-DD. Send together with to; default is the current month."},
		{Name: "to", Type: "string", Description: "End date, ISO format YYYY-MM-DD (inclusive)."},
	}
}
