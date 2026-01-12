package handlers

import (
	"encoding/json"
	"strings"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// WebhookVerify handles Meta's webhook verification challenge
func (a *App) WebhookVerify(r *fastglue.Request) error {
	mode := string(r.RequestCtx.QueryArgs().Peek("hub.mode"))
	token := string(r.RequestCtx.QueryArgs().Peek("hub.verify_token"))
	challenge := string(r.RequestCtx.QueryArgs().Peek("hub.challenge"))

	if mode != "subscribe" {
		a.Log.Warn("Webhook verification failed - invalid mode", "mode", mode)
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Verification failed", nil, "")
	}

	// First check against global config token
	if token == a.Config.WhatsApp.WebhookVerifyToken && token != "" {
		a.Log.Info("Webhook verified successfully (global token)")
		r.RequestCtx.SetStatusCode(fasthttp.StatusOK)
		r.RequestCtx.SetBodyString(challenge)
		return nil
	}

	// Then check against tokens stored in WhatsApp accounts
	var account models.WhatsAppAccount
	result := a.DB.Where("webhook_verify_token = ?", token).First(&account)
	if result.Error == nil {
		a.Log.Info("Webhook verified successfully (account token)", "account", account.Name)
		r.RequestCtx.SetStatusCode(fasthttp.StatusOK)
		r.RequestCtx.SetBodyString(challenge)
		return nil
	}

	a.Log.Warn("Webhook verification failed - token not found", "token", token)
	return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Verification failed", nil, "")
}

// WebhookStatusError represents an error in a status update
type WebhookStatusError struct {
	Code    int    `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// TemplateStatusUpdate represents a template status update from Meta webhook
type TemplateStatusUpdate struct {
	Event                   string `json:"event"`
	MessageTemplateID       int64  `json:"message_template_id"`
	MessageTemplateName     string `json:"message_template_name"`
	MessageTemplateLanguage string `json:"message_template_language"`
	Reason                  string `json:"reason,omitempty"`
}

// WebhookStatus represents a message status update from Meta
type WebhookStatus struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Timestamp    string `json:"timestamp"`
	RecipientID  string `json:"recipient_id"`
	Conversation *struct {
		ID string `json:"id"`
	} `json:"conversation,omitempty"`
	Pricing *struct {
		Billable     bool   `json:"billable"`
		PricingModel string `json:"pricing_model"`
		Category     string `json:"category"`
	} `json:"pricing,omitempty"`
	Errors []WebhookStatusError `json:"errors,omitempty"`
}

// WebhookPayload represents the incoming webhook from Meta
type WebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				MessagingProduct string `json:"messaging_product"`
				Metadata         struct {
					DisplayPhoneNumber string `json:"display_phone_number"`
					PhoneNumberID      string `json:"phone_number_id"`
				} `json:"metadata"`
				// Template status update fields (when field == "message_template_status_update")
				Event                   string `json:"event,omitempty"`
				MessageTemplateID       int64  `json:"message_template_id,omitempty"`
				MessageTemplateName     string `json:"message_template_name,omitempty"`
				MessageTemplateLanguage string `json:"message_template_language,omitempty"`
				Reason                  string `json:"reason,omitempty"`
				Contacts                []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
					WaID string `json:"wa_id"`
				} `json:"contacts"`
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Type      string `json:"type"`
					Text      *struct {
						Body string `json:"body"`
					} `json:"text,omitempty"`
					Image *struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						SHA256   string `json:"sha256"`
						Caption  string `json:"caption,omitempty"`
					} `json:"image,omitempty"`
					Document *struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						SHA256   string `json:"sha256"`
						Filename string `json:"filename"`
						Caption  string `json:"caption,omitempty"`
					} `json:"document,omitempty"`
					Audio *struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
					} `json:"audio,omitempty"`
					Video *struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						SHA256   string `json:"sha256"`
						Caption  string `json:"caption,omitempty"`
					} `json:"video,omitempty"`
					Interactive *struct {
						Type        string `json:"type"`
						ButtonReply *struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						} `json:"button_reply,omitempty"`
						ListReply *struct {
							ID          string `json:"id"`
							Title       string `json:"title"`
							Description string `json:"description"`
						} `json:"list_reply,omitempty"`
						NFMReply *struct {
							ResponseJSON string `json:"response_json"`
							Body         string `json:"body"`
							Name         string `json:"name"`
						} `json:"nfm_reply,omitempty"`
					} `json:"interactive,omitempty"`
					Reaction *struct {
						MessageID string `json:"message_id"`
						Emoji     string `json:"emoji"`
					} `json:"reaction,omitempty"`
					Location *struct {
						Latitude  float64 `json:"latitude"`
						Longitude float64 `json:"longitude"`
						Name      string  `json:"name,omitempty"`
						Address   string  `json:"address,omitempty"`
					} `json:"location,omitempty"`
					Contacts []struct {
						Name struct {
							FormattedName string `json:"formatted_name"`
							FirstName     string `json:"first_name,omitempty"`
							LastName      string `json:"last_name,omitempty"`
						} `json:"name"`
						Phones []struct {
							Phone string `json:"phone"`
							Type  string `json:"type,omitempty"`
						} `json:"phones,omitempty"`
					} `json:"contacts,omitempty"`
					Context *struct {
						From string `json:"from"`
						ID   string `json:"id"`
					} `json:"context,omitempty"`
				} `json:"messages,omitempty"`
				Statuses []WebhookStatus `json:"statuses,omitempty"`
			} `json:"value"`
			Field string `json:"field"`
		} `json:"changes"`
	} `json:"entry"`
}

// WebhookHandler processes incoming webhook events from Meta
func (a *App) WebhookHandler(r *fastglue.Request) error {
	var payload WebhookPayload
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &payload); err != nil {
		a.Log.Error("Failed to parse webhook payload", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid payload", nil, "")
	}

	// Process each entry
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			// Handle template status updates
			if change.Field == "message_template_status_update" {
				a.Log.Info("Received template status update",
					"event", change.Value.Event,
					"template_name", change.Value.MessageTemplateName,
					"template_language", change.Value.MessageTemplateLanguage,
					"waba_id", entry.ID,
				)
				go a.processTemplateStatusUpdate(entry.ID, change.Value.Event, change.Value.MessageTemplateName, change.Value.MessageTemplateLanguage, change.Value.Reason)
				continue
			}

			if change.Field != "messages" {
				continue
			}

			phoneNumberID := change.Value.Metadata.PhoneNumberID

			// Process messages
			for _, msg := range change.Value.Messages {
				a.Log.Info("Received message",
					"from", msg.From,
					"type", msg.Type,
					"phone_number_id", phoneNumberID,
				)

				// Get contact profile name
				profileName := ""
				for _, contact := range change.Value.Contacts {
					if contact.WaID == msg.From {
						profileName = contact.Profile.Name
						break
					}
				}

				// Process message asynchronously
				go a.processIncomingMessage(phoneNumberID, msg, profileName)
			}

			// Process status updates
			for _, status := range change.Value.Statuses {
				a.Log.Info("Received status update",
					"message_id", status.ID,
					"status", status.Status,
				)

				go a.processStatusUpdate(phoneNumberID, status)
			}
		}
	}

	// Always respond with 200 to acknowledge receipt
	return r.SendEnvelope(map[string]string{"status": "ok"})
}

func (a *App) processIncomingMessage(phoneNumberID string, msg interface{}, profileName string) {
	// Convert msg interface to the message struct
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		a.Log.Error("Failed to marshal message", "error", err)
		return
	}

	var textMsg IncomingTextMessage
	if err := json.Unmarshal(msgBytes, &textMsg); err != nil {
		a.Log.Error("Failed to unmarshal message", "error", err)
		return
	}

	// Check for duplicate message - Meta sometimes sends the same message multiple times
	if textMsg.ID != "" {
		var existingMsg models.Message
		if err := a.DB.Where("whats_app_message_id = ?", textMsg.ID).First(&existingMsg).Error; err == nil {
			a.Log.Debug("Duplicate message detected, skipping", "message_id", textMsg.ID)
			return
		}
	}

	// Process the message with chatbot logic
	a.processIncomingMessageFull(phoneNumberID, textMsg, profileName)
}

func (a *App) processStatusUpdate(phoneNumberID string, status WebhookStatus) {
	messageID := status.ID
	statusValue := status.Status

	a.Log.Info("Processing status update", "message_id", messageID, "status", statusValue, "phone_number_id", phoneNumberID)

	// Update messages table - this also handles campaign stats via incrementCampaignStat
	a.updateMessageStatus(messageID, statusValue, status.Errors)
}

// updateMessageStatus updates the status of a regular message in the messages table
func (a *App) updateMessageStatus(whatsappMsgID, statusValue string, errors []WebhookStatusError) {
	// Find the message by WhatsApp message ID
	var message models.Message
	result := a.DB.Where("whats_app_message_id = ?", whatsappMsgID).First(&message)
	if result.Error != nil {
		a.Log.Debug("No message found for status update", "whats_app_message_id", whatsappMsgID)
		return
	}

	updates := map[string]interface{}{}

	switch models.MessageStatus(statusValue) {
	case models.MessageStatusSent:
		updates["status"] = models.MessageStatusSent
	case models.MessageStatusDelivered:
		updates["status"] = models.MessageStatusDelivered
	case models.MessageStatusRead:
		updates["status"] = models.MessageStatusRead
	case models.MessageStatusFailed:
		updates["status"] = models.MessageStatusFailed
		if len(errors) > 0 {
			updates["error_message"] = errors[0].Message
		}
	default:
		a.Log.Debug("Ignoring message status update", "status", statusValue)
		return
	}

	if err := a.DB.Model(&message).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update message status", "error", err, "message_id", message.ID)
		return
	}

	a.Log.Info("Updated message status", "message_id", message.ID, "status", statusValue)

	// Update campaign stats if this is a campaign message
	if message.Metadata != nil {
		if campaignID, ok := message.Metadata["campaign_id"].(string); ok && campaignID != "" {
			a.incrementCampaignStat(campaignID, statusValue)
		}
	}

	// Broadcast status update via WebSocket
	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(message.OrganizationID, websocket.WSMessage{
			Type: websocket.TypeStatusUpdate,
			Payload: map[string]any{
				"message_id": message.ID.String(),
				"status":     statusValue,
			},
		})
	}
}

// processTemplateStatusUpdate updates template status when Meta sends a status update webhook
func (a *App) processTemplateStatusUpdate(wabaID, event, templateName, templateLanguage, reason string) {
	if templateName == "" {
		a.Log.Warn("Template status update missing template name")
		return
	}

	// Keep status uppercase to match existing template status format
	// Events: APPROVED, REJECTED, PENDING, DISABLED, PENDING_DELETION, DELETED, REINSTATED, FLAGGED
	status := strings.ToUpper(event)

	// Find WhatsApp accounts that use this WABA ID (business_id field)
	var accounts []models.WhatsAppAccount
	if err := a.DB.Where("business_id = ?", wabaID).Find(&accounts).Error; err != nil {
		a.Log.Error("Failed to find WhatsApp accounts for WABA", "error", err, "waba_id", wabaID)
		return
	}

	if len(accounts) == 0 {
		a.Log.Warn("No WhatsApp accounts found for WABA", "waba_id", wabaID)
		return
	}

	// Update template for each account that has it
	for _, account := range accounts {
		// Find and update the template
		result := a.DB.Model(&models.Template{}).
			Where("whats_app_account = ? AND name = ? AND language = ?", account.Name, templateName, templateLanguage).
			Update("status", status)

		if result.Error != nil {
			a.Log.Error("Failed to update template status",
				"error", result.Error,
				"account", account.Name,
				"template", templateName,
				"language", templateLanguage,
			)
			continue
		}

		if result.RowsAffected > 0 {
			a.Log.Info("Updated template status from webhook",
				"account", account.Name,
				"template", templateName,
				"language", templateLanguage,
				"status", status,
				"reason", reason,
			)
		}
	}
}
