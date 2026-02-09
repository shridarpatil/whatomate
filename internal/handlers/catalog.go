package handlers

import (
	"context"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// CatalogRequest represents the request body for creating a catalog
type CatalogRequest struct {
	WhatsAppAccount string `json:"whatsapp_account"`
	Name            string `json:"name"`
}

// CatalogResponse represents the API response for a catalog
type CatalogResponse struct {
	ID              uuid.UUID                `json:"id"`
	MetaCatalogID   string                   `json:"meta_catalog_id"`
	WhatsAppAccount string                   `json:"whatsapp_account"`
	Name            string                   `json:"name"`
	IsActive        bool                     `json:"is_active"`
	ProductCount    int                      `json:"product_count"`
	Products        []CatalogProductResponse `json:"products,omitempty"`
	CreatedAt       string                   `json:"created_at"`
	UpdatedAt       string                   `json:"updated_at"`
}

// CatalogProductRequest represents the request body for creating/updating a product
type CatalogProductRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int64  `json:"price"`    // Price in cents
	Currency    string `json:"currency"`
	URL         string `json:"url"`
	ImageURL    string `json:"image_url"`
	RetailerID  string `json:"retailer_id"` // SKU
}

// CatalogProductResponse represents the API response for a product
type CatalogProductResponse struct {
	ID            uuid.UUID `json:"id"`
	MetaProductID string    `json:"meta_product_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Price         int64     `json:"price"`
	Currency      string    `json:"currency"`
	URL           string    `json:"url"`
	ImageURL      string    `json:"image_url"`
	RetailerID    string    `json:"retailer_id"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
}

// SyncCatalogsRequest represents the request body for syncing catalogs
type SyncCatalogsRequest struct {
	WhatsAppAccount string `json:"whatsapp_account"`
}

// ListCatalogs returns all catalogs for the organization
func (a *App) ListCatalogs(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	whatsAppAccount := string(r.RequestCtx.QueryArgs().Peek("whatsapp_account"))

	query := a.DB.Where("organization_id = ?", orgID)
	if whatsAppAccount != "" {
		query = query.Where("whats_app_account = ?", whatsAppAccount)
	}

	var catalogs []models.Catalog
	if err := query.Order("name ASC").Find(&catalogs).Error; err != nil {
		a.Log.Error("Failed to list catalogs", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list catalogs", nil, "")
	}

	result := make([]CatalogResponse, len(catalogs))
	for i, c := range catalogs {
		// Get product count
		var productCount int64
		a.DB.Model(&models.CatalogProduct{}).Where("catalog_id = ?", c.ID).Count(&productCount)
		result[i] = catalogToResponse(c, int(productCount))
	}

	return r.SendEnvelope(map[string]interface{}{
		"catalogs": result,
	})
}

// CreateCatalog creates a new catalog in Meta and stores it locally
func (a *App) CreateCatalog(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req CatalogRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name == "" || req.WhatsAppAccount == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name and whatsapp_account are required", nil, "")
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Create catalog in Meta
	ctx := context.Background()
	waAccount := a.toWhatsAppAccount(&account)

	metaCatalogID, err := a.WhatsApp.CreateCatalog(ctx, waAccount, req.Name)
	if err != nil {
		a.Log.Error("Failed to create catalog in Meta", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create catalog", nil, "")
	}

	// Store catalog locally
	catalog := models.Catalog{
		OrganizationID:  orgID,
		WhatsAppAccount: req.WhatsAppAccount,
		MetaCatalogID:   metaCatalogID,
		Name:            req.Name,
		IsActive:        true,
	}

	if err := a.DB.Create(&catalog).Error; err != nil {
		a.Log.Error("Failed to save catalog", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save catalog", nil, "")
	}

	return r.SendEnvelope(catalogToResponse(catalog, 0))
}

// GetCatalog returns a single catalog with its products
func (a *App) GetCatalog(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "catalog")
	if err != nil {
		return nil
	}

	var catalog models.Catalog
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		Preload("Products").First(&catalog).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Catalog not found", nil, "")
	}

	resp := catalogToResponse(catalog, len(catalog.Products))
	resp.Products = make([]CatalogProductResponse, len(catalog.Products))
	for i, p := range catalog.Products {
		resp.Products[i] = productToResponse(p)
	}

	return r.SendEnvelope(resp)
}

// DeleteCatalog deletes a catalog from Meta and locally
func (a *App) DeleteCatalog(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "catalog")
	if err != nil {
		return nil
	}

	catalog, err := findByIDAndOrg[models.Catalog](a.DB, r, id, orgID, "Catalog")
	if err != nil {
		return nil
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, catalog.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Delete from Meta
	ctx := context.Background()
	waAccount := a.toWhatsAppAccount(&account)

	if err := a.WhatsApp.DeleteCatalog(ctx, waAccount, catalog.MetaCatalogID); err != nil {
		a.Log.Error("Failed to delete catalog from Meta", "error", err)
		// Continue with local deletion even if Meta fails
	}

	// Delete products first
	a.DB.Where("catalog_id = ?", id).Delete(&models.CatalogProduct{})

	// Delete catalog
	if err := a.DB.Delete(catalog).Error; err != nil {
		a.Log.Error("Failed to delete catalog", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete catalog", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Catalog deleted"})
}

// SyncCatalogs syncs catalogs from Meta API
func (a *App) SyncCatalogs(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req SyncCatalogsRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.WhatsAppAccount == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "whatsapp_account is required", nil, "")
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Fetch catalogs from Meta
	ctx := context.Background()
	waAccount := a.toWhatsAppAccount(&account)

	metaCatalogs, err := a.WhatsApp.ListCatalogs(ctx, waAccount)
	if err != nil {
		a.Log.Error("Failed to fetch catalogs from Meta", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch catalogs", nil, "")
	}

	// Sync each catalog
	synced := 0
	for _, mc := range metaCatalogs {
		var existing models.Catalog
		err := a.DB.Where("organization_id = ? AND meta_catalog_id = ?", orgID, mc.ID).First(&existing).Error
		if err != nil {
			// Create new catalog
			catalog := models.Catalog{
				OrganizationID:  orgID,
				WhatsAppAccount: req.WhatsAppAccount,
				MetaCatalogID:   mc.ID,
				Name:            mc.Name,
				IsActive:        true,
			}
			if err := a.DB.Create(&catalog).Error; err != nil {
				a.Log.Error("Failed to create synced catalog", "error", err, "meta_id", mc.ID)
				continue
			}
			synced++
		} else {
			// Update existing
			existing.Name = mc.Name
			a.DB.Save(&existing)
			synced++
		}
	}

	return r.SendEnvelope(map[string]interface{}{
		"message": "Catalogs synced",
		"synced":  synced,
		"total":   len(metaCatalogs),
	})
}

// ListCatalogProducts returns all products in a catalog
func (a *App) ListCatalogProducts(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	catalogID, err := parsePathUUID(r, "id", "catalog")
	if err != nil {
		return nil
	}

	// Verify catalog belongs to org
	catalog, err := findByIDAndOrg[models.Catalog](a.DB, r, catalogID, orgID, "Catalog")
	if err != nil {
		return nil
	}
	_ = catalog

	var products []models.CatalogProduct
	if err := a.DB.Where("catalog_id = ?", catalogID).Order("name ASC").Find(&products).Error; err != nil {
		a.Log.Error("Failed to list products", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list products", nil, "")
	}

	result := make([]CatalogProductResponse, len(products))
	for i, p := range products {
		result[i] = productToResponse(p)
	}

	return r.SendEnvelope(map[string]interface{}{
		"products": result,
	})
}

// CreateCatalogProduct creates a new product in a catalog
func (a *App) CreateCatalogProduct(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	catalogID, err := parsePathUUID(r, "id", "catalog")
	if err != nil {
		return nil
	}

	var req CatalogProductRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name == "" || req.Price <= 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name and price are required", nil, "")
	}

	// Get catalog and verify ownership
	catalog, err := findByIDAndOrg[models.Catalog](a.DB, r, catalogID, orgID, "Catalog")
	if err != nil {
		return nil
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, catalog.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Set defaults
	if req.Currency == "" {
		req.Currency = "USD"
	}

	// Create product in Meta
	ctx := context.Background()
	waAccount := a.toWhatsAppAccount(&account)

	productInput := &whatsapp.ProductInput{
		Name:        req.Name,
		Price:       req.Price,
		Currency:    req.Currency,
		URL:         req.URL,
		ImageURL:    req.ImageURL,
		RetailerID:  req.RetailerID,
		Description: req.Description,
	}

	metaProductID, err := a.WhatsApp.CreateProduct(ctx, waAccount, catalog.MetaCatalogID, productInput)
	if err != nil {
		a.Log.Error("Failed to create product in Meta", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create product", nil, "")
	}

	// Store product locally
	product := models.CatalogProduct{
		OrganizationID: orgID,
		CatalogID:      catalogID,
		MetaProductID:  metaProductID,
		Name:           req.Name,
		Description:    req.Description,
		Price:          req.Price,
		Currency:       req.Currency,
		URL:            req.URL,
		ImageURL:       req.ImageURL,
		RetailerID:     req.RetailerID,
		IsActive:       true,
	}

	if err := a.DB.Create(&product).Error; err != nil {
		a.Log.Error("Failed to save product", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save product", nil, "")
	}

	return r.SendEnvelope(productToResponse(product))
}

// GetCatalogProduct returns a single product
func (a *App) GetCatalogProduct(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "product")
	if err != nil {
		return nil
	}

	product, err := findByIDAndOrg[models.CatalogProduct](a.DB, r, id, orgID, "Product")
	if err != nil {
		return nil
	}

	return r.SendEnvelope(productToResponse(*product))
}

// UpdateCatalogProduct updates a product
func (a *App) UpdateCatalogProduct(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "product")
	if err != nil {
		return nil
	}

	product, err := findByIDAndOrg[models.CatalogProduct](a.DB, r, id, orgID, "Product")
	if err != nil {
		return nil
	}

	var req CatalogProductRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Get catalog to get WhatsApp account
	var catalog models.Catalog
	if err := a.DB.Where("id = ?", product.CatalogID).First(&catalog).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Catalog not found", nil, "")
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, catalog.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Update product in Meta
	ctx := context.Background()
	waAccount := a.toWhatsAppAccount(&account)

	productInput := &whatsapp.ProductInput{
		Name:        req.Name,
		Price:       req.Price,
		Currency:    req.Currency,
		URL:         req.URL,
		ImageURL:    req.ImageURL,
		Description: req.Description,
	}

	if err := a.WhatsApp.UpdateProduct(ctx, waAccount, product.MetaProductID, productInput); err != nil {
		a.Log.Error("Failed to update product in Meta", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update product", nil, "")
	}

	// Update locally
	if req.Name != "" {
		product.Name = req.Name
	}
	if req.Description != "" {
		product.Description = req.Description
	}
	if req.Price > 0 {
		product.Price = req.Price
	}
	if req.Currency != "" {
		product.Currency = req.Currency
	}
	if req.URL != "" {
		product.URL = req.URL
	}
	if req.ImageURL != "" {
		product.ImageURL = req.ImageURL
	}
	if req.RetailerID != "" {
		product.RetailerID = req.RetailerID
	}

	if err := a.DB.Save(product).Error; err != nil {
		a.Log.Error("Failed to save product", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save product", nil, "")
	}

	return r.SendEnvelope(productToResponse(*product))
}

// DeleteCatalogProduct deletes a product
func (a *App) DeleteCatalogProduct(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "product")
	if err != nil {
		return nil
	}

	product, err := findByIDAndOrg[models.CatalogProduct](a.DB, r, id, orgID, "Product")
	if err != nil {
		return nil
	}

	// Get catalog to get WhatsApp account
	var catalog models.Catalog
	if err := a.DB.Where("id = ?", product.CatalogID).First(&catalog).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Catalog not found", nil, "")
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, catalog.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Delete from Meta
	ctx := context.Background()
	waAccount := a.toWhatsAppAccount(&account)

	if err := a.WhatsApp.DeleteProduct(ctx, waAccount, product.MetaProductID); err != nil {
		a.Log.Error("Failed to delete product from Meta", "error", err)
		// Continue with local deletion
	}

	if err := a.DB.Delete(product).Error; err != nil {
		a.Log.Error("Failed to delete product", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete product", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Product deleted"})
}

// Helper functions

func catalogToResponse(c models.Catalog, productCount int) CatalogResponse {
	return CatalogResponse{
		ID:              c.ID,
		MetaCatalogID:   c.MetaCatalogID,
		WhatsAppAccount: c.WhatsAppAccount,
		Name:            c.Name,
		IsActive:        c.IsActive,
		ProductCount:    productCount,
		CreatedAt:       c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func productToResponse(p models.CatalogProduct) CatalogProductResponse {
	return CatalogProductResponse{
		ID:            p.ID,
		MetaProductID: p.MetaProductID,
		Name:          p.Name,
		Description:   p.Description,
		Price:         p.Price,
		Currency:      p.Currency,
		URL:           p.URL,
		ImageURL:      p.ImageURL,
		RetailerID:    p.RetailerID,
		IsActive:      p.IsActive,
		CreatedAt:     p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
