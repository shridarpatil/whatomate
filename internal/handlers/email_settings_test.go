package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/shridarpatil/whatomate/internal/crypto"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestApp_GetEmailSettings_MasksPassword(t *testing.T) {
	app := newTestApp(t)

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// Encrypt a fake password and store it
	encPass, _ := crypto.Encrypt("real_smtp_password", app.Config.App.EncryptionKey)

	org.Settings = models.JSONB{
		"smtp_host": "smtp.gmail.com",
		"smtp_pass": encPass,
	}
	app.DB.Save(org)

	req := testutil.NewRequest(t)
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("user_id", user.ID)

	err := app.GetEmailSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			SMTPHost string `json:"smtp_host"`
			SMTPPass string `json:"smtp_pass"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "smtp.gmail.com", resp.Data.SMTPHost)
	assert.Equal(t, "••••••••", resp.Data.SMTPPass, "Password must be masked in the response")
}

func TestApp_UpdateEmailSettings_IgnoresMaskedPassword(t *testing.T) {
	app := newTestApp(t)

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// Setup initial settings
	originalEncPass, _ := crypto.Encrypt("initial_password", app.Config.App.EncryptionKey)
	org.Settings = models.JSONB{
		"email_enabled": true,
		"smtp_host":     "smtp.test.com",
		"smtp_pass":     originalEncPass,
	}
	app.DB.Save(org)

	// Send an update that modifies the host but tries to pass back the UI masked password
	hostStr := "new.smtp.com"
	passStr := "••••••••"

	req := testutil.NewJSONRequest(t, map[string]any{
		"smtp_host": &hostStr,
		"smtp_pass": &passStr,
	})
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("user_id", user.ID)

	err := app.UpdateEmailSettings(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Re-fetch org from DB
	var updatedOrg models.Organization
	app.DB.Where("id = ?", org.ID).First(&updatedOrg)

	newHost, _ := updatedOrg.Settings["smtp_host"].(string)
	newPass, _ := updatedOrg.Settings["smtp_pass"].(string)

	assert.Equal(t, "new.smtp.com", newHost, "Host should be updated")
	assert.Equal(t, originalEncPass, newPass, "Password should NOT be updated because it sent the masked string")
}

func TestApp_UpdateEmailSettings_UpdatesRealPassword(t *testing.T) {
	app := newTestApp(t)

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	hostStr := "smtp.mailgun.org"
	passStr := "new_real_password_123"

	req := testutil.NewJSONRequest(t, map[string]any{
		"smtp_host": &hostStr,
		"smtp_pass": &passStr,
	})
	req.RequestCtx.SetUserValue("organization_id", org.ID)
	req.RequestCtx.SetUserValue("user_id", user.ID)

	err := app.UpdateEmailSettings(req)
	require.NoError(t, err)

	var updatedOrg models.Organization
	app.DB.Where("id = ?", org.ID).First(&updatedOrg)

	newPassEnc, _ := updatedOrg.Settings["smtp_pass"].(string)

	decryptedPass, _ := crypto.Decrypt(newPassEnc, app.Config.App.EncryptionKey)
	assert.Equal(t, "new_real_password_123", decryptedPass, "Password should be encrypted and saved correctly")
}
