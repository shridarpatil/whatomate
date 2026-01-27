package handlers_test

import (
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

func TestApp_Login_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	email := testutil.UniqueEmail("login-success")
	password := "validpassword123"
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email), testutil.WithPassword(password))

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	email := testutil.UniqueEmail("wrong-pwd")
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email), testutil.WithPassword("correctpassword"))

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    email,
		"password": "wrongpassword",
	})

	err := app.Login(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid credentials")
}

func TestApp_Login_UserNotFound(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    testutil.UniqueEmail("nonexistent"),
		"password": "anypassword",
	})

	err := app.Login(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid credentials")
}

func TestApp_Login_InactiveUser(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	email := testutil.UniqueEmail("inactive")
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email), testutil.WithPassword("validpassword123"), testutil.WithInactive())

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    email,
		"password": "validpassword123",
	})

	err := app.Login(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Account is disabled")
}

func TestApp_Login_InvalidRequestBody(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")

	err := app.Login(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_Login_UserWithRole(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	email := testutil.UniqueEmail("role-test")
	password := "testpassword123"

	// Create an actual role first
	role := &models.CustomRole{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "Test Role " + uuid.New().String()[:8],
		IsSystem:       false,
	}
	require.NoError(t, app.DB.Create(role).Error)

	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email), testutil.WithPassword(password), testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":    email,
		"password": password,
	})

	err := app.Login(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

func TestApp_Register_Success(t *testing.T) {
	app := newTestApp(t)
	email := testutil.UniqueEmail("register")

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
				ID       string `json:"id"`
				Email    string `json:"email"`
				FullName string `json:"full_name"`
				RoleID   string `json:"role_id"`
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
	assert.NotEmpty(t, resp.Data.User.RoleID, "User should have a role assigned")
	assert.True(t, resp.Data.User.IsActive)

	// Verify the user has admin role in the database
	userID, err := uuid.Parse(resp.Data.User.ID)
	require.NoError(t, err)
	var user models.User
	require.NoError(t, app.DB.Preload("Role").Where("id = ?", userID).First(&user).Error)
	assert.NotNil(t, user.Role)
	assert.Equal(t, "admin", user.Role.Name)
	assert.True(t, user.Role.IsSystem)
}

func TestApp_Register_EmailAlreadyExists(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	email := testutil.UniqueEmail("existing")
	testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email), testutil.WithPassword("password123"))

	req := testutil.NewJSONRequest(t, map[string]string{
		"email":             email,
		"password":          "securepassword123",
		"full_name":         "Another User",
		"organization_name": "Another Org",
	})

	err := app.Register(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusConflict, "Email already registered")
}

func TestApp_Register_InvalidRequestBody(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")

	err := app.Register(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_RefreshToken_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("refresh")), testutil.WithPassword("password123"))
	refreshToken := testutil.GenerateTestRefreshToken(t, user, testutil.TestJWTSecret, 7*24*time.Hour)

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
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("expired")), testutil.WithPassword("password123"))
	expiredToken := testutil.GenerateTestRefreshToken(t, user, testutil.TestJWTSecret, -time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": expiredToken,
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid refresh token")
}

func TestApp_RefreshToken_InvalidSignature(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("invalid-sig")), testutil.WithPassword("password123"))
	wrongSecretToken := testutil.GenerateTestRefreshToken(t, user, "wrong-secret-key-that-is-long", 7*24*time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": wrongSecretToken,
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid refresh token")
}

func TestApp_RefreshToken_UserNotFound(t *testing.T) {
	app := newTestApp(t)
	fakeUser := &models.User{
		BaseModel: models.BaseModel{
			ID: uuid.New(),
		},
		OrganizationID: uuid.New(),
		Email:          "fake@example.com",
	}
	token := testutil.GenerateTestRefreshToken(t, fakeUser, testutil.TestJWTSecret, 7*24*time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": token,
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "User not found")
}

func TestApp_RefreshToken_DisabledUser(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("disabled")), testutil.WithPassword("password123"), testutil.WithInactive())
	token := testutil.GenerateTestRefreshToken(t, user, testutil.TestJWTSecret, 7*24*time.Hour)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": token,
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Account is disabled")
}

func TestApp_RefreshToken_MalformedToken(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewJSONRequest(t, map[string]string{
		"refresh_token": "not.a.valid.jwt.token",
	})

	err := app.RefreshToken(req)
	require.NoError(t, err)
	testutil.AssertErrorResponse(t, req, fasthttp.StatusUnauthorized, "Invalid refresh token")
}

func TestApp_RefreshToken_InvalidRequestBody(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewRequest(t)
	req.RequestCtx.Request.SetBody([]byte("invalid json"))
	req.RequestCtx.Request.Header.SetContentType("application/json")

	err := app.RefreshToken(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GeneratedTokensAreValid(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	email := testutil.UniqueEmail("tokentest")
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(email), testutil.WithPassword("password123"))

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
	accessToken, err := jwt.ParseWithClaims(resp.Data.AccessToken, &middleware.JWTClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(testutil.TestJWTSecret), nil
	})
	require.NoError(t, err)
	require.True(t, accessToken.Valid)

	accessClaims, ok := accessToken.Claims.(*middleware.JWTClaims)
	require.True(t, ok)
	assert.Equal(t, user.ID, accessClaims.UserID)
	assert.Equal(t, org.ID, accessClaims.OrganizationID)
	assert.Equal(t, user.Email, accessClaims.Email)
	assert.Equal(t, user.RoleID, accessClaims.RoleID)
	assert.Equal(t, "whatomate", accessClaims.Issuer)

	// Verify refresh token can be parsed
	refreshToken, err := jwt.ParseWithClaims(resp.Data.RefreshToken, &middleware.JWTClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(testutil.TestJWTSecret), nil
	})
	require.NoError(t, err)
	require.True(t, refreshToken.Valid)

	refreshClaims, ok := refreshToken.Claims.(*middleware.JWTClaims)
	require.True(t, ok)
	assert.Equal(t, user.ID, refreshClaims.UserID)
}
