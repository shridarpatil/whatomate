package mcp

func init() {
	registerArea(Area{
		Key:   "admin",
		Title: "Super-admin repair tools",
		Description: "Super-admin-only data repair across organisations: chats stored under the wrong organisation or WhatsApp account, and contacts whose organisation does not match their phone number ID. " +
			"Safe order for chat repair: 1) GET /api/admin/chat-repair to look (read-only); 2) POST /api/admin/chat-repair/scan, which auto-applies every safe move and returns what is left; " +
			"3) POST /api/admin/chat-repair/apply only for remaining candidates you have reviewed, listing merge cases explicitly in manual_merge_contact_ids. " +
			"For contact organisation mismatches: GET /api/admin/org-mismatch first, then POST /api/admin/org-mismatch/apply.",
	})
	register(
		Entry{
			Area:       "admin",
			Method:     "GET",
			Path:       "/api/admin/chat-repair",
			Summary:    "Preview chats that look misrouted to the wrong organisation or WhatsApp account, using all message history. Read-only; changes nothing.",
			Permission: "super_admin",
			Query: []Param{
				{Name: "limit", Type: "integer", Description: "Maximum candidates to return (default 100, max 500)."},
			},
		},
		Entry{
			Area:        "admin",
			Method:      "POST",
			Path:        "/api/admin/chat-repair/scan",
			Summary:     "Scan for misrouted chats and immediately fix every safe move candidate (including same-location chats whose messages drift from the resolved organisation/account). Returns what was applied plus the remaining candidates, e.g. merge-required ones.",
			Permission:  "super_admin",
			SideEffects: "Moves contacts and their messages to the correct organisation/account for every safe candidate, in one transaction, without asking for each.",
		},
		Entry{
			Area:       "admin",
			Method:     "POST",
			Path:       "/api/admin/chat-repair/apply",
			Summary:    "Apply chosen chat repair candidates from the latest scan. Run the scan first; stale or no-longer-safe selections are rejected and need a re-scan.",
			Permission: "super_admin",
			Body: admObj(map[string]any{
				"contact_ids": admArr(admStr("Contact ID of a safe move candidate."),
					"Safe move candidates to apply (contact IDs from the preview/scan)."),
				"manual_merge_contact_ids": admArr(admStr("Contact ID of a merge-required candidate."),
					"Merge-required candidates you explicitly approve: the contact is merged into the existing contact at the correct location. A contact must not appear in both lists."),
			}),
			SideEffects: "Moves or merges contacts and their message history across organisations/accounts. Merges cannot be undone.",
		},
		Entry{
			Area:       "admin",
			Method:     "GET",
			Path:       "/api/admin/org-mismatch",
			Summary:    "List contacts whose organisation differs from the organisation that owns the WhatsApp phone number they wrote to. Read-only.",
			Permission: "super_admin",
		},
		Entry{
			Area:        "admin",
			Method:      "POST",
			Path:        "/api/admin/org-mismatch/apply",
			Summary:     "Move every mismatched contact (as found by GET /api/admin/org-mismatch at this moment) to the organisation that owns its phone number. No selection: it fixes all of them.",
			Permission:  "super_admin",
			SideEffects: "Re-assigns all mismatched contacts and their messages to another organisation in one go.",
		},
	)
}
