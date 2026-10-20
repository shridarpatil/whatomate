package saas

import (
	"testing"
	"time"
)

func TestPlanValidation(t *testing.T) {
	valid := PlanInput{Name: "Basic", PriceCents: 9900, Currency: "BRL", IntervalMonths: 1, TrialDays: 7, UserLimit: 5, AccountLimit: 1, TeamLimit: -1}
	for _, tc := range []struct {
		name   string
		change func(*PlanInput)
		want   string
	}{
		{"valid", func(p *PlanInput) {}, ""},
		{"empty name", func(p *PlanInput) { p.Name = " " }, "saas.invalid_name"},
		{"negative money", func(p *PlanInput) { p.PriceCents = -1 }, "saas.invalid_price"},
		{"unsupported currency", func(p *PlanInput) { p.Currency = "USD" }, "saas.invalid_currency"},
		{"interval", func(p *PlanInput) { p.IntervalMonths = 2 }, "saas.invalid_interval"},
		{"trial", func(p *PlanInput) { p.TrialDays = 366 }, "saas.invalid_trial"},
		{"limits", func(p *PlanInput) { p.UserLimit = -2 }, "saas.invalid_limit"},
		{"zero allocation", func(p *PlanInput) { p.UserLimit = 0 }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := valid
			tc.change(&p)
			err := p.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestEffectiveStatus(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	for _, status := range []string{"active", "trialing"} {
		if got := EffectiveStatus(status, now, now); got != "expired" {
			t.Fatalf("boundary: %s", got)
		}
		if got := EffectiveStatus(status, now.Add(time.Second), now); got != status {
			t.Fatalf("future: %s", got)
		}
	}
	for _, status := range []string{"canceled", "suspended"} {
		if got := EffectiveStatus(status, now.Add(-time.Hour), now); got != status {
			t.Fatal(got)
		}
	}
	if ValidStatus("paid") || ValidStatus("expired") || ValidStatus("") {
		t.Fatal("invalid administrative status accepted")
	}
}
