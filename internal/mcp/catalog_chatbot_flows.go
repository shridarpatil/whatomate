package mcp

// Area "chatbot" (continued): step-engine chatbot flows. The v2 graph engine
// is dormant; every flow (and RSVP) runs on these ordered steps.

const flowStepGuide = "A flow is an ordered list of steps run by the step engine (the graph engine is dormant). " +
	"Step names must be unique. The bot sends each step's message, waits for the customer's reply when input_type is not none, " +
	"stores it in session data under store_as (buttons also store <store_as>_title), then moves on: conditional_next[reply option ID " +
	"or exact reply text] → step name, else conditional_next.default, else next_step, else the next step in order. " +
	"Targets __complete__ / __end__ finish the flow. Use {{variable}} in messages to insert stored answers " +
	"({{phone_number}} is the sender when the flow did not collect one)."

func flowStepSchema() map[string]any {
	option := setupObj(map[string]any{
		"id":           setupStr("Stable option ID; routing in conditional_next uses this ID."),
		"title":        setupStr("Label (buttons max 20 chars, list rows max 24)."),
		"description":  setupStr("List mode only: row description."),
		"type":         setupEnum("reply (default) or a call-to-action url/phone button (not mixable with reply buttons).", "reply", "url", "phone"),
		"url":          setupStr("For type=url."),
		"phone_number": setupStr("For type=phone."),
	}, "title")
	return setupObj(map[string]any{
		"step_name":  setupStr("Unique step key used for routing, e.g. ask_name."),
		"step_order": setupInt("Ignored on save: order follows the array position."),
		"message":    setupStr("Text the bot sends (required except for transfer steps). Supports {{variables}}."),
		"message_type": setupEnum("What the step sends: text; buttons (reply buttons or list picker from buttons[]); api_fetch "+
			"(call api_config then send message with mapped data); transfer (hand to a human agent); whatsapp_flow (open a WhatsApp Form); "+
			"template/script are legacy. Default text.",
			"text", "buttons", "api_fetch", "transfer", "whatsapp_flow", "template", "script"),
		"input_type": setupEnum("What reply to expect: none sends and moves on without waiting; button for buttons steps; "+
			"whatsapp_flow for form steps; others validate typed text.",
			"none", "text", "number", "email", "phone", "date", "select", "button", "whatsapp_flow"),
		"input_config": setupObj(map[string]any{
			"reply_mode":         setupEnum("buttons step: buttons = up to 3 reply buttons; list = list picker with up to 10 rows. When omitted, more than 3 options becomes a list.", "buttons", "list"),
			"list_button_text":   setupStr("List mode: text on the button that opens the list (default Select an option)."),
			"list_section_title": setupStr("List mode: section heading (default Options)."),
			"media_type":         setupEnum("Optional header media sent with the step.", "image", "video", "document"),
			"media_url":          setupStr("Public URL of the header media."),
			"whatsapp_flow_id":   setupStr("whatsapp_flow step: Meta flow ID of the WhatsApp Form to open."),
			"flow_header":        setupStr("whatsapp_flow step: header text."),
			"flow_cta":           setupStr("whatsapp_flow step: button text (default Open Form)."),
		}),
		"buttons": setupArr(option, "Options for buttons steps (max 3 in buttons mode, 10 in list mode)."),
		"api_config": map[string]any{
			"type": "object",
			"description": "api_fetch steps. The step message is rendered with the mapped data; with no message the response's " +
				"\"message\" field (or raw body) is sent, and a \"buttons\" array in the response becomes reply buttons.",
			"properties": map[string]any{
				"url":              setupStr("URL; {{variables}} allowed."),
				"method":           setupEnum("HTTP method.", "GET", "POST", "PUT", "PATCH", "DELETE"),
				"headers":          setupFreeObj("HTTP headers."),
				"body":             setupStr("Request body template (JSON text with {{variables}})."),
				"response_mapping": setupFreeObj("Map session variable name → dot path in the JSON response, e.g. {\"plan\":\"data.plan\"}."),
				"fallback_message": setupStr("Sent when the API call fails."),
			},
		},
		"transfer_config": setupObj(map[string]any{
			"team_id": setupStr("transfer step: team queue ID (empty or _general = general queue)."),
			"notes":   setupStr("Notes shown to the agent."),
		}),
		"validation_regex": setupStr("Optional regex the typed reply must match."),
		"validation_error": setupStr("Message sent when validation fails."),
		"store_as":         setupStr("Session variable name for the reply (shown in Collected Answers)."),
		"next_step":        setupStr("Default next step name, or __complete__ / __end__."),
		"conditional_next": setupFreeObj("Routing map: reply option ID (or exact reply text) → step name; key default for anything else."),
		"skip_condition":   setupStr("Skip this step when true, e.g. plan == 'gold' AND age >= 18 (operators == != > < >= <=, AND/OR, parentheses)."),
		"retry_on_invalid": setupBool("Ask again after an invalid reply."),
		"max_retries":      setupInt("0-10 retries (0 is saved as 3)."),
	}, "step_name")
}

func chatbotFlowBody(required ...string) map[string]any {
	return setupObj(map[string]any{
		"name":                     setupStr("Flow name."),
		"whatsapp_account":         setupStr("Restrict the flow to this WhatsApp account name (recommended); empty = all numbers."),
		"description":              setupStr("Internal description."),
		"trigger_keywords":         setupStrArr("Words that start the flow."),
		"initial_message":          setupStr("First message sent when the flow starts."),
		"initial_media_type":       setupEnum("Optional media header on the first message.", "image", "video", "document"),
		"initial_media_url":        setupStr("Public URL for that media."),
		"initial_buttons":          setupArr(setupObj(map[string]any{"id": setupStr("Option ID."), "title": setupStr("Label.")}, "title"), "Up to 3 reply buttons on the first message."),
		"initial_conditional_next": setupFreeObj("Routing for initial buttons: button ID → step name (or __complete__/__end__), default key allowed."),
		"initial_store_as":         setupStr("Session variable for the initial button reply."),
		"completion_message":       setupStr("Sent when the flow finishes."),
		"on_complete_action":       setupEnum("none, or webhook to POST the collected answers.", "none", "webhook"),
		"completion_config": setupObj(map[string]any{
			"url":     setupStr("Webhook URL."),
			"method":  setupEnum("HTTP method (default POST).", "POST", "PUT"),
			"headers": setupFreeObj("HTTP headers."),
			"body":    setupStr("Optional body template with {{variables}}."),
		}),
		"panel_config":  setupFreeObj("Agent side-panel layout (sections of collected fields)."),
		"canvas_layout": setupFreeObj("Editor node positions; safe to omit."),
		"enabled":       setupBool("Flow is active (default true on create)."),
		"steps":         setupArr(flowStepSchema(), "Ordered steps. On update, sending steps replaces all steps."),
	}, required...)
}

func init() {
	base := "/api/chatbot/flows"
	register(
		Entry{Area: "chatbot", Method: "GET", Path: base, Tool: "whatomate_list_chatbot_flows",
			Summary:    "List chatbot flows with trigger keywords, account, enabled flag and step count.",
			Permission: "flows.chatbot:read",
			Query:      append(setupPage(50), setupSearch("name, description or trigger keywords"))},
		Entry{Area: "chatbot", Method: "POST", Path: base,
			Summary: "Create a chatbot flow. " + flowStepGuide, Permission: "flows.chatbot:write",
			Body:        chatbotFlowBody("name"),
			SideEffects: "Enabled flows start replying to customers who send a trigger keyword."},
		Entry{Area: "chatbot", Method: "GET", Path: base + "/{id}",
			Summary: "Get a chatbot flow with all its steps (use it before editing).", Permission: "flows.chatbot:read"},
		Entry{Area: "chatbot", Method: "PUT", Path: base + "/{id}",
			Summary:    "Update a chatbot flow. Omitted fields keep their value; when steps is sent it replaces every step. " + flowStepGuide,
			Permission: "flows.chatbot:write", Body: chatbotFlowBody(),
			SideEffects: "Changes live bot behaviour for customers."},
		Entry{Area: "chatbot", Method: "DELETE", Path: base + "/{id}",
			Summary: "Delete a chatbot flow and its steps.", Permission: "flows.chatbot:delete",
			SideEffects: "Deletes the flow; customers can no longer trigger it."},
	)
}
