package whatsapp_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- VerifyWebhook ---

func TestVerifyWebhook_ValidChallenge(t *testing.T) {
	t.Parallel()
	challenge, err := whatsapp.VerifyWebhook("subscribe", "my-token", "challenge-123", "my-token")
	require.NoError(t, err)
	assert.Equal(t, "challenge-123", challenge)
}

func TestVerifyWebhook_WrongToken(t *testing.T) {
	t.Parallel()
	_, err := whatsapp.VerifyWebhook("subscribe", "wrong-token", "challenge-123", "my-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token mismatch")
}

func TestVerifyWebhook_WrongMode(t *testing.T) {
	t.Parallel()
	_, err := whatsapp.VerifyWebhook("unsubscribe", "my-token", "challenge-123", "my-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mode")
}

func TestVerifyWebhook_EmptyMode(t *testing.T) {
	t.Parallel()
	_, err := whatsapp.VerifyWebhook("", "my-token", "challenge-123", "my-token")
	require.Error(t, err)
}

// --- ParseWebhook ---

func TestParseWebhook_ValidPayload(t *testing.T) {
	t.Parallel()
	body := []byte(`{
		"object": "whatsapp_business_account",
		"entry": [{
			"id": "123",
			"changes": [{
				"value": {
					"messaging_product": "whatsapp",
					"metadata": {
						"display_phone_number": "15551234567",
						"phone_number_id": "phone-123"
					},
					"messages": [{
						"from": "15559876543",
						"id": "wamid.abc123",
						"timestamp": "1700000000",
						"type": "text",
						"text": {"body": "Hello"}
					}]
				},
				"field": "messages"
			}]
		}]
	}`)

	payload, err := whatsapp.ParseWebhook(body)
	require.NoError(t, err)
	assert.Equal(t, "whatsapp_business_account", payload.Object)
	assert.Len(t, payload.Entry, 1)
	assert.Len(t, payload.Entry[0].Changes, 1)
}

func TestParseWebhook_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := whatsapp.ParseWebhook([]byte(`{invalid json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestParseWebhook_EmptyBody(t *testing.T) {
	t.Parallel()
	// Empty JSON object is valid
	payload, err := whatsapp.ParseWebhook([]byte(`{}`))
	require.NoError(t, err)
	assert.Empty(t, payload.Entry)
}

// --- ExtractMessages ---

func TestExtractMessages_TextMessage(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
					Contacts: []whatsapp.WebhookContact{{
						Profile: struct {
							Name string `json:"name"`
						}{Name: "Test User"},
					}},
					Messages: []whatsapp.WebhookMessage{{
						From:      "15559876543",
						ID:        "wamid.abc123",
						Timestamp: "1700000000",
						Type:      "text",
						Text:      &whatsapp.WebhookText{Body: "Hello World"},
					}},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "15559876543", messages[0].From)
	assert.Equal(t, "wamid.abc123", messages[0].ID)
	assert.Equal(t, "text", messages[0].Type)
	assert.Equal(t, "Hello World", messages[0].Text)
	assert.Equal(t, "phone-123", messages[0].PhoneNumberID)
	assert.Equal(t, "Test User", messages[0].ContactName)
	assert.False(t, messages[0].Timestamp.IsZero())
}

func TestExtractMessages_ImageMessage(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
					Messages: []whatsapp.WebhookMessage{{
						From:      "15559876543",
						ID:        "wamid.img123",
						Timestamp: "1700000000",
						Type:      "image",
						Image:     &whatsapp.WebhookMedia{ID: "media-123", MimeType: "image/jpeg", Caption: "Check this"},
					}},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "image", messages[0].Type)
	assert.Equal(t, "media-123", messages[0].MediaID)
	assert.Equal(t, "image/jpeg", messages[0].MediaMimeType)
	assert.Equal(t, "Check this", messages[0].Caption)
}

func TestExtractMessages_DocumentMessage(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
					Messages: []whatsapp.WebhookMessage{{
						From:      "15559876543",
						ID:        "wamid.doc123",
						Timestamp: "1700000000",
						Type:      "document",
						Document:  &whatsapp.WebhookMedia{ID: "doc-media-123", MimeType: "application/pdf", Caption: "Report"},
					}},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "document", messages[0].Type)
	assert.Equal(t, "doc-media-123", messages[0].MediaID)
	assert.Equal(t, "application/pdf", messages[0].MediaMimeType)
	assert.Equal(t, "Report", messages[0].Caption)
}

func TestExtractMessages_AudioMessage(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
					Messages: []whatsapp.WebhookMessage{{
						From:      "15559876543",
						ID:        "wamid.aud123",
						Timestamp: "1700000000",
						Type:      "audio",
						Audio:     &whatsapp.WebhookMedia{ID: "audio-media-123", MimeType: "audio/ogg"},
					}},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "audio", messages[0].Type)
	assert.Equal(t, "audio-media-123", messages[0].MediaID)
	assert.Equal(t, "audio/ogg", messages[0].MediaMimeType)
}

func TestExtractMessages_VideoMessage(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
					Messages: []whatsapp.WebhookMessage{{
						From:      "15559876543",
						ID:        "wamid.vid123",
						Timestamp: "1700000000",
						Type:      "video",
						Video:     &whatsapp.WebhookMedia{ID: "vid-media-123", MimeType: "video/mp4", Caption: "Watch this"},
					}},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "video", messages[0].Type)
	assert.Equal(t, "vid-media-123", messages[0].MediaID)
	assert.Equal(t, "video/mp4", messages[0].MediaMimeType)
	assert.Equal(t, "Watch this", messages[0].Caption)
}

func TestExtractMessages_ButtonReply(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
					Messages: []whatsapp.WebhookMessage{{
						From:      "15559876543",
						ID:        "wamid.btn123",
						Timestamp: "1700000000",
						Type:      "interactive",
						Interactive: &whatsapp.WebhookInteractive{
							Type: "button_reply",
							ButtonReply: &whatsapp.WebhookButtonReply{
								ID:    "btn_yes",
								Title: "Yes",
							},
						},
					}},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "interactive", messages[0].Type)
	assert.Equal(t, "btn_yes", messages[0].ButtonReplyID)
	assert.Equal(t, "Yes", messages[0].Text)
}

func TestExtractMessages_ListReply(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
					Messages: []whatsapp.WebhookMessage{{
						From:      "15559876543",
						ID:        "wamid.list123",
						Timestamp: "1700000000",
						Type:      "interactive",
						Interactive: &whatsapp.WebhookInteractive{
							Type: "list_reply",
							ListReply: &whatsapp.WebhookListReply{
								ID:    "option_1",
								Title: "Option 1",
							},
						},
					}},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "option_1", messages[0].ListReplyID)
	assert.Equal(t, "Option 1", messages[0].Text)
}

func TestExtractMessages_NFMReply(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
					Messages: []whatsapp.WebhookMessage{{
						From:      "15559876543",
						ID:        "wamid.nfm123",
						Timestamp: "1700000000",
						Type:      "interactive",
						Interactive: &whatsapp.WebhookInteractive{
							Type: "nfm_reply",
							NFMReply: &whatsapp.WebhookNFMReply{
								Body: "Flow completed",
								Name: "test_flow",
							},
						},
					}},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "Flow completed", messages[0].Text)
}

func TestExtractMessages_NoMessages(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	assert.Empty(t, messages)
}

func TestExtractMessages_NonMessageField(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "account_update",
				Value: whatsapp.WebhookValue{
					Messages: []whatsapp.WebhookMessage{{
						From: "15559876543",
						Type: "text",
						Text: &whatsapp.WebhookText{Body: "Should be ignored"},
					}},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	assert.Empty(t, messages)
}

func TestExtractMessages_MultipleMessages(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Field: "messages",
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-123"},
					Messages: []whatsapp.WebhookMessage{
						{From: "111", ID: "msg1", Type: "text", Text: &whatsapp.WebhookText{Body: "First"}},
						{From: "222", ID: "msg2", Type: "text", Text: &whatsapp.WebhookText{Body: "Second"}},
					},
				},
			}},
		}},
	}

	messages := payload.ExtractMessages()
	require.Len(t, messages, 2)
	assert.Equal(t, "First", messages[0].Text)
	assert.Equal(t, "Second", messages[1].Text)
}

// --- ExtractStatuses ---

func TestExtractStatuses_Sent(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{
					Statuses: []whatsapp.WebhookStatus{{
						ID:          "wamid.abc123",
						Status:      "sent",
						Timestamp:   "1700000000",
						RecipientID: "15559876543",
					}},
				},
			}},
		}},
	}

	statuses := payload.ExtractStatuses()
	require.Len(t, statuses, 1)
	assert.Equal(t, "wamid.abc123", statuses[0].MessageID)
	assert.Equal(t, "sent", statuses[0].Status)
	assert.Equal(t, "15559876543", statuses[0].RecipientID)
	assert.False(t, statuses[0].Timestamp.IsZero())
}

func TestExtractStatuses_Delivered(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{
					Statuses: []whatsapp.WebhookStatus{{
						ID:          "wamid.abc123",
						Status:      "delivered",
						Timestamp:   "1700000000",
						RecipientID: "15559876543",
					}},
				},
			}},
		}},
	}

	statuses := payload.ExtractStatuses()
	require.Len(t, statuses, 1)
	assert.Equal(t, "delivered", statuses[0].Status)
}

func TestExtractStatuses_Read(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{
					Statuses: []whatsapp.WebhookStatus{{
						ID:     "wamid.abc123",
						Status: "read",
					}},
				},
			}},
		}},
	}

	statuses := payload.ExtractStatuses()
	require.Len(t, statuses, 1)
	assert.Equal(t, "read", statuses[0].Status)
}

func TestExtractStatuses_FailedWithError(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{
					Statuses: []whatsapp.WebhookStatus{{
						ID:     "wamid.abc123",
						Status: "failed",
						Errors: []whatsapp.WebhookStatusError{{
							Code:    131047,
							Title:   "Re-engagement message",
							Message: "More than 24 hours have passed",
						}},
					}},
				},
			}},
		}},
	}

	statuses := payload.ExtractStatuses()
	require.Len(t, statuses, 1)
	assert.Equal(t, "failed", statuses[0].Status)
	assert.Equal(t, 131047, statuses[0].ErrorCode)
	assert.Equal(t, "Re-engagement message", statuses[0].ErrorTitle)
	assert.Equal(t, "More than 24 hours have passed", statuses[0].ErrorMsg)
}

func TestExtractStatuses_NoStatuses(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{},
			}},
		}},
	}

	statuses := payload.ExtractStatuses()
	assert.Empty(t, statuses)
}

// --- GetPhoneNumberID ---

func TestGetPhoneNumberID_Present(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{
					Metadata: whatsapp.WebhookMetadata{PhoneNumberID: "phone-456"},
				},
			}},
		}},
	}

	assert.Equal(t, "phone-456", payload.GetPhoneNumberID())
}

func TestGetPhoneNumberID_Missing(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{},
			}},
		}},
	}

	assert.Empty(t, payload.GetPhoneNumberID())
}

func TestGetPhoneNumberID_EmptyPayload(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{}
	assert.Empty(t, payload.GetPhoneNumberID())
}

// --- HasMessages / HasStatuses ---

func TestHasMessages_True(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{
					Messages: []whatsapp.WebhookMessage{{From: "123", Type: "text"}},
				},
			}},
		}},
	}
	assert.True(t, payload.HasMessages())
}

func TestHasMessages_False(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{},
			}},
		}},
	}
	assert.False(t, payload.HasMessages())
}

func TestHasMessages_EmptyPayload(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{}
	assert.False(t, payload.HasMessages())
}

func TestHasStatuses_True(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{
					Statuses: []whatsapp.WebhookStatus{{ID: "msg1", Status: "sent"}},
				},
			}},
		}},
	}
	assert.True(t, payload.HasStatuses())
}

func TestHasStatuses_False(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{
		Entry: []whatsapp.WebhookEntry{{
			Changes: []whatsapp.WebhookChange{{
				Value: whatsapp.WebhookValue{},
			}},
		}},
	}
	assert.False(t, payload.HasStatuses())
}

func TestHasStatuses_EmptyPayload(t *testing.T) {
	t.Parallel()
	payload := &whatsapp.WebhookPayload{}
	assert.False(t, payload.HasStatuses())
}
