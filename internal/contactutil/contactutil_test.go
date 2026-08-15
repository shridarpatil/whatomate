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

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"5511999999999":       "5511999999999",
		"+5511999999999":      "5511999999999",
		"55 11 99999-9999":    "5511999999999",
		"+55 (11) 99999-9999": "5511999999999",
		"":                    "",
		"+":                   "",
		"abc":                 "",
	}
	for in, want := range cases {
		assert.Equal(t, want, NormalizePhone(in), "NormalizePhone(%q)", in)
	}
}

// A differently-FORMATTED same number must resolve to the existing contact,
// not create a duplicate — the identity is the digits, not the exact string.
func TestGetOrCreateContact_MatchesAcrossFormatting(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	existing := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "5511999999999",
		ProfileName:    "Ana",
	}
	require.NoError(t, db.Create(&existing).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "55 11 99999-9999", "Ana")
	require.NoError(t, err)
	assert.False(t, isNew, "formatted form of an existing number must not create a new contact")
	assert.Equal(t, existing.ID, contact.ID)
}

// A newly-created contact is stored in the digits-only canonical form,
// regardless of the format it arrived in.
func TestGetOrCreateContact_StoresDigitsOnly(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "+55 (11) 98888-7777", "Novo")
	require.NoError(t, err)
	assert.True(t, isNew)
	assert.Equal(t, "5511988887777", contact.PhoneNumber)
}

func TestFindContactUnscoped_MatchesAcrossFormatting(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)
	existing := models.Contact{BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, PhoneNumber: "5511977776666"}
	require.NoError(t, db.Create(&existing).Error)

	got, err := FindContactUnscoped(db, org.ID, "+55 11 97777-6666")
	require.NoError(t, err)
	assert.Equal(t, existing.ID, got.ID)
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

// A Brazilian mobile has two valid digit strings for the same subscriber: the
// current 9-digit form and the legacy 8-digit form the carrier issued before
// the 9th digit was added. WhatsApp still reports the legacy form for older
// accounts, so an agent who registers "+55 71 99123-4567" must be matched when
// the reply arrives as "5571 9123-4567" — otherwise the reply lands as a brand
// new contact with no owner and falls into the general queue.
func TestPhoneIdentities_BrazilianNinthDigit(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		want  []string
	}{
		{"mobile with 9th digit yields the legacy form", "5571991234567", []string{"5571991234567", "557191234567"}},
		{"legacy mobile yields the 9th-digit form", "557191234567", []string{"557191234567", "5571991234567"}},
		{"landline is never given a 9th digit", "557131234567", []string{"557131234567"}},
		{"formatting is stripped before the rule is applied", "+55 (11) 99999-9999", []string{"5511999999999", "551199999999"}},
		{"a US number is never expanded", "12125551234", []string{"12125551234"}},
		{"a 55-prefixed number of the wrong length is left alone", "5571991234", []string{"5571991234"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, PhoneIdentities(tt.phone))
		})
	}
}

// The end-to-end symptom: contact registered with the 9, reply arrives without
// it. Must resolve to the SAME row, keeping its owner.
func TestGetOrCreateContact_MatchesBrazilianLegacyForm(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	// A real user row: assigned_user_id carries a foreign key.
	owner := models.User{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Email:          "agent-" + uid + "@test.com",
		FullName:       "Agente Dono",
		IsActive:       true,
	}
	require.NoError(t, db.Create(&owner).Error)

	existing := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "5571991234567",
		AssignedUserID: &owner.ID,
	}
	require.NoError(t, db.Create(&existing).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "557191234567", "Cliente")
	require.NoError(t, err)
	assert.False(t, isNew, "the legacy 8-digit form must not create a second contact")
	assert.Equal(t, existing.ID, contact.ID)
	require.NotNil(t, contact.AssignedUserID, "the reply must keep the owning agent, not fall to the general queue")
	assert.Equal(t, owner.ID, *contact.AssignedUserID)
}

// And the inverse: registered without the 9, reply arrives with it.
func TestGetOrCreateContact_MatchesBrazilianNinthDigitForm(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)

	existing := models.Contact{BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, PhoneNumber: "557188887777"}
	require.NoError(t, db.Create(&existing).Error)

	contact, isNew, err := GetOrCreateContact(db, org.ID, "+55 71 98888-7777", "Cliente")
	require.NoError(t, err)
	assert.False(t, isNew)
	assert.Equal(t, existing.ID, contact.ID)
}

func TestFindContactUnscoped_MatchesBrazilianLegacyForm(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "test-" + uid, Slug: "test-" + uid}
	require.NoError(t, db.Create(&org).Error)
	existing := models.Contact{BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, PhoneNumber: "5571993334444"}
	require.NoError(t, db.Create(&existing).Error)

	got, err := FindContactUnscoped(db, org.ID, "557193334444")
	require.NoError(t, err)
	assert.Equal(t, existing.ID, got.ID)
}
