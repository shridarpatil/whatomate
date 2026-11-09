package mcp

func init() {
	registerArea(Area{
		Key:   "import_export",
		Title: "Import and export",
		Description: "CSV export of contacts, tags and leads, and CSV import of contacts and tags. " +
			"Importing contacts never sends a message.",
	})

	register(
		Entry{
			Area:   "import_export",
			Method: "GET",
			Path:   "/api/export/{table}/config",
			Summary: "Which columns can be exported for a table ({table} = contacts, tags or leads) " +
				"and which are chosen by default. Needs export permission on that table (contacts:export, tags:export or lead_ads:export).",
			Permission: "contacts:export",
		},
		Entry{
			Area:       "import_export",
			Method:     "POST",
			Path:       "/api/export",
			Summary:    "Export a table as a CSV file (returned as text, not JSON). Needs contacts:export, tags:export or lead_ads:export for the chosen table.",
			Permission: "contacts:export",
			Body: mxObj([]string{"table"}, map[string]any{
				"table":   mxEnum("What to export", "contacts", "tags", "leads"),
				"columns": mxStrArr("Columns to include; default columns when empty (see the config endpoint)"),
				"filters": mxStrMap("Optional column filters, column name to value"),
				"format":  mxEnum("Output format (default csv)", "csv"),
			}),
		},
		Entry{
			Area:   "import_export",
			Method: "GET",
			Path:   "/api/import/{table}/config",
			Summary: "Required and optional CSV columns for importing a table ({table} = contacts or tags). " +
				"contacts needs phone_number; tags needs name. Needs contacts:import or tags:import.",
			Permission: "contacts:import",
		},
		Entry{
			Area:   "import_export",
			Method: "POST",
			Path:   "/api/import",
			Summary: "Import contacts or tags from a CSV file (max 10 MB). Phone numbers are stored in canonical " +
				"form; existing rows are skipped unless update_on_duplicate is true. No messages are sent. Needs contacts:import or tags:import.",
			Permission: "contacts:import",
			Multipart: &MultipartSpec{
				FileField: "file",
				Accept:    "text/csv",
				ExtraFields: []Param{
					{Name: "table", Type: "string", Description: "Table to import into", Required: true, Enum: []string{"contacts", "tags"}},
					mxQ("update_on_duplicate", "boolean", "\"true\" updates rows that already exist"),
					mxQ("column_mapping", "string", "JSON object mapping CSV header to column name, e.g. {\"Mobile\":\"phone_number\"}"),
				},
			},
		},
	)
}
