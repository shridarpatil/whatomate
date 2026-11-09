package mcp

func init() {
	registerArea(Area{
		Key:         "settings",
		Title:       "Settings: sign-in and Meta onboarding",
		Description: "Single sign-on providers for the organisation, and the super-admin Meta app configuration used by the WhatsApp onboarding wizard. Organisation general/calling settings live under organizations.",
	})
	register(
		Entry{
			Area:    "settings",
			Method:  "GET",
			Path:    "/api/settings/sso",
			Summary: "List the organisation's single sign-on providers and whether each is enabled (secrets are not shown).",
		},
		Entry{
			Area:    "settings",
			Method:  "PUT",
			Path:    "/api/settings/sso/{provider}",
			Summary: "Create or update a single sign-on provider. provider is one of google, microsoft, github, facebook, custom.",
			Body: admObj(map[string]any{
				"client_id":         admStr("OAuth client ID from the provider."),
				"client_secret":     admStr("OAuth client secret. Leave empty on update to keep the stored one."),
				"is_enabled":        admBool("Show this sign-in option on the login page."),
				"allow_auto_create": admBool("Create an account automatically the first time someone signs in."),
				"default_role":      admStr("Role name given to auto-created users."),
				"allowed_domains":   admStr("Comma-separated email domains allowed to sign in, e.g. example.com,example.org. Empty = any."),
				"auth_url":          admStr("custom provider only: authorization URL."),
				"token_url":         admStr("custom provider only: token URL."),
				"user_info_url":     admStr("custom provider only: user-info URL."),
			}, "client_id"),
			SideEffects: "Changes who can sign in to the organisation and how.",
		},
		Entry{
			Area:        "settings",
			Method:      "DELETE",
			Path:        "/api/settings/sso/{provider}",
			Summary:     "Remove a single sign-on provider. provider is one of google, microsoft, github, facebook, custom.",
			SideEffects: "People can no longer sign in with that provider.",
		},
		Entry{
			Area:       "settings",
			Method:     "GET",
			Path:       "/api/settings/meta-onboarding",
			Summary:    "Show the app-level Meta configuration used by the WhatsApp onboarding wizard (super admins only).",
			Permission: "super_admin",
		},
		Entry{
			Area:       "settings",
			Method:     "PUT",
			Path:       "/api/settings/meta-onboarding",
			Summary:    "Change the app-level Meta configuration for the WhatsApp onboarding wizard (super admins only).",
			Permission: "super_admin",
			Body: admObj(map[string]any{
				"meta_app_id":               admStr("Meta app ID."),
				"meta_app_secret":           admStr("Meta app secret. Leave empty to keep the stored one."),
				"embedded_signup_config_id": admStr("Embedded Signup configuration ID."),
				"graph_api_version":         admStr("Graph API version, e.g. v21.0."),
				"public_webhook_base_url":   admStr("Public base URL Meta should send webhooks to."),
				"required_scopes":           admArr(admStr("Meta permission scope, e.g. whatsapp_business_management."), "Scopes the wizard asks for."),
			}),
			SideEffects: "Affects WhatsApp onboarding for every organisation on this server.",
		},
	)
}
