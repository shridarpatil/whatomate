package mcp

func init() {
	registerArea(Area{
		Key:         "ai_connector",
		Title:       "AI Connector",
		Description: "This Claude connector itself: its URL and the AI apps you have connected. Approving a new connection is always done by a person on the Whatomate consent page.",
	})
	register(
		Entry{
			Area:    "ai_connector",
			Method:  "GET",
			Path:    "/api/mcp/info",
			Summary: "Show the connector URL people paste into Claude, and whether the connector is switched on.",
		},
		Entry{
			Area:    "ai_connector",
			Method:  "GET",
			Path:    "/api/mcp/grants",
			Summary: "List the AI apps (such as Claude) you have connected to Whatomate, with organisation, connected date and last use.",
		},
		Entry{
			Area:        "ai_connector",
			Method:      "DELETE",
			Path:        "/api/mcp/grants/{id}",
			Summary:     "Disconnect one of your connected AI apps. Only your own connections can be removed.",
			SideEffects: "Deletes the connection; that app loses access immediately. Revoking the connection Claude is using now ends this session.",
		},
	)
}
