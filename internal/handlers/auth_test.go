package handlers_test

import (
	"encoding/json"
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
	"github.com/zerodha/fastglue"
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

// uniqueEmail generates a unique email for test isolation.
func uniqueEmail(prefix string) string {
	return prefix + "-" + uuid.New().String()[:8] + "@example.com"
}

// createTestOrganization creates a test organization in the database.
func createTestOrganization(t *testing.T, app *handlers.App) *models.Organization {
	t.Helper()

	org := &models.Organization{
		Name: "Test Organization " + uuid.New().String()[:8],
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
		IsActive:       true, // Create with default, then update if needed
	}
	require.NoError(t, app.DB.Create(user).Error)

	// Explicitly update IsActive if false (GORM ignores false due to default:true tag)
	if !isActive {
		require.NoError(t, app.DB.Model(user).Update("is_active", false).Error)
		user.IsActive = false
	}
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

// assertErrorResponse checks that the response contains an error message.
func assertErrorResponse(t *testing.T, req *fastglue.Request, expectedStatus int, expectedMessage string) {
	t.Helper()

	assert.Equal(t, expectedStatus, testutil.GetResponseStatusCode(req))

	body := testutil.GetResponseBody(req)
	assert.Contains(t, string(body), expectedMessage)
}

func TestApp_Login_Success(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	email := uniqueEmail("login-success")
	password := "validpassword123"
	createTestUser(t, app, org.ID, email, password, "admin", true)

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    email,
		"password": password,
	})

	err := app.Login(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Parse the response
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			User         struct {
				Email string `json:"email"`
				Role  string `json:"role"`
			} `json:"user"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.NotEmpty(t, resp.Data.AccessToken)
	assert.NotEmpty(t, resp.Data.RefreshToken)
	assert.Equal(t, 15*60, resp.Data.ExpiresIn)
	assert.Equal(t, email, resp.Data.User.Email)
}

func TestApp_Login_WrongPassword(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	email := uniqueEmail("wrong-pwd")
	createTestUser(t, app, org.ID, email, "correctpassword", "admin", true)

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    email,
		"password": "wrongpassword",
	})

	err := app.Login(req)
	require.NoError(t, err)
	assertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid credentials")
}

func TestApp_Login_UserNotFound(t *testing.T) {
	app := testApp(t)

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    uniqueEmail("nonexistent"),
		"password": "anypassword",
	})

	err := app.Login(req)
	require.NoError(t, err)
	assertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid credentials")
}

func TestApp_Login_InactiveUser(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	email := uniqueEmail("inactive")
	createTestUser(t, app, org.ID, email, "validpassword123", "admin", false)

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    email,
		"password": "validpassword123",
	})

	err := app.Login(req)
	require.NoError(t, err)
	assertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Account is disabled")
}

func TestApp_Login_InvalidRequestBody(t *testing.T) {
	app := testApp(t)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")

	err := app.Login(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_Login_DifferentRoles(t *testing.T) {
	roles := []string{"admin", "manager", "agent"}

	for _, role := range roles {
		t.Run("role_"+role, func(t *testing.T) {
			app := testApp(t)
			org := createTestOrganization(t, app)
			email := uniqueEmail("role-" + role)
			password := "testpassword123"
			createTestUser(t, app, org.ID, email, password, role, true)

			req := testutil.NewJSONRequest(t, map[string]string{
				"email":    email,
				"password": password,
			})

			err := app.Login(req)
			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

			var resp struct {
				Data struct {
					User struct {
						Role string `json:"role"`
					} `json:"user"`
				} `json:"data"`
			}
			_ = json.Unmarshal(testutil.GetResponseBody(req), &resp)
			assert.Equal(t, role, resp.Data.User.Role)
		})
	}
}

func TestApp_Register_Success(t *testing.T) {
	app := testApp(t)
	email := uniqueEmail("register")

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":             email,
		"password":          "securepassword123",
		"full_name":         "New User",
		"organization_name": "New Organization " + uuid.New().String()[:8],
	})

	err := app.Register(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			User         struct {
				Email    string `json:"email"`
				FullName string `json:"full_name"`
				Role     string `json:"role"`
				IsActive bool   `json:"is_active"`
			} `json:"user"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.NotEmpty(t, resp.Data.AccessToken)
	assert.NotEmpty(t, resp.Data.RefreshToken)
	assert.Equal(t, email, resp.Data.User.Email)
	assert.Equal(t, "New User", resp.Data.User.FullName)
	assert.Equal(t, "admin", resp.Data.User.Role)
	assert.True(t, resp.Data.User.IsActive)
}

func TestApp_Register_EmailAlreadyExists(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	email := uniqueEmail("existing")
	createTestUser(t, app, org.ID, email, "password123", "admin", true)

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":             email,
		"password":          "securepassword123",
		"full_name":         "Another User",
		"organization_name": "Another Org",
	})

	err := app.Register(req)
	require.NoError(t, err)
	assertErrorResponse(t, req, fasthttp.StatusConflict, "Email already registered")
}

func TestApp_Register_InvalidRequestBody(t *testing.T) {
	app := testApp(t)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")

	err := app.Register(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_RefreshToken_Success(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("refresh"), "password123", "admin", true)
	refreshToken := generateTestRefreshToken(t, user, testJWTSecret, 7*24*time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": refreshToken,
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.NotEmpty(t, resp.Data.AccessToken)
	assert.NotEmpty(t, resp.Data.RefreshToken)
	assert.Equal(t, 15*60, resp.Data.ExpiresIn)
}

func TestApp_RefreshToken_Expired(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("expired"), "password123", "admin", true)
	expiredToken := generateTestRefreshToken(t, user, testJWTSecret, -time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": expiredToken,
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	assertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid refresh token")
}

func TestApp_RefreshToken_InvalidSignature(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("invalid-sig"), "password123", "admin", true)
	wrongSecretToken := generateTestRefreshToken(t, user, "wrong-secret-key-that-is-long", 7*24*time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": wrongSecretToken,
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	assertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid refresh token")
}

func TestApp_RefreshToken_UserNotFound(t *testing.T) {
	app := testApp(t)
	fakeUser := &models.User{
		BaseModel: models.BaseModel{
			ID: uuid.New(),
		},
		OrganizationID: uuid.New(),
		Email:          "fake@example.com",
		Role:           "admin",
	}
	token := generateTestRefreshToken(t, fakeUser, testJWTSecret, 7*24*time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": token,
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	assertErrorResponse(t, req, fasthttp.StatusUnauthorized, "User not found")
}

func TestApp_RefreshToken_DisabledUser(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("disabled"), "password123", "admin", false)
	token := generateTestRefreshToken(t, user, testJWTSecret, 7*24*time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": token,
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	assertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Account is disabled")
}

func TestApp_RefreshToken_MalformedToken(t *testing.T) {
	app := testApp(t)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": "not.a.valid.jwt.token",
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	assertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid refresh token")
}

func TestApp_RefreshToken_InvalidRequestBody(t *testing.T) {
	app := testApp(t)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")

	err := app.RefreshToken(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GeneratedTokensAreValid(t *testing.T) {
	app := testApp(t)
	org := createTestOrganization(t, app)
	email := uniqueEmail("tokentest")
	user := createTestUser(t, app, org.ID, email, "password123", "admin", true)

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    email,
		"password": "password123",
	})

	err := app.Login(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	// Verify access token can be parsed
	accessToken, err := jwt.ParseWithClaims(resp.Data.AccessToken, &handlers.JWTClaims{}, func(token *jwt.Token) (any, error) {
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
	refreshToken, err := jwt.ParseWithClaims(resp.Data.RefreshToken, &handlers.JWTClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	})
	require.NoError(t, err)
	require.True(t, refreshToken.Valid)

	refreshClaims, ok := refreshToken.Claims.(*handlers.JWTClaims)
	require.True(t, ok)
	assert.Equal(t, user.ID, refreshClaims.UserID)
}
