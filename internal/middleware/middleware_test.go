package middleware_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/middleware"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

const testJWTSecret = "test-secret-key-must-be-at-least-32-chars"

// newTestRequest creates a fastglue request for testing.
func newTestRequest() *fastglue.Request {
	ctx := &fasthttp.RequestCtx{}
	return &fastglue.Request{RequestCtx: ctx}
}

// generateTestToken creates a valid JWT token for testing.
func generateTestToken(t *testing.T, userID, orgID uuid.UUID, email, role string, expiry time.Duration) string {
	t.Helper()

	claims := middleware.JWTClaims{
		UserID:         userID,
		OrganizationID: orgID,
		Email:          email,
		Role:           role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return tokenString
}

func TestCORS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		origin       string
		wantOrigin   string
	}{
		{
			name:       "with origin header",
			origin:     "https://example.com",
			wantOrigin: "https://example.com",
		},
		{
			name:       "without origin header",
			origin:     "",
			wantOrigin: "*",
		},
		{
			name:       "localhost origin",
			origin:     "http://localhost:3000",
			wantOrigin: "http://localhost:3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := newTestRequest()
			if tt.origin != "" {
				req.RequestCtx.Request.Header.Set("Origin", tt.origin)
			}

			corsMiddleware := middleware.CORS()
			result := corsMiddleware(req)

			require.NotNil(t, result, "CORS middleware should return request")

			// Check CORS headers
			assert.Equal(t, tt.wantOrigin, string(result.RequestCtx.Response.Header.Peek("Access-Control-Allow-Origin")))
			assert.Contains(t, string(result.RequestCtx.Response.Header.Peek("Access-Control-Allow-Methods")), "GET")
			assert.Contains(t, string(result.RequestCtx.Response.Header.Peek("Access-Control-Allow-Methods")), "POST")
			assert.Contains(t, string(result.RequestCtx.Response.Header.Peek("Access-Control-Allow-Headers")), "Authorization")
			assert.Contains(t, string(result.RequestCtx.Response.Header.Peek("Access-Control-Allow-Headers")), "X-API-Key")
			assert.Equal(t, "true", string(result.RequestCtx.Response.Header.Peek("Access-Control-Allow-Credentials")))
		})
	}
}

func TestRecovery(t *testing.T) {
	t.Parallel()

	log := testutil.NopLogger()
	recoveryMiddleware := middleware.Recovery(log)

	t.Run("normal request passes through", func(t *testing.T) {
		t.Parallel()

		req := newTestRequest()
		result := recoveryMiddleware(req)

		require.NotNil(t, result, "should return request")
	})

	// Note: Testing panic recovery is tricky because the panic happens
	// after the middleware returns. The Recovery middleware is designed
	// to wrap handlers, not to be tested in isolation.
}

func TestAuth_ValidJWT(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	orgID := uuid.New()
	email := "test@example.com"
	role := "admin"

	token := generateTestToken(t, userID, orgID, email, role, time.Hour)

	req := newTestRequest()
	req.RequestCtx.Request.Header.Set("Authorization", "Bearer "+token)

	authMiddleware := middleware.Auth(testJWTSecret)
	result := authMiddleware(req)

	require.NotNil(t, result, "should return request for valid token")

	// Verify context values were set
	gotUserID, ok := result.RequestCtx.UserValue(middleware.ContextKeyUserID).(uuid.UUID)
	require.True(t, ok, "user_id should be uuid.UUID")
	assert.Equal(t, userID, gotUserID)

	gotOrgID, ok := result.RequestCtx.UserValue(middleware.ContextKeyOrganizationID).(uuid.UUID)
	require.True(t, ok, "organization_id should be uuid.UUID")
	assert.Equal(t, orgID, gotOrgID)

	gotEmail, ok := result.RequestCtx.UserValue(middleware.ContextKeyEmail).(string)
	require.True(t, ok, "email should be string")
	assert.Equal(t, email, gotEmail)

	gotRole, ok := result.RequestCtx.UserValue(middleware.ContextKeyRole).(string)
	require.True(t, ok, "role should be string")
	assert.Equal(t, role, gotRole)
}

func TestAuth_ExpiredJWT(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	orgID := uuid.New()

	// Create an expired token
	token := generateTestToken(t, userID, orgID, "test@example.com", "admin", -time.Hour)

	req := newTestRequest()
	req.RequestCtx.Request.Header.Set("Authorization", "Bearer "+token)

	authMiddleware := middleware.Auth(testJWTSecret)
	result := authMiddleware(req)

	assert.Nil(t, result, "should return nil for expired token")
	assert.Equal(t, fasthttp.StatusUnauthorized, req.RequestCtx.Response.StatusCode())
}

func TestAuth_InvalidJWT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "missing authorization header",
			header: "",
		},
		{
			name:   "invalid format - no Bearer prefix",
			header: "invalid-token",
		},
		{
			name:   "invalid format - wrong prefix",
			header: "Basic some-token",
		},
		{
			name:   "malformed token",
			header: "Bearer not.a.valid.jwt",
		},
		{
			name:   "wrong secret",
			header: "Bearer " + generateTokenWithSecret(t, "wrong-secret-key-that-is-long-enough"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := newTestRequest()
			if tt.header != "" {
				req.RequestCtx.Request.Header.Set("Authorization", tt.header)
			}

			authMiddleware := middleware.Auth(testJWTSecret)
			result := authMiddleware(req)

			assert.Nil(t, result, "should return nil for invalid token")
			assert.Equal(t, fasthttp.StatusUnauthorized, req.RequestCtx.Response.StatusCode())
		})
	}
}

func TestAuth_DifferentRoles(t *testing.T) {
	t.Parallel()

	roles := []string{"admin", "manager", "agent"}

	for _, role := range roles {
		t.Run("role_"+role, func(t *testing.T) {
			t.Parallel()

			userID := uuid.New()
			orgID := uuid.New()
			token := generateTestToken(t, userID, orgID, "test@example.com", role, time.Hour)

			req := newTestRequest()
			req.RequestCtx.Request.Header.Set("Authorization", "Bearer "+token)

			authMiddleware := middleware.Auth(testJWTSecret)
			result := authMiddleware(req)

			require.NotNil(t, result)

			gotRole := result.RequestCtx.UserValue(middleware.ContextKeyRole).(string)
			assert.Equal(t, role, gotRole)
		})
	}
}

func TestRequireRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		userRole     string
		allowedRoles []string
		wantAllowed  bool
	}{
		{
			name:         "admin allowed for admin-only",
			userRole:     "admin",
			allowedRoles: []string{"admin"},
			wantAllowed:  true,
		},
		{
			name:         "agent denied for admin-only",
			userRole:     "agent",
			allowedRoles: []string{"admin"},
			wantAllowed:  false,
		},
		{
			name:         "manager allowed for admin or manager",
			userRole:     "manager",
			allowedRoles: []string{"admin", "manager"},
			wantAllowed:  true,
		},
		{
			name:         "agent allowed for multi-role",
			userRole:     "agent",
			allowedRoles: []string{"admin", "manager", "agent"},
			wantAllowed:  true,
		},
		{
			name:         "unknown role denied",
			userRole:     "unknown",
			allowedRoles: []string{"admin", "manager", "agent"},
			wantAllowed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := newTestRequest()
			req.RequestCtx.SetUserValue(middleware.ContextKeyRole, tt.userRole)

			roleMiddleware := middleware.RequireRole(tt.allowedRoles...)
			result := roleMiddleware(req)

			if tt.wantAllowed {
				assert.NotNil(t, result, "should allow access")
			} else {
				assert.Nil(t, result, "should deny access")
				assert.Equal(t, fasthttp.StatusForbidden, req.RequestCtx.Response.StatusCode())
			}
		})
	}
}

func TestRequireRole_NoRoleInContext(t *testing.T) {
	t.Parallel()

	req := newTestRequest()
	// Don't set any role in context

	roleMiddleware := middleware.RequireRole("admin")
	result := roleMiddleware(req)

	assert.Nil(t, result, "should deny when role not in context")
	assert.Equal(t, fasthttp.StatusForbidden, req.RequestCtx.Response.StatusCode())
}

func TestGetUserID(t *testing.T) {
	t.Parallel()

	t.Run("user ID exists", func(t *testing.T) {
		t.Parallel()

		expectedID := uuid.New()
		req := newTestRequest()
		req.RequestCtx.SetUserValue(middleware.ContextKeyUserID, expectedID)

		userID, ok := middleware.GetUserID(req)

		assert.True(t, ok)
		assert.Equal(t, expectedID, userID)
	})

	t.Run("user ID not set", func(t *testing.T) {
		t.Parallel()

		req := newTestRequest()

		_, ok := middleware.GetUserID(req)

		assert.False(t, ok)
	})

	t.Run("wrong type in context", func(t *testing.T) {
		t.Parallel()

		req := newTestRequest()
		req.RequestCtx.SetUserValue(middleware.ContextKeyUserID, "not-a-uuid")

		_, ok := middleware.GetUserID(req)

		assert.False(t, ok)
	})
}

func TestGetOrganizationID(t *testing.T) {
	t.Parallel()

	t.Run("organization ID exists", func(t *testing.T) {
		t.Parallel()

		expectedID := uuid.New()
		req := newTestRequest()
		req.RequestCtx.SetUserValue(middleware.ContextKeyOrganizationID, expectedID)

		orgID, ok := middleware.GetOrganizationID(req)

		assert.True(t, ok)
		assert.Equal(t, expectedID, orgID)
	})

	t.Run("organization ID not set", func(t *testing.T) {
		t.Parallel()

		req := newTestRequest()

		_, ok := middleware.GetOrganizationID(req)

		assert.False(t, ok)
	})
}

func TestRequestLogger(t *testing.T) {
	t.Parallel()

	log := testutil.NopLogger()
	loggerMiddleware := middleware.RequestLogger(log)

	req := newTestRequest()
	result := loggerMiddleware(req)

	require.NotNil(t, result)

	// Check that request_start was set
	startTime, ok := result.RequestCtx.UserValue("request_start").(time.Time)
	assert.True(t, ok, "request_start should be set")
	assert.WithinDuration(t, time.Now(), startTime, time.Second)
}

func TestJWTClaims(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	orgID := uuid.New()

	claims := middleware.JWTClaims{
		UserID:         userID,
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID.String(),
		},
	}

	// Create and sign token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)

	// Parse token back
	parsedToken, err := jwt.ParseWithClaims(tokenString, &middleware.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(testJWTSecret), nil
	})
	require.NoError(t, err)
	require.True(t, parsedToken.Valid)

	parsedClaims, ok := parsedToken.Claims.(*middleware.JWTClaims)
	require.True(t, ok)

	assert.Equal(t, userID, parsedClaims.UserID)
	assert.Equal(t, orgID, parsedClaims.OrganizationID)
	assert.Equal(t, "test@example.com", parsedClaims.Email)
	assert.Equal(t, "admin", parsedClaims.Role)
}

func TestAuth_MultipleMiddlewareChain(t *testing.T) {
	t.Parallel()

	// Test that Auth works correctly when chained with other middleware
	userID := uuid.New()
	orgID := uuid.New()
	token := generateTestToken(t, userID, orgID, "test@example.com", "admin", time.Hour)

	req := newTestRequest()
	req.RequestCtx.Request.Header.Set("Authorization", "Bearer "+token)
	req.RequestCtx.Request.Header.Set("Origin", "https://example.com")

	// Apply CORS first
	corsMiddleware := middleware.CORS()
	req = corsMiddleware(req)
	require.NotNil(t, req)

	// Then Auth
	authMiddleware := middleware.Auth(testJWTSecret)
	req = authMiddleware(req)
	require.NotNil(t, req)

	// Then RequireRole
	roleMiddleware := middleware.RequireRole("admin")
	req = roleMiddleware(req)
	require.NotNil(t, req)

	// Verify all context values are still present
	assert.Equal(t, userID, req.RequestCtx.UserValue(middleware.ContextKeyUserID))
	assert.Equal(t, orgID, req.RequestCtx.UserValue(middleware.ContextKeyOrganizationID))
	assert.Equal(t, "admin", req.RequestCtx.UserValue(middleware.ContextKeyRole))

	// Verify CORS headers are still present
	assert.Equal(t, "https://example.com", string(req.RequestCtx.Response.Header.Peek("Access-Control-Allow-Origin")))
}

// generateTokenWithSecret creates a token signed with a specific secret.
func generateTokenWithSecret(t *testing.T, secret string) string {
	t.Helper()

	claims := middleware.JWTClaims{
		UserID:         uuid.New(),
		OrganizationID: uuid.New(),
		Email:          "test@example.com",
		Role:           "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return tokenString
}
