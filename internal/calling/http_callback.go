package calling

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPCallbackResult holds the response from an HTTP callback.
type HTTPCallbackResult struct {
	StatusCode int
	Body       string
}

// executeHTTPCallback performs an HTTP request with configurable method, headers, and body.
// The URL is admin-configured via the IVR flow editor (not end-user input).
func executeHTTPCallback(url, method string, headers map[string]string, body string, timeout time.Duration) (*HTTPCallbackResult, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // limit to 64KB
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &HTTPCallbackResult{
		StatusCode: resp.StatusCode,
		Body:       string(respBody),
	}, nil
}

// interpolateTemplate replaces {{key}} placeholders with values from the variables map.
func interpolateTemplate(tpl string, vars map[string]string) string {
	for k, v := range vars {
		tpl = strings.ReplaceAll(tpl, "{{"+k+"}}", v)
	}
	return tpl
}
