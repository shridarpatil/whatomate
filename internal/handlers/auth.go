package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/middleware"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest represents login credentials
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=12"`
}

// RegisterRequest represents registration data
type RegisterRequest struct {
	Email          string    `json:"email" validate:"required,email"`
	Password       string    `json:"password" validate:"required,min=12"`
	FullName       string    `json:"full_name" validate:"required"`
	OrganizationID uuid.UUID `json:"organization_id" validate:"required"`
}

// CookieAuthResponse represents authentication response when tokens are in cookies.
// No tokens in the body — only the expiry hint and user object.
type CookieAuthResponse struct {
	ExpiresIn int         `json:"expires_in"`
	User      models.User `json:"user"`
}

// RefreshRequest represents token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
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
		// Run dummy bcrypt to prevent timing-based account enumeration
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"), []byte(req.Password))
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

	a.setAuthCookies(r, accessToken, refreshToken)

	return r.SendEnvelope(CookieAuthResponse{
		ExpiresIn: a.Config.JWT.AccessExpiryMins * 60,
		User:      user,
	})
}

// Register creates a new user in an existing organization
func (a *App) Register(r *fastglue.Request) error {
	var req RegisterRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.OrganizationID == uuid.Nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "organization_id is required", nil, "")
	}

	// Validate the organization exists
	var org models.Organization
	if err := a.DB.Where("id = ?", req.OrganizationID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	// Get the org's default role
	var defaultRole models.CustomRole
	if err := a.DB.Where("organization_id = ? AND is_default = ?", req.OrganizationID, true).First(&defaultRole).Error; err != nil {
		if err := a.DB.Where("organization_id = ? AND name = ? AND is_system = ?", req.OrganizationID, "agent", true).First(&defaultRole).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to find default role", nil, "")
		}
	}

	// Check if email already exists
	var existingUser models.User
	if err := a.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		// User exists — verify password and add to this org
		if err := bcrypt.CompareHashAndPassword([]byte(existingUser.PasswordHash), []byte(req.Password)); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusConflict, "An account with this email already exists. Please sign in and ask your organization admin to add you.", nil, "")
		}

		// Check if user account is disabled
		if !existingUser.IsActive {
			return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Account is disabled", nil, "")
		}

		// Check if already a member of this org
		var count int64
		a.DB.Model(&models.UserOrganization{}).
			Where("user_id = ? AND organization_id = ?", existingUser.ID, req.OrganizationID).
			Count(&count)
		if count > 0 {
			return r.SendErrorEnvelope(fasthttp.StatusConflict, "You are already a member of this organization", nil, "")
		}

		// Add as member with default role
		userOrg := models.UserOrganization{
			UserID:         existingUser.ID,
			OrganizationID: req.OrganizationID,
			RoleID:         &defaultRole.ID,
			IsDefault:      false,
		}
		if err := a.DB.Create(&userOrg).Error; err != nil {
			a.Log.Error("Failed to add existing user to organization", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to join organization", nil, "")
		}

		a.Log.Info("Existing user joined organization", "user_id", existingUser.ID, "org_id", req.OrganizationID)

		// Set org context to the new org for token generation
		existingUser.OrganizationID = req.OrganizationID
		existingUser.Role = &defaultRole
		existingUser.RoleID = &defaultRole.ID

		accessToken, _ := a.generateAccessToken(&existingUser)
		refreshToken, _ := a.generateRefreshToken(&existingUser)

		a.setAuthCookies(r, accessToken, refreshToken)

		return r.SendEnvelope(CookieAuthResponse{
			ExpiresIn: a.Config.JWT.AccessExpiryMins * 60,
			User:      existingUser,
		})
	}

	// New user — run dummy bcrypt to prevent timing-based account enumeration
	_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"), []byte(req.Password))

	// Create account
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("Failed to hash password", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	tx := a.DB.Begin()
	if tx.Error != nil {
		a.Log.Error("Failed to begin transaction", "error", tx.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	user := models.User{
		OrganizationID: req.OrganizationID,
		Email:          req.Email,
		PasswordHash:   string(hashedPassword),
		FullName:       req.FullName,
		RoleID:         &defaultRole.ID,
		IsActive:       true,
	}

	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to create user", "error", err, "email", req.Email, "org_id", req.OrganizationID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	userOrg := models.UserOrganization{
		UserID:         user.ID,
		OrganizationID: req.OrganizationID,
		RoleID:         &defaultRole.ID,
		IsDefault:      true,
	}
	if err := tx.Create(&userOrg).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to create user organization entry", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	if err := tx.Commit().Error; err != nil {
		a.Log.Error("Failed to commit transaction", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	a.Log.Info("Registration completed", "user_id", user.ID, "org_id", req.OrganizationID)

	user.Role = &defaultRole

	accessToken, _ := a.generateAccessToken(&user)
	refreshToken, _ := a.generateRefreshToken(&user)

	a.setAuthCookies(r, accessToken, refreshToken)

	return r.SendEnvelope(CookieAuthResponse{
		ExpiresIn: a.Config.JWT.AccessExpiryMins * 60,
		User:      user,
	})
}

// RefreshToken refreshes access token using refresh token with rotation.
// The old refresh token is invalidated (single-use) and a new one is issued.
func (a *App) RefreshToken(r *fastglue.Request) error {
	// Read refresh token from cookie first, fall back to JSON body.
	refreshTokenStr := string(r.RequestCtx.Request.Header.Cookie(cookieRefreshName))
	if refreshTokenStr == "" {
		var req RefreshRequest
		_ = r.Decode(&req, "json")
		refreshTokenStr = req.RefreshToken
	}
	if refreshTokenStr == "" {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Missing refresh token", nil, "")
	}

	// Parse and validate refresh token
	token, err := jwt.ParseWithClaims(refreshTokenStr, &middleware.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.Config.JWT.Secret), nil
	})

	if err != nil || !token.Valid {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid refresh token", nil, "")
	}

	claims, ok := token.Claims.(*middleware.JWTClaims)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid token claims", nil, "")
	}

	// Validate JTI in Redis (single-use: delete on consumption)
	if claims.ID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		deleted, err := a.Redis.Del(ctx, refreshTokenKey(claims.ID)).Result()
		if err != nil || deleted == 0 {
			// Token was already used or revoked
			return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Refresh token has been revoked", nil, "")
		}
	}

	// Get user
	var user models.User
	if err := a.DB.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User not found", nil, "")
	}

	if !user.IsActive {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Account is disabled", nil, "")
	}

	// Generate new tokens (rotation: new refresh token with new JTI)
	accessToken, _ := a.generateAccessToken(&user)
	newRefreshToken, _ := a.generateRefreshToken(&user)

	a.setAuthCookies(r, accessToken, newRefreshToken)

	return r.SendEnvelope(CookieAuthResponse{
		ExpiresIn: a.Config.JWT.AccessExpiryMins * 60,
		User:      user,
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
	jti := uuid.New().String()
	expiry := time.Duration(a.Config.JWT.RefreshExpiryDays) * 24 * time.Hour

	claims := middleware.JWTClaims{
		UserID:         user.ID,
		OrganizationID: user.OrganizationID,
		Email:          user.Email,
		RoleID:         user.RoleID,
		IsSuperAdmin:   user.IsSuperAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "whatomate",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(a.Config.JWT.Secret))
	if err != nil {
		return "", err
	}

	// Store JTI in Redis so it can be revoked
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Redis.Set(ctx, refreshTokenKey(jti), user.ID.String(), expiry).Err(); err != nil {
		a.Log.Error("Failed to store refresh token in Redis", "error", err)
	}

	return signed, nil
}

// refreshTokenKey returns the Redis key for a refresh token JTI.
func refreshTokenKey(jti string) string {
	return fmt.Sprintf("refresh:%s", jti)
}

// SwitchOrgRequest represents the request body for switching organization
type SwitchOrgRequest struct {
	OrganizationID uuid.UUID `json:"organization_id"`
}

// SwitchOrg generates new tokens for a different organization the user belongs to
func (a *App) SwitchOrg(r *fastglue.Request) error {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req SwitchOrgRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.OrganizationID == uuid.Nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "organization_id is required", nil, "")
	}

	// Verify the organization exists
	var org models.Organization
	if err := a.DB.Where("id = ?", req.OrganizationID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	// Get the user
	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	// Super admins can switch to any org; others need membership
	if !user.IsSuperAdmin {
		var userOrg models.UserOrganization
		if err := a.DB.Where("user_id = ? AND organization_id = ?", userID, req.OrganizationID).First(&userOrg).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You are not a member of this organization", nil, "")
		}
		// Use the role from the user_organizations table for the target org
		if userOrg.RoleID != nil {
			user.RoleID = userOrg.RoleID
		}
	}

	// Set the target org on the user for token generation
	user.OrganizationID = req.OrganizationID

	// Preload role with permissions for the response
	if user.RoleID != nil {
		var role models.CustomRole
		if err := a.DB.Where("id = ?", *user.RoleID).First(&role).Error; err == nil {
			user.Role = &role
			cachedPerms, err := a.GetRolePermissionsCached(*user.RoleID)
			if err == nil {
				permissions := make([]models.Permission, 0, len(cachedPerms))
				for _, p := range cachedPerms {
					parts := splitPermission(p)
					if len(parts) == 2 {
						permissions = append(permissions, models.Permission{
							Resource: parts[0],
							Action:   parts[1],
						})
					}
				}
				user.Role.Permissions = permissions
			}
		}
	}

	// Generate new tokens with the target org
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

	a.setAuthCookies(r, accessToken, refreshToken)

	return r.SendEnvelope(CookieAuthResponse{
		ExpiresIn: a.Config.JWT.AccessExpiryMins * 60,
		User:      user,
	})
}

// LogoutRequest represents logout request body
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Logout invalidates the user's refresh token
func (a *App) Logout(r *fastglue.Request) error {
	// Read refresh token from cookie first, fall back to body.
	refreshTokenStr := string(r.RequestCtx.Request.Header.Cookie(cookieRefreshName))
	if refreshTokenStr == "" {
		var req LogoutRequest
		_ = r.Decode(&req, "json")
		refreshTokenStr = req.RefreshToken
	}

	if refreshTokenStr != "" {
		// Parse the token to extract JTI (don't need to fully validate — just extract claims)
		token, _ := jwt.ParseWithClaims(refreshTokenStr, &middleware.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(a.Config.JWT.Secret), nil
		})
		if token != nil {
			if claims, ok := token.Claims.(*middleware.JWTClaims); ok && claims.ID != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				a.Redis.Del(ctx, refreshTokenKey(claims.ID))
			}
		}
	}

	a.clearAuthCookies(r)

	return r.SendEnvelope(map[string]string{"status": "logged_out"})
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

// GetWSToken returns a short-lived single-use JWT for WebSocket authentication.
// This is needed because httpOnly cookies cannot be read by JavaScript to pass
// as a query parameter to the WebSocket connection URL.
func (a *App) GetWSToken(r *fastglue.Request) error {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	orgID, _ := r.RequestCtx.UserValue("organization_id").(uuid.UUID)

	claims := middleware.JWTClaims{
		UserID:         userID,
		OrganizationID: orgID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "whatomate",
			Subject:   "ws",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(a.Config.JWT.Secret))
	if err != nil {
		a.Log.Error("Failed to generate WS token", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate token", nil, "")
	}

	return r.SendEnvelope(map[string]string{"token": signed})
}
