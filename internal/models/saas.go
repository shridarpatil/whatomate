package models

import (
	"time"

	"github.com/google/uuid"
)

// SaaSPlan is a commercial catalog entry; amounts are integer minor units.
// Existing organizations are not enrolled automatically.
type SaaSPlan struct {
	BaseModel
	Name           string `gorm:"size:120;not null" json:"name"`
	PriceCents     int64  `gorm:"not null" json:"price_cents"`
	Currency       string `gorm:"size:3;not null" json:"currency"`
	IntervalMonths int    `gorm:"not null" json:"interval_months"`
	TrialDays      int    `gorm:"not null" json:"trial_days"`
	UserLimit      int    `gorm:"not null" json:"user_limit"`
	AccountLimit   int    `gorm:"not null" json:"account_limit"`
	TeamLimit      int    `gorm:"not null" json:"team_limit"`
	Active         bool   `json:"active"`
	Public         bool   `json:"public"`
}

// SaaSSubscription records a manual commercial assignment, not a payment.
type SaaSSubscription struct {
	BaseModel
	OrganizationID uuid.UUID    `gorm:"type:uuid;not null;uniqueIndex" json:"organization_id"`
	PlanID         uuid.UUID    `gorm:"type:uuid;not null;index" json:"plan_id"`
	Status         string       `gorm:"size:20;not null" json:"status"`
	ExpiresAt      time.Time    `gorm:"not null" json:"expires_at"`
	Organization   Organization `gorm:"foreignKey:OrganizationID" json:"organization"`
	Plan           SaaSPlan     `gorm:"foreignKey:PlanID" json:"plan"`
}

func (SaaSPlan) TableName() string         { return "saas_plans" }
func (SaaSSubscription) TableName() string { return "saas_subscriptions" }
