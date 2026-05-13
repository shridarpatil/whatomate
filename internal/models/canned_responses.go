package models

import (
	"github.com/google/uuid"
)

// CannedResponse represents a pre-defined response text for quick insertion in chat
type CannedResponse struct {
	BaseModel
	OrganizationID uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	Name           string     `gorm:"size:100;not null" json:"name"`
	Shortcut       string     `gorm:"size:50;index" json:"shortcut"`
	Content        string     `gorm:"type:text;not null" json:"content"`
	Category       string     `gorm:"size:50" json:"category"`
	IsActive       bool       `gorm:"default:true" json:"is_active"`
	UsageCount     int        `gorm:"default:0" json:"usage_count"`
	// Buttons stored in the same shape as chatbot flow steps:
	// [{id, title, type:'reply'|'url'|'phone', url?, phone_number?}]
	Buttons     JSONBArray `gorm:"type:jsonb;default:'[]'" json:"buttons"`
	CreatedByID uuid.UUID  `gorm:"type:uuid" json:"created_by_id"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	CreatedBy    *User         `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
}

func (CannedResponse) TableName() string {
	return "canned_responses"
}
