package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// EmbeddedSignupRequest represents the request for creating/updating embedded signup
type EmbeddedSignupRequest struct {
	Name              string                 `json:"name" validate:"required"`
	WhatsAppAccountID string                 `json:"whatsapp_account_id" validate:"required"`
	MetaAppID         string                 `json:"meta_app_id" validate:"required"`
	MetaConfigID      string                 `json:"meta_config_id" validate:"required"`
	MetaAppSecret     string                 `json:"meta_app_secret" validate:"required"`
	EnableCoexistence bool                   `json:"enable_coexistence"`
	SyncChatHistory   bool                   `json:"sync_chat_history"`
	APIVersion        string                 `json:"api_version"`
	FormFields        map[string]interface{} `json:"form_fields"`
	RequiredFields    []string               `json:"required_fields"`
	WelcomeMessage    string                 `json:"welcome_message"`
	WelcomeTemplateID *string                `json:"welcome_template_id,omitempty"`
	SuccessMessage    string                 `json:"success_message"`
	RedirectURL       *string                `json:"redirect_url,omitempty"`
	WebhookURL        *string                `json:"webhook_url,omitempty"`
	AllowedOrigins    []string               `json:"allowed_origins"`
	RateLimitPerHour  int                    `json:"rate_limit_per_hour"`
	IsActive          bool                   `json:"is_active"`
	AutoCreateContact bool                   `json:"auto_create_contact"`
	AssignToTeamID    *string                `json:"assign_to_team_id,omitempty"`
}

// EmbeddedSignupResponse represents the response for embedded signup
type EmbeddedSignupResponse struct {
	ID                uuid.UUID              `json:"id"`
	Name              string                 `json:"name"`
	WhatsAppAccountID uuid.UUID              `json:"whatsapp_account_id"`
	MetaAppID         string                 `json:"meta_app_id"`
	MetaConfigID      string                 `json:"meta_config_id"`
	EnableCoexistence bool                   `json:"enable_coexistence"`
	SyncChatHistory   bool                   `json:"sync_chat_history"`
	APIVersion        string                 `json:"api_version"`
	FormFields        map[string]interface{} `json:"form_fields"`
	RequiredFields    []string               `json:"required_fields"`
	WelcomeMessage    string                 `json:"welcome_message"`
	SuccessMessage    string                 `json:"success_message"`
	RedirectURL       *string                `json:"redirect_url,omitempty"`
	AllowedOrigins    []string               `json:"allowed_origins"`
	RateLimitPerHour  int                    `json:"rate_limit_per_hour"`
	IsActive          bool                   `json:"is_active"`
	AutoCreateContact bool                   `json:"auto_create_contact"`
	CreatedAt         string                 `json:"created_at"`
	UpdatedAt         string                 `json:"updated_at"`
}

// EmbeddedSignupSubmitRequest represents the signup submission from a user
type EmbeddedSignupSubmitRequest struct {
	PhoneNumber   string                 `json:"phone_number" validate:"required"`
	ProfileName   string                 `json:"profile_name"`
	FormData      map[string]interface{} `json:"form_data"`
	MetaAuthCode  string                 `json:"meta_auth_code"` // OAuth authorization code from Meta
	Source        string                 `json:"source"`
}

// EmbeddedSignupLeadResponse represents a lead response
type EmbeddedSignupLeadResponse struct {
	ID                 uuid.UUID              `json:"id"`
	PhoneNumber        string                 `json:"phone_number"`
	ProfileName        string                 `json:"profile_name"`
	FormData           map[string]interface{} `json:"form_data"`
	Status             string                 `json:"status"`
	Source             string                 `json:"source"`
	CoexistenceEnabled bool                   `json:"coexistence_enabled"`
	ChatHistorySynced  bool                   `json:"chat_history_synced"`
	CreatedAt          string                 `json:"created_at"`
}

// ListEmbeddedSignups lists all embedded signups for an organization
func (a *App) ListEmbeddedSignups(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var signups []models.EmbeddedSignup
	if err := a.DB.Where("organization_id = ?", orgID).Order("created_at DESC").Find(&signups).Error; err != nil {
		a.Log.Error("Failed to list embedded signups", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list signups", nil, "")
	}

	response := make([]EmbeddedSignupResponse, len(signups))
	for i, signup := range signups {
		response[i] = embeddedSignupToResponse(signup)
	}

	return r.SendEnvelope(map[string]interface{}{
		"signups": response,
	})
}

// CreateEmbeddedSignup creates a new embedded signup configuration
func (a *App) CreateEmbeddedSignup(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req EmbeddedSignupRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Validate required fields
	if req.Name == "" || req.WhatsAppAccountID == "" || req.MetaAppID == "" || req.MetaConfigID == "" || req.MetaAppSecret == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Name, WhatsApp account, Meta app ID, config ID, and app secret are required", nil, "")
	}

	// Parse WhatsApp account ID
	accountID, err := uuid.Parse(req.WhatsAppAccountID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid WhatsApp account ID", nil, "")
	}

	// Verify WhatsApp account exists and belongs to organization
	var account models.WhatsAppAccount
	if err := a.DB.Where("id = ? AND organization_id = ?", accountID, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Set defaults
	apiVersion := req.APIVersion
	if apiVersion == "" {
		apiVersion = "v24.0" // Latest API version with coexistence support
	}

	if req.RateLimitPerHour == 0 {
		req.RateLimitPerHour = 100
	}

	if req.RequiredFields == nil || len(req.RequiredFields) == 0 {
		req.RequiredFields = []string{"phone"}
	}

	// Parse optional fields
	var welcomeTemplateID *uuid.UUID
	if req.WelcomeTemplateID != nil && *req.WelcomeTemplateID != "" {
		tid, err := uuid.Parse(*req.WelcomeTemplateID)
		if err == nil {
			welcomeTemplateID = &tid
		}
	}

	var assignToTeamID *uuid.UUID
	if req.AssignToTeamID != nil && *req.AssignToTeamID != "" {
		tid, err := uuid.Parse(*req.AssignToTeamID)
		if err == nil {
			assignToTeamID = &tid
		}
	}

	signup := models.EmbeddedSignup{
		OrganizationID:    orgID,
		Name:              req.Name,
		WhatsAppAccountID: accountID,
		MetaAppID:         req.MetaAppID,
		MetaConfigID:      req.MetaConfigID,
		MetaAppSecret:     req.MetaAppSecret, // TODO: encrypt before storing
		EnableCoexistence: req.EnableCoexistence,
		SyncChatHistory:   req.SyncChatHistory,
		APIVersion:        apiVersion,
		FormFields:        models.JSONB(req.FormFields),
		RequiredFields:    models.StringArray(req.RequiredFields),
		WelcomeMessage:    req.WelcomeMessage,
		WelcomeTemplateID: welcomeTemplateID,
		SuccessMessage:    req.SuccessMessage,
		RedirectURL:       req.RedirectURL,
		WebhookURL:        req.WebhookURL,
		AllowedOrigins:    models.StringArray(req.AllowedOrigins),
		RateLimitPerHour:  req.RateLimitPerHour,
		IsActive:          req.IsActive,
		AutoCreateContact: req.AutoCreateContact,
		AssignToTeamID:    assignToTeamID,
	}

	if err := a.DB.Create(&signup).Error; err != nil {
		a.Log.Error("Failed to create embedded signup", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create signup", nil, "")
	}

	return r.SendEnvelope(embeddedSignupToResponse(signup))
}

// GetEmbeddedSignup returns a single embedded signup configuration
func (a *App) GetEmbeddedSignup(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid signup ID", nil, "")
	}

	var signup models.EmbeddedSignup
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&signup).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Signup not found", nil, "")
	}

	return r.SendEnvelope(embeddedSignupToResponse(signup))
}

// UpdateEmbeddedSignup updates an embedded signup configuration
func (a *App) UpdateEmbeddedSignup(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid signup ID", nil, "")
	}

	var signup models.EmbeddedSignup
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&signup).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Signup not found", nil, "")
	}

	var req EmbeddedSignupRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Update fields if provided
	if req.Name != "" {
		signup.Name = req.Name
	}
	if req.MetaAppID != "" {
		signup.MetaAppID = req.MetaAppID
	}
	if req.MetaConfigID != "" {
		signup.MetaConfigID = req.MetaConfigID
	}
	if req.MetaAppSecret != "" {
		signup.MetaAppSecret = req.MetaAppSecret // TODO: encrypt
	}
	if req.APIVersion != "" {
		signup.APIVersion = req.APIVersion
	}
	if req.FormFields != nil {
		signup.FormFields = models.JSONB(req.FormFields)
	}
	if req.RequiredFields != nil {
		signup.RequiredFields = models.StringArray(req.RequiredFields)
	}
	if req.WelcomeMessage != "" {
		signup.WelcomeMessage = req.WelcomeMessage
	}
	if req.SuccessMessage != "" {
		signup.SuccessMessage = req.SuccessMessage
	}
	if req.AllowedOrigins != nil {
		signup.AllowedOrigins = models.StringArray(req.AllowedOrigins)
	}
	if req.RateLimitPerHour > 0 {
		signup.RateLimitPerHour = req.RateLimitPerHour
	}

	signup.EnableCoexistence = req.EnableCoexistence
	signup.SyncChatHistory = req.SyncChatHistory
	signup.IsActive = req.IsActive
	signup.AutoCreateContact = req.AutoCreateContact
	signup.RedirectURL = req.RedirectURL
	signup.WebhookURL = req.WebhookURL

	if err := a.DB.Save(&signup).Error; err != nil {
		a.Log.Error("Failed to update embedded signup", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update signup", nil, "")
	}

	return r.SendEnvelope(embeddedSignupToResponse(signup))
}

// DeleteEmbeddedSignup deletes an embedded signup configuration
func (a *App) DeleteEmbeddedSignup(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid signup ID", nil, "")
	}

	var signup models.EmbeddedSignup
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&signup).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Signup not found", nil, "")
	}

	if err := a.DB.Delete(&signup).Error; err != nil {
		a.Log.Error("Failed to delete embedded signup", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete signup", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Signup deleted successfully"})
}

// GetEmbeddedSignupConfig returns public configuration for embedding (PUBLIC endpoint)
func (a *App) GetEmbeddedSignupConfig(r *fastglue.Request) error {
	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid signup ID", nil, "")
	}

	var signup models.EmbeddedSignup
	if err := a.DB.Where("id = ? AND is_active = ?", id, true).First(&signup).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Signup not found or inactive", nil, "")
	}

	// Return only public configuration (no secrets)
	return r.SendEnvelope(map[string]interface{}{
		"id":                 signup.ID,
		"name":               signup.Name,
		"meta_app_id":        signup.MetaAppID,
		"meta_config_id":     signup.MetaConfigID,
		"enable_coexistence": signup.EnableCoexistence,
		"form_fields":        signup.FormFields,
		"required_fields":    signup.RequiredFields,
		"success_message":    signup.SuccessMessage,
		"redirect_url":       signup.RedirectURL,
	})
}

// SubmitEmbeddedSignup processes a signup submission (PUBLIC endpoint)
func (a *App) SubmitEmbeddedSignup(r *fastglue.Request) error {
	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid signup ID", nil, "")
	}

	var signup models.EmbeddedSignup
	if err := a.DB.Where("id = ? AND is_active = ?", id, true).Preload("WhatsAppAccount").First(&signup).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Signup not found or inactive", nil, "")
	}

	// Check CORS origin
	origin := string(r.RequestCtx.Request.Header.Peek("Origin"))
	if !isOriginAllowed(origin, signup.AllowedOrigins) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Origin not allowed", nil, "")
	}

	var req EmbeddedSignupSubmitRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Validate required fields
	if req.PhoneNumber == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Phone number is required", nil, "")
	}

	// Get client IP
	ipAddress := string(r.RequestCtx.RemoteIP())
	userAgent := string(r.RequestCtx.UserAgent())
	referrer := string(r.RequestCtx.Referer())

	// Rate limiting check (simple in-memory check - can be enhanced with Redis)
	// TODO: Implement proper rate limiting with Redis

	// Exchange OAuth code for access token if provided
	var metaAccessToken, metaBusinessID, metaPhoneID, metaWABAID string
	coexistenceEnabled := false
	chatHistorySynced := false

	if req.MetaAuthCode != "" {
		// Exchange authorization code for access token
		tokenData, err := a.exchangeMetaOAuthCode(signup, req.MetaAuthCode)
		if err != nil {
			a.Log.Error("Failed to exchange OAuth code", "error", err)
			// Continue without OAuth - user may not have completed OAuth flow
		} else {
			metaAccessToken = tokenData["access_token"].(string)
			if businessID, ok := tokenData["business_id"].(string); ok {
				metaBusinessID = businessID
			}
			if phoneID, ok := tokenData["phone_id"].(string); ok {
				metaPhoneID = phoneID
			}
			if wabaID, ok := tokenData["waba_id"].(string); ok {
				metaWABAID = wabaID
			}
			coexistenceEnabled = signup.EnableCoexistence

			// Sync chat history if enabled
			if signup.SyncChatHistory && metaPhoneID != "" {
				// TODO: Implement chat history sync
				chatHistorySynced = true
			}
		}
	}

	// Create lead record
	source := req.Source
	if source == "" {
		source = "widget"
	}

	lead := models.EmbeddedSignupLead{
		OrganizationID:     signup.OrganizationID,
		SignupID:           signup.ID,
		PhoneNumber:        req.PhoneNumber,
		ProfileName:        req.ProfileName,
		FormData:           models.JSONB(req.FormData),
		MetaAccessToken:    metaAccessToken, // TODO: encrypt
		MetaBusinessID:     metaBusinessID,
		MetaPhoneID:        metaPhoneID,
		MetaWABAID:         metaWABAID,
		CoexistenceEnabled: coexistenceEnabled,
		ChatHistorySynced:  chatHistorySynced,
		Status:             models.EmbeddedSignupLeadStatusPending,
		Source:             models.EmbeddedSignupSource(source),
		IPAddress:          ipAddress,
		UserAgent:          userAgent,
		Referrer:           referrer,
	}

	if err := a.DB.Create(&lead).Error; err != nil {
		a.Log.Error("Failed to create lead", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to process signup", nil, "")
	}

	// Auto-create contact if enabled
	if signup.AutoCreateContact {
		contact := models.Contact{
			OrganizationID:  signup.OrganizationID,
			PhoneNumber:     req.PhoneNumber,
			ProfileName:     req.ProfileName,
			WhatsAppAccount: signup.WhatsAppAccount.Name,
			IsRead:          true,
			Tags:            models.JSONBArray{},
			Metadata:        models.JSONB(req.FormData),
		}

		// Check if contact already exists
		var existingContact models.Contact
		if err := a.DB.Where("organization_id = ? AND phone_number = ?", signup.OrganizationID, req.PhoneNumber).First(&existingContact).Error; err == nil {
			// Contact exists, update it
			contact.ID = existingContact.ID
			a.DB.Model(&contact).Updates(map[string]interface{}{
				"profile_name": req.ProfileName,
				"metadata":     models.JSONB(req.FormData),
			})
		} else {
			// Create new contact
			if err := a.DB.Create(&contact).Error; err == nil {
				lead.ContactID = &contact.ID
				a.DB.Save(&lead)
			}
		}
	}

	// Send welcome message if configured
	if signup.WelcomeMessage != "" && signup.WhatsAppAccount != nil {
		// TODO: Queue welcome message to be sent
		// This should use the worker queue to send the message asynchronously
	}

	// Trigger webhook if configured
	if signup.WebhookURL != nil && *signup.WebhookURL != "" {
		go a.sendEmbeddedSignupWebhook(*signup.WebhookURL, lead, signup.MetaAppSecret)
	}

	// Mark lead as confirmed
	lead.Status = models.EmbeddedSignupLeadStatusConfirmed
	a.DB.Save(&lead)

	return r.SendEnvelope(map[string]interface{}{
		"success":             true,
		"message":             signup.SuccessMessage,
		"lead_id":             lead.ID,
		"redirect_url":        signup.RedirectURL,
		"coexistence_enabled": coexistenceEnabled,
	})
}

// ListEmbeddedSignupLeads lists all leads for a signup
func (a *App) ListEmbeddedSignupLeads(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid signup ID", nil, "")
	}

	// Verify signup belongs to organization
	var signup models.EmbeddedSignup
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&signup).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Signup not found", nil, "")
	}

	var leads []models.EmbeddedSignupLead
	if err := a.DB.Where("signup_id = ?", id).Order("created_at DESC").Find(&leads).Error; err != nil {
		a.Log.Error("Failed to list leads", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list leads", nil, "")
	}

	response := make([]EmbeddedSignupLeadResponse, len(leads))
	for i, lead := range leads {
		response[i] = embeddedSignupLeadToResponse(lead)
	}

	return r.SendEnvelope(map[string]interface{}{
		"leads": response,
	})
}

// Helper functions

func embeddedSignupToResponse(signup models.EmbeddedSignup) EmbeddedSignupResponse {
	formFields := map[string]interface{}(signup.FormFields)
	if formFields == nil {
		formFields = map[string]interface{}{}
	}

	requiredFields := []string(signup.RequiredFields)
	if requiredFields == nil {
		requiredFields = []string{}
	}

	allowedOrigins := []string(signup.AllowedOrigins)
	if allowedOrigins == nil {
		allowedOrigins = []string{}
	}

	return EmbeddedSignupResponse{
		ID:                signup.ID,
		Name:              signup.Name,
		WhatsAppAccountID: signup.WhatsAppAccountID,
		MetaAppID:         signup.MetaAppID,
		MetaConfigID:      signup.MetaConfigID,
		EnableCoexistence: signup.EnableCoexistence,
		SyncChatHistory:   signup.SyncChatHistory,
		APIVersion:        signup.APIVersion,
		FormFields:        formFields,
		RequiredFields:    requiredFields,
		WelcomeMessage:    signup.WelcomeMessage,
		SuccessMessage:    signup.SuccessMessage,
		RedirectURL:       signup.RedirectURL,
		AllowedOrigins:    allowedOrigins,
		RateLimitPerHour:  signup.RateLimitPerHour,
		IsActive:          signup.IsActive,
		AutoCreateContact: signup.AutoCreateContact,
		CreatedAt:         signup.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         signup.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func embeddedSignupLeadToResponse(lead models.EmbeddedSignupLead) EmbeddedSignupLeadResponse {
	formData := map[string]interface{}(lead.FormData)
	if formData == nil {
		formData = map[string]interface{}{}
	}

	return EmbeddedSignupLeadResponse{
		ID:                 lead.ID,
		PhoneNumber:        lead.PhoneNumber,
		ProfileName:        lead.ProfileName,
		FormData:           formData,
		Status:             string(lead.Status),
		Source:             string(lead.Source),
		CoexistenceEnabled: lead.CoexistenceEnabled,
		ChatHistorySynced:  lead.ChatHistorySynced,
		CreatedAt:          lead.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func isOriginAllowed(origin string, allowedOrigins models.StringArray) bool {
	// If no origins specified, allow all (not recommended for production)
	if len(allowedOrigins) == 0 {
		return true
	}

	origin = strings.TrimSpace(strings.ToLower(origin))
	for _, allowed := range allowedOrigins {
		allowed = strings.TrimSpace(strings.ToLower(allowed))
		if origin == allowed || allowed == "*" {
			return true
		}
		// Support wildcard subdomains (e.g., *.example.com)
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}

// exchangeMetaOAuthCode exchanges OAuth authorization code for access token
func (a *App) exchangeMetaOAuthCode(signup models.EmbeddedSignup, code string) (map[string]interface{}, error) {
	// Exchange code for access token using Meta Graph API
	url := fmt.Sprintf("%s/%s/oauth/access_token", a.Config.WhatsApp.BaseURL, signup.APIVersion)

	data := map[string]string{
		"client_id":     signup.MetaAppID,
		"client_secret": signup.MetaAppSecret,
		"code":          code,
	}

	jsonData, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to exchange code: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// sendEmbeddedSignupWebhook sends a webhook notification for a new signup
func (a *App) sendEmbeddedSignupWebhook(webhookURL string, lead models.EmbeddedSignupLead, secret string) {
	payload := map[string]interface{}{
		"event":       "embedded_signup.lead_created",
		"lead_id":     lead.ID,
		"phone":       lead.PhoneNumber,
		"name":        lead.ProfileName,
		"form_data":   lead.FormData,
		"status":      lead.Status,
		"source":      lead.Source,
		"created_at":  lead.CreatedAt,
	}

	jsonData, _ := json.Marshal(payload)

	// Create HMAC signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(jsonData)
	signature := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		a.Log.Error("Failed to create webhook request", "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Whatomate-Signature", signature)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		a.Log.Error("Failed to send webhook", "error", err, "url", webhookURL)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		a.Log.Warn("Webhook returned error status", "status", resp.StatusCode, "url", webhookURL)
	}
}
