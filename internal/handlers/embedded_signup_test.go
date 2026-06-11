package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/crypto"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// --- ExchangeToken Tests ---

func TestApp_ExchangeToken_Success_AutoRegistration(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	// Use unique IDs to prevent conflicts with parallel tests
	phoneID := fmt.Sprintf("123456789%d", time.Now().UnixNano()%1000000)
	wabaID := fmt.Sprintf("987654321%d", time.Now().UnixNano()%1000000)

	// Mock Meta API server
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/oauth/access_token"):
			// Token exchange (query parameters are in r.URL.RawQuery, not path)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": "EAABwzLixnjYBO1234567890",
			})
		case strings.Contains(path, phoneID):
			if strings.HasSuffix(path, "/register") {
				// Registration
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			} else {
				// Phone info - use timestamp to ensure unique account names
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"verified_name":        fmt.Sprintf("Test Business %d", time.Now().UnixNano()),
					"display_phone_number": "+1234567890",
				})
			}
		case strings.Contains(path, wabaID):
			// Webhook subscription
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer metaServer.Close()

	// Override WhatsApp client to use test server
	app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, metaServer.URL)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"code":     "test_auth_code_123",
		"phone_id": phoneID,
		"waba_id":  wabaID,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ExchangeToken(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	accountMap, ok := resp.Data["account"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "active", accountMap["status"])
	assert.Equal(t, phoneID, accountMap["phone_id"])
	assert.Equal(t, wabaID, accountMap["business_id"])
	assert.NotEmpty(t, resp.Data["pin"]) // PIN should be returned

	// Verify account was created in database
	var account models.WhatsAppAccount
	err = app.DB.Where("phone_id = ? AND organization_id = ?", phoneID, org.ID).First(&account).Error
	require.NoError(t, err)
	assert.Equal(t, "active", account.Status)
	assert.NotEmpty(t, account.Pin)
	assert.True(t, crypto.IsEncrypted(account.AccessToken))
	assert.True(t, crypto.IsEncrypted(account.Pin))
	account.DecryptSecrets(app.Config.App.EncryptionKey)
	assert.Equal(t, "EAABwzLixnjYBO1234567890", account.AccessToken)

	// Verify audit log exists
	assert.Eventually(t, func() bool {
		var auditCount int64
		if err := app.DB.Model(&models.AuditLog{}).Where("organization_id = ? AND resource_type = ? AND action = ?", org.ID, "account", models.AuditActionCreated).Count(&auditCount).Error; err != nil {
			return false
		}
		return auditCount > 0
	}, 1*time.Second, 10*time.Millisecond)
}

func TestApp_ExchangeToken_Success_PendingRegistration(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	// Use unique IDs to prevent conflicts with parallel tests
	phoneID := fmt.Sprintf("223456789%d", time.Now().UnixNano()%1000000)
	wabaID := fmt.Sprintf("887654321%d", time.Now().UnixNano()%1000000)

	// Mock Meta API server - registration fails (PIN already exists)
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/oauth/access_token"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": "test_token",
			})
		case strings.Contains(path, phoneID):
			if strings.HasSuffix(path, "/register") {
				// Registration fails - PIN already exists
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(whatsapp.MetaAPIError{
					Error: struct {
						Message      string `json:"message"`
						Type         string `json:"type"`
						Code         int    `json:"code"`
						ErrorSubcode int    `json:"error_subcode"`
						ErrorUserMsg string `json:"error_user_msg"`
						ErrorData    struct {
							Details string `json:"details"`
						} `json:"error_data"`
						FBTraceID string `json:"fbtrace_id"`
					}{
						Message: "Two-step verification is already enabled",
						Code:    33,
					},
				})
			} else {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"verified_name": fmt.Sprintf("Test Business %d", time.Now().UnixNano()),
				})
			}
		case strings.Contains(path, wabaID):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer metaServer.Close()

	app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, metaServer.URL)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"code":     "test_code",
		"phone_id": phoneID,
		"waba_id":  wabaID,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ExchangeToken(req)
	require.NoError(t, err)

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	accountMap, ok := resp.Data["account"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "pending_registration", accountMap["status"])
	assert.Nil(t, resp.Data["pin"]) // No PIN when pending
}

func TestApp_ExchangeToken_InvalidCode(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	// Mock Meta API server - invalid code
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(whatsapp.MetaAPIError{
			Error: struct {
				Message      string `json:"message"`
				Type         string `json:"type"`
				Code         int    `json:"code"`
				ErrorSubcode int    `json:"error_subcode"`
				ErrorUserMsg string `json:"error_user_msg"`
				ErrorData    struct {
					Details string `json:"details"`
				} `json:"error_data"`
				FBTraceID string `json:"fbtrace_id"`
			}{
				Message: "Invalid authorization code",
				Code:    100,
			},
		})
	}))
	defer metaServer.Close()

	app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, metaServer.URL)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"code":     "invalid_code",
		"phone_id": "123456789",
		"waba_id":  "987654321",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ExchangeToken(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	body := string(testutil.GetResponseBody(req))
	assert.Contains(t, body, "Invalid authorization code")
}

func TestApp_ExchangeToken_Success_CodeOnly_Discovery(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	phoneID := fmt.Sprintf("333456789%d", time.Now().UnixNano()%1000000)
	wabaID := fmt.Sprintf("777654321%d", time.Now().UnixNano()%1000000)

	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/oauth/access_token"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": "discovery_token",
			})
		case strings.Contains(path, "/debug_token"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": whatsapp.TokenDebugInfo{
					AppID:   "test_app",
					IsValid: true,
					GranularScopes: []struct {
						Scope     string   `json:"scope"`
						TargetIds []string `json:"target_ids,omitempty"`
					}{
						{
							Scope:     "whatsapp_business_management",
							TargetIds: []string{wabaID},
						},
					},
				},
			})
		case strings.Contains(path, wabaID) && strings.Contains(path, "/phone_numbers"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(whatsapp.WABAPhoneNumbersResponse{
				Data: []struct {
					ID                 string `json:"id"`
					DisplayPhoneNumber string `json:"display_phone_number"`
					VerifiedName       string `json:"verified_name"`
					QualityRating      string `json:"quality_rating"`
				}{
					{
						ID:                 phoneID,
						DisplayPhoneNumber: "+1999999999",
						VerifiedName:       "Discovered Phone",
						QualityRating:      "GREEN",
					},
				},
			})
		case strings.Contains(path, "/me/accounts"):
			// Fallback mock (should not be reached if debug_token works)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(whatsapp.SharedWABAResponse{})
		case strings.Contains(path, phoneID) && strings.Contains(path, "/register"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		case strings.Contains(path, wabaID) && strings.Contains(path, "/subscribed_apps"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		case strings.Contains(path, phoneID): // Phone Info
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"verified_name":        "Discovered Phone",
				"display_phone_number": "+1999999999",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer metaServer.Close()

	app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, metaServer.URL)

	// Omit phone_id and waba_id
	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"code": "test_code_only",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ExchangeToken(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	accountMap, ok := resp.Data["account"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "active", accountMap["status"])
	assert.Equal(t, phoneID, accountMap["phone_id"])
	assert.Equal(t, wabaID, accountMap["business_id"])
	assert.Equal(t, "v21.0", accountMap["api_version"])
}

func TestApp_ExchangeToken_MissingFields(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"phone_id": "123",
		"waba_id":  "456",
	}) // missing code
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ExchangeToken(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_ExchangeToken_Unauthorized(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"code":     "test",
		"phone_id": "123",
		"waba_id":  "456",
	})
	// No auth context set

	err := app.ExchangeToken(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- RegisterPhoneNumber Tests ---

func TestApp_RegisterPhoneNumber_Success_WithPIN(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	// Create account with pending_registration status
	account := &models.WhatsAppAccount{
		OrganizationID: org.ID,
		Name:           "Test Account - RegisterPhone WithPIN",
		PhoneID:        "123456789",
		BusinessID:     "987654321",
		AccessToken:    "test_token",
		APIVersion:     "v21.0",
		Status:         "pending_registration",
	}
	require.NoError(t, app.DB.Create(account).Error)

	// Mock Meta API server
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/register") {
			// Registration call
			assert.Equal(t, "/v21.0/123456789/register", r.URL.Path)
			assert.Equal(t, "Bearer test_token", r.Header.Get("Authorization"))

			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			assert.Equal(t, "654321", body["pin"])

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		} else {
			// GetPhoneNumberInfo call — return non-SMB phone info
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"id":            "123456789",
				"platform_type": "CLOUD_API",
			})
		}
	}))
	defer metaServer.Close()

	app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, metaServer.URL)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"pin": "654321",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.RegisterPhoneNumber(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Data["success"].(bool))
	assert.Equal(t, "654321", resp.Data["pin"])

	// Verify account status updated
	var updated models.WhatsAppAccount
	require.NoError(t, app.DB.Where("id = ?", account.ID).First(&updated).Error)
	assert.Equal(t, "active", updated.Status)
	assert.True(t, crypto.IsEncrypted(updated.Pin))
	updated.DecryptSecrets(app.Config.App.EncryptionKey)
	assert.Equal(t, "654321", updated.Pin)

	// Verify audit log exists
	time.Sleep(50 * time.Millisecond)
	var auditCount int64
	require.NoError(t, app.DB.Model(&models.AuditLog{}).Where("organization_id = ? AND resource_type = ? AND action = ?", org.ID, "account", models.AuditActionUpdated).Count(&auditCount).Error)
	assert.Greater(t, auditCount, int64(0))
}

func TestApp_RegisterPhoneNumber_Success_GeneratedPIN(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	account := &models.WhatsAppAccount{
		OrganizationID: org.ID,
		Name:           "Test Account - GeneratedPIN",
		PhoneID:        "123456789",
		BusinessID:     "987654321",
		AccessToken:    "test_token",
		APIVersion:     "v21.0",
		Status:         "pending_registration",
	}
	require.NoError(t, app.DB.Create(account).Error)

	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/register") {
			// Registration call — validate the generated PIN
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			assert.Len(t, body["pin"], 6) // Generated PIN should be 6 digits

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		} else {
			// GetPhoneNumberInfo call — return non-SMB phone info
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"id":            "123456789",
				"platform_type": "CLOUD_API",
			})
		}
	}))
	defer metaServer.Close()

	app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, metaServer.URL)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		// No PIN provided - should generate one
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.RegisterPhoneNumber(req)
	require.NoError(t, err)

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Data["success"].(bool))
	assert.NotEmpty(t, resp.Data["pin"])
	assert.Len(t, resp.Data["pin"].(string), 6)

	// Verify account status updated
	var updated models.WhatsAppAccount
	require.NoError(t, app.DB.Where("id = ?", account.ID).First(&updated).Error)
	assert.Equal(t, "active", updated.Status)
	assert.True(t, crypto.IsEncrypted(updated.Pin))
	updated.DecryptSecrets(app.Config.App.EncryptionKey)
	assert.Len(t, updated.Pin, 6)

	// Verify audit log exists
	time.Sleep(50 * time.Millisecond)
	var auditCount int64
	require.NoError(t, app.DB.Model(&models.AuditLog{}).Where("organization_id = ? AND resource_type = ? AND action = ?", org.ID, "account", models.AuditActionUpdated).Count(&auditCount).Error)
	assert.Greater(t, auditCount, int64(0))
}

func TestApp_RegisterPhoneNumber_RegistrationFailed(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	account := &models.WhatsAppAccount{
		OrganizationID: org.ID,
		Name:           "Test Account - RegFailed",
		PhoneID:        "123456789",
		BusinessID:     "987654321",
		AccessToken:    "test_token",
		APIVersion:     "v21.0",
		Status:         "pending_registration",
	}
	require.NoError(t, app.DB.Create(account).Error)

	// Mock Meta API server - registration fails
	metaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(whatsapp.MetaAPIError{
			Error: struct {
				Message      string `json:"message"`
				Type         string `json:"type"`
				Code         int    `json:"code"`
				ErrorSubcode int    `json:"error_subcode"`
				ErrorUserMsg string `json:"error_user_msg"`
				ErrorData    struct {
					Details string `json:"details"`
				} `json:"error_data"`
				FBTraceID string `json:"fbtrace_id"`
			}{
				Message: "Phone number must be verified before registration",
				Code:    368,
			},
		})
	}))
	defer metaServer.Close()

	app.WhatsApp = whatsapp.NewWithBaseURL(app.Log, metaServer.URL)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"pin": "123456",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.RegisterPhoneNumber(req)
	require.NoError(t, err)

	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
	body := string(testutil.GetResponseBody(req))
	assert.Contains(t, body, "Phone number must be verified")

	// Verify account status NOT updated
	var updated models.WhatsAppAccount
	require.NoError(t, app.DB.Where("id = ?", account.ID).First(&updated).Error)
	assert.Equal(t, "pending_registration", updated.Status)
}

func TestApp_RegisterPhoneNumber_AccountNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"pin": "123456",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.RegisterPhoneNumber(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_RegisterPhoneNumber_InvalidID(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"pin": "123456",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.RegisterPhoneNumber(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_RegisterPhoneNumber_CrossOrgIsolation(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org1 := testutil.CreateTestOrganization(t, app.DB)
	org2 := testutil.CreateTestOrganization(t, app.DB)
	user2 := createAdminUser(t, app, org2.ID)

	// Create account in org1
	account := &models.WhatsAppAccount{
		OrganizationID: org1.ID,
		Name:           "Test Account - CrossOrg Isolation",
		PhoneID:        "123456789",
		BusinessID:     "987654321",
		AccessToken:    "test_token",
		APIVersion:     "v21.0",
		Status:         "pending_registration",
	}
	require.NoError(t, app.DB.Create(account).Error)

	// User from org2 tries to register org1's account
	req := testutil.NewJSONRequest(t, map[string]interface{}{
		"pin": "123456",
	})
	testutil.SetAuthContext(req, org2.ID, user2.ID)
	testutil.SetPathParam(req, "id", account.ID.String())

	err := app.RegisterPhoneNumber(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}
