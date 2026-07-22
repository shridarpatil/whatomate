package models

import "github.com/google/uuid"

// Catalog represents a WhatsApp product catalog
type Catalog struct {
	BaseModel
	OrganizationID  uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount string    `gorm:"size:100;index" json:"whatsapp_account"` // Links to WhatsAppAccount.Name
	MetaCatalogID   string    `gorm:"size:100;uniqueIndex" json:"meta_catalog_id"`
	Name            string    `gorm:"size:255;not null" json:"name"`
	IsActive        bool      `gorm:"default:true" json:"is_active"`

	// Relations
	Organization *Organization    `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Products     []CatalogProduct `gorm:"foreignKey:CatalogID" json:"products,omitempty"`
}

func (Catalog) TableName() string {
	return "catalogs"
}

// CatalogProduct represents a product in a catalog
type CatalogProduct struct {
	BaseModel
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	CatalogID      uuid.UUID `gorm:"type:uuid;index;not null" json:"catalog_id"`
	MetaProductID  string    `gorm:"size:100;uniqueIndex" json:"meta_product_id"`
	Name           string    `gorm:"size:255;not null" json:"name"`
	Description    string    `gorm:"type:text" json:"description"`
	Price          int64     `gorm:"not null" json:"price"` // Price in cents
	Currency       string    `gorm:"size:3;default:'USD'" json:"currency"`
	URL            string    `gorm:"size:500" json:"url"`
	ImageURL       string    `gorm:"size:500" json:"image_url"`
	RetailerID     string    `gorm:"size:100" json:"retailer_id"` // SKU
	IsActive       bool      `gorm:"default:true" json:"is_active"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Catalog      *Catalog      `gorm:"foreignKey:CatalogID" json:"catalog,omitempty"`
}

func (CatalogProduct) TableName() string {
	return "catalog_products"
}
