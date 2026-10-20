package saasapi_test

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/database"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/saas"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"github.com/zerodha/logf"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTrialProvisioningAndIsolation(t *testing.T) {
	dsn := os.Getenv("SAAS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SAAS_TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(db))
	tx := db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, database.SeedPermissionsAndRoles(tx))
	a := &handlers.App{DB: tx, Log: logf.New(logf.Opts{})}
	platform := models.Organization{Name: "Platform", Slug: uuid.NewString()}
	require.NoError(t, tx.Create(&platform).Error)
	admin := models.User{OrganizationID: platform.ID, Email: uuid.NewString() + "@test.invalid", IsSuperAdmin: true}
	require.NoError(t, tx.Create(&admin).Error)
	plan := models.SaaSPlan{Name: "Trial", TrialDays: 14, UserLimit: 3, Active: true, PriceCents: 9900, Currency: "BRL", IntervalMonths: 1}
	require.NoError(t, tx.Create(&plan).Error)
	request := func(user models.User, body any) *fastglue.Request {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.Header.SetContentType("application/json")
		ctx.SetUserValue("user_id", user.ID)
		ctx.SetUserValue("organization_id", user.OrganizationID)
		b, err := json.Marshal(body)
		require.NoError(t, err)
		ctx.Request.SetBody(b)
		return &fastglue.Request{RequestCtx: ctx}
	}
	input := saas.TrialInput{Name: "Trial tenant", OwnerName: "Owner", Email: uuid.NewString() + "@test.invalid", Password: "OnlyForTest-12345", PlanID: plan.ID}
	before := time.Now().UTC()
	r := request(admin, input)
	require.NoError(t, a.CreateSaaSTrial(r))
	require.Equal(t, 200, r.RequestCtx.Response.StatusCode(), string(r.RequestCtx.Response.Body()))
	require.NotContains(t, string(r.RequestCtx.Response.Body()), input.Password)
	var owner models.User
	require.NoError(t, tx.Where("email = ?", input.Email).First(&owner).Error)
	require.False(t, owner.IsSuperAdmin)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(owner.PasswordHash), []byte(input.Password)))
	var sub models.SaaSSubscription
	require.NoError(t, tx.Where("organization_id = ?", owner.OrganizationID).First(&sub).Error)
	require.Equal(t, "trialing", sub.Status)
	require.WithinDuration(t, before.AddDate(0, 0, 14), sub.ExpiresAt, 5*time.Second)
	var members int64
	require.NoError(t, tx.Model(&models.UserOrganization{}).Where("organization_id = ?", owner.OrganizationID).Count(&members).Error)
	require.Equal(t, int64(1), members)
	r = request(owner, input)
	require.NoError(t, a.CreateSaaSTrial(r))
	require.Equal(t, 403, r.RequestCtx.Response.StatusCode())
	r = request(owner, nil)
	require.NoError(t, a.ListOrganizations(r))
	require.Equal(t, 200, r.RequestCtx.Response.StatusCode())
	require.NotContains(t, string(r.RequestCtx.Response.Body()), platform.ID.String())
	require.Contains(t, string(r.RequestCtx.Response.Body()), owner.OrganizationID.String())
	r = request(owner, map[string]string{"name": "Bypass"})
	require.NoError(t, a.CreateOrganization(r))
	require.Equal(t, 403, r.RequestCtx.Response.StatusCode())
	r = request(admin, map[string]any{"organization_id": owner.OrganizationID, "email": "stranger@test.invalid", "password": "OnlyForTest-12345", "full_name": "Stranger"})
	require.NoError(t, a.Register(r))
	require.Equal(t, 403, r.RequestCtx.Response.StatusCode())
	var orgsBefore int64
	require.NoError(t, tx.Model(&models.Organization{}).Count(&orgsBefore).Error)
	// Retrying a completed registration, including different email casing, cannot
	// create a second company or reset another user's password.
	input.Email = strings.ToUpper(input.Email)
	r = request(admin, input)
	require.NoError(t, a.CreateSaaSTrial(r))
	require.Equal(t, 409, r.RequestCtx.Response.StatusCode())
	input.Email = uuid.NewString() + "@test.invalid"
	require.NoError(t, tx.Model(&plan).Update("active", false).Error)
	r = request(admin, input)
	require.NoError(t, a.CreateSaaSTrial(r))
	require.Equal(t, 400, r.RequestCtx.Response.StatusCode())
	var orgsAfter int64
	require.NoError(t, tx.Model(&models.Organization{}).Count(&orgsAfter).Error)
	require.Equal(t, orgsBefore, orgsAfter)
	// A failure after the company, roles and owner were inserted must roll back
	// all of them, not leave an inaccessible or unbilled tenant behind.
	require.NoError(t, tx.Model(&plan).Update("active", true).Error)
	const hook = "test:fail_trial_chatbot"
	require.NoError(t, tx.Callback().Create().Before("gorm:create").Register(hook, func(query *gorm.DB) {
		if query.Statement.Table == "chatbot_settings" {
			query.AddError(errors.New("injected provisioning failure"))
		}
	}))
	defer tx.Callback().Create().Remove(hook)
	r = request(admin, input)
	require.NoError(t, a.CreateSaaSTrial(r))
	require.Equal(t, 500, r.RequestCtx.Response.StatusCode())
	require.NoError(t, tx.Model(&models.Organization{}).Count(&orgsAfter).Error)
	require.Equal(t, orgsBefore, orgsAfter)
	var missing models.User
	require.ErrorIs(t, tx.Where("email = ?", input.Email).First(&missing).Error, gorm.ErrRecordNotFound)
}
