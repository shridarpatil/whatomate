package mcp

func init() {
	registerArea(Area{
		Key:         "api_keys",
		Title:       "API keys",
		Description: "Keys that let other systems call the Whatomate API on behalf of an organisation. AI apps can list, view and delete keys; creating or re-enabling a key is done in Whatomate.",
	})
	register(
		Entry{
			Area:       "api_keys",
			Method:     "GET",
			Path:       "/api/api-keys",
			Summary:    "List API keys of the current organisation (super admins see every key). The secret itself is never shown again.",
			Permission: "api_keys:read",
			Query: append(admPaging(),
				Param{Name: "search", Type: "string", Description: "Match key name."},
			),
		},
		Entry{
			Area:       "api_keys",
			Method:     "GET",
			Path:       "/api/api-keys/{id}",
			Summary:    "Show one API key (name, prefix, status, expiry, last use).",
			Permission: "api_keys:read",
		},
		// Creating and re-enabling keys is deliberately excluded (see
		// ExcludedRoutes): a key would outlive the AI connection.
		Entry{
			Area:        "api_keys",
			Method:      "DELETE",
			Path:        "/api/api-keys/{id}",
			Summary:     "Delete an API key permanently.",
			Permission:  "api_keys:delete",
			SideEffects: "Revokes the key; any integration using it stops working.",
		},
	)
}
