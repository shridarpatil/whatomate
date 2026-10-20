package handlers

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/saas"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (a *App) requireSaaSAdmin(r *fastglue.Request) bool {
	_, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		_ = r.SendErrorEnvelope(401, "Unauthorized", nil, "")
		return false
	}
	if !a.IsSuperAdmin(userID) {
		_ = r.SendErrorEnvelope(403, "Forbidden", nil, "")
		return false
	}
	return true
}

func (a *App) ListSaaSPlans(r *fastglue.Request) error {
	if !a.requireSaaSAdmin(r) {
		return nil
	}
	plans := []models.SaaSPlan{}
	if err := a.DB.Order("created_at DESC").Limit(1000).Find(&plans).Error; err != nil {
		return r.SendErrorEnvelope(500, "saas.load_failed", nil, "")
	}
	return r.SendEnvelope(plans)
}

func (a *App) SaveSaaSPlan(r *fastglue.Request) error {
	if !a.requireSaaSAdmin(r) {
		return nil
	}
	var input saas.PlanInput
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &input); err != nil {
		return r.SendErrorEnvelope(400, "saas.invalid_request", nil, "")
	}
	if err := input.Validate(); err != nil {
		return r.SendErrorEnvelope(400, err.Error(), nil, "")
	}
	plan := models.SaaSPlan{Name: input.Name, PriceCents: input.PriceCents, Currency: input.Currency,
		IntervalMonths: input.IntervalMonths, TrialDays: input.TrialDays, UserLimit: input.UserLimit,
		AccountLimit: input.AccountLimit, TeamLimit: input.TeamLimit, Active: input.Active, Public: input.Public}
	if string(r.RequestCtx.Method()) == fasthttp.MethodPut {
		id, err := parsePathUUID(r, "id", "plan")
		if err != nil {
			return nil
		}
		var previous models.SaaSPlan
		if err := a.DB.First(&previous, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return r.SendErrorEnvelope(404, "saas.plan_not_found", nil, "")
			}
			return r.SendErrorEnvelope(500, "saas.load_failed", nil, "")
		}
		plan.BaseModel = previous.BaseModel
	}
	if err := a.DB.Save(&plan).Error; err != nil {
		return r.SendErrorEnvelope(500, "saas.save_failed", nil, "")
	}
	return r.SendEnvelope(plan)
}

func (a *App) ListSaaSSubscriptions(r *fastglue.Request) error {
	if !a.requireSaaSAdmin(r) {
		return nil
	}
	subscriptions := []models.SaaSSubscription{}
	if err := a.DB.Preload("Plan").Preload("Organization").Order("created_at DESC").Limit(1000).Find(&subscriptions).Error; err != nil {
		return r.SendErrorEnvelope(500, "saas.load_failed", nil, "")
	}
	return r.SendEnvelope(subscriptions)
}

func (a *App) AssignSaaSSubscription(r *fastglue.Request) error {
	if !a.requireSaaSAdmin(r) {
		return nil
	}
	var input struct {
		OrganizationID uuid.UUID `json:"organization_id"`
		PlanID         uuid.UUID `json:"plan_id"`
		Status         string    `json:"status"`
		ExpiresAt      time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &input); err != nil || input.OrganizationID == uuid.Nil || input.PlanID == uuid.Nil || !saas.ValidStatus(input.Status) || input.ExpiresAt.IsZero() {
		return r.SendErrorEnvelope(400, "saas.invalid_subscription", nil, "")
	}
	if (input.Status == "active" || input.Status == "trialing") && !input.ExpiresAt.After(time.Now()) {
		return r.SendErrorEnvelope(400, "saas.invalid_expiry", nil, "")
	}
	var subscription models.SaaSSubscription
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the tenant so concurrent assignments cannot create duplicate rows.
		var org models.Organization
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&org, "id = ?", input.OrganizationID).Error; err != nil {
			return err
		}
		var plan models.SaaSPlan
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND active = ?", input.PlanID, true).First(&plan).Error; err != nil {
			return err
		}
		err := tx.Where("organization_id = ?", input.OrganizationID).First(&subscription).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		subscription.OrganizationID, subscription.PlanID = input.OrganizationID, input.PlanID
		subscription.Status, subscription.ExpiresAt = input.Status, input.ExpiresAt.UTC()
		return tx.Omit("Organization", "Plan").Save(&subscription).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.SendErrorEnvelope(404, "saas.assignment_not_found", nil, "")
	}
	if err != nil {
		return r.SendErrorEnvelope(500, "saas.save_failed", nil, "")
	}
	return r.SendEnvelope(map[string]any{"id": subscription.ID})
}

func (a *App) GetSaaSBilling(r *fastglue.Request) error {
	orgID, _, err := a.requireAuth(r, models.ResourceSettingsGeneral, models.ActionRead)
	if err != nil {
		return nil
	}
	var subscription models.SaaSSubscription
	var data *models.SaaSSubscription
	status := "unmanaged"
	err = a.DB.Preload("Plan").Where("organization_id = ?", orgID).First(&subscription).Error
	if err == nil {
		data = &subscription
		status = saas.EffectiveStatus(subscription.Status, subscription.ExpiresAt, time.Now())
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return r.SendErrorEnvelope(500, "saas.load_failed", nil, "")
	}
	usage := map[string]int64{}
	for name, model := range map[string]any{"users": &models.UserOrganization{}, "accounts": &models.WhatsAppAccount{}, "teams": &models.Team{}} {
		var count int64
		if err := a.DB.Model(model).Where("organization_id = ?", orgID).Count(&count).Error; err != nil {
			return r.SendErrorEnvelope(500, "saas.load_failed", nil, "")
		}
		usage[name] = count
	}
	payments := a.paymentService()
	return r.SendEnvelope(map[string]any{"subscription": data, "status": status, "usage": usage,
		"checkout_enabled": payments.Ready(), "payment_provider": payments.Gateway(), "limits_enforced": false})
}
