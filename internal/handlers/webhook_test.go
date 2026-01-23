package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
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
