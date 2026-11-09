package mcp

func init() {
	registerArea(Area{
		Key:   "campaigns",
		Title: "Bulk campaigns",
		Description: "Bulk WhatsApp template campaigns: create, add recipients, schedule or start, pause, cancel, " +
			"retry failures, resend and download reports. Campaigns created by RSVP reminders are locked: " +
			"they cannot be edited, deleted, resent or have recipients changed.",
	})

	campaignBody := mxObj([]string{"name", "whatsapp_account", "template_id"}, map[string]any{
		"name":             mxStr("Campaign name"),
		"whatsapp_account": mxStr("WhatsApp account name to send from"),
		"template_id":      mxStr("UUID of an approved template"),
		"header_media_url": mxStr("Public URL of the header image/video/document (media-header templates)"),
		"header_media_id":  mxStr("Already-uploaded WhatsApp media ID for the header"),
		"scheduled_at":     mxStr("ISO 8601 date-time to send at; empty means send when started"),
	})

	register(
		Entry{
			Area:    "campaigns",
			Method:  "GET",
			Path:    "/api/campaigns",
			Summary: "List campaigns with their status and delivery counts.",
			Query: mxWithPaging(
				mxQEnum("status", "Only campaigns in this status",
					"draft", "scheduled", "queued", "processing", "paused", "completed", "cancelled", "failed"),
				mxQ("whatsapp_account", "string", "Only campaigns of this WhatsApp account (account name)"),
				mxQ("search", "string", "Text to match against campaign name"),
			),
			Tool: "whatomate_list_campaigns",
		},
		Entry{
			Area:    "campaigns",
			Method:  "POST",
			Path:    "/api/campaigns",
			Summary: "Create a draft campaign. Add recipients next, then start it. Nothing is sent yet.",
			Body:    campaignBody,
		},
		Entry{
			Area:   "campaigns",
			Method: "GET",
			Path:   "/api/campaigns/{id}",
			Summary: "Get one campaign with progress (sent, delivered, read, failed counts) and any pause " +
				"reason (e.g. WhatsApp delivery reports not arriving).",
			Tool: "whatomate_get_campaign",
		},
		Entry{
			Area:    "campaigns",
			Method:  "GET",
			Path:    "/api/campaigns/{id}/progress",
			Summary: "Same as getting the campaign: current status and delivery counts, for polling progress.",
		},
		Entry{
			Area:    "campaigns",
			Method:  "PUT",
			Path:    "/api/campaigns/{id}",
			Summary: "Edit a draft campaign. RSVP reminder campaigns cannot be edited.",
			Body:    campaignBody,
		},
		Entry{
			Area:   "campaigns",
			Method: "DELETE",
			Path:   "/api/campaigns/{id}",
			Summary: "Delete a campaign and its recipients. Running campaigns and RSVP reminder campaigns " +
				"cannot be deleted.",
			SideEffects: "deletes data",
		},
		Entry{
			Area:   "campaigns",
			Method: "POST",
			Path:   "/api/campaigns/{id}/start",
			Summary: "Start (or resume) a draft, scheduled or paused campaign. With a future scheduled_at it is " +
				"scheduled instead. Refused with 409 while WhatsApp delivery reports for the number are not arriving.",
			SideEffects: "starts or schedules a campaign that sends WhatsApp template messages to every pending recipient",
		},
		Entry{
			Area:        "campaigns",
			Method:      "POST",
			Path:        "/api/campaigns/{id}/pause",
			Summary:     "Pause a running campaign; pending recipients wait until it is started again.",
			SideEffects: "stops sending",
		},
		Entry{
			Area:        "campaigns",
			Method:      "POST",
			Path:        "/api/campaigns/{id}/cancel",
			Summary:     "Cancel a campaign for good; unsent recipients are not messaged.",
			SideEffects: "stops sending permanently",
		},
		Entry{
			Area:        "campaigns",
			Method:      "POST",
			Path:        "/api/campaigns/{id}/retry-failed",
			Summary:     "Send again to the recipients that failed in a completed, paused or failed campaign.",
			SideEffects: "sends WhatsApp template messages",
		},
		Entry{
			Area:   "campaigns",
			Method: "POST",
			Path:   "/api/campaigns/{id}/resend",
			Summary: "Copy a finished campaign and start the copy to the same recipients (inactive contacts are " +
				"skipped). Not allowed for RSVP reminder campaigns: start a new reminder from RSVP Results.",
			SideEffects: "creates and starts a new campaign that sends WhatsApp template messages",
		},
		Entry{
			Area:    "campaigns",
			Method:  "GET",
			Path:    "/api/campaigns/{id}/report.xlsx",
			Summary: "Download the campaign delivery report as an Excel file.",
		},
	)
}
