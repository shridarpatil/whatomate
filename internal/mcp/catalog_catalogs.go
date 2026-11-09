package mcp

// Area "catalogs": WhatsApp product catalogs and their products.

func productBody(required ...string) map[string]any {
	return setupObj(map[string]any{
		"name":        setupStr("Product name."),
		"description": setupStr("Product description."),
		"price":       setupInt("Price in minor units (cents/paise): 49900 means 499.00. Must be greater than 0."),
		"currency":    setupStr("ISO currency code, e.g. INR (default USD on create)."),
		"url":         setupStr("Product page URL."),
		"image_url":   setupStr("Public image URL."),
		"retailer_id": setupStr("Your SKU / retailer ID."),
	}, required...)
}

func init() {
	registerArea(Area{
		Key:   "catalogs",
		Title: "Product catalogs",
		Description: "WhatsApp (Meta Commerce) product catalogs per WhatsApp account, and their products. " +
			"Prices are in minor units (cents). Meta errors are returned as-is with HTTP 502.",
	})
	register(
		Entry{Area: "catalogs", Method: "GET", Path: "/api/catalogs", Summary: "List product catalogs with product counts.",
			Permission: "catalogs:read",
			Query:      []Param{{Name: "whatsapp_account", Type: "string", Description: "Only catalogs of this WhatsApp account name."}}},
		Entry{Area: "catalogs", Method: "POST", Path: "/api/catalogs",
			Summary: "Create a product catalog at Meta for a WhatsApp account.", Permission: "catalogs:write",
			Body: setupObj(map[string]any{
				"whatsapp_account": setupStr("WhatsApp account name."),
				"name":             setupStr("Catalog name."),
			}, "whatsapp_account", "name"),
			SideEffects: "Creates the catalog at Meta."},
		Entry{Area: "catalogs", Method: "GET", Path: "/api/catalogs/{id}", Summary: "Get one catalog.", Permission: "catalogs:read"},
		Entry{Area: "catalogs", Method: "DELETE", Path: "/api/catalogs/{id}",
			Summary: "Delete a catalog at Meta and in Whatomate.", Permission: "catalogs:delete",
			SideEffects: "Deletes the catalog and its products at Meta."},
		Entry{Area: "catalogs", Method: "POST", Path: "/api/catalogs/sync",
			Summary: "Import the catalogs that exist at Meta for a WhatsApp account.", Permission: "catalogs:sync",
			Body:        setupObj(map[string]any{"whatsapp_account": setupStr("WhatsApp account name.")}, "whatsapp_account"),
			SideEffects: "Reads catalogs from Meta and updates local copies."},
		Entry{Area: "catalogs", Method: "GET", Path: "/api/catalogs/{id}/products", Summary: "List the products of a catalog.", Permission: "catalogs:read"},
		Entry{Area: "catalogs", Method: "POST", Path: "/api/catalogs/{id}/products",
			Summary: "Add a product to a catalog (created at Meta).", Permission: "catalogs:write",
			Body: productBody("name", "price"), SideEffects: "Creates the product at Meta."},
		Entry{Area: "catalogs", Method: "GET", Path: "/api/products/{id}", Summary: "Get one product.", Permission: "catalogs:read"},
		Entry{Area: "catalogs", Method: "PUT", Path: "/api/products/{id}",
			Summary: "Update a product at Meta and in Whatomate. Send the full product: Meta receives the fields as sent, while Whatomate keeps its stored value for empty fields.", Permission: "catalogs:write",
			Body: productBody(), SideEffects: "Updates the product at Meta."},
		Entry{Area: "catalogs", Method: "DELETE", Path: "/api/products/{id}",
			Summary: "Delete a product (at Meta too).", Permission: "catalogs:delete",
			SideEffects: "Deletes the product at Meta."},
	)
}
