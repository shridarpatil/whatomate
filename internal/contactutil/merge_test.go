package contactutil

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func mergeTestOrg(t *testing.T, db *gorm.DB) models.Organization {
	t.Helper()
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "merge-" + uid, Slug: "merge-" + uid}
	require.NoError(t, db.Create(&org).Error)
	return org
}

func mergeTestContact(t *testing.T, db *gorm.DB, orgID uuid.UUID, phone, name string, lastMsg *time.Time, tags ...any) models.Contact {
	t.Helper()
	c := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		PhoneNumber:    phone,
		ProfileName:    name,
		LastMessageAt:  lastMsg,
		Tags:           models.JSONBArray(tags),
		IsActive:       true,
		IsRead:         true,
	}
	require.NoError(t, db.Create(&c).Error)
	return c
}

func mergeTestMessage(t *testing.T, db *gorm.DB, c models.Contact, content string) models.Message {
	t.Helper()
	m := models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  c.OrganizationID,
		ContactID:       c.ID,
		WhatsAppAccount: "acct",
		Direction:       models.DirectionIncoming,
		MessageType:     models.MessageTypeText,
		Content:         content,
	}
	require.NoError(t, db.Create(&m).Error)
	return m
}

func TestMergeDuplicateContacts_FoldsPhoneForms(t *testing.T) {
	db := testutil.SetupIsolatedTestDB(t)
	org := mergeTestOrg(t, db)
	older := time.Now().Add(-2 * time.Hour).UTC()
	newer := time.Now().Add(-time.Minute).UTC()

	canonical := mergeTestContact(t, db, org.ID, "919876543210", "Ravi WA", &newer, "vip")
	bare := mergeTestContact(t, db, org.ID, "9876543210", "Ravi Kumar", &older, "member")
	plus := mergeTestContact(t, db, org.ID, "+919876543210", "", nil, "vip", "donor")
	other := mergeTestContact(t, db, org.ID, "919000000001", "Someone", nil)

	m1 := mergeTestMessage(t, db, bare, "from bare")
	m2 := mergeTestMessage(t, db, plus, "from plus")
	author := testutil.CreateTestUser(t, db, org.ID)
	note := models.ConversationNote{BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, ContactID: bare.ID, CreatedByID: author.ID, Content: "note"}
	require.NoError(t, db.Create(&note).Error)

	report, err := MergeDuplicateContacts(db)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Groups)
	assert.Equal(t, 2, report.Merged)

	var live []models.Contact
	require.NoError(t, db.Unscoped().Where("organization_id = ?", org.ID).Order("phone_number").Find(&live).Error)
	require.Len(t, live, 2, "duplicates hard-deleted")
	survivor := live[1]
	assert.Equal(t, canonical.ID, survivor.ID)
	assert.Equal(t, "919876543210", survivor.PhoneNumber)
	assert.Equal(t, "Ravi WA", survivor.ProfileName)
	assert.ElementsMatch(t, models.JSONBArray{"vip", "member", "donor"}, survivor.Tags)
	assert.Equal(t, other.ID, live[0].ID)

	for _, id := range []uuid.UUID{m1.ID, m2.ID} {
		var m models.Message
		require.NoError(t, db.First(&m, "id = ?", id).Error)
		assert.Equal(t, canonical.ID, m.ContactID)
	}
	var n models.ConversationNote
	require.NoError(t, db.First(&n, "id = ?", note.ID).Error)
	assert.Equal(t, canonical.ID, n.ContactID)

	again, err := MergeDuplicateContacts(db)
	require.NoError(t, err)
	assert.Equal(t, MergeReport{}, again, "second run is a no-op")
}

func TestMergeDuplicateContacts_NormalisesSingleAndKeepsLockedName(t *testing.T) {
	db := testutil.SetupIsolatedTestDB(t)
	org := mergeTestOrg(t, db)

	single := mergeTestContact(t, db, org.ID, "+91 99999 11111", "Solo", nil)
	a := mergeTestContact(t, db, org.ID, "919888877777", "WA Name", nil)
	b := mergeTestContact(t, db, org.ID, "9888877777", "Agent Name", nil)
	require.NoError(t, db.Model(&b).Update("profile_name_locked", true).Error)

	report, err := MergeDuplicateContacts(db)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Renamed)
	assert.Equal(t, 1, report.Merged)

	var s models.Contact
	require.NoError(t, db.First(&s, "id = ?", single.ID).Error)
	assert.Equal(t, "919999911111", s.PhoneNumber)

	var survivor models.Contact
	require.NoError(t, db.First(&survivor, "id = ?", a.ID).Error)
	assert.Equal(t, "Agent Name", survivor.ProfileName)
	assert.True(t, survivor.ProfileNameLocked)
}

func TestGetOrCreateContact_MatchesLegacyBareNumber(t *testing.T) {
	db := testutil.SetupTestDB(t)
	org := mergeTestOrg(t, db)
	legacy := mergeTestContact(t, db, org.ID, "9811122233", "Legacy", nil)

	c, isNew, err := GetOrCreateContact(db, org.ID, "919811122233", "WA Profile")
	require.NoError(t, err)
	assert.False(t, isNew)
	assert.Equal(t, legacy.ID, c.ID)
	assert.Equal(t, "919811122233", c.PhoneNumber, "legacy row upgraded to canonical")
}

func TestGetOrCreateContact_LockedNameNotOverwritten(t *testing.T) {
	db := testutil.SetupTestDB(t)
	org := mergeTestOrg(t, db)
	c := mergeTestContact(t, db, org.ID, "919811100000", "Agent Set", nil)
	require.NoError(t, db.Model(&c).Update("profile_name_locked", true).Error)

	got, _, err := GetOrCreateContact(db, org.ID, "919811100000", "WhatsApp Name")
	require.NoError(t, err)
	assert.Equal(t, "Agent Set", got.ProfileName)
}
