package handlers

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/database"
	"github.com/shridarpatil/whatomate/internal/middleware"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest represents login credentials
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// RegisterRequest represents registration data
type RegisterRequest struct {
	Email            string `json:"email" validate:"required,email"`
	Password         string `json:"password" validate:"required,min=8"`
	FullName         string `json:"full_name" validate:"required"`
	OrganizationName string `json:"organization_name" validate:"required"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int         `json:"expires_in"`
	User         models.User `json:"user"`
}

// RefreshRequest represents token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Login authenticates a user and returns tokens
func (a *App) Login(r *fastglue.Request) error {
	var req LoginRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Find user by email with role preloaded
	var user models.User
	if err := a.DB.Preload("Role").Where("email = ?", req.Email).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid credentials", nil, "")
	}

	// Load permissions from cache
	if user.Role != nil && user.RoleID != nil {
		cachedPerms, err := a.GetRolePermissionsCached(*user.RoleID)
		if err == nil {
			permissions := make([]models.Permission, 0, len(cachedPerms))
			for _, p := range cachedPerms {
				for i := len(p) - 1; i >= 0; i-- {
					if p[i] == ':' {
						permissions = append(permissions, models.Permission{
							Resource: p[:i],
							Action:   p[i+1:],
						})
						break
					}
				}
			}
			user.Role.Permissions = permissions
		}
	}

	// Check if user is active
	if !user.IsActive {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Account is disabled", nil, "")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid credentials", nil, "")
	}

	// Generate tokens
	accessToken, err := a.generateAccessToken(&user)
	if err != nil {
		a.Log.Error("Failed to generate access token", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate token", nil, "")
	}

	refreshToken, err := a.generateRefreshToken(&user)
	if err != nil {
		a.Log.Error("Failed to generate refresh token", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate token", nil, "")
	}

	return r.SendEnvelope(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    a.Config.JWT.AccessExpiryMins * 60,
		User:         user,
	})
}

// Register creates a new user and organization
func (a *App) Register(r *fastglue.Request) error {
	var req RegisterRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Check if email already exists
	var existingUser models.User
	if err := a.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Email already registered", nil, "")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("Failed to hash password", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	// Create organization
	org := models.Organization{
		Name: req.OrganizationName,
		Slug: generateSlug(req.OrganizationName),
	}

	// Start transaction
	tx := a.DB.Begin()
	if tx.Error != nil {
		a.Log.Error("Failed to begin transaction", "error", tx.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	if err := tx.Create(&org).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to create organization", "error", err, "org_name", req.OrganizationName)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	a.Log.Info("Created organization", "org_id", org.ID, "org_name", org.Name)

	// Seed system roles for the new organization
	if err := database.SeedSystemRolesForOrg(tx, org.ID); err != nil {
		tx.Rollback()
		a.Log.Error("Failed to seed system roles", "error", err, "org_id", org.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	// Get admin role for this org
	var adminRole models.CustomRole
	if err := tx.Where("organization_id = ? AND name = ? AND is_system = ?", org.ID, "admin", true).First(&adminRole).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to find admin role", "error", err, "org_id", org.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	// Create user (first user of org is always admin)
	user := models.User{
		OrganizationID: org.ID,
		Email:          req.Email,
		PasswordHash:   string(hashedPassword),
		FullName:       req.FullName,
		RoleID:         &adminRole.ID,
		IsActive:       true,
	}

	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to create user", "error", err, "email", req.Email, "org_id", org.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	a.Log.Info("Created user", "user_id", user.ID, "email", user.Email)

	// Create default chatbot settings
	chatbotSettings := models.ChatbotSettings{
		OrganizationID:     org.ID,
		IsEnabled:          false,
		SessionTimeoutMins: 30,
	}

	if err := tx.Create(&chatbotSettings).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to create chatbot settings", "error", err, "org_id", org.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	if err := tx.Commit().Error; err != nil {
		a.Log.Error("Failed to commit transaction", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	a.Log.Info("Registration completed", "user_id", user.ID, "org_id", org.ID)

	// Populate the role for the response
	user.Role = &adminRole

	// Generate tokens
	accessToken, _ := a.generateAccessToken(&user)
	refreshToken, _ := a.generateRefreshToken(&user)

	return r.SendEnvelope(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    a.Config.JWT.AccessExpiryMins * 60,
		User:         user,
	})
}

// RefreshToken refreshes access token using refresh token
func (a *App) RefreshToken(r *fastglue.Request) error {
	var req RefreshRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Parse and validate refresh token
	token, err := jwt.ParseWithClaims(req.RefreshToken, &middleware.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.Config.JWT.Secret), nil
	})

	if err != nil || !token.Valid {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid refresh token", nil, "")
	}

	claims, ok := token.Claims.(*middleware.JWTClaims)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid token claims", nil, "")
	}

	// Get user
	var user models.User
	if err := a.DB.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User not found", nil, "")
	}

	if !user.IsActive {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Account is disabled", nil, "")
	}

	// Generate new tokens
	accessToken, _ := a.generateAccessToken(&user)
	refreshToken, _ := a.generateRefreshToken(&user)

	return r.SendEnvelope(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    a.Config.JWT.AccessExpiryMins * 60,
		User:         user,
	})
}

func (a *App) generateAccessToken(user *models.User) (string, error) {
	claims := middleware.JWTClaims{
		UserID:         user.ID,
		OrganizationID: user.OrganizationID,
		Email:          user.Email,
		RoleID:         user.RoleID,
		IsSuperAdmin:   user.IsSuperAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(a.Config.JWT.AccessExpiryMins) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "whatomate",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.Config.JWT.Secret))
}

func (a *App) generateRefreshToken(user *models.User) (string, error) {
	claims := middleware.JWTClaims{
		UserID:         user.ID,
		OrganizationID: user.OrganizationID,
		Email:          user.Email,
		RoleID:         user.RoleID,
		IsSuperAdmin:   user.IsSuperAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(a.Config.JWT.RefreshExpiryDays) * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "whatomate",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.Config.JWT.Secret))
}

func generateSlug(name string) string {
	// Simple slug generation - in production, use a proper slugify library
	slug := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			slug += string(c)
		} else if c >= 'A' && c <= 'Z' {
			slug += string(c + 32)
		} else if c == ' ' || c == '-' {
			slug += "-"
		}
	}
	return slug + "-" + uuid.New().String()[:8]
}
