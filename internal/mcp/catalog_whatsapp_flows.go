package mcp

// Area "whatsapp_flows": WhatsApp Flows (Meta forms) under /api/flows.

func whatsappFlowBody(required ...string) map[string]any {
	return setupObj(map[string]any{
		"whatsapp_account": setupStr("WhatsApp account name the form belongs to."),
		"name":             setupStr("Form name."),
		"category": setupEnum("Meta flow category.", "SIGN_UP", "SIGN_IN", "APPOINTMENT_BOOKING", "LEAD_GENERATION",
			"CONTACT_US", "CUSTOMER_SUPPORT", "SURVEY", "OTHER"),
		"json_version": setupStr("WhatsApp Flow JSON version, e.g. 6.0."),
		"flow_json":    setupFreeObj("Full WhatsApp Flow JSON (version, screens, routing) as Meta expects it."),
		"screens":      setupArr(setupFreeObj("Screen definition."), "Editor screens; kept in sync with flow_json by the builder."),
	}, required...)
}

func init() {
	registerArea(Area{
		Key:   "whatsapp_flows",
		Title: "WhatsApp Flows (forms)",
		Description: "Meta WhatsApp Flows: interactive in-chat forms. Build locally, save to Meta as a draft, then publish. " +
			"Published forms can be opened from chatbot whatsapp_flow steps and template FLOW buttons.",
	})
	base := "/api/flows"
	register(
		Entry{Area: "whatsapp_flows", Method: "GET", Path: base,
			Summary: "List WhatsApp Flows (forms) with status (DRAFT, PUBLISHED, DEPRECATED) and whether there are unsaved local changes.",
			Query: append(setupPage(50),
				Param{Name: "account", Type: "string", Description: "Only flows of this WhatsApp account name."},
				Param{Name: "status", Type: "string", Description: "Filter by status, e.g. DRAFT or PUBLISHED."},
				setupSearch("flow name"))},
		Entry{Area: "whatsapp_flows", Method: "POST", Path: base,
			Summary: "Create a WhatsApp Flow locally (not yet on Meta). Next: save-to-meta, then publish.",
			Body:    whatsappFlowBody("whatsapp_account", "name")},
		Entry{Area: "whatsapp_flows", Method: "GET", Path: base + "/{id}", Summary: "Get one WhatsApp Flow with its JSON and screens."},
		Entry{Area: "whatsapp_flows", Method: "PUT", Path: base + "/{id}",
			Summary: "Update a WhatsApp Flow in Whatomate. Changes reach Meta only after save-to-meta.", Body: whatsappFlowBody()},
		Entry{Area: "whatsapp_flows", Method: "DELETE", Path: base + "/{id}",
			Summary: "Delete a WhatsApp Flow from Whatomate (does not delete it at Meta).", SideEffects: "Deletes the local flow."},
		Entry{Area: "whatsapp_flows", Method: "POST", Path: base + "/{id}/save-to-meta",
			Summary:     "Create or update the flow at Meta as a DRAFT and upload its JSON. Deprecated flows cannot be saved.",
			SideEffects: "Creates/updates the flow at Meta."},
		Entry{Area: "whatsapp_flows", Method: "POST", Path: base + "/{id}/publish",
			Summary:     "Publish the flow at Meta so customers can use it. Save to Meta first. Published flows can no longer be edited, only deprecated.",
			SideEffects: "Publishes at Meta; this cannot be undone."},
		Entry{Area: "whatsapp_flows", Method: "POST", Path: base + "/{id}/deprecate",
			Summary:     "Deprecate a published flow at Meta so it can no longer be sent.",
			SideEffects: "Deprecates at Meta; this cannot be undone."},
		Entry{Area: "whatsapp_flows", Method: "POST", Path: base + "/{id}/duplicate",
			Summary: "Copy a flow into a new local DRAFT named \"<name> (Copy)\" (use it to edit a published flow)."},
		Entry{Area: "whatsapp_flows", Method: "POST", Path: base + "/sync",
			Summary:     "Import/refresh the flows that exist at Meta for one WhatsApp account.",
			Body:        setupObj(map[string]any{"whatsapp_account": setupStr("WhatsApp account name to sync.")}, "whatsapp_account"),
			SideEffects: "Reads flows from Meta and updates local copies."},
	)
}
