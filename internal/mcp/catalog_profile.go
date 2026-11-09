package mcp

func init() {
	registerArea(Area{
		Key:         "profile",
		Title:       "My profile",
		Description: "The signed-in user: who you are, your notification settings, your availability and the organisations you belong to.",
	})
	register(
		Entry{
			Area:    "profile",
			Method:  "GET",
			Path:    "/api/me",
			Summary: "Show the signed-in user: name, email, role, permissions, current organisation and settings.",
		},
		Entry{
			Area:    "profile",
			Method:  "PUT",
			Path:    "/api/me/settings",
			Summary: "Change your own notification settings and the ringtone used for incoming calls.",
			Body: admObj(map[string]any{
				"email_notifications": admBool("Receive email notifications."),
				"new_message_alerts":  admBool("Show alerts for new messages."),
				"campaign_updates":    admBool("Receive campaign progress updates."),
				"call_ringtone":       admStr("Sound played for an incoming call. Empty keeps the current choice."),
			}),
			SideEffects: "Changes your own notification preferences.",
		},
		Entry{
			Area:    "profile",
			Method:  "PUT",
			Path:    "/api/me/availability",
			Summary: "Mark yourself available or away for chat assignment.",
			Body: admObj(map[string]any{
				"is_available": admBool("true = available to receive chats and transfers, false = away."),
			}, "is_available"),
			SideEffects: "Changes whether new chats and transfers are routed to you; logs an availability period.",
		},
		Entry{
			Area:    "profile",
			Method:  "GET",
			Path:    "/api/me/organizations",
			Summary: "List the organisations you belong to (use them to switch organisation).",
		},
	)
}
