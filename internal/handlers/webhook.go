package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/contactutil"
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
	Code      int    `json:"code"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	ErrorData struct {
		Details string `json:"details"`
	} `json:"error_data"`
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
						Name     string `json:"name"`
						Username string `json:"username,omitempty"`
					} `json:"profile"`
					WaID   string `json:"wa_id"`
					UserID string `json:"user_id,omitempty"` // BSUID
				} `json:"contacts"`
				Messages        []IncomingTextMessage `json:"messages,omitempty"`
				Statuses        []WebhookStatus       `json:"statuses,omitempty"`
				UserPreferences []struct {
					WaID      string `json:"wa_id"`
					UserID    string `json:"user_id,omitempty"`
					Category  string `json:"category"`
					Value     string `json:"value"`
					Timestamp int64  `json:"timestamp"`
				} `json:"user_preferences,omitempty"`
				Calls []struct {
					ID         string `json:"id"`
					From       string `json:"from"`
					FromUserID string `json:"from_user_id,omitempty"` // BSUID
					To         string `json:"to"`
					ToUserID   string `json:"to_user_id,omitempty"` // BSUID
					Timestamp  string `json:"timestamp"`
					Type       string `json:"type"`
					Event      string `json:"event"`
					Direction  string `json:"direction,omitempty"`
					Session    *struct {
						SDPType string `json:"sdp_type"`
						SDP     string `json:"sdp"`
					} `json:"session,omitempty"`
					Error *struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					} `json:"error,omitempty"`
					// Terminate webhook fields
					Status    json.RawMessage `json:"status,omitempty"`
					StartTime string          `json:"start_time,omitempty"`
					EndTime   string          `json:"end_time,omitempty"`
					Duration  int             `json:"duration,omitempty"`
				} `json:"calls,omitempty"`
				// Contact state sync fields (when field == "smb_app_state_sync")
				Action             string `json:"action,omitempty"`
				ContactName        string `json:"contact_name,omitempty"`
				ContactFirstName   string `json:"contact_first_name,omitempty"`
				ContactPhoneNumber string `json:"contact_phone_number,omitempty"`
				// Message echoes fields (when field == "smb_message_echoes")
				MessageEchoes []IncomingTextMessage `json:"message_echoes,omitempty"`
			} `json:"value"`
			Field string `json:"field"`
		} `json:"changes"`
	} `json:"entry"`
}

// WebhookHandler processes incoming webhook events from Meta
func (a *App) WebhookHandler(r *fastglue.Request) error {
	body := r.RequestCtx.PostBody()
	signature := r.RequestCtx.Request.Header.Peek("X-Hub-Signature-256")

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		a.Log.Error("Failed to parse webhook payload", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid payload", nil, "")
	}

	// Verify webhook signature before processing any fields.
	// Find a phoneNumberID from any change to look up the account's AppSecret.
	if len(signature) > 0 {
		for _, entry := range payload.Entry {
			for _, change := range entry.Changes {
				phoneNumberID := change.Value.Metadata.PhoneNumberID
				if phoneNumberID == "" {
					continue
				}
				account, err := a.getWhatsAppAccountCached(phoneNumberID)
				if err != nil || account.AppSecret == "" {
					continue
				}
				if !verifyWebhookSignature(body, signature, []byte(account.AppSecret)) {
					a.Log.Warn("Invalid webhook signature", "phone_id", phoneNumberID)
					return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Invalid signature", nil, "")
				}
				a.Log.Debug("Webhook signature verified successfully")
				break
			}
		}
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

			// Handle user preferences (marketing opt-out/in)
			if change.Field == "user_preferences" {
				for _, pref := range change.Value.UserPreferences {
					if pref.Category == "marketing_messages" {
						go a.processMarketingPreference(change.Value.Metadata.PhoneNumberID, pref.WaID, pref.UserID, pref.Value)
					}
				}
				continue
			}

			// Handle voice call events (processed sequentially to preserve event order
			// and avoid race conditions between ringing/connect for the same call)
			if change.Field == "calls" {
				phoneNumberID := change.Value.Metadata.PhoneNumberID
				for _, call := range change.Value.Calls {
					a.Log.Info("Received call event",
						"call_id", call.ID,
						"from", call.From,
						"event", call.Event,
						"direction", call.Direction,
						"has_sdp", call.Session != nil && call.Session.SDP != "",
						"phone_number_id", phoneNumberID,
					)
					a.processCallWebhook(phoneNumberID, call)
				}

				// Business-initiated call status webhooks (RINGING/ACCEPTED/REJECTED)
				// arrive in the statuses array under field="calls"
				for _, status := range change.Value.Statuses {
					if status.Status == "" {
						continue
					}
					a.Log.Info("Received call status event",
						"call_id", status.ID,
						"status", status.Status,
					)
					a.processCallStatusWebhook(status)
				}
				continue
			}

			// Handle contact state sync (coexistence)
			if change.Field == "smb_app_state_sync" {
				phoneNumberID := change.Value.Metadata.PhoneNumberID
				a.Log.Info("Received smb_app_state_sync event",
					"phone_number_id", phoneNumberID,
					"action", change.Value.Action,
					"contact_phone_number", change.Value.ContactPhoneNumber,
					"contact_name", change.Value.ContactName,
				)
				go a.processContactSync(phoneNumberID, change.Value.ContactPhoneNumber, change.Value.ContactName, change.Value.Action)
				continue
			}

			// Handle message echoes (coexistence)
			if change.Field == "smb_message_echoes" {
				phoneNumberID := change.Value.Metadata.PhoneNumberID
				for _, echo := range change.Value.MessageEchoes {
					a.Log.Info("Received message echo",
						"from", echo.From,
						"type", echo.Type,
						"phone_number_id", phoneNumberID,
					)
					go a.processMessageEcho(phoneNumberID, echo)
				}
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

				// Handle call permission replies before regular message processing
				if msg.Type == "interactive" && msg.Interactive != nil &&
					msg.Interactive.Type == "call_permission_reply" &&
					msg.Interactive.CallPermissionReply != nil {
					cpr := msg.Interactive.CallPermissionReply
					expTS, err := cpr.ExpirationTimestamp.Int64()
					if err != nil {
						a.Log.Error("Failed to parse call permission expiration timestamp", "error", err, "from", msg.From)
						continue
					}
					go a.processCallPermissionReply(phoneNumberID, msg.From, &CallPermissionReplyData{
						Response:            cpr.Response,
						IsPermanent:         cpr.IsPermanent,
						ExpirationTimestamp: expTS,
						ResponseSource:      cpr.ResponseSource,
					})
					continue
				}

				// Get contact profile name (match by phone or BSUID)
				profileName := ""
				for _, contact := range change.Value.Contacts {
					if (msg.From != "" && contact.WaID == msg.From) || (msg.FromUserID != "" && contact.UserID == msg.FromUserID) {
						profileName = contact.Profile.Name
						break
					}
				}

				// If phone number is missing (username user), skip — BSUID-only messaging not yet supported
				if msg.From == "" {
					a.Log.Warn("Incoming message without phone number (username user), skipping",
						"bsuid", msg.FromUserID, "message_id", msg.ID)
					continue
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

func (a *App) processIncomingMessage(phoneNumberID string, msg IncomingTextMessage, profileName string) {
	defer func() {
		if r := recover(); r != nil {
			a.Log.Error("Panic recovered in processIncomingMessage", "panic", r, "phone_id", phoneNumberID, "message_id", msg.ID)
		}
	}()

	// Check for duplicate message - Meta sometimes sends the same message multiple times
	if msg.ID != "" {
		var existingMsg models.Message
		if err := a.DB.Where("whats_app_message_id = ?", msg.ID).First(&existingMsg).Error; err == nil {
			a.Log.Debug("Duplicate message detected, skipping", "message_id", msg.ID)
			return
		}
	}

	// Process the message with chatbot logic
	a.processIncomingMessageFull(phoneNumberID, msg, profileName)
}

func (a *App) processStatusUpdate(phoneNumberID string, status WebhookStatus) {
	defer func() {
		if r := recover(); r != nil {
			a.Log.Error("Panic recovered in processStatusUpdate", "panic", r, "phone_id", phoneNumberID, "status_id", status.ID)
		}
	}()

	messageID := status.ID
	statusValue := status.Status

	a.Log.Info("Processing status update", "message_id", messageID, "status", statusValue, "phone_number_id", phoneNumberID)

	// Update messages table - this also handles campaign stats via incrementCampaignStat
	a.updateMessageStatus(messageID, statusValue, status.Errors)
}

// statusPriority returns the priority of a status (higher = more progressed)
func statusPriority(status models.MessageStatus) int {
	switch status {
	case models.MessageStatusPending:
		return 0
	case models.MessageStatusSent:
		return 1
	case models.MessageStatusDelivered:
		return 2
	case models.MessageStatusRead:
		return 3
	case models.MessageStatusFailed:
		return 4 // Failed can override any status
	default:
		return -1
	}
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

	newStatus := models.MessageStatus(statusValue)
	currentPriority := statusPriority(message.Status)
	newPriority := statusPriority(newStatus)

	// Only update if new status is a progression (higher priority) or if it's failed
	if newPriority <= currentPriority && newStatus != models.MessageStatusFailed {
		a.Log.Debug("Ignoring status update - not a progression",
			"message_id", message.ID,
			"current_status", message.Status,
			"new_status", statusValue)
		return
	}

	updates := map[string]any{}

	switch newStatus {
	case models.MessageStatusSent:
		updates["status"] = models.MessageStatusSent
	case models.MessageStatusDelivered:
		updates["status"] = models.MessageStatusDelivered
	case models.MessageStatusRead:
		updates["status"] = models.MessageStatusRead
	case models.MessageStatusFailed:
		updates["status"] = models.MessageStatusFailed
		if len(errors) > 0 {
			// Prefer error_data.details (most descriptive), then Message, then Title.
			errText := errors[0].ErrorData.Details
			if errText == "" {
				errText = errors[0].Message
			}
			if errText == "" || errText == errors[0].Title {
				errText = errors[0].Title
			}

			updates["error_message"] = errText
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

	// Update campaign stats and recipient status if this is a campaign message
	if message.Metadata != nil {
		if campaignID, ok := message.Metadata["campaign_id"].(string); ok && campaignID != "" {
			a.incrementCampaignStat(campaignID, statusValue)

			// Update the BulkMessageRecipient status and timestamps
			recipientUpdates := map[string]any{
				"status": newStatus,
			}
			switch newStatus {
			case models.MessageStatusDelivered:
				recipientUpdates["delivered_at"] = time.Now()
			case models.MessageStatusRead:
				recipientUpdates["read_at"] = time.Now()
			case models.MessageStatusFailed:
				if errMsg, ok := updates["error_message"].(string); ok && errMsg != "" {
					recipientUpdates["error_message"] = errMsg
				}
			}
			a.DB.Model(&models.BulkMessageRecipient{}).
				Where("whats_app_message_id = ?", whatsappMsgID).
				Updates(recipientUpdates)
		}
	}

	// Broadcast status update via WebSocket
	if a.WSHub != nil {
		wsPayload := map[string]any{
			"message_id": message.ID.String(),
			"status":     statusValue,
		}
		if errMsg, ok := updates["error_message"].(string); ok && errMsg != "" {
			wsPayload["error_message"] = errMsg
		}
		a.WSHub.BroadcastToOrg(message.OrganizationID, websocket.WSMessage{
			Type:    websocket.TypeStatusUpdate,
			Payload: wsPayload,
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

// verifyWebhookSignature verifies the X-Hub-Signature-256 header from Meta.
// The signature is HMAC-SHA256 of the request body using the App Secret.
func verifyWebhookSignature(body, signature, appSecret []byte) bool {
	// Signature format: "sha256=<hex_signature>"
	prefix := []byte("sha256=")
	if !bytes.HasPrefix(signature, prefix) {
		return false
	}

	expectedSig := bytes.TrimPrefix(signature, prefix)

	// Compute HMAC-SHA256
	mac := hmac.New(sha256.New, appSecret)
	mac.Write(body)
	computedSig := make([]byte, hex.EncodedLen(mac.Size()))
	hex.Encode(computedSig, mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal(expectedSig, computedSig)
}

// processMarketingPreference updates a contact's marketing opt-out status
// based on the user_preferences webhook from Meta.
func (a *App) processMarketingPreference(phoneNumberID, userPhone, bsuid, value string) {
	// Find the WhatsApp account by phone_number_id
	var account models.WhatsAppAccount
	if err := a.DB.Where("phone_id = ?", phoneNumberID).First(&account).Error; err != nil {
		a.Log.Error("Failed to find account for marketing preference", "error", err, "phone_id", phoneNumberID)
		return
	}

	// Find contact by phone number, or by BSUID if phone is empty
	var contact models.Contact
	if userPhone != "" {
		if err := a.DB.Where("phone_number = ? AND organization_id = ?", userPhone, account.OrganizationID).First(&contact).Error; err != nil {
			a.Log.Info("Contact not found for marketing preference", "phone", userPhone)
			return
		}
	} else if bsuid != "" {
		if err := a.DB.Where("bsuid = ? AND organization_id = ?", bsuid, account.OrganizationID).First(&contact).Error; err != nil {
			a.Log.Info("Contact not found by BSUID for marketing preference", "bsuid", bsuid)
			return
		}
	} else {
		a.Log.Warn("Marketing preference webhook with no phone or BSUID, skipping")
		return
	}

	optOut := value == "stop"
	if err := a.DB.Model(&contact).Update("marketing_opt_out", optOut).Error; err != nil {
		a.Log.Error("Failed to update marketing opt-out", "error", err, "contact_id", contact.ID, "opt_out", optOut)
		return
	}

	a.Log.Info("Marketing preference updated",
		"contact_id", contact.ID,
		"phone", userPhone,
		"bsuid", bsuid,
		"opt_out", optOut,
	)
}

// processMessageEcho handles mirroring of messages sent from the mobile WhatsApp Business App.
func (a *App) processMessageEcho(phoneNumberID string, msg IncomingTextMessage) {
	defer func() {
		if r := recover(); r != nil {
			a.Log.Error("Panic recovered in processMessageEcho", "panic", r, "phone_id", phoneNumberID, "message_id", msg.ID)
		}
	}()

	// Find the WhatsApp account by phone_number_id (use cache)
	account, err := a.getWhatsAppAccountCached(phoneNumberID)
	if err != nil {
		a.Log.Error("WhatsApp account not found for echo", "phone_id", phoneNumberID, "error", err)
		return
	}

	// Check for duplicate message - Meta sometimes sends the same message multiple times
	if msg.ID != "" {
		var existingMsg models.Message
		if err := a.DB.Where("whats_app_message_id = ?", msg.ID).First(&existingMsg).Error; err == nil {
			a.Log.Debug("Duplicate echo message detected, skipping", "message_id", msg.ID)
			return
		}
	}

	// Get or create contact (always do this for all echoed messages)
	// For message echoes, the message is sent TO the contact FROM the business.
	contactPhone := msg.To
	if contactPhone == "" {
		a.Log.Warn("Message echo missing 'to' field, falling back to 'from'", "from", msg.From)
		contactPhone = msg.From
	}

	contact, _, err := contactutil.GetOrCreateContact(a.DB, account.OrganizationID, contactPhone, "")
	if err != nil {
		a.Log.Error("Failed to get or create contact for echo", "phone", contactPhone, "error", err)
		return
	}

	// Store BSUID if provided and not already set
	if msg.FromUserID != "" && contact.BSUID != msg.FromUserID {
		a.DB.Model(contact).Update("bsuid", msg.FromUserID)
		contact.BSUID = msg.FromUserID
	}

	// Get message content - handle text and media
	extracted := a.extractMessageContent(context.Background(), msg, account)
	messageText := extracted.Text
	messageType := extracted.Type
	mediaInfo := extracted.Media

	// Save message as outgoing, status sent
	now := time.Now()
	message := models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New()},
		OrganizationID:    account.OrganizationID,
		WhatsAppAccount:   account.Name,
		ContactID:         contact.ID,
		WhatsAppMessageID: msg.ID,
		Direction:         models.DirectionOutgoing,
		MessageType:       models.MessageType(messageType),
		Content:           messageText,
		Status:            models.MessageStatusSent,
	}

	// Reply context
	if msg.Context != nil && msg.Context.ID != "" {
		var replyToMsg models.Message
		if err := a.DB.Where("whats_app_message_id = ?", msg.Context.ID).First(&replyToMsg).Error; err == nil {
			message.IsReply = true
			message.ReplyToMessageID = &replyToMsg.ID
		}
	}

	// Add media fields if present
	if mediaInfo != nil {
		message.MediaURL = mediaInfo.MediaURL
		message.MediaMimeType = mediaInfo.MediaMimeType
		message.MediaFilename = mediaInfo.MediaFilename
	}

	if err := a.DB.Create(&message).Error; err != nil {
		a.Log.Error("Failed to save echoed message", "error", err)
		return
	}

	// Update contact's last message info
	preview := messageText
	if len(preview) > 100 {
		preview = preview[:97] + "..."
	}
	if messageType != "text" && messageType != "button_reply" && messageType != "nfm_reply" {
		preview = "[" + messageType + "]"
	}

	a.DB.Model(contact).Updates(map[string]any{
		"last_message_at":      now,
		"last_message_preview": preview,
		"is_read":              true, // Echoes from mobile app are outgoing, so conversation is read
		"whats_app_account":    account.Name,
	})

	// Broadcast new message via WebSocket to keep UI updated
	a.broadcastNewMessage(account.OrganizationID, &message, contact)

	// Dispatch webhook for outgoing message
	a.DispatchWebhook(account.OrganizationID, models.WebhookEventMessageOutgoing, MessageEventData{
		MessageID:       message.ID.String(),
		ContactID:       contact.ID.String(),
		ContactPhone:    contact.PhoneNumber,
		ContactName:     contact.ProfileName,
		MessageType:     models.MessageType(messageType),
		Content:         messageText,
		WhatsAppAccount: account.Name,
		Direction:       models.DirectionOutgoing,
	})
}

// processContactSync handles contact additions and deletions from the mobile app address book.
func (a *App) processContactSync(phoneNumberID, contactPhone, contactName, action string) {
	defer func() {
		if r := recover(); r != nil {
			a.Log.Error("Panic recovered in processContactSync", "panic", r, "phone_id", phoneNumberID, "phone", contactPhone)
		}
	}()

	// Find the WhatsApp account by phone_number_id (use cache)
	account, err := a.getWhatsAppAccountCached(phoneNumberID)
	if err != nil {
		a.Log.Error("WhatsApp account not found for contact sync", "phone_id", phoneNumberID, "error", err)
		return
	}

	switch action {
	case "add":
		contact, isNewContact, err := contactutil.GetOrCreateContact(a.DB, account.OrganizationID, contactPhone, contactName)
		if err != nil {
			a.Log.Error("Failed to sync new contact from app state sync", "phone", contactPhone, "error", err)
			return
		}

		a.Log.Info("Synced contact (add) from mobile app", "contact_id", contact.ID, "is_new", isNewContact)

		if isNewContact {
			a.DispatchWebhook(account.OrganizationID, models.WebhookEventContactCreated, ContactEventData{
				ContactID:       contact.ID.String(),
				ContactPhone:    contact.PhoneNumber,
				ContactName:     contact.ProfileName,
				WhatsAppAccount: account.Name,
			})
		}
	case "remove":
		// Try to find the contact first using the FindContact helper
		contact, err := contactutil.FindContact(a.DB, account.OrganizationID, contactPhone)
		if err == nil {
			if err := a.DB.Delete(contact).Error; err != nil {
				a.Log.Error("Failed to delete contact on sync remove", "contact_id", contact.ID, "error", err)
			} else {
				a.Log.Info("Soft-deleted synced contact (remove) from mobile app", "contact_id", contact.ID, "phone", contactPhone)
			}
		} else {
			a.Log.Info("Contact not found for sync remove", "phone", contactPhone)
		}
	default:
		a.Log.Warn("Unknown contact sync action", "action", action)
	}
}
