package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// --- Creating users without a password (SSO-only accounts) ---

func TestApp_CreateUser_WithoutPassword(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	admin := createAdminUser(t, app, org.ID)
	email := testutil.UniqueEmail("sso-only")

	req := testutil.NewJSONRequest(t, map[string]any{"email": email, "full_name": "SSO Only"})
	testutil.SetAuthContext(req, org.ID, admin.ID)
	require.NoError(t, app.CreateUser(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var user models.User
	require.NoError(t, app.DB.Where("email = ?", email).First(&user).Error)
	assert.Empty(t, user.PasswordHash, "no password means no password hash")

	// Password login can't be used: same answer as a wrong password.
	for _, pw := range []string{"", "anything"} {
		login := testutil.NewJSONRequest(t, map[string]string{"email": email, "password": pw})
		require.NoError(t, app.Login(login))
		testutil.AssertErrorResponse(t, login, fasthttp.StatusUnauthorized, "Invalid credentials")
	}
}

// --- Require SSO for an organization ---

// ssoPolicyOrg creates an org with sso_required set and, if enabled, an
// enabled SSO provider.
func ssoPolicyOrg(t *testing.T, app *handlers.App, required, withProvider bool) *models.Organization {
	t.Helper()
	org := testutil.CreateTestOrganization(t, app.DB)
	org.Settings = models.JSONB{"sso_required": required}
	require.NoError(t, app.DB.Model(org).Update("settings", org.Settings).Error)
	if withProvider {
		require.NoError(t, app.DB.Create(&models.SSOProvider{
			BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, Provider: "google",
			ClientID: "id", ClientSecret: "s", IsEnabled: true,
		}).Error)
	}
	return org
}

func passwordLogin(t *testing.T, app *handlers.App, email, password string) *fastglue.Request {
	t.Helper()
	req := testutil.NewJSONRequest(t, map[string]string{"email": email, "password": password})
	require.NoError(t, app.Login(req))
	return req
}

func TestApp_Login_SSORequired(t *testing.T) {
	app := newTestApp(t)
	org := ssoPolicyOrg(t, app, true, true)

	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	agentEmail := testutil.UniqueEmail("sso-req-agent")
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(agentEmail),
		testutil.WithPassword("password123"), testutil.WithRoleID(&agentRole.ID))

	adminRole := testutil.CreateTestRoleExact(t, app.DB, org.ID, "admin", true, false, nil)
	adminEmail := testutil.UniqueEmail("sso-req-admin")
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(adminEmail),
		testutil.WithPassword("password123"), testutil.WithRoleID(&adminRole.ID))

	superEmail := testutil.UniqueEmail("sso-req-super")
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(superEmail),
		testutil.WithPassword("password123"), testutil.WithRoleID(&agentRole.ID), testutil.WithSuperAdmin())

	t.Run("non-admin with correct password is told to use SSO", func(t *testing.T) {
		req := passwordLogin(t, app, agentEmail, "password123")
		testutil.AssertErrorResponse(t, req, fasthttp.StatusForbidden, "sign in with SSO")
		assert.Empty(t, testutil.GetResponseCookie(req, "whm_access"))
	})

	t.Run("wrong password reveals nothing about the policy", func(t *testing.T) {
		req := passwordLogin(t, app, agentEmail, "wrong")
		testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid credentials")
	})

	t.Run("org admin can still use a password", func(t *testing.T) {
		req := passwordLogin(t, app, adminEmail, "password123")
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	})

	t.Run("super admin can still use a password", func(t *testing.T) {
		req := passwordLogin(t, app, superEmail, "password123")
		assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	})
}

func TestApp_Login_SSORequiredWithoutProviderAllowsPassword(t *testing.T) {
	app := newTestApp(t)
	// Required but no enabled provider: blocking would lock everyone out.
	org := ssoPolicyOrg(t, app, true, false)
	email := testutil.UniqueEmail("sso-req-noprov")
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email), testutil.WithPassword("password123"))

	req := passwordLogin(t, app, email, "password123")
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

func TestApp_Login_SSONotRequiredAllowsPassword(t *testing.T) {
	app := newTestApp(t)
	org := ssoPolicyOrg(t, app, false, true)
	email := testutil.UniqueEmail("sso-not-req")
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email), testutil.WithPassword("password123"))

	req := passwordLogin(t, app, email, "password123")
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

// --- Policy endpoints ---

func TestApp_SSOPolicy(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	admin := createAdminUser(t, app, org.ID)
	readerRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "sso-reader", []string{"settings.sso:read"})
	reader := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&readerRole.ID))
	agentRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "agent", []string{"chat:read"})
	agent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))

	get := func(userID uuid.UUID) (*fastglue.Request, bool) {
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, userID)
		require.NoError(t, app.GetSSOPolicy(req))
		var resp struct {
			Data struct {
				SSORequired bool `json:"sso_required"`
			} `json:"data"`
		}
		_ = json.Unmarshal(testutil.GetResponseBody(req), &resp)
		return req, resp.Data.SSORequired
	}
	put := func(userID uuid.UUID, required bool) *fastglue.Request {
		req := testutil.NewJSONRequest(t, map[string]any{"sso_required": required})
		testutil.SetAuthContext(req, org.ID, userID)
		require.NoError(t, app.UpdateSSOPolicy(req))
		return req
	}

	req, required := get(reader.ID)
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	assert.False(t, required, "off by default")

	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(put(admin.ID, true)))
	_, required = get(reader.ID)
	assert.True(t, required)

	// Other org settings survive the update.
	var saved models.Organization
	require.NoError(t, app.DB.First(&saved, "id = ?", org.ID).Error)
	assert.Equal(t, true, saved.Settings["sso_required"])

	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(put(reader.ID, false)), "read-only can't change it")
	req, _ = get(agent.ID)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}
