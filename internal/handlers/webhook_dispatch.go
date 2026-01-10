package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
)

// WebhookEvent types
const (
	EventMessageIncoming  = "message.incoming"
	EventMessageSent      = "message.sent"
	EventContactCreated   = "contact.created"
	EventTransferCreated  = "transfer.created"
	EventTransferAssigned = "transfer.assigned"
	EventTransferResumed  = "transfer.resumed"
)

// OutboundWebhookPayload represents the structure sent to external webhook endpoints
type OutboundWebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// MessageEventData represents data for message events
type MessageEventData struct {
	MessageID       string `json:"message_id"`
	ContactID       string `json:"contact_id"`
	ContactPhone    string `json:"contact_phone"`
	ContactName     string `json:"contact_name"`
	MessageType     string `json:"message_type"`
	Content         string `json:"content"`
	WhatsAppAccount string `json:"whatsapp_account"`
	Direction       string `json:"direction,omitempty"`
	SentByUserID    string `json:"sent_by_user_id,omitempty"`
}

// ContactEventData represents data for contact events
type ContactEventData struct {
	ContactID       string `json:"contact_id"`
	ContactPhone    string `json:"contact_phone"`
	ContactName     string `json:"contact_name"`
	WhatsAppAccount string `json:"whatsapp_account"`
}

// TransferEventData represents data for transfer events
type TransferEventData struct {
	TransferID      string  `json:"transfer_id"`
	ContactID       string  `json:"contact_id"`
	ContactPhone    string  `json:"contact_phone"`
	ContactName     string  `json:"contact_name"`
	Source          string  `json:"source"`
	Reason          string  `json:"reason,omitempty"`
	AgentID         *string `json:"agent_id,omitempty"`
	AgentName       *string `json:"agent_name,omitempty"`
	WhatsAppAccount string  `json:"whatsapp_account"`
}

// DispatchWebhook sends an event to all matching webhooks for the organization
func (a *App) DispatchWebhook(orgID uuid.UUID, eventType string, data interface{}) {
	go a.dispatchWebhookAsync(orgID, eventType, data)
}

func (a *App) dispatchWebhookAsync(orgID uuid.UUID, eventType string, data interface{}) {
	// Find all active webhooks for this org that subscribe to this event (use cache)
	webhooks, err := a.getWebhooksCached(orgID)
	if err != nil {
		a.Log.Error("failed to fetch webhooks", "error", err)
		return
	}

	for _, webhook := range webhooks {
		// Check if webhook subscribes to this event
		if !containsEvent(webhook.Events, eventType) {
			continue
		}

		// Send webhook asynchronously
		go a.sendWebhook(webhook, eventType, data)
	}
}

func containsEvent(events models.StringArray, event string) bool {
	for _, e := range events {
		if e == event {
			return true
		}
	}
	return false
}

func (a *App) sendWebhook(webhook models.Webhook, eventType string, data interface{}) {
	payload := OutboundWebhookPayload{
		Event:     eventType,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		a.Log.Error("failed to marshal webhook payload", "error", err, "webhook_id", webhook.ID)
		return
	}

	// Retry logic with exponential backoff
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}

		if err := a.sendWebhookRequest(webhook, jsonData); err != nil {
			a.Log.Warn("webhook delivery failed",
				"error", err,
				"webhook_id", webhook.ID,
				"attempt", attempt+1,
				"max_retries", maxRetries,
			)
			continue
		}

		// Success
		a.Log.Debug("webhook delivered",
			"webhook_id", webhook.ID,
			"event", eventType,
			"url", webhook.URL,
		)
		return
	}

	a.Log.Error("webhook delivery failed after all retries",
		"webhook_id", webhook.ID,
		"event", eventType,
		"url", webhook.URL,
	)
}

func (a *App) sendWebhookRequest(webhook models.Webhook, jsonData []byte) error {
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Whatomate-Webhook/1.0")

	// Add custom headers from webhook config
	if webhook.Headers != nil {
		for key, value := range webhook.Headers {
			if strValue, ok := value.(string); ok {
				req.Header.Set(key, strValue)
			}
		}
	}

	// Add HMAC signature if secret is configured
	if webhook.Secret != "" {
		signature := computeHMACSignature(jsonData, webhook.Secret)
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// Send request with timeout
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for successful status code (2xx)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &WebhookError{StatusCode: resp.StatusCode}
	}

	return nil
}

func computeHMACSignature(data []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// WebhookError represents a webhook delivery error
type WebhookError struct {
	StatusCode int
}

func (e *WebhookError) Error() string {
	return "webhook returned non-2xx status: " + http.StatusText(e.StatusCode)
}
