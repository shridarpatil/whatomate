package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// getAPIKeyPermissions returns api_keys permissions from the full permission set.
func getAPIKeyPermissions(t *testing.T, app *handlers.App) []models.Permission {
	t.Helper()

	allPerms := testutil.GetOrCreateTestPermissions(t, app.DB)

	var apiKeyPerms []models.Permission
	for _, p := range allPerms {
		if p.Resource == "api_keys" {
			apiKeyPerms = append(apiKeyPerms, p)
		}
	}
	require.NotEmpty(t, apiKeyPerms, "expected api_keys permissions in default set")
	return apiKeyPerms
}

// createTestAPIKey creates a test API key directly in the database.
func createTestAPIKey(t *testing.T, app *handlers.App, orgID, userID uuid.UUID, name string) *models.APIKey {
	t.Helper()

	apiKey := &models.APIKey{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		UserID:         userID,
		Name:           name,
		KeyPrefix:      "abcd1234",
		KeyHash:        "$2a$10$dummyhashvaluefortesting000000000000000000000000000000",
		IsActive:       true,
	}
	require.NoError(t, app.DB.Create(apiKey).Error)
	return apiKey
}

// --- ListAPIKeys Tests ---

func TestApp_ListAPIKeys(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Manager", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-apikeys")), testutil.WithRoleID(&role.ID))

		createTestAPIKey(t, app, org.ID, user.ID, "Key One")
		createTestAPIKey(t, app, org.ID, user.ID, "Key Two")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListAPIKeys(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data []handlers.APIKeyResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data, 2)

		// Verify ordering is by created_at DESC (most recent first)
		assert.Equal(t, "Key Two", resp.Data[0].Name)
		assert.Equal(t, "Key One", resp.Data[1].Name)
	})

	t.Run("empty list", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Reader", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-apikeys-empty")), testutil.WithRoleID(&role.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.ListAPIKeys(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data []handlers.APIKeyResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Data)
	})
}

// --- CreateAPIKey Tests ---

func TestApp_CreateAPIKey(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Creator", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-apikey")), testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "My API Key",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateAPIKey(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.APIKeyCreateResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "My API Key", resp.Data.Name)
		assert.NotEmpty(t, resp.Data.ID)
		assert.NotEmpty(t, resp.Data.Key)
		assert.True(t, len(resp.Data.Key) > 4, "key should have whm_ prefix plus random bytes")
		assert.Equal(t, "whm_", resp.Data.Key[:4])
		assert.NotEmpty(t, resp.Data.KeyPrefix)
		assert.Equal(t, resp.Data.Key[4:12], resp.Data.KeyPrefix)
		assert.NotEmpty(t, resp.Data.CreatedAt)

		// Verify the key was persisted in the database
		var dbKey models.APIKey
		err = app.DB.Where("id = ?", resp.Data.ID).First(&dbKey).Error
		require.NoError(t, err)
		assert.Equal(t, org.ID, dbKey.OrganizationID)
		assert.Equal(t, user.ID, dbKey.UserID)
		assert.Equal(t, "My API Key", dbKey.Name)
		assert.True(t, dbKey.IsActive)
	})

	t.Run("success with expiry", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Creator Exp", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-apikey-exp")), testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":       "Expiring Key",
			"expires_at": "2099-12-31T23:59:59Z",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateAPIKey(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		var resp struct {
			Data handlers.APIKeyCreateResponse `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)

		assert.Equal(t, "Expiring Key", resp.Data.Name)
		assert.NotNil(t, resp.Data.ExpiresAt)
	})

	t.Run("validation error missing name", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Creator MN", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-apikey-noname")), testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateAPIKey(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

		testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Name is required")
	})

	t.Run("validation error empty name", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Creator EN", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-apikey-emptyname")), testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateAPIKey(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

		testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Name is required")
	})

	t.Run("validation error invalid expiry format", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Creator IE", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-apikey-badexp")), testutil.WithRoleID(&role.ID))

		req := testutil.NewJSONRequest(t, map[string]any{
			"name":       "Bad Expiry Key",
			"expires_at": "not-a-date",
		})
		testutil.SetAuthContext(req, org.ID, user.ID)

		err := app.CreateAPIKey(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

		testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid expires_at format")
	})
}

// --- DeleteAPIKey Tests ---

func TestApp_DeleteAPIKey(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Deleter", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-apikey")), testutil.WithRoleID(&role.ID))

		apiKey := createTestAPIKey(t, app, org.ID, user.ID, "Key To Delete")

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", apiKey.ID.String())

		err := app.DeleteAPIKey(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

		// Verify the key is soft-deleted
		var count int64
		app.DB.Model(&models.APIKey{}).Where("id = ?", apiKey.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("not found", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Deleter NF", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-apikey-nf")), testutil.WithRoleID(&role.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", uuid.New().String())

		err := app.DeleteAPIKey(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	})

	t.Run("cross-org isolation", func(t *testing.T) {
		app := newTestApp(t)

		org1 := testutil.CreateTestOrganization(t, app.DB)
		org2 := testutil.CreateTestOrganization(t, app.DB)

		perms := getAPIKeyPermissions(t, app)
		role1 := testutil.CreateTestRoleExact(t, app.DB, org1.ID, "API Key Manager Org1", false, false, perms)
		role2 := testutil.CreateTestRoleExact(t, app.DB, org2.ID, "API Key Manager Org2", false, false, perms)

		user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("cross-del-1")), testutil.WithRoleID(&role1.ID))
		user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("cross-del-2")), testutil.WithRoleID(&role2.ID))

		// Create API key in org1
		apiKey := createTestAPIKey(t, app, org1.ID, user1.ID, "Org1 Key")

		// User from org2 tries to delete org1's key
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org2.ID, user2.ID)
		testutil.SetPathParam(req, "id", apiKey.ID.String())

		err := app.DeleteAPIKey(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

		// Key should still exist
		var count int64
		app.DB.Model(&models.APIKey{}).Where("id = ?", apiKey.ID).Count(&count)
		assert.Equal(t, int64(1), count)
	})
}
