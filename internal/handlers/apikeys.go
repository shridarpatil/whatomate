package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
)

// APIKeyRequest represents the request body for creating an API key
type APIKeyRequest struct {
	Name      string  `json:"name"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

// APIKeyResponse represents an API key in list responses
type APIKeyResponse struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  string     `json:"created_at"`
}

// APIKeyCreateResponse includes the full key (only shown once)
type APIKeyCreateResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Key       string     `json:"key"` // Full key, only returned on create
	KeyPrefix string     `json:"key_prefix"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt string     `json:"created_at"`
}

// generateAPIKey generates a random API key with whm_ prefix
func generateAPIKey() (string, error) {
	bytes := make([]byte, 16) // 32 hex chars
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "whm_" + hex.EncodeToString(bytes), nil
}

// ListAPIKeys returns all API keys for the organization
func (a *App) ListAPIKeys(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAPIKeys, models.ActionRead); err != nil {
		return nil
	}

	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	query := a.DB.Model(&models.APIKey{}).Where("organization_id = ?", orgID)

	// Apply search filter - search by name or key prefix (case-insensitive)
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("name ILIKE ? OR key_prefix ILIKE ?", searchPattern, searchPattern)
	}

	var total int64
	query.Count(&total)

	var apiKeys []models.APIKey
	if err := pg.Apply(query.Order("created_at DESC")).
		Find(&apiKeys).Error; err != nil {
		a.Log.Error("Failed to list API keys", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list API keys", nil, "")
	}

	response := make([]APIKeyResponse, len(apiKeys))
	for i, key := range apiKeys {
		response[i] = APIKeyResponse{
			ID:         key.ID,
			Name:       key.Name,
			KeyPrefix:  key.KeyPrefix,
			LastUsedAt: key.LastUsedAt,
			ExpiresAt:  key.ExpiresAt,
			IsActive:   key.IsActive,
			CreatedAt:  key.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return r.SendEnvelope(map[string]any{
		"api_keys": response,
		"total":    total,
		"page":     pg.Page,
		"limit":    pg.Limit,
	})
}

// CreateAPIKey creates a new API key
func (a *App) CreateAPIKey(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAPIKeys, models.ActionWrite); err != nil {
		return nil
	}

	var req APIKeyRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate required fields
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Name is required", nil, "")
	}

	// Parse expiration date if provided
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid expires_at format. Use RFC3339 format", nil, "")
		}
		expiresAt = &t
	}

	// Generate the API key
	fullKey, err := generateAPIKey()
	if err != nil {
		a.Log.Error("Failed to generate API key", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate API key", nil, "")
	}

	// Hash the key for storage
	hashedKey, err := bcrypt.GenerateFromPassword([]byte(fullKey), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("Failed to hash API key", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create API key", nil, "")
	}

	// Extract prefix (first 16 chars after "whm_")
	keyPrefix := fullKey[4:20]

	apiKey := models.APIKey{
		OrganizationID: orgID,
		UserID:         userID,
		Name:           req.Name,
		KeyPrefix:      keyPrefix,
		KeyHash:        string(hashedKey),
		ExpiresAt:      expiresAt,
		IsActive:       true,
	}

	if err := a.DB.Create(&apiKey).Error; err != nil {
		a.Log.Error("Failed to create API key", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create API key", nil, "")
	}

	// Return full key only on creation
	return r.SendEnvelope(APIKeyCreateResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		Key:       fullKey, // This is the only time the full key is returned
		KeyPrefix: apiKey.KeyPrefix,
		ExpiresAt: apiKey.ExpiresAt,
		CreatedAt: apiKey.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// DeleteAPIKey revokes an API key
func (a *App) DeleteAPIKey(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAPIKeys, models.ActionDelete); err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "API key")
	if err != nil {
		return nil
	}

	result := a.DB.Where("id = ? AND organization_id = ?", id, orgID).Delete(&models.APIKey{})
	if result.Error != nil {
		a.Log.Error("Failed to delete API key", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete API key", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "API key not found", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "API key deleted successfully"})
}
