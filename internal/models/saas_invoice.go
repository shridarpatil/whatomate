package models

import (
	"github.com/google/uuid"
	"time"
)

// SaaSInvoice snapshots commercial terms. A cycle can have only one invoice;
// uncertain requests are retried with the same provider and idempotency key.
type SaaSInvoice struct {
	BaseModel
	NotificationURL    string     `gorm:"type:text;not null;default:''" json:"-"`
	OrganizationID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
	SubscriptionID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"subscription_id"`
	PlanID             uuid.UUID  `gorm:"type:uuid;not null" json:"plan_id"`
	Cycle              string     `gorm:"size:160;not null;uniqueIndex" json:"-"`
	PriceCents         int64      `gorm:"not null" json:"price_cents"`
	Currency           string     `gorm:"size:3;not null" json:"currency"`
	IntervalMonths     int        `gorm:"not null" json:"interval_months"`
	Provider           string     `gorm:"size:24;not null" json:"provider"`
	PaymentID          string     `gorm:"size:120" json:"-"`
	PayerEmail         string     `gorm:"size:255;not null" json:"-"`
	Status             string     `gorm:"size:24;not null;index" json:"status"`
	PixCode            string     `gorm:"type:text" json:"pix_code"`
	ExpiresAt          time.Time  `gorm:"not null" json:"expires_at"`
	RequestedExpiresAt time.Time  `gorm:"not null" json:"-"`
	BaseExpiresAt      time.Time  `gorm:"not null" json:"-"`
	PaidAt             *time.Time `json:"paid_at"`
	LastCheckedAt      time.Time  `gorm:"index" json:"last_checked_at"`
	ErrorCode          string     `gorm:"size:80" json:"error_code"`
}

func (SaaSInvoice) TableName() string { return "saas_invoices" }
