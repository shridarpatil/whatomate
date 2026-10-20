package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/shridarpatil/whatomate/internal/billing"
	"github.com/zerodha/fastglue"
)

func (a *App) MercadoPagoWebhook(r *fastglue.Request) error {
	if a.Config == nil || a.Config.Payments.MPWebhookSecret == "" {
		return r.SendErrorEnvelope(503, "Webhook not configured", nil, "")
	}
	if len(r.RequestCtx.PostBody()) > 65536 {
		return r.SendErrorEnvelope(413, "Payload too large", nil, "")
	}
	args := r.RequestCtx.QueryArgs()
	if len(args.PeekMulti("data.id")) != 1 {
		return r.SendErrorEnvelope(401, "Invalid signature", nil, "")
	}
	id := string(args.Peek("data.id"))
	if !billing.VerifyMPWebhook(a.Config.Payments.MPWebhookSecret,
		string(r.RequestCtx.Request.Header.Peek("x-signature")),
		string(r.RequestCtx.Request.Header.Peek("x-request-id")), id, time.Now()) {
		return r.SendErrorEnvelope(401, "Invalid signature", nil, "")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
	defer cancel()
	err := a.paymentService().ProcessMPNotification(ctx, id)
	if err != nil && !errors.Is(err, billing.ErrMismatch) {
		// Non-2xx asks Mercado Pago to retry. The reconciliation worker also
		// recovers after outages; never acknowledge a transient DB/API failure.
		a.Log.Warn("Mercado Pago webhook processing deferred")
		return r.SendErrorEnvelope(503, "Payment verification unavailable", nil, "")
	}
	// A mismatch is retained as review, without granting subscription time.
	return r.SendEnvelope(map[string]bool{"received": true})
}
