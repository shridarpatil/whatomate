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

			_, err := client.SendInteractiveButtons(ctx, account, whatsapp.Recipient{Phone: tt.phone}, tt.bodyText, tt.buttons)

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

	_, err := client.SendInteractiveButtons(ctx, account, whatsapp.Recipient{Phone: "1234567890"}, "Choose:", buttons)
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
		bodyParams   map[string]string
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
			bodyParams:   map[string]string{"1": "John", "2": "12345", "3": "$99.99"},
			wantErr:      false,
		},
		{
			name:         "template with different language",
			phone:        "1234567890",
			templateName: "welcome_message",
			language:     "es",
			bodyParams:   map[string]string{"1": "María"},
			wantErr:      false,
		},
		{
			name:         "template with named params",
			phone:        "1234567890",
			templateName: "named_template",
			language:     "en",
			bodyParams:   map[string]string{"name": "John", "order_id": "12345"},
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

			components := whatsapp.BodyParamsToComponents(tt.bodyParams)
			msgID, err := client.SendTemplateMessage(ctx, account, whatsapp.Recipient{Phone: tt.phone}, tt.templateName, tt.language, components)

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

				// Verify each param has type "text" and a text value
				for _, p := range params {
					param := p.(map[string]interface{})
					assert.Equal(t, "text", param["type"])
					assert.NotEmpty(t, param["text"])
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

			msgID, err := client.SendCTAURLButton(ctx, account, whatsapp.Recipient{Phone: tt.phone}, tt.bodyText, tt.buttonText, tt.url)

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

func TestClient_SendTemplateMessage_WithComponents(t *testing.T) {
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

	msgID, err := client.SendTemplateMessage(ctx, account, whatsapp.Recipient{Phone: "1234567890"}, "order_template", "en", components)

	require.NoError(t, err)
	assert.Equal(t, "wamid.comp123", msgID)

	// Verify components were passed correctly
	template := capturedBody["template"].(map[string]interface{})
	sentComponents := template["components"].([]interface{})
	assert.Len(t, sentComponents, 2)
}

func TestButtonURLParamsToComponents(t *testing.T) {
	t.Parallel()

	t.Run("nil params returns nil", func(t *testing.T) {
		result := whatsapp.ButtonURLParamsToComponents(nil)
		assert.Nil(t, result)
	})

	t.Run("empty params returns nil", func(t *testing.T) {
		result := whatsapp.ButtonURLParamsToComponents(map[string]string{})
		assert.Nil(t, result)
	})

	t.Run("single URL button param", func(t *testing.T) {
		params := map[string]string{"0": "12345"}
		result := whatsapp.ButtonURLParamsToComponents(params)

		require.Len(t, result, 1)
		assert.Equal(t, "button", result[0]["type"])
		assert.Equal(t, "url", result[0]["sub_type"])
		assert.Equal(t, "0", result[0]["index"])

		parameters := result[0]["parameters"].([]map[string]interface{})
		require.Len(t, parameters, 1)
		assert.Equal(t, "text", parameters[0]["type"])
		assert.Equal(t, "12345", parameters[0]["text"])
	})

	t.Run("multiple URL button params sorted by index", func(t *testing.T) {
		params := map[string]string{"1": "xyz", "0": "abc"}
		result := whatsapp.ButtonURLParamsToComponents(params)

		require.Len(t, result, 2)
		assert.Equal(t, "0", result[0]["index"])
		assert.Equal(t, "1", result[1]["index"])
	})

	t.Run("COPY_CODE button from template metadata", func(t *testing.T) {
		params := map[string]string{"0": "WELCOME10"}
		templateButtons := []interface{}{
			map[string]interface{}{"type": "COPY_CODE", "text": "Copy Code"},
		}
		result := whatsapp.ButtonURLParamsToComponents(params, templateButtons)

		require.Len(t, result, 1)
		assert.Equal(t, "button", result[0]["type"])
		assert.Equal(t, "copy_code", result[0]["sub_type"])
		assert.Equal(t, "0", result[0]["index"])

		parameters := result[0]["parameters"].([]map[string]interface{})
		require.Len(t, parameters, 1)
		assert.Equal(t, "coupon_code", parameters[0]["type"])
		assert.Equal(t, "WELCOME10", parameters[0]["coupon_code"])
	})

	t.Run("mixed URL and COPY_CODE buttons", func(t *testing.T) {
		params := map[string]string{"0": "track123", "1": "SAVE20"}
		templateButtons := []interface{}{
			map[string]interface{}{"type": "URL", "text": "Track", "url": "https://example.com/{{1}}"},
			map[string]interface{}{"type": "COPY_CODE", "text": "Copy Code"},
		}
		result := whatsapp.ButtonURLParamsToComponents(params, templateButtons)

		require.Len(t, result, 2)

		// First: URL button
		assert.Equal(t, "url", result[0]["sub_type"])
		urlParams := result[0]["parameters"].([]map[string]interface{})
		assert.Equal(t, "track123", urlParams[0]["text"])

		// Second: COPY_CODE button
		assert.Equal(t, "copy_code", result[1]["sub_type"])
		codeParams := result[1]["parameters"].([]map[string]interface{})
		assert.Equal(t, "SAVE20", codeParams[0]["coupon_code"])
	})

	t.Run("case insensitive button type matching", func(t *testing.T) {
		params := map[string]string{"0": "CODE1"}
		templateButtons := []interface{}{
			map[string]interface{}{"type": "copy_code", "text": "Copy"},
		}
		result := whatsapp.ButtonURLParamsToComponents(params, templateButtons)

		require.Len(t, result, 1)
		assert.Equal(t, "copy_code", result[0]["sub_type"])
	})

	t.Run("no template buttons defaults to URL", func(t *testing.T) {
		params := map[string]string{"0": "value"}
		result := whatsapp.ButtonURLParamsToComponents(params)

		require.Len(t, result, 1)
		assert.Equal(t, "url", result[0]["sub_type"])
	})
}

