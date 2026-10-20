package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAmountsAndCalendar(t *testing.T) {
	for input, expected := range map[string]int64{"0": 0, "12.3": 1230, "99.99": 9999, "1000000.00": 100000000} {
		n, err := cents(input)
		require.NoError(t, err)
		require.Equal(t, expected, n)
		roundtrip, err := cents(decimal(n))
		require.NoError(t, err)
		require.Equal(t, n, roundtrip)
	}
	for _, input := range []string{"-1", "1.001", "1e2", "NaN", "1,00", ""} {
		_, err := cents(input)
		require.Error(t, err)
	}
	date := time.Date(2028, 1, 31, 12, 0, 0, 0, time.UTC)
	require.Equal(t, "2028-02-29", AddMonths(date, 1).Format("2006-01-02"))
	require.Equal(t, "2029-02-28", AddMonths(AddMonths(date, 1), 12).Format("2006-01-02"))
}

func TestMercadoPagoIdempotencyAndNormalization(t *testing.T) {
	c := Charge{Reference: "immutable-invoice", Amount: 9999, Email: "payer@example.invalid", ExpiresAt: time.Now().Add(time.Hour), NotificationURL: "https://billing.example.invalid/subscription/mercadopagowebhook"}
	var first string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
		if r.Method == "POST" {
			require.Equal(t, c.Reference, r.Header.Get("X-Idempotency-Key"))
			var body map[string]json.RawMessage
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "99.99", string(body["transaction_amount"]))
			var callback string
			require.NoError(t, json.Unmarshal(body["notification_url"], &callback))
			require.Equal(t, c.NotificationURL, callback)
			var expiration string
			require.NoError(t, json.Unmarshal(body["date_of_expiration"], &expiration))
			require.Equal(t, c.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000-07:00"), expiration)
			b, _ := json.Marshal(body)
			if first == "" {
				first = string(b)
			} else {
				require.Equal(t, first, string(b))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1234,"external_reference":"immutable-invoice","transaction_amount":99.99,"currency_id":"BRL","status":"approved","payment_method_id":"pix"}`))
	}))
	defer server.Close()
	p := &MercadoPago{Token: "test-secret", Client: server.Client(), BaseURL: server.URL}
	for i := 0; i < 2; i++ {
		payment, err := p.Create(context.Background(), c)
		require.NoError(t, err)
		require.Equal(t, "paid", payment.Status)
		require.Equal(t, int64(9999), payment.Amount)
	}
	_, err := p.Lookup(context.Background(), "../other")
	require.Error(t, err)
}

func TestEFIRecoversExistingWithoutNewCharge(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth/token" {
			_, _ = w.Write([]byte(`{"access_token":"test-only"}`))
			return
		}
		if r.Method == "PUT" {
			puts++
		}
		require.Equal(t, "Bearer test-only", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"txid":"` + id + `","status":"CONCLUIDA","valor":{"original":"10.00"},"pix":[{"valor":"10.00"}],"calendario":{"criacao":"2026-01-01T12:00:00Z","expiracao":1800}}`))
	}))
	defer server.Close()
	e := &EFI{Client: server.Client(), BaseURL: server.URL}
	p, err := e.Create(context.Background(), Charge{Reference: id, Amount: 1000})
	require.NoError(t, err)
	require.Equal(t, "paid", p.Status)
	require.Zero(t, puts)
}

func TestProviderErrorsDoNotExposeSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"token":"secret-value"}`))
	}))
	defer server.Close()
	p := &MercadoPago{Token: "secret-value", Client: server.Client(), BaseURL: server.URL}
	_, err := p.Lookup(context.Background(), "123")
	require.ErrorIs(t, err, ErrUnavailable)
	require.NotContains(t, err.Error(), "secret-value")
}

func TestMercadoPagoExpirationErrorIsSafeAndSpecific(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"message":"sensitive-provider-text","cause":[{"code":23}]}`))
	}))
	defer server.Close()
	p := &MercadoPago{Token: "secret", Client: server.Client(), BaseURL: server.URL}
	_, err := p.Create(context.Background(), Charge{Amount: 1000, ExpiresAt: time.Now()})
	require.ErrorIs(t, err, ErrUnavailable)
	require.Equal(t, "saas.payment_expiration", err.Error())
}
