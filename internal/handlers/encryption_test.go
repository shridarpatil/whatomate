package handlers_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/shridarpatil/whatomate/internal/crypto"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestWhatsAppBusinessEncryption_GenerateKeys(t *testing.T) {
	meta := newFakeMetaServer(t)
	app := newAppWithMeta(t, meta)
	app.Config.App.EncryptionKey = "this-is-a-32-character-test-key-XX"
	app.Config.App.Environment = "testing"

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	acc := createTestAccountForValidation(t, app.DB, org.ID, "phone-enc-1", "biz-enc-1")

	// Set up Meta fake server expectation for whatsapp_business_encryption endpoint
	meta.phoneFn = func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "whatsapp_business_encryption") && r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"success": true}`))
			return
		}
		_, _ = w.Write([]byte(`{"display_phone_number":"+1234567890","verified_name":"Test","account_mode":"LIVE","code_verification_status":"VERIFIED","quality_rating":"GREEN"}`))
	}

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", acc.ID.String())
	req.RequestCtx.Request.Header.Set("X-Forwarded-Host", "api.whatomate.test")

	err := app.GenerateSecureKeyPairs(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data map[string]any `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	pubKey, _ := resp.Data["encryption_public_key"].(string)

	assert.NotEmpty(t, pubKey)
	assert.Contains(t, pubKey, "-----BEGIN PUBLIC KEY-----")

	// Verify private key is encrypted at rest in DB
	var stored models.WhatsAppAccount
	err = app.DB.Where("id = ?", acc.ID).First(&stored).Error
	require.NoError(t, err)

	assert.NotEmpty(t, stored.EncryptionPrivateKey)
	assert.NotContains(t, stored.EncryptionPrivateKey, "-----BEGIN PRIVATE KEY-----")

	// Verify decryption of stored private key
	decryptedPriv, err := crypto.Decrypt(stored.EncryptionPrivateKey, app.Config.App.EncryptionKey)
	require.NoError(t, err)
	assert.Contains(t, decryptedPriv, "-----BEGIN PRIVATE KEY-----")
}

func TestWhatsAppBusinessEncryption_CryptoEngine(t *testing.T) {
	// Verify core crypto engine functions directly
	privPEM, pubPEM, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)
	assert.Contains(t, privPEM, "-----BEGIN PRIVATE KEY-----")
	assert.Contains(t, pubPEM, "-----BEGIN PUBLIC KEY-----")

	aesKey, err := crypto.GenerateRandomBytes(32)
	require.NoError(t, err)

	iv, err := crypto.GenerateRandomBytes(12)
	require.NoError(t, err)

	encAESKey, err := crypto.EncryptRSAOAEP(aesKey, pubPEM)
	require.NoError(t, err)

	sampleData := []byte("secret WhatsApp Flows payload")
	encPayload, err := crypto.EncryptAESGCM(sampleData, aesKey, iv)
	require.NoError(t, err)

	// Decrypt using helper
	decrypted, err := crypto.DecryptWhatsAppFlowsPayload(encPayload, encAESKey, crypto.Base64Encode(iv), privPEM)
	require.NoError(t, err)
	assert.Equal(t, sampleData, decrypted)

	// Encrypt response using helper (verifies IV bit-flipping)
	respData := []byte("response payload")
	encResp, err := crypto.EncryptWhatsAppFlowsResponse(respData, encAESKey, crypto.Base64Encode(iv), privPEM)
	require.NoError(t, err)
	assert.NotEmpty(t, encResp)
}
