package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// AccountRequest represents the request body for creating/updating an account
type AccountRequest struct {
	Name               string `json:"name" validate:"required"`
	AppID              string `json:"app_id"`
	PhoneID            string `json:"phone_id" validate:"required"`
	BusinessID         string `json:"business_id" validate:"required"`
	AccessToken        string `json:"access_token" validate:"required"`
	WebhookVerifyToken string `json:"webhook_verify_token"`
	APIVersion         string `json:"api_version"`
	IsDefaultIncoming  bool   `json:"is_default_incoming"`
	IsDefaultOutgoing  bool   `json:"is_default_outgoing"`
	AutoReadReceipt    bool   `json:"auto_read_receipt"`
}

// AccountResponse represents the response for an account (without sensitive data)
type AccountResponse struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	AppID              string    `json:"app_id"`
	PhoneID            string    `json:"phone_id"`
	BusinessID         string    `json:"business_id"`
	WebhookVerifyToken string    `json:"webhook_verify_token"`
	APIVersion         string    `json:"api_version"`
	IsDefaultIncoming  bool      `json:"is_default_incoming"`
	IsDefaultOutgoing  bool      `json:"is_default_outgoing"`
	AutoReadReceipt    bool      `json:"auto_read_receipt"`
	Status             string    `json:"status"`
	HasAccessToken     bool      `json:"has_access_token"`
	PhoneNumber        string    `json:"phone_number,omitempty"`
	DisplayName        string    `json:"display_name,omitempty"`
	CreatedAt          string    `json:"created_at"`
	UpdatedAt          string    `json:"updated_at"`
}

// ListAccounts returns all WhatsApp accounts for the organization
func (a *App) ListAccounts(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var accounts []models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ?", orgID).Order("created_at DESC").Find(&accounts).Error; err != nil {
		a.Log.Error("Failed to list accounts", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list accounts", nil, "")
	}

	// Convert to response format (hide sensitive data)
	response := make([]AccountResponse, len(accounts))
	for i, acc := range accounts {
		response[i] = accountToResponse(acc)
	}

	return r.SendEnvelope(map[string]interface{}{
		"accounts": response,
	})
}

// CreateAccount creates a new WhatsApp account
func (a *App) CreateAccount(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req AccountRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Validate required fields
	if req.Name == "" || req.PhoneID == "" || req.BusinessID == "" || req.AccessToken == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Name, phone_id, business_id, and access_token are required", nil, "")
	}

	// Generate webhook verify token if not provided
	webhookVerifyToken := req.WebhookVerifyToken
	if webhookVerifyToken == "" {
		webhookVerifyToken = generateVerifyToken()
	}

	// Set default API version
	apiVersion := req.APIVersion
	if apiVersion == "" {
		apiVersion = "v21.0"
	}

	account := models.WhatsAppAccount{
		OrganizationID:     orgID,
		Name:               req.Name,
		AppID:              req.AppID,
		PhoneID:            req.PhoneID,
		BusinessID:         req.BusinessID,
		AccessToken:        req.AccessToken, // TODO: encrypt before storing
		WebhookVerifyToken: webhookVerifyToken,
		APIVersion:         apiVersion,
		IsDefaultIncoming:  req.IsDefaultIncoming,
		IsDefaultOutgoing:  req.IsDefaultOutgoing,
		AutoReadReceipt:    req.AutoReadReceipt,
		Status:             "active",
	}

	// If this is set as default, unset other defaults
	if req.IsDefaultIncoming {
		a.DB.Model(&models.WhatsAppAccount{}).
			Where("organization_id = ? AND is_default_incoming = ?", orgID, true).
			Update("is_default_incoming", false)
	}
	if req.IsDefaultOutgoing {
		a.DB.Model(&models.WhatsAppAccount{}).
			Where("organization_id = ? AND is_default_outgoing = ?", orgID, true).
			Update("is_default_outgoing", false)
	}

	if err := a.DB.Create(&account).Error; err != nil {
		a.Log.Error("Failed to create account", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	return r.SendEnvelope(accountToResponse(account))
}

// GetAccount returns a single WhatsApp account
func (a *App) GetAccount(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid account ID", nil, "")
	}

	var account models.WhatsAppAccount
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Account not found", nil, "")
	}

	return r.SendEnvelope(accountToResponse(account))
}

// UpdateAccount updates a WhatsApp account
func (a *App) UpdateAccount(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr, ok := r.RequestCtx.UserValue("id").(string)
	if !ok || idStr == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Missing account ID", nil, "")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid account ID", nil, "")
	}

	var account models.WhatsAppAccount
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Account not found", nil, "")
	}

	var req AccountRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Update fields if provided
	if req.Name != "" {
		account.Name = req.Name
	}
	if req.AppID != "" {
		account.AppID = req.AppID
	}
	if req.PhoneID != "" {
		account.PhoneID = req.PhoneID
	}
	if req.BusinessID != "" {
		account.BusinessID = req.BusinessID
	}
	if req.AccessToken != "" {
		account.AccessToken = req.AccessToken // TODO: encrypt
	}
	if req.WebhookVerifyToken != "" {
		account.WebhookVerifyToken = req.WebhookVerifyToken
	}
	if req.APIVersion != "" {
		account.APIVersion = req.APIVersion
	}
	account.AutoReadReceipt = req.AutoReadReceipt

	// Handle default flags
	if req.IsDefaultIncoming && !account.IsDefaultIncoming {
		a.DB.Model(&models.WhatsAppAccount{}).
			Where("organization_id = ? AND is_default_incoming = ?", orgID, true).
			Update("is_default_incoming", false)
	}
	if req.IsDefaultOutgoing && !account.IsDefaultOutgoing {
		a.DB.Model(&models.WhatsAppAccount{}).
			Where("organization_id = ? AND is_default_outgoing = ?", orgID, true).
			Update("is_default_outgoing", false)
	}
	account.IsDefaultIncoming = req.IsDefaultIncoming
	account.IsDefaultOutgoing = req.IsDefaultOutgoing

	if err := a.DB.Save(&account).Error; err != nil {
		a.Log.Error("Failed to update account", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update account", nil, "")
	}

	// Invalidate cache
	a.InvalidateWhatsAppAccountCache(account.PhoneID)

	return r.SendEnvelope(accountToResponse(account))
}

// DeleteAccount deletes a WhatsApp account
func (a *App) DeleteAccount(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid account ID", nil, "")
	}

	// Get account first for cache invalidation
	var account models.WhatsAppAccount
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Account not found", nil, "")
	}

	if err := a.DB.Delete(&account).Error; err != nil {
		a.Log.Error("Failed to delete account", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete account", nil, "")
	}

	// Invalidate cache
	a.InvalidateWhatsAppAccountCache(account.PhoneID)

	return r.SendEnvelope(map[string]string{"message": "Account deleted successfully"})
}

// TestAccountConnection tests the WhatsApp API connection
func (a *App) TestAccountConnection(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid account ID", nil, "")
	}

	var account models.WhatsAppAccount
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Account not found", nil, "")
	}

	// Test the connection by fetching phone number details from Meta API
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s?fields=display_phone_number,verified_name,quality_rating,messaging_limit_tier",
		account.APIVersion, account.PhoneID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return r.SendEnvelope(map[string]interface{}{
			"success": false,
			"error":   "Failed to connect to WhatsApp API: " + err.Error(),
		})
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errorResp map[string]interface{}
		_ = json.Unmarshal(body, &errorResp)
		return r.SendEnvelope(map[string]interface{}{
			"success": false,
			"error":   "API error",
			"details": errorResp,
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)

	return r.SendEnvelope(map[string]interface{}{
		"success":              true,
		"display_phone_number": result["display_phone_number"],
		"verified_name":        result["verified_name"],
		"quality_rating":       result["quality_rating"],
		"messaging_limit_tier": result["messaging_limit_tier"],
	})
}

// Helper functions

func accountToResponse(acc models.WhatsAppAccount) AccountResponse {
	return AccountResponse{
		ID:                 acc.ID,
		Name:               acc.Name,
		AppID:              acc.AppID,
		PhoneID:            acc.PhoneID,
		BusinessID:         acc.BusinessID,
		WebhookVerifyToken: acc.WebhookVerifyToken,
		APIVersion:         acc.APIVersion,
		IsDefaultIncoming:  acc.IsDefaultIncoming,
		IsDefaultOutgoing:  acc.IsDefaultOutgoing,
		AutoReadReceipt:    acc.AutoReadReceipt,
		Status:             acc.Status,
		HasAccessToken:     acc.AccessToken != "",
		CreatedAt:          acc.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          acc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func generateVerifyToken() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func getOrganizationID(r *fastglue.Request) (uuid.UUID, error) {
	// Try to get as uuid.UUID first (set by auth middleware)
	if orgID, ok := r.RequestCtx.UserValue("organization_id").(uuid.UUID); ok {
		return orgID, nil
	}
	// Fallback to string parsing
	if orgIDStr, ok := r.RequestCtx.UserValue("organization_id").(string); ok {
		return uuid.Parse(orgIDStr)
	}
	return uuid.Nil, fmt.Errorf("organization_id not found in context")
}
