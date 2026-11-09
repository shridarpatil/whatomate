package mcp

func init() {
	registerArea(Area{
		Key:         "audit_logs",
		Title:       "Audit log",
		Description: "Who changed what in the organisation, with before/after values.",
	})
	register(
		Entry{
			Area:       "audit_logs",
			Method:     "GET",
			Path:       "/api/audit-logs",
			Summary:    "List audit log entries, newest first, optionally filtered by resource, user, action or date.",
			Permission: "audit_logs:read",
			Query: append(admPaging(),
				Param{Name: "resource_type", Type: "string", Description: "Kind of record, e.g. user, role, contact, campaign, settings.general."},
				Param{Name: "resource_id", Type: "string", Description: "ID of one record."},
				Param{Name: "user_id", Type: "string", Description: "Only changes made by this user."},
				Param{Name: "action", Type: "string", Description: "Type of change, e.g. created, updated, deleted."},
				Param{Name: "from", Type: "string", Description: "From date, ISO format YYYY-MM-DD or full RFC3339 date-time."},
				Param{Name: "to", Type: "string", Description: "To date, ISO format YYYY-MM-DD (whole day included) or full RFC3339 date-time."},
			),
		},
		Entry{
			Area:       "audit_logs",
			Method:     "GET",
			Path:       "/api/audit-logs/{id}",
			Summary:    "Show one audit log entry with the full before/after change details.",
			Permission: "audit_logs:read",
		},
	)
}
