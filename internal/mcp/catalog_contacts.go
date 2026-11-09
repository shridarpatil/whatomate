package mcp

func init() {
	registerArea(Area{
		Key:   "contacts",
		Title: "Contacts",
		Description: "WhatsApp contacts: search, create, rename, tag, assign to an agent, " +
			"deactivate or delete. Phone numbers are stored in one canonical form (country code, digits only).",
	})

	register(
		Entry{
			Area:   "contacts",
			Method: "GET",
			Path:   "/api/contacts",
			Summary: "Search and list contacts, newest conversation first. Users without contacts:read " +
				"only see contacts assigned to them.",
			Query: mxWithPaging(
				mxQ("search", "string", "Text to match against contact name or phone number"),
				mxQ("tags", "string", "Comma-separated tag names; contact must have any of them"),
				mxQ("whatsapp_account", "string", "Only contacts on this WhatsApp account (account name)"),
				mxQ("unread_only", "boolean", "Only contacts with unread incoming messages"),
				mxQ("is_active", "boolean", "Filter by active (true) or inactive (false) contacts"),
			),
			Tool: "whatomate_search_contacts",
		},
		Entry{
			Area:       "contacts",
			Method:     "POST",
			Path:       "/api/contacts",
			Summary:    "Create a contact. The phone number is saved in canonical form; no message is sent.",
			Permission: "contacts:write",
			Body: mxObj([]string{"phone_number"}, map[string]any{
				"phone_number":     mxStr("Phone number with country code (e.g. 919876543210); 10-digit numbers get 91"),
				"profile_name":     mxStr("Display name"),
				"whatsapp_account": mxStr("WhatsApp account name to talk to this contact from"),
				"tags":             mxStrArr("Tag names"),
				"metadata":         mxFreeObj("Free-form extra fields"),
			}),
		},
		Entry{
			Area:    "contacts",
			Method:  "GET",
			Path:    "/api/contacts/{id}",
			Summary: "Get one contact by ID (assigned contacts only for users without contacts:read).",
		},
		Entry{
			Area:   "contacts",
			Method: "PUT",
			Path:   "/api/contacts/{id}",
			Summary: "Update a contact. Setting profile_name locks the name so WhatsApp and campaigns " +
				"stop overwriting it; an empty name unlocks it.",
			Permission: "contacts:write",
			Body: mxObj(nil, map[string]any{
				"profile_name":         mxStr("New display name; empty string unlocks automatic naming"),
				"whatsapp_account":     mxStr("WhatsApp account name"),
				"tags":                 mxStrArr("Replace tags with this list"),
				"metadata":             mxFreeObj("Free-form extra fields"),
				"assigned_user_id":     mxStr("User UUID to assign the contact to"),
				"is_active":            mxBool("Active (true) or inactive (false)"),
				"clear_assigned_agent": mxBool("true removes the assigned agent"),
			}),
		},
		Entry{
			Area:        "contacts",
			Method:      "DELETE",
			Path:        "/api/contacts/{id}",
			Summary:     "Delete a contact, optionally with its whole chat history.",
			Permission:  "contacts:delete",
			Query:       []Param{mxQ("include_messages", "boolean", "Also delete all messages of this contact")},
			SideEffects: "deletes data",
		},
		Entry{
			Area:       "contacts",
			Method:     "PUT",
			Path:       "/api/contacts/{id}/assign",
			Summary:    "Assign the contact to an agent, or unassign with user_id null.",
			Permission: "contacts:write",
			Body: mxObj(nil, map[string]any{
				"user_id": map[string]any{"type": []string{"string", "null"}, "description": "User UUID; null to unassign"},
			}),
		},
		Entry{
			Area:       "contacts",
			Method:     "PUT",
			Path:       "/api/contacts/{id}/tags",
			Summary:    "Replace the contact's tags.",
			Permission: "contacts:write",
			Body: mxObj([]string{"tags"}, map[string]any{
				"tags": mxStrArr("Full list of tag names the contact should have"),
			}),
		},
		Entry{
			Area:   "contacts",
			Method: "GET",
			Path:   "/api/contacts/{id}/session-data",
			Summary: "Chatbot answers collected from this contact plus the contact-info panel layout " +
				"shown beside the chat.",
		},
	)
}
