package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

type MercadoPago struct {
	Token   string
	Client  *http.Client
	BaseURL string
}

type mpPayment struct {
	ID          json.Number `json:"id"`
	Reference   string      `json:"external_reference"`
	Amount      json.Number `json:"transaction_amount"`
	Refunded    json.Number `json:"transaction_amount_refunded"`
	Currency    string      `json:"currency_id"`
	Status      string      `json:"status"`
	Method      string      `json:"payment_method_id"`
	ExpiresAt   time.Time   `json:"date_of_expiration"`
	Interaction struct {
		Data struct {
			Code string `json:"qr_code"`
		} `json:"transaction_data"`
	} `json:"point_of_interaction"`
}

func (m *MercadoPago) request(ctx context.Context, method, path string, body any, key string) (Payment, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return Payment{}, ErrUnavailable
	}
	base := m.BaseURL
	if base == "" {
		base = "https://api.mercadopago.com"
	}
	var requestBody io.Reader
	if body != nil {
		requestBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, requestBody)
	if err != nil {
		return Payment{}, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+m.Token)
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-Idempotency-Key", key)
	}
	var raw mpPayment
	if _, err := decode(m.Client, req, &raw); err != nil {
		return Payment{}, err
	}
	amount, err := cents(string(raw.Amount))
	if err != nil {
		return Payment{}, err
	}
	if raw.ID == "" || raw.Method != "pix" {
		return Payment{}, ErrMismatch
	}
	status := "pending"
	switch raw.Status {
	case "approved":
		status = "paid"
	case "cancelled", "rejected":
		status = "canceled"
	case "refunded", "charged_back":
		status = "review"
	case "pending", "in_process", "authorized":
	default:
		status = "review"
	}
	if raw.Refunded != "" {
		refunded, err := cents(string(raw.Refunded))
		if err != nil {
			return Payment{}, err
		}
		if refunded > 0 {
			status = "review"
		}
	}
	return Payment{ID: string(raw.ID), Reference: raw.Reference, Amount: amount, Currency: raw.Currency, Status: status, Code: raw.Interaction.Data.Code, ExpiresAt: raw.ExpiresAt}, nil
}

func (m *MercadoPago) Create(ctx context.Context, c Charge) (Payment, error) {
	body := map[string]any{
		"transaction_amount": json.Number(decimal(c.Amount)), "external_reference": c.Reference,
		"description": "Whatomate - " + c.Reference, "payment_method_id": "pix",
		// Mercado Pago rejects the RFC3339 UTC shorthand "Z" here. Use the
		// explicit offset and millisecond precision accepted by its Payments API.
		"date_of_expiration": c.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000-07:00"),
		"payer":              map[string]string{"email": c.Email},
	}
	if c.NotificationURL != "" {
		body["notification_url"] = c.NotificationURL
	}
	return m.request(ctx, "POST", "/v1/payments", body, c.Reference)
}

func (m *MercadoPago) Lookup(ctx context.Context, id string) (Payment, error) {
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(id) {
		return Payment{}, ErrMismatch
	}
	return m.request(ctx, "GET", "/v1/payments/"+url.PathEscape(id), nil, "")
}

func (m *MercadoPago) Cancel(ctx context.Context, id string) (Payment, error) {
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(id) {
		return Payment{}, ErrMismatch
	}
	_, err := m.request(ctx, "PUT", "/v1/payments/"+id, map[string]string{"status": "cancelled"}, "")
	if err != nil {
		return Payment{}, err
	}
	return m.Lookup(ctx, id)
}

// Recover finds a payment whose creation response was lost, without issuing
// another charge. Multiple matches require operator review.
func (m *MercadoPago) Recover(ctx context.Context, reference string) (Payment, error) {
	base := m.BaseURL
	if base == "" {
		base = "https://api.mercadopago.com"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/v1/payments/search?external_reference="+url.QueryEscape(reference)+"&limit=2", nil)
	if err != nil {
		return Payment{}, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+m.Token)
	var result struct {
		Results []struct {
			ID json.Number `json:"id"`
		} `json:"results"`
	}
	if _, err := decode(m.Client, req, &result); err != nil {
		return Payment{}, err
	}
	if len(result.Results) == 0 {
		return Payment{}, ErrUnavailable
	}
	if len(result.Results) != 1 {
		return Payment{}, ErrMismatch
	}
	return m.Lookup(ctx, string(result.Results[0].ID))
}
