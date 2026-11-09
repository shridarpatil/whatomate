package mcp

import "strconv"

// Small JSON-schema builders shared by the setup-area catalog files
// (accounts, chatbot, whatsapp_flows, lead_ads, catalogs, webhooks,
// custom_actions). Prefixed "setup" so they never clash with helpers other
// catalog files define.

// setupObj builds an object schema. Pass required field names after props.
func setupObj(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// setupFreeObj is an object with free-form keys.
func setupFreeObj(desc string) map[string]any {
	return map[string]any{"type": "object", "description": desc, "additionalProperties": true}
}

func setupStr(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func setupEnum(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

func setupInt(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func setupBool(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func setupArr(items map[string]any, desc string) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": items}
}

func setupStrArr(desc string) map[string]any {
	return setupArr(map[string]any{"type": "string"}, desc)
}

// setupPage returns the standard page/limit query params (limit max 100).
func setupPage(defaultLimit int) []Param {
	return []Param{
		{Name: "page", Type: "integer", Description: "Page number, starting at 1."},
		{Name: "limit", Type: "integer", Description: "Items per page (max 100, default " + strconv.Itoa(defaultLimit) + ")."},
	}
}

func setupSearch(what string) Param {
	return Param{Name: "search", Type: "string", Description: "Text search on " + what + "."}
}
