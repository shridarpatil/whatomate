package models

import (
	"time"

	"github.com/google/uuid"
)

// Tag represents an organization-scoped tag for contacts
// Uses composite primary key (organization_id, name) for simplicity
type Tag struct {
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;primaryKey" json:"organization_id"`
	Name           string    `gorm:"size:50;not null;primaryKey" json:"name"`
	Color          string    `gorm:"size:20" json:"color"` // predefined: blue, red, green, yellow, purple, gray
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (Tag) TableName() string {
	return "tags"
}

// ValidTagColors defines the allowed tag colors
var ValidTagColors = []string{"blue", "red", "green", "yellow", "purple", "gray"}

// IsValidTagColor checks if a color is valid
func IsValidTagColor(color string) bool {
	if color == "" {
		return true // empty color is allowed (defaults to gray)
	}
	for _, c := range ValidTagColors {
		if c == color {
			return true
		}
	}
	return false
}
