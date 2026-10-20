package billing

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrSubscription = errors.New("saas.payment_subscription")

type Service struct {
	DB      *gorm.DB
	Config  config.PaymentsConfig
	Factory func(string) (Provider, error)
}

func (s *Service) Gateway() string {
	if s.Config.EFIContingency {
		return "efi"
	}
	return "mercadopago"
}

func (s *Service) Provider(name string) (Provider, error) {
	if s.Factory != nil {
		return s.Factory(name)
	}
	switch name {
	case "mercadopago":
		if s.Config.MPAccessToken == "" {
			return nil, ErrUnavailable
		}
		return &MercadoPago{Token: s.Config.MPAccessToken, Client: &http.Client{Timeout: 20 * time.Second}}, nil
	case "efi":
		if s.Config.EFIClientID == "" || s.Config.EFIClientSecret == "" || s.Config.EFIPixKey == "" {
			return nil, ErrUnavailable
		}
		return NewEFI(s.Config)
	default:
		return nil, ErrUnavailable
	}
}

func (s *Service) Ready() bool {
	if !s.Config.Enabled {
		return false
	}
	_, err := s.Provider(s.Gateway())
	return err == nil
}

func lockOrganization(tx *gorm.DB, id uuid.UUID) error {
	var org models.Organization
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&org, "id = ?", id).Error
}

// Create commits the invoice before any network request. Retries and concurrent
// callers for a subscription cycle reuse its immutable payload and gateway.
func (s *Service) Create(ctx context.Context, org, user uuid.UUID, expectedExpiry time.Time) (models.SaaSInvoice, error) {
	return s.create(ctx, org, user, expectedExpiry, uuid.Nil)
}

// Reissue is explicit and only allowed after a provider-confirmed cancellation.
// Its predecessor ID makes repeated reissue requests idempotent as well.
func (s *Service) Reissue(ctx context.Context, org, user, previous uuid.UUID) (models.SaaSInvoice, error) {
	var invoice models.SaaSInvoice
	if err := s.DB.WithContext(ctx).Where("id = ? AND organization_id = ?", previous, org).First(&invoice).Error; err != nil {
		return invoice, err
	}
	return s.create(ctx, org, user, invoice.BaseExpiresAt, previous)
}

func (s *Service) create(ctx context.Context, org, user uuid.UUID, expectedExpiry time.Time, previous uuid.UUID) (models.SaaSInvoice, error) {
	var invoice models.SaaSInvoice
	if !s.Config.Enabled {
		return invoice, ErrUnavailable
	}
	if expectedExpiry.IsZero() {
		return invoice, ErrSubscription
	}
	cycle := org.String() + ":" + expectedExpiry.UTC().Format(time.RFC3339Nano)
	if previous != uuid.Nil {
		cycle += ":" + previous.String()
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockOrganization(tx, org); err != nil {
			return err
		}
		if previous != uuid.Nil {
			var prior models.SaaSInvoice
			if err := tx.Where("id = ? AND organization_id = ?", previous, org).First(&prior).Error; err != nil {
				return err
			}
			if prior.Status != "canceled" || prior.PaidAt != nil {
				return ErrSubscription
			}
		}
		err := tx.Where("organization_id = ? AND cycle = ?", org, cycle).First(&invoice).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var sub models.SaaSSubscription
		if err := tx.Preload("Plan").Where("organization_id = ?", org).First(&sub).Error; err != nil {
			return ErrSubscription
		}
		if !sub.ExpiresAt.Equal(expectedExpiry) || !sub.Plan.Active || sub.Plan.PriceCents <= 0 || sub.Plan.Currency != "BRL" || (sub.Status != "active" && sub.Status != "trialing") {
			return ErrSubscription
		}
		// An unresolved payment must be reconciled before changing commercial terms.
		var count int64
		if err := tx.Model(&models.SaaSInvoice{}).Where("organization_id = ? AND status IN ?", org, []string{"creating", "unknown", "pending", "review"}).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return ErrSubscription
		}
		var payer models.User
		if err := tx.First(&payer, "id = ?", user).Error; err != nil {
			return err
		}
		address, err := mail.ParseAddress(payer.Email)
		if err != nil || address.Address != payer.Email {
			return ErrSubscription
		}
		if _, err := s.Provider(s.Gateway()); err != nil {
			return err
		}
		now := time.Now().UTC().Truncate(time.Second)
		notificationURL := ""
		if s.Gateway() == "mercadopago" {
			notificationURL = strings.TrimSpace(s.Config.MPNotificationURL)
			if notificationURL != "" {
				u, err := url.Parse(notificationURL)
				if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
					return ErrUnavailable
				}
			}
		}
		invoice = models.SaaSInvoice{BaseModel: models.BaseModel{ID: uuid.New(), CreatedAt: now},
			NotificationURL: notificationURL,
			OrganizationID:  org, SubscriptionID: sub.ID, PlanID: sub.PlanID, Cycle: cycle,
			PriceCents: sub.Plan.PriceCents, Currency: sub.Plan.Currency, IntervalMonths: sub.Plan.IntervalMonths,
			Provider: s.Gateway(), PayerEmail: payer.Email, Status: "creating", BaseExpiresAt: sub.ExpiresAt,
			ExpiresAt: now.Add(30 * time.Minute), RequestedExpiresAt: now.Add(30 * time.Minute), LastCheckedAt: now}
		return tx.Create(&invoice).Error
	})
	if err != nil {
		return invoice, err
	}
	return s.Sync(ctx, org, invoice.ID, true)
}

// AddMonths clamps to the last day instead of overflowing January 31 into March.
func AddMonths(t time.Time, months int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(months), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	last := first.AddDate(0, 1, -1).Day()
	day := t.Day()
	if day > last {
		day = last
	}
	return first.AddDate(0, 0, day-1)
}

func (s *Service) Sync(ctx context.Context, org, id uuid.UUID, allowCreate bool) (models.SaaSInvoice, error) {
	return s.sync(ctx, org, id, allowCreate, false, "")
}

func (s *Service) Cancel(ctx context.Context, org, id uuid.UUID) (models.SaaSInvoice, error) {
	return s.sync(ctx, org, id, false, true, "")
}

func (s *Service) sync(ctx context.Context, org, id uuid.UUID, allowCreate, cancelPayment bool, notifiedPaymentID string) (models.SaaSInvoice, error) {
	var invoice models.SaaSInvoice
	var providerErr error
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Same lock order as manual assignments, across all application replicas.
		if err := lockOrganization(tx, org); err != nil {
			return err
		}
		if err := tx.Where("id = ? AND organization_id = ?", id, org).First(&invoice).Error; err != nil {
			return err
		}
		if invoice.PaidAt != nil || invoice.Status == "canceled" || invoice.Status == "review" {
			return nil
		}
		provider, err := s.Provider(invoice.Provider)
		var payment Payment
		if err == nil {
			if notifiedPaymentID != "" {
				payment, err = provider.Lookup(ctx, notifiedPaymentID)
			} else if invoice.PaymentID != "" {
				payment, err = provider.Lookup(ctx, invoice.PaymentID)
			} else if invoice.Provider == "efi" && (!allowCreate || !s.Config.Enabled || !time.Now().Before(invoice.RequestedExpiresAt)) {
				payment, err = provider.Lookup(ctx, strings.ReplaceAll(invoice.ID.String(), "-", ""))
			} else if allowCreate && s.Config.Enabled && time.Now().Before(invoice.RequestedExpiresAt) {
				payment, err = provider.Create(ctx, Charge{Reference: invoice.ID.String(), Amount: invoice.PriceCents, Email: invoice.PayerEmail, CreatedAt: invoice.CreatedAt, ExpiresAt: invoice.RequestedExpiresAt, NotificationURL: invoice.NotificationURL})
			} else if recovery, ok := provider.(interface {
				Recover(context.Context, string) (Payment, error)
			}); ok {
				payment, err = recovery.Recover(ctx, invoice.ID.String())
			} else {
				err = ErrUnavailable
			}
		}
		expected := invoice.ID.String()
		if invoice.Provider == "efi" {
			expected = strings.ReplaceAll(expected, "-", "")
		}
		matches := func(p Payment) bool {
			return p.Reference == expected && p.Amount == invoice.PriceCents && p.Currency == invoice.Currency && p.ID != "" && (invoice.PaymentID == "" || p.ID == invoice.PaymentID) && (notifiedPaymentID == "" || p.ID == notifiedPaymentID)
		}
		if err == nil && !matches(payment) {
			err = ErrMismatch
		}
		if err == nil && cancelPayment && payment.Status == "pending" {
			if canceler, ok := provider.(interface {
				Cancel(context.Context, string) (Payment, error)
			}); ok {
				payment, err = canceler.Cancel(ctx, payment.ID)
			} else {
				err = ErrUnavailable
			}
		}
		invoice.LastCheckedAt = time.Now().UTC()
		if err != nil {
			providerErr = err
			invoice.ErrorCode = ErrUnavailable.Error()
			var detail *ProviderError
			if errors.As(err, &detail) {
				invoice.ErrorCode = detail.Key
			}
			if errors.Is(err, ErrMismatch) {
				invoice.ErrorCode = ErrMismatch.Error()
				invoice.Status = "review"
			} else if invoice.PaymentID == "" {
				invoice.Status = "unknown"
			}
			return tx.Save(&invoice).Error
		}
		if !matches(payment) {
			invoice.Status, invoice.ErrorCode, providerErr = "review", ErrMismatch.Error(), ErrMismatch
			return tx.Save(&invoice).Error
		}
		invoice.PaymentID, invoice.PixCode, invoice.Status, invoice.ErrorCode = payment.ID, payment.Code, payment.Status, ""
		if !payment.ExpiresAt.IsZero() {
			invoice.ExpiresAt = payment.ExpiresAt
		}
		if payment.Status == "paid" {
			now := time.Now().UTC()
			invoice.PaidAt = &now
			var sub models.SaaSSubscription
			if err := tx.First(&sub, "id = ? AND organization_id = ?", invoice.SubscriptionID, org).Error; err != nil {
				return err
			}
			if sub.PlanID != invoice.PlanID || !sub.ExpiresAt.Equal(invoice.BaseExpiresAt) || (sub.Status != "active" && sub.Status != "trialing") {
				invoice.Status, invoice.ErrorCode = "review", "saas.payment_review"
			} else {
				base := sub.ExpiresAt
				if base.Before(now) {
					base = now
				}
				sub.ExpiresAt, sub.Status = AddMonths(base, invoice.IntervalMonths), "active"
				if err := tx.Omit("Plan", "Organization").Save(&sub).Error; err != nil {
					return err
				}
			}
		}
		return tx.Save(&invoice).Error
	})
	if err != nil {
		return invoice, err
	}
	return invoice, providerErr
}

// Reconcile uses authenticated GET requests only, including after checkout has
// been disabled. Failed requests remain visible and never trigger failover.
func (s *Service) Reconcile(ctx context.Context) {
	var invoices []models.SaaSInvoice
	if err := s.DB.WithContext(ctx).Where("status IN ? AND last_checked_at < ?", []string{"creating", "unknown", "pending"}, time.Now().Add(-time.Minute)).Order("last_checked_at ASC").Limit(100).Find(&invoices).Error; err != nil {
		return
	}
	for _, invoice := range invoices {
		if ctx.Err() != nil {
			return
		}
		callCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		_, _ = s.Sync(callCtx, invoice.OrganizationID, invoice.ID, false)
		cancel()
	}
}
