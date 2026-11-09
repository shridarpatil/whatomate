package mcp

func init() {
	registerArea(Area{
		Key:         "api_keys",
		Title:       "API keys",
		Description: "Keys that let other systems call the Whatomate API on behalf of an organisation.",
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
		Entry{
			Area:       "api_keys",
			Method:     "POST",
			Path:       "/api/api-keys",
			Summary:    "Create an API key. The full key is returned only once in the response; store it safely.",
			Permission: "api_keys:write",
			Body: admObj(map[string]any{
				"name":               admStr("Label for the key, e.g. the system that will use it."),
				"expires_at":         admStr("Optional expiry as an ISO date-time (RFC3339, e.g. 2027-01-31T00:00:00Z). Omit for no expiry."),
				"organization_id":    admStr("Super admins only: the organisation the key acts for. Regular users always get a key for their current organisation."),
				"is_super_admin_key": admBool("Super admins only: a key with super-admin reach across organisations."),
			}, "name"),
			SideEffects: "Creates a new credential that can read and change data in the organisation.",
		},
		Entry{
			Area:       "api_keys",
			Method:     "PUT",
			Path:       "/api/api-keys/{id}",
			Summary:    "Turn an API key on or off.",
			Permission: "api_keys:write",
			Body: admObj(map[string]any{
				"is_active": admBool("false disables the key immediately; true re-enables it."),
			}),
			SideEffects: "Disabling a key breaks any integration using it until re-enabled.",
		},
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
