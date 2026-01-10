package handlers_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/bcrypt"
)

const testJWTSecret = "test-secret-key-must-be-at-least-32-chars"

// testApp creates an App instance for testing with a test database.
func testApp(t *testing.T) *handlers.App {
	t.Helper()

	db := testutil.SetupTestDB(t)
	log := testutil.NopLogger()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:            testJWTSecret,
			AccessExpiryMins:  15,
			RefreshExpiryDays: 7,
		},
	}

	return &handlers.App{
		Config: cfg,
		DB:     db,
		Log:    log,
	}
}

// createTestOrganization creates a test organization in the database.
func createTestOrganization(t *testing.T, app *handlers.App) *models.Organization {
	t.Helper()

	org := &models.Organization{
		Name: "Test Organization",
		Slug: "test-org-" + uuid.New().String()[:8],
	}
	require.NoError(t, app.DB.Create(org).Error)
	return org
}

// createTestUser creates a test user in the database with a hashed password.
func createTestUser(t *testing.T, app *handlers.App, orgID uuid.UUID, email, password, role string, isActive bool) *models.User {
	t.Helper()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := &models.User{
		OrganizationID: orgID,
		Email:          email,
		PasswordHash:   string(hashedPassword),
		FullName:       "Test User",
		Role:           role,
		IsActive:       isActive,
	}
	require.NoError(t, app.DB.Create(user).Error)
	return user
}

// generateTestRefreshToken creates a valid refresh token for testing.
func generateTestRefreshToken(t *testing.T, user *models.User, secret string, expiry time.Duration) string {
	t.Helper()

	claims := handlers.JWTClaims{
		UserID:         user.ID,
		OrganizationID: user.OrganizationID,
		Email:          user.Email,
		Role:           user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "whatomate",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return tokenString
}

func TestApp_Login(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		setupData          func(t *testing.T, app *handlers.App) (email, password string)
		inputEmail         string
		inputPassword      string
		wantStatus         int
		wantErrContains    string
		wantSuccessful     bool
	}{
		{
			name: "successful login",
			setupData: func(t *testing.T, app *handlers.App) (string, string) {
				org := createTestOrganization(t, app)
				email := "valid@example.com"
				password := "validpassword123"
				createTestUser(t, app, org.ID, email, password, "admin", true)
				return email, password
			},
			wantStatus:     fasthttp.StatusOK,
			wantSuccessful: true,
		},
		{
			name: "wrong password",
			setupData: func(t *testing.T, app *handlers.App) (string, string) {
				org := createTestOrganization(t, app)
				email := "user@example.com"
				createTestUser(t, app, org.ID, email, "correctpassword", "admin", true)
				return email, "wrongpassword"
			},
			wantStatus:      fasthttp.StatusUnauthorized,
			wantErrContains: "Invalid credentials",
		},
		{
			name: "user not found",
			setupData: func(t *testing.T, app *handlers.App) (string, string) {
				return "nonexistent@example.com", "anypassword"
			},
			wantStatus:      fasthttp.StatusUnauthorized,
			wantErrContains: "Invalid credentials",
		},
		{
			name: "inactive user",
			setupData: func(t *testing.T, app *handlers.App) (string, string) {
				org := createTestOrganization(t, app)
				email := "inactive@example.com"
				password := "validpassword123"
				createTestUser(t, app, org.ID, email, password, "admin", false)
				return email, password
			},
			wantStatus:      fasthttp.StatusUnauthorized,
			wantErrContains: "Account is disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := testApp(t)

			email, password := tt.setupData(t, app)
			if tt.inputEmail != "" {
				email = tt.inputEmail
			}
			if tt.inputPassword != "" {
				password = tt.inputPassword
			}

			req := testutil.NewJSONRequest(t, map[string]string{
				"email":    email,
				"password": password,
			})

			err := app.Login(req)
			require.NoError(t, err, "handler should not return error")

			assert.Equal(t, tt.wantStatus, testutil.GetResponseStatusCode(req))

			if tt.wantSuccessful {
				var envelope testutil.APIEnvelope
				testutil.ParseJSONResponse(t, req, &envelope)
				assert.Equal(t, "success", envelope.Status)

				var authResp handlers.AuthResponse
				testutil.ParseEnvelopeResponse(t, req, &authResp)
				assert.NotEmpty(t, authResp.AccessToken)
				assert.NotEmpty(t, authResp.RefreshToken)
				assert.Equal(t, 15*60, authResp.ExpiresIn) // 15 minutes in seconds
				assert.Equal(t, email, authResp.User.Email)
			}

			if tt.wantErrContains != "" {
				testutil.AssertErrorResponse(t, req, tt.wantStatus, tt.wantErrContains)
			}
		})
	}
}

func TestApp_Login_InvalidRequestBody(t *testing.T) {
	t.Parallel()

	app := testApp(t)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")

	err := app.Login(req)
	require.NoError(t, err)

	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_Login_DifferentRoles(t *testing.T) {
	t.Parallel()

	roles := []string{"admin", "manager", "agent"}

	for _, role := range roles {
		t.Run("role_"+role, func(t *testing.T) {
			t.Parallel()

			app := testApp(t)
			org := createTestOrganization(t, app)
			email := role + "@example.com"
			password := "testpassword123"
			createTestUser(t, app, org.ID, email, password, role, true)

			req := testutil.NewJSONRequest(t, map[string]string{
				"email":    email,
				"password": password,
			})

			err := app.Login(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

			var authResp handlers.AuthResponse
			testutil.ParseEnvelopeResponse(t, req, &authResp)
			assert.Equal(t, role, authResp.User.Role)
		})
	}
}

func TestApp_Register(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setupData       func(t *testing.T, app *handlers.App)
		request         map[string]string
		wantStatus      int
		wantErrContains string
		wantSuccessful  bool
	}{
		{
			name:      "successful registration",
			setupData: func(t *testing.T, app *handlers.App) {},
			request: map[string]string{
				"email":             "newuser@example.com",
				"password":          "securepassword123",
				"full_name":         "New User",
				"organization_name": "New Organization",
			},
			wantStatus:     fasthttp.StatusOK,
			wantSuccessful: true,
		},
		{
			name: "email already registered",
			setupData: func(t *testing.T, app *handlers.App) {
				org := createTestOrganization(t, app)
				createTestUser(t, app, org.ID, "existing@example.com", "password123", "admin", true)
			},
			request: map[string]string{
				"email":             "existing@example.com",
				"password":          "securepassword123",
				"full_name":         "Another User",
				"organization_name": "Another Org",
			},
			wantStatus:      fasthttp.StatusConflict,
			wantErrContains: "Email already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := testApp(t)
			tt.setupData(t, app)

			req := testutil.NewJSONRequest(t, tt.request)

			err := app.Register(req)
			require.NoError(t, err, "handler should not return error")

			assert.Equal(t, tt.wantStatus, testutil.GetResponseStatusCode(req))

			if tt.wantSuccessful {
				var authResp handlers.AuthResponse
				testutil.ParseEnvelopeResponse(t, req, &authResp)

				assert.NotEmpty(t, authResp.AccessToken)
				assert.NotEmpty(t, authResp.RefreshToken)
				assert.Equal(t, tt.request["email"], authResp.User.Email)
				assert.Equal(t, tt.request["full_name"], authResp.User.FullName)
				assert.Equal(t, "admin", authResp.User.Role) // First user is always admin
				assert.True(t, authResp.User.IsActive)

				// Verify organization was created
				var org models.Organization
				err := app.DB.First(&org, authResp.User.OrganizationID).Error
				require.NoError(t, err)
				assert.Equal(t, tt.request["organization_name"], org.Name)

				// Verify chatbot settings were created
				var settings models.ChatbotSettings
				err = app.DB.Where("organization_id = ?", org.ID).First(&settings).Error
				require.NoError(t, err)
				assert.False(t, settings.IsEnabled)
				assert.Equal(t, 30, settings.SessionTimeoutMins)
			}

			if tt.wantErrContains != "" {
				testutil.AssertErrorResponse(t, req, tt.wantStatus, tt.wantErrContains)
			}
		})
	}
}

func TestApp_Register_InvalidRequestBody(t *testing.T) {
	t.Parallel()

	app := testApp(t)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")

	err := app.Register(req)
	require.NoError(t, err)

	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_RefreshToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setupData       func(t *testing.T, app *handlers.App) string // returns refresh token
		wantStatus      int
		wantErrContains string
		wantSuccessful  bool
	}{
		{
			name: "successful refresh",
			setupData: func(t *testing.T, app *handlers.App) string {
				org := createTestOrganization(t, app)
				user := createTestUser(t, app, org.ID, "refresh@example.com", "password123", "admin", true)
				return generateTestRefreshToken(t, user, testJWTSecret, 7*24*time.Hour)
			},
			wantStatus:     fasthttp.StatusOK,
			wantSuccessful: true,
		},
		{
			name: "expired refresh token",
			setupData: func(t *testing.T, app *handlers.App) string {
				org := createTestOrganization(t, app)
				user := createTestUser(t, app, org.ID, "expired@example.com", "password123", "admin", true)
				return generateTestRefreshToken(t, user, testJWTSecret, -time.Hour) // Expired
			},
			wantStatus:      fasthttp.StatusUnauthorized,
			wantErrContains: "Invalid refresh token",
		},
		{
			name: "invalid token signature",
			setupData: func(t *testing.T, app *handlers.App) string {
				org := createTestOrganization(t, app)
				user := createTestUser(t, app, org.ID, "invalid@example.com", "password123", "admin", true)
				return generateTestRefreshToken(t, user, "wrong-secret-key-that-is-long", 7*24*time.Hour)
			},
			wantStatus:      fasthttp.StatusUnauthorized,
			wantErrContains: "Invalid refresh token",
		},
		{
			name: "user not found",
			setupData: func(t *testing.T, app *handlers.App) string {
				// Create a token for a non-existent user
				fakeUser := &models.User{
					BaseModel: models.BaseModel{
						ID: uuid.New(),
					},
					OrganizationID: uuid.New(),
					Email:          "fake@example.com",
					Role:           "admin",
				}
				return generateTestRefreshToken(t, fakeUser, testJWTSecret, 7*24*time.Hour)
			},
			wantStatus:      fasthttp.StatusUnauthorized,
			wantErrContains: "User not found",
		},
		{
			name: "disabled user",
			setupData: func(t *testing.T, app *handlers.App) string {
				org := createTestOrganization(t, app)
				user := createTestUser(t, app, org.ID, "disabled@example.com", "password123", "admin", false)
				return generateTestRefreshToken(t, user, testJWTSecret, 7*24*time.Hour)
			},
			wantStatus:      fasthttp.StatusUnauthorized,
			wantErrContains: "Account is disabled",
		},
		{
			name: "malformed token",
			setupData: func(t *testing.T, app *handlers.App) string {
				return "not.a.valid.jwt.token"
			},
			wantStatus:      fasthttp.StatusUnauthorized,
			wantErrContains: "Invalid refresh token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := testApp(t)
			refreshToken := tt.setupData(t, app)

			req := testutil.NewJSONRequest(t, map[string]string{
				"refresh_token": refreshToken,
			})

			err := app.RefreshToken(req)
			require.NoError(t, err, "handler should not return error")

			assert.Equal(t, tt.wantStatus, testutil.GetResponseStatusCode(req))

			if tt.wantSuccessful {
				var authResp handlers.AuthResponse
				testutil.ParseEnvelopeResponse(t, req, &authResp)

				assert.NotEmpty(t, authResp.AccessToken)
				assert.NotEmpty(t, authResp.RefreshToken)
				assert.Equal(t, 15*60, authResp.ExpiresIn)
			}

			if tt.wantErrContains != "" {
				testutil.AssertErrorResponse(t, req, tt.wantStatus, tt.wantErrContains)
			}
		})
	}
}

func TestApp_RefreshToken_InvalidRequestBody(t *testing.T) {
	t.Parallel()

	app := testApp(t)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")

	err := app.RefreshToken(req)
	require.NoError(t, err)

	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GeneratedTokensAreValid(t *testing.T) {
	t.Parallel()

	app := testApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, "tokentest@example.com", "password123", "admin", true)

	// Login to get tokens
	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    "tokentest@example.com",
		"password": "password123",
	})

	err := app.Login(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var authResp handlers.AuthResponse
	testutil.ParseEnvelopeResponse(t, req, &authResp)

	// Verify access token can be parsed
	accessToken, err := jwt.ParseWithClaims(authResp.AccessToken, &handlers.JWTClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	})
	require.NoError(t, err)
	require.True(t, accessToken.Valid)

	accessClaims, ok := accessToken.Claims.(*handlers.JWTClaims)
	require.True(t, ok)
	assert.Equal(t, user.ID, accessClaims.UserID)
	assert.Equal(t, org.ID, accessClaims.OrganizationID)
	assert.Equal(t, user.Email, accessClaims.Email)
	assert.Equal(t, user.Role, accessClaims.Role)
	assert.Equal(t, "whatomate", accessClaims.Issuer)

	// Verify refresh token can be parsed
	refreshToken, err := jwt.ParseWithClaims(authResp.RefreshToken, &handlers.JWTClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	})
	require.NoError(t, err)
	require.True(t, refreshToken.Valid)

	refreshClaims, ok := refreshToken.Claims.(*handlers.JWTClaims)
	require.True(t, ok)
	assert.Equal(t, user.ID, refreshClaims.UserID)
}
