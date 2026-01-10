package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// CustomActionRequest represents the request body for creating/updating a custom action
type CustomActionRequest struct {
	Name         string                 `json:"name"`
	Icon         string                 `json:"icon"`
	ActionType   string                 `json:"action_type"` // webhook, url, javascript
	Config       map[string]interface{} `json:"config"`
	IsActive     bool                   `json:"is_active"`
	DisplayOrder int                    `json:"display_order"`
}

// CustomActionResponse represents the API response for a custom action
type CustomActionResponse struct {
	ID           uuid.UUID              `json:"id"`
	Name         string                 `json:"name"`
	Icon         string                 `json:"icon"`
	ActionType   string                 `json:"action_type"`
	Config       map[string]interface{} `json:"config"`
	IsActive     bool                   `json:"is_active"`
	DisplayOrder int                    `json:"display_order"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

// ExecuteActionRequest represents the request to execute a custom action
type ExecuteActionRequest struct {
	ContactID string `json:"contact_id"`
}

// ActionResult represents the result of executing a custom action
type ActionResult struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message,omitempty"`
	RedirectURL string                 `json:"redirect_url,omitempty"`
	Clipboard   string                 `json:"clipboard,omitempty"`
	Toast       *ToastConfig           `json:"toast,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// ToastConfig represents a toast notification configuration
type ToastConfig struct {
	Message string `json:"message"`
	Type    string `json:"type"` // success, error, info, warning
}

// Redirect token storage (in production, use Redis)
var (
	redirectTokens     = make(map[string]redirectToken)
	redirectTokenMutex sync.RWMutex
)

type redirectToken struct {
	URL       string
	ExpiresAt time.Time
}

// ListCustomActions returns all custom actions for the organization
func (a *App) ListCustomActions(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var actions []models.CustomAction
	if err := a.DB.Where("organization_id = ?", orgID).Order("display_order ASC, created_at DESC").Find(&actions).Error; err != nil {
		a.Log.Error("Failed to list custom actions", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list custom actions", nil, "")
	}

	result := make([]CustomActionResponse, len(actions))
	for i, action := range actions {
		result[i] = customActionToResponse(action)
	}

	return r.SendEnvelope(map[string]interface{}{
		"custom_actions": result,
	})
}

// GetCustomAction returns a single custom action by ID
func (a *App) GetCustomAction(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	actionID, err := uuid.Parse(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid action ID", nil, "")
	}

	var action models.CustomAction
	if err := a.DB.Where("id = ? AND organization_id = ?", actionID, orgID).First(&action).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Custom action not found", nil, "")
	}

	return r.SendEnvelope(customActionToResponse(action))
}

// CreateCustomAction creates a new custom action
func (a *App) CreateCustomAction(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req CustomActionRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Validate required fields
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Name is required", nil, "")
	}
	if req.ActionType == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Action type is required", nil, "")
	}
	if req.ActionType != "webhook" && req.ActionType != "url" && req.ActionType != "javascript" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid action type. Must be webhook, url, or javascript", nil, "")
	}

	// Validate config based on action type
	if err := validateActionConfig(req.ActionType, req.Config); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	action := models.CustomAction{
		OrganizationID: orgID,
		Name:           req.Name,
		Icon:           req.Icon,
		ActionType:     req.ActionType,
		Config:         models.JSONB(req.Config),
		IsActive:       req.IsActive,
		DisplayOrder:   req.DisplayOrder,
	}

	if err := a.DB.Create(&action).Error; err != nil {
		a.Log.Error("Failed to create custom action", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create custom action", nil, "")
	}

	a.Log.Info("Custom action created", "action_id", action.ID, "name", action.Name, "type", action.ActionType)
	return r.SendEnvelope(customActionToResponse(action))
}

// UpdateCustomAction updates an existing custom action
func (a *App) UpdateCustomAction(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	actionID, err := uuid.Parse(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid action ID", nil, "")
	}

	var action models.CustomAction
	if err := a.DB.Where("id = ? AND organization_id = ?", actionID, orgID).First(&action).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Custom action not found", nil, "")
	}

	var req CustomActionRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Build updates
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}
	if req.ActionType != "" {
		if req.ActionType != "webhook" && req.ActionType != "url" && req.ActionType != "javascript" {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid action type", nil, "")
		}
		updates["action_type"] = req.ActionType
	}
	if req.Config != nil {
		actionType := req.ActionType
		if actionType == "" {
			actionType = action.ActionType
		}
		if err := validateActionConfig(actionType, req.Config); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
		configJSON, _ := json.Marshal(req.Config)
		updates["config"] = configJSON
	}
	updates["is_active"] = req.IsActive
	updates["display_order"] = req.DisplayOrder

	if err := a.DB.Model(&action).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update custom action", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update custom action", nil, "")
	}

	// Reload to get updated values
	a.DB.First(&action, actionID)

	a.Log.Info("Custom action updated", "action_id", action.ID)
	return r.SendEnvelope(customActionToResponse(action))
}

// DeleteCustomAction deletes a custom action
func (a *App) DeleteCustomAction(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	actionID, err := uuid.Parse(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid action ID", nil, "")
	}

	result := a.DB.Where("id = ? AND organization_id = ?", actionID, orgID).Delete(&models.CustomAction{})
	if result.Error != nil {
		a.Log.Error("Failed to delete custom action", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete custom action", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Custom action not found", nil, "")
	}

	a.Log.Info("Custom action deleted", "action_id", actionID)
	return r.SendEnvelope(map[string]string{"status": "deleted"})
}

// ExecuteCustomAction executes a custom action with the given context
func (a *App) ExecuteCustomAction(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	actionID, err := uuid.Parse(r.RequestCtx.UserValue("id").(string))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid action ID", nil, "")
	}

	var req ExecuteActionRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Get the action
	var action models.CustomAction
	if err := a.DB.Where("id = ? AND organization_id = ?", actionID, orgID).First(&action).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Custom action not found", nil, "")
	}

	if !action.IsActive {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Custom action is not active", nil, "")
	}

	// Get contact details
	contactID, err := uuid.Parse(req.ContactID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact ID", nil, "")
	}

	var contact models.Contact
	if err := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID).First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Get user details
	var user models.User
	a.DB.First(&user, userID)

	// Get organization details
	var org models.Organization
	a.DB.First(&org, orgID)

	// Build context for variable replacement
	context := buildActionContext(contact, user, org)

	// Execute based on action type
	var result *ActionResult
	switch action.ActionType {
	case "webhook":
		result, err = a.executeWebhookAction(action, context)
	case "url":
		result, err = a.executeURLAction(action, context)
	case "javascript":
		result, err = a.executeJavaScriptAction(action, context)
	default:
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Unknown action type", nil, "")
	}

	if err != nil {
		a.Log.Error("Failed to execute custom action", "error", err, "action_id", actionID)
		return r.SendEnvelope(ActionResult{
			Success: false,
			Message: err.Error(),
			Toast:   &ToastConfig{Message: "Action failed: " + err.Error(), Type: "error"},
		})
	}

	a.Log.Info("Custom action executed", "action_id", actionID, "contact_id", contactID)
	return r.SendEnvelope(result)
}

// CustomActionRedirect handles redirect tokens for URL actions
func (a *App) CustomActionRedirect(r *fastglue.Request) error {
	token := r.RequestCtx.UserValue("token").(string)

	redirectTokenMutex.RLock()
	rt, exists := redirectTokens[token]
	redirectTokenMutex.RUnlock()

	if !exists {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Invalid or expired redirect token", nil, "")
	}

	if time.Now().After(rt.ExpiresAt) {
		// Clean up expired token
		redirectTokenMutex.Lock()
		delete(redirectTokens, token)
		redirectTokenMutex.Unlock()
		return r.SendErrorEnvelope(fasthttp.StatusGone, "Redirect token has expired", nil, "")
	}

	// Delete token (one-time use)
	redirectTokenMutex.Lock()
	delete(redirectTokens, token)
	redirectTokenMutex.Unlock()

	// Redirect to the actual URL
	r.RequestCtx.Redirect(rt.URL, fasthttp.StatusFound)
	return nil
}

// executeWebhookAction executes a webhook action
func (a *App) executeWebhookAction(action models.CustomAction, context map[string]interface{}) (*ActionResult, error) {
	// Parse config from JSONB (already a map)
	configBytes, err := json.Marshal(action.Config)
	if err != nil {
		return nil, err
	}
	var config struct {
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, err
	}

	// Replace variables in URL
	url := replaceVariables(config.URL, context)

	// Replace variables in headers
	headers := make(map[string]string)
	for k, v := range config.Headers {
		headers[k] = replaceVariables(v, context)
	}

	// Replace variables in body or use default
	var body string
	if config.Body != "" {
		body = replaceVariables(config.Body, context)
	} else {
		// Default body with all context
		bodyJSON, _ := json.Marshal(context)
		body = string(bodyJSON)
	}

	// Make HTTP request
	method := config.Method
	if method == "" {
		method = "POST"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)

	// Parse response
	var responseData map[string]interface{}
	_ = json.Unmarshal(respBody, &responseData) // Ignore parse errors for optional response data

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	message := "Webhook executed successfully"
	if !success {
		message = "Webhook returned status " + resp.Status
	}

	return &ActionResult{
		Success: success,
		Message: message,
		Data:    responseData,
		Toast:   &ToastConfig{Message: message, Type: boolToToastType(success)},
	}, nil
}

// executeURLAction executes a URL action by creating a redirect token
func (a *App) executeURLAction(action models.CustomAction, context map[string]interface{}) (*ActionResult, error) {
	// Parse config from JSONB (already a map)
	configBytes, err := json.Marshal(action.Config)
	if err != nil {
		return nil, err
	}
	var config struct {
		URL          string `json:"url"`
		OpenInNewTab bool   `json:"open_in_new_tab"`
	}
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, err
	}

	// Replace variables in URL
	finalURL := replaceVariables(config.URL, context)

	// Generate a random token
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	// Store the redirect token (expires in 30 seconds)
	redirectTokenMutex.Lock()
	redirectTokens[token] = redirectToken{
		URL:       finalURL,
		ExpiresAt: time.Now().Add(30 * time.Second),
	}
	redirectTokenMutex.Unlock()

	// Return the redirect URL
	redirectURL := "/api/custom-actions/redirect/" + token

	return &ActionResult{
		Success:     true,
		Message:     "Opening URL",
		RedirectURL: redirectURL,
	}, nil
}

// executeJavaScriptAction executes a JavaScript action
func (a *App) executeJavaScriptAction(action models.CustomAction, context map[string]interface{}) (*ActionResult, error) {
	// Parse config from JSONB (already a map)
	configBytes, err := json.Marshal(action.Config)
	if err != nil {
		return nil, err
	}
	var config struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, err
	}

	// For MVP: Just return the context data and let frontend handle it
	// In future: Use goja to execute JavaScript on the server
	return &ActionResult{
		Success: true,
		Message: "JavaScript action executed",
		Data: map[string]interface{}{
			"code":    config.Code,
			"context": context,
		},
		Toast: &ToastConfig{Message: "Action completed", Type: "success"},
	}, nil
}

// buildActionContext builds the context object for variable replacement
func buildActionContext(contact models.Contact, user models.User, org models.Organization) map[string]interface{} {
	return map[string]interface{}{
		"contact": map[string]interface{}{
			"id":           contact.ID.String(),
			"phone_number": contact.PhoneNumber,
			"name":         contact.ProfileName,
			"profile_name": contact.ProfileName,
			"tags":         contact.Tags,
			"metadata":     contact.Metadata,
		},
		"user": map[string]interface{}{
			"id":    user.ID.String(),
			"name":  user.FullName,
			"email": user.Email,
			"role":  user.Role,
		},
		"organization": map[string]interface{}{
			"id":   org.ID.String(),
			"name": org.Name,
		},
	}
}

// replaceVariables replaces {{variable}} placeholders with context values
func replaceVariables(template string, context map[string]interface{}) string {
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	return re.ReplaceAllStringFunc(template, func(match string) string {
		// Extract variable path (e.g., "contact.phone_number")
		path := strings.TrimSuffix(strings.TrimPrefix(match, "{{"), "}}")
		path = strings.TrimSpace(path)

		parts := strings.Split(path, ".")
		var value interface{} = context

		for _, part := range parts {
			if m, ok := value.(map[string]interface{}); ok {
				value = m[part]
			} else {
				return match // Return original if path not found
			}
		}

		if value == nil {
			return ""
		}

		switch v := value.(type) {
		case string:
			return v
		case []string:
			return strings.Join(v, ", ")
		default:
			jsonBytes, _ := json.Marshal(v)
			return string(jsonBytes)
		}
	})
}

// validateActionConfig validates the config based on action type
func validateActionConfig(actionType string, config map[string]interface{}) error {
	switch actionType {
	case "webhook":
		if _, ok := config["url"]; !ok {
			return &ValidationError{Field: "config.url", Message: "URL is required for webhook actions"}
		}
	case "url":
		if _, ok := config["url"]; !ok {
			return &ValidationError{Field: "config.url", Message: "URL is required for URL actions"}
		}
	case "javascript":
		if _, ok := config["code"]; !ok {
			return &ValidationError{Field: "config.code", Message: "Code is required for JavaScript actions"}
		}
	}
	return nil
}

// customActionToResponse converts a CustomAction model to response
func customActionToResponse(action models.CustomAction) CustomActionResponse {
	// Config is already a map[string]interface{}, just use it directly
	config := map[string]interface{}(action.Config)

	return CustomActionResponse{
		ID:           action.ID,
		Name:         action.Name,
		Icon:         action.Icon,
		ActionType:   action.ActionType,
		Config:       config,
		IsActive:     action.IsActive,
		DisplayOrder: action.DisplayOrder,
		CreatedAt:    action.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    action.UpdatedAt.Format(time.RFC3339),
	}
}

// boolToToastType converts success boolean to toast type
func boolToToastType(success bool) string {
	if success {
		return "success"
	}
	return "error"
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
