package mcp

// Area "custom_actions": buttons agents run from a chat. /api/custom-actions/redirect/{token}
// is excluded from the catalog: it is a one-time browser redirect, not an API call.

func customActionBody(required ...string) map[string]any {
	return setupObj(map[string]any{
		"name":        setupStr("Button label shown to agents."),
		"icon":        setupStr("Icon name."),
		"action_type": setupEnum("webhook calls a URL server-side; url opens a link; javascript runs code in the agent's browser.", "webhook", "url", "javascript"),
		"config": setupObj(map[string]any{
			"url":             setupStr("webhook/url: target URL; {{contact.*}} variables allowed."),
			"method":          setupEnum("webhook: HTTP method.", "GET", "POST", "PUT", "PATCH", "DELETE"),
			"headers":         setupFreeObj("webhook: HTTP headers."),
			"body":            setupStr("webhook: body template (default sends contact and user details)."),
			"open_in_new_tab": setupBool("url: open in a new tab."),
			"code":            setupStr("javascript: code to run."),
		}),
		"is_active":     setupBool("Show the action. Always written on update."),
		"display_order": setupInt("Sort order. Always written on update."),
	}, required...)
}

func init() {
	registerArea(Area{
		Key:         "custom_actions",
		Title:       "Custom actions",
		Description: "Buttons agents can run from a chat: call a webhook, open a URL or run JavaScript with the contact's details.",
	})
	register(
		Entry{Area: "custom_actions", Method: "GET", Path: "/api/custom-actions", Summary: "List custom actions.",
			Query: append(setupPage(50), setupSearch("action name"))},
		Entry{Area: "custom_actions", Method: "POST", Path: "/api/custom-actions", Summary: "Create a custom action.",
			Body: customActionBody("name", "action_type", "config")},
		Entry{Area: "custom_actions", Method: "GET", Path: "/api/custom-actions/{id}", Summary: "Get one custom action."},
		Entry{Area: "custom_actions", Method: "PUT", Path: "/api/custom-actions/{id}", Summary: "Update a custom action; empty text fields and a missing config keep their value.", Body: customActionBody()},
		Entry{Area: "custom_actions", Method: "DELETE", Path: "/api/custom-actions/{id}", Summary: "Delete a custom action.", SideEffects: "Deletes the action."},
		Entry{Area: "custom_actions", Method: "POST", Path: "/api/custom-actions/{id}/execute",
			Summary:     "Run an active custom action for a contact. Returns the result (message, toast, data, or a one-time redirect_url for url actions).",
			Body:        setupObj(map[string]any{"contact_id": setupStr("Contact the action runs for.")}, "contact_id"),
			SideEffects: "webhook actions send the contact's details to an external URL."},
	)
}
