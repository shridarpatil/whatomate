package contactutil

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrCreateContact_CreatesNew(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "1234567890", "Alice")
	require.NoError(t, err)
	assert.True(t, isNew)
	assert.Equal(t, "1234567890", contact.PhoneNumber)
	assert.Equal(t, "Alice", contact.ProfileName)
}

func TestGetOrCreateContact_FindsExisting(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	existing := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "1234567890",
		ProfileName:    "Alice",
	}
	require.NoError(t, db.Create(&existing).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "1234567890", "Alice")
	require.NoError(t, err)
	assert.False(t, isNew)
	assert.Equal(t, existing.ID, contact.ID)
}

func TestGetOrCreateContact_NormalizesPlus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	existing := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "1234567890",
		ProfileName:    "Bob",
	}
	require.NoError(t, db.Create(&existing).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "+1234567890", "Bob")
	require.NoError(t, err)
	assert.False(t, isNew)
	assert.Equal(t, existing.ID, contact.ID)
}

func TestGetOrCreateContact_FindsPlusPrefix(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	existing := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "+1234567890",
		ProfileName:    "Charlie",
	}
	require.NoError(t, db.Create(&existing).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "1234567890", "Charlie")
	require.NoError(t, err)
	assert.False(t, isNew)
	assert.Equal(t, existing.ID, contact.ID)
}

func TestGetOrCreateContact_UpdatesProfileName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	existing := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "1234567890",
		ProfileName:    "Old Name",
	}
	require.NoError(t, db.Create(&existing).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "1234567890", "New Name")
	require.NoError(t, err)
	assert.False(t, isNew)

	var reloaded models.Contact
	require.NoError(t, db.First(&reloaded, contact.ID).Error)
	assert.Equal(t, "New Name", reloaded.ProfileName)
}

// TestGetOrCreateContact_PreservesManuallySetName guards the case where a user
// renames a contact in the UI: GetOrCreateContact runs on every inbound message,
// so without the NameManuallySet check the next reply from that contact would
// silently restore the name reported by WhatsApp.
func TestGetOrCreateContact_PreservesManuallySetName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	existing := models.Contact{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		PhoneNumber:     "1234567890",
		ProfileName:     "Ferretería Ruiz",
		NameManuallySet: true,
	}
	require.NoError(t, db.Create(&existing).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "1234567890", "Juanito 🔥")
	require.NoError(t, err)
	assert.False(t, isNew)
	assert.Equal(t, "Ferretería Ruiz", contact.ProfileName)

	var reloaded models.Contact
	require.NoError(t, db.First(&reloaded, contact.ID).Error)
	assert.Equal(t, "Ferretería Ruiz", reloaded.ProfileName, "a manually set name must survive incoming messages")
}

// TestGetOrCreateContact_PreservesManuallySetNamePlusPrefix covers the same
// guard on the +prefix lookup branch, which duplicates the update logic.
func TestGetOrCreateContact_PreservesManuallySetNamePlusPrefix(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	existing := models.Contact{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		PhoneNumber:     "+1234567890",
		ProfileName:     "Ferretería Ruiz",
		NameManuallySet: true,
	}
	require.NoError(t, db.Create(&existing).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "1234567890", "Juanito 🔥")
	require.NoError(t, err)
	assert.False(t, isNew)

	var reloaded models.Contact
	require.NoError(t, db.First(&reloaded, contact.ID).Error)
	assert.Equal(t, "Ferretería Ruiz", reloaded.ProfileName)
}
