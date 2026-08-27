package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateInteractiveButtons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		buttons []InteractiveButton
		wantErr string // substring; empty means the combination must be accepted
	}{
		{
			name:    "no buttons is valid",
			buttons: nil,
		},
		{
			name:    "single reply button",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Yes", Type: "reply"}},
		},
		{
			name:    "single url button with absolute https url",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Docs", Type: "url", URL: "https://example.com"}},
		},
		{
			name:    "http scheme is accepted",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Docs", Type: "url", URL: "http://example.com"}},
		},
		{
			name:    "url without a scheme is rejected",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Docs", Type: "url", URL: "example.com"}},
			wantErr: "absolute http",
		},
		{
			name:    "url with a non-http scheme is rejected",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Mail", Type: "url", URL: "javascript:alert(1)"}},
			wantErr: "absolute http",
		},
		{
			name:    "empty url is rejected",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Docs", Type: "url"}},
			wantErr: "absolute http",
		},
		{
			name: "two url buttons are rejected",
			buttons: []InteractiveButton{
				{ID: "btn_1", Title: "Docs", Type: "url", URL: "https://example.com"},
				{ID: "btn_2", Title: "Blog", Type: "url", URL: "https://example.org"},
			},
			wantErr: "one URL button",
		},
		{
			name: "reply and url buttons cannot mix",
			buttons: []InteractiveButton{
				{ID: "btn_1", Title: "Yes", Type: "reply"},
				{ID: "btn_2", Title: "Docs", Type: "url", URL: "https://example.com"},
			},
			wantErr: "cannot be mixed",
		},
		{
			name:    "phone buttons are unsupported in free-form messages",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Call", Type: "phone", PhoneNumber: "+1234567890"}},
			wantErr: "phone",
		},
		{
			name:    "button without a title is rejected",
			buttons: []InteractiveButton{{ID: "btn_1", Type: "reply"}},
			wantErr: "title",
		},
		{
			name:    "title longer than 20 characters is rejected",
			buttons: []InteractiveButton{{ID: "btn_1", Title: strings.Repeat("a", 21), Type: "reply"}},
			wantErr: "20 characters",
		},
		{
			name:    "title of exactly 20 characters is accepted",
			buttons: []InteractiveButton{{ID: "btn_1", Title: strings.Repeat("a", 20), Type: "reply"}},
		},
		{
			// Meta counts characters, so a 20-rune multi-byte label is within
			// the cap even though it is 60 bytes long.
			name:    "multi-byte title is measured in characters not bytes",
			buttons: []InteractiveButton{{ID: "btn_1", Title: strings.Repeat("न", 20), Type: "reply"}},
		},
		{
			name:    "multi-byte title over the cap is rejected",
			buttons: []InteractiveButton{{ID: "btn_1", Title: strings.Repeat("न", 21), Type: "reply"}},
			wantErr: "20 characters",
		},
		{
			name:    "duplicate button ids are rejected",
			buttons: []InteractiveButton{{ID: "btn_2", Title: "A", Type: "reply"}, {ID: "btn_2", Title: "B", Type: "reply"}},
			wantErr: "unique",
		},
		{
			name:    "eleven reply buttons are rejected",
			buttons: replyButtons(11),
			wantErr: "10 reply buttons",
		},
		{
			name:    "ten reply buttons are accepted",
			buttons: replyButtons(10),
		},
		{
			name: "voice_call cannot be combined with other types",
			buttons: []InteractiveButton{
				{ID: "btn_1", Title: "Call", Type: "voice_call"},
				{ID: "btn_2", Title: "Yes", Type: "reply"},
			},
			wantErr: "cannot be combined",
		},
		{
			name:    "voice_call ttl above 60 is rejected",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Call", Type: "voice_call", TTLMinutes: 61}},
			wantErr: "ttl_minutes",
		},
		{
			name:    "flow button needs a flow id",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Open", Type: "flow"}},
			wantErr: "flow_id",
		},
		{
			name:    "untyped button is treated as a reply button",
			buttons: []InteractiveButton{{ID: "btn_1", Title: "Yes"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateInteractiveButtons(tt.buttons)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func replyButtons(n int) []InteractiveButton {
	out := make([]InteractiveButton, 0, n)
	for i := range n {
		out = append(out, InteractiveButton{
			ID:    "btn_" + string(rune('a'+i)),
			Title: "Option",
			Type:  "reply",
		})
	}
	return out
}

func TestInteractiveButtonsFromMaps(t *testing.T) {
	t.Parallel()

	t.Run("maps the fields the chatbot settings handler receives", func(t *testing.T) {
		t.Parallel()

		got := interactiveButtonsFromMaps([]map[string]any{
			{"id": "btn_1", "title": "Docs", "type": "url", "url": "https://example.com"},
			{"id": "btn_2", "title": "Call", "type": "voice_call", "ttl_minutes": float64(30)},
		})

		require.Len(t, got, 2)
		assert.Equal(t, InteractiveButton{ID: "btn_1", Title: "Docs", Type: "url", URL: "https://example.com"}, got[0])
		assert.Equal(t, InteractiveButton{ID: "btn_2", Title: "Call", Type: "voice_call", TTLMinutes: 30}, got[1])
	})

	t.Run("ignores values of the wrong type instead of panicking", func(t *testing.T) {
		t.Parallel()

		got := interactiveButtonsFromMaps([]map[string]any{{"id": 42, "title": nil, "ttl_minutes": "nope"}})

		require.Len(t, got, 1)
		assert.Equal(t, InteractiveButton{}, got[0])
	})
}
