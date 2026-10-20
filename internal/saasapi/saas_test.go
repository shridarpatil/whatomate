package saasapi_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/database"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"github.com/zerodha/logf"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Use a disposable database. This package does not depend on legacy testutil.
func TestSaaSAuthorizationAndAssignment(t *testing.T) {
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
	a := &handlers.App{DB: tx, Log: logf.New(logf.Opts{})}
	orgA := models.Organization{Name: "A", Slug: uuid.NewString()}
	orgB := models.Organization{Name: "B", Slug: uuid.NewString()}
	require.NoError(t, tx.Create(&orgA).Error)
	require.NoError(t, tx.Create(&orgB).Error)
	permission := models.Permission{Resource: models.ResourceSettingsGeneral, Action: models.ActionRead}
	require.NoError(t, tx.Where("resource = ? AND action = ?", permission.Resource, permission.Action).FirstOrCreate(&permission).Error)
	role := models.CustomRole{OrganizationID: orgA.ID, Name: "Billing reader", Permissions: []models.Permission{permission}}
	require.NoError(t, tx.Create(&role).Error)
	admin := models.User{OrganizationID: orgA.ID, Email: uuid.NewString() + "@test.invalid", IsSuperAdmin: true}
	member := models.User{OrganizationID: orgA.ID, Email: uuid.NewString() + "@test.invalid", RoleID: &role.ID}
	require.NoError(t, tx.Create(&admin).Error)
	require.NoError(t, tx.Create(&member).Error)
	request := func(user models.User, body any) *fastglue.Request {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod("POST")
		ctx.SetUserValue("user_id", user.ID)
		ctx.SetUserValue("organization_id", user.OrganizationID)
		b, err := json.Marshal(body)
		require.NoError(t, err)
		ctx.Request.SetBody(b)
		return &fastglue.Request{RequestCtx: ctx}
	}
	planBody := map[string]any{"name": "Starter", "price_cents": 9900, "currency": "BRL", "interval_months": 1, "trial_days": 7, "active": true, "user_limit": 5, "account_limit": 1, "team_limit": -1}
	r := request(member, planBody)
	require.NoError(t, a.SaveSaaSPlan(r))
	require.Equal(t, 403, r.RequestCtx.Response.StatusCode())
	r = request(admin, planBody)
	require.NoError(t, a.SaveSaaSPlan(r))
	require.Equal(t, 200, r.RequestCtx.Response.StatusCode())
	var response struct {
		Data models.SaaSPlan `json:"data"`
	}
	require.NoError(t, json.Unmarshal(r.RequestCtx.Response.Body(), &response))
	planID := response.Data.ID
	require.NotEqual(t, uuid.Nil, planID)
	body := map[string]any{"organization_id": orgB.ID, "plan_id": planID, "status": "trialing", "expires_at": time.Now().Add(24 * time.Hour)}
	for i := 0; i < 2; i++ {
		r = request(admin, body)
		require.NoError(t, a.AssignSaaSSubscription(r))
		require.Equal(t, 200, r.RequestCtx.Response.StatusCode(), string(r.RequestCtx.Response.Body()))
	}
	var count int64
	require.NoError(t, tx.Model(&models.SaaSSubscription{}).Where("organization_id = ?", orgB.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
	r = request(member, body)
	require.NoError(t, a.AssignSaaSSubscription(r))
	require.Equal(t, 403, r.RequestCtx.Response.StatusCode())
	r = request(member, nil)
	r.RequestCtx.Request.Header.Set("X-Organization-ID", orgB.ID.String())
	require.NoError(t, a.GetSaaSBilling(r))
	require.Equal(t, 200, r.RequestCtx.Response.StatusCode())
	require.Contains(t, string(r.RequestCtx.Response.Body()), `"status":"unmanaged"`)
	require.NotContains(t, string(r.RequestCtx.Response.Body()), orgB.ID.String())
	// Financial actions require write permission, even when the user can read
	// commercial settings. A forged organization header cannot reveal invoices.
	for _, action := range []func(*fastglue.Request) error{a.CreateSaaSInvoice, a.RefreshSaaSInvoice, a.CancelSaaSInvoice, a.ReissueSaaSInvoice} {
		r = request(member, nil)
		require.NoError(t, action(r))
		require.Equal(t, 403, r.RequestCtx.Response.StatusCode())
	}
	foreignInvoice := models.SaaSInvoice{OrganizationID: orgB.ID, SubscriptionID: uuid.New(), PlanID: planID, Cycle: uuid.NewString(), PriceCents: 9900, Currency: "BRL", IntervalMonths: 1, Provider: "mercadopago", Status: "pending", PayerEmail: "private@example.invalid", PixCode: "private-pix-code", ExpiresAt: time.Now(), RequestedExpiresAt: time.Now(), BaseExpiresAt: time.Now(), LastCheckedAt: time.Now()}
	require.NoError(t, tx.Create(&foreignInvoice).Error)
	r = request(member, nil)
	r.RequestCtx.Request.Header.Set("X-Organization-ID", orgB.ID.String())
	require.NoError(t, a.ListSaaSInvoices(r))
	require.Equal(t, 200, r.RequestCtx.Response.StatusCode())
	require.NotContains(t, string(r.RequestCtx.Response.Body()), "private-pix-code")
	require.NotContains(t, string(r.RequestCtx.Response.Body()), foreignInvoice.ID.String())
	planBody["active"] = false
	r = request(admin, planBody)
	r.RequestCtx.Request.Header.SetMethod("PUT")
	r.RequestCtx.SetUserValue("id", planID.String())
	require.NoError(t, a.SaveSaaSPlan(r))
	require.Equal(t, 200, r.RequestCtx.Response.StatusCode())
	r = request(admin, body)
	require.NoError(t, a.AssignSaaSSubscription(r))
	require.Equal(t, 404, r.RequestCtx.Response.StatusCode())
}
