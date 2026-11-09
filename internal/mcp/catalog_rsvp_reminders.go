package mcp

// rsvpParamsDesc explains RSVP reminder/follow-up template variable mappings.
const rsvpParamsDesc = "Template placeholder mapping, placeholder name or position to a value. Values may be fixed text " +
	"or tokens resolved per guest: {{member_name}}, {{event_name}}, {{event_date}}, {{answer.<key>}}. " +
	"Every placeholder of the template must be mapped."

func init() {
	register(
		Entry{
			Area:   "rsvp",
			Method: "GET",
			Path:   "/api/rsvp-events/{id}/reminders/preview",
			Summary: "Preview who a reminder would reach: not-started guests (or the listed responses) and who " +
				"would be skipped and why. Sends nothing.",
			Permission: "rsvp:read",
			Query:      []Param{mxQ("response_ids", "string", "Comma-separated response UUIDs; empty previews all not-started guests")},
		},
		Entry{
			Area:   "rsvp",
			Method: "POST",
			Path:   "/api/rsvp-events/{id}/reminders/media",
			Summary: "Upload header media (max 16 MB) for a reminder or follow-up template. Returns staging_id and " +
				"staging_filename to pass to the send call.",
			Permission: "rsvp:execute",
			Multipart:  &MultipartSpec{FileField: "file", Accept: "image/*,video/*,application/pdf"},
		},
		Entry{
			Area:   "rsvp",
			Method: "POST",
			Path:   "/api/rsvp-events/{id}/reminders/send",
			Summary: "Send a reminder now to selected guests or to all guests who have not started (minus exclusions). " +
				"Creates a locked RSVP reminder campaign that rechecks each guest just before sending.",
			Permission: "rsvp:execute",
			Body: mxObj(nil, map[string]any{
				"response_ids":         mxStrArr("Response UUIDs to remind (or set all_not_started)"),
				"all_not_started":      mxBool("Remind every guest who has not started"),
				"exclude_response_ids": mxStrArr("With all_not_started: response UUIDs to leave out"),
				"template_id":          mxStr("Reminder template UUID (default: the event's reminder template)"),
				"template_params":      mxStrMap(rsvpParamsDesc),
				"staging_id":           mxStr("From the reminders/media upload, for media-header templates"),
				"staging_filename":     mxStr("From the reminders/media upload"),
			}),
			SideEffects: "creates and starts a campaign that sends WhatsApp template messages",
		},
		Entry{
			Area:       "rsvp",
			Method:     "GET",
			Path:       "/api/rsvp-events/{id}/reminders",
			Summary:    "List scheduled reminders of the event with their status and linked campaign.",
			Permission: "rsvp:read",
		},
		Entry{
			Area:       "rsvp",
			Method:     "POST",
			Path:       "/api/rsvp-events/{id}/reminders",
			Summary:    "Schedule a reminder. At that time every guest who still has not started gets the template.",
			Permission: "rsvp:write",
			Body: mxObj([]string{"scheduled_at", "template_id"}, map[string]any{
				"scheduled_at":    mxStr("When to send, ISO 8601"),
				"template_id":     mxStr("Reminder template UUID"),
				"template_params": mxStrMap(rsvpParamsDesc),
			}),
			SideEffects: "schedules WhatsApp template messages",
		},
		Entry{
			Area:       "rsvp",
			Method:     "DELETE",
			Path:       "/api/rsvp-events/{id}/reminders/{scheduleId}",
			Summary:    "Cancel a scheduled reminder that has not run yet.",
			Permission: "rsvp:write",
		},
		Entry{
			Area:       "rsvp",
			Method:     "GET",
			Path:       "/api/rsvp-events/{id}/followup/preview",
			Summary:    "Preview a follow-up audience: who would be messaged and who skipped and why. Sends nothing.",
			Permission: "rsvp:read",
			Query: []Param{
				mxQEnum("audience", "Which guests", "not_started", "responded_yes", "responded_no", "missing_answer"),
				mxQ("answer_key", "string", "With missing_answer: the question (answer key) they have not answered"),
			},
		},
		Entry{
			Area:   "rsvp",
			Method: "POST",
			Path:   "/api/rsvp-events/{id}/followup/send",
			Summary: "Send a follow-up template that starts a second chatbot flow to a guest audience. " +
				"Creates a campaign.",
			Permission: "rsvp:execute",
			Body: mxObj([]string{"audience", "template_id", "flow_id"}, map[string]any{
				"audience":         mxEnum("Which guests", "not_started", "responded_yes", "responded_no", "missing_answer"),
				"answer_key":       mxStr("With missing_answer: the unanswered answer key"),
				"flow_id":          mxStr("Chatbot flow UUID the follow-up starts (not the event's main RSVP flow; same WhatsApp account)"),
				"template_id":      mxStr("Follow-up template UUID"),
				"template_params":  mxStrMap(rsvpParamsDesc),
				"staging_id":       mxStr("From the reminders/media upload, for media-header templates"),
				"staging_filename": mxStr("From the reminders/media upload"),
				"response_ids":     mxStrArr("Optional: only these response UUIDs within the audience"),
			}),
			SideEffects: "creates and starts a campaign that sends WhatsApp template messages",
		},
	)
}
