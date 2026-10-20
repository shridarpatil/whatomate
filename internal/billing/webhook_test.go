package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMPWebhookAuthentication(t *testing.T) {
	now := time.Unix(1792000000, 0)
	for _, timestamp := range []string{strconv.FormatInt(now.Unix(), 10), strconv.FormatInt(now.UnixMilli(), 10)} {
		mac := hmac.New(sha256.New, []byte("test-secret"))
		mac.Write([]byte("id:12345;request-id:request-abc;ts:" + timestamp + ";"))
		signature := "ts=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
		require.True(t, VerifyMPWebhook("test-secret", signature, "request-abc", "12345", now))
		require.False(t, VerifyMPWebhook("wrong", signature, "request-abc", "12345", now))
		require.False(t, VerifyMPWebhook("", signature, "request-abc", "12345", now))
		require.False(t, VerifyMPWebhook("test-secret", signature, "other-request", "12345", now))
		require.False(t, VerifyMPWebhook("test-secret", signature, "request-abc", "12346", now))
		require.False(t, VerifyMPWebhook("test-secret", signature, "request-abc", "12345", now.Add(11*time.Minute)))
		require.False(t, VerifyMPWebhook("test-secret", signature, "request-abc", "12345", now.Add(-11*time.Minute)))
		require.False(t, VerifyMPWebhook("test-secret", signature+",ts="+timestamp, "request-abc", "12345", now))
	}
	for _, malformed := range []string{"", "v1=bad", "ts=abcdefghij,v1=00", "ts=1792000000,v1=not-hex"} {
		require.False(t, VerifyMPWebhook("test-secret", malformed, "request-abc", "12345", now))
	}
}
