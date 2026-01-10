package middleware

import (
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"github.com/zerodha/logf"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Context keys
const (
	ContextKeyUserID         = "user_id"
	ContextKeyOrganizationID = "organization_id"
	ContextKeyEmail          = "email"
	ContextKeyRole           = "role"
	ContextKeyUser           = "user"
	ContextKeyOrganization   = "organization"
)

// JWTClaims represents JWT claims
type JWTClaims struct {
	UserID         uuid.UUID `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	jwt.RegisteredClaims
}

// RequestLogger logs incoming requests
func RequestLogger(log logf.Logger) fastglue.FastMiddleware {
	return func(r *fastglue.Request) *fastglue.Request {
		start := time.Now()

		// Store start time for later use
		r.RequestCtx.SetUserValue("request_start", start)

		return r
	}
}

// CORS handles Cross-Origin Resource Sharing
func CORS() fastglue.FastMiddleware {
	return func(r *fastglue.Request) *fastglue.Request {
		origin := string(r.RequestCtx.Request.Header.Peek("Origin"))
		if origin == "" {
			origin = "*"
		}

		r.RequestCtx.Response.Header.Set("Access-Control-Allow-Origin", origin)
		r.RequestCtx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		r.RequestCtx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Requested-With")
		r.RequestCtx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		r.RequestCtx.Response.Header.Set("Access-Control-Max-Age", "86400")

		return r
	}
}

// Recovery recovers from panics
func Recovery(log logf.Logger) fastglue.FastMiddleware {
	return func(r *fastglue.Request) *fastglue.Request {
		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic recovered", "error", err, "path", string(r.RequestCtx.Path()))
				r.RequestCtx.SetStatusCode(fasthttp.StatusInternalServerError)
				r.RequestCtx.SetBodyString(`{"status":"error","message":"Internal server error"}`)
			}
		}()
		return r
	}
}

// Auth validates JWT tokens (legacy - use AuthWithDB for API key support)
func Auth(secret string) fastglue.FastMiddleware {
	return AuthWithDB(secret, nil)
}

// AuthWithDB validates both JWT tokens and API keys
func AuthWithDB(secret string, db *gorm.DB) fastglue.FastMiddleware {
	return func(r *fastglue.Request) *fastglue.Request {
		authHeader := string(r.RequestCtx.Request.Header.Peek("Authorization"))
		apiKey := string(r.RequestCtx.Request.Header.Peek("X-API-Key"))

		// Try API key authentication first
		if apiKey != "" && db != nil {
			if validateAPIKey(r, apiKey, db) {
				return r
			}
			// API key was provided but invalid
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid API key", nil, "")
			return nil
		}

		// Fall back to JWT authentication
		if authHeader == "" {
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Missing authorization header", nil, "")
			return nil
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid authorization header format", nil, "")
			return nil
		}

		tokenString := parts[1]

		// Parse and validate token
		token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid or expired token", nil, "")
			return nil
		}

		claims, ok := token.Claims.(*JWTClaims)
		if !ok {
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid token claims", nil, "")
			return nil
		}

		// Store claims in context
		r.RequestCtx.SetUserValue(ContextKeyUserID, claims.UserID)
		r.RequestCtx.SetUserValue(ContextKeyOrganizationID, claims.OrganizationID)
		r.RequestCtx.SetUserValue(ContextKeyEmail, claims.Email)
		r.RequestCtx.SetUserValue(ContextKeyRole, claims.Role)

		return r
	}
}

// validateAPIKey validates an API key and sets context values
func validateAPIKey(r *fastglue.Request, key string, db *gorm.DB) bool {
	// API key format: whm_<32 hex chars>
	if len(key) != 36 || key[:4] != "whm_" {
		return false
	}

	// Extract prefix for lookup (first 8 chars after "whm_")
	keyPrefix := key[4:12]

	// Find API keys with matching prefix
	var apiKeys []models.APIKey
	if err := db.Preload("User").Where("key_prefix = ? AND is_active = ?", keyPrefix, true).Find(&apiKeys).Error; err != nil {
		return false
	}

	// Check each key with bcrypt
	for _, apiKey := range apiKeys {
		if err := bcrypt.CompareHashAndPassword([]byte(apiKey.KeyHash), []byte(key)); err == nil {
			// Key matches - check expiration
			if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
				return false // Key expired
			}

			// Update last used timestamp (async to not block request)
			go func(id uuid.UUID) {
				now := time.Now()
				db.Model(&models.APIKey{}).Where("id = ?", id).Update("last_used_at", now)
			}(apiKey.ID)

			// Set context values from the user who created the key
			if apiKey.User != nil {
				r.RequestCtx.SetUserValue(ContextKeyUserID, apiKey.UserID)
				r.RequestCtx.SetUserValue(ContextKeyOrganizationID, apiKey.OrganizationID)
				r.RequestCtx.SetUserValue(ContextKeyEmail, apiKey.User.Email)
				r.RequestCtx.SetUserValue(ContextKeyRole, apiKey.User.Role)
				return true
			}
		}
	}

	return false
}

// OrganizationContext loads organization and user from database
func OrganizationContext(db *gorm.DB) fastglue.FastMiddleware {
	return func(r *fastglue.Request) *fastglue.Request {
		userID, ok := r.RequestCtx.UserValue(ContextKeyUserID).(uuid.UUID)
		if !ok {
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User ID not found in context", nil, "")
			return nil
		}

		orgID, ok := r.RequestCtx.UserValue(ContextKeyOrganizationID).(uuid.UUID)
		if !ok {
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Organization ID not found in context", nil, "")
			return nil
		}

		// Load user
		var user models.User
		if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User not found", nil, "")
			return nil
		}

		if !user.IsActive {
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Account is disabled", nil, "")
			return nil
		}

		// Load organization
		var org models.Organization
		if err := db.Where("id = ?", orgID).First(&org).Error; err != nil {
			_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Organization not found", nil, "")
			return nil
		}

		// Store in context
		r.RequestCtx.SetUserValue(ContextKeyUser, &user)
		r.RequestCtx.SetUserValue(ContextKeyOrganization, &org)

		return r
	}
}

// RequireRole checks if user has required role
func RequireRole(roles ...string) fastglue.FastMiddleware {
	return func(r *fastglue.Request) *fastglue.Request {
		role, ok := r.RequestCtx.UserValue(ContextKeyRole).(string)
		if !ok {
			_ = r.SendErrorEnvelope(fasthttp.StatusForbidden, "Role not found", nil, "")
			return nil
		}

		for _, allowedRole := range roles {
			if role == allowedRole {
				return r
			}
		}

		_ = r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
		return nil
	}
}

// GetUserID extracts user ID from request context
func GetUserID(r *fastglue.Request) (uuid.UUID, bool) {
	userID, ok := r.RequestCtx.UserValue(ContextKeyUserID).(uuid.UUID)
	return userID, ok
}

// GetOrganizationID extracts organization ID from request context
func GetOrganizationID(r *fastglue.Request) (uuid.UUID, bool) {
	orgID, ok := r.RequestCtx.UserValue(ContextKeyOrganizationID).(uuid.UUID)
	return orgID, ok
}

// GetUser extracts user from request context
func GetUser(r *fastglue.Request) (*models.User, bool) {
	user, ok := r.RequestCtx.UserValue(ContextKeyUser).(*models.User)
	return user, ok
}

// GetOrganization extracts organization from request context
func GetOrganization(r *fastglue.Request) (*models.Organization, bool) {
	org, ok := r.RequestCtx.UserValue(ContextKeyOrganization).(*models.Organization)
	return org, ok
}
