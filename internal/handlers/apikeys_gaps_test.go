package handlers_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// --- GetAPIKey (single) ---

func TestApp_GetAPIKey_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Get APIKey", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	key := createTestAPIKey(t, app, org.ID, user.ID, "fetched key")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", key.ID.String())

	require.NoError(t, app.GetAPIKey(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.APIKeyResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, key.ID, resp.Data.ID)
	assert.Equal(t, "fetched key", resp.Data.Name)
}

func TestApp_GetAPIKey_NotFound(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Get APIKey 404", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	require.NoError(t, app.GetAPIKey(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "API key not found")
}

func TestApp_GetAPIKey_CrossOrgIsolation(t *testing.T) {
	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role1 := testutil.CreateTestRoleExact(t, app.DB, org1.ID, "Get APIKey ISO 1", false, false, perms)
	role2 := testutil.CreateTestRoleExact(t, app.DB, org2.ID, "Get APIKey ISO 2", false, false, perms)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&role1.ID))
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithRoleID(&role2.ID))

	key := createTestAPIKey(t, app, org1.ID, user1.ID, "org1 key")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", key.ID.String())

	require.NoError(t, app.GetAPIKey(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req),
		"cross-org fetch must look like not-found, never expose the row")
}

func TestApp_GetAPIKey_InvalidID(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Get APIKey BadID", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	require.NoError(t, app.GetAPIKey(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetAPIKey_PermissionDenied(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// Role has no api_keys permissions.
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "No APIKey Perms", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	owner := testutil.CreateTestUser(t, app.DB, org.ID)
	key := createTestAPIKey(t, app, org.ID, owner.ID, "protected")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", key.ID.String())

	require.NoError(t, app.GetAPIKey(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// --- UpdateAPIKey ---

func TestApp_UpdateAPIKey_TogglesIsActive(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Update APIKey", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	key := createTestAPIKey(t, app, org.ID, user.ID, "toggleable")
	require.True(t, key.IsActive)

	// Disable.
	req := testutil.NewJSONRequest(t, map[string]any{"is_active": false})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", key.ID.String())

	require.NoError(t, app.UpdateAPIKey(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var got models.APIKey
	require.NoError(t, app.DB.Where("id = ?", key.ID).First(&got).Error)
	assert.False(t, got.IsActive, "is_active should be flipped to false")

	// Re-enable.
	req2 := testutil.NewJSONRequest(t, map[string]any{"is_active": true})
	testutil.SetAuthContext(req2, org.ID, user.ID)
	testutil.SetPathParam(req2, "id", key.ID.String())
	require.NoError(t, app.UpdateAPIKey(req2))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))

	require.NoError(t, app.DB.Where("id = ?", key.ID).First(&got).Error)
	assert.True(t, got.IsActive)
}

func TestApp_UpdateAPIKey_NilIsActiveLeavesUnchanged(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Update APIKey Nil", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	key := createTestAPIKey(t, app, org.ID, user.ID, "stays active")

	// Empty body — is_active not provided. Pointer is nil → no change.
	req := testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", key.ID.String())

	require.NoError(t, app.UpdateAPIKey(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var got models.APIKey
	require.NoError(t, app.DB.Where("id = ?", key.ID).First(&got).Error)
	assert.True(t, got.IsActive, "missing is_active in body must not flip the flag")
}

func TestApp_UpdateAPIKey_NotFound(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Update APIKey 404", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{"is_active": false})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	require.NoError(t, app.UpdateAPIKey(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusNotFound, "API key not found")
}

func TestApp_UpdateAPIKey_CrossOrgIsolation(t *testing.T) {
	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role1 := testutil.CreateTestRoleExact(t, app.DB, org1.ID, "Update APIKey ISO 1", false, false, perms)
	role2 := testutil.CreateTestRoleExact(t, app.DB, org2.ID, "Update APIKey ISO 2", false, false, perms)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID, testutil.WithRoleID(&role1.ID))
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID, testutil.WithRoleID(&role2.ID))

	key := createTestAPIKey(t, app, org1.ID, user1.ID, "org1 key")

	req := testutil.NewJSONRequest(t, map[string]any{"is_active": false})
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", key.ID.String())

	require.NoError(t, app.UpdateAPIKey(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	// Original is still active in org1.
	var got models.APIKey
	require.NoError(t, app.DB.Where("id = ?", key.ID).First(&got).Error)
	assert.True(t, got.IsActive)
}

func TestApp_UpdateAPIKey_PermissionDenied(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "No Update Perms", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	owner := testutil.CreateTestUser(t, app.DB, org.ID)
	key := createTestAPIKey(t, app, org.ID, owner.ID, "protected")

	req := testutil.NewJSONRequest(t, map[string]any{"is_active": false})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", key.ID.String())

	require.NoError(t, app.UpdateAPIKey(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// --- CreateAPIKey: validation gaps ---

func TestApp_CreateAPIKey_EmptyNameRejected(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Create APIKey EmptyName", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{"name": ""})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateAPIKey(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "Name is required")
}

func TestApp_CreateAPIKey_ExpiresAtParsedAndStored(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Create APIKey Expiry", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	expiry := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	req := testutil.NewJSONRequest(t, map[string]any{
		"name":       "with-expiry",
		"expires_at": expiry,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateAPIKey(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.APIKeyCreateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.NotNil(t, resp.Data.ExpiresAt, "expires_at should be persisted and returned")

	// Round-trip through DB to confirm storage.
	var got models.APIKey
	require.NoError(t, app.DB.Where("id = ?", resp.Data.ID).First(&got).Error)
	require.NotNil(t, got.ExpiresAt)
	assert.WithinDuration(t, resp.Data.ExpiresAt.UTC(), got.ExpiresAt.UTC(), time.Second)
}

func TestApp_CreateAPIKey_InvalidExpiresAtFormat(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Create APIKey BadExpiry", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":       "bad-expiry",
		"expires_at": "tomorrow at 3pm",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateAPIKey(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusBadRequest, "expires_at format")
}

func TestApp_CreateAPIKey_HashIsBcryptOfFullKey(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Create APIKey Hash", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{"name": "hash-test"})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateAPIKey(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.APIKeyCreateResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))

	var got models.APIKey
	require.NoError(t, app.DB.Where("id = ?", resp.Data.ID).First(&got).Error)
	assert.NotEqual(t, resp.Data.Key, got.KeyHash, "stored hash must not be the plaintext key")
	assert.Greater(t, len(got.KeyHash), 50, "stored hash should look like a bcrypt hash")
	// Stored prefix is the first 16 chars after "whm_" — 4..20 in the full key.
	assert.Equal(t, resp.Data.Key[4:20], got.KeyPrefix)
}

func TestApp_CreateAPIKey_PermissionDenied(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "No Create Perms", false, false, nil)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{"name": "blocked"})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateAPIKey(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// --- ListAPIKeys: search filter ---

func TestApp_ListAPIKeys_SearchFilter(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAPIKeyPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "List APIKey Search", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	createTestAPIKey(t, app, org.ID, user.ID, "alpha-prod")
	createTestAPIKey(t, app, org.ID, user.ID, "beta-staging")
	createTestAPIKey(t, app, org.ID, user.ID, "gamma-prod")

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "search", "prod")

	require.NoError(t, app.ListAPIKeys(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			APIKeys []handlers.APIKeyResponse `json:"api_keys"`
			Total   int                       `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, 2, resp.Data.Total, "search 'prod' should match alpha-prod + gamma-prod only")
	for _, k := range resp.Data.APIKeys {
		assert.Contains(t, k.Name, "prod")
	}
}
