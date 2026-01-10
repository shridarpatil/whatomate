package whatsapp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_SendInteractiveButtons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		phone           string
		bodyText        string
		buttons         []whatsapp.Button
		wantInteractive string // "button" or "list"
		wantErr         bool
		wantErrContains string
	}{
		{
			name:     "3 buttons uses button format",
			phone:    "1234567890",
			bodyText: "Choose an option:",
			buttons: []whatsapp.Button{
				{ID: "1", Title: "Option 1"},
				{ID: "2", Title: "Option 2"},
				{ID: "3", Title: "Option 3"},
			},
			wantInteractive: "button",
			wantErr:         false,
		},
		{
			name:     "4 buttons uses list format",
			phone:    "1234567890",
			bodyText: "Choose an option:",
			buttons: []whatsapp.Button{
				{ID: "1", Title: "Option 1"},
				{ID: "2", Title: "Option 2"},
				{ID: "3", Title: "Option 3"},
				{ID: "4", Title: "Option 4"},
			},
			wantInteractive: "list",
			wantErr:         false,
		},
		{
			name:     "10 buttons uses list format",
			phone:    "1234567890",
			bodyText: "Choose an option:",
			buttons: func() []whatsapp.Button {
				buttons := make([]whatsapp.Button, 10)
				for i := range buttons {
					buttons[i] = whatsapp.Button{ID: string(rune('a' + i)), Title: "Option"}
				}
				return buttons
			}(),
			wantInteractive: "list",
			wantErr:         false,
		},
		{
			name:            "empty buttons returns error",
			phone:           "1234567890",
			bodyText:        "Choose:",
			buttons:         []whatsapp.Button{},
			wantErr:         true,
			wantErrContains: "at least one button",
		},
		{
			name:     "more than 10 buttons returns error",
			phone:    "1234567890",
			bodyText: "Choose:",
			buttons: func() []whatsapp.Button {
				buttons := make([]whatsapp.Button, 11)
				for i := range buttons {
					buttons[i] = whatsapp.Button{ID: string(rune('a' + i)), Title: "Option"}
				}
				return buttons
			}(),
			wantErr:         true,
			wantErrContains: "maximum 10 buttons",
		},
		{
			name:     "button title truncated to 20 chars",
			phone:    "1234567890",
			bodyText: "Choose:",
			buttons: []whatsapp.Button{
				{ID: "1", Title: "This is a very long button title that exceeds 20 characters"},
			},
			wantInteractive: "button",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedBody map[string]interface{}
			var serverCalled bool

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serverCalled = true
				_ = json.NewDecoder(r.Body).Decode(&capturedBody)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"messages": []map[string]string{{"id": "wamid.test"}},
				})
			}))
			defer server.Close()

			log := testutil.NopLogger()
			client := whatsapp.NewWithTimeout(log, 5*time.Second)
			client.HTTPClient = &http.Client{
				Transport: &testServerTransport{serverURL: server.URL},
			}

			account := &whatsapp.Account{
				PhoneID:     "123456789",
				BusinessID:  "987654321",
				APIVersion:  "v21.0",
				AccessToken: "test-token",
			}
			ctx := testutil.TestContext(t)

			_, err := client.SendInteractiveButtons(ctx, account, tt.phone, tt.bodyText, tt.buttons)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			require.True(t, serverCalled, "server should have been called")

			// Verify interactive type
			interactive := capturedBody["interactive"].(map[string]interface{})
			assert.Equal(t, tt.wantInteractive, interactive["type"])
		})
	}
}

func TestClient_SendInteractiveButtons_ButtonTruncation(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"messages": []map[string]string{{"id": "wamid.test"}},
		})
	}))
	defer server.Close()

	log := testutil.NopLogger()
	client := whatsapp.NewWithTimeout(log, 5*time.Second)
	client.HTTPClient = &http.Client{
		Transport: &testServerTransport{serverURL: server.URL},
	}

	account := &whatsapp.Account{
		PhoneID:     "123456789",
		BusinessID:  "987654321",
		APIVersion:  "v21.0",
		AccessToken: "test-token",
	}
	ctx := testutil.TestContext(t)

	longTitle := "This title is definitely longer than 20 characters"
	buttons := []whatsapp.Button{
		{ID: "1", Title: longTitle},
	}

	_, err := client.SendInteractiveButtons(ctx, account, "1234567890", "Choose:", buttons)
	require.NoError(t, err)

	// Verify button title was truncated
	interactive := capturedBody["interactive"].(map[string]interface{})
	action := interactive["action"].(map[string]interface{})
	buttonsList := action["buttons"].([]interface{})
	button := buttonsList[0].(map[string]interface{})
	reply := button["reply"].(map[string]interface{})

	// Should be truncated to 20 chars
	assert.Len(t, reply["title"], 20)
}

func TestClient_SendTemplateMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		phone        string
		templateName string
		language     string
		bodyParams   []string
		wantErr      bool
	}{
		{
			name:         "template without params",
			phone:        "1234567890",
			templateName: "hello_world",
			language:     "en",
			bodyParams:   nil,
			wantErr:      false,
		},
		{
			name:         "template with body params",
			phone:        "1234567890",
			templateName: "order_confirmation",
			language:     "en",
			bodyParams:   []string{"John", "12345", "$99.99"},
			wantErr:      false,
		},
		{
			name:         "template with different language",
			phone:        "1234567890",
			templateName: "welcome_message",
			language:     "es",
			bodyParams:   []string{"María"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedBody map[string]interface{}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&capturedBody)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"messages": []map[string]string{{"id": "wamid.template123"}},
				})
			}))
			defer server.Close()

			log := testutil.NopLogger()
			client := whatsapp.NewWithTimeout(log, 5*time.Second)
			client.HTTPClient = &http.Client{
				Transport: &testServerTransport{serverURL: server.URL},
			}

			account := &whatsapp.Account{
				PhoneID:     "123456789",
				BusinessID:  "987654321",
				APIVersion:  "v21.0",
				AccessToken: "test-token",
			}
			ctx := testutil.TestContext(t)

			msgID, err := client.SendTemplateMessage(ctx, account, tt.phone, tt.templateName, tt.language, tt.bodyParams)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "wamid.template123", msgID)

			// Verify request body
			assert.Equal(t, "template", capturedBody["type"])
			assert.Equal(t, tt.phone, capturedBody["to"])

			template := capturedBody["template"].(map[string]interface{})
			assert.Equal(t, tt.templateName, template["name"])

			language := template["language"].(map[string]interface{})
			assert.Equal(t, tt.language, language["code"])

			// If params were provided, verify components
			if len(tt.bodyParams) > 0 {
				components := template["components"].([]interface{})
				assert.Len(t, components, 1)

				bodyComponent := components[0].(map[string]interface{})
				assert.Equal(t, "body", bodyComponent["type"])

				params := bodyComponent["parameters"].([]interface{})
				assert.Len(t, params, len(tt.bodyParams))

				for i, p := range tt.bodyParams {
					param := params[i].(map[string]interface{})
					assert.Equal(t, "text", param["type"])
					assert.Equal(t, p, param["text"])
				}
			}
		})
	}
}

func TestClient_SendCTAURLButton(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		phone           string
		bodyText        string
		buttonText      string
		url             string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:       "valid CTA button",
			phone:      "1234567890",
			bodyText:   "Click to visit our website",
			buttonText: "Visit Now",
			url:        "https://example.com",
			wantErr:    false,
		},
		{
			name:            "empty button text",
			phone:           "1234567890",
			bodyText:        "Click here",
			buttonText:      "",
			url:             "https://example.com",
			wantErr:         true,
			wantErrContains: "button text and URL are required",
		},
		{
			name:            "empty URL",
			phone:           "1234567890",
			bodyText:        "Click here",
			buttonText:      "Click",
			url:             "",
			wantErr:         true,
			wantErrContains: "button text and URL are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedBody map[string]interface{}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&capturedBody)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"messages": []map[string]string{{"id": "wamid.cta123"}},
				})
			}))
			defer server.Close()

			log := testutil.NopLogger()
			client := whatsapp.NewWithTimeout(log, 5*time.Second)
			client.HTTPClient = &http.Client{
				Transport: &testServerTransport{serverURL: server.URL},
			}

			account := &whatsapp.Account{
				PhoneID:     "123456789",
				BusinessID:  "987654321",
				APIVersion:  "v21.0",
				AccessToken: "test-token",
			}
			ctx := testutil.TestContext(t)

			msgID, err := client.SendCTAURLButton(ctx, account, tt.phone, tt.bodyText, tt.buttonText, tt.url)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "wamid.cta123", msgID)

			// Verify request body
			interactive := capturedBody["interactive"].(map[string]interface{})
			assert.Equal(t, "cta_url", interactive["type"])

			action := interactive["action"].(map[string]interface{})
			params := action["parameters"].(map[string]interface{})
			assert.Equal(t, tt.url, params["url"])
		})
	}
}

func TestClient_SendTemplateMessageWithComponents(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"messages": []map[string]string{{"id": "wamid.comp123"}},
		})
	}))
	defer server.Close()

	log := testutil.NopLogger()
	client := whatsapp.NewWithTimeout(log, 5*time.Second)
	client.HTTPClient = &http.Client{
		Transport: &testServerTransport{serverURL: server.URL},
	}

	account := &whatsapp.Account{
		PhoneID:     "123456789",
		BusinessID:  "987654321",
		APIVersion:  "v21.0",
		AccessToken: "test-token",
	}
	ctx := testutil.TestContext(t)

	// Test with header and body components
	components := []map[string]interface{}{
		{
			"type": "header",
			"parameters": []map[string]interface{}{
				{"type": "image", "image": map[string]string{"link": "https://example.com/image.jpg"}},
			},
		},
		{
			"type": "body",
			"parameters": []map[string]interface{}{
				{"type": "text", "text": "John Doe"},
				{"type": "text", "text": "Order #12345"},
			},
		},
	}

	msgID, err := client.SendTemplateMessageWithComponents(ctx, account, "1234567890", "order_template", "en", components)

	require.NoError(t, err)
	assert.Equal(t, "wamid.comp123", msgID)

	// Verify components were passed correctly
	template := capturedBody["template"].(map[string]interface{})
	sentComponents := template["components"].([]interface{})
	assert.Len(t, sentComponents, 2)
}

