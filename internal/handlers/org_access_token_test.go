package handlers_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/handlers"
	"github.com/nikyjain/whatomate/internal/middleware"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveOrgUser_SwitchOrgRules(t *testing.T) {
	app := newTestApp(t)
	home := testutil.CreateTestOrganization(t, app.DB)
	other := testutil.CreateTestOrganization(t, app.DB)
	homeRole := testutil.CreateTestRoleWithKeys(t, app.DB, home.ID, "home-"+uuid.NewString()[:8], []string{"contacts:read"})
	otherRole := testutil.CreateTestRoleWithKeys(t, app.DB, other.ID, "other-"+uuid.NewString()[:8], []string{"users:read"})
	user := testutil.CreateTestUser(t, app.DB, home.ID, testutil.WithRoleID(&homeRole.ID))

	_, err := app.ResolveOrgUser(user.ID, other.ID)
	assert.ErrorIs(t, err, handlers.ErrNotOrgMember)

	require.NoError(t, app.DB.Create(&models.UserOrganization{
		BaseModel: models.BaseModel{ID: uuid.New()}, UserID: user.ID, OrganizationID: other.ID, RoleID: &otherRole.ID,
	}).Error)
	got, err := app.ResolveOrgUser(user.ID, other.ID)
	require.NoError(t, err)
	assert.Equal(t, other.ID, got.OrganizationID)
	require.NotNil(t, got.RoleID)
	assert.Equal(t, otherRole.ID, *got.RoleID, "role must come from the target org membership")

	_, err = app.ResolveOrgUser(user.ID, uuid.New())
	assert.ErrorIs(t, err, handlers.ErrOrgNotFound)

	super := testutil.CreateTestUser(t, app.DB, home.ID, testutil.WithSuperAdmin(), testutil.WithRoleID(&homeRole.ID))
	got, err = app.ResolveOrgUser(super.ID, other.ID)
	require.NoError(t, err)
	assert.Equal(t, other.ID, got.OrganizationID)
	assert.True(t, got.IsSuperAdmin)

	inactive := testutil.CreateTestUser(t, app.DB, home.ID, testutil.WithInactive())
	_, err = app.ResolveOrgUser(inactive.ID, home.ID)
	assert.ErrorIs(t, err, handlers.ErrUserInactive)
}

func TestMintAccessToken_SessionClaims(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "mint-"+uuid.NewString()[:8], []string{"contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	resolved, err := app.ResolveOrgUser(user.ID, org.ID)
	require.NoError(t, err)

	token, err := app.MintAccessToken(resolved, 5*time.Minute)
	require.NoError(t, err)
	claims := &middleware.JWTClaims{}
	_, err = jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return []byte(testutil.TestJWTSecret), nil })
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, org.ID, claims.OrganizationID)
	assert.Equal(t, role.ID, *claims.RoleID)
	assert.Equal(t, user.Email, claims.Email)
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), claims.ExpiresAt.Time, 5*time.Second)
}
