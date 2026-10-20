// Package billing implements PIX payments. Only authoritative provider lookups
// can settle an invoice; browser requests never supply amounts or paid status.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("saas.payment_unavailable")
var ErrMismatch = errors.New("saas.payment_mismatch")

// ProviderError exposes only controlled translation keys, never provider text.
type ProviderError struct{ Key string }

func (e *ProviderError) Error() string { return e.Key }
func (e *ProviderError) Unwrap() error { return ErrUnavailable }

type Charge struct {
	NotificationURL string
	Reference       string
	Amount          int64
	Email           string
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

type Payment struct {
	ID        string
	Reference string
	Amount    int64
	Currency  string
	Status    string // pending, paid, expired, canceled, review
	Code      string
	ExpiresAt time.Time
}

type Provider interface {
	Create(context.Context, Charge) (Payment, error)
	Lookup(context.Context, string) (Payment, error)
}

var decimalPattern = regexp.MustCompile(`^[0-9]{1,10}(\.[0-9]{1,2})?$`)

func cents(s string) (int64, error) {
	if !decimalPattern.MatchString(s) {
		return 0, ErrMismatch
	}
	p := strings.SplitN(s, ".", 2)
	n, _ := strconv.ParseInt(p[0], 10, 64)
	if len(p) == 1 {
		return n * 100, nil
	}
	fraction, _ := strconv.ParseInt((p[1] + "0")[:2], 10, 64)
	return n*100 + fraction, nil
}

func decimal(n int64) string { return fmt.Sprintf("%d.%02d", n/100, n%100) }

// decode never returns provider bodies or credentials to callers/logs.
func decode(client *http.Client, req *http.Request, out any) (int, error) {
	res, err := client.Do(req)
	if err != nil {
		return 0, ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		key := "saas.payment_unavailable"
		switch res.StatusCode {
		case 401, 403:
			key = "saas.payment_credentials"
		case 400, 422:
			key = "saas.payment_invalid_request"
		case 429:
			key = "saas.payment_rate_limited"
		}
		// Mercado Pago error 23 identifies an invalid expiration format/value.
		if req.URL.Host == "api.mercadopago.com" || req.URL.Path == "/v1/payments" {
			var body struct {
				Cause []struct {
					Code json.RawMessage `json:"code"`
				} `json:"cause"`
			}
			if json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&body) == nil {
				for _, cause := range body.Cause {
					if string(cause.Code) == "23" || string(cause.Code) == `"23"` {
						key = "saas.payment_expiration"
					}
				}
			}
		}
		return res.StatusCode, &ProviderError{Key: key}
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(out); err != nil {
		return res.StatusCode, ErrUnavailable
	}
	return res.StatusCode, nil
}
