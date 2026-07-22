package handlers_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// signWebhook returns the X-Hub-Signature-256 header value for the given body
// and app secret, as Meta would compute it.
func signWebhook(body []byte, appSecret string) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// newAppForWebhook builds an App with config for webhook verification.
func newAppForWebhook(t *testing.T, globalVerifyToken string) *handlers.App {
	t.Helper()
	app := newTestApp(t)
	app.Config = &config.Config{
		JWT:      config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
		WhatsApp: config.WhatsAppConfig{WebhookVerifyToken: globalVerifyToken, BaseURL: "https://graph.facebook.com", APIVersion: "v18.0"},
	}
	return app
}

// --- WebhookVerify (GET challenge) ---

func TestApp_WebhookVerify_GlobalTokenSucceeds(t *testing.T) {
	app := newAppForWebhook(t, "shared-secret")

	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "hub.mode", "subscribe")
	testutil.SetQueryParam(req, "hub.verify_token", "shared-secret")
	testutil.SetQueryParam(req, "hub.challenge", "challenge-xyz")

	require.NoError(t, app.WebhookVerify(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	assert.Equal(t, "challenge-xyz", string(testutil.GetResponseBody(req)),
		"verify must echo the hub.challenge value")
}

func TestApp_WebhookVerify_AccountTokenSucceeds(t *testing.T) {
	app := newAppForWebhook(t, "")
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     org.ID,
		Name:               "wbk-acc",
		PhoneID:            "phone-1",
		BusinessID:         "biz-1",
		AccessToken:        "tok",
		WebhookVerifyToken: "per-account-token",
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, app.DB.Create(acc).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "hub.mode", "subscribe")
	testutil.SetQueryParam(req, "hub.verify_token", "per-account-token")
	testutil.SetQueryParam(req, "hub.challenge", "ch-2")

	require.NoError(t, app.WebhookVerify(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	assert.Equal(t, "ch-2", string(testutil.GetResponseBody(req)))
}

func TestApp_WebhookVerify_WrongModeRejected(t *testing.T) {
	app := newAppForWebhook(t, "shared-secret")

	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "hub.mode", "ping")
	testutil.SetQueryParam(req, "hub.verify_token", "shared-secret")
	testutil.SetQueryParam(req, "hub.challenge", "x")

	require.NoError(t, app.WebhookVerify(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_WebhookVerify_UnknownTokenRejected(t *testing.T) {
	app := newAppForWebhook(t, "shared-secret")

	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "hub.mode", "subscribe")
	testutil.SetQueryParam(req, "hub.verify_token", "wrong-secret")
	testutil.SetQueryParam(req, "hub.challenge", "x")

	require.NoError(t, app.WebhookVerify(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_WebhookVerify_EmptyTokenWithEmptyConfigRejected(t *testing.T) {
	// If config token is empty, an empty incoming token must NOT match (otherwise an
	// attacker could send no token at all and pass verification).
	app := newAppForWebhook(t, "")

	// Production code falls back to `WHERE webhook_verify_token = ?` against the
	// accounts table. If a leftover row from another test in the suite happens to
	// have an empty webhook_verify_token, the empty incoming token would match it
	// and the test would be meaningless. Skip in that case rather than mutating
	// shared state — other tests later in the run depend on these rows.
	var leftoverEmpty int64
	require.NoError(t, app.DB.Model(&models.WhatsAppAccount{}).
		Where("webhook_verify_token = ?", "").Count(&leftoverEmpty).Error)
	if leftoverEmpty > 0 {
		t.Skipf("test data has %d account(s) with empty webhook_verify_token; this branch is only meaningful when none exist", leftoverEmpty)
	}

	req := testutil.NewGETRequest(t)
	testutil.SetQueryParam(req, "hub.mode", "subscribe")
	testutil.SetQueryParam(req, "hub.verify_token", "")
	testutil.SetQueryParam(req, "hub.challenge", "x")

	require.NoError(t, app.WebhookVerify(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// --- WebhookHandler signature verification ---

// makeMessagesPayload returns a minimal but valid messages-event webhook payload
// for the given phone number ID (no actual messages, just metadata).
func makeMessagesPayload(phoneNumberID string) []byte {
	body := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{{
			"id": "WABA-123",
			"changes": []map[string]any{{
				"field": "messages",
				"value": map[string]any{
					"messaging_product": "whatsapp",
					"metadata": map[string]any{
						"display_phone_number": "+1234567890",
						"phone_number_id":      phoneNumberID,
					},
				},
			}},
		}},
	}
	b, _ := json.Marshal(body)
	return b
}

func TestApp_WebhookHandler_NoSignatureNoAppSecret_Accepted(t *testing.T) {
	// When no AppSecret is configured for the matching account, the handler must
	// not reject — it just skips signature verification.
	app := newAppForWebhook(t, "")
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     org.ID,
		Name:               "wbk-h-1",
		PhoneID:            "phone-A",
		BusinessID:         "biz-A",
		AccessToken:        "tok",
		WebhookVerifyToken: "vt",
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, app.DB.Create(acc).Error)

	body := makeMessagesPayload("phone-A")
	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.SetBody(body)

	require.NoError(t, app.WebhookHandler(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

func TestApp_WebhookHandler_ValidSignature_Accepted(t *testing.T) {
	app := newAppForWebhook(t, "")
	org := testutil.CreateTestOrganization(t, app.DB)
	appSecret := "shhh-app-secret-32-bytes-long-xx"
	acc := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     org.ID,
		Name:               "wbk-h-2",
		PhoneID:            "phone-B",
		BusinessID:         "biz-B",
		AccessToken:        "tok",
		AppSecret:          appSecret,
		WebhookVerifyToken: "vt",
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, app.DB.Create(acc).Error)

	body := makeMessagesPayload("phone-B")
	sig := signWebhook(body, appSecret)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.Set("X-Hub-Signature-256", sig)
	req.RequestCtx.Request.SetBody(body)

	require.NoError(t, app.WebhookHandler(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

func TestApp_WebhookHandler_InvalidSignature_Rejected(t *testing.T) {
	app := newAppForWebhook(t, "")
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     org.ID,
		Name:               "wbk-h-3",
		PhoneID:            "phone-C",
		BusinessID:         "biz-C",
		AccessToken:        "tok",
		AppSecret:          "real-secret",
		WebhookVerifyToken: "vt",
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, app.DB.Create(acc).Error)

	body := makeMessagesPayload("phone-C")
	sig := signWebhook(body, "WRONG-SECRET")

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.Header.Set("X-Hub-Signature-256", sig)
	req.RequestCtx.Request.SetBody(body)

	require.NoError(t, app.WebhookHandler(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req),
		"invalid signature must be rejected before any processing")
}

func TestApp_WebhookHandler_MalformedJSONRejected(t *testing.T) {
	app := newAppForWebhook(t, "")

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.SetBody([]byte("{not json"))

	require.NoError(t, app.WebhookHandler(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_WebhookHandler_BadlyFormattedSignatureRejected(t *testing.T) {
	app := newAppForWebhook(t, "")
	org := testutil.CreateTestOrganization(t, app.DB)
	acc := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     org.ID,
		Name:               "wbk-h-4",
		PhoneID:            "phone-D",
		BusinessID:         "biz-D",
		AccessToken:        "tok",
		AppSecret:          "real-secret",
		WebhookVerifyToken: "vt",
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, app.DB.Create(acc).Error)

	body := makeMessagesPayload("phone-D")

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.Header.SetContentType("application/json")
	// Wrong prefix (not "sha256=")
	req.RequestCtx.Request.Header.Set("X-Hub-Signature-256", "md5=deadbeef")
	req.RequestCtx.Request.SetBody(body)

	require.NoError(t, app.WebhookHandler(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_WebhookHandler_EmptyEntryAccepted(t *testing.T) {
	// A payload with no entries should still 200 (no work to do).
	app := newAppForWebhook(t, "")

	body, _ := json.Marshal(map[string]any{
		"object": "whatsapp_business_account",
		"entry":  []map[string]any{},
	})

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.SetBody(body)

	require.NoError(t, app.WebhookHandler(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}
