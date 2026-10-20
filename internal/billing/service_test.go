package billing_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/billing"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/database"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeProvider struct {
	charge  billing.Charge
	status  string
	amount  int64
	fail    bool
	creates int
}

func (f *fakeProvider) Cancel(ctx context.Context, id string) (billing.Payment, error) {
	f.status = "canceled"
	return f.Lookup(ctx, id)
}

func (f *fakeProvider) Create(_ context.Context, c billing.Charge) (billing.Payment, error) {
	f.creates++
	if f.charge.Reference != "" && f.charge != c {
		panic("idempotent retry changed its payload")
	}
	f.charge = c
	if f.fail {
		return billing.Payment{}, billing.ErrUnavailable
	}
	return f.Lookup(context.Background(), "1234")
}
func (f *fakeProvider) Lookup(_ context.Context, id string) (billing.Payment, error) {
	if f.fail {
		return billing.Payment{}, billing.ErrUnavailable
	}
	return billing.Payment{ID: id, Reference: f.charge.Reference, Amount: f.amount, Currency: "BRL", Status: f.status, Code: "fake-pix", ExpiresAt: f.charge.ExpiresAt}, nil
}

func TestPaymentLifecycle(t *testing.T) {
	dsn := os.Getenv("SAAS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SAAS_TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(db))
	for _, scenario := range []string{"renew_once", "wrong_amount", "contingency", "lost_response", "changed_subscription", "wrong_org", "disabled", "cancel_reissue", "paid_before_cancel", "automatic_reconciliation", "webhook_duplicate", "webhook_wrong_amount", "webhook_lost_response"} {
		t.Run(scenario, func(t *testing.T) {
			tx := db.Begin()
			require.NoError(t, tx.Error)
			defer tx.Rollback()
			org := models.Organization{Name: "Billing test", Slug: uuid.NewString()}
			require.NoError(t, tx.Create(&org).Error)
			payer := models.User{OrganizationID: org.ID, Email: uuid.NewString() + "@example.invalid"}
			require.NoError(t, tx.Create(&payer).Error)
			plan := models.SaaSPlan{Name: "Paid", PriceCents: 9900, Currency: "BRL", IntervalMonths: 1, Active: true}
			require.NoError(t, tx.Create(&plan).Error)
			expiry := time.Date(2028, 1, 31, 12, 0, 0, 0, time.UTC)
			sub := models.SaaSSubscription{OrganizationID: org.ID, PlanID: plan.ID, Status: "active", ExpiresAt: expiry}
			require.NoError(t, tx.Omit("Plan", "Organization").Create(&sub).Error)
			fake := &fakeProvider{status: "pending", amount: 9900}
			gateway := ""
			s := &billing.Service{DB: tx, Config: config.PaymentsConfig{Enabled: true, MPNotificationURL: "https://billing.example.invalid/original"}, Factory: func(name string) (billing.Provider, error) { gateway = name; return fake, nil }}
			if scenario == "disabled" {
				s.Config.Enabled = false
				_, err := s.Create(context.Background(), org.ID, payer.ID, expiry)
				require.Error(t, err)
				require.Zero(t, fake.creates)
				return
			}
			if scenario == "lost_response" || scenario == "webhook_lost_response" {
				fake.fail = true
			}
			invoice, err := s.Create(context.Background(), org.ID, payer.ID, expiry)
			if scenario == "webhook_lost_response" {
				require.ErrorIs(t, err, billing.ErrUnavailable)
				fake.fail = false
				fake.status = "paid"
				require.NoError(t, s.ProcessMPNotification(context.Background(), "1234"))
				require.NoError(t, s.ProcessMPNotification(context.Background(), "1234"))
				require.NoError(t, tx.First(&sub, "id = ?", sub.ID).Error)
				require.Equal(t, "2028-02-29", sub.ExpiresAt.Format("2006-01-02"))
				require.Equal(t, 1, fake.creates)
				return
			} else if scenario == "lost_response" {
				require.ErrorIs(t, err, billing.ErrUnavailable)
				require.Equal(t, "unknown", invoice.Status)
				fake.fail = false
				s.Config.MPNotificationURL = "https://billing.example.invalid/changed"
				invoice, err = s.Create(context.Background(), org.ID, payer.ID, expiry)
				require.NoError(t, err)
				require.Equal(t, 2, fake.creates)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, "pending", invoice.Status)
			if scenario == "cancel_reissue" {
				_, err := s.Reissue(context.Background(), org.ID, payer.ID, invoice.ID)
				require.ErrorIs(t, err, billing.ErrSubscription)
				canceled, err := s.Cancel(context.Background(), org.ID, invoice.ID)
				require.NoError(t, err)
				require.Equal(t, "canceled", canceled.Status)
				fake.charge = billing.Charge{}
				fake.status = "pending"
				child, err := s.Reissue(context.Background(), org.ID, payer.ID, invoice.ID)
				require.NoError(t, err)
				require.NotEqual(t, invoice.ID, child.ID)
				again, err := s.Reissue(context.Background(), org.ID, payer.ID, invoice.ID)
				require.NoError(t, err)
				require.Equal(t, child.ID, again.ID)
				require.Equal(t, 2, fake.creates)
				return
			}
			fake.status = "paid"
			if scenario == "wrong_amount" || scenario == "webhook_wrong_amount" {
				fake.amount = 1
			}
			if scenario == "contingency" {
				s.Config.EFIContingency = true
			}
			if scenario == "changed_subscription" {
				require.NoError(t, tx.Model(&sub).Update("status", "suspended").Error)
			}
			if scenario == "wrong_org" {
				other := models.Organization{Name: "Other", Slug: uuid.NewString()}
				require.NoError(t, tx.Create(&other).Error)
				_, err := s.Sync(context.Background(), other.ID, invoice.ID, true)
				require.ErrorIs(t, err, gorm.ErrRecordNotFound)
				require.NoError(t, tx.First(&sub, "id = ?", sub.ID).Error)
				require.True(t, sub.ExpiresAt.Equal(expiry))
				return
			}
			var result models.SaaSInvoice
			if scenario == "automatic_reconciliation" {
				require.NoError(t, tx.Model(&invoice).Update("last_checked_at", time.Now().Add(-2*time.Minute)).Error)
				s.Reconcile(context.Background())
				require.NoError(t, tx.First(&result, "id = ?", invoice.ID).Error)
			} else if scenario == "webhook_duplicate" || scenario == "webhook_wrong_amount" {
				err = s.ProcessMPNotification(context.Background(), "1234")
				if scenario == "webhook_duplicate" {
					require.NoError(t, err)
					require.NoError(t, s.ProcessMPNotification(context.Background(), "1234"))
				}
				require.NoError(t, tx.First(&result, "id = ?", invoice.ID).Error)
			} else if scenario == "paid_before_cancel" {
				result, err = s.Cancel(context.Background(), org.ID, invoice.ID)
			} else {
				result, err = s.Sync(context.Background(), org.ID, invoice.ID, false)
			}
			if scenario == "wrong_amount" || scenario == "webhook_wrong_amount" {
				require.ErrorIs(t, err, billing.ErrMismatch)
				require.Equal(t, "review", result.Status)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, tx.First(&sub, "id = ?", sub.ID).Error)
			if scenario == "wrong_amount" || scenario == "webhook_wrong_amount" || scenario == "changed_subscription" {
				require.True(t, sub.ExpiresAt.Equal(expiry))
				return
			}
			require.Equal(t, "paid", result.Status)
			require.Equal(t, "2028-02-29", sub.ExpiresAt.Format("2006-01-02"))
			if scenario == "contingency" {
				require.Equal(t, "mercadopago", gateway)
			}
			// Repeated button clicks carrying the original cycle reuse the paid invoice.
			again, err := s.Create(context.Background(), org.ID, payer.ID, expiry)
			require.NoError(t, err)
			require.Equal(t, invoice.ID, again.ID)
			_, err = s.Sync(context.Background(), org.ID, invoice.ID, false)
			require.NoError(t, err)
			require.NoError(t, tx.First(&sub, "id = ?", sub.ID).Error)
			require.Equal(t, "2028-02-29", sub.ExpiresAt.Format("2006-01-02"))
			var count int64
			require.NoError(t, tx.Model(&models.SaaSInvoice{}).Where("organization_id = ?", org.ID).Count(&count).Error)
			require.Equal(t, int64(1), count)
		})
	}
}
