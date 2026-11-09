package mcp

// Area "chatbot": everything under /api/chatbot except the flow builder
// (catalog_chatbot_flows.go).

func chatbotButtonSchema() map[string]any {
	return setupObj(map[string]any{
		"id":           setupStr("Reply ID returned when tapped (auto btn_N when empty)."),
		"title":        setupStr("Button label (max 20 characters)."),
		"type":         setupEnum("reply (default), url or phone.", "reply", "url", "phone"),
		"url":          setupStr("For type=url: the link."),
		"phone_number": setupStr("For type=phone: the number to call."),
	}, "title")
}

func chatbotSettingsBody() map[string]any {
	hours := setupObj(map[string]any{
		"day":        setupInt("0=Sunday … 6=Saturday."),
		"enabled":    setupBool("Open on this day."),
		"start_time": setupStr("Opening time HH:MM (24h)."),
		"end_time":   setupStr("Closing time HH:MM (24h)."),
	})
	return setupObj(map[string]any{
		"enabled":                         setupBool("Turn the chatbot on/off for the organisation."),
		"greeting_message":                setupStr("Default greeting sent when no flow or keyword matches a new conversation."),
		"greeting_buttons":                setupArr(chatbotButtonSchema(), "Up to 3 reply buttons (or up to 2 url/phone buttons) under the greeting."),
		"fallback_message":                setupStr("Reply when the bot does not understand."),
		"fallback_buttons":                setupArr(chatbotButtonSchema(), "Buttons under the fallback message."),
		"session_timeout_minutes":         setupInt("Minutes of silence after which a bot session ends."),
		"business_hours_enabled":          setupBool("Apply business hours."),
		"business_hours":                  setupArr(hours, "One entry per weekday."),
		"out_of_hours_message":            setupStr("Message sent outside business hours."),
		"allow_automated_outside_hours":   setupBool("Let flows/keywords keep running outside hours."),
		"skip_transfer_outside_hours":     setupBool("Opt-in: outside hours, flow/queue transfers only send the out-of-hours message (default off keeps classic behaviour)."),
		"mark_bot_handled_as_read":        setupBool("Opt-in: mark messages the bot handled as read."),
		"allow_agent_queue_pickup":        setupBool("Agents may pick chats from the transfer queue."),
		"assign_to_same_agent":            setupBool("Route returning customers to their previous agent."),
		"agent_current_conversation_only": setupBool("Agents only see the current conversation's history."),
		"ai_enabled":                      setupBool("Enable AI replies."),
		"ai_provider":                     setupEnum("AI provider.", "openai", "anthropic", "google"),
		"ai_api_key":                      setupStr("Provider API key (empty keeps the stored key)."),
		"ai_model":                        setupStr("Model name for the provider."),
		"ai_max_tokens":                   setupInt("Max tokens per AI reply."),
		"ai_system_prompt":                setupStr("System prompt for AI replies."),
		"sla_enabled":                     setupBool("Enable SLA tracking for transferred chats."),
		"sla_response_minutes":            setupInt("Target first-response minutes."),
		"sla_resolution_minutes":          setupInt("Target resolution minutes."),
		"sla_escalation_minutes":          setupInt("Escalate after this many minutes."),
		"sla_auto_close_hours":            setupInt("Auto-close idle chats after hours."),
		"sla_auto_close_message":          setupStr("Message sent on auto-close."),
		"sla_warning_message":             setupStr("Message sent on SLA warning."),
		"sla_escalation_notify_ids":       setupStrArr("User IDs notified on escalation."),
		"client_reminder_enabled":         setupBool("Remind customers who stop replying to the bot."),
		"client_reminder_minutes":         setupInt("Minutes before the reminder."),
		"client_reminder_message":         setupStr("Reminder text."),
		"client_auto_close_minutes":       setupInt("Minutes before closing an idle bot chat."),
		"client_auto_close_message":       setupStr("Text sent when closing it."),
	})
}

func keywordRuleBody(required ...string) map[string]any {
	return setupObj(map[string]any{
		"name":          setupStr("Rule name (defaults to the first keyword)."),
		"keywords":      setupStrArr("Words or phrases that trigger the rule."),
		"match_type":    setupEnum("How keywords match (default contains).", "exact", "contains", "starts_with", "regex"),
		"response_type": setupEnum("text replies with response_content.body; transfer hands the chat to a human agent.", "text", "transfer", "template", "media", "flow", "script"),
		"response_content": setupObj(map[string]any{
			"body":    setupStr("Reply text (for transfer: optional message before transferring)."),
			"buttons": setupArr(chatbotButtonSchema(), "Optional reply buttons (text responses)."),
		}),
		"priority": setupInt("Higher priority rules are checked first."),
		"enabled":  setupBool("Rule is active."),
	}, required...)
}

func aiContextBody(required ...string) map[string]any {
	return setupObj(map[string]any{
		"name":             setupStr("Context name."),
		"context_type":     setupEnum("static text or live api data (default static).", "static", "api"),
		"trigger_keywords": setupStrArr("Use this context only when a message contains one of these (empty = always)."),
		"static_content":   setupStr("Knowledge text for static contexts."),
		"api_config": setupObj(map[string]any{
			"url":           setupStr("URL to fetch; supports {{phone_number}} and session variables."),
			"method":        setupEnum("HTTP method.", "GET", "POST"),
			"headers":       setupFreeObj("HTTP headers."),
			"body":          setupStr("Request body template."),
			"response_path": setupStr("Dot path into the JSON response to use as context."),
		}),
		"priority": setupInt("Higher priority contexts are added first."),
		"enabled":  setupBool("Context is active."),
	}, required...)
}

func init() {
	registerArea(Area{
		Key:   "chatbot",
		Title: "Chatbot",
		Description: "Chatbot settings (greeting, hours, AI, SLA), keyword auto-replies, step-based chatbot flows, " +
			"AI knowledge contexts, human-agent transfers and bot conversation history.",
	})
	kw, ai, tr := "/api/chatbot/keywords", "/api/chatbot/ai-contexts", "/api/chatbot/transfers"
	register(
		Entry{Area: "chatbot", Method: "GET", Path: "/api/chatbot/settings",
			Summary: "Get the organisation's chatbot settings: on/off, greeting and fallback, business hours, agent assignment, AI and SLA options."},
		Entry{Area: "chatbot", Method: "PUT", Path: "/api/chatbot/settings",
			Summary: "Update chatbot settings. Send only the fields to change; omitted fields keep their value.",
			Body:    chatbotSettingsBody(), SideEffects: "Changes how the bot answers every incoming WhatsApp message."},

		Entry{Area: "chatbot", Method: "GET", Path: kw, Summary: "List keyword auto-reply rules.",
			Query: append(setupPage(50), setupSearch("rule name or keywords"))},
		Entry{Area: "chatbot", Method: "POST", Path: kw, Summary: "Create a keyword auto-reply rule.",
			Body: keywordRuleBody("keywords"), SideEffects: "Customers sending matching words get this reply automatically."},
		Entry{Area: "chatbot", Method: "GET", Path: kw + "/{id}", Summary: "Get one keyword rule."},
		Entry{Area: "chatbot", Method: "PUT", Path: kw + "/{id}", Summary: "Update a keyword rule; omitted fields keep their value.", Body: keywordRuleBody()},
		Entry{Area: "chatbot", Method: "DELETE", Path: kw + "/{id}", Summary: "Delete a keyword rule.", SideEffects: "Deletes the rule."},

		Entry{Area: "chatbot", Method: "GET", Path: ai, Summary: "List AI knowledge contexts the AI assistant uses when answering.",
			Query: append(setupPage(50), setupSearch("context name, content or trigger keywords"))},
		Entry{Area: "chatbot", Method: "POST", Path: ai, Summary: "Create an AI knowledge context (static text or data fetched from an API).", Body: aiContextBody("name")},
		Entry{Area: "chatbot", Method: "GET", Path: ai + "/{id}", Summary: "Get one AI context."},
		Entry{Area: "chatbot", Method: "PUT", Path: ai + "/{id}", Summary: "Update an AI context; omitted fields keep their value.", Body: aiContextBody()},
		Entry{Area: "chatbot", Method: "DELETE", Path: ai + "/{id}", Summary: "Delete an AI context.", SideEffects: "Deletes the context."},

		Entry{Area: "chatbot", Method: "GET", Path: tr,
			Summary: "List human-agent transfers (the agent queue). Users with transfers:write see all; others see their own and their teams' queues.",
			Query: []Param{
				{Name: "status", Type: "string", Description: "Filter by status.", Enum: []string{"active", "resumed", "expired"}},
				{Name: "team_id", Type: "string", Description: "Filter by team ID, or general for the no-team queue."},
				{Name: "limit", Type: "integer", Description: "Max rows (1-100, default 100)."},
				{Name: "offset", Type: "integer", Description: "Rows to skip."},
				{Name: "include", Type: "string", Description: "Relations to load: all (default) or a comma list of contact,agent,team,transferred_by,resumed_by."},
			}},
		Entry{Area: "chatbot", Method: "POST", Path: tr,
			Summary: "Hand a contact's chat from the bot to a human agent or team queue (pauses the bot for that contact).",
			Body: setupObj(map[string]any{
				"contact_id":       setupStr("Contact ID."),
				"whatsapp_account": setupStr("WhatsApp account name the chat is on."),
				"agent_id":         setupStr("Optional agent user ID to assign directly."),
				"team_id":          setupStr("Optional team queue ID."),
				"notes":            setupStr("Notes for the agent."),
				"source":           setupEnum("Why the transfer happened (default manual).", "manual", "flow", "keyword"),
			}, "contact_id"),
			SideEffects: "Stops bot replies to the contact and notifies agents."},
		Entry{Area: "chatbot", Method: "POST", Path: tr + "/pick",
			Summary:     "Pick the next waiting transfer from the queue and assign it to yourself. Needs transfers:write or transfers:pickup.",
			Query:       []Param{{Name: "team_id", Type: "string", Description: "Only pick from this team's queue."}},
			SideEffects: "Assigns a waiting chat to the calling user."},
		Entry{Area: "chatbot", Method: "PUT", Path: tr + "/{id}/resume",
			Summary: "Close a transfer and hand the chat back to the bot.", SideEffects: "Bot resumes replying to the contact."},
		Entry{Area: "chatbot", Method: "PUT", Path: tr + "/{id}/assign",
			Summary: "Assign an active transfer to an agent (needs transfers:write unless assigning to yourself), unassign it (agent_id empty/null), or move it to another team.",
			Body: setupObj(map[string]any{
				"agent_id": setupStr("Agent user ID; null or empty string unassigns."),
				"team_id":  setupStr("Optional: move to this team queue."),
			})},

		Entry{Area: "chatbot", Method: "GET", Path: "/api/chatbot/sessions",
			Summary:    "List chatbot conversation sessions (who talked to which flow, status, message count). Engine keys starting with _ are hidden from answers.",
			Permission: "flows.chatbot:read",
			Query: append(setupPage(100),
				Param{Name: "status", Type: "string", Description: "Filter by status.", Enum: []string{"active", "completed", "cancelled", "timeout"}},
				Param{Name: "flow_id", Type: "string", Description: "Only sessions of this chatbot flow."},
				setupSearch("contact name or phone"),
				Param{Name: "from", Type: "string", Description: "Started on/after this date, YYYY-MM-DD."},
				Param{Name: "to", Type: "string", Description: "Started on/before this date, YYYY-MM-DD."})},
		Entry{Area: "chatbot", Method: "GET", Path: "/api/chatbot/sessions/{id}",
			Summary:    "Get one chatbot session with its collected answers and message log.",
			Permission: "flows.chatbot:read"},
	)
}
