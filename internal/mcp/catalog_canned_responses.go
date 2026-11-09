package mcp

func init() {
	registerArea(Area{
		Key:         "canned_responses",
		Title:       "Canned responses",
		Description: "Saved quick replies agents insert into chats, optionally with buttons.",
	})

	cannedBody := mxObj([]string{"name", "content"}, map[string]any{
		"name":      mxStr("Name of the canned response"),
		"shortcut":  mxStr("Shortcut agents type to insert it, e.g. /thanks"),
		"content":   mxStr("Message text"),
		"category":  mxStr("Category used to group responses"),
		"is_active": mxBool("Whether agents can use it"),
		"buttons": mxArr("Optional buttons sent with the message", mxObj([]string{"title"}, map[string]any{
			"id":           mxStr("Button ID"),
			"title":        mxStr("Button label (CTA label for flow, button text for voice_call)"),
			"type":         mxEnum("Button kind (default reply)", "reply", "url", "phone", "voice_call", "flow"),
			"url":          mxStr("Link for url buttons"),
			"phone_number": mxStr("Number for phone buttons"),
			"ttl_minutes":  mxInt("voice_call only: minutes the button stays clickable"),
			"flow_id":      mxStr("flow only: Meta flow ID"),
			"screen":       mxStr("flow only: first screen"),
		})),
	})

	register(
		Entry{
			Area:    "canned_responses",
			Method:  "GET",
			Path:    "/api/canned-responses",
			Summary: "List canned responses.",
			Query: mxWithPaging(
				mxQ("category", "string", "Only this category"),
				mxQ("search", "string", "Text to match against name, shortcut or content"),
				mxQ("active_only", "boolean", "true to list only active responses"),
			),
		},
		Entry{
			Area:    "canned_responses",
			Method:  "POST",
			Path:    "/api/canned-responses",
			Summary: "Create a canned response.",
			Body:    cannedBody,
		},
		Entry{
			Area:    "canned_responses",
			Method:  "GET",
			Path:    "/api/canned-responses/{id}",
			Summary: "Get one canned response.",
		},
		Entry{
			Area:   "canned_responses",
			Method: "PUT",
			Path:   "/api/canned-responses/{id}",
			Summary: "Update a canned response. Send every field: shortcut, category, is_active and buttons " +
				"are overwritten even when left out (is_active then becomes false).",
			Body: mxObj(nil, cannedBody["properties"].(map[string]any)),
		},
		Entry{
			Area:        "canned_responses",
			Method:      "DELETE",
			Path:        "/api/canned-responses/{id}",
			Summary:     "Delete a canned response.",
			SideEffects: "deletes data",
		},
		Entry{
			Area:    "canned_responses",
			Method:  "POST",
			Path:    "/api/canned-responses/{id}/use",
			Summary: "Count one use of a canned response (for its usage statistics). Sends nothing.",
		},
	)
}
