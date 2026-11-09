package mcp

func init() {
	registerArea(Area{
		Key:         "organizations",
		Title:       "Organisations and members",
		Description: "Organisations, the current organisation's general and calling settings, its hold/ringback audio, and which existing users are members (keyed by user_id; new accounts are created under users).",
	})
	register(
		Entry{
			Area:       "organizations",
			Method:     "GET",
			Path:       "/api/organizations",
			Summary:    "List organisations (super admins see all; others need organizations:read).",
			Permission: "organizations:read",
		},
		Entry{
			Area:       "organizations",
			Method:     "POST",
			Path:       "/api/organizations",
			Summary:    "Create a new organisation; you become its admin.",
			Permission: "organizations:write",
			Body: admObj(map[string]any{
				"name": admStr("Organisation name."),
			}, "name"),
			SideEffects: "Creates a new organisation with default roles and adds you to it.",
		},
		Entry{
			Area:    "organizations",
			Method:  "GET",
			Path:    "/api/organizations/current",
			Summary: "Show the organisation you are currently working in.",
		},
		Entry{
			Area:       "organizations",
			Method:     "GET",
			Path:       "/api/organizations/members",
			Summary:    "List members of the current organisation with their role here.",
			Permission: "organizations:read",
			Query: append(admPaging(),
				Param{Name: "search", Type: "string", Description: "Match member name or email."},
			),
		},
		Entry{
			Area:       "organizations",
			Method:     "POST",
			Path:       "/api/organizations/members",
			Summary:    "Add an existing user (by user_id or email) to the current organisation with a role. To create a brand-new account use POST /api/users.",
			Permission: "organizations:assign",
			Body: admObj(map[string]any{
				"user_id": admStr("ID of the existing user to add. Give this or email."),
				"email":   admStr("Email of the existing user to add. Give this or user_id."),
				"role_id": admStr("Role ID in this organisation (see GET /api/roles). Omit for the default role."),
			}),
			SideEffects: "Gives the user access to this organisation.",
		},
		Entry{
			Area:       "organizations",
			Method:     "PUT",
			Path:       "/api/organizations/members/{member_id}",
			Summary:    "Change a member's role in the current organisation. member_id is the member's user_id.",
			Permission: "organizations:assign",
			Body: admObj(map[string]any{
				"role_id": admStr("New role ID in this organisation."),
			}, "role_id"),
			SideEffects: "Changes what the member can do in this organisation.",
		},
		Entry{
			Area:        "organizations",
			Method:      "DELETE",
			Path:        "/api/organizations/members/{member_id}",
			Summary:     "Remove a member from the current organisation. member_id is the member's user_id; the user account itself is kept.",
			Permission:  "organizations:assign",
			SideEffects: "The user loses access to this organisation.",
		},
		Entry{
			Area:    "organizations",
			Method:  "GET",
			Path:    "/api/org/settings",
			Summary: "Show the current organisation's settings: name, timezone, date format, phone masking, message retention, calling options and Meta app details.",
		},
		Entry{
			Area:       "organizations",
			Method:     "PUT",
			Path:       "/api/org/settings",
			Summary:    "Change the current organisation's settings. Only the fields you send change. Changing meta_app_id, meta_config_id or meta_app_secret needs accounts:write.",
			Permission: "",
			Body: admObj(map[string]any{
				"name":                   admStr("Organisation name (empty is ignored)."),
				"timezone":               admStr("IANA timezone, e.g. Asia/Kolkata."),
				"date_format":            admStr("Display date format."),
				"mask_phone_numbers":     admBool("Hide contact phone numbers from agents."),
				"message_retention_days": admInt("0 = keep messages forever, 1-60 = delete messages older than N days."),
				"calling_enabled":        admBool("Turn WhatsApp calling on or off."),
				"max_call_duration":      admInt("Longest allowed call, in seconds."),
				"transfer_timeout_secs":  admInt("How long a transferred call waits for an agent, in seconds."),
				"hold_music_file":        admStr("Hold-music audio file name (upload via POST /api/org/audio?type=hold_music)."),
				"ringback_file":          admStr("Ringback audio file name (upload via POST /api/org/audio?type=ringback)."),
				"meta_app_id":            admStr("Meta app ID used for this organisation (needs accounts:write)."),
				"meta_config_id":         admStr("Meta Embedded Signup configuration ID (needs accounts:write)."),
				"meta_app_secret":        admStr("Meta app secret (needs accounts:write)."),
			}),
			SideEffects: "Changes organisation-wide behaviour; a shorter message_retention_days makes older messages get deleted.",
		},
		Entry{
			Area:       "organizations",
			Method:     "POST",
			Path:       "/api/org/audio",
			Summary:    "Upload the organisation's hold music or ringback audio (max 5 MB; ogg, opus, mp3, wav).",
			Permission: "organizations:write",
			Query: []Param{
				{Name: "type", Type: "string", Required: true, Enum: []string{"hold_music", "ringback"}, Description: "Which sound this file is."},
			},
			Multipart:   &MultipartSpec{FileField: "file", Accept: "audio/*"},
			SideEffects: "Stores an audio file used during calls.",
		},
	)
}
