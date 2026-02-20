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
	"github.com/zerodha/fastglue"
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
			Data struct {
				APIKeys []handlers.APIKeyResponse `json:"api_keys"`
				Total   int                       `json:"total"`
				Page    int                       `json:"page"`
				Limit   int                       `json:"limit"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Data.APIKeys, 2)
		assert.Equal(t, 2, resp.Data.Total)

		// Verify ordering is by created_at DESC (most recent first)
		assert.Equal(t, "Key Two", resp.Data.APIKeys[0].Name)
		assert.Equal(t, "Key One", resp.Data.APIKeys[1].Name)
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
			Data struct {
				APIKeys []handlers.APIKeyResponse `json:"api_keys"`
				Total   int                       `json:"total"`
				Page    int                       `json:"page"`
				Limit   int                       `json:"limit"`
			} `json:"data"`
		}
		err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Data.APIKeys)
		assert.Equal(t, 0, resp.Data.Total)
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
		assert.Equal(t, resp.Data.Key[4:20], resp.Data.KeyPrefix)
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

// --- ListAPIKeys Additional Tests ---

func TestApp_ListAPIKeys_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)

	perms := getAPIKeyPermissions(t, app)
	role1 := testutil.CreateTestRoleExact(t, app.DB, org1.ID, "API Key Mgr Org1 List", false, false, perms)
	role2 := testutil.CreateTestRoleExact(t, app.DB, org2.ID, "API Key Mgr Org2 List", false, false, perms)

	user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithEmail(testutil.UniqueEmail("list-cross-1")), testutil.WithRoleID(&role1.ID))
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithEmail(testutil.UniqueEmail("list-cross-2")), testutil.WithRoleID(&role2.ID))

	// Create keys in org1
	createTestAPIKey(t, app, org1.ID, user1.ID, "Org1 Key A")
	createTestAPIKey(t, app, org1.ID, user1.ID, "Org1 Key B")

	// Create key in org2
	createTestAPIKey(t, app, org2.ID, user2.ID, "Org2 Key C")

	// User from org2 should only see org2's key
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)

	err := app.ListAPIKeys(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			APIKeys []handlers.APIKeyResponse `json:"api_keys"`
			Total   int                       `json:"total"`
			Page    int                       `json:"page"`
			Limit   int                       `json:"limit"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.APIKeys, 1)
	assert.Equal(t, "Org2 Key C", resp.Data.APIKeys[0].Name)
}

func TestApp_ListAPIKeys_ResponseFields(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Reader Fields", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-fields")), testutil.WithRoleID(&role.ID))

	apiKey := createTestAPIKey(t, app, org.ID, user.ID, "Field Check Key")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListAPIKeys(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			APIKeys []handlers.APIKeyResponse `json:"api_keys"`
			Total   int                       `json:"total"`
			Page    int                       `json:"page"`
			Limit   int                       `json:"limit"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Data.APIKeys, 1)

	item := resp.Data.APIKeys[0]
	assert.Equal(t, apiKey.ID, item.ID)
	assert.Equal(t, "Field Check Key", item.Name)
	assert.Equal(t, "abcd1234", item.KeyPrefix)
	assert.True(t, item.IsActive)
	assert.NotEmpty(t, item.CreatedAt)
	// The full key hash should never be in the list response (it's json:"-" on the model)
	// We verify that Key field is NOT present in list response by checking the raw JSON
	body := testutil.GetResponseBody(req)
	assert.NotContains(t, string(body), "key_hash")
}

func TestApp_ListAPIKeys_ExcludesDeletedKeys(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key RD", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("list-deleted")), testutil.WithRoleID(&role.ID))

	keyToKeep := createTestAPIKey(t, app, org.ID, user.ID, "Keep Me")
	keyToDelete := createTestAPIKey(t, app, org.ID, user.ID, "Delete Me")

	// Delete one key
	require.NoError(t, app.DB.Delete(&models.APIKey{}, "id = ?", keyToDelete.ID).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListAPIKeys(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			APIKeys []handlers.APIKeyResponse `json:"api_keys"`
			Total   int                       `json:"total"`
			Page    int                       `json:"page"`
			Limit   int                       `json:"limit"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.APIKeys, 1)
	assert.Equal(t, keyToKeep.ID, resp.Data.APIKeys[0].ID)
}

// --- CreateAPIKey Additional Tests ---

func TestApp_CreateAPIKey_KeyFormat(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Creator KF", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-keyfmt")), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Format Test Key",
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

	key := resp.Data.Key
	// Key must start with "whm_"
	assert.True(t, len(key) > 4, "key must be longer than prefix")
	assert.Equal(t, "whm_", key[:4])

	// After prefix: 16 random bytes = 32 hex characters, total = 36
	assert.Len(t, key, 36, "whm_ (4) + 32 hex chars = 36 total")

	// The hex part should be valid hex
	hexPart := key[4:]
	for _, c := range hexPart {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"character '%c' should be valid lowercase hex", c)
	}

	// KeyPrefix should be the first 16 chars of the hex part
	assert.Equal(t, hexPart[:16], resp.Data.KeyPrefix)
}

func TestApp_CreateAPIKey_UniqueKeys(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Creator Uniq", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-unique")), testutil.WithRoleID(&role.ID))

	keys := make(map[string]bool)
	for i := 0; i < 5; i++ {
		req := testutil.NewJSONRequest(t, map[string]any{
			"name": "Unique Key",
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

		assert.False(t, keys[resp.Data.Key], "key should be unique, got duplicate: %s", resp.Data.Key)
		keys[resp.Data.Key] = true
	}
	assert.Len(t, keys, 5)
}

func TestApp_CreateAPIKey_DatabasePersistence(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Creator Persist", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-persist")), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Persisted Key",
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

	// Verify the key was persisted with the correct org and user
	var dbKey models.APIKey
	err = app.DB.Where("id = ?", resp.Data.ID).First(&dbKey).Error
	require.NoError(t, err)
	assert.Equal(t, org.ID, dbKey.OrganizationID)
	assert.Equal(t, user.ID, dbKey.UserID)
	assert.Equal(t, "Persisted Key", dbKey.Name)
	assert.True(t, dbKey.IsActive)
	assert.Equal(t, resp.Data.KeyPrefix, dbKey.KeyPrefix)
	// The hash should not be empty
	assert.NotEmpty(t, dbKey.KeyHash)
	// The hash should NOT be the plaintext key
	assert.NotEqual(t, resp.Data.Key, dbKey.KeyHash)
}

func TestApp_CreateAPIKey_InvalidJSONBody(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Creator BadJSON", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("create-badjson")), testutil.WithRoleID(&role.ID))

	// Create a raw request with invalid JSON
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody([]byte(`{invalid json`))
	req := &fastglue.Request{RequestCtx: ctx}
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateAPIKey(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
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

	t.Run("invalid id format", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Deleter BadID", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-apikey-badid")), testutil.WithRoleID(&role.ID))

		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, user.ID)
		testutil.SetPathParam(req, "id", "not-a-valid-uuid")

		err := app.DeleteAPIKey(req)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

		testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Invalid API key ID")
	})

	t.Run("already deleted", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		perms := getAPIKeyPermissions(t, app)
		role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "API Key Deleter Twice", false, false, perms)
		user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("delete-apikey-twice")), testutil.WithRoleID(&role.ID))

		apiKey := createTestAPIKey(t, app, org.ID, user.ID, "Key To Double Delete")

		// First delete should succeed
		req1 := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req1, org.ID, user.ID)
		testutil.SetPathParam(req1, "id", apiKey.ID.String())

		err := app.DeleteAPIKey(req1)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req1))

		// Second delete should return not found
		req2 := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req2, org.ID, user.ID)
		testutil.SetPathParam(req2, "id", apiKey.ID.String())

		err = app.DeleteAPIKey(req2)
		require.NoError(t, err)
		assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req2))
	})
}
