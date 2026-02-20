package models

import (
	"github.com/google/uuid"
)

// ConversationNote represents a private internal note on a contact, visible only to agents.
type ConversationNote struct {
	BaseModel
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	ContactID      uuid.UUID `gorm:"type:uuid;index;not null" json:"contact_id"`
	CreatedByID    uuid.UUID `gorm:"type:uuid;not null" json:"created_by_id"`
	Content        string    `gorm:"type:text;not null" json:"content"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Contact      *Contact      `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	CreatedBy    *User         `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
}

func (ConversationNote) TableName() string {
	return "conversation_notes"
}
