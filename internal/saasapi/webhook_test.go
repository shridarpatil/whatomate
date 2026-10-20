package saasapi_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"github.com/zerodha/logf"
)

func TestWebhookFailsClosed(t *testing.T) {
	for _, scenario := range []string{"missing_secret", "forged", "duplicate_id", "stale", "provider_unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			a := &handlers.App{Config: &config.Config{Payments: config.PaymentsConfig{MPWebhookSecret: "test-secret"}}, Log: logf.New(logf.Opts{})}
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/api/billing/webhooks/mercadopago?data.id=123")
			ctx.Request.Header.SetMethod("POST")
			ctx.Request.SetBodyString(`{"data":{"id":"999"},"status":"approved","transaction_amount":1}`)
			now := time.Now()
			if scenario == "stale" {
				now = now.Add(-time.Hour)
			}
			ts := strconv.FormatInt(now.Unix(), 10)
			mac := hmac.New(sha256.New, []byte("test-secret"))
			mac.Write([]byte("id:123;request-id:req-123;ts:" + ts + ";"))
			ctx.Request.Header.Set("x-signature", "ts="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
			ctx.Request.Header.Set("x-request-id", "req-123")
			status := 401
			switch scenario {
			case "missing_secret":
				a.Config.Payments.MPWebhookSecret = ""
				status = 503
			case "forged":
				ctx.Request.Header.Set("x-signature", "ts="+ts+",v1=bad")
			case "duplicate_id":
				ctx.Request.URI().QueryArgs().Add("data.id", "999")
			case "provider_unavailable":
				status = 503
			}
			require.NoError(t, a.MercadoPagoWebhook(&fastglue.Request{RequestCtx: ctx}))
			require.Equal(t, status, ctx.Response.StatusCode())
		})
	}
}
