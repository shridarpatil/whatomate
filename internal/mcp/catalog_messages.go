package mcp

func init() {
	registerArea(Area{
		Key:   "messages",
		Title: "Chats and messages",
		Description: "The chat inbox: conversation list, message history, sending text, template and media " +
			"messages, reactions, internal notes, message search, delivery status and media files.",
	})

	register(
		Entry{
			Area:   "messages",
			Method: "GET",
			Path:   "/api/contacts",
			Summary: "Conversation list as the chat inbox shows it: contacts ordered by latest message, " +
				"with unread counts. Same endpoint as the contact search.",
			Query: mxWithPaging(
				mxQ("unread_only", "boolean", "Only conversations with unread incoming messages"),
				mxQ("search", "string", "Text to match against contact name or phone number"),
				mxQ("tags", "string", "Comma-separated tag names"),
				mxQ("whatsapp_account", "string", "Only conversations on this WhatsApp account (account name)"),
				mxQ("is_active", "boolean", "Filter by active (true) or inactive (false) contacts"),
			),
			Tool: "whatomate_list_conversations",
		},
		Entry{
			Area:   "messages",
			Method: "GET",
			Path:   "/api/contacts/{id}/messages",
			Summary: "Messages of one chat. Default returns the latest page in time order; use before_id " +
				"to load older messages or focus_id to load the messages around one message.",
			Query: []Param{
				mxQ("limit", "integer", "Messages per page (default 50, max 100)"),
				mxQ("page", "integer", "Page number counted back from the newest messages"),
				mxQ("before_id", "string", "Message UUID; return messages older than it"),
				mxQ("focus_id", "string", "Message UUID; return messages surrounding it (e.g. from search)"),
				mxQ("account", "string", "Only messages on this WhatsApp account (account name)"),
			},
			Tool: "whatomate_get_messages",
		},
		Entry{
			Area:   "messages",
			Method: "POST",
			Path:   "/api/contacts/{id}/messages",
			Summary: "Send a text (or interactive buttons/list/CTA/flow) message to a contact. WhatsApp only " +
				"allows free-form messages within 24 hours of the contact's last message; otherwise send a template.",
			Body: mxObj([]string{"type"}, map[string]any{
				"type": mxEnum("Message type; use text for a plain message", "text", "interactive"),
				"content": mxObj(nil, map[string]any{
					"body": mxStr("Message text"),
				}),
				"reply_to_message_id": mxStr("Message UUID to quote-reply to"),
				"whatsapp_account":    mxStr("Send from this WhatsApp account instead of the contact's default"),
				"interactive": mxObj(nil, map[string]any{
					"type":         mxEnum("Interactive kind", "button", "list", "cta_url", "voice_call", "flow"),
					"body":         mxStr("Body text"),
					"buttons":      mxArr("Buttons (button/list)", mxObj(nil, map[string]any{"id": mxStr("Button ID"), "title": mxStr("Label")})),
					"button_text":  mxStr("CTA label for cta_url and flow"),
					"url":          mxStr("Link for cta_url"),
					"display_text": mxStr("voice_call button label"),
					"ttl_minutes":  mxInt("voice_call button lifetime in minutes"),
					"flow_id":      mxStr("WhatsApp Flow ID (flow only)"),
					"first_screen": mxStr("First flow screen (flow only)"),
					"header":       mxStr("Optional header (flow only)"),
				}),
			}),
			SideEffects: "sends a WhatsApp message",
			Tool:        "whatomate_send_text",
		},
		Entry{
			Area:        "messages",
			Method:      "DELETE",
			Path:        "/api/contacts/{id}/messages/{message_id}",
			Summary:     "Delete one message from Whatomate's chat history (it is not unsent on the contact's phone).",
			Permission:  "contacts:delete",
			SideEffects: "deletes data",
		},
		Entry{
			Area:    "messages",
			Method:  "POST",
			Path:    "/api/contacts/{id}/mark-read",
			Summary: "Mark all incoming messages of this chat as read, clearing its unread badge.",
		},
		Entry{
			Area:        "messages",
			Method:      "POST",
			Path:        "/api/contacts/{id}/messages/{message_id}/reaction",
			Summary:     "React to a message with an emoji; an empty emoji removes the reaction.",
			Body:        mxObj([]string{"emoji"}, map[string]any{"emoji": mxStr("Emoji character, or empty string to remove")}),
			SideEffects: "sends a WhatsApp reaction",
		},
		Entry{
			Area:   "messages",
			Method: "GET",
			Path:   "/api/messages/search",
			Summary: "Search message text across all chats. Users without contacts:read only search " +
				"their assigned contacts.",
			Query: []Param{
				mxQ("q", "string", "Text to search for (alias: search)"),
				mxQ("search", "string", "Alias of q"),
				mxQ("limit", "integer", "Maximum results"),
				mxQ("account", "string", "Only messages on this WhatsApp account (account name)"),
			},
		},
		Entry{
			Area:   "messages",
			Method: "GET",
			Path:   "/api/media/{message_id}",
			Summary: "Download the media file (image, video, audio, document) attached to a message. " +
				"Returns the file, not JSON.",
		},
		Entry{
			Area:   "messages",
			Method: "POST",
			Path:   "/api/chat/messages/{id}/reprocess",
			Summary: "Run the chatbot again on a received message (continues or starts the matching flow) " +
				"and ends any active agent transfer for that contact.",
			SideEffects: "may send chatbot WhatsApp replies",
		},
	)
}
