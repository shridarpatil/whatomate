package saas

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type PlanInput struct {
	Name           string `json:"name"`
	PriceCents     int64  `json:"price_cents"`
	Currency       string `json:"currency"`
	IntervalMonths int    `json:"interval_months"`
	TrialDays      int    `json:"trial_days"`
	UserLimit      int    `json:"user_limit"`
	AccountLimit   int    `json:"account_limit"`
	TeamLimit      int    `json:"team_limit"`
	Active         bool   `json:"active"`
	Public         bool   `json:"public"`
}

func (p *PlanInput) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 120 {
		return errors.New("saas.invalid_name")
	}
	if p.PriceCents < 0 || p.PriceCents > 100000000 {
		return errors.New("saas.invalid_price")
	}
	if p.Currency != "BRL" {
		return errors.New("saas.invalid_currency")
	}
	if p.IntervalMonths != 1 && p.IntervalMonths != 3 && p.IntervalMonths != 6 && p.IntervalMonths != 12 {
		return errors.New("saas.invalid_interval")
	}
	if p.TrialDays < 0 || p.TrialDays > 365 {
		return errors.New("saas.invalid_trial")
	}
	// -1 means unlimited, 0 means no allocation, positive values are limits.
	if p.UserLimit < -1 || p.AccountLimit < -1 || p.TeamLimit < -1 || p.UserLimit > 1000000 || p.AccountLimit > 1000000 || p.TeamLimit > 1000000 {
		return errors.New("saas.invalid_limit")
	}
	return nil
}

func ValidStatus(status string) bool {
	return status == "trialing" || status == "active" || status == "suspended" || status == "canceled"
}

// EffectiveStatus does not mutate stored state or grant access to operations.
func EffectiveStatus(status string, expiresAt, now time.Time) string {
	if (status == "active" || status == "trialing") && !expiresAt.After(now) {
		return "expired"
	}
	return status
}
