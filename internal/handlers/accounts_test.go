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

// --- ListAccounts Tests ---

func TestApp_ListAccounts_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// Create two accounts for this org
	acc1 := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	acc2 := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListAccounts(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Accounts []handlers.AccountResponse `json:"accounts"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Accounts, 2)

	// Accounts are ordered by created_at DESC, so acc2 should be first
	assert.Equal(t, acc2.ID, resp.Data.Accounts[0].ID)
	assert.Equal(t, acc1.ID, resp.Data.Accounts[1].ID)

	// Verify sensitive fields are not exposed (has_access_token is set instead)
	assert.True(t, resp.Data.Accounts[0].HasAccessToken)
}

func TestApp_ListAccounts_Empty(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListAccounts(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Accounts []handlers.AccountResponse `json:"accounts"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Data.Accounts, 0)
}

func TestApp_ListAccounts_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context set

	err := app.ListAccounts(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

func TestApp_ListAccounts_OrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user1 := testutil.CreateTestUser(t, app.DB, org1.ID)
	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	// Create accounts for org1
	testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)
	testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)

	// Create one account for org2
	testutil.CreateTestWhatsAppAccount(t, app.DB, org2.ID)

	// org1 user should see 2 accounts
	req1 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req1, org1.ID, user1.ID)

	err := app.ListAccounts(req1)
	require.NoError(t, err)

	var resp1 struct {
		Data struct {
			Accounts []handlers.AccountResponse `json:"accounts"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req1), &resp1)
	require.NoError(t, err)
	assert.Len(t, resp1.Data.Accounts, 2)

	// org2 user should see 1 account
	req2 := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req2, org2.ID, user2.ID)

	err = app.ListAccounts(req2)
	require.NoError(t, err)

	var resp2 struct {
		Data struct {
			Accounts []handlers.AccountResponse `json:"accounts"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req2), &resp2)
	require.NoError(t, err)
	assert.Len(t, resp2.Data.Accounts, 1)
}

// --- CreateAccount Tests ---

func TestApp_CreateAccount_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":         "My WhatsApp Account",
		"phone_id":     "123456789",
		"business_id":  "987654321",
		"access_token": "test-access-token",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.AccountResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "My WhatsApp Account", resp.Data.Name)
	assert.Equal(t, "123456789", resp.Data.PhoneID)
	assert.Equal(t, "987654321", resp.Data.BusinessID)
	assert.Equal(t, "active", resp.Data.Status)
	assert.Equal(t, "v21.0", resp.Data.APIVersion) // default version
	assert.True(t, resp.Data.HasAccessToken)
	assert.NotEmpty(t, resp.Data.WebhookVerifyToken) // auto-generated
	assert.NotEqual(t, uuid.Nil, resp.Data.ID)
}

func TestApp_CreateAccount_WithOptionalFields(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":                 "Full Account",
		"phone_id":             "111222333",
		"business_id":          "444555666",
		"access_token":         "my-token",
		"app_id":               "my-app-id",
		"app_secret":           "my-app-secret",
		"webhook_verify_token": "custom-verify-token",
		"api_version":          "v19.0",
		"is_default_incoming":  true,
		"is_default_outgoing":  true,
		"auto_read_receipt":    true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.AccountResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Full Account", resp.Data.Name)
	assert.Equal(t, "my-app-id", resp.Data.AppID)
	assert.Equal(t, "custom-verify-token", resp.Data.WebhookVerifyToken)
	assert.Equal(t, "v19.0", resp.Data.APIVersion)
	assert.True(t, resp.Data.IsDefaultIncoming)
	assert.True(t, resp.Data.IsDefaultOutgoing)
	assert.True(t, resp.Data.AutoReadReceipt)
	assert.True(t, resp.Data.HasAccessToken)
	assert.True(t, resp.Data.HasAppSecret)
}

func TestApp_CreateAccount_ValidationErrors(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing_name",
			body: map[string]interface{}{
				"phone_id":     "123",
				"business_id":  "456",
				"access_token": "tok",
			},
		},
		{
			name: "missing_phone_id",
			body: map[string]interface{}{
				"name":         "Test",
				"business_id":  "456",
				"access_token": "tok",
			},
		},
		{
			name: "missing_business_id",
			body: map[string]interface{}{
				"name":         "Test",
				"phone_id":     "123",
				"access_token": "tok",
			},
		},
		{
			name: "missing_access_token",
			body: map[string]interface{}{
				"name":        "Test",
				"phone_id":    "123",
				"business_id": "456",
			},
		},
		{
			name: "all_fields_empty",
			body: map[string]interface{}{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.NewJSONRequest(t, tc.body)
			testutil.SetAuthContext(req, org.ID, user.ID)

			err := app.CreateAccount(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
		})
	}
}

func TestApp_CreateAccount_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":         "Test",
		"phone_id":     "123",
		"business_id":  "456",
		"access_token": "tok",
	})
	// No auth context set

	err := app.CreateAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- GetAccount Tests ---

func TestApp_GetAccount_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.GetAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.AccountResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, account.ID, resp.Data.ID)
	assert.Equal(t, account.Name, resp.Data.Name)
	assert.Equal(t, account.PhoneID, resp.Data.PhoneID)
	assert.Equal(t, account.BusinessID, resp.Data.BusinessID)
	assert.Equal(t, account.APIVersion, resp.Data.APIVersion)
	assert.Equal(t, "active", resp.Data.Status)
	assert.True(t, resp.Data.HasAccessToken)
}

func TestApp_GetAccount_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetAccount_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetAccount_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	// Create two separate organizations
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)

	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	// Create account in org1
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)

	// User from org2 tries to access org1's account
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.GetAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- UpdateAccount Tests ---

func TestApp_UpdateAccount_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name":              "Updated Account Name",
		"phone_id":          "new-phone-id",
		"business_id":       "new-business-id",
		"access_token":      "new-access-token",
		"api_version":       "v20.0",
		"auto_read_receipt": true,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.UpdateAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.AccountResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, account.ID, resp.Data.ID)
	assert.Equal(t, "Updated Account Name", resp.Data.Name)
	assert.Equal(t, "new-phone-id", resp.Data.PhoneID)
	assert.Equal(t, "new-business-id", resp.Data.BusinessID)
	assert.Equal(t, "v20.0", resp.Data.APIVersion)
	assert.True(t, resp.Data.AutoReadReceipt)
	assert.True(t, resp.Data.HasAccessToken)

	// Verify the update persisted in the database
	var updated models.WhatsAppAccount
	require.NoError(t, app.DB.Where("id = ?", account.ID).First(&updated).Error)
	assert.Equal(t, "Updated Account Name", updated.Name)
	assert.Equal(t, "new-phone-id", updated.PhoneID)
	assert.Equal(t, "new-access-token", updated.AccessToken)
}

func TestApp_UpdateAccount_PartialUpdate(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	// Only update the name, leave other fields unchanged
	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Only Name Changed",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.UpdateAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.AccountResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Only Name Changed", resp.Data.Name)
	// Original values should be preserved
	assert.Equal(t, account.PhoneID, resp.Data.PhoneID)
	assert.Equal(t, account.BusinessID, resp.Data.BusinessID)
	assert.Equal(t, account.APIVersion, resp.Data.APIVersion)
}

func TestApp_UpdateAccount_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Updated Name",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_UpdateAccount_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"name": "Updated",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.UpdateAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- DeleteAccount Tests ---

func TestApp_DeleteAccount_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.DeleteAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Account deleted successfully", resp.Data.Message)

	// Verify account is soft-deleted (GORM default with DeletedAt)
	var count int64
	app.DB.Model(&models.WhatsAppAccount{}).Where("id = ?", account.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestApp_DeleteAccount_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteAccount_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.DeleteAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_DeleteAccount_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)

	user2 := testutil.CreateTestUser(t, app.DB, org2.ID)

	// Create account in org1
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org1.ID)

	// User from org2 tries to delete org1's account
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.DeleteAccount(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	// Verify the account still exists in org1
	var count int64
	app.DB.Model(&models.WhatsAppAccount{}).Where("id = ?", account.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}
