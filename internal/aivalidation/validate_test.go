package aivalidation

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name, provider, model, key string
		status                     int
		body, want                 string
	}{
		{"google", "google", "gemini-test", "secret", 200, `{}`, ""},
		{"openai", "openai", "gpt-test", "secret", 200, `{}`, ""},
		{"anthropic", "anthropic", "claude-test", "secret", 200, `{}`, ""},
		{"google invalid", "google", "model", "secret", 400, `{"error":{"message":"API key not valid. secret"}}`, "ai_key_invalid"},
		{"unauthorized", "openai", "model", "secret", 401, "secret", "ai_key_invalid"},
		{"permissions", "google", "model", "secret", 403, "secret", "ai_access_denied"},
		{"model", "google", "model", "secret", 404, "secret", "ai_model_unavailable"},
		{"quota", "google", "model", "secret", 429, "secret", "ai_quota_exceeded"},
		{"bad request", "google", "model", "secret", 400, "secret", "ai_request_failed"},
		{"outage", "google", "model", "secret", 503, "secret", "ai_provider_unavailable"},
		{"network", "google", "model", "secret", 0, "", "ai_unreachable"},
		{"key required", "google", "model", "", 0, "", "ai_key_required"},
		{"model required", "google", "", "secret", 0, "", "ai_model_required"},
		{"provider", "unknown", "model", "secret", 0, "", "ai_provider_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if strings.Contains(r.URL.String(), "secret") {
					t.Fatal("key leaked in URL")
				}
				if _, ok := r.Context().Deadline(); !ok {
					t.Fatal("missing deadline")
				}
				header, want := "x-goog-api-key", "secret"
				if tc.provider == "openai" {
					header, want = "Authorization", "Bearer secret"
				}
				if tc.provider == "anthropic" {
					header = "x-api-key"
				}
				if r.Header.Get(header) != want {
					t.Fatal("missing credentials")
				}
				if tc.status == 0 {
					return nil, errors.New("network secret")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			if got := Validate(context.Background(), client, tc.provider, tc.model, tc.key); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
