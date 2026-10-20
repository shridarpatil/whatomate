package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/shridarpatil/whatomate/internal/billing"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

func (a *App) paymentService() *billing.Service {
	var cfg config.PaymentsConfig
	if a.Config != nil {
		cfg = a.Config.Payments
	}
	return &billing.Service{DB: a.DB, Config: cfg}
}

func paymentError(r *fastglue.Request, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.SendErrorEnvelope(404, "saas.invoice_not_found", nil, "")
	}
	if errors.Is(err, billing.ErrSubscription) {
		return r.SendErrorEnvelope(409, err.Error(), nil, "")
	}
	if errors.Is(err, billing.ErrUnavailable) || errors.Is(err, billing.ErrMismatch) {
		return r.SendErrorEnvelope(502, err.Error(), nil, "")
	}
	return r.SendErrorEnvelope(500, "saas.save_failed", nil, "")
}

func (a *App) ListSaaSInvoices(r *fastglue.Request) error {
	org, _, err := a.requireAuth(r, models.ResourceSettingsGeneral, models.ActionRead)
	if err != nil {
		return nil
	}
	invoices := []models.SaaSInvoice{}
	if err := a.DB.Where("organization_id = ?", org).Order("created_at DESC").Limit(100).Find(&invoices).Error; err != nil {
		return paymentError(r, err)
	}
	return r.SendEnvelope(invoices)
}

func (a *App) CreateSaaSInvoice(r *fastglue.Request) error {
	org, user, err := a.requireAuth(r, models.ResourceSettingsGeneral, models.ActionWrite)
	if err != nil {
		return nil
	}
	var input struct {
		ExpiresAt time.Time `json:"subscription_expires_at"`
	}
	if json.Unmarshal(r.RequestCtx.PostBody(), &input) != nil || input.ExpiresAt.IsZero() {
		return r.SendErrorEnvelope(400, "saas.invalid_request", nil, "")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	invoice, err := a.paymentService().Create(ctx, org, user, input.ExpiresAt)
	if err != nil {
		return paymentError(r, err)
	}
	return r.SendEnvelope(invoice)
}

func (a *App) RefreshSaaSInvoice(r *fastglue.Request) error {
	org, _, err := a.requireAuth(r, models.ResourceSettingsGeneral, models.ActionWrite)
	if err != nil {
		return nil
	}
	id, err := parsePathUUID(r, "id", "invoice")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	invoice, err := a.paymentService().Sync(ctx, org, id, true)
	if err != nil {
		return paymentError(r, err)
	}
	return r.SendEnvelope(invoice)
}

func (a *App) RunPaymentReconciliation(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	service := a.paymentService()
	service.Reconcile(ctx) // Recover outstanding invoices immediately after restart.
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.Reconcile(ctx)
		}
	}
}

func (a *App) CancelSaaSInvoice(r *fastglue.Request) error {
	org, _, err := a.requireAuth(r, models.ResourceSettingsGeneral, models.ActionWrite)
	if err != nil {
		return nil
	}
	id, err := parsePathUUID(r, "id", "invoice")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	invoice, err := a.paymentService().Cancel(ctx, org, id)
	if err != nil {
		return paymentError(r, err)
	}
	return r.SendEnvelope(invoice)
}

func (a *App) ReissueSaaSInvoice(r *fastglue.Request) error {
	org, user, err := a.requireAuth(r, models.ResourceSettingsGeneral, models.ActionWrite)
	if err != nil {
		return nil
	}
	id, err := parsePathUUID(r, "id", "invoice")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	invoice, err := a.paymentService().Reissue(ctx, org, user, id)
	if err != nil {
		return paymentError(r, err)
	}
	return r.SendEnvelope(invoice)
}
