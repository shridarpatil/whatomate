package contactutil

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
)

// GetOrCreateContact finds or creates a contact for the given phone number.
// Merges behaviors from both handler and worker implementations:
//   - Normalizes phone (strips leading "+")
//   - Tries both normalized and +prefix forms
//   - Refreshes the profile name from WhatsApp, unless a user set it manually
//     (Contact.NameManuallySet)
//   - Handles race conditions on create by re-fetching
//   - Restores soft-deleted contacts if found
//
// Returns the contact, whether it was newly created, and any error.
func GetOrCreateContact(db *gorm.DB, orgID uuid.UUID, phoneNumber, profileName string) (*models.Contact, bool, error) {
	// Normalize phone number (remove + prefix if present)
	normalizedPhone := phoneNumber
	if len(normalizedPhone) > 0 && normalizedPhone[0] == '+' {
		normalizedPhone = normalizedPhone[1:]
	}

	// Try to find existing contact with normalized phone (including soft-deleted)
	var contact models.Contact
	if err := db.Unscoped().Where("organization_id = ? AND phone_number = ?", orgID, normalizedPhone).First(&contact).Error; err == nil {
		// Restore if soft-deleted
		if contact.DeletedAt.Valid {
			db.Unscoped().Model(&contact).Update("deleted_at", nil)
			contact.DeletedAt.Valid = false
		}
		refreshProfileName(db, &contact, profileName)
		return &contact, false, nil
	}

	// Also try with + prefix (contacts may have been stored with it)
	if err := db.Unscoped().Where("organization_id = ? AND phone_number = ?", orgID, "+"+normalizedPhone).First(&contact).Error; err == nil {
		// Restore if soft-deleted
		if contact.DeletedAt.Valid {
			db.Unscoped().Model(&contact).Update("deleted_at", nil)
			contact.DeletedAt.Valid = false
		}
		refreshProfileName(db, &contact, profileName)
		return &contact, false, nil
	}

	// Create new contact
	contact = models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		PhoneNumber:    normalizedPhone,
		ProfileName:    profileName,
	}
	if err := db.Create(&contact).Error; err != nil {
		// Race condition: another goroutine may have created the contact
		if err2 := db.Unscoped().Where("organization_id = ? AND phone_number = ?", orgID, normalizedPhone).First(&contact).Error; err2 == nil {
			// Restore if soft-deleted
			if contact.DeletedAt.Valid {
				db.Unscoped().Model(&contact).Update("deleted_at", nil)
				contact.DeletedAt.Valid = false
			}
			return &contact, false, nil
		}
		return nil, false, err
	}
	return &contact, true, nil
}

// refreshProfileName syncs the stored profile name with the one WhatsApp reports.
//
// It is a no-op when the name was set manually (via the UI or an import): the
// contact list is a place users curate, and GetOrCreateContact runs on every
// inbound message, so without this guard a single reply from the contact would
// silently revert any name a user had typed.
func refreshProfileName(db *gorm.DB, contact *models.Contact, profileName string) {
	if contact.NameManuallySet || profileName == "" || contact.ProfileName == profileName {
		return
	}
	if err := db.Model(contact).Update("profile_name", profileName).Error; err == nil {
		contact.ProfileName = profileName
	}
}

// FindContact finds a contact for the given phone number with both forms (normalized and +prefix).
func FindContact(db *gorm.DB, orgID uuid.UUID, phoneNumber string) (*models.Contact, error) {
	normalizedPhone := phoneNumber
	if len(normalizedPhone) > 0 && normalizedPhone[0] == '+' {
		normalizedPhone = normalizedPhone[1:]
	}

	var contact models.Contact
	if err := db.Where("organization_id = ? AND phone_number = ?", orgID, normalizedPhone).First(&contact).Error; err == nil {
		return &contact, nil
	}

	if err := db.Where("organization_id = ? AND phone_number = ?", orgID, "+"+normalizedPhone).First(&contact).Error; err == nil {
		return &contact, nil
	}

	return nil, gorm.ErrRecordNotFound
}
