package mcp

// Area "accounts" (continued): the resumable onboarding wizard
// (/api/accounts/onboarding/sessions/*) plus the upstream embedded-signup
// compatibility endpoints.

const onboardingOrder = "Wizard order: 1) create session, 2) embedded-signup/complete OR manual-import, " +
	"3) request-code, 4) verify-code, 5) register-phone, 6) validate-webhook, 7) subscribe-webhooks, 8) finalize. " +
	"Every step returns the updated session (current_step, step_state, readiness); GET the session to resume."

func init() {
	sess := "/api/accounts/onboarding/sessions/{id}"
	register(
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/onboarding/sessions",
			Summary: "Start a new WhatsApp onboarding wizard session (the preferred way to connect a number). " + onboardingOrder,
			Body: setupObj(map[string]any{
				"mode": setupEnum("embedded_signup (Meta pop-up flow, default) or manual_import (paste credentials).",
					"embedded_signup", "manual_import"),
				"account_name": setupStr("Name to give the new WhatsApp account in Whatomate."),
			})},
		Entry{Area: "accounts", Method: "GET", Path: sess,
			Summary: "Get an onboarding session with its current step, per-step state, readiness checklist and last error. Use it to resume the wizard."},
		Entry{Area: "accounts", Method: "POST", Path: sess + "/embedded-signup/complete",
			Summary: "Wizard step 2 (embedded signup): hand over what Meta's Embedded Signup returned (a code to exchange, or an access token) " +
				"plus phone/WABA IDs; Whatomate imports the account. Needs the super-admin Meta app config when a code is sent.",
			Body: setupObj(map[string]any{
				"account_name":         setupStr("Account name in Whatomate."),
				"code":                 setupStr("Authorization code returned by Meta Embedded Signup (send this or access_token)."),
				"access_token":         setupStr("Business access token (send this or code)."),
				"app_id":               setupStr("Meta App ID (defaults to the platform app)."),
				"phone_id":             setupStr("Phone number ID returned by Embedded Signup."),
				"business_id":          setupStr("WABA ID returned by Embedded Signup."),
				"webhook_verify_token": setupStr("Optional webhook verify token."),
				"api_version":          setupStr("Optional Graph API version, e.g. v21.0."),
				"metadata":             setupFreeObj("Optional raw Embedded Signup event data to keep on the session."),
			}),
			SideEffects: "Calls Meta to exchange the code; creates or updates the WhatsApp account."},
		Entry{Area: "accounts", Method: "POST", Path: sess + "/manual-import",
			Summary: "Wizard step 2 (manual): import a number from credentials copied from Meta Business Manager; Whatomate validates them with Meta and creates the account.",
			Body: setupObj(map[string]any{
				"account_name":         setupStr("Account name in Whatomate."),
				"app_id":               setupStr("Meta App ID."),
				"app_secret":           setupStr("Meta App Secret (needed for webhook signature checks)."),
				"phone_id":             setupStr("Phone number ID."),
				"business_id":          setupStr("WABA ID."),
				"access_token":         setupStr("System-user access token."),
				"webhook_verify_token": setupStr("Optional webhook verify token."),
				"api_version":          setupStr("Optional Graph API version, e.g. v21.0."),
			}, "phone_id", "business_id", "access_token"),
			SideEffects: "Calls Meta to validate; creates or updates the WhatsApp account."},
		Entry{Area: "accounts", Method: "POST", Path: sess + "/request-code",
			Summary: "Wizard step 3: ask Meta to send a verification code to the phone (skip when the number is already verified or is a Coexistence number).",
			Body: setupObj(map[string]any{
				"code_method": setupEnum("Delivery method (default SMS).", "SMS", "VOICE"),
				"language":    setupStr("Language for the code message, e.g. en_US."),
			}),
			SideEffects: "Meta sends an SMS or voice call to the phone."},
		Entry{Area: "accounts", Method: "POST", Path: sess + "/verify-code",
			Summary:     "Wizard step 4: submit the verification code the phone received.",
			Body:        setupObj(map[string]any{"code": setupStr("The received code.")}, "code"),
			SideEffects: "Calls Meta to verify the phone."},
		Entry{Area: "accounts", Method: "POST", Path: sess + "/register-phone",
			Summary: "Wizard step 5: register the number with Meta using a 6-digit two-step PIN. " +
				"Coexistence (WhatsApp Business app) numbers skip Meta registration automatically.",
			Body: setupObj(map[string]any{
				"pin":                          setupStr("6-digit two-step verification PIN."),
				"backup_password":              setupStr("Optional backup password for history migration."),
				"backup_data":                  setupStr("Optional backup data blob."),
				"data_localization_region":     setupStr("Optional data localisation region code."),
				"meta_store_retention_minutes": setupInt("Optional Meta message store retention in minutes."),
			}, "pin"),
			SideEffects: "Registers the phone number with Meta."},
		Entry{Area: "accounts", Method: "POST", Path: sess + "/validate-webhook",
			Summary:     "Wizard step 6: check that the public HTTPS webhook callback URL answers Meta's verification challenge with this account's verify token.",
			SideEffects: "Makes an HTTP request to the public callback URL."},
		Entry{Area: "accounts", Method: "POST", Path: sess + "/subscribe-webhooks",
			Summary:     "Wizard step 7: subscribe the Meta app to the WABA's webhooks so messages and delivery reports arrive.",
			SideEffects: "Calls Meta to subscribe webhooks."},
		Entry{Area: "accounts", Method: "POST", Path: sess + "/finalize",
			Summary:     "Wizard step 8: run the final connection test and compute readiness (ready / action_required / waiting_on_meta with Meta-owned checkpoints).",
			SideEffects: "Calls Meta Graph API (read only)."},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/exchange-token",
			Summary: "Upstream-compatible Embedded Signup: exchange a Meta signup code for a token and create the account in one call. " +
				"Kept for API compatibility; prefer the onboarding wizard.",
			Permission: "accounts:write",
			Body: setupObj(map[string]any{
				"code":                 setupStr("Authorization code from Meta Embedded Signup."),
				"phone_id":             setupStr("Optional phone number ID (discovered from the token when empty)."),
				"waba_id":              setupStr("Optional WABA ID (discovered from the token when empty)."),
				"name":                 setupStr("Optional account name."),
				"webhook_verify_token": setupStr("Optional webhook verify token."),
			}, "code"),
			SideEffects: "Calls Meta, creates the WhatsApp account and subscribes it to webhooks."},
		Entry{Area: "accounts", Method: "POST", Path: "/api/accounts/{id}/register",
			Summary: "Upstream-compatible phone registration for an existing account (random PIN when none given). " +
				"Skips Meta registration for Coexistence numbers. Prefer /api/accounts/{id}/register-phone.",
			Permission:  "accounts:write",
			Body:        setupObj(map[string]any{"pin": setupStr("Optional 6-digit PIN; generated when empty.")}),
			SideEffects: "Registers the phone number with Meta."},
	)
}
