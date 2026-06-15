package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/audit"
	"github.com/shridarpatil/whatomate/internal/crypto"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// AccountRequest represents the request body for creating/updating an account
type AccountRequest struct {
	Name                   string `json:"name" validate:"required"`
	AppID                  string `json:"app_id"`
	PhoneID                string `json:"phone_id" validate:"required"`
	BusinessID             string `json:"business_id" validate:"required"`
	AccessToken            string `json:"access_token" validate:"required"`
	AppSecret              string `json:"app_secret"` // Meta App Secret for webhook signature verification
	WebhookVerifyToken     string `json:"webhook_verify_token"`
	APIVersion             string `json:"api_version"`
	IsDefaultIncoming      bool   `json:"is_default_incoming"`
	IsDefaultOutgoing      bool   `json:"is_default_outgoing"`
	AutoReadReceipt        bool   `json:"auto_read_receipt"`
	BusinessCallingEnabled bool   `json:"business_calling_enabled"`
}

// AccountResponse represents the response for an account (without sensitive data)
type AccountResponse struct {
	ID                     uuid.UUID  `json:"id"`
	Name                   string     `json:"name"`
	AppID                  string     `json:"app_id"`
	PhoneID                string     `json:"phone_id"`
	BusinessID             string     `json:"business_id"`
	WebhookVerifyToken     string     `json:"webhook_verify_token"`
	APIVersion             string     `json:"api_version"`
	IsDefaultIncoming      bool       `json:"is_default_incoming"`
	IsDefaultOutgoing      bool       `json:"is_default_outgoing"`
	AutoReadReceipt        bool       `json:"auto_read_receipt"`
	BusinessCallingEnabled bool       `json:"business_calling_enabled"`
	Status                 string     `json:"status"`
	HasAccessToken         bool       `json:"has_access_token"`
	HasAppSecret           bool       `json:"has_app_secret"`
	PhoneNumber            string     `json:"phone_number,omitempty"`
	DisplayName            string     `json:"display_name,omitempty"`
	CreatedByID            *uuid.UUID `json:"created_by_id,omitempty"`
	CreatedByName          string     `json:"created_by_name,omitempty"`
	UpdatedByID            *uuid.UUID `json:"updated_by_id,omitempty"`
	UpdatedByName          string     `json:"updated_by_name,omitempty"`
	CreatedAt              string     `json:"created_at"`
	UpdatedAt              string     `json:"updated_at"`
}

// ListAccounts returns all WhatsApp accounts for the organization
func (a *App) ListAccounts(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAccounts, models.ActionRead); err != nil {
		return nil
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

	return r.SendEnvelope(map[string]any{
		"accounts": response,
	})
}

// CreateAccount creates a new WhatsApp account
func (a *App) CreateAccount(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAccounts, models.ActionWrite); err != nil {
		return nil
	}

	var req AccountRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
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
		apiVersion = a.defaultAPIVersion()
	}

	account := models.WhatsAppAccount{
		OrganizationID:         orgID,
		Name:                   req.Name,
		AppID:                  req.AppID,
		PhoneID:                req.PhoneID,
		BusinessID:             req.BusinessID,
		AccessToken:            req.AccessToken,
		AppSecret:              req.AppSecret,
		WebhookVerifyToken:     webhookVerifyToken,
		APIVersion:             apiVersion,
		IsDefaultIncoming:      req.IsDefaultIncoming,
		IsDefaultOutgoing:      req.IsDefaultOutgoing,
		AutoReadReceipt:        req.AutoReadReceipt,
		BusinessCallingEnabled: req.BusinessCallingEnabled,
		Status:                 "active",
		CreatedByID:            &userID,
		UpdatedByID:            &userID,
	}

	if err := a.encryptAccountSecrets(&account); err != nil {
		a.Log.Error("Failed to encrypt account secrets", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
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

	a.DB.Preload("CreatedBy").Preload("UpdatedBy").First(&account, "id = ?", account.ID)
	audit.LogAudit(a.DB, orgID, userID, audit.GetUserName(a.DB, userID),
		"account", account.ID, models.AuditActionCreated, nil, &account)

	return r.SendEnvelope(accountToResponse(account))
}

// GetAccount returns a single WhatsApp account
func (a *App) GetAccount(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAccounts, models.ActionRead); err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "account")
	if err != nil {
		return nil
	}

	account, err := findByIDAndOrg[models.WhatsAppAccount](
		a.DB.Preload("CreatedBy").Preload("UpdatedBy"), r, id, orgID, "Account")
	if err != nil {
		return nil
	}

	return r.SendEnvelope(accountToResponse(*account))
}

// UpdateAccount updates a WhatsApp account
func (a *App) UpdateAccount(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAccounts, models.ActionWrite); err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "account")
	if err != nil {
		return nil
	}

	account, err := a.resolveWhatsAppAccountByID(r, id, orgID)
	if err != nil {
		return nil
	}

	oldAccount := *account // value copy for audit

	var req AccountRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
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
	tokenChanged := false
	secretChanged := false
	if req.AccessToken != "" {
		enc, err := crypto.Encrypt(req.AccessToken, a.Config.App.EncryptionKey)
		if err != nil {
			a.Log.Error("Failed to encrypt access token", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update account", nil, "")
		}
		account.AccessToken = enc
		tokenChanged = true
	}
	if req.AppSecret != "" {
		enc, err := crypto.Encrypt(req.AppSecret, a.Config.App.EncryptionKey)
		if err != nil {
			a.Log.Error("Failed to encrypt app secret", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update account", nil, "")
		}
		account.AppSecret = enc
		secretChanged = true
	}
	if req.WebhookVerifyToken != "" {
		account.WebhookVerifyToken = req.WebhookVerifyToken
	}
	if req.APIVersion != "" {
		account.APIVersion = req.APIVersion
	}
	account.AutoReadReceipt = req.AutoReadReceipt
	account.BusinessCallingEnabled = req.BusinessCallingEnabled

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
	account.UpdatedByID = &userID

	if err := a.DB.Save(account).Error; err != nil {
		a.Log.Error("Failed to update account", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update account", nil, "")
	}

	// Invalidate cache
	a.InvalidateWhatsAppAccountCache(account.PhoneID)

	a.DB.Preload("CreatedBy").Preload("UpdatedBy").First(account, "id = ?", account.ID)

	var sensitiveChanges []map[string]any
	if tokenChanged {
		sensitiveChanges = append(sensitiveChanges, map[string]any{
			"field": "access_token", "old_value": "********", "new_value": "********",
		})
	}
	if secretChanged {
		sensitiveChanges = append(sensitiveChanges, map[string]any{
			"field": "app_secret", "old_value": "********", "new_value": "********",
		})
	}
	audit.LogAudit(a.DB, orgID, userID, audit.GetUserName(a.DB, userID),
		"account", account.ID, models.AuditActionUpdated, &oldAccount, account, sensitiveChanges...)

	return r.SendEnvelope(accountToResponse(*account))
}

// DeleteAccount deletes a WhatsApp account
func (a *App) DeleteAccount(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAccounts, models.ActionDelete); err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "account")
	if err != nil {
		return nil
	}

	// Get account first for cache invalidation and audit
	account, err := findByIDAndOrg[models.WhatsAppAccount](a.DB, r, id, orgID, "Account")
	if err != nil {
		return nil
	}

	if err := a.DB.Delete(account).Error; err != nil {
		a.Log.Error("Failed to delete account", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete account", nil, "")
	}

	// Invalidate cache
	a.InvalidateWhatsAppAccountCache(account.PhoneID)

	audit.LogAudit(a.DB, orgID, userID, audit.GetUserName(a.DB, userID),
		"account", id, models.AuditActionDeleted, account, nil)

	return r.SendEnvelope(map[string]string{"message": "Account deleted successfully"})
}

// TestAccountConnection tests the WhatsApp API connection
// This validates both PhoneID and BusinessID to ensure all credentials are correct
func (a *App) TestAccountConnection(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "account")
	if err != nil {
		return nil
	}

	account, err := a.resolveWhatsAppAccountByID(r, id, orgID)
	if err != nil {
		return nil
	}

	// Use the comprehensive validation function
	if err := a.validateAccountCredentials(account.PhoneID, account.BusinessID, account.AccessToken, account.APIVersion); err != nil {
		a.Log.Error("Account test failed", "error", err, "account", account.Name)
		return r.SendEnvelope(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Account credential validation failed: %s", err.Error()),
		})
	}

	// Fetch additional details for display
	phoneURL := fmt.Sprintf("%s/%s/%s?fields=display_phone_number,verified_name,code_verification_status,account_mode,quality_rating,messaging_limit_tier,whatsapp_business_manager_messaging_limit",
		a.Config.WhatsApp.BaseURL, account.APIVersion, account.PhoneID)

	result, status, err := a.fetchMetaJSON(phoneURL, account.AccessToken)
	if err != nil {
		a.Log.Error("Failed to connect to WhatsApp API", "error", err)
		return r.SendEnvelope(map[string]any{
			"success": false,
			"error":   "Failed to connect to WhatsApp API",
		})
	}
	if status != http.StatusOK {
		return r.SendEnvelope(map[string]any{
			"success": false,
			"error":   "API error",
			"details": result,
		})
	}

	// Check if this is a test/sandbox number
	accountMode, _ := result["account_mode"].(string)
	isTestNumber := accountMode == "SANDBOX"

	// Resolve messaging limit tier, falling back to newer portfolio-based field if deprecated field is missing/null
	messagingLimitTier := result["messaging_limit_tier"]
	if messagingLimitTier == nil || messagingLimitTier == "" {
		messagingLimitTier = result["whatsapp_business_manager_messaging_limit"]
	}

	// If still empty/null, query the WABA ID (BusinessID) as a fallback
	if (messagingLimitTier == nil || messagingLimitTier == "") && account.BusinessID != "" {
		wabaURL := fmt.Sprintf("%s/%s/%s?fields=whatsapp_business_manager_messaging_limit",
			a.Config.WhatsApp.BaseURL, account.APIVersion, account.BusinessID)
		wabaResult, wabaStatus, wabaErr := a.fetchMetaJSON(wabaURL, account.AccessToken)
		switch {
		case wabaErr != nil:
			a.Log.Warn("WABA fallback request failed", "waba_id", account.BusinessID, "error", wabaErr)
		case wabaStatus != http.StatusOK:
			a.Log.Warn("WABA fallback returned non-200", "waba_id", account.BusinessID, "status", wabaStatus)
		default:
			if val, ok := wabaResult["whatsapp_business_manager_messaging_limit"]; ok && val != nil && val != "" {
				messagingLimitTier = val
				a.Log.Info("Resolved messaging limit tier from WABA as fallback", "waba_id", account.BusinessID, "limit", val)
			}
		}
	}

	// Prepare response
	response := map[string]any{
		"success":                  true,
		"display_phone_number":     result["display_phone_number"],
		"verified_name":            result["verified_name"],
		"quality_rating":           result["quality_rating"],
		"messaging_limit_tier":     messagingLimitTier,
		"code_verification_status": result["code_verification_status"],
		"account_mode":             result["account_mode"],
		"is_test_number":           isTestNumber,
	}

	// Add warning for test/sandbox numbers or expired verification
	if isTestNumber {
		response["warning"] = "This is a test/sandbox number. Not suitable for production use."
	} else if verificationStatus, ok := result["code_verification_status"].(string); ok && verificationStatus == "EXPIRED" {
		response["warning"] = "Phone verification has expired. Consider re-verifying at: https://business.facebook.com/wa/manage/phone-numbers/"
	}

	return r.SendEnvelope(response)
}

// fetchMetaJSON performs a Bearer-authenticated GET against the Meta Graph API
// and decodes the JSON body into a generic map. The decoded body is returned
// regardless of HTTP status, so callers can surface error envelopes from Meta.
// Returns (nil, 0, err) only when the request itself fails (network/decode).
func (a *App) fetchMetaJSON(url, accessToken string) (map[string]any, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	var out map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, resp.StatusCode, err
		}
	}
	return out, resp.StatusCode, nil
}

// Helper functions

func accountToResponse(acc models.WhatsAppAccount) AccountResponse {
	resp := AccountResponse{
		ID:                     acc.ID,
		Name:                   acc.Name,
		AppID:                  acc.AppID,
		PhoneID:                acc.PhoneID,
		BusinessID:             acc.BusinessID,
		WebhookVerifyToken:     acc.WebhookVerifyToken,
		APIVersion:             acc.APIVersion,
		IsDefaultIncoming:      acc.IsDefaultIncoming,
		IsDefaultOutgoing:      acc.IsDefaultOutgoing,
		AutoReadReceipt:        acc.AutoReadReceipt,
		BusinessCallingEnabled: acc.BusinessCallingEnabled,
		Status:                 acc.Status,
		HasAccessToken:         acc.AccessToken != "",
		HasAppSecret:           acc.AppSecret != "",
		CreatedByID:            acc.CreatedByID,
		UpdatedByID:            acc.UpdatedByID,
		CreatedAt:              acc.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:              acc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if acc.CreatedBy != nil {
		resp.CreatedByName = acc.CreatedBy.FullName
	}
	if acc.UpdatedBy != nil {
		resp.UpdatedByName = acc.UpdatedBy.FullName
	}
	return resp
}

func generateVerifyToken() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// validateAccountCredentials validates WhatsApp account credentials with Meta API
func (a *App) validateAccountCredentials(phoneID, businessID, accessToken, apiVersion string) error {
	ctx := context.Background()
	_, err := a.WhatsApp.ValidateCredentials(ctx, phoneID, businessID, accessToken, apiVersion)
	if err != nil {
		return err
	}
	a.Log.Info("Account credentials validated successfully", "phone_id", phoneID, "business_id", businessID)
	return nil
}

// SubscribeApp subscribes the app to webhooks for the WhatsApp Business Account.
// This is required after phone number registration to receive incoming messages from Meta.
func (a *App) SubscribeApp(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "account")
	if err != nil {
		return nil
	}

	account, err := a.resolveWhatsAppAccountByID(r, id, orgID)
	if err != nil {
		return nil
	}

	// Subscribe the app to webhooks
	ctx := context.Background()
	if err := a.WhatsApp.SubscribeApp(ctx, a.toWhatsAppAccount(account)); err != nil {
		a.Log.Error("Failed to subscribe app to webhooks", "error", err, "account", account.Name)
		return r.SendEnvelope(map[string]any{
			"success": false,
			"error":   "Failed to subscribe app to webhooks. Check your credentials.",
		})
	}

	a.Log.Info("App subscribed to webhooks successfully", "account", account.Name, "business_id", account.BusinessID)
	return r.SendEnvelope(map[string]any{
		"success": true,
		"message": "App subscribed to webhooks successfully. You should now receive incoming messages.",
	})
}

// resolveMetaAppCreds resolves Meta app ID, App Secret, and Config ID for an organization,
// preferring organization-specific settings and falling back to global config defaults.
func (a *App) resolveMetaAppCreds(orgID uuid.UUID) (string, string, string, error) {
	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return "", "", "", err
	}

	appID := a.Config.WhatsApp.AppID
	appSecret := a.Config.WhatsApp.AppSecret
	configID := a.Config.WhatsApp.ConfigID

	if org.Settings != nil {
		if v, ok := org.Settings["meta_app_id"].(string); ok && v != "" {
			appID = v
		}
		if v, ok := org.Settings["meta_config_id"].(string); ok && v != "" {
			configID = v
		}
		if v, ok := org.Settings["meta_app_secret_encrypted"].(string); ok && v != "" {
			decrypted, err := crypto.Decrypt(v, a.Config.App.EncryptionKey)
			if err == nil && decrypted != "" {
				appSecret = decrypted
			} else if err != nil {
				a.Log.Error("Failed to decrypt meta app secret from organization settings", "error", err)
			}
		}
	}

	return appID, appSecret, configID, nil
}

// ExchangeToken exchanges the temporary code for a permanent access token and creates the account
func (a *App) ExchangeToken(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAccounts, models.ActionWrite); err != nil {
		return nil
	}

	var req struct {
		Code               string `json:"code" validate:"required"`
		PhoneID            string `json:"phone_id"` // Optional: Discovered via token if missing
		WABAID             string `json:"waba_id"`  // Optional: Discovered via token if missing
		Name               string `json:"name"`
		WebhookVerifyToken string `json:"webhook_verify_token"`
	}
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	a.Log.Info("Received embedded signup exchange token request",
		"phone_id", req.PhoneID,
		"waba_id", req.WABAID,
		"organization_id", orgID)

	if req.Code == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Code is required", nil, "")
	}

	// 1. Resolve Meta credentials for this org
	appID, appSecret, _, err := a.resolveMetaAppCreds(orgID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to resolve credentials", nil, "")
	}

	// 2. Exchange code for user access token using WhatsApp service
	ctx := context.Background()
	a.Log.Info("Exchanging code for access token")

	accessToken, err := a.WhatsApp.ExchangeCodeForToken(ctx, req.Code,
		appID, appSecret, a.Config.WhatsApp.APIVersion)
	if err != nil {
		a.Log.Error("Failed to exchange token", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	// DISCOVERY: If IDs are missing, try to find them using the token
	phoneID, wabaID, name, err := a.discoverWABAAndPhone(ctx, orgID, accessToken, req.PhoneID, req.WABAID, req.Name)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	// 3. We can now create/update the account
	account, phoneInfo, existingAccount, oldAccount, err := a.createOrUpdateAccount(ctx, orgID, phoneID, wabaID, name, req.WebhookVerifyToken, accessToken, appSecret)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, err.Error(), nil, "")
	}

	// 4. Attempt Auto-Registration
	var priorStatus string
	if oldAccount != nil {
		priorStatus = oldAccount.Status
	}
	regErr := a.attemptAutoRegistration(ctx, account, phoneInfo, accessToken, priorStatus)

	// 5. Subscribe app to WABA webhooks
	if err := a.WhatsApp.SubscribeApp(ctx, a.toWhatsAppAccount(account)); err != nil {
		a.Log.Error("Failed to subscribe app to WABA", "error", err)
	}

	// 6. Encrypt credentials at rest
	plaintextPin := account.Pin
	if err := a.encryptAccountSecrets(account); err != nil {
		a.Log.Error("Failed to encrypt account secrets", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, err.Error(), nil, "")
	}

	if err := a.DB.Save(account).Error; err != nil {
		a.Log.Error("Failed to save account", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save account", nil, "")
	}

	// Invalidate cache
	a.InvalidateWhatsAppAccountCache(account.PhoneID)

	a.Log.Info("WhatsApp account connected via embedded signup successfully",
		"account_id", account.ID,
		"phone_id", account.PhoneID,
		"status", account.Status)

	// Audit Logging
	a.DB.Preload("CreatedBy").Preload("UpdatedBy").First(account, "id = ?", account.ID)
	auditAction := models.AuditActionCreated
	var auditOld any = nil
	if existingAccount {
		auditAction = models.AuditActionUpdated
		auditOld = oldAccount
	}
	audit.LogAudit(a.DB, orgID, userID, audit.GetUserName(a.DB, userID),
		"account", account.ID, auditAction, auditOld, account)

	// Construction of response map (reusing accountToResponse)
	out := map[string]any{
		"account": accountToResponse(*account),
	}
	if account.Status == "active" && plaintextPin != "" {
		out["pin"] = plaintextPin
	}
	if regErr != nil {
		out["warning"] = "Registration failed: " + regErr.Error()
	}

	return r.SendEnvelope(out)
}

func (a *App) discoverWABAAndPhone(ctx context.Context, orgID uuid.UUID, accessToken, phoneID, wabaID, name string) (string, string, string, error) {
	if phoneID != "" && wabaID != "" {
		return phoneID, wabaID, name, nil
	}

	a.Log.Info("Missing PhoneID/WABAID, attempting discovery via debug_token")

	// 1. Resolve Meta credentials for this org
	appID, appSecret, _, err := a.resolveMetaAppCreds(orgID)
	if err != nil {
		return "", "", "", err
	}

	appAccessToken := fmt.Sprintf("%s|%s", appID, appSecret)

	debugInfo, err := a.WhatsApp.GetTokenDebugInfo(ctx, accessToken, appAccessToken)
	if err != nil {
		a.Log.Error("Failed to debug token", "error", err)
		return "", "", "", fmt.Errorf("failed to validate token details: %w", err)
	}

	// 2. Find WABA ID from Granular Scopes
	var discoveredWABAID string
	for _, scope := range debugInfo.GranularScopes {
		if scope.Scope == "whatsapp_business_management" {
			if len(scope.TargetIds) > 0 {
				discoveredWABAID = scope.TargetIds[0]
				break
			}
		}
	}

	if discoveredWABAID == "" {
		a.Log.Warn("No WABA ID found in granular scopes, falling back to /me/accounts strategy")
		sharedInfo, err := a.WhatsApp.GetSharedWABA(ctx, accessToken)
		if err == nil && len(sharedInfo.Data) > 0 {
			discoveredWABAID = sharedInfo.Data[0].ID
		}
	}

	if discoveredWABAID == "" {
		return "", "", "", fmt.Errorf("could not discover WhatsApp Business Account ID from token")
	}

	wabaID = discoveredWABAID
	a.Log.Info("Discovered WABA ID", "waba_id", wabaID)

	if phoneID == "" {
		phonesResp, err := a.WhatsApp.GetWABAPhoneNumbers(ctx, wabaID, accessToken)
		if err != nil {
			a.Log.Error("Failed to fetch phone numbers from Meta", "error", err)
			return "", "", "", fmt.Errorf("failed to fetch phone numbers from WABA: %w", err)
		}

		if len(phonesResp.Data) == 0 {
			return "", "", "", fmt.Errorf("no phone numbers found in this WhatsApp Business Account")
		}

		if len(phonesResp.Data) > 1 {
			a.Log.Warn("Multiple phone numbers discovered in WABA; picking the first one", "count", len(phonesResp.Data))
		}

		// User selects only ONE account in the flow, so we take the first one found.
		phone := phonesResp.Data[0]
		phoneID = phone.ID
		name = fmt.Sprintf("%s (%s)", phone.VerifiedName, phone.DisplayPhoneNumber)
		a.Log.Info("Discovered Phone ID", "phone_id", phoneID)
	}

	return phoneID, wabaID, name, nil
}

func (a *App) createOrUpdateAccount(ctx context.Context, orgID uuid.UUID, phoneID, wabaID, name, webhookVerifyToken, accessToken, appSecret string) (*models.WhatsAppAccount, *whatsapp.PhoneNumberInfo, bool, *models.WhatsAppAccount, error) {
	var account models.WhatsAppAccount
	var existingAccount bool
	var oldAccount *models.WhatsAppAccount

	// Use Unscoped to find even soft-deleted accounts to avoid unique constraint violations
	if err := a.DB.Unscoped().Where("phone_id = ? AND organization_id = ?", phoneID, orgID).First(&account).Error; err == nil {
		existingAccount = true
		temp := account
		oldAccount = &temp
	}

	// Fetch phone info from Meta using WhatsApp service unconditionally
	phoneInfo, err := a.WhatsApp.GetPhoneNumberInfo(ctx, phoneID, accessToken, a.Config.WhatsApp.APIVersion)
	if err != nil {
		a.Log.Warn("Failed to fetch phone info from Meta", "error", err)
	}

	if name == "" {
		if err == nil && phoneInfo != nil && phoneInfo.VerifiedName != "" {
			suffixPIN, err := generateNumericPIN(4)
			if err != nil {
				return nil, nil, false, nil, fmt.Errorf("failed to generate security identifier: %w", err)
			}
			name = fmt.Sprintf("%s %s", phoneInfo.VerifiedName, suffixPIN)
		} else {
			// Safe substring handling
			suffix := phoneID
			if len(phoneID) > 4 {
				suffix = phoneID[len(phoneID)-4:]
			}
			name = "WhatsApp Account " + suffix
		}
	}

	// Generate verify token if needed
	if webhookVerifyToken == "" {
		if existingAccount {
			webhookVerifyToken = account.WebhookVerifyToken
		} else {
			webhookVerifyToken = generateVerifyToken()
		}
	}

	var isSMB bool
	if phoneInfo != nil {
		if phoneInfo.IsOnBizApp || phoneInfo.PlatformType == "SMB" || phoneInfo.PlatformType == "SMB_CLOUD_API" {
			isSMB = true
		}
	}

	account.OrganizationID = orgID
	account.Name = name
	account.PhoneID = phoneID
	account.BusinessID = wabaID
	account.AccessToken = accessToken
	account.AppSecret = appSecret
	account.WebhookVerifyToken = webhookVerifyToken
	if !existingAccount {
		account.Status = "pending_registration"
	}
	account.IsSMB = isSMB

	// Only fill account.AppID / APIVersion if empty
	if account.AppID == "" {
		appID, _, _, _ := a.resolveMetaAppCreds(orgID)
		account.AppID = appID
	}
	if account.APIVersion == "" {
		account.APIVersion = a.defaultAPIVersion()
	}

	if !existingAccount {
		account.IsDefaultIncoming = false
		account.IsDefaultOutgoing = false
		account.AutoReadReceipt = false
	}

	return &account, phoneInfo, existingAccount, oldAccount, nil
}

func (a *App) attemptAutoRegistration(ctx context.Context, account *models.WhatsAppAccount, phoneInfo *whatsapp.PhoneNumberInfo, accessToken, priorStatus string) error {
	if account.IsSMB {
		account.Status = "active"
		account.Pin = ""
		a.Log.Info("SMB account detected via Meta API, skipped registration, setting to active", "phone_id", account.PhoneID)
		return nil
	}

	generatedPin, err := generateNumericPIN(6)
	if err != nil {
		return fmt.Errorf("failed to generate secure random PIN: %w", err)
	}

	a.Log.Info("Attempting phone number auto-registration", "phone_id", account.PhoneID)
	regErr := a.WhatsApp.RegisterPhoneNumber(ctx, account.PhoneID, generatedPin, accessToken, account.APIVersion)

	if regErr == nil {
		account.Status = "active"
		account.Pin = generatedPin
		a.Log.Info("Phone number auto-registration successful", "phone_id", account.PhoneID)
	} else {
		a.Log.Warn("Phone number auto-registration failed",
			"error", regErr,
			"phone_id", account.PhoneID)
		if priorStatus != "" {
			account.Status = priorStatus
		} else {
			account.Status = "pending_registration"
		}
	}

	return regErr
}

// RegisterPhoneNumber registers the phone number with Two-Step Verification
func (a *App) RegisterPhoneNumber(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAccounts, models.ActionWrite); err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "account")
	if err != nil {
		return nil
	}

	var req struct {
		Pin string `json:"pin"` // Optional custom PIN
	}
	_ = r.Decode(&req, "json")

	account, err := a.resolveWhatsAppAccountByID(r, id, orgID)
	if err != nil {
		return nil
	}

	oldAccount := *account

	// If PIN is not provided, generate a random one
	pin := req.Pin
	if pin == "" {
		var err error
		pin, err = generateNumericPIN(6)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate secure random PIN", nil, "")
		}
	}

	ctx := context.Background()

	// Check if this is an SMB phone — SMB numbers are already registered
	// via the Business App and don't support the two-step registration API.
	if account.IsSMB {
		a.Log.Info("Manual registration: SMB account detected, skipping registration", "phone_id", account.PhoneID)
		pin = ""
	} else {
		// Call Meta Register endpoint using WhatsApp service
		if err := a.WhatsApp.RegisterPhoneNumber(ctx, account.PhoneID, pin, account.AccessToken, account.APIVersion); err != nil {
			a.Log.Error("Manual registration failed", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
	}

	// Success
	account.Status = "active"
	account.Pin = pin

	// Encrypt secrets before saving
	if err := a.encryptAccountSecrets(account); err != nil {
		a.Log.Error("Failed to encrypt account secrets", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, err.Error(), nil, "")
	}

	if err := a.DB.Save(account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update account status", nil, "")
	}

	// Invalidate cache
	a.InvalidateWhatsAppAccountCache(account.PhoneID)

	// Log audit!
	a.DB.Preload("CreatedBy").Preload("UpdatedBy").First(account, "id = ?", account.ID)
	audit.LogAudit(a.DB, orgID, userID, audit.GetUserName(a.DB, userID),
		"account", account.ID, models.AuditActionUpdated, &oldAccount, account)

	return r.SendEnvelope(map[string]interface{}{
		"success": true,
		"message": "Phone number registered successfully",
		"pin":     pin,
	})
}

func generateNumericPIN(length int) (string, error) {
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b[i] = byte(num.Int64()) + '0'
	}
	return string(b), nil
}

func (a *App) defaultAPIVersion() string {
	if a.Config.WhatsApp.APIVersion != "" {
		return a.Config.WhatsApp.APIVersion
	}
	return "v21.0"
}

func (a *App) encryptAccountSecrets(account *models.WhatsAppAccount) error {
	encKey := a.Config.App.EncryptionKey
	var err error
	if account.AccessToken != "" && !crypto.IsEncrypted(account.AccessToken) {
		account.AccessToken, err = crypto.Encrypt(account.AccessToken, encKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt access token: %w", err)
		}
	}
	if account.AppSecret != "" && !crypto.IsEncrypted(account.AppSecret) {
		account.AppSecret, err = crypto.Encrypt(account.AppSecret, encKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt app secret: %w", err)
		}
	}
	if account.Pin != "" && !crypto.IsEncrypted(account.Pin) {
		account.Pin, err = crypto.Encrypt(account.Pin, encKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt PIN: %w", err)
		}
	}
	return nil
}
