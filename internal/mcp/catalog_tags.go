package mcp

func init() {
	registerArea(Area{
		Key:         "tags",
		Title:       "Tags",
		Description: "Coloured labels used to group contacts (and to pick campaign recipients).",
	})

	tagBody := mxObj([]string{"name"}, map[string]any{
		"name":  mxStr("Tag name"),
		"color": mxStr("Colour name or hex code"),
	})

	register(
		Entry{
			Area:       "tags",
			Method:     "GET",
			Path:       "/api/tags",
			Summary:    "List tags.",
			Permission: "tags:read",
			Query:      mxWithPaging(mxQ("search", "string", "Text to match against tag name")),
		},
		Entry{
			Area:       "tags",
			Method:     "POST",
			Path:       "/api/tags",
			Summary:    "Create a tag.",
			Permission: "tags:write",
			Body:       tagBody,
		},
		Entry{
			Area:   "tags",
			Method: "PUT",
			Path:   "/api/tags/{name}",
			Summary: "Rename or recolour a tag ({name} is the current tag name, URL-encoded). " +
				"A rename is applied to every contact carrying the tag.",
			Permission: "tags:write",
			Body:       tagBody,
		},
		Entry{
			Area:        "tags",
			Method:      "DELETE",
			Path:        "/api/tags/{name}",
			Summary:     "Delete a tag ({name} URL-encoded) and remove it from all contacts.",
			Permission:  "tags:delete",
			SideEffects: "deletes data",
		},
	)
}
