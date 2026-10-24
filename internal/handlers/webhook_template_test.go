package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedTime = time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

func TestRenderWebhookBody_EmptyTemplateUsesEnvelope(t *testing.T) {
	t.Parallel()

	data := MessageEventData{
		ContactName: "Jane",
		Content:     "hi",
	}
	body, err := renderWebhookBody(models.Webhook{}, "message.incoming", data, fixedTime)
	require.NoError(t, err)

	var env OutboundWebhookPayload
	require.NoError(t, json.Unmarshal(body, &env))
	assert.Equal(t, "message.incoming", env.Event)
	assert.Equal(t, fixedTime, env.Timestamp.UTC())

	// Data survives round-trip as the nested object
	dataMap, ok := env.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Jane", dataMap["contact_name"])
	assert.Equal(t, "hi", dataMap["content"])
}

func TestRenderWebhookBody_SlackTemplateProducesValidJSON(t *testing.T) {
	t.Parallel()

	wh := models.Webhook{
		BodyTemplate: `{"text": {{ json (printf "%s: %s" .Data.contact_name .Data.content) }}}`,
	}
	data := MessageEventData{
		ContactName: "Jane",
		Content:     "hello",
	}
	body, err := renderWebhookBody(wh, "message.incoming", data, fixedTime)
	require.NoError(t, err)

	var slack map[string]any
	require.NoError(t, json.Unmarshal(body, &slack), "rendered body must be valid JSON: %s", body)
	assert.Equal(t, "Jane: hello", slack["text"])
}

func TestRenderWebhookBody_JSONFuncEscapesSpecialChars(t *testing.T) {
	t.Parallel()

	wh := models.Webhook{BodyTemplate: `{"text": {{ json .Data.content }}}`}
	// Content with quotes and a newline would break naive interpolation.
	data := MessageEventData{Content: "he said \"hi\"\nbye"}

	body, err := renderWebhookBody(wh, "message.incoming", data, fixedTime)
	require.NoError(t, err)

	var slack map[string]any
	require.NoError(t, json.Unmarshal(body, &slack), "body must stay valid JSON: %s", body)
	assert.Equal(t, "he said \"hi\"\nbye", slack["text"])
}

func TestRenderWebhookBody_ExposesEventAndTimestamp(t *testing.T) {
	t.Parallel()

	wh := models.Webhook{BodyTemplate: `{"event": {{ json .Event }}, "ts": {{ json .Timestamp }}}`}
	body, err := renderWebhookBody(wh, "transfer.created", nil, fixedTime)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "transfer.created", out["event"])
	assert.Equal(t, fixedTime.Format(time.RFC3339), out["ts"])
}

func TestValidateWebhookBodyTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"whitespace only is valid", "   \n", false},
		{"valid template", `{"text": {{ json .Data.content }}}`, false},
		{"unclosed action", `{"text": {{ .Data.content }`, true},
		{"unknown function", `{{ nope .Data.content }}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateWebhookBodyTemplate(tt.body)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDataToMap(t *testing.T) {
	t.Parallel()

	t.Run("nil returns empty map", func(t *testing.T) {
		t.Parallel()
		m, err := dataToMap(nil)
		require.NoError(t, err)
		assert.Empty(t, m)
	})

	t.Run("struct is keyed by json tags", func(t *testing.T) {
		t.Parallel()
		m, err := dataToMap(MessageEventData{ContactName: "Jane", ContactPhone: "+1"})
		require.NoError(t, err)
		assert.Equal(t, "Jane", m["contact_name"])
		assert.Equal(t, "+1", m["contact_phone"])
	})

	t.Run("map passes through", func(t *testing.T) {
		t.Parallel()
		in := map[string]any{"foo": "bar"}
		m, err := dataToMap(in)
		require.NoError(t, err)
		assert.Equal(t, "bar", m["foo"])
	})
}
