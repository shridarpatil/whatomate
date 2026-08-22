package handlers

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// maxButtonTitleRunes is Meta's cap on a button label. It counts characters,
// not bytes, so the check below must too.
const maxButtonTitleRunes = 20

// maxReplyButtons is the most reply buttons a single free-form interactive
// message can carry (1-3 send as buttons, 4-10 as a list).
const maxReplyButtons = 10

// InteractiveButton is the button shape shared by every free-form interactive
// message we send — canned responses, chatbot greeting/fallback, flow nodes.
// It exists so the combo rules live in one place instead of being restated per
// caller with slightly different holes.
type InteractiveButton struct {
	ID          string
	Title       string
	Type        string
	URL         string
	PhoneNumber string
	TTLMinutes  int
	FlowID      string
}

// validateInteractiveButtons enforces the combo rules WhatsApp Cloud API
// imposes on free-form interactive messages. We block at save time so the user
// gets a clear error instead of a message that is silently dropped at send.
//
// Sendable shapes:
//   - 0 buttons
//   - 1-10 reply buttons
//   - exactly 1 url button (interactive.type:"cta_url")
//   - exactly 1 voice_call button (standalone)
//   - exactly 1 flow button (standalone)
//
// Phone buttons only exist in approved templates, and multi-URL or mixed
// combos cannot be carried by any free-form interactive message.
//
// The frontend mirrors these checks in validateWhatsAppButtons
// (frontend/src/lib/whatsappButtons.ts); keep the two in sync.
func validateInteractiveButtons(buttons []InteractiveButton) error {
	if len(buttons) == 0 {
		return nil
	}

	var replies, urls, phones, voiceCalls, flows int
	seenIDs := make(map[string]struct{}, len(buttons))

	for _, b := range buttons {
		title := strings.TrimSpace(b.Title)
		if title == "" {
			return fmt.Errorf("every button needs a title")
		}
		if utf8.RuneCountInString(title) > maxButtonTitleRunes {
			return fmt.Errorf("button title %q exceeds %d characters", title, maxButtonTitleRunes)
		}
		if b.ID != "" {
			if _, dup := seenIDs[b.ID]; dup {
				return fmt.Errorf("button ids must be unique, %q is used more than once", b.ID)
			}
			seenIDs[b.ID] = struct{}{}
		}

		switch strings.ToLower(b.Type) {
		case "url":
			urls++
			if !isSendableButtonURL(b.URL) {
				return fmt.Errorf("button %q needs an absolute http(s) URL", title)
			}
		case "phone":
			phones++
		case "voice_call":
			voiceCalls++
			if b.TTLMinutes < 0 || b.TTLMinutes > 60 {
				return fmt.Errorf("voice_call ttl_minutes must be between 0 and 60")
			}
		case "flow":
			flows++
			if strings.TrimSpace(b.FlowID) == "" {
				return fmt.Errorf("flow button needs a flow_id")
			}
		default:
			replies++
		}
	}

	// voice_call and flow each render as the whole interactive message.
	if voiceCalls > 1 {
		return fmt.Errorf("only one voice_call button is allowed per message")
	}
	if flows > 1 {
		return fmt.Errorf("only one flow button is allowed per message")
	}
	if exclusive := voiceCalls + flows; exclusive > 0 && exclusive != len(buttons) {
		return fmt.Errorf("voice_call and flow buttons cannot be combined with other button types")
	}
	if phones > 0 {
		return fmt.Errorf("phone buttons cannot be sent in free-form messages, only in approved templates")
	}
	if urls > 1 {
		return fmt.Errorf("only one URL button is allowed per message")
	}
	if replies > 0 && urls > 0 {
		return fmt.Errorf("reply and URL buttons cannot be mixed in a single message")
	}
	if replies > maxReplyButtons {
		return fmt.Errorf("at most %d reply buttons are allowed per message", maxReplyButtons)
	}
	return nil
}

// isSendableButtonURL reports whether raw is an absolute http(s) URL. Meta
// rejects anything else, and a bare host like "example.com" is the easy
// mistake to make in the UI.
func isSendableButtonURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != ""
}

// interactiveButtonsFromMaps converts the loosely-typed button payload the
// chatbot settings handler accepts into the validated shape. Values of an
// unexpected type are dropped rather than rejected — validation reports the
// resulting empty title or URL with a message aimed at the user.
func interactiveButtonsFromMaps(raw []map[string]any) []InteractiveButton {
	out := make([]InteractiveButton, 0, len(raw))
	for _, m := range raw {
		btn := InteractiveButton{
			ID:          mapString(m, "id"),
			Title:       mapString(m, "title"),
			Type:        mapString(m, "type"),
			URL:         mapString(m, "url"),
			PhoneNumber: mapString(m, "phone_number"),
			FlowID:      mapString(m, "flow_id"),
		}
		// JSON numbers decode as float64.
		if ttl, ok := m["ttl_minutes"].(float64); ok {
			btn.TTLMinutes = int(ttl)
		}
		out = append(out, btn)
	}
	return out
}

func mapString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}
