package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// --- parsePathUUID ---

func TestParsePathUUID_Valid(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	expected := uuid.New()
	testutil.SetPathParam(req, "id", expected.String())

	id, err := parsePathUUID(req, "id", "item")
	require.NoError(t, err)
	assert.Equal(t, expected, id)
}

func TestParsePathUUID_Invalid(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	id, err := parsePathUUID(req, "id", "item")
	assert.ErrorIs(t, err, errEnvelopeSent)
	assert.Equal(t, uuid.Nil, id)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestParsePathUUID_Missing(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)

	id, err := parsePathUUID(req, "id", "item")
	assert.ErrorIs(t, err, errEnvelopeSent)
	assert.Equal(t, uuid.Nil, id)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- parsePagination ---

func TestParsePagination_Defaults(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)

	p := parsePagination(req)
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 50, p.Limit)
	assert.Equal(t, 0, p.Offset)
}

func TestParsePagination_CustomValues(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "page", 3)
	testutil.SetQueryParam(req, "limit", 20)

	p := parsePagination(req)
	assert.Equal(t, 3, p.Page)
	assert.Equal(t, 20, p.Limit)
	assert.Equal(t, 40, p.Offset) // (3-1)*20
}

func TestParsePagination_MaxLimitCapping(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "limit", 500)

	p := parsePagination(req)
	assert.Equal(t, 50, p.Limit) // Exceeds max(100), falls back to default(50)
}

func TestParsePagination_ZeroPageDefaults(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "page", 0)
	testutil.SetQueryParam(req, "limit", 10)

	p := parsePagination(req)
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 10, p.Limit)
	assert.Equal(t, 0, p.Offset)
}

func TestParsePagination_NegativeValues(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "page", -1)
	testutil.SetQueryParam(req, "limit", -5)

	p := parsePagination(req)
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 50, p.Limit)
}

// --- parsePaginationWithDefaults ---

func TestParsePaginationWithDefaults_CustomDefaultAndMax(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)

	p := parsePaginationWithDefaults(req, 25, 200)
	assert.Equal(t, 25, p.Limit)
}

func TestParsePaginationWithDefaults_LimitExceedsMax(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "limit", 300)

	p := parsePaginationWithDefaults(req, 25, 200)
	assert.Equal(t, 25, p.Limit)
}

// --- parseDateParam ---

func TestParseDateParam_Valid(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "start_date", "2024-06-15")

	result, ok := parseDateParam(req, "start_date")
	assert.True(t, ok)
	assert.Equal(t, 2024, result.Year())
	assert.Equal(t, time.June, result.Month())
	assert.Equal(t, 15, result.Day())
}

func TestParseDateParam_Invalid(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "start_date", "not-a-date")

	_, ok := parseDateParam(req, "start_date")
	assert.False(t, ok)
}

func TestParseDateParam_Missing(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)

	_, ok := parseDateParam(req, "start_date")
	assert.False(t, ok)
}

func TestParseDateParam_WrongFormat(t *testing.T) {
	t.Parallel()
	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "start_date", "15/06/2024")

	_, ok := parseDateParam(req, "start_date")
	assert.False(t, ok)
}

// --- endOfDay ---

func TestEndOfDay(t *testing.T) {
	t.Parallel()
	day := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	end := endOfDay(day)

	assert.Equal(t, 2024, end.Year())
	assert.Equal(t, time.June, end.Month())
	assert.Equal(t, 15, end.Day())
	assert.Equal(t, 23, end.Hour())
	assert.Equal(t, 59, end.Minute())
	assert.Equal(t, 59, end.Second())
}

// --- findByIDAndOrg ---

func TestFindByIDAndOrg_Found(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)

	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "find-test-" + uuid.New().String()[:8],
		Slug:      "find-test-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)

	account := &models.WhatsAppAccount{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "test-acct-" + uuid.New().String()[:8],
		PhoneID:        "p-" + uuid.New().String()[:8],
		BusinessID:     "b-" + uuid.New().String()[:8],
		AccessToken:    "tok",
	}
	require.NoError(t, db.Create(account).Error)

	req := testutil.NewGETRequest(t)
	result, err := findByIDAndOrg[models.WhatsAppAccount](db, req, account.ID, org.ID, "Account")
	require.NoError(t, err)
	assert.Equal(t, account.ID, result.ID)
}

func TestFindByIDAndOrg_NotFound(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)

	req := testutil.NewGETRequest(t)
	_, err := findByIDAndOrg[models.WhatsAppAccount](db, req, uuid.New(), uuid.New(), "Account")
	assert.ErrorIs(t, err, errEnvelopeSent)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestFindByIDAndOrg_CrossOrgIsolation(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)

	org1 := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "org1-" + uuid.New().String()[:8],
		Slug:      "org1-" + uuid.New().String()[:8],
	}
	org2 := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "org2-" + uuid.New().String()[:8],
		Slug:      "org2-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org1).Error)
	require.NoError(t, db.Create(org2).Error)

	account := &models.WhatsAppAccount{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org1.ID,
		Name:           "acct-" + uuid.New().String()[:8],
		PhoneID:        "p-" + uuid.New().String()[:8],
		BusinessID:     "b-" + uuid.New().String()[:8],
		AccessToken:    "tok",
	}
	require.NoError(t, db.Create(account).Error)

	// Try to access org1's account with org2's ID
	req := testutil.NewGETRequest(t)
	_, err := findByIDAndOrg[models.WhatsAppAccount](db, req, account.ID, org2.ID, "Account")
	assert.ErrorIs(t, err, errEnvelopeSent)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- MaskPhoneNumber ---

func TestMaskPhoneNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		phone string
		want  string
	}{
		{"standard phone", "+1234567890", "*******7890"},
		{"short number", "1234", "1234"},
		{"very short", "12", "12"},
		{"empty", "", ""},
		{"exactly 5 chars", "12345", "*2345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, MaskPhoneNumber(tt.phone))
		})
	}
}

// --- LooksLikePhoneNumber ---

func TestLooksLikePhoneNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"standard phone", "+1234567890", true},
		{"digits only", "9876543210", true},
		{"with dashes", "123-456-7890", true},
		{"with spaces", "123 456 7890", true},
		{"too short", "12345", false},
		{"text", "hello world", false},
		{"email", "test@example.com", false},
		{"mixed mostly text", "abc1234567xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, LooksLikePhoneNumber(tt.s))
		})
	}
}

// --- MaskIfPhoneNumber ---

func TestMaskIfPhoneNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		want string
	}{
		{"phone number", "+1234567890", "*******7890"},
		{"not phone", "hello", "hello"},
		{"email", "test@example.com", "test@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, MaskIfPhoneNumber(tt.s))
		})
	}
}
