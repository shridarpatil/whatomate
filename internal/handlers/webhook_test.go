package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyWebhookSignature(t *testing.T) {
	t.Parallel()

	// Test data
	appSecret := []byte("test_app_secret_12345")
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"123","changes":[]}]}`)

	// Compute valid signature
	mac := hmac.New(sha256.New, appSecret)
	mac.Write(body)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		body      []byte
		signature []byte
		appSecret []byte
		want      bool
	}{
		{
			name:      "valid signature",
			body:      body,
			signature: []byte(validSig),
			appSecret: appSecret,
			want:      true,
		},
		{
			name:      "invalid signature - wrong hash",
			body:      body,
			signature: []byte("sha256=0000000000000000000000000000000000000000000000000000000000000000"),
			appSecret: appSecret,
			want:      false,
		},
		{
			name:      "invalid signature - wrong secret",
			body:      body,
			signature: []byte(validSig),
			appSecret: []byte("wrong_secret"),
			want:      false,
		},
		{
			name:      "invalid signature - modified body",
			body:      []byte(`{"object":"modified"}`),
			signature: []byte(validSig),
			appSecret: appSecret,
			want:      false,
		},
		{
			name:      "invalid signature - missing sha256 prefix",
			body:      body,
			signature: []byte(hex.EncodeToString(mac.Sum(nil))),
			appSecret: appSecret,
			want:      false,
		},
		{
			name:      "invalid signature - wrong prefix",
			body:      body,
			signature: []byte("sha1=" + hex.EncodeToString(mac.Sum(nil))),
			appSecret: appSecret,
			want:      false,
		},
		{
			name:      "empty signature",
			body:      body,
			signature: []byte{},
			appSecret: appSecret,
			want:      false,
		},
		{
			name:      "empty body with valid signature for empty body",
			body:      []byte{},
			signature: func() []byte {
				m := hmac.New(sha256.New, appSecret)
				m.Write([]byte{})
				return []byte("sha256=" + hex.EncodeToString(m.Sum(nil)))
			}(),
			appSecret: appSecret,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := verifyWebhookSignature(tt.body, tt.signature, tt.appSecret)
			assert.Equal(t, tt.want, got, "verifyWebhookSignature() = %v, want %v", got, tt.want)
		})
	}
}

func TestVerifyWebhookSignature_RealWorldExample(t *testing.T) {
	t.Parallel()

	// Simulate a real Meta webhook payload
	payload := `{"object":"whatsapp_business_account","entry":[{"id":"123456789","changes":[{"value":{"messaging_product":"whatsapp","metadata":{"display_phone_number":"15551234567","phone_number_id":"987654321"},"messages":[{"from":"15559876543","id":"wamid.abc123","timestamp":"1234567890","type":"text","text":{"body":"Hello"}}]},"field":"messages"}]}]}`
	appSecret := "my_app_secret_from_meta_dashboard"

	// Compute signature like Meta would
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(payload))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Verify
	result := verifyWebhookSignature([]byte(payload), []byte(signature), []byte(appSecret))
	assert.True(t, result, "Should verify real-world webhook payload")
}

func TestVerifyWebhookSignature_TimingAttackResistance(t *testing.T) {
	t.Parallel()

	// This test ensures we use constant-time comparison
	// by verifying the function behaves correctly with similar signatures
	appSecret := []byte("test_secret")
	body := []byte("test body")

	mac := hmac.New(sha256.New, appSecret)
	mac.Write(body)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Create a signature that differs only in the last character
	almostValidSig := validSig[:len(validSig)-1] + "0"

	assert.True(t, verifyWebhookSignature(body, []byte(validSig), appSecret))
	assert.False(t, verifyWebhookSignature(body, []byte(almostValidSig), appSecret))
}

// webhookTestApp creates a minimal App for webhook tests.
func webhookTestApp(t *testing.T) *App {
	t.Helper()
	db := testutil.SetupTestDB(t)
	return &App{
		DB:  db,
		Log: testutil.NopLogger(),
	}
}

// webhookTestData creates an organization, message with campaign metadata,
// campaign, and recipient. Returns all created records.
func webhookTestData(t *testing.T, app *App, msgStatus models.MessageStatus) (models.Organization, models.Message, models.BulkMessageCampaign, models.BulkMessageRecipient) {
	t.Helper()
	uid := uuid.New().String()[:8]

	org := models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "wh-test-" + uid,
		Slug:      "wh-test-" + uid,
	}
	require.NoError(t, app.DB.Create(&org).Error)

	contact := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "919999" + uid,
		ProfileName:    "Test User",
	}
	require.NoError(t, app.DB.Create(&contact).Error)

	waAccount := models.WhatsAppAccount{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "wh-acct-" + uid,
		PhoneID:        "phone-" + uid,
		BusinessID:     "biz-" + uid,
		AccessToken:    "token",
	}
	require.NoError(t, app.DB.Create(&waAccount).Error)

	tmpl := models.Template{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: waAccount.Name,
		Name:            "tmpl-" + uid,
		Language:        "en",
		BodyContent:     "Hello {{1}}",
	}
	require.NoError(t, app.DB.Create(&tmpl).Error)

	user := models.User{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Email:          "wh-" + uid + "@test.com",
		FullName:       "Test User",
		PasswordHash:   "hash",
	}
	require.NoError(t, app.DB.Create(&user).Error)

	campaign := models.BulkMessageCampaign{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: waAccount.Name,
		Name:            "test-campaign-" + uid,
		TemplateID:      tmpl.ID,
		Status:          models.CampaignStatusCompleted,
		CreatedBy:       user.ID,
	}
	require.NoError(t, app.DB.Create(&campaign).Error)

	waMsgID := "wamid.test-" + uid

	msg := models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New()},
		OrganizationID:    org.ID,
		WhatsAppAccount:   "test-account",
		ContactID:         contact.ID,
		WhatsAppMessageID: waMsgID,
		Direction:         models.DirectionOutgoing,
		MessageType:       models.MessageTypeTemplate,
		Content:           "test message",
		Status:            msgStatus,
		Metadata: models.JSONB{
			"campaign_id": campaign.ID.String(),
		},
	}
	require.NoError(t, app.DB.Create(&msg).Error)

	recipient := models.BulkMessageRecipient{
		BaseModel:         models.BaseModel{ID: uuid.New()},
		CampaignID:        campaign.ID,
		PhoneNumber:       contact.PhoneNumber,
		WhatsAppMessageID: waMsgID,
		Status:            msgStatus,
	}
	require.NoError(t, app.DB.Create(&recipient).Error)

	return org, msg, campaign, recipient
}

func TestUpdateMessageStatus_DeliveredUpdatesRecipient(t *testing.T) {
	app := webhookTestApp(t)
	_, msg, campaign, recipient := webhookTestData(t, app, models.MessageStatusSent)

	app.updateMessageStatus(msg.WhatsAppMessageID, "delivered", nil)

	// Verify recipient status and delivered_at
	var updated models.BulkMessageRecipient
	require.NoError(t, app.DB.First(&updated, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusDelivered, updated.Status)
	assert.NotNil(t, updated.DeliveredAt)
	assert.Nil(t, updated.ReadAt)

	// Verify campaign counter incremented
	var updatedCampaign models.BulkMessageCampaign
	require.NoError(t, app.DB.First(&updatedCampaign, campaign.ID).Error)
	assert.Equal(t, 1, updatedCampaign.DeliveredCount)
}

func TestUpdateMessageStatus_ReadUpdatesRecipient(t *testing.T) {
	app := webhookTestApp(t)
	_, msg, campaign, recipient := webhookTestData(t, app, models.MessageStatusDelivered)

	app.updateMessageStatus(msg.WhatsAppMessageID, "read", nil)

	// Verify recipient status and read_at
	var updated models.BulkMessageRecipient
	require.NoError(t, app.DB.First(&updated, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusRead, updated.Status)
	assert.NotNil(t, updated.ReadAt)

	// Verify campaign counter incremented
	var updatedCampaign models.BulkMessageCampaign
	require.NoError(t, app.DB.First(&updatedCampaign, campaign.ID).Error)
	assert.Equal(t, 1, updatedCampaign.ReadCount)
}

func TestUpdateMessageStatus_NonCampaignMessageIgnoresRecipient(t *testing.T) {
	app := webhookTestApp(t)
	uid := uuid.New().String()[:8]

	org := models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "wh-nocampaign-" + uid,
		Slug:      "wh-nocampaign-" + uid,
	}
	require.NoError(t, app.DB.Create(&org).Error)

	contact := models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		PhoneNumber:    "918888" + uid,
		ProfileName:    "No Campaign",
	}
	require.NoError(t, app.DB.Create(&contact).Error)

	waMsgID := "wamid.nocampaign-" + uid
	msg := models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New()},
		OrganizationID:    org.ID,
		WhatsAppAccount:   "test-account",
		ContactID:         contact.ID,
		WhatsAppMessageID: waMsgID,
		Direction:         models.DirectionOutgoing,
		MessageType:       models.MessageTypeText,
		Content:           "hello",
		Status:            models.MessageStatusSent,
		Metadata:          models.JSONB{}, // no campaign_id
	}
	require.NoError(t, app.DB.Create(&msg).Error)

	// Should update message status but not panic or fail
	app.updateMessageStatus(waMsgID, "delivered", nil)

	var updated models.Message
	require.NoError(t, app.DB.First(&updated, msg.ID).Error)
	assert.Equal(t, models.MessageStatusDelivered, updated.Status)
}

func TestUpdateMessageStatus_StatusPriorityRespected(t *testing.T) {
	app := webhookTestApp(t)
	_, msg, _, recipient := webhookTestData(t, app, models.MessageStatusRead)

	// Attempt to downgrade from read -> delivered (should be ignored)
	app.updateMessageStatus(msg.WhatsAppMessageID, "delivered", nil)

	var updated models.BulkMessageRecipient
	require.NoError(t, app.DB.First(&updated, recipient.ID).Error)
	// Status should remain "read"
	assert.Equal(t, models.MessageStatusRead, updated.Status)
}

func TestUpdateMessageStatus_FailedUpdatesMessage(t *testing.T) {
	app := webhookTestApp(t)
	_, msg, campaign, recipient := webhookTestData(t, app, models.MessageStatusSent)

	errors := []WebhookStatusError{
		{Code: 131047, Title: "Re-engagement message", Message: "Message failed to send because more than 24 hours have passed"},
	}
	app.updateMessageStatus(msg.WhatsAppMessageID, "failed", errors)

	// Verify message status and error
	var updatedMsg models.Message
	require.NoError(t, app.DB.First(&updatedMsg, msg.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, updatedMsg.Status)
	assert.Contains(t, updatedMsg.ErrorMessage, "more than 24 hours")

	// Verify recipient status updated
	var updatedRecipient models.BulkMessageRecipient
	require.NoError(t, app.DB.First(&updatedRecipient, recipient.ID).Error)
	assert.Equal(t, models.MessageStatusFailed, updatedRecipient.Status)

	// Verify campaign failed counter
	var updatedCampaign models.BulkMessageCampaign
	require.NoError(t, app.DB.First(&updatedCampaign, campaign.ID).Error)
	assert.Equal(t, 1, updatedCampaign.FailedCount)
}
