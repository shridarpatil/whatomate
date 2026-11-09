package mcp

func init() {
	registerArea(Area{
		Key:   "rsvp",
		Title: "RSVP events",
		Description: "RSVP events answered over WhatsApp: event setup, guest roster, results and tallies, " +
			"invitations, reminders, follow-ups and re-prompts. Adding or importing guests never sends a message; " +
			"invitations, reminders and follow-ups are explicit sends.",
	})

	eventBody := mxObj(nil, map[string]any{
		"name":                 mxStr("Event name (required when creating)"),
		"description":          mxStr("Event description"),
		"event_date":           mxStr("Event date-time, ISO 8601"),
		"rsvp_close_at":        mxStr("When replies stop being accepted, ISO 8601"),
		"whatsapp_account":     mxStr("WhatsApp account name used for the event"),
		"flow_id":              mxStr("Chatbot flow UUID that collects the RSVP answers"),
		"keyword":              mxStr("Keyword guests send to start the RSVP (unique among active events)"),
		"attendance_field":     mxStr("Flow answer key that holds the attendance reply"),
		"attendance_map":       mxFreeObj("Maps answer values to attendance (yes, no, maybe)"),
		"spouse_mobile_field":  mxStr("Flow answer key holding the spouse mobile number"),
		"duplicate_message":    mxStr("Reply sent when a guest who already answered starts again"),
		"template_id":          mxStr("Invitation template UUID"),
		"reminder_enabled":     mxBool("Legacy single reminder switch"),
		"reminder_at":          mxStr("Legacy single reminder time, ISO 8601"),
		"reminder_template_id": mxStr("Reminder template UUID"),
		"access_mode":          mxEnum("Who may answer", "guest_list", "open_keyword"),
		"not_invited_message":  mxStr("Reply for people not on the guest list (guest_list mode)"),
		"headcount_contributors": mxArr("Extra headcount columns built from answers", mxObj(nil, map[string]any{
			"label":        mxStr("Column label"),
			"answer_key":   mxStr("Flow answer key"),
			"mode":         mxEnum("How the answer counts", "boolean", "numeric", "attendance"),
			"match_values": mxStrArr("Answer values that count (boolean mode)"),
		})),
	})

	register(
		Entry{
			Area:    "rsvp",
			Method:  "GET",
			Path:    "/api/rsvp-events",
			Summary: "List RSVP events.",
			Query: mxWithPaging(
				mxQ("search", "string", "Text to match against event name"),
				mxQEnum("status", "Only events in this status", "draft", "active", "closed"),
			),
			Tool: "whatomate_list_rsvp_events",
		},
		Entry{Area: "rsvp", Method: "POST", Path: "/api/rsvp-events", Summary: "Create an RSVP event (starts as draft). Sends nothing.", Body: eventBody},
		Entry{Area: "rsvp", Method: "GET", Path: "/api/rsvp-events/{id}", Summary: "Get one RSVP event with its settings."},
		Entry{Area: "rsvp", Method: "PUT", Path: "/api/rsvp-events/{id}", Summary: "Update an RSVP event's settings. Sends nothing.", Body: eventBody},
		Entry{
			Area:        "rsvp",
			Method:      "DELETE",
			Path:        "/api/rsvp-events/{id}",
			Summary:     "Delete an RSVP event.",
			SideEffects: "deletes data",
		},
		Entry{
			Area:    "rsvp",
			Method:  "POST",
			Path:    "/api/rsvp-events/{id}/activate",
			Summary: "Activate an event so its keyword starts the RSVP flow. Sends nothing by itself.",
		},
		Entry{
			Area:    "rsvp",
			Method:  "POST",
			Path:    "/api/rsvp-events/{id}/close",
			Summary: "Close an event; new RSVP replies are no longer accepted.",
		},
		Entry{
			Area:    "rsvp",
			Method:  "GET",
			Path:    "/api/rsvp-events/{id}/tally",
			Summary: "RSVP results summary: member attendance and spouse attendance counts (attending, not attending, maybe, pending) and headcounts.",
			Tool:    "whatomate_get_rsvp_results",
		},
		Entry{
			Area:    "rsvp",
			Method:  "GET",
			Path:    "/api/rsvp-events/{id}/responses",
			Summary: "List received RSVP responses with their answers.",
			Query: mxWithPaging(
				mxQEnum("attendance", "Only this attendance", "pending", "yes", "no", "maybe"),
				mxQ("title_field", "string", "Answer key to filter on (e.g. attendance_title)"),
				mxQ("title_value", "string", "Value that title_field must equal; __pending__ matches empty"),
				mxQ("title_field2", "string", "Second answer key filter (combined with AND)"),
				mxQ("title_value2", "string", "Value for title_field2; __pending__ matches empty"),
				mxQ("search", "string", "Text to match against guest name or phone"),
			),
		},
		Entry{
			Area:    "rsvp",
			Method:  "PUT",
			Path:    "/api/rsvp-events/{id}/responses/{responseId}",
			Summary: "Correct a guest's response by hand (attendance, answers or notes). Sends nothing.",
			Body: mxObj(nil, map[string]any{
				"attendance": mxEnum("Attendance", "pending", "yes", "no", "maybe"),
				"answers":    mxFreeObj("Answer values keyed by answer key"),
				"notes":      mxStr("Organiser notes"),
			}),
		},
		Entry{
			Area:        "rsvp",
			Method:      "DELETE",
			Path:        "/api/rsvp-events/{id}/responses/{responseId}",
			Summary:     "Remove a guest/response from the event.",
			SideEffects: "deletes data",
		},
		Entry{
			Area:    "rsvp",
			Method:  "GET",
			Path:    "/api/rsvp-events/{id}/export",
			Summary: "Download all responses with their answers as an Excel file.",
		},
		Entry{
			Area:   "rsvp",
			Method: "POST",
			Path:   "/api/rsvp-events/{id}/flow-form",
			Summary: "Create a DRAFT WhatsApp Flow form (mobile, attendance, spouse mobile) for the event. " +
				"Review it under Flows, then save to Meta and publish from there.",
		},
	)
}
