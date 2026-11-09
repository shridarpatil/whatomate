package mcp

func init() {
	registerArea(Area{
		Key:         "users",
		Title:       "Users",
		Description: "Team member accounts in the current organisation: list, create, edit, deactivate and delete users.",
	})
	userProps := func() map[string]any {
		return map[string]any{
			"email":           admStr("Login email address (must be a valid, unused address)."),
			"password":        admStr("Login password (another user's only; AI apps cannot change your own)."),
			"full_name":       admStr("Person's full name."),
			"role_id":         admStr("ID of the role to give the user in this organisation (see GET /api/roles)."),
			"is_active":       admBool("false blocks the user from signing in."),
			"organization_id": admStr("Super admins only: put the user in this organisation instead of the current one."),
			"is_super_admin":  admBool("Super admins only: grant or remove super-admin rights."),
		}
	}
	register(
		Entry{
			Area:       "users",
			Method:     "GET",
			Path:       "/api/users",
			Summary:    "List users in the current organisation, with their role and online status.",
			Permission: "users:read",
			Query: append(admPaging(),
				Param{Name: "search", Type: "string", Description: "Match name or email."},
				Param{Name: "role_id", Type: "string", Description: "Only users with this role ID."},
				Param{Name: "online_only", Type: "boolean", Description: "true = only users online right now."},
			),
		},
		Entry{
			Area:        "users",
			Method:      "POST",
			Path:        "/api/users",
			Summary:     "Create a new user account in the current organisation.",
			Permission:  "users:write",
			Body:        admObj(userProps(), "email", "password", "full_name"),
			SideEffects: "Creates a login for a new person (or restores a previously deleted account with the same email).",
		},
		Entry{
			Area:    "users",
			Method:  "GET",
			Path:    "/api/users/{id}",
			Summary: "Show one user of the current organisation, including their role here.",
		},
		Entry{
			Area:        "users",
			Method:      "PUT",
			Path:        "/api/users/{id}",
			Summary:     "Edit a user: name, email, password, role or active status. You may always edit yourself; editing others or changing a role needs users:write. Changing your own password or email is not available to AI apps; the person does it in Whatomate under their profile.",
			Permission:  "users:write",
			Body:        admObj(userProps()),
			SideEffects: "Changes the user's details; a new password or is_active=false affects their ability to sign in.",
		},
		Entry{
			Area:        "users",
			Method:      "DELETE",
			Path:        "/api/users/{id}",
			Summary:     "Delete a user (or remove a member who belongs to another organisation from this one). You cannot delete yourself.",
			Permission:  "users:delete",
			SideEffects: "Deletes the user's account or their membership in this organisation; they lose access.",
		},
	)
}
