package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
)


// OutboundWebhookPayload represents the structure sent to external webhook endpoints
type OutboundWebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// MessageEventData represents data for message events
type MessageEventData struct {
	MessageID       string             `json:"message_id"`
	ContactID       string             `json:"contact_id"`
	ContactPhone    string             `json:"contact_phone"`
	ContactName     string             `json:"contact_name"`
	MessageType     models.MessageType `json:"message_type"`
	Content         string             `json:"content"`
	WhatsAppAccount string             `json:"whatsapp_account"`
	Direction       models.Direction   `json:"direction,omitempty"`
	SentByUserID    string             `json:"sent_by_user_id,omitempty"`
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
	TransferID      string                `json:"transfer_id"`
	ContactID       string                `json:"contact_id"`
	ContactPhone    string                `json:"contact_phone"`
	ContactName     string                `json:"contact_name"`
	Source          models.TransferSource `json:"source"`
	Reason          string                `json:"reason,omitempty"`
	AgentID         *string               `json:"agent_id,omitempty"`
	AgentName       *string               `json:"agent_name,omitempty"`
	WhatsAppAccount string                `json:"whatsapp_account"`
}

// maxConcurrentWebhooks limits the number of concurrent webhook deliveries per dispatch
const maxConcurrentWebhooks = 10

// DispatchWebhook sends an event to all matching webhooks for the organization
func (a *App) DispatchWebhook(orgID uuid.UUID, eventType models.WebhookEvent, data interface{}) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		// Use detached context with timeout for webhook delivery
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		a.dispatchWebhookAsync(ctx, orgID, string(eventType), data)
	}()
}

func (a *App) dispatchWebhookAsync(ctx context.Context, orgID uuid.UUID, eventType string, data interface{}) {
	// Find all active webhooks for this org that subscribe to this event (use cache)
	webhooks, err := a.getWebhooksCached(orgID)
	if err != nil {
		a.Log.Error("failed to fetch webhooks", "error", err)
		return
	}

	// Use semaphore to limit concurrent webhook calls
	sem := make(chan struct{}, maxConcurrentWebhooks)
	var wg sync.WaitGroup

	for _, webhook := range webhooks {
		// Check if webhook subscribes to this event
		if !containsEvent(webhook.Events, eventType) {
			continue
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			a.Log.Warn("webhook dispatch cancelled", "reason", ctx.Err())
			break
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore slot

		go func(wh models.Webhook) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore slot
			a.sendWebhook(ctx, wh, eventType, data)
		}(webhook)
	}

	wg.Wait()
}

func containsEvent(events models.StringArray, event string) bool {
	for _, e := range events {
		if e == event {
			return true
		}
	}
	return false
}

func (a *App) sendWebhook(ctx context.Context, webhook models.Webhook, eventType string, data interface{}) {
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
		// Check if context was cancelled before retry
		if ctx.Err() != nil {
			a.Log.Warn("webhook delivery cancelled", "reason", ctx.Err(), "webhook_id", webhook.ID)
			return
		}

		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			select {
			case <-ctx.Done():
				a.Log.Warn("webhook delivery cancelled during backoff", "reason", ctx.Err(), "webhook_id", webhook.ID)
				return
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}

		if err := a.sendWebhookRequest(ctx, webhook, jsonData); err != nil {
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

func (a *App) sendWebhookRequest(ctx context.Context, webhook models.Webhook, jsonData []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewBuffer(jsonData))
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

	// Send request
	resp, err := a.HTTPClient.Do(req)
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
