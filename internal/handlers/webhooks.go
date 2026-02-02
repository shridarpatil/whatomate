package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// WebhookRequest represents the request body for creating/updating a webhook
type WebhookRequest struct {
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Events   []string          `json:"events"`
	Headers  map[string]string `json:"headers"`
	Secret   string            `json:"secret"`
	IsActive bool              `json:"is_active"`
}

// WebhookResponse represents the API response for a webhook
type WebhookResponse struct {
	ID        uuid.UUID         `json:"id"`
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Events    []string          `json:"events"`
	Headers   map[string]string `json:"headers"`
	IsActive  bool              `json:"is_active"`
	HasSecret bool              `json:"has_secret"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// AvailableWebhookEvents returns the list of available webhook event types
var AvailableWebhookEvents = []map[string]string{
	{"value": string(models.WebhookEventMessageIncoming), "label": "Message Incoming", "description": "When a new message is received from a contact"},
	{"value": string(models.WebhookEventMessageSent), "label": "Message Sent", "description": "When an agent sends a message"},
	{"value": string(models.WebhookEventContactCreated), "label": "Contact Created", "description": "When a new contact is created"},
	{"value": string(models.WebhookEventTransferCreated), "label": "Transfer Created", "description": "When a transfer to human agent is requested"},
	{"value": string(models.WebhookEventTransferAssigned), "label": "Transfer Assigned", "description": "When a transfer is assigned to an agent"},
	{"value": string(models.WebhookEventTransferResumed), "label": "Transfer Resumed", "description": "When chatbot is resumed (transfer closed)"},
}

// ListWebhooks returns all webhooks for the organization
func (a *App) ListWebhooks(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	pg := parsePagination(r)

	var total int64
	a.DB.Model(&models.Webhook{}).Where("organization_id = ?", orgID).Count(&total)

	var webhooks []models.Webhook
	if err := pg.Apply(a.DB.Where("organization_id = ?", orgID).
		Order("created_at DESC")).
		Find(&webhooks).Error; err != nil {
		a.Log.Error("Failed to list webhooks", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list webhooks", nil, "")
	}

	result := make([]WebhookResponse, len(webhooks))
	for i, wh := range webhooks {
		result[i] = webhookToResponse(wh)
	}

	return r.SendEnvelope(map[string]any{
		"webhooks":         result,
		"available_events": AvailableWebhookEvents,
		"total":            total,
		"page":             pg.Page,
		"limit":            pg.Limit,
	})
}

// GetWebhook returns a single webhook by ID
func (a *App) GetWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	webhookID, err := parsePathUUID(r, "id", "webhook")
	if err != nil {
		return nil
	}

	webhook, err := findByIDAndOrg[models.Webhook](a.DB, r, webhookID, orgID, "Webhook")
	if err != nil {
		return nil
	}

	return r.SendEnvelope(webhookToResponse(*webhook))
}

// CreateWebhook creates a new webhook
func (a *App) CreateWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req WebhookRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name == "" || req.URL == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name and url are required", nil, "")
	}

	if len(req.Events) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "at least one event must be selected", nil, "")
	}

	// Convert headers to JSONB
	headers := models.JSONB{}
	for k, v := range req.Headers {
		headers[k] = v
	}

	webhook := models.Webhook{
		OrganizationID: orgID,
		Name:           req.Name,
		URL:            req.URL,
		Events:         req.Events,
		Headers:        headers,
		Secret:         req.Secret,
		IsActive:       true,
	}

	if err := a.DB.Create(&webhook).Error; err != nil {
		a.Log.Error("Failed to create webhook", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create webhook", nil, "")
	}

	// Invalidate cache
	a.InvalidateWebhooksCache(orgID)

	return r.SendEnvelope(webhookToResponse(webhook))
}

// UpdateWebhook updates an existing webhook
func (a *App) UpdateWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	webhookID, err := parsePathUUID(r, "id", "webhook")
	if err != nil {
		return nil
	}

	webhook, err := findByIDAndOrg[models.Webhook](a.DB, r, webhookID, orgID, "Webhook")
	if err != nil {
		return nil
	}

	var req WebhookRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name != "" {
		webhook.Name = req.Name
	}
	if req.URL != "" {
		webhook.URL = req.URL
	}
	if len(req.Events) > 0 {
		webhook.Events = req.Events
	}

	// Update headers if provided
	if req.Headers != nil {
		headers := models.JSONB{}
		for k, v := range req.Headers {
			headers[k] = v
		}
		webhook.Headers = headers
	}

	// Update secret if provided (empty string clears it)
	if req.Secret != "" {
		webhook.Secret = req.Secret
	}

	webhook.IsActive = req.IsActive

	if err := a.DB.Save(webhook).Error; err != nil {
		a.Log.Error("Failed to update webhook", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update webhook", nil, "")
	}

	// Invalidate cache
	a.InvalidateWebhooksCache(orgID)

	return r.SendEnvelope(webhookToResponse(*webhook))
}

// DeleteWebhook deletes a webhook
func (a *App) DeleteWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	webhookID, err := parsePathUUID(r, "id", "webhook")
	if err != nil {
		return nil
	}

	result := a.DB.Where("id = ? AND organization_id = ?", webhookID, orgID).Delete(&models.Webhook{})
	if result.Error != nil {
		a.Log.Error("Failed to delete webhook", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete webhook", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Webhook not found", nil, "")
	}

	// Invalidate cache
	a.InvalidateWebhooksCache(orgID)

	return r.SendEnvelope(map[string]string{"message": "Webhook deleted successfully"})
}

// TestWebhook sends a test event to a webhook
func (a *App) TestWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	webhookID, err := parsePathUUID(r, "id", "webhook")
	if err != nil {
		return nil
	}

	webhook, err := findByIDAndOrg[models.Webhook](a.DB, r, webhookID, orgID, "Webhook")
	if err != nil {
		return nil
	}

	// Send a test event synchronously
	testData := map[string]interface{}{
		"test":      true,
		"message":   "This is a test webhook from Whatomate",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	payload := OutboundWebhookPayload{
		Event:     "test",
		Timestamp: time.Now().UTC(),
		Data:      testData,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create test payload", nil, "")
	}

	// Use timeout context for test webhook request
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := a.sendWebhookRequest(ctx, *webhook, jsonData); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadGateway, "Webhook test failed: "+err.Error(), nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Test webhook sent successfully"})
}

func webhookToResponse(wh models.Webhook) WebhookResponse {
	// Convert events
	events := make([]string, len(wh.Events))
	copy(events, wh.Events)

	// Convert headers
	headers := make(map[string]string)
	for k, v := range wh.Headers {
		if strVal, ok := v.(string); ok {
			headers[k] = strVal
		}
	}

	return WebhookResponse{
		ID:        wh.ID,
		Name:      wh.Name,
		URL:       wh.URL,
		Events:    events,
		Headers:   headers,
		IsActive:  wh.IsActive,
		HasSecret: wh.Secret != "",
		CreatedAt: wh.CreatedAt.Format(time.RFC3339),
		UpdatedAt: wh.UpdatedAt.Format(time.RFC3339),
	}
}
