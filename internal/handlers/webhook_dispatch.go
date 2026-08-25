package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
)

// OutboundWebhookPayload represents the structure sent to external webhook endpoints
type OutboundWebhookPayload struct {
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
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

// WebhookTemplateContext is the data exposed to a webhook body template. Data is
// the event payload flattened to a map keyed by JSON field names, so templates
// reference fields as {{ .Data.contact_name }} rather than by Go field name.
type WebhookTemplateContext struct {
	Event     string         `json:"event"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// webhookTemplateFuncs are the helpers available inside a webhook body template.
var webhookTemplateFuncs = template.FuncMap{
	// json marshals a value to a JSON literal (with surrounding quotes for
	// strings and proper escaping), making it safe to embed inside a JSON body.
	// e.g. {"text": {{ json .Data.content }}} handles quotes/newlines correctly.
	"json":  func(v any) (string, error) { b, err := json.Marshal(v); return string(b), err },
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
}

// parseWebhookTemplate compiles a webhook body template with the shared helpers.
// Create/update validation and delivery both use this so a template that
// validates on save renders identically at delivery time.
func parseWebhookTemplate(body string) (*template.Template, error) {
	return template.New("webhook_body").Funcs(webhookTemplateFuncs).Parse(body)
}

// dataToMap converts a typed event payload to a map keyed by JSON field names so
// templates can reference .Data.contact_name rather than the Go field name.
func dataToMap(data any) (map[string]any, error) {
	if data == nil {
		return map[string]any{}, nil
	}
	if m, ok := data.(map[string]any); ok {
		return m, nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// renderWebhookBody builds the HTTP body for a webhook delivery. With no
// BodyTemplate it returns the default JSON envelope ({event,timestamp,data}).
// With a template it renders that instead, exposing WebhookTemplateContext.
func renderWebhookBody(webhook models.Webhook, eventType string, data any, now time.Time) ([]byte, error) {
	if strings.TrimSpace(webhook.BodyTemplate) == "" {
		return json.Marshal(OutboundWebhookPayload{
			Event:     eventType,
			Timestamp: now,
			Data:      data,
		})
	}

	tmpl, err := parseWebhookTemplate(webhook.BodyTemplate)
	if err != nil {
		return nil, err
	}

	dataMap, err := dataToMap(data)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, WebhookTemplateContext{
		Event:     eventType,
		Timestamp: now.Format(time.RFC3339),
		Data:      dataMap,
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DispatchWebhook sends an event to all matching webhooks for the organization
func (a *App) DispatchWebhook(orgID uuid.UUID, eventType models.WebhookEvent, data any) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		// Use detached context with timeout for webhook delivery
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		a.dispatchWebhookAsync(ctx, orgID, string(eventType), data)
	}()
}

func (a *App) dispatchWebhookAsync(ctx context.Context, orgID uuid.UUID, eventType string, data any) {
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

func (a *App) sendWebhook(ctx context.Context, webhook models.Webhook, eventType string, data any) {
	jsonData, err := renderWebhookBody(webhook, eventType, data, time.Now().UTC())
	if err != nil {
		a.Log.Error("failed to render webhook body", "error", err, "webhook_id", webhook.ID)
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
