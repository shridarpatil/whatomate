package mcp

func init() {
	registerArea(Area{
		Key:   "templates",
		Title: "Message templates",
		Description: "WhatsApp message templates: list, draft, edit, submit to Meta for approval, " +
			"sync statuses from Meta, upload header samples, delete.",
	})

	templateBody := mxObj([]string{"whatsapp_account", "name", "language", "category", "body_content"}, map[string]any{
		"whatsapp_account": mxStr("WhatsApp account name the template belongs to"),
		"name":             mxStr("Template name (saved lowercase with underscores)"),
		"display_name":     mxStr("Friendly name shown in Whatomate"),
		"language":         mxStr("Language code, e.g. en, en_US, hi"),
		"category":         mxEnum("Meta template category", "MARKETING", "UTILITY", "AUTHENTICATION"),
		"header_type":      mxEnum("Header kind", "NONE", "TEXT", "IMAGE", "VIDEO", "DOCUMENT"),
		"header_content":   mxStr("Header text, or the media handle/URL for a media header sample"),
		"body_content":     mxStr("Body text with {{1}} or {{name}} placeholders (do not mix the two styles)"),
		"footer_content":   mxStr("Footer text"),
		"buttons": mxArr("Buttons as Meta defines them (QUICK_REPLY, URL, PHONE_NUMBER, COPY_CODE, FLOW…)",
			mxFreeObj("One button, e.g. {\"type\":\"QUICK_REPLY\",\"text\":\"Yes\"}")),
		"sample_values":               mxArr("Example values for the placeholders, required by Meta for review", map[string]any{}),
		"add_security_recommendation": mxBool("AUTHENTICATION only: add the do-not-share warning"),
		"code_expiration_minutes":     mxInt("AUTHENTICATION only: code expiry 1-90 minutes; 0 for none"),
	})

	register(
		Entry{
			Area:    "templates",
			Method:  "GET",
			Path:    "/api/templates",
			Summary: "List message templates with their approval status.",
			Query: mxWithPaging(
				mxQ("account", "string", "Only templates of this WhatsApp account (account name)"),
				mxQ("status", "string", "Status filter, e.g. APPROVED, PENDING, REJECTED, DRAFT"),
				mxQ("category", "string", "MARKETING, UTILITY or AUTHENTICATION"),
				mxQ("search", "string", "Text to match against template name"),
			),
			Tool: "whatomate_list_templates",
		},
		Entry{
			Area:    "templates",
			Method:  "POST",
			Path:    "/api/templates",
			Summary: "Create a template as a local DRAFT. Nothing goes to Meta until it is submitted.",
			Body:    templateBody,
		},
		Entry{
			Area:    "templates",
			Method:  "GET",
			Path:    "/api/templates/{id}",
			Summary: "Get one template with its content, buttons and status.",
		},
		Entry{
			Area:   "templates",
			Method: "PUT",
			Path:   "/api/templates/{id}",
			Summary: "Edit a template. Editing an approved or rejected template turns it back into a DRAFT " +
				"that must be submitted again.",
			Body: templateBody,
		},
		Entry{
			Area:        "templates",
			Method:      "DELETE",
			Path:        "/api/templates/{id}",
			Summary:     "Delete a template, also from Meta when it was submitted there.",
			SideEffects: "deletes data; calls Meta to delete the template",
		},
		Entry{
			Area:    "templates",
			Method:  "POST",
			Path:    "/api/templates/sync",
			Summary: "Pull all templates and their approval statuses for one WhatsApp account from Meta.",
			Query:   []Param{mxQ("account", "string", "WhatsApp account name (or send whatsapp_account in the body)")},
			Body: mxObj(nil, map[string]any{
				"whatsapp_account": mxStr("WhatsApp account name (if not given as ?account=)"),
			}),
			SideEffects: "calls Meta",
		},
		Entry{
			Area:        "templates",
			Method:      "POST",
			Path:        "/api/templates/{id}/publish",
			Summary:     "Submit a template to Meta for approval (status becomes PENDING until Meta decides).",
			SideEffects: "calls Meta to submit the template for review",
		},
		Entry{
			Area:   "templates",
			Method: "POST",
			Path:   "/api/templates/upload-media",
			Summary: "Upload a sample image, video or document for a template's media header. Returns a " +
				"Meta file handle to put in header_content.",
			Multipart: &MultipartSpec{
				FileField:   "file",
				Accept:      "image/*,video/*,application/pdf",
				ExtraFields: []Param{mxQ("account", "string", "WhatsApp account name the template belongs to")},
			},
			SideEffects: "uploads the file to Meta",
		},
	)
}
