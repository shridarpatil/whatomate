package mcp

func init() {
	register(
		Entry{
			Area:       "rsvp",
			Method:     "GET",
			Path:       "/api/rsvp-events/{id}/guests",
			Summary:    "The guest roster: every guest with journey status (not started, in progress, responded), attendance and reminder count.",
			Permission: "rsvp:read",
			Query: mxWithPaging(
				mxQEnum("journey_status", "Where the guest is in the RSVP", "not_started", "in_progress", "responded"),
				mxQEnum("attendance", "Stored attendance", "pending", "yes", "no", "maybe"),
				mxQEnum("member_status", "Member attendance", "attending", "not_attending", "maybe", "pending"),
				mxQEnum("spouse_status", "Spouse attendance (pending includes attending spouses without a valid mobile)",
					"attending", "not_attending", "maybe", "pending"),
				mxQEnum("source", "How the guest was added",
					"contact_selection", "spreadsheet_import", "open_keyword", "api", "legacy"),
				mxQEnum("reminded", "Whether a reminder was sent", "yes", "no"),
				mxQ("search", "string", "Text to match against guest name or phone"),
			),
		},
		Entry{
			Area:       "rsvp",
			Method:     "GET",
			Path:       "/api/rsvp-events/{id}/guest-candidates",
			Summary:    "Contacts that can be added to the guest list.",
			Permission: "rsvp:read",
			Query:      mxWithPaging(mxQ("search", "string", "Text to match against contact name or phone")),
		},
		Entry{
			Area:       "rsvp",
			Method:     "POST",
			Path:       "/api/rsvp-events/{id}/guests",
			Summary:    "Add existing contacts to the guest list. Never sends a message; invite them explicitly.",
			Permission: "rsvp:write",
			Body: mxObj([]string{"contact_ids"}, map[string]any{
				"contact_ids": mxStrArr("Contact UUIDs to add"),
			}),
		},
		Entry{
			Area:   "rsvp",
			Method: "POST",
			Path:   "/api/rsvp-events/{id}/guests/import",
			Summary: "Import guests from a CSV or XLSX file (max 10 MB, 10 000 rows) with a phone/mobile column " +
				"and optional name column. Creates missing contacts in canonical phone form. Never sends a message.",
			Permission: "rsvp:write",
			Multipart: &MultipartSpec{
				FileField: "file",
				Accept:    "text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			},
		},
		Entry{
			Area:   "rsvp",
			Method: "POST",
			Path:   "/api/rsvp-events/{id}/send-invites",
			Summary: "Add the given contacts as guests and send each the event's invitation template " +
				"(best effort; failures are counted, not fatal).",
			Permission: "rsvp:execute",
			Body: mxObj([]string{"contact_ids"}, map[string]any{
				"contact_ids": mxStrArr("Contact UUIDs to invite"),
			}),
			SideEffects: "sends WhatsApp invitation template messages",
		},
		Entry{
			Area:    "rsvp",
			Method:  "GET",
			Path:    "/api/rsvp-events/{id}/reprompt/preview",
			Summary: "Preview which guests with an unfinished RSVP would be re-prompted and the message they would get. Sends nothing.",
		},
		Entry{
			Area:   "rsvp",
			Method: "POST",
			Path:   "/api/rsvp-events/{id}/reprompt",
			Summary: "Re-send the RSVP flow to guests who started but did not finish, resetting their response to " +
				"pending first. Optionally only the listed phone numbers.",
			Body: mxObj(nil, map[string]any{
				"phones": mxStrArr("Only re-prompt these phone numbers (matched on trailing digits); empty means all incomplete guests"),
			}),
			SideEffects: "sends WhatsApp messages",
		},
		Entry{
			Area:   "rsvp",
			Method: "POST",
			Path:   "/api/rsvp-events/{id}/recover-partials",
			Summary: "Save partial answers of guests who dropped off mid-flow, rebuilt from their chat history. " +
				"Safe to repeat. Sends nothing.",
		},
	)
}
