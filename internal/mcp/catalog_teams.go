package mcp

func init() {
	registerArea(Area{
		Key:         "teams",
		Title:       "Teams",
		Description: "Agent teams that receive chat and call transfers, how work is shared among their members, and who is in each team.",
	})
	teamBody := func() map[string]any {
		return admObj(map[string]any{
			"name":                   admStr("Team name."),
			"description":            admStr("What the team handles."),
			"assignment_strategy":    admEnum("How new chats are shared among agents (default round_robin).", "round_robin", "load_balanced", "manual"),
			"per_agent_timeout_secs": admInt("Seconds an agent has to pick up before the next one is tried."),
			"is_active":              admBool("false stops the team receiving new transfers."),
		}, "name")
	}
	register(
		Entry{
			Area:       "teams",
			Method:     "GET",
			Path:       "/api/teams",
			Summary:    "List teams. With teams:read you see every team; otherwise only the teams you belong to.",
			Permission: "teams:read",
			Query: append(admPaging(),
				Param{Name: "search", Type: "string", Description: "Match team name."},
			),
		},
		Entry{
			Area:        "teams",
			Method:      "POST",
			Path:        "/api/teams",
			Summary:     "Create a team.",
			Permission:  "teams:write",
			Body:        teamBody(),
			SideEffects: "Adds a team that chats and calls can be transferred to.",
		},
		Entry{
			Area:       "teams",
			Method:     "GET",
			Path:       "/api/teams/{id}",
			Summary:    "Show one team with its members.",
			Permission: "teams:read",
		},
		Entry{
			Area:        "teams",
			Method:      "PUT",
			Path:        "/api/teams/{id}",
			Summary:     "Edit a team's name, description, assignment strategy, timeout or active status (teams:write or a manager of the team).",
			Permission:  "teams:write",
			Body:        teamBody(),
			SideEffects: "Changes how incoming chats are routed to this team's agents.",
		},
		Entry{
			Area:        "teams",
			Method:      "DELETE",
			Path:        "/api/teams/{id}",
			Summary:     "Delete a team.",
			Permission:  "teams:delete",
			SideEffects: "Removes the team and its memberships; it can no longer receive transfers.",
		},
		Entry{
			Area:       "teams",
			Method:     "GET",
			Path:       "/api/teams/{id}/members",
			Summary:    "List the members of a team and their team role.",
			Permission: "teams:read",
		},
		Entry{
			Area:       "teams",
			Method:     "POST",
			Path:       "/api/teams/{id}/members",
			Summary:    "Add a user to a team (teams:write, or a team manager adding agents).",
			Permission: "teams:write",
			Body: admObj(map[string]any{
				"user_id": admStr("ID of the user to add (must belong to this organisation)."),
				"role":    admEnum("Role inside the team (default agent). Only teams:write holders can add managers.", "manager", "agent"),
			}, "user_id"),
			SideEffects: "The user starts receiving this team's chats and transfers.",
		},
		Entry{
			Area:        "teams",
			Method:      "DELETE",
			Path:        "/api/teams/{id}/members/{member_user_id}",
			Summary:     "Remove a user from a team. member_user_id is the user's ID (teams:write, or a team manager removing agents).",
			Permission:  "teams:write",
			SideEffects: "The user stops receiving this team's chats and transfers.",
		},
	)
}
