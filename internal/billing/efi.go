package billing

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shridarpatil/whatomate/internal/config"
)

type EFI struct {
	Client                            *http.Client
	BaseURL, ClientID, Secret, PixKey string
}

func NewEFI(c config.PaymentsConfig) (*EFI, error) {
	cert, err := os.ReadFile(c.EFICertificate)
	if err != nil {
		return nil, ErrUnavailable
	}
	keyPath := c.EFIPrivateKey
	if keyPath == "" {
		keyPath = c.EFICertificate
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, ErrUnavailable
	}
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, ErrUnavailable
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Instances reload certificate material for each operation; do not retain
	// idle TLS connections beyond this short-lived instance.
	transport.DisableKeepAlives = true
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}
	base := "https://pix.api.efipay.com.br"
	if c.EFISandbox {
		base = "https://pix-h.api.efipay.com.br"
	}
	return &EFI{Client: &http.Client{Transport: transport, Timeout: 20 * time.Second}, BaseURL: base, ClientID: c.EFIClientID, Secret: c.EFIClientSecret, PixKey: c.EFIPixKey}, nil
}

func (e *EFI) token(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", e.BaseURL+"/oauth/token", strings.NewReader(`{"grant_type":"client_credentials"}`))
	if err != nil {
		return "", ErrUnavailable
	}
	req.SetBasicAuth(e.ClientID, e.Secret)
	req.Header.Set("Content-Type", "application/json")
	var out struct {
		Token string `json:"access_token"`
	}
	if _, err := decode(e.Client, req, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", ErrUnavailable
	}
	return out.Token, nil
}

type efiCharge struct {
	ID     string `json:"txid"`
	Status string `json:"status"`
	Code   string `json:"pixCopiaECola"`
	Amount struct {
		Original string `json:"original"`
	} `json:"valor"`
	Calendar struct {
		Created    time.Time `json:"criacao"`
		Expiration int64     `json:"expiracao"`
	} `json:"calendario"`
	Pix []struct {
		Amount  string            `json:"valor"`
		Refunds []json.RawMessage `json:"devolucoes"`
	} `json:"pix"`
}

func (e *EFI) request(ctx context.Context, method, id string, body any, token string) (Payment, int, error) {
	// A UUID without separators is a valid 32-character Efí txid.
	if len(id) != 32 {
		return Payment{}, 0, ErrMismatch
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return Payment{}, 0, ErrMismatch
		}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return Payment{}, 0, ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, method, e.BaseURL+"/v2/cob/"+id, bytes.NewReader(b))
	if err != nil {
		return Payment{}, 0, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	var raw efiCharge
	code, err := decode(e.Client, req, &raw)
	if err != nil {
		return Payment{}, code, err
	}
	amount, err := cents(raw.Amount.Original)
	if err != nil {
		return Payment{}, code, err
	}
	if raw.ID != id {
		return Payment{}, code, ErrMismatch
	}
	status := "pending"
	switch raw.Status {
	case "CONCLUIDA":
		var total int64
		for _, pix := range raw.Pix {
			n, err := cents(pix.Amount)
			if err != nil || len(pix.Refunds) > 0 {
				return Payment{}, code, ErrMismatch
			}
			total += n
		}
		if total != amount {
			return Payment{}, code, ErrMismatch
		}
		status = "paid"
	case "ATIVA":
	case "REMOVIDA_PELO_USUARIO_RECEBEDOR", "REMOVIDA_PELO_PSP":
		status = "canceled"
	default:
		status = "review"
	}
	return Payment{ID: raw.ID, Reference: id, Amount: amount, Currency: "BRL", Status: status, Code: raw.Code, ExpiresAt: raw.Calendar.Created.Add(time.Duration(raw.Calendar.Expiration) * time.Second)}, code, nil
}

func (e *EFI) Create(ctx context.Context, c Charge) (Payment, error) {
	token, err := e.token(ctx)
	if err != nil {
		return Payment{}, err
	}
	id := strings.ReplaceAll(c.Reference, "-", "")
	// Recover an earlier request before attempting PUT; never overwrite a charge.
	p, code, err := e.request(ctx, "GET", id, nil, token)
	if err == nil {
		return p, nil
	}
	if code != http.StatusNotFound {
		return Payment{}, err
	}
	p, _, err = e.request(ctx, "PUT", id, map[string]any{
		"calendario": map[string]int64{"expiracao": int64(c.ExpiresAt.Sub(c.CreatedAt).Seconds())},
		"valor":      map[string]string{"original": decimal(c.Amount)}, "chave": e.PixKey,
		"solicitacaoPagador": "Whatomate - " + c.Reference,
	}, token)
	return p, err
}

func (e *EFI) Lookup(ctx context.Context, id string) (Payment, error) {
	token, err := e.token(ctx)
	if err != nil {
		return Payment{}, err
	}
	p, _, err := e.request(ctx, "GET", id, nil, token)
	return p, err
}

func (e *EFI) Cancel(ctx context.Context, id string) (Payment, error) {
	token, err := e.token(ctx)
	if err != nil {
		return Payment{}, err
	}
	_, _, err = e.request(ctx, "PATCH", id, map[string]string{"status": "REMOVIDA_PELO_USUARIO_RECEBEDOR"}, token)
	if err != nil {
		return Payment{}, err
	}
	p, _, err := e.request(ctx, "GET", id, nil, token)
	return p, err
}
