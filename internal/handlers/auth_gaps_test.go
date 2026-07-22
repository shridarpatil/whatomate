package handlers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/middleware"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// generateRefreshTokenWithJTI creates a refresh JWT whose claims.ID matches the
// provided JTI, so production's single-use Redis check actually fires.
func generateRefreshTokenWithJTI(t *testing.T, secret string, user *models.User, jti string, expiry time.Duration) string {
	t.Helper()
	claims := middleware.JWTClaims{
		UserID:         user.ID,
		OrganizationID: user.OrganizationID,
		Email:          user.Email,
		RoleID:         user.RoleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "whatomate",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

// --- SwitchOrg: role-from-user_org and JSON edge cases not covered elsewhere ---

func TestApp_SwitchOrg_AccessTokenCarriesUserOrgRole(t *testing.T) {
	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	targetOrg := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, homeOrg.ID)

	// Membership in target org carries a different role than the user's home role.
	targetRole := testutil.CreateTestRole(t, app.DB, targetOrg.ID, "agent", nil)
	require.NoError(t, app.DB.Create(&models.UserOrganization{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		UserID:         user.ID,
		OrganizationID: targetOrg.ID,
		RoleID:         &targetRole.ID,
	}).Error)

	req := testutil.NewJSONRequest(t, map[string]any{
		"organization_id": targetOrg.ID.String(),
	})
	testutil.SetPathParam(req, "user_id", user.ID)

	require.NoError(t, app.SwitchOrg(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	accessTokenStr := testutil.GetResponseCookie(req, "whm_access")
	require.NotEmpty(t, accessTokenStr)

	parsed, err := jwt.ParseWithClaims(accessTokenStr, &middleware.JWTClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(testutil.TestJWTSecret), nil
	})
	require.NoError(t, err)
	claims, ok := parsed.Claims.(*middleware.JWTClaims)
	require.True(t, ok)
	assert.Equal(t, targetOrg.ID, claims.OrganizationID)
	require.NotNil(t, claims.RoleID)
	assert.Equal(t, targetRole.ID, *claims.RoleID, "access token role must come from user_organizations entry for target org")
}

func TestApp_SwitchOrg_InvalidJSON(t *testing.T) {
	app := newTestApp(t)
	homeOrg := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, homeOrg.ID)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("not json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")
	testutil.SetPathParam(req, "user_id", user.ID)

	require.NoError(t, app.SwitchOrg(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- Logout ---

func TestApp_Logout_ClearsCookiesAndReturnsOK(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]string{})
	require.NoError(t, app.Logout(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	for _, name := range []string{"whm_access", "whm_refresh", "whm_csrf"} {
		var seen bool
		req.RequestCtx.Response.Header.VisitAllCookie(func(key, _ []byte) {
			if string(key) == name {
				seen = true
			}
		})
		assert.True(t, seen, "expected logout to set Set-Cookie for %s", name)
	}
}

func TestApp_Logout_RevokesRefreshTokenJTI(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	jti := uuid.New().String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, app.Redis.Set(ctx, "refresh:"+jti, user.ID.String(), time.Hour).Err())

	token := generateRefreshTokenWithJTI(t, testutil.TestJWTSecret, user, jti, time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{"refresh_token": token})
	require.NoError(t, app.Logout(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	exists, err := app.Redis.Exists(ctx, "refresh:"+jti).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "refresh token JTI should be revoked from Redis")
}

func TestApp_Logout_NoTokenStillSucceeds(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]string{})
	require.NoError(t, app.Logout(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

func TestApp_Logout_GarbageRefreshTokenStillSucceeds(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]string{"refresh_token": "garbage.not.a.jwt"})
	require.NoError(t, app.Logout(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

func TestApp_Logout_FromCookie(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	jti := uuid.New().String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, app.Redis.Set(ctx, "refresh:"+jti, user.ID.String(), time.Hour).Err())

	token := generateRefreshTokenWithJTI(t, testutil.TestJWTSecret, user, jti, time.Hour)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetCookie("whm_refresh", token)
	req.RequestCtx.Request.Header.SetContentType("application/json")

	require.NoError(t, app.Logout(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	exists, err := app.Redis.Exists(ctx, "refresh:"+jti).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "cookie-based logout must also revoke JTI")
}

// --- GetWSToken ---

func TestApp_GetWSToken_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.GetWSToken(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Equal(t, "success", resp.Status)
	require.NotEmpty(t, resp.Data.Token)

	parsed, err := jwt.ParseWithClaims(resp.Data.Token, &middleware.JWTClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(testutil.TestJWTSecret), nil
	})
	require.NoError(t, err)
	claims, ok := parsed.Claims.(*middleware.JWTClaims)
	require.True(t, ok)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, org.ID, claims.OrganizationID)
	assert.Equal(t, "ws", claims.Subject, "WS token must have subject=ws")

	require.NotNil(t, claims.ExpiresAt)
	ttl := time.Until(claims.ExpiresAt.Time)
	assert.LessOrEqual(t, ttl, 30*time.Second)
	assert.Greater(t, ttl, 25*time.Second, "WS token should be issued with ~30s TTL")
}

func TestApp_GetWSToken_MissingUserID(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("organization_id", org.ID)

	require.NoError(t, app.GetWSToken(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Unauthorized")
}

func TestApp_GetWSToken_MissingOrgID(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.SetUserValue("user_id", uuid.New())

	require.NoError(t, app.GetWSToken(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Unauthorized")
}

// --- RefreshToken rotation (JTI single-use) ---

func TestApp_RefreshToken_RevokedJTI(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	// JTI is NOT stored in Redis → simulates a revoked / already-used token.
	jti := uuid.New().String()
	token := generateRefreshTokenWithJTI(t, testutil.TestJWTSecret, user, jti, time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{"refresh_token": token})
	require.NoError(t, app.RefreshToken(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "revoked")
}

func TestApp_RefreshToken_RotatesJTI_ReplayFails(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	jti := uuid.New().String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, app.Redis.Set(ctx, "refresh:"+jti, user.ID.String(), time.Hour).Err())

	token := generateRefreshTokenWithJTI(t, testutil.TestJWTSecret, user, jti, time.Hour)

	// First refresh consumes the JTI.
	req1 := testutil.NewJSONRequest(t, map[string]string{"refresh_token": token})
	require.NoError(t, app.RefreshToken(req1))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req1))

	// Replay must fail.
	req2 := testutil.NewJSONRequest(t, map[string]string{"refresh_token": token})
	require.NoError(t, app.RefreshToken(req2))
	testutil.AssertErrorResponse(t, req2, fasthttp.StatusUnauthorized, "revoked")

	// Rotated refresh token must have a different JTI.
	newRefresh := testutil.GetResponseCookie(req1, "whm_refresh")
	require.NotEmpty(t, newRefresh)
	parsed, err := jwt.ParseWithClaims(newRefresh, &middleware.JWTClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(testutil.TestJWTSecret), nil
	})
	require.NoError(t, err)
	newClaims, ok := parsed.Claims.(*middleware.JWTClaims)
	require.True(t, ok)
	assert.NotEqual(t, jti, newClaims.ID, "rotation must produce a fresh JTI")
}

func TestApp_RefreshToken_FromCookie(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	jti := uuid.New().String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, app.Redis.Set(ctx, "refresh:"+jti, user.ID.String(), time.Hour).Err())

	token := generateRefreshTokenWithJTI(t, testutil.TestJWTSecret, user, jti, time.Hour)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetCookie("whm_refresh", token)
	req.RequestCtx.Request.Header.SetContentType("application/json")

	require.NoError(t, app.RefreshToken(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}
