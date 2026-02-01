package handlers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// --- getOrgAndUserID ---

func TestApp_getOrgAndUserID_Success(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)
	user := testutil.CreateTestUser(t, db, org.ID)

	app := &App{DB: db}

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	orgID, userID, err := app.getOrgAndUserID(req)
	require.NoError(t, err)
	assert.Equal(t, org.ID, orgID)
	assert.Equal(t, user.ID, userID)
}

func TestApp_getOrgAndUserID_MissingOrgID(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)
	user := testutil.CreateTestUser(t, db, org.ID)

	app := &App{DB: db}

	req := testutil.NewGETRequest(t)
	// Only set user_id, not org_id
	req.RequestCtx.SetUserValue("user_id", user.ID)

	_, _, err := app.getOrgAndUserID(req)
	assert.Error(t, err)
}

func TestApp_getOrgAndUserID_MissingUserID(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)

	app := &App{DB: db}

	req := testutil.NewGETRequest(t)
	// Only set org_id, not user_id
	req.RequestCtx.SetUserValue("organization_id", org.ID)

	_, _, err := app.getOrgAndUserID(req)
	assert.Error(t, err)
}

func TestApp_getOrgAndUserID_UserIDAsString(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)
	user := testutil.CreateTestUser(t, db, org.ID)

	app := &App{DB: db}

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("user_id", user.ID.String()) // String instead of UUID

	orgID, userID, err := app.getOrgAndUserID(req)
	require.NoError(t, err)
	assert.Equal(t, org.ID, orgID)
	assert.Equal(t, user.ID, userID)
}

func TestApp_getOrgAndUserID_InvalidUserIDString(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)

	app := &App{DB: db}

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("user_id", "not-a-uuid")

	_, _, err := app.getOrgAndUserID(req)
	assert.Error(t, err)
}

func TestApp_getOrgAndUserID_InvalidUserIDType(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)

	app := &App{DB: db}

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("user_id", 12345) // Integer instead of UUID

	_, _, err := app.getOrgAndUserID(req)
	assert.Error(t, err)
}

// --- requirePermission ---

func TestApp_requirePermission_Allowed(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)

	// Create a role with the permission
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "test-role", []string{"contacts:read"})
	user := testutil.CreateTestUser(t, db, org.ID, testutil.WithRoleID(&role.ID))

	app := &App{DB: db}

	req := testutil.NewGETRequest(t)
	err := app.requirePermission(req, user.ID, "contacts", "read")
	assert.NoError(t, err)
}

func TestApp_requirePermission_Denied(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)

	// Create a role without the required permission
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "test-role", []string{"contacts:read"})
	user := testutil.CreateTestUser(t, db, org.ID, testutil.WithRoleID(&role.ID))

	app := &App{DB: db}

	req := testutil.NewGETRequest(t)
	err := app.requirePermission(req, user.ID, "contacts", "delete") // No delete permission
	assert.ErrorIs(t, err, errEnvelopeSent)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_requirePermission_SuperAdmin(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)

	// Super admin should have all permissions
	user := testutil.CreateTestUser(t, db, org.ID, testutil.WithSuperAdmin())

	app := &App{DB: db}

	req := testutil.NewGETRequest(t)
	err := app.requirePermission(req, user.ID, "anything", "any_action")
	assert.NoError(t, err)
}

// --- decodeRequest ---

func TestApp_decodeRequest_Success(t *testing.T) {
	t.Parallel()

	app := &App{}

	type TestPayload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":  "test",
		"count": 42,
	})

	var payload TestPayload
	err := app.decodeRequest(req, &payload)
	require.NoError(t, err)
	assert.Equal(t, "test", payload.Name)
	assert.Equal(t, 42, payload.Count)
}

func TestApp_decodeRequest_InvalidJSON(t *testing.T) {
	t.Parallel()

	app := &App{}

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.SetBody([]byte("not valid json"))

	var payload map[string]interface{}
	err := app.decodeRequest(req, &payload)
	assert.ErrorIs(t, err, errEnvelopeSent)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_decodeRequest_EmptyBody(t *testing.T) {
	t.Parallel()

	app := &App{}

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.SetBody([]byte(""))

	var payload map[string]interface{}
	err := app.decodeRequest(req, &payload)
	assert.ErrorIs(t, err, errEnvelopeSent)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_decodeRequest_NestedStruct(t *testing.T) {
	t.Parallel()

	app := &App{}

	type Address struct {
		City    string `json:"city"`
		Country string `json:"country"`
	}
	type TestPayload struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "test",
		"address": map[string]string{
			"city":    "New York",
			"country": "USA",
		},
	})

	var payload TestPayload
	err := app.decodeRequest(req, &payload)
	require.NoError(t, err)
	assert.Equal(t, "test", payload.Name)
	assert.Equal(t, "New York", payload.Address.City)
	assert.Equal(t, "USA", payload.Address.Country)
}

// --- Integration: Handler using helpers ---

func TestApp_HandlerWithHelpers_Integration(t *testing.T) {
	t.Parallel()
	db := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "manager", []string{"contacts:read", "contacts:write"})
	user := testutil.CreateTestUser(t, db, org.ID, testutil.WithRoleID(&role.ID))

	app := &App{DB: db}

	// Simulate a handler that uses all three helpers
	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"phone_number": "+1234567890",
		"name":         "Test Contact",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	// 1. Get org and user ID
	orgID, userID, err := app.getOrgAndUserID(req)
	require.NoError(t, err)
	assert.Equal(t, org.ID, orgID)
	assert.Equal(t, user.ID, userID)

	// 2. Check permission
	err = app.requirePermission(req, userID, "contacts", "write")
	assert.NoError(t, err)

	// 3. Decode request body
	var payload struct {
		PhoneNumber string `json:"phone_number"`
		Name        string `json:"name"`
	}
	err = app.decodeRequest(req, &payload)
	require.NoError(t, err)
	assert.Equal(t, "+1234567890", payload.PhoneNumber)
	assert.Equal(t, "Test Contact", payload.Name)

	// Create contact in DB to verify the flow works end-to-end
	contact := &models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		PhoneNumber:    payload.PhoneNumber,
		ProfileName:    payload.Name,
	}
	require.NoError(t, db.Create(contact).Error)

	// Verify contact was created
	var found models.Contact
	require.NoError(t, db.Where("id = ?", contact.ID).First(&found).Error)
	assert.Equal(t, payload.Name, found.ProfileName)
}
