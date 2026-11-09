package mcp

// Area "webhooks": outgoing event webhooks.

var webhookEvents = []string{"message.incoming", "message.outgoing", "message.sent", "contact.created",
	"transfer.created", "transfer.assigned", "transfer.resumed", "lead.received", "lead.sent"}

func webhookBody(required ...string) map[string]any {
	return setupObj(map[string]any{
		"name": setupStr("Webhook name."),
		"url":  setupStr("Public http(s) URL (internal/private addresses are rejected)."),
		"events": setupArr(map[string]any{"type": "string", "enum": webhookEvents},
			"Events to deliver (at least one on create)."),
		"headers":   setupFreeObj("Extra HTTP headers (string values)."),
		"secret":    setupStr("Signing secret; generated when empty on create, kept when empty on update."),
		"is_active": setupBool("Deliver events. Always written on update, so send the current value."),
	}, required...)
}

func init() {
	registerArea(Area{
		Key:         "webhooks",
		Title:       "Outgoing webhooks",
		Description: "Send Whatomate events (messages, contacts, transfers, leads) to your own systems by HTTP.",
	})
	register(
		Entry{Area: "webhooks", Method: "GET", Path: "/api/webhooks", Summary: "List outgoing webhooks and the available event names.",
			Query: append(setupPage(50), setupSearch("webhook name or URL"))},
		Entry{Area: "webhooks", Method: "POST", Path: "/api/webhooks", Summary: "Create an outgoing webhook.",
			Body: webhookBody("name", "url", "events"), SideEffects: "Whatomate will POST event data to this URL."},
		Entry{Area: "webhooks", Method: "GET", Path: "/api/webhooks/{id}", Summary: "Get one webhook (the secret is never returned)."},
		Entry{Area: "webhooks", Method: "PUT", Path: "/api/webhooks/{id}", Summary: "Update a webhook; empty text fields keep their value.", Body: webhookBody()},
		Entry{Area: "webhooks", Method: "DELETE", Path: "/api/webhooks/{id}", Summary: "Delete a webhook.", SideEffects: "Deletes the webhook."},
		Entry{Area: "webhooks", Method: "POST", Path: "/api/webhooks/{id}/test", Summary: "Send a test event to the webhook URL and report the response.",
			SideEffects: "Makes an HTTP request to the webhook URL."},
	)
}
