// Package aivalidation checks AI credentials with a small, synthetic request.
package aivalidation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Validate returns a safe, stable error code. Provider bodies and credentials
// are never returned or logged. No customer messages are sent.
func Validate(ctx context.Context, client *http.Client, provider, model, key string) string {
	if key == "" {
		return "ai_key_required"
	}
	if model == "" {
		return "ai_model_required"
	}
	var endpoint string
	var payload any
	switch provider {
	case "google":
		endpoint = "https://generativelanguage.googleapis.com/v1beta/models/" + url.PathEscape(model) + ":generateContent"
		payload = map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": "Reply OK."}}}}, "generationConfig": map[string]any{"maxOutputTokens": 32}}
	case "openai":
		endpoint = "https://api.openai.com/v1/chat/completions"
		payload = map[string]any{"model": model, "messages": []any{map[string]string{"role": "user", "content": "Reply OK."}}, "max_tokens": 32}
	case "anthropic":
		endpoint = "https://api.anthropic.com/v1/messages"
		payload = map[string]any{"model": model, "messages": []any{map[string]string{"role": "user", "content": "Reply OK."}}, "max_tokens": 32}
	default:
		return "ai_provider_invalid"
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "ai_request_failed"
	}
	req.Header.Set("Content-Type", "application/json")
	switch provider {
	case "google":
		req.Header.Set("x-goog-api-key", key)
	case "openai":
		req.Header.Set("Authorization", "Bearer "+key)
	case "anthropic":
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	if client == nil {
		client = http.DefaultClient
	}
	// Never forward credentials to a redirected host.
	validationClient := *client
	validationClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := validationClient.Do(req)
	if err != nil {
		return "ai_unreachable"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return ""
	}
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return "ai_key_invalid"
	case http.StatusForbidden:
		return "ai_access_denied"
	case http.StatusNotFound:
		return "ai_model_unavailable"
	case http.StatusTooManyRequests:
		return "ai_quota_exceeded"
	case http.StatusBadRequest:
		text := strings.ToLower(string(responseBody))
		if strings.Contains(text, "api_key_invalid") || strings.Contains(text, "api key not valid") || strings.Contains(text, "invalid_api_key") {
			return "ai_key_invalid"
		}
		return "ai_request_failed"
	default:
		return "ai_provider_unavailable"
	}
}
