package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
)

var paymentIDPattern = regexp.MustCompile(`^[0-9]{1,30}$`)
var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

// VerifyMPWebhook authenticates the URL data.id, not an unsigned body field.
// Seconds and milliseconds both occur in the official notification formats.
func VerifyMPWebhook(secret, signature, requestID, paymentID string, now time.Time) bool {
	if secret == "" || len(signature) > 512 || !paymentIDPattern.MatchString(paymentID) || !requestIDPattern.MatchString(requestID) {
		return false
	}
	values := map[string]string{}
	for _, part := range strings.Split(signature, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 || values[kv[0]] != "" {
			return false
		}
		values[kv[0]] = kv[1]
	}
	ts := values["ts"]
	if len(ts) != 10 && len(ts) != 13 {
		return false
	}
	n, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	stamp := time.Unix(n, 0)
	if len(ts) == 13 {
		stamp = time.UnixMilli(n)
	}
	if delta := now.Sub(stamp); delta > 10*time.Minute || delta < -10*time.Minute {
		return false
	}
	provided, err := hex.DecodeString(values["v1"])
	if err != nil || len(provided) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("id:" + paymentID + ";request-id:" + requestID + ";ts:" + ts + ";"))
	return hmac.Equal(mac.Sum(nil), provided)
}

// ProcessMPNotification treats notifications as hints only. The authenticated
// provider API supplies the reference and settlement status. No browser session,
// tenant header, body amount or body status can affect the target invoice.
func (s *Service) ProcessMPNotification(ctx context.Context, paymentID string) error {
	if !paymentIDPattern.MatchString(paymentID) {
		return ErrMismatch
	}
	provider, err := s.Provider("mercadopago")
	if err != nil {
		return err
	}
	payment, err := provider.Lookup(ctx, paymentID)
	if err != nil {
		return err
	}
	if payment.ID != paymentID {
		return ErrMismatch
	}
	id, err := uuid.Parse(payment.Reference)
	if err != nil {
		return nil
	} // Other product using the same merchant account.
	var invoice models.SaaSInvoice
	err = s.DB.WithContext(ctx).Where("id = ? AND provider = ?", id, "mercadopago").First(&invoice).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.sync(ctx, invoice.OrganizationID, invoice.ID, false, false, paymentID)
	return err
}
