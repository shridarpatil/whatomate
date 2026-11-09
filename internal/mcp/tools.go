package mcp

// toolDef is one MCP tool as returned by tools/list.
type toolDef struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

// Static tool names.
const (
	toolWhoami        = "whatomate_whoami"
	toolListOrgs      = "whatomate_list_organizations"
	toolSwitchOrg     = "whatomate_switch_organization"
	toolListAreas     = "whatomate_list_areas"
	toolDescribeArea  = "whatomate_describe_area"
	toolCall          = "whatomate_call"
	toolUpload        = "whatomate_upload"
	methodDescription = "HTTP method: GET reads; POST/PUT/PATCH/DELETE change data"
)

func readOnly(title string) map[string]any {
	return map[string]any{"title": title, "readOnlyHint": true, "destructiveHint": false, "openWorldHint": false}
}

func destructive(title string) map[string]any {
	return map[string]any{"title": title, "readOnlyHint": false, "destructiveHint": true, "openWorldHint": true}
}

func objectSchema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

var methodEnum = map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}, "description": methodDescription}

// staticTools is the fixed discovery + generic access tool list.
func staticTools() []toolDef {
	return []toolDef{
		{
			Name:        toolWhoami,
			Description: "Who Claude is acting as: the user, email, current organisation, role and permission list, and how the connector is authenticated.",
			InputSchema: objectSchema(map[string]any{}),
			Annotations: readOnly("Who am I"),
		},
		{
			Name:        toolListOrgs,
			Description: "Organisations the connected user belongs to, with their role in each, and which one is current.",
			InputSchema: objectSchema(map[string]any{}),
			Annotations: readOnly("List organisations"),
		},
		{
			Name:        toolSwitchOrg,
			Description: "Switch this connection to another organisation the user belongs to (OAuth connections only; API keys are fixed to one organisation). Later calls use the user's role in that organisation.",
			InputSchema: objectSchema(map[string]any{
				"organization_id": map[string]any{"type": "string", "format": "uuid", "description": "Target organisation ID from whatomate_list_organizations"},
			}, "organization_id"),
			Annotations: map[string]any{"title": "Switch organisation", "readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			Name:        toolListAreas,
			Description: "List the functional areas of Whatomate (contacts, chats, templates, campaigns, RSVP, chatbot, accounts, settings...) with a description and endpoint count. Start here to find an endpoint, then call whatomate_describe_area.",
			InputSchema: objectSchema(map[string]any{}),
			Annotations: readOnly("List areas"),
		},
		{
			Name:        toolDescribeArea,
			Description: "Every API endpoint in one area: method, path template, summary, required permission, query parameters, JSON body schema and side effects. Use the result to build a whatomate_call or whatomate_upload.",
			InputSchema: objectSchema(map[string]any{
				"area": map[string]any{"type": "string", "description": "Area key from whatomate_list_areas"},
			}, "area"),
			Annotations: readOnly("Describe area"),
		},
		{
			Name:        toolCall,
			Description: "Call any catalogued Whatomate JSON API endpoint as the connected user. The path must be a concrete /api path matching an endpoint from whatomate_describe_area (fill {id} placeholders). Permissions, organisation scoping and validation are enforced by the server. POST/PUT/PATCH/DELETE change real data and some send WhatsApp messages - check the endpoint's side effects first.",
			InputSchema: objectSchema(map[string]any{
				"method": methodEnum,
				"path":   map[string]any{"type": "string", "description": "Concrete API path, e.g. /api/contacts or /api/contacts/0b6c... (no query string)"},
				"query":  map[string]any{"type": "object", "description": "Query parameters, e.g. {\"page\": 1, \"limit\": 20, \"search\": \"ravi\"}", "additionalProperties": true},
				"body":   map[string]any{"description": "JSON request body for POST/PUT/PATCH (object or array), as described by whatomate_describe_area"},
			}, "method", "path"),
			Annotations: destructive("Call Whatomate API"),
		},
		{
			Name:        toolUpload,
			Description: "Upload a file to a catalogued multipart endpoint (media, CSV imports, template samples, RSVP guest import, flow/IVR media, profile photo). The file is sent base64-encoded; max 16 MB decoded.",
			InputSchema: objectSchema(map[string]any{
				"method":       map[string]any{"type": "string", "enum": []string{"POST", "PUT", "PATCH"}, "description": "Defaults to POST"},
				"path":         map[string]any{"type": "string", "description": "Concrete multipart endpoint path from whatomate_describe_area"},
				"file_base64":  map[string]any{"type": "string", "description": "File content, base64-encoded"},
				"filename":     map[string]any{"type": "string", "description": "File name including extension, e.g. guests.csv"},
				"content_type": map[string]any{"type": "string", "description": "MIME type, e.g. text/csv or image/jpeg (guessed from the file name when omitted)"},
				"file_field":   map[string]any{"type": "string", "description": "Form field for the file (defaults to the endpoint's file field)"},
				"fields":       map[string]any{"type": "object", "description": "Other form fields as strings", "additionalProperties": map[string]any{"type": "string"}},
			}, "path", "file_base64", "filename"),
			Annotations: destructive("Upload file to Whatomate"),
		},
	}
}

// allTools returns the static tools followed by the featured catalog tools.
func allTools() []toolDef {
	tools := staticTools()
	seen := map[string]bool{}
	for _, t := range tools {
		seen[t.Name] = true
	}
	for _, ft := range featuredTools() {
		if seen[ft.def.Name] {
			continue
		}
		seen[ft.def.Name] = true
		tools = append(tools, ft.def)
	}
	return tools
}
