package mcp

func init() {
	registerArea(Area{
		Key:         "roles",
		Title:       "Roles and permissions",
		Description: "Custom roles for the current organisation and the full list of permissions a role can grant.",
	})
	roleBody := func(required ...string) map[string]any {
		return admObj(map[string]any{
			"name":        admStr("Role name, unique in the organisation."),
			"description": admStr("What the role is for."),
			"is_default":  admBool("true = new users get this role by default (clears the flag on other roles)."),
			"permissions": admArr(admStr("Permission key \"resource:action\", e.g. \"contacts:read\", \"campaigns:write\", \"organizations:assign\"."),
				"Full list of permissions the role grants, each as \"resource:action\" (see GET /api/permissions for valid keys). On update this replaces the whole list; send an empty list only if the role should have no permissions."),
		}, required...)
	}
	register(
		Entry{
			Area:    "roles",
			Method:  "GET",
			Path:    "/api/roles",
			Summary: "List roles in the current organisation with their permissions and how many users have each.",
			Query: append(admPaging(),
				Param{Name: "search", Type: "string", Description: "Match role name."},
			),
		},
		Entry{
			Area:        "roles",
			Method:      "POST",
			Path:        "/api/roles",
			Summary:     "Create a custom role with a set of permissions.",
			Body:        roleBody("name"),
			SideEffects: "Adds a role; with is_default=true it becomes the default role for new users.",
		},
		Entry{
			Area:    "roles",
			Method:  "GET",
			Path:    "/api/roles/{id}",
			Summary: "Show one role and its permissions.",
		},
		Entry{
			Area:        "roles",
			Method:      "PUT",
			Path:        "/api/roles/{id}",
			Summary:     "Edit a role's name, description, default flag or permission list. For system roles only the description can change, plus permissions when you are a super admin.",
			Body:        roleBody(),
			SideEffects: "Changes what every user with this role can see and do, immediately.",
		},
		Entry{
			Area:        "roles",
			Method:      "DELETE",
			Path:        "/api/roles/{id}",
			Summary:     "Delete a custom role (system roles and roles still assigned to users cannot be deleted).",
			SideEffects: "Permanently removes the role.",
		},
		Entry{
			Area:    "roles",
			Method:  "GET",
			Path:    "/api/permissions",
			Summary: "List every permission (resource + action) that can be put on a role.",
		},
	)
}
