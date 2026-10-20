package handlers

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/audit"
	"github.com/shridarpatil/whatomate/internal/database"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/saas"
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateSaaSTrial provisions an isolated company. Only the platform operator can
// grant trials; this endpoint never joins an existing user or grants superadmin.
func (a *App) CreateSaaSTrial(r *fastglue.Request) error {
	if !a.requireSaaSAdmin(r) {
		return nil
	}
	var input saas.TrialInput
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &input); err != nil {
		return r.SendErrorEnvelope(400, "saas.invalid_trial_signup", nil, "")
	}
	if err := input.Validate(); err != nil {
		return r.SendErrorEnvelope(400, err.Error(), nil, "")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return r.SendErrorEnvelope(500, "saas.save_failed", nil, "")
	}
	invalidPlan := errors.New("invalid trial plan")
	duplicateEmail := errors.New("existing email")
	var org models.Organization
	var owner models.User
	var sub models.SaaSSubscription
	_, actor, _ := a.getOrgAndUserID(r)
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		var plan models.SaaSPlan
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND active = ?", input.PlanID, true).First(&plan).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return invalidPlan
			}
			return err
		}
		if plan.TrialDays < 1 || plan.TrialDays > 365 || plan.UserLimit == 0 {
			return invalidPlan
		}
		// Serialize equivalent emails, including requests from different replicas.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "saas-owner:"+input.Email).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Unscoped().Model(&models.User{}).Where("LOWER(email) = ?", input.Email).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return duplicateEmail
		}
		org = models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: input.Name, Slug: "company-" + uuid.NewString(), Settings: models.JSONB{}}
		if err := tx.Create(&org).Error; err != nil {
			return err
		}
		if err := database.SeedSystemRolesForOrg(tx, org.ID); err != nil {
			return err
		}
		var role models.CustomRole
		if err := tx.Where("organization_id = ? AND name = ? AND is_system = ?", org.ID, "admin", true).First(&role).Error; err != nil {
			return err
		}
		owner = models.User{OrganizationID: org.ID, Email: input.Email, FullName: input.OwnerName, PasswordHash: string(hash), RoleID: &role.ID, IsActive: true, IsSuperAdmin: false}
		if err := tx.Create(&owner).Error; err != nil {
			return err
		}
		membership := models.UserOrganization{UserID: owner.ID, OrganizationID: org.ID, RoleID: &role.ID, IsDefault: true}
		if err := tx.Create(&membership).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.ChatbotSettings{OrganizationID: org.ID, IsEnabled: false, SessionTimeoutMins: 30}).Error; err != nil {
			return err
		}
		if err := database.SeedDefaultWidgetsForOrg(tx, org.ID, owner.ID); err != nil {
			return err
		}
		sub = models.SaaSSubscription{OrganizationID: org.ID, PlanID: plan.ID, Status: "trialing", ExpiresAt: time.Now().UTC().AddDate(0, 0, plan.TrialDays)}
		if err := tx.Omit("Plan", "Organization").Create(&sub).Error; err != nil {
			return err
		}
		changes := models.JSONBArray{}
		for _, change := range audit.ComputeChanges(nil, map[string]any{"name": org.Name, "plan_id": sub.PlanID, "owner_id": owner.ID, "trial_expires_at": sub.ExpiresAt}) {
			changes = append(changes, change)
		}
		return tx.Create(&models.AuditLog{OrganizationID: org.ID, UserID: actor, UserName: audit.GetUserName(tx, actor), ResourceType: "organization", ResourceID: org.ID, Action: models.AuditActionCreated, Changes: changes}).Error
	})
	if errors.Is(err, invalidPlan) {
		return r.SendErrorEnvelope(400, "saas.trial_plan_required", nil, "")
	}
	if errors.Is(err, duplicateEmail) {
		return r.SendErrorEnvelope(409, "saas.owner_email_exists", nil, "")
	}
	if err != nil {
		return r.SendErrorEnvelope(500, "saas.save_failed", nil, "")
	}
	return r.SendEnvelope(map[string]any{"organization_id": org.ID, "owner_id": owner.ID, "expires_at": sub.ExpiresAt})
}
