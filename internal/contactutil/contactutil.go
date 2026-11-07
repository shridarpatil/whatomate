package contactutil

import (
	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetOrCreateContact finds or creates a contact for the given phone number.
//   - Canonicalises the phone (see CanonicalPhone), so +91…, 91… and bare
//     10-digit forms resolve to one contact
//   - Finds legacy rows stored in any of those forms and upgrades the row to
//     the canonical form when nothing else holds it
//   - Updates the profile name if changed, unless an agent renamed the
//     contact (ProfileNameLocked)
//   - Handles race conditions on create by re-fetching
//   - Restores soft-deleted contacts if found
//
// Returns the contact, whether it was newly created, and any error.
func GetOrCreateContact(db *gorm.DB, orgID uuid.UUID, phoneNumber, profileName string) (*models.Contact, bool, error) {
	canonical := CanonicalPhone(phoneNumber)

	if contact, err := findAnyVariant(db.Unscoped(), orgID, phoneNumber); err == nil {
		if contact.DeletedAt.Valid {
			db.Unscoped().Model(contact).Update("deleted_at", nil)
			contact.DeletedAt.Valid = false
		}
		if profileName != "" && contact.ProfileName != profileName && !contact.ProfileNameLocked {
			db.Model(contact).Update("profile_name", profileName)
			contact.ProfileName = profileName
		}
		upgradeToCanonical(db, contact, canonical)
		return contact, false, nil
	}

	contact := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		PhoneNumber:    canonical,
		ProfileName:    profileName,
		IsActive:       true,
	}
	if err := db.Create(&contact).Error; err != nil {
		// Race condition: another goroutine may have created the contact
		if existing, err2 := findAnyVariant(db.Unscoped(), orgID, phoneNumber); err2 == nil {
			if existing.DeletedAt.Valid {
				db.Unscoped().Model(existing).Update("deleted_at", nil)
				existing.DeletedAt.Valid = false
			}
			return existing, false, nil
		}
		return nil, false, err
	}
	return &contact, true, nil
}

// FindContact finds a live contact for the given phone number in any stored form.
func FindContact(db *gorm.DB, orgID uuid.UUID, phoneNumber string) (*models.Contact, error) {
	return findAnyVariant(db, orgID, phoneNumber)
}

// FindContactUnscoped is FindContact including soft-deleted contacts.
func FindContactUnscoped(db *gorm.DB, orgID uuid.UUID, phoneNumber string) (*models.Contact, error) {
	return findAnyVariant(db.Unscoped(), orgID, phoneNumber)
}

// findAnyVariant prefers a live row, then the canonical form, then the most
// recently active row.
func findAnyVariant(db *gorm.DB, orgID uuid.UUID, phoneNumber string) (*models.Contact, error) {
	variants := PhoneVariants(phoneNumber)
	if len(variants) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var contact models.Contact
	err := db.Where("organization_id = ? AND phone_number IN ?", orgID, variants).
		Clauses(clause.OrderBy{Expression: clause.Expr{
			SQL:                "(deleted_at IS NULL) DESC, (phone_number = ?) DESC, last_message_at DESC NULLS LAST, created_at ASC",
			Vars:               []any{variants[0]},
			WithoutParentheses: true,
		}}).
		Take(&contact).Error // Take, not First: First's implicit ORDER BY id would drop this expression
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

// upgradeToCanonical rewrites a legacy phone form to the canonical one. It is
// best effort: if another row already holds the canonical form the unique
// index rejects the update and the duplicate merge handles it instead.
func upgradeToCanonical(db *gorm.DB, contact *models.Contact, canonical string) {
	if contact.PhoneNumber == canonical || !IsMergeablePhone(canonical) {
		return
	}
	res := db.Unscoped().Model(&models.Contact{}).
		Where("id = ? AND NOT EXISTS (SELECT 1 FROM contacts c2 WHERE c2.organization_id = ? AND c2.phone_number = ?)",
			contact.ID, contact.OrganizationID, canonical).
		Update("phone_number", canonical)
	if res.Error == nil && res.RowsAffected == 1 {
		contact.PhoneNumber = canonical
	}
}
