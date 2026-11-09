package mcp

// Area "accounts": WhatsApp business numbers, their phone setup and business
// profile. The onboarding wizard lives in catalog_accounts_onboarding.go.

func accountBodySchema(required ...string) map[string]any {
	return setupObj(map[string]any{
		"name":                              setupStr("Unique display name for this number inside Whatomate (used as the account key by templates, flows and campaigns)."),
		"app_id":                            setupStr("Meta App ID."),
		"phone_id":                          setupStr("Meta phone number ID."),
		"business_id":                       setupStr("WhatsApp Business Account (WABA) ID."),
		"access_token":                      setupStr("Meta system-user access token. On update, leave empty to keep the stored token."),
		"app_secret":                        setupStr("Meta App Secret, used to verify webhook signatures. On update, leave empty to keep it."),
		"webhook_verify_token":              setupStr("Webhook verify token. Generated automatically when empty on create."),
		"api_version":                       setupStr("Graph API version such as v21.0. Defaults to the server setting."),
		"is_default_incoming":               setupBool("Make this the default number for incoming messages (clears the flag on the other numbers)."),
		"is_default_outgoing":               setupBool("Make this the default number for outgoing messages (clears the flag on the other numbers)."),
		"auto_read_receipt":                 setupBool("Mark incoming messages as read automatically."),
		"marketing_messages_lite_onboarded": setupBool("Confirms the number meets Marketing Messages Lite requirements."),
		"marketing_messages_lite_enabled":   setupBool("Route marketing templates through Marketing Messages Lite. Needs marketing_messages_lite_onboarded=true."),
		"business_calling_enabled":          setupBool("Enable WhatsApp business calling for this number."),
	}, required...)
}

func init() {
	registerArea(Area{
		Key:   "accounts",
		Title: "WhatsApp accounts",
		Description: "WhatsApp business numbers: list, connect (onboarding wizard is the preferred way), edit, test, " +
			"phone verification/registration, two-step PIN and the business profile shown to customers.",
	})

	const pinReg = "Registers the phone number with Meta (Cloud API) so it can send and receive messages."
	register(
		Entry{Area: "accounts", Method: "GET", Path: "/api/accounts", Tool: "whatomate_list_accounts",
			Summary:    "List the organisation's WhatsApp numbers with name, phone, status, default incoming/outgoing flags and whether a token/app secret is stored (secrets are never returned).",
			Permission: "accounts:read"},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts",
			Summary: "Manually add a WhatsApp number from Meta credentials (legacy break-glass editor). " +
				"Prefer the onboarding wizard (POST /api/accounts/onboarding/sessions) for new numbers.",
			Permission:  "accounts:write",
			Body:        accountBodySchema("name", "phone_id", "business_id", "access_token"),
			SideEffects: "Stores Meta credentials for the organisation."},
		Entry{Area: "accounts", Method: "GET", Path: "/api/accounts/{id}",
			Summary: "Get one WhatsApp number's settings.", Permission: "accounts:read"},
		Entry{Area: "accounts", Method: "PUT", Path: "/api/accounts/{id}",
			Summary: "Update a WhatsApp number. Text fields left empty keep their stored value; the boolean flags " +
				"are always written, so send their current values when you do not mean to change them.",
			Permission:  "accounts:write",
			Body:        accountBodySchema(),
			SideEffects: "Changing the token or app secret affects live sending and webhook verification."},
		Entry{Area: "accounts", Method: "DELETE", Path: "/api/accounts/{id}",
			Summary: "Delete a WhatsApp number from Whatomate.", Permission: "accounts:delete",
			SideEffects: "Deletes the account; messages can no longer be sent from it."},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/{id}/test",
			Summary: "Test the connection to Meta for this number (checks the token and reads phone details).", Permission: "",
			SideEffects: "Calls Meta Graph API (read only)."},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/{id}/subscribe",
			Summary: "Subscribe the Meta app to this number's WhatsApp Business Account webhooks so incoming messages and delivery reports reach Whatomate.", Permission: "",
			SideEffects: "Calls Meta to subscribe webhooks for the WABA."},
		Entry{Area: "accounts", Method: "GET", Path: "/api/accounts/{id}/phone-status",
			Summary: "Read the phone's verification status, quality rating, messaging tier and a warning if it is a test number or not verified.", Permission: "",
			SideEffects: "Calls Meta Graph API (read only)."},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/{id}/request-code",
			Summary: "Phone setup step 1: ask Meta to send a verification code to the phone by SMS or voice call. Then call verify-code.", Permission: "",
			Body: setupObj(map[string]any{
				"code_method": setupEnum("How to deliver the code (default SMS).", "SMS", "VOICE"),
				"language":    setupStr("Language for the code message, e.g. en_US."),
			}),
			SideEffects: "Meta sends an SMS or voice call to the business phone number."},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/{id}/verify-code",
			Summary: "Phone setup step 2: submit the code the phone received. Then call register-phone.", Permission: "",
			Body:        setupObj(map[string]any{"code": setupStr("The code received by SMS/voice.")}, "code"),
			SideEffects: "Calls Meta to verify the phone number."},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/{id}/register-phone",
			Summary: "Phone setup step 3: register the verified number with a 6-digit two-step PIN. " +
				"Coexistence (WhatsApp Business app) numbers skip Meta registration and answer skipped=true.", Permission: "",
			Body: setupObj(map[string]any{
				"pin":                          setupStr("6-digit two-step verification PIN."),
				"backup_password":              setupStr("Optional backup password for migrating message history."),
				"backup_data":                  setupStr("Optional backup data blob for migration."),
				"data_localization_region":     setupStr("Optional data localisation region code (e.g. IN)."),
				"meta_store_retention_minutes": setupInt("Optional Meta message store retention in minutes."),
			}, "pin"),
			SideEffects: pinReg},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/{id}/two-step-verification",
			Summary: "Set or change the number's 6-digit two-step verification PIN at Meta.", Permission: "",
			Body:        setupObj(map[string]any{"pin": setupStr("New 6-digit PIN.")}, "pin"),
			SideEffects: "Changes the two-step PIN at Meta."},
		Entry{Area: "accounts", Method: "GET", Path: "/api/accounts/{id}/business_profile",
			Summary: "Read the WhatsApp business profile (about, description, address, email, websites, category, photo) customers see.", Permission: "",
			SideEffects: "Calls Meta Graph API (read only)."},
		Entry{Area: "accounts", Method: "PUT", Path: "/api/accounts/{id}/business_profile",
			Summary: "Update the WhatsApp business profile customers see. Send only the fields to change.", Permission: "",
			Body: setupObj(map[string]any{
				"messaging_product": setupEnum("Always whatsapp.", "whatsapp"),
				"about":             setupStr("Short status text (max 139 characters)."),
				"description":       setupStr("Business description (max 512 characters)."),
				"address":           setupStr("Business address."),
				"email":             setupStr("Contact email."),
				"vertical":          setupStr("Business category, e.g. RETAIL, EDU, HEALTH, PROF_SERVICES, OTHER."),
				"websites":          setupStrArr("Up to 2 website URLs."),
			}),
			SideEffects: "Changes the public profile at Meta."},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/{id}/business_profile/photo",
			Summary: "Upload a new WhatsApp profile picture (square JPG/PNG).", Permission: "",
			Multipart:   &MultipartSpec{FileField: "file", Accept: "image/jpeg,image/png"},
			SideEffects: "Uploads the image to Meta and changes the public profile photo."},
	)
}
