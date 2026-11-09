package mcp

func init() {
	register(
		Entry{
			Area:    "campaigns",
			Method:  "GET",
			Path:    "/api/campaigns/{id}/recipients",
			Summary: "List every recipient of a campaign with its delivery status and error.",
		},
		Entry{
			Area:   "campaigns",
			Method: "POST",
			Path:   "/api/campaigns/{id}/recipients/import",
			Summary: "Add recipients to a campaign by phone number list, existing contacts or tag names. " +
				"Phone numbers are stored in canonical form. Not allowed on RSVP reminder campaigns.",
			Body: mxObj(nil, map[string]any{
				"recipients": mxArr("Recipients by phone number", mxObj([]string{"phone_number"}, map[string]any{
					"phone_number":    mxStr("Phone number with country code"),
					"recipient_name":  mxStr("Name used for the contact and name placeholders"),
					"template_params": mxFreeObj("Body placeholder values keyed by name or position (\"1\")"),
					"header_params":   mxFreeObj("TEXT header placeholder value keyed by its name"),
				})),
				"contact_ids": mxStrArr("Existing contact UUIDs"),
				"tag_names":   mxStrArr("Add every contact carrying any of these tags"),
			}),
			SideEffects: "when the campaign is already running, new recipients are queued and messaged straight away",
		},
		Entry{
			Area:        "campaigns",
			Method:      "DELETE",
			Path:        "/api/campaigns/{id}/recipients/{recipientId}",
			Summary:     "Remove one recipient from a draft campaign.",
			SideEffects: "deletes data",
		},
		Entry{
			Area:   "campaigns",
			Method: "POST",
			Path:   "/api/campaigns/{id}/media",
			Summary: "Upload the header image, video or document (max 16 MB) for a draft campaign whose " +
				"template has a media header.",
			Multipart: &MultipartSpec{
				FileField: "file",
				Accept:    "image/*,video/*,application/pdf",
			},
		},
		Entry{
			Area:    "campaigns",
			Method:  "GET",
			Path:    "/api/campaigns/{id}/media",
			Summary: "Download the campaign's uploaded header media file (preview).",
		},
	)
}
