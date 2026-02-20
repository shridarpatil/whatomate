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

	"fmt"

	"github.com/dop251/goja"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// CustomActionRequest represents the request body for creating/updating a custom action
type CustomActionRequest struct {
	Name         string                 `json:"name"`
	Icon         string                 `json:"icon"`
	ActionType   models.ActionType      `json:"action_type"` // webhook, url, javascript
	Config       map[string]interface{} `json:"config"`
	IsActive     bool                   `json:"is_active"`
	DisplayOrder int                    `json:"display_order"`
}

// CustomActionResponse represents the API response for a custom action
type CustomActionResponse struct {
	ID           uuid.UUID              `json:"id"`
	Name         string                 `json:"name"`
	Icon         string                 `json:"icon"`
	ActionType   models.ActionType      `json:"action_type"`
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
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	query := a.DB.Model(&models.CustomAction{}).Where("organization_id = ?", orgID)

	// Apply search filter - search by name (case-insensitive)
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("name ILIKE ?", searchPattern)
	}

	var total int64
	query.Count(&total)

	var actions []models.CustomAction
	if err := pg.Apply(query.Order("display_order ASC, created_at DESC")).
		Find(&actions).Error; err != nil {
		a.Log.Error("Failed to list custom actions", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list custom actions", nil, "")
	}

	result := make([]CustomActionResponse, len(actions))
	for i, action := range actions {
		result[i] = customActionToResponse(action)
	}

	return r.SendEnvelope(map[string]any{
		"custom_actions": result,
		"total":          total,
		"page":           pg.Page,
		"limit":          pg.Limit,
	})
}

// GetCustomAction returns a single custom action by ID
func (a *App) GetCustomAction(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	actionID, err := parsePathUUID(r, "id", "action")
	if err != nil {
		return nil
	}

	action, err := findByIDAndOrg[models.CustomAction](a.DB, r, actionID, orgID, "Custom action")
	if err != nil {
		return nil
	}

	return r.SendEnvelope(customActionToResponse(*action))
}

// CreateCustomAction creates a new custom action
func (a *App) CreateCustomAction(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req CustomActionRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate required fields
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Name is required", nil, "")
	}
	if req.ActionType == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Action type is required", nil, "")
	}
	if req.ActionType != models.ActionTypeWebhook && req.ActionType != models.ActionTypeURL && req.ActionType != models.ActionTypeJavascript {
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
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	actionID, err := parsePathUUID(r, "id", "action")
	if err != nil {
		return nil
	}

	action, err := findByIDAndOrg[models.CustomAction](a.DB, r, actionID, orgID, "Custom action")
	if err != nil {
		return nil
	}

	var req CustomActionRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
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
		if req.ActionType != models.ActionTypeWebhook && req.ActionType != models.ActionTypeURL && req.ActionType != models.ActionTypeJavascript {
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

	if err := a.DB.Model(action).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update custom action", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update custom action", nil, "")
	}

	// Reload to get updated values
	a.DB.First(action, actionID)

	a.Log.Info("Custom action updated", "action_id", action.ID)
	return r.SendEnvelope(customActionToResponse(*action))
}

// DeleteCustomAction deletes a custom action
func (a *App) DeleteCustomAction(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	actionID, err := parsePathUUID(r, "id", "action")
	if err != nil {
		return nil
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
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	actionID, err := parsePathUUID(r, "id", "action")
	if err != nil {
		return nil
	}

	var req ExecuteActionRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Get the action
	action, err := findByIDAndOrg[models.CustomAction](a.DB, r, actionID, orgID, "Custom action")
	if err != nil {
		return nil
	}

	if !action.IsActive {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Custom action is not active", nil, "")
	}

	// Get contact details
	contactID, err := uuid.Parse(req.ContactID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact ID", nil, "")
	}

	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	// Get user details
	var user models.User
	a.DB.First(&user, userID)

	// Get organization details
	var org models.Organization
	a.DB.First(&org, orgID)

	// Build context for variable replacement
	context := buildActionContext(*contact, user, org)

	// Execute based on action type
	var result *ActionResult
	switch action.ActionType {
	case models.ActionTypeWebhook:
		result, err = a.executeWebhookAction(*action, context)
	case models.ActionTypeURL:
		result, err = a.executeURLAction(*action, context)
	case models.ActionTypeJavascript:
		result, err = a.executeJavaScriptAction(*action, context)
	default:
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Unknown action type", nil, "")
	}

	if err != nil {
		a.Log.Error("Failed to execute custom action", "error", err, "action_id", actionID)
		return r.SendEnvelope(ActionResult{
			Success: false,
			Message: "Action execution failed",
			Toast:   &ToastConfig{Message: "Action failed", Type: "error"},
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

	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.HTTPClient.Do(req)
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

// executeJavaScriptAction executes a JavaScript action server-side using goja.
// The code runs in a sandboxed VM with no access to filesystem, network, or globals.
func (a *App) executeJavaScriptAction(action models.CustomAction, context map[string]interface{}) (*ActionResult, error) {
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

	if config.Code == "" {
		return &ActionResult{Success: true, Message: "No code to execute"}, nil
	}

	vm := goja.New()

	// Inject context variables (read-only data)
	if err := vm.Set("context", context); err != nil {
		return nil, fmt.Errorf("failed to set context: %w", err)
	}
	if contact, ok := context["contact"]; ok {
		_ = vm.Set("contact", contact)
	}
	if user, ok := context["user"]; ok {
		_ = vm.Set("user", user)
	}
	if org, ok := context["organization"]; ok {
		_ = vm.Set("organization", org)
	}

	// Wrap user code in an IIFE so return works
	wrapped := fmt.Sprintf("(function(context, contact, user, organization) { %s })(context, contact, user, organization)", config.Code)

	val, err := vm.RunString(wrapped)
	if err != nil {
		return nil, fmt.Errorf("javascript execution error: %w", err)
	}

	result := &ActionResult{
		Success: true,
		Message: "JavaScript action executed",
		Toast:   &ToastConfig{Message: "Action completed", Type: "success"},
	}

	// Extract structured result from the JS return value
	if val != nil && !goja.IsUndefined(val) && !goja.IsNull(val) {
		exported := val.Export()
		if jsResult, ok := exported.(map[string]interface{}); ok {
			if t, ok := jsResult["toast"].(map[string]interface{}); ok {
				result.Toast = &ToastConfig{
					Message: fmt.Sprintf("%v", t["message"]),
					Type:    fmt.Sprintf("%v", t["type"]),
				}
			}
			if clip, ok := jsResult["clipboard"].(string); ok {
				result.Clipboard = clip
			}
			if url, ok := jsResult["url"].(string); ok {
				result.RedirectURL = url
			}
			if msg, ok := jsResult["message"].(string); ok {
				result.Message = msg
			}
		}
	}

	return result, nil
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
func validateActionConfig(actionType models.ActionType, config map[string]interface{}) error {
	switch actionType {
	case models.ActionTypeWebhook:
		urlVal, ok := config["url"]
		if !ok {
			return &ValidationError{Field: "config.url", Message: "URL is required for webhook actions"}
		}
		if urlStr, ok := urlVal.(string); ok {
			if err := validateWebhookURL(urlStr); err != nil {
				return &ValidationError{Field: "config.url", Message: err.Error()}
			}
		}
	case models.ActionTypeURL:
		if _, ok := config["url"]; !ok {
			return &ValidationError{Field: "config.url", Message: "URL is required for URL actions"}
		}
	case models.ActionTypeJavascript:
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
