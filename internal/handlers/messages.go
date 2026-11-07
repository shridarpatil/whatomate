package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/contactutil"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/internal/templateutil"
	"github.com/nikyjain/whatomate/internal/utils"
	"github.com/nikyjain/whatomate/internal/websocket"
	"github.com/nikyjain/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// ============================================================================
// Unified Message Sending
// ============================================================================

// OutgoingMessageRequest contains all parameters for sending any type of message
type OutgoingMessageRequest struct {
	// Required
	Account *models.WhatsAppAccount
	Contact *models.Contact

	// Message type determines which fields are used
	Type models.MessageType // text, image, video, audio, document, interactive, template

	// Text messages
	Content string

	// Media messages (image, video, audio, document)
	MediaID       string // WhatsApp media ID (if already uploaded)
	MediaData     []byte // Raw media data (if upload needed)
	MediaURL      string // Local media URL (for storage)
	MediaMimeType string
	MediaFilename string
	Caption       string

	// Interactive messages
	InteractiveType string            // "button", "list", "cta_url", "voice_call"
	BodyText        string            // Body text for interactive messages
	Buttons         []whatsapp.Button // For button/list messages
	ButtonText      string            // For CTA URL button or list picker label
	ListSectionText string            // For list messages
	URL             string            // For CTA URL button
	// Optional media header (image/video/document) shown above a reply-button
	// message, e.g. an event poster with Attending/Not-Attending buttons.
	HeaderMediaType string // "image" | "video" | "document"
	HeaderMediaLink string // Public URL to the header media

	// voice_call interactive (WhatsApp Business Calling)
	DisplayText      string // Button face label
	TTLMinutes       int    // How long the button stays clickable; 0 ⇒ Meta default
	VoiceCallPayload string // Opaque round-trip string; server-set, e.g. "agent:<uuid>" for sticky routing

	// Template messages
	Template                    *models.Template
	BodyParams                  map[string]string // Parameter name -> value (supports both named and positional)
	HeaderParams                map[string]string // Header-only param values; falls back to BodyParams if empty (used for TEXT headers with a {{var}})
	ButtonParams                map[int]string    // URL button index -> dynamic value
	ButtonPayloads              map[int]string    // Quick reply button index -> hidden payload
	ButtonURLParams             map[string]string // Button index (as string) -> dynamic URL/COPY_CODE param value
	HeaderMediaID               string            // WhatsApp media ID for template header (IMAGE/VIDEO/DOCUMENT)
	HeaderMediaFilename         string            // Filename — required by Meta for DOCUMENT headers
	ResponseCallbackURL         string
	ResponseCallbackBearerToken string

	// Delivery status callback (optional). When set, Whatomate POSTs every
	// status change of this message (sent, delivered, read, failed) to the URL.
	StatusCallbackURL         string
	StatusCallbackBearerToken string

	// WhatsApp Flow messages
	FlowID          string // Meta Flow ID
	FlowHeader      string // Optional header text for flow
	FlowCTA         string // CTA button text (max 20 chars)
	FlowToken       string // Unique token for flow response tracking
	FlowFirstScreen string // First screen name to navigate to

	// Reply context
	ReplyToMessage *models.Message
}

// MessageSendOptions configures optional behaviors for message sending
type MessageSendOptions struct {
	// BroadcastWebSocket enables WebSocket broadcast to org (default: true)
	BroadcastWebSocket bool

	// DispatchWebhook enables webhook dispatch for message.sent event (default: true)
	DispatchWebhook bool

	// TrackSLA enables SLA tracking for chatbot messages (default: false)
	TrackSLA bool

	// SentByUserID sets the user who sent the message (for agent messages)
	SentByUserID *uuid.UUID

	// Async if true, sends in background goroutine and returns immediately
	// Message is persisted before send, status updated after
	Async bool

	// MarkIncomingRead marks the contact's incoming messages as read after a
	// successful send. Used for chatbot replies so a bot-handled exchange
	// doesn't leave an "unread" badge in the agent's contact list. Only takes
	// effect when the org opted in via ChatbotSettings.MarkBotHandledAsRead.
	MarkIncomingRead bool
}

// DefaultSendOptions returns options suitable for agent UI sends
func DefaultSendOptions() MessageSendOptions {
	return MessageSendOptions{
		BroadcastWebSocket: true,
		DispatchWebhook:    true,
		TrackSLA:           false,
		Async:              true,
	}
}

// ChatbotSendOptions returns options suitable for chatbot sends
func ChatbotSendOptions() MessageSendOptions {
	return MessageSendOptions{
		BroadcastWebSocket: true,
		DispatchWebhook:    false,
		TrackSLA:           true,
		Async:              false,
		MarkIncomingRead:   true,
	}
}

// APISendOptions returns options suitable for API/template sends
func APISendOptions() MessageSendOptions {
	return MessageSendOptions{
		BroadcastWebSocket: false,
		DispatchWebhook:    true,
		TrackSLA:           false,
		Async:              true,
	}
}

// SLASendOptions returns options suitable for SLA system notifications
func SLASendOptions() MessageSendOptions {
	return MessageSendOptions{
		BroadcastWebSocket: true,
		DispatchWebhook:    false,
		TrackSLA:           false,
		Async:              false, // Sync to ensure message is sent before continuing
	}
}

// SendOutgoingMessage is the unified method for sending all types of WhatsApp messages.
// It handles: text, media (image/video/audio/document), interactive (buttons/list/cta_url), and template messages.
func (a *App) SendOutgoingMessage(ctx context.Context, req OutgoingMessageRequest, opts MessageSendOptions) (*models.Message, error) {
	// 1. Create message record
	msg := a.createOutgoingMessage(req, opts)

	// Save to database
	if err := a.DB.Create(msg).Error; err != nil {
		a.Log.Error("Failed to create message", "error", err)
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	// 2. Define the send function based on message type
	sendFn := func(sendCtx context.Context) (string, error) {
		waAccount := a.toWhatsAppAccount(req.Account)
		rcpt := whatsapp.Recipient{Phone: req.Contact.PhoneNumber, BSUID: req.Contact.BSUID}

		// Get reply-to message ID if this is a reply
		var replyToMsgID string
		if req.ReplyToMessage != nil && req.ReplyToMessage.WhatsAppMessageID != "" {
			replyToMsgID = req.ReplyToMessage.WhatsAppMessageID
		}

		switch req.Type {
		case models.MessageTypeText:
			return a.WhatsApp.SendTextMessage(sendCtx, waAccount, rcpt, req.Content, replyToMsgID)

		case models.MessageTypeImage, models.MessageTypeVideo, models.MessageTypeAudio, models.MessageTypeDocument:
			// Upload media if MediaData is provided and MediaID is not set
			mediaID := req.MediaID
			if mediaID == "" && len(req.MediaData) > 0 {
				var err error
				mediaID, err = a.WhatsApp.UploadMedia(sendCtx, waAccount, req.MediaData, req.MediaMimeType, req.MediaFilename)
				if err != nil {
					return "", fmt.Errorf("failed to upload media: %w", err)
				}
			}
			// Send the appropriate media type
			switch req.Type {
			case models.MessageTypeImage:
				return a.WhatsApp.SendImageMessage(sendCtx, waAccount, rcpt, mediaID, req.Caption)
			case models.MessageTypeVideo:
				return a.WhatsApp.SendVideoMessage(sendCtx, waAccount, rcpt, mediaID, req.Caption)
			case models.MessageTypeAudio:
				return a.WhatsApp.SendAudioMessage(sendCtx, waAccount, rcpt, mediaID)
			default: // document
				return a.WhatsApp.SendDocumentMessage(sendCtx, waAccount, rcpt, mediaID, req.MediaFilename, req.Caption)
			}

		case models.MessageTypeInteractive:
			switch req.InteractiveType {
			case "cta_url":
				return a.WhatsApp.SendCTAURLButton(sendCtx, waAccount, rcpt, req.BodyText, req.ButtonText, req.URL)
			case "voice_call":
				return a.WhatsApp.SendVoiceCallButton(sendCtx, waAccount, rcpt, req.BodyText, req.DisplayText, req.TTLMinutes, req.VoiceCallPayload)
			case "list":
				return a.WhatsApp.SendInteractiveList(sendCtx, waAccount, rcpt, req.BodyText, req.ButtonText, req.ListSectionText, req.Buttons)
			case "button":
				if strings.TrimSpace(req.HeaderMediaLink) != "" {
					return a.WhatsApp.SendInteractiveReplyButtonsWithHeader(sendCtx, waAccount, rcpt, req.BodyText, req.Buttons, req.HeaderMediaType, req.HeaderMediaLink)
				}
				return a.WhatsApp.SendInteractiveReplyButtons(sendCtx, waAccount, rcpt, req.BodyText, req.Buttons)
			default:
				return a.WhatsApp.SendInteractiveButtons(sendCtx, waAccount, rcpt, req.BodyText, req.Buttons)
			}

		case models.MessageTypeTemplate:
			if req.Template == nil {
				return "", fmt.Errorf("template is required for template messages")
			}
			headerFilename := req.MediaFilename
			if req.HeaderMediaFilename != "" {
				headerFilename = req.HeaderMediaFilename
			}
			components, err := whatsapp.BuildTemplateComponentsWithHeaderText(req.BodyParams, req.ButtonParams, req.ButtonPayloads, req.Template.Buttons, req.Template.HeaderType, req.Template.HeaderContent, req.HeaderParams, req.HeaderMediaID, headerFilename)
			if err != nil {
				return "", fmt.Errorf("failed to build template components: %w", err)
			}
			// Add auto-generated button components (Flow needs flow_token)
			components = append(components, whatsapp.AutoButtonComponents(req.Template.Buttons)...)
			// Add URL/COPY_CODE/OTP button components keyed by string index
			// (upstream-style button params) for buttons ButtonParams did not
			// already cover, so no button index is sent twice.
			if extra := buttonURLParamsExcluding(req.ButtonURLParams, req.ButtonParams); len(extra) > 0 {
				components = append(components, whatsapp.ButtonURLParamsToComponents(extra, req.Template.Buttons)...)
			}
			route := models.ResolveTemplateDeliveryRoute(req.Account, req.Template)
			if route == models.TemplateDeliveryRouteMarketingMessagesLite {
				return a.WhatsApp.SendMarketingTemplateMessage(sendCtx, waAccount, rcpt, req.Template.Name, req.Template.Language, components)
			}
			return a.WhatsApp.SendTemplateMessage(sendCtx, waAccount, rcpt, req.Template.Name, req.Template.Language, components)

		case models.MessageTypeFlow:
			if req.FlowID == "" {
				return "", fmt.Errorf("flow ID is required for flow messages")
			}
			return a.WhatsApp.SendFlowMessage(sendCtx, waAccount, rcpt, req.FlowID, req.FlowHeader, req.BodyText, req.FlowCTA, req.FlowToken, req.FlowFirstScreen)

		default:
			return "", fmt.Errorf("unsupported message type: %s", req.Type)
		}
	}

	// 3. Execute send (async or sync)
	if opts.Async {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			wamid, sendErr := sendFn(asyncCtx)
			a.finalizeMessageSend(msg, req, opts, wamid, sendErr)
		}()
	} else {
		wamid, err := sendFn(ctx)
		a.finalizeMessageSend(msg, req, opts, wamid, err)
	}

	// 4. Immediate actions (before send completes for async)
	if opts.BroadcastWebSocket {
		a.broadcastNewMessage(req.Account.OrganizationID, msg, req.Contact)
	}

	if opts.TrackSLA {
		a.UpdateContactChatbotMessage(req.Contact.ID)
	}

	// Update contact's last message
	preview := a.getMessagePreview(req)
	a.updateContactLastMessage(req.Contact, preview)

	return msg, nil
}

// ============================================================================
// Internal Helpers
// ============================================================================

// toWhatsAppAccount converts models.WhatsAppAccount to whatsapp.Account
func (a *App) toWhatsAppAccount(account *models.WhatsAppAccount) *whatsapp.Account {
	return account.ToWAAccount()
}

// createOutgoingMessage creates a Message model from the request
func (a *App) createOutgoingMessage(req OutgoingMessageRequest, opts MessageSendOptions) *models.Message {
	msg := &models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  req.Account.OrganizationID,
		WhatsAppAccount: req.Account.Name,
		ContactID:       req.Contact.ID,
		Direction:       models.DirectionOutgoing,
		MessageType:     req.Type,
		Status:          models.MessageStatusPending,
		SentByUserID:    opts.SentByUserID,
	}

	// Set content based on message type
	switch req.Type {
	case models.MessageTypeText:
		msg.Content = req.Content

	case models.MessageTypeImage, models.MessageTypeVideo, models.MessageTypeAudio, models.MessageTypeDocument:
		msg.Content = req.Caption
		msg.MediaURL = req.MediaURL
		msg.MediaMimeType = req.MediaMimeType
		msg.MediaFilename = req.MediaFilename

	case models.MessageTypeInteractive:
		msg.Content = req.BodyText
		msg.InteractiveData = a.buildInteractiveData(req)

	case models.MessageTypeFlow:
		msg.Content = req.BodyText
		msg.InteractiveData = models.JSONB{
			"type":        "flow",
			"body":        req.BodyText,
			"button_text": req.FlowCTA,
			"flow_id":     req.FlowID,
		}

	case models.MessageTypeTemplate:
		if req.Template != nil {
			// Store actual rendered content instead of just template name
			content := templateutil.ReplaceWithStringParams(req.Template.BodyContent, req.BodyParams)
			if content == "" {
				content = fmt.Sprintf("[Template: %s]", req.Template.DisplayName)
			}
			msg.Content = content
			msg.TemplateName = req.Template.Name
			msg.Metadata = models.JSONB{
				"template_name":  req.Template.Name,
				"template_id":    req.Template.ID.String(),
				"delivery_route": string(models.ResolveTemplateDeliveryRoute(req.Account, req.Template)),
			}
			if len(req.ButtonPayloads) > 0 {
				msg.Metadata["button_payloads"] = indexedStringMapToJSON(req.ButtonPayloads)
			}
			if req.ResponseCallbackURL != "" {
				msg.Metadata["response_callback_url"] = req.ResponseCallbackURL
			}
			if req.ResponseCallbackBearerToken != "" {
				msg.Metadata["response_callback_bearer_token"] = req.ResponseCallbackBearerToken
			}
			// Store header media so it renders in the chat bubble
			if req.MediaURL != "" {
				msg.MediaURL = req.MediaURL
				msg.MediaMimeType = req.MediaMimeType
				// Persist the DOCUMENT header filename too, otherwise the chat
				// bubble falls back to the literal "Document" label even though
				// the recipient received the file with its real name (the name
				// is only put on the outgoing Meta payload, never stored).
				msg.MediaFilename = req.HeaderMediaFilename
			}
			// Store template buttons so they render in the chat bubble
			if len(req.Template.Buttons) > 0 {
				msg.InteractiveData = a.buildInteractiveData(req)
			}
		}
	}

	// Persist the delivery status callback config (applies to every message type).
	// Stored on the message because status updates from Meta (delivered / read /
	// failed) arrive long after the send call has returned.
	if strings.TrimSpace(req.StatusCallbackURL) != "" {
		if msg.Metadata == nil {
			msg.Metadata = models.JSONB{}
		}
		msg.Metadata[metaKeyStatusCallbackURL] = strings.TrimSpace(req.StatusCallbackURL)
		if req.StatusCallbackBearerToken != "" {
			msg.Metadata[metaKeyStatusCallbackToken] = req.StatusCallbackBearerToken
		}
	}

	// Handle reply context
	if req.ReplyToMessage != nil {
		msg.IsReply = true
		replyID := req.ReplyToMessage.ID
		msg.ReplyToMessageID = &replyID
	}

	return msg
}

// buildInteractiveData creates the InteractiveData JSONB for interactive and template messages
func (a *App) buildInteractiveData(req OutgoingMessageRequest) models.JSONB {
	// Template buttons: stored as JSONBArray on Template.Buttons
	// Resolve dynamic URL params (e.g., {{1}}) before storing
	if req.Template != nil && len(req.Template.Buttons) > 0 {
		buttons := make([]any, len(req.Template.Buttons))
		for i, btn := range req.Template.Buttons {
			btnMap, ok := btn.(map[string]any)
			if !ok {
				buttons[i] = btn
				continue
			}
			resolved := make(map[string]any, len(btnMap))
			maps.Copy(resolved, btnMap)
			if resolved["type"] == "URL" {
				if urlStr, ok := resolved["url"].(string); ok {
					idx := fmt.Sprintf("%d", i)
					if val, exists := req.ButtonURLParams[idx]; exists {
						resolved["url"] = templateutil.ParameterPattern.ReplaceAllString(urlStr, val)
					}
				}
			}
			buttons[i] = resolved
		}
		return models.JSONB{
			"type":    "button",
			"buttons": buttons,
		}
	}

	switch req.InteractiveType {
	case "cta_url":
		return models.JSONB{
			"type":        "cta_url",
			"body":        req.BodyText,
			"button_text": req.ButtonText,
			"url":         req.URL,
		}
	case "voice_call":
		// Don't store the payload — it carries server-only context (the
		// originating agent id) and the chat history doesn't need it.
		out := models.JSONB{
			"type":         "voice_call",
			"body":         req.BodyText,
			"display_text": req.DisplayText,
		}
		if req.TTLMinutes > 0 {
			out["ttl_minutes"] = req.TTLMinutes
		}
		return out
	case "list":
		rows := make([]any, len(req.Buttons))
		for i, btn := range req.Buttons {
			row := map[string]string{"id": btn.ID, "title": btn.Title}
			if btn.Description != "" {
				row["description"] = btn.Description
			}
			rows[i] = row
		}
		return models.JSONB{
			"type": "list",
			"body": req.BodyText,
			"rows": rows,
		}
	default: // "button"
		buttons := make([]any, len(req.Buttons))
		for i, btn := range req.Buttons {
			buttons[i] = map[string]string{"id": btn.ID, "title": btn.Title}
		}
		return models.JSONB{
			"type":    "button",
			"body":    req.BodyText,
			"buttons": buttons,
		}
	}
}

func resolveTemplateButtonPayloads(buttons []interface{}, raw map[string]string) (map[int]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	resolved := make(map[int]string)
	for key, payload := range raw {
		key = strings.TrimSpace(key)
		if key == "" || payload == "" {
			continue
		}
		if idx, err := strconv.Atoi(key); err == nil {
			if idx < 0 {
				return nil, fmt.Errorf("button_payloads contains invalid button index %q", key)
			}
			resolved[idx] = payload
			continue
		}

		matched := false
		for idx, button := range buttons {
			btnMap, ok := button.(map[string]interface{})
			if !ok {
				continue
			}
			text, _ := btnMap["text"].(string)
			if strings.EqualFold(strings.TrimSpace(text), key) {
				resolved[idx] = payload
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("button_payloads key %q does not match a button index or text", key)
		}
	}
	if len(resolved) == 0 {
		return nil, nil
	}
	return resolved, nil
}

func indexedStringMapToJSON(values map[int]string) map[string]string {
	out := make(map[string]string, len(values))
	for idx, value := range values {
		out[strconv.Itoa(idx)] = value
	}
	return out
}

// finalizeMessageSend updates message status and triggers post-send actions
func (a *App) finalizeMessageSend(msg *models.Message, req OutgoingMessageRequest, opts MessageSendOptions, wamid string, err error) {
	// Use Where instead of Model(msg) to avoid mutating the shared msg struct,
	// which may be read concurrently by the caller when sending is async.
	if err != nil {
		errMsg := err.Error()

		a.DB.Model(&models.Message{}).Where("id = ?", msg.ID).Updates(map[string]any{
			"status":        models.MessageStatusFailed,
			"error_message": errMsg,
		})
		a.Log.Error("Failed to send message", "error", err, "message_id", msg.ID, "type", msg.MessageType)

		// Notify the caller-supplied status callback that the send failed
		a.dispatchMessageStatusCallback(msg, req.Contact, models.MessageStatusFailed, "", errMsg)

		// Broadcast failure status via WebSocket so frontend updates immediately
		if opts.BroadcastWebSocket && a.WSHub != nil {
			a.WSHub.BroadcastToOrg(req.Account.OrganizationID, websocket.WSMessage{
				Type: websocket.TypeStatusUpdate,
				Payload: map[string]any{
					"message_id":    msg.ID,
					"contact_id":    req.Contact.ID,
					"status":        models.MessageStatusFailed,
					"error_message": errMsg,
				},
			})
		}
		return
	}

	a.DB.Model(&models.Message{}).Where("id = ?", msg.ID).Updates(map[string]any{
		"status":               models.MessageStatusSent,
		"whats_app_message_id": wamid,
	})
	a.Log.Info("Message sent", "message_id", msg.ID, "wa_message_id", wamid, "type", msg.MessageType)

	// Notify the caller-supplied status callback that WhatsApp accepted the message
	a.dispatchMessageStatusCallback(msg, req.Contact, models.MessageStatusSent, wamid, "")

	// Dispatch webhook for successful send
	if opts.DispatchWebhook {
		a.dispatchMessageSentWebhook(req.Account, req.Contact, msg)
	}

	// Broadcast status update via WebSocket
	if opts.BroadcastWebSocket && a.WSHub != nil {
		a.WSHub.BroadcastToOrg(req.Account.OrganizationID, websocket.WSMessage{
			Type: websocket.TypeStatusUpdate,
			Payload: map[string]any{
				"message_id": msg.ID,
				"contact_id": req.Contact.ID,
				"status":     models.MessageStatusSent,
				"wamid":      wamid,
			},
		})
	}

	// Mark the contact's incoming messages as read once a chatbot reply has
	// gone out. Keeps the agent's contact-list unread count clean for
	// conversations the bot is auto-handling. See issue #280.
	if opts.MarkIncomingRead && a.chatbotMarksBotHandledRead(req.Account.OrganizationID, req.Account.Name) {
		a.markMessagesAsRead(req.Account.OrganizationID, req.Contact.ID, req.Contact)
	}
}

// broadcastNewMessage broadcasts a new message via WebSocket
func (a *App) broadcastNewMessage(orgID uuid.UUID, msg *models.Message, contact *models.Contact) {
	if a.WSHub == nil {
		return
	}

	var assignedUserIDStr string
	if contact.AssignedUserID != nil {
		assignedUserIDStr = contact.AssignedUserID.String()
	}
	profileName := contact.ProfileName
	if a.ShouldMaskPhoneNumbers(orgID) {
		profileName = utils.MaskIfPhoneNumber(profileName)
	}

	payload := map[string]any{
		"id":               msg.ID.String(),
		"contact_id":       contact.ID.String(),
		"assigned_user_id": assignedUserIDStr,
		"profile_name":     profileName,
		"direction":        msg.Direction,
		"message_type":     msg.MessageType,
		"content":          map[string]string{"body": msg.Content},
		"media_url":        messageMediaURL(msg),
		"media_mime_type":  msg.MediaMimeType,
		"media_filename":   msg.MediaFilename,
		"interactive_data": msg.InteractiveData,
		"status":           msg.Status,
		"wamid":            msg.WhatsAppMessageID,
		"created_at":       msg.CreatedAt,
		"updated_at":       msg.UpdatedAt,
		"is_reply":         msg.IsReply,
	}

	// Add interactive data
	if msg.InteractiveData != nil {
		payload["interactive_data"] = msg.InteractiveData
	}

	// Add reply context
	if msg.IsReply && msg.ReplyToMessageID != nil {
		payload["reply_to_message_id"] = msg.ReplyToMessageID.String()

		// Include reply preview for UI
		var replyToMsg models.Message
		if err := a.DB.First(&replyToMsg, msg.ReplyToMessageID).Error; err == nil {
			payload["reply_to_message"] = map[string]any{
				"id":           replyToMsg.ID.String(),
				"content":      map[string]string{"body": replyToMsg.Content},
				"message_type": replyToMsg.MessageType,
				"direction":    replyToMsg.Direction,
			}
		}
	}

	a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
		Type:    websocket.TypeNewMessage,
		Payload: payload,
	})
}

// broadcastReactionUpdate broadcasts a reaction update via WebSocket
func (a *App) broadcastReactionUpdate(orgID uuid.UUID, messageID, contactID uuid.UUID, reactions any) {
	if a.WSHub == nil {
		return
	}
	a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
		Type: "reaction_update",
		Payload: map[string]any{
			"message_id": messageID.String(),
			"contact_id": contactID.String(),
			"reactions":  reactions,
		},
	})
}

// dispatchMessageSentWebhook dispatches webhook for message.sent event
func (a *App) dispatchMessageSentWebhook(account *models.WhatsAppAccount, contact *models.Contact, msg *models.Message) {
	var sentByUserID string
	if msg.SentByUserID != nil {
		sentByUserID = msg.SentByUserID.String()
	}

	a.DispatchWebhook(account.OrganizationID, models.WebhookEventMessageSent, MessageEventData{
		MessageID:       msg.ID.String(),
		ContactID:       contact.ID.String(),
		ContactPhone:    contact.PhoneNumber,
		ContactName:     contact.ProfileName,
		MessageType:     msg.MessageType,
		Content:         msg.Content,
		MediaURL:        messageMediaURL(msg),
		MediaMimeType:   msg.MediaMimeType,
		MediaFilename:   msg.MediaFilename,
		WhatsAppAccount: account.Name,
		Direction:       models.DirectionOutgoing,
		SentByUserID:    sentByUserID,
	})
}

// updateContactLastMessage updates contact's last_message_at and preview
func (a *App) updateContactLastMessage(contact *models.Contact, preview string) {
	a.DB.Model(contact).Updates(map[string]any{
		"last_message_at":      time.Now(),
		"last_message_preview": preview,
	})
}

// getMessagePreview returns a preview string for the message
func (a *App) getMessagePreview(req OutgoingMessageRequest) string {
	switch req.Type {
	case models.MessageTypeText:
		return truncateString(req.Content, 100)
	case models.MessageTypeImage:
		if req.Caption != "" {
			return truncateString(req.Caption, 100)
		}
		return "[Image]"
	case models.MessageTypeVideo:
		if req.Caption != "" {
			return truncateString(req.Caption, 100)
		}
		return "[Video]"
	case models.MessageTypeAudio:
		return "[Audio]"
	case models.MessageTypeDocument:
		if req.MediaFilename != "" {
			return "[Document: " + req.MediaFilename + "]"
		}
		return "[Document]"
	case models.MessageTypeInteractive:
		return truncateString(req.BodyText, 100)
	case models.MessageTypeFlow:
		return truncateString(req.BodyText, 100)
	case models.MessageTypeTemplate:
		if req.Template != nil {
			return fmt.Sprintf("[Template: %s]", req.Template.DisplayName)
		}
		return "[Template]"
	default:
		return "[Message]"
	}
}

// ============================================================================
// HTTP Handlers
// ============================================================================

// SendTemplateMessageRequest represents the request to send a template message
type SendTemplateMessageRequest struct {
	ContactID                   string            `json:"contact_id"`
	PhoneNumber                 string            `json:"phone_number"`    // Alternative to contact_id - send to phone directly
	PhoneNumberID               string            `json:"phone_number_id"` // Optional: resolve WhatsApp account from Meta phone number ID
	TemplateName                string            `json:"template_name"`   // Template name
	TemplateID                  string            `json:"template_id"`     // Alternative: template UUID
	TemplateParams              map[string]string `json:"template_params"` // Named or positional params
	ButtonPayloads              map[string]string `json:"button_payloads"` // Quick reply button payloads by index ("0") or button text
	ButtonParams                map[string]string `json:"button_params"`   // Button index -> dynamic URL/COPY_CODE param value
	AccountName                 string            `json:"account_name"`    // Optional: specific WhatsApp account
	ResponseCallbackURL         string            `json:"response_callback_url"`
	ResponseCallbackBearerToken string            `json:"response_callback_bearer_token"`

	// Optional delivery status callback. When supplied, Whatomate POSTs the
	// status of this message (sent, delivered, read, failed) to this URL as it
	// changes. Omit both fields to keep the previous fire-and-forget behaviour.
	StatusCallbackURL         string `json:"status_callback_url"`
	StatusCallbackBearerToken string `json:"status_callback_bearer_token"`

	// Header media for templates with IMAGE/VIDEO/DOCUMENT headers.
	// Three options (in priority order):
	//   1. header_media_id  — pre-uploaded WhatsApp media ID (skip upload)
	//   2. header_media_url — URL to fetch the media from (server downloads & uploads to WhatsApp)
	//   3. multipart header_file — raw file upload via multipart/form-data
	HeaderMediaID       string `json:"header_media_id"`       // Already-uploaded WhatsApp media ID
	HeaderMediaURL      string `json:"header_media_url"`      // URL to download media from
	HeaderMediaFilename string `json:"header_media_filename"` // Filename — required by Meta for DOCUMENT headers (#351)

	// Header text parameter values for TEXT headers that contain a {{var}}.
	// Meta only permits one variable in a TEXT header. Keyed by the variable's
	// name (named templates) or by "1" (positional). Optional — if absent, the
	// value is looked up in TemplateParams as a fallback.
	HeaderParams map[string]string `json:"header_params"`
}

// SendTemplateMessage sends a template message to a contact or phone number.
// Accepts either JSON body or multipart/form-data (when a header media file is included).
func (a *App) SendTemplateMessage(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req SendTemplateMessageRequest
	var headerFileData []byte
	var headerFileMimeType string
	var headerFileFilename string

	contentType := string(r.RequestCtx.Request.Header.ContentType())
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form — used when template has a media header
		form, err := r.RequestCtx.MultipartForm()
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid multipart form", nil, "")
		}
		if v := form.Value["contact_id"]; len(v) > 0 {
			req.ContactID = v[0]
		}
		if v := form.Value["phone_number"]; len(v) > 0 {
			req.PhoneNumber = v[0]
		}
		if v := form.Value["phone_number_id"]; len(v) > 0 {
			req.PhoneNumberID = v[0]
		}
		if v := form.Value["template_name"]; len(v) > 0 {
			req.TemplateName = v[0]
		}
		if v := form.Value["template_id"]; len(v) > 0 {
			req.TemplateID = v[0]
		}
		if v := form.Value["account_name"]; len(v) > 0 {
			req.AccountName = v[0]
		}
		if v := form.Value["response_callback_url"]; len(v) > 0 {
			req.ResponseCallbackURL = v[0]
		}
		if v := form.Value["response_callback_bearer_token"]; len(v) > 0 {
			req.ResponseCallbackBearerToken = v[0]
		}
		if v := form.Value["status_callback_url"]; len(v) > 0 {
			req.StatusCallbackURL = v[0]
		}
		if v := form.Value["status_callback_bearer_token"]; len(v) > 0 {
			req.StatusCallbackBearerToken = v[0]
		}
		// Parse template_params from JSON string
		if v := form.Value["template_params"]; len(v) > 0 && v[0] != "" {
			if err := json.Unmarshal([]byte(v[0]), &req.TemplateParams); err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid template_params JSON", nil, "")
			}
		}
		if v := form.Value["header_media_filename"]; len(v) > 0 {
			req.HeaderMediaFilename = v[0]
		}
		if v := form.Value["button_payloads"]; len(v) > 0 && v[0] != "" {
			if err := json.Unmarshal([]byte(v[0]), &req.ButtonPayloads); err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid button_payloads JSON", nil, "")
			}
		}
		// Parse button_params from JSON string
		if v := form.Value["button_params"]; len(v) > 0 && v[0] != "" {
			if err := json.Unmarshal([]byte(v[0]), &req.ButtonParams); err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid button_params JSON", nil, "")
			}
		}
		// Parse header_params from JSON string
		if v := form.Value["header_params"]; len(v) > 0 && v[0] != "" {
			if err := json.Unmarshal([]byte(v[0]), &req.HeaderParams); err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid header_params JSON", nil, "")
			}
		}
		// Read header media file
		if files := form.File["header_file"]; len(files) > 0 {
			fh := files[0]
			f, err := fh.Open()
			if err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to read header file", nil, "")
			}
			defer f.Close() //nolint:errcheck
			headerFileData, err = io.ReadAll(f)
			if err != nil {
				a.Log.Error("Failed to read header file", "error", err)
				return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to read header file", nil, "")
			}
			headerFileMimeType = fh.Header.Get("Content-Type")
			if headerFileMimeType == "" {
				headerFileMimeType = "application/octet-stream"
			}
			headerFileFilename = fh.Filename
			// If the client didn't supply an explicit filename, fall back to multipart filename.
			if req.HeaderMediaFilename == "" && fh.Filename != "" {
				req.HeaderMediaFilename = fh.Filename
			}
		}
	} else {
		if err := a.decodeRequest(r, &req); err != nil {
			return nil
		}
	}

	// Must have either contact_id or phone_number
	if req.ContactID == "" && req.PhoneNumber == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Either contact_id or phone_number is required", nil, "")
	}

	// Reject an unusable status callback URL up front rather than failing silently later
	if err := validateCallbackURL("status_callback_url", req.StatusCallbackURL); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	// Must have either template_name or template_id
	if req.TemplateName == "" && req.TemplateID == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Either template_name or template_id is required", nil, "")
	}

	// Get template
	var template models.Template
	if req.TemplateID != "" {
		templateID, err := uuid.Parse(req.TemplateID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid template_id", nil, "")
		}
		t, err := findByIDAndOrg[models.Template](a.DB, r, templateID, orgID, "Template")
		if err != nil {
			return nil
		}
		template = *t
	} else {
		if err := a.DB.Where("name = ? AND organization_id = ?", req.TemplateName, orgID).First(&template).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Template not found", nil, "")
		}
	}

	// Check template is approved
	if template.Status != "APPROVED" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, fmt.Sprintf("Template is not approved (status: %s)", template.Status), nil, "")
	}

	// Get contact or use phone number directly
	var contact *models.Contact

	if req.ContactID != "" {
		cID, err := uuid.Parse(req.ContactID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
		}
		c, err := findByIDAndOrg[models.Contact](a.DB, r, cID, orgID, "Contact")
		if err != nil {
			return nil
		}
		contact = c
	} else {
		// Find or create contact from phone number
		phoneNumber := req.PhoneNumber
		c, created, err := contactutil.GetOrCreateContact(a.DB, orgID, phoneNumber, "")
		if err != nil {
			a.Log.Error("Failed to create contact", "error", err, "phone", phoneNumber)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create contact", nil, "")
		}
		if created {
			a.Log.Info("Contact created from API", "contact_id", c.ID, "phone", c.PhoneNumber)
		}
		contact = c
	}

	// Determine which WhatsApp account to use.
	// Priority: phone_number_id > account_name > template account > contact account > default.
	var account *models.WhatsAppAccount
	if req.PhoneNumberID != "" {
		account, err = a.resolveWhatsAppAccountByPhoneNumberID(orgID, req.PhoneNumberID)
	} else {
		accountName := req.AccountName
		if accountName == "" {
			accountName = template.WhatsAppAccount
		}
		if accountName == "" && contact != nil {
			accountName = contact.WhatsAppAccount
		}
		account, err = a.resolveWhatsAppAccount(orgID, accountName)
	}
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	// Extract parameter names and resolve values
	paramNames := templateutil.ExtParamNames(template.BodyContent)
	buttonParamNames := templateutil.ExtractURLButtonParamNames(template.Buttons)
	bodyParams := templateutil.ResolveParamsFromMap(paramNames, req.TemplateParams)
	resolvedBodyParams := templateutil.ResolveParamsMapFromMap(paramNames, req.TemplateParams)
	buttonParams, missingButtonParams := templateutil.ResolveURLButtonParamsFromMap(template.Buttons, urlButtonParamSource(req.TemplateParams, req.ButtonParams))
	buttonPayloads, err := resolveTemplateButtonPayloads(template.Buttons, req.ButtonPayloads)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	// Validate that all required parameters are provided
	var missingParams []string
	if len(paramNames) > 0 {
		for i, name := range paramNames {
			if i >= len(bodyParams) || bodyParams[i] == "" {
				missingParams = append(missingParams, name)
			}
		}
	}
	if len(missingButtonParams) > 0 {
		missingParams = append(missingParams, missingButtonParams...)
	}
	if len(missingParams) > 0 {
		expectedParams := append([]string{}, paramNames...)
		expectedParams = append(expectedParams, buttonParamNames...)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
			fmt.Sprintf("Missing template parameters: %s. Expected parameters: %v", strings.Join(missingParams, ", "), expectedParams),
			nil, "")
	}

	// Validate the header variable (TEXT headers only). Meta allows at most one
	// variable in a TEXT header — surface a clean 400 if it's missing.
	if template.HeaderType == "TEXT" {
		headerNames := templateutil.ExtParamNames(template.HeaderContent)
		if len(headerNames) > 1 {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
				fmt.Sprintf("Template header text contains %d variables; Meta allows at most 1", len(headerNames)),
				nil, "")
		}
		if len(headerNames) == 1 {
			name := headerNames[0]
			if req.HeaderParams[name] == "" && req.TemplateParams[name] == "" {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
					fmt.Sprintf("Missing header parameter %q. Pass it in header_params or template_params.", name),
					nil, "")
			}
		}
	}

	// Resolve header media for templates with IMAGE/VIDEO/DOCUMENT headers.
	// Priority: header_media_id > header_media_url > multipart header_file
	var headerMediaID string
	var headerMediaData []byte
	var headerMimeType string
	if template.HeaderType == "IMAGE" || template.HeaderType == "VIDEO" || template.HeaderType == "DOCUMENT" {
		if req.HeaderMediaID != "" {
			// Option 1: Pre-uploaded WhatsApp media ID — use directly (no local preview)
			headerMediaID = req.HeaderMediaID
		} else if req.HeaderMediaURL != "" {
			// Option 2: Download from URL, then upload to WhatsApp
			resp, err := http.Get(req.HeaderMediaURL)
			if err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to download header media from URL", nil, "")
			}
			defer resp.Body.Close() //nolint:errcheck
			if resp.StatusCode != http.StatusOK {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, fmt.Sprintf("Header media URL returned status %d", resp.StatusCode), nil, "")
			}
			headerMediaData, err = io.ReadAll(resp.Body)
			if err != nil {
				a.Log.Error("Failed to read header media from URL", "error", err)
				return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to read header media from URL", nil, "")
			}
			headerMimeType = resp.Header.Get("Content-Type")
			if headerMimeType == "" {
				headerMimeType = "application/octet-stream"
			}
		} else if len(headerFileData) > 0 {
			// Option 3: Multipart file upload
			headerMediaData = headerFileData
			headerMimeType = headerFileMimeType
		}

		// Upload to WhatsApp if we have raw data (options 2 & 3)
		if len(headerMediaData) > 0 {
			waAcct := a.toWhatsAppAccount(account)
			// Build a sensible filename: explicit > URL basename > "header".
			uploadFilename := strings.TrimSpace(req.HeaderMediaFilename)
			if uploadFilename == "" && req.HeaderMediaURL != "" {
				if parsedURL, err := url.Parse(req.HeaderMediaURL); err == nil {
					base := path.Base(parsedURL.Path)
					if base != "" && base != "/" && base != "." {
						uploadFilename = base
					}
				}
			}
			if uploadFilename == "" {
				uploadFilename = "header"
			}
			mediaID, err := a.WhatsApp.UploadMedia(context.Background(), waAcct, headerMediaData, headerMimeType, uploadFilename)
			if err != nil {
				a.Log.Error("Failed to upload template header media", "error", err)
				return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to upload header media to WhatsApp", nil, "")
			}
			headerMediaID = mediaID
		}
	}

	// Save header media locally so it can be served for chat preview
	var headerLocalPath string
	if len(headerMediaData) > 0 {
		localPath, err := a.saveMediaLocally(headerMediaData, headerMimeType, "header")
		if err != nil {
			a.Log.Error("Failed to save template header media locally", "error", err)
			// Non-fatal — message will still send, just won't show preview
		} else {
			headerLocalPath = localPath
		}
	}

	// Check marketing opt-out
	if contact.MarketingOptOut && strings.EqualFold(template.Category, "MARKETING") {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Contact has opted out of marketing messages", nil, "")
	}

	// For authentication templates with OTP COPY_CODE buttons, Meta expects
	// a button component with sub_type "url" and the OTP code as a text parameter.
	// Auto-populate from template_params["1"] so callers don't need button_params.
	buttonURLParams := req.ButtonParams
	if strings.EqualFold(template.Category, "AUTHENTICATION") && len(buttonURLParams) == 0 {
		if code, ok := req.TemplateParams["1"]; ok && code != "" {
			for i, raw := range template.Buttons {
				if btn, ok := raw.(map[string]any); ok {
					btnType, _ := btn["type"].(string)
					if strings.EqualFold(btnType, "OTP") {
						if buttonURLParams == nil {
							buttonURLParams = make(map[string]string)
						}
						buttonURLParams[fmt.Sprintf("%d", i)] = code
						break
					}
				}
			}
		}
	}

	// Resolve filename for DOCUMENT headers — required by Meta (#351).
	// Caller-supplied wins, then the multipart filename.
	headerMediaFilename := req.HeaderMediaFilename
	if headerMediaFilename == "" {
		headerMediaFilename = headerFileFilename
	}
	if headerMediaFilename == "" && req.HeaderMediaURL != "" {
		if mediaURL, err := url.Parse(req.HeaderMediaURL); err == nil {
			headerMediaFilename = path.Base(mediaURL.Path)
			if headerMediaFilename == "." || headerMediaFilename == "/" {
				headerMediaFilename = ""
			}
		}
	}

	// A contact with no WhatsApp account (typically one just auto-created from
	// phone_number) is bound to the sending account so the chat, replies and
	// account-scoped flows land on the right number. An existing assignment is
	// never overwritten here.
	if contact.WhatsAppAccount == "" {
		if err := a.DB.Model(contact).Update("whats_app_account", account.Name).Error; err != nil {
			a.Log.Warn("Failed to bind contact to sending account", "error", err, "contact_id", contact.ID)
		} else {
			contact.WhatsAppAccount = account.Name
		}
	}

	// Send using unified message sender
	msgReq := OutgoingMessageRequest{
		Account:                     account,
		Contact:                     contact,
		Type:                        models.MessageTypeTemplate,
		Template:                    &template,
		BodyParams:                  resolvedBodyParams,
		HeaderParams:                headerParamsWithFallback(req.HeaderParams, req.TemplateParams),
		ButtonParams:                buttonParams,
		ButtonPayloads:              buttonPayloads,
		HeaderMediaID:               headerMediaID,
		MediaURL:                    headerLocalPath,
		MediaMimeType:               headerMimeType,
		MediaFilename:               req.HeaderMediaFilename,
		HeaderMediaFilename:         headerMediaFilename,
		ButtonURLParams:             nonURLButtonParams(template.Buttons, buttonURLParams),
		ResponseCallbackURL:         req.ResponseCallbackURL,
		ResponseCallbackBearerToken: req.ResponseCallbackBearerToken,
		StatusCallbackURL:           req.StatusCallbackURL,
		StatusCallbackBearerToken:   req.StatusCallbackBearerToken,
	}

	opts := DefaultSendOptions()
	opts.SentByUserID = &userID

	ctx := context.Background()
	message, err := a.SendOutgoingMessage(ctx, msgReq, opts)
	if err != nil {
		a.Log.Error("Failed to send template message", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to send template message", nil, "")
	}

	// Build full message response (same shape as SendMessage)
	response := MessageResponse{
		ID:              message.ID,
		ContactID:       message.ContactID,
		Direction:       message.Direction,
		MessageType:     message.MessageType,
		Content:         map[string]string{"body": message.Content},
		InteractiveData: message.InteractiveData,
		MediaURL:        message.MediaURL,
		MediaMimeType:   message.MediaMimeType,
		MediaFilename:   message.MediaFilename,
		Status:          message.Status,
		IsReply:         message.IsReply,
		WhatsAppAccount: message.WhatsAppAccount,
		CreatedAt:       message.CreatedAt,
		UpdatedAt:       message.UpdatedAt,
	}
	return r.SendEnvelope(response)
}

// CreateExternalMessageRequest represents an externally-sourced outgoing message
// that should be persisted without calling the WhatsApp API.
type CreateExternalMessageRequest struct {
	ContactID       string             `json:"contact_id"`
	PhoneNumber     string             `json:"phone_number"`
	PhoneNumberID   string             `json:"phone_number_id"`
	ProfileName     string             `json:"profile_name"`
	WhatsAppAccount string             `json:"whatsapp_account"`
	Type            models.MessageType `json:"type"`
	Content         struct {
		Body string `json:"body"`
	} `json:"content"`
	MediaURL            string       `json:"media_url"`
	MediaMimeType       string       `json:"media_mime_type"`
	MediaFilename       string       `json:"media_filename"`
	HeaderMediaURL      string       `json:"header_media_url"`
	HeaderMediaMimeType string       `json:"header_media_mime_type"`
	HeaderMediaFilename string       `json:"header_media_filename"`
	InteractiveData     models.JSONB `json:"interactive_data"`
	TemplateName        string       `json:"template_name"`
	TemplateParams      models.JSONB `json:"template_params"`
	FlowResponse        models.JSONB `json:"flow_response"`
	Metadata            models.JSONB `json:"metadata"`
	WhatsAppMessageID   string       `json:"whatsapp_message_id"`
	ExternalMessageID   string       `json:"external_message_id"`
	ReplyToMessageID    string       `json:"reply_to_message_id"`
	SentAt              *time.Time   `json:"sent_at"`
}

// CreateExternalMessage persists an outbound message record from an external source
// without sending it to WhatsApp.
func (a *App) CreateExternalMessage(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req CreateExternalMessageRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.ContactID == "" && req.PhoneNumber == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Either contact_id or phone_number is required", nil, "")
	}
	if req.Type == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "type is required", nil, "")
	}

	var account *models.WhatsAppAccount
	if req.PhoneNumberID != "" {
		orgID, account, err = a.resolveExternalMessageTargetOrgAndAccount(r, userID, orgID, req.PhoneNumberID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
	}

	if !a.HasPermission(userID, models.ResourceChat, models.ActionWrite, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
	}

	// Resolve or create contact.
	var contact models.Contact
	if req.ContactID != "" {
		contactID, err := uuid.Parse(req.ContactID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
		}

		query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
		if !a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
			query = query.Where("assigned_user_id = ?", userID)
		}
		if err := query.First(&contact).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
		}
	} else {
		if !a.HasPermission(userID, models.ResourceContacts, models.ActionWrite, orgID) {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to create contacts", nil, "")
		}

		c, _, err := contactutil.GetOrCreateContact(a.DB, orgID, strings.TrimSpace(req.PhoneNumber), req.ProfileName)
		if err != nil {
			a.Log.Error("Failed to resolve contact for external message", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to resolve contact", nil, "")
		}
		contact = *c
	}

	if account == nil {
		account, err = a.resolveExternalMessageWhatsAppAccount(orgID, &contact, req)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
	}

	if contact.WhatsAppAccount == "" || contact.WhatsAppAccount != account.Name {
		a.DB.Model(&contact).Update("whats_app_account", account.Name)
		contact.WhatsAppAccount = account.Name
	}

	var replyToMessage *models.Message
	if req.ReplyToMessageID != "" {
		replyToID, err := uuid.Parse(req.ReplyToMessageID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid reply_to_message_id", nil, "")
		}
		var replyTo models.Message
		if err := a.DB.Where("id = ? AND contact_id = ?", replyToID, contact.ID).First(&replyTo).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Reply-to message not found", nil, "")
		}
		replyToMessage = &replyTo
	}

	createdAt := time.Now()
	if req.SentAt != nil && !req.SentAt.IsZero() {
		createdAt = req.SentAt.UTC()
	}

	metadata := req.Metadata
	if metadata == nil {
		metadata = models.JSONB{}
	}
	if _, ok := metadata["source"]; !ok {
		metadata["source"] = "external_api"
	}
	if req.ExternalMessageID != "" {
		metadata["external_message_id"] = req.ExternalMessageID
	}
	if req.PhoneNumberID != "" {
		if _, ok := metadata["phone_number_id"]; !ok {
			metadata["phone_number_id"] = req.PhoneNumberID
		}
	}

	contentBody := req.Content.Body
	interactiveData := req.InteractiveData
	mediaURL := req.MediaURL
	mediaMimeType := req.MediaMimeType
	mediaFilename := req.MediaFilename
	if req.Type == models.MessageTypeTemplate {
		var template *models.Template
		template, err = a.findTemplateForExternalMessage(orgID, account.Name, req.TemplateName)
		if err != nil {
			a.Log.Warn("Template not found for external message render", "template_name", req.TemplateName, "account", account.Name, "error", err)
		} else if template != nil {
			renderedContent := templateutil.ReplaceWithStringParams(template.BodyContent, jsonbToStringMap(req.TemplateParams))
			if renderedContent != "" {
				contentBody = renderedContent
			}
			if interactiveData == nil && len(template.Buttons) > 0 {
				interactiveData = a.buildInteractiveData(OutgoingMessageRequest{Template: template})
			}
		}
		mediaURL, mediaMimeType, mediaFilename = resolveExternalTemplateMedia(req, template, mediaURL, mediaMimeType, mediaFilename)
		if contentBody == "" {
			contentBody = externalTemplateFallback(req.TemplateName)
		}
	}
	if isRemoteMediaURL(mediaURL) {
		originalMediaURL := mediaURL
		localMediaURL, resolvedMimeType, resolvedFilename, err := a.persistExternalMedia(mediaURL, mediaMimeType, mediaFilename)
		if err != nil {
			a.Log.Warn("Failed to persist external message media locally", "error", err, "media_url", mediaURL, "message_type", req.Type)
		} else {
			mediaURL = localMediaURL
			mediaMimeType = resolvedMimeType
			mediaFilename = resolvedFilename
			metadata["external_media_url"] = originalMediaURL
		}
	}

	msg := &models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New(), CreatedAt: createdAt, UpdatedAt: createdAt},
		OrganizationID:    orgID,
		WhatsAppAccount:   account.Name,
		ContactID:         contact.ID,
		WhatsAppMessageID: req.WhatsAppMessageID,
		Direction:         models.DirectionOutgoing,
		MessageType:       req.Type,
		Content:           contentBody,
		MediaURL:          mediaURL,
		MediaMimeType:     mediaMimeType,
		MediaFilename:     mediaFilename,
		TemplateName:      req.TemplateName,
		TemplateParams:    req.TemplateParams,
		InteractiveData:   interactiveData,
		FlowResponse:      req.FlowResponse,
		Status:            models.MessageStatusSent,
		SentByUserID:      &userID,
		Metadata:          metadata,
	}

	if replyToMessage != nil {
		msg.IsReply = true
		msg.ReplyToMessageID = &replyToMessage.ID
	}

	// Idempotency: if a message with the same WhatsApp message ID already exists for
	// this organisation, return the existing record instead of creating a duplicate.
	// This prevents 100+ copies when Lambda retries after a timeout.
	if req.WhatsAppMessageID != "" {
		var existing models.Message
		if err := a.DB.Where("organization_id = ? AND whats_app_message_id = ?",
			orgID, req.WhatsAppMessageID).First(&existing).Error; err == nil {
			a.Log.Debug("Duplicate external message detected, returning existing record",
				"wamid", req.WhatsAppMessageID, "id", existing.ID)
			return r.SendEnvelope(MessageResponse{
				ID:              existing.ID,
				ContactID:       existing.ContactID,
				Direction:       existing.Direction,
				MessageType:     existing.MessageType,
				Content:         map[string]string{"body": existing.Content},
				MediaURL:        existing.MediaURL,
				MediaMimeType:   existing.MediaMimeType,
				MediaFilename:   existing.MediaFilename,
				InteractiveData: existing.InteractiveData,
				Status:          existing.Status,
				WAMID:           existing.WhatsAppMessageID,
				IsReply:         existing.IsReply,
				WhatsAppAccount: existing.WhatsAppAccount,
				CreatedAt:       existing.CreatedAt,
				UpdatedAt:       existing.UpdatedAt,
			})
		}
	}

	if err := a.DB.Create(msg).Error; err != nil {
		a.Log.Error("Failed to create external message", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create message", nil, "")
	}

	preview := a.getPersistedMessagePreview(msg)
	a.DB.Model(&contact).Updates(map[string]any{
		"last_message_at":      createdAt,
		"last_message_preview": preview,
		"whats_app_account":    account.Name,
	})
	contact.LastMessageAt = &createdAt
	contact.LastMessagePreview = preview

	a.broadcastNewMessage(orgID, msg, &contact)

	response := MessageResponse{
		ID:              msg.ID,
		ContactID:       msg.ContactID,
		Direction:       msg.Direction,
		MessageType:     msg.MessageType,
		Content:         map[string]string{"body": msg.Content},
		MediaURL:        msg.MediaURL,
		MediaMimeType:   msg.MediaMimeType,
		MediaFilename:   msg.MediaFilename,
		InteractiveData: msg.InteractiveData,
		Status:          msg.Status,
		WAMID:           msg.WhatsAppMessageID,
		IsReply:         msg.IsReply,
		WhatsAppAccount: msg.WhatsAppAccount,
		CreatedAt:       msg.CreatedAt,
		UpdatedAt:       msg.UpdatedAt,
	}
	if msg.IsReply && msg.ReplyToMessageID != nil && replyToMessage != nil {
		replyToID := msg.ReplyToMessageID.String()
		response.ReplyToMessageID = &replyToID
		response.ReplyToMessage = &ReplyPreview{
			ID:          replyToMessage.ID.String(),
			Content:     map[string]string{"body": replyToMessage.Content},
			MessageType: replyToMessage.MessageType,
			Direction:   replyToMessage.Direction,
		}
	}

	return r.SendEnvelope(response)
}

func (a *App) getPersistedMessagePreview(msg *models.Message) string {
	switch msg.MessageType {
	case models.MessageTypeText, models.MessageTypeInteractive, models.MessageTypeLocation, models.MessageTypeContact:
		return truncateString(msg.Content, 100)
	case models.MessageTypeImage:
		if msg.Content != "" {
			return truncateString(msg.Content, 100)
		}
		return "[Image]"
	case models.MessageTypeVideo:
		if msg.Content != "" {
			return truncateString(msg.Content, 100)
		}
		return "[Video]"
	case models.MessageTypeAudio:
		return "[Audio]"
	case models.MessageTypeDocument:
		if msg.MediaFilename != "" {
			return "[Document: " + msg.MediaFilename + "]"
		}
		return "[Document]"
	case models.MessageTypeTemplate:
		if msg.Content != "" {
			return truncateString(msg.Content, 100)
		}
		if msg.TemplateName != "" {
			return "[Template: " + msg.TemplateName + "]"
		}
		return "[Template]"
	case models.MessageTypeFlow:
		if msg.Content != "" {
			return truncateString(msg.Content, 100)
		}
		return "[Flow]"
	case models.MessageTypeReaction:
		if msg.Content != "" {
			return truncateString(msg.Content, 100)
		}
		return "[Reaction]"
	default:
		if msg.Content != "" {
			return truncateString(msg.Content, 100)
		}
		return "[Message]"
	}
}

func (a *App) findTemplateForExternalMessage(orgID uuid.UUID, accountName, templateName string) (*models.Template, error) {
	if templateName == "" {
		return nil, nil
	}

	var template models.Template
	query := a.DB.Where("organization_id = ? AND name = ?", orgID, templateName)
	if accountName != "" {
		query = query.Where("whats_app_account = ?", accountName)
	}
	if err := query.First(&template).Error; err != nil && accountName != "" {
		if err := a.DB.Where("organization_id = ? AND name = ?", orgID, templateName).First(&template).Error; err != nil {
			return nil, err
		}
		return &template, nil
	} else if err != nil {
		return nil, err
	}

	return &template, nil
}

func jsonbToStringMap(data models.JSONB) map[string]string {
	if len(data) == 0 {
		return nil
	}

	result := make(map[string]string, len(data))
	for key, value := range data {
		if value == nil {
			continue
		}
		result[key] = fmt.Sprintf("%v", value)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func externalTemplateFallback(templateName string) string {
	if templateName != "" {
		return "[Template: " + templateName + "]"
	}
	return "[Template]"
}

func (a *App) resolveExternalMessageWhatsAppAccount(orgID uuid.UUID, contact *models.Contact, req CreateExternalMessageRequest) (*models.WhatsAppAccount, error) {
	if req.PhoneNumberID != "" {
		return a.resolveWhatsAppAccountByPhoneNumberID(orgID, req.PhoneNumberID)
	}

	accountName := req.WhatsAppAccount
	if accountName == "" && contact != nil {
		accountName = contact.WhatsAppAccount
	}
	return a.resolveWhatsAppAccount(orgID, accountName)
}

func (a *App) resolveExternalMessageTargetOrgAndAccount(r *fastglue.Request, userID, fallbackOrgID uuid.UUID, phoneNumberID string) (uuid.UUID, *models.WhatsAppAccount, error) {
	account, err := a.resolveWhatsAppAccountByPhoneNumberID(fallbackOrgID, phoneNumberID)
	if err == nil {
		return fallbackOrgID, account, nil
	}

	// If the caller pinned an org via X-Organization-ID, respect that; no global search.
	if headerHasOrganizationOverride(r) {
		return fallbackOrgID, nil, err
	}

	// Allow global cross-org lookup when:
	//   a) the caller is a super-admin (JWT session), OR
	//   b) the request is authenticated via an API key (X-API-Key header) — API keys are
	//      explicitly provisioned tokens, and cross-org routing by phone_number_id is the
	//      intended behaviour for external-send Lambda integrations.
	if !a.IsSuperAdmin(userID) && !isAPIKeyRequest(r) {
		return fallbackOrgID, nil, err
	}

	account, err = a.resolveWhatsAppAccountByPhoneNumberIDGlobal(phoneNumberID)
	if err != nil {
		return fallbackOrgID, nil, err
	}

	return account.OrganizationID, account, nil
}

// isAPIKeyRequest reports whether the request was authenticated via an API key
// (X-API-Key header) rather than a JWT session cookie/bearer token.
func isAPIKeyRequest(r *fastglue.Request) bool {
	return strings.TrimSpace(string(r.RequestCtx.Request.Header.Peek("X-API-Key"))) != ""
}

func (a *App) resolveWhatsAppAccountByPhoneNumberID(orgID uuid.UUID, phoneNumberID string) (*models.WhatsAppAccount, error) {
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND phone_id = ?", orgID, phoneNumberID).First(&account).Error; err != nil {
		return nil, fmt.Errorf("WhatsApp account not found for phone_number_id")
	}
	a.decryptAccountSecrets(&account)
	return &account, nil
}

func (a *App) resolveWhatsAppAccountByPhoneNumberIDGlobal(phoneNumberID string) (*models.WhatsAppAccount, error) {
	var accounts []models.WhatsAppAccount
	if err := a.DB.Where("phone_id = ?", phoneNumberID).Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("WhatsApp account not found for phone_number_id")
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("WhatsApp account not found for phone_number_id")
	}
	if len(accounts) > 1 {
		return nil, fmt.Errorf("Multiple WhatsApp accounts found for phone_number_id; send X-Organization-ID") //nolint:staticcheck // ST1005: user-facing external API message, text must stay unchanged
	}

	account := accounts[0]
	a.decryptAccountSecrets(&account)
	return &account, nil
}

func headerHasOrganizationOverride(r *fastglue.Request) bool {
	return strings.TrimSpace(string(r.RequestCtx.Request.Header.Peek("X-Organization-ID"))) != ""
}

func resolveExternalTemplateMedia(req CreateExternalMessageRequest, template *models.Template, mediaURL, mediaMimeType, mediaFilename string) (string, string, string) {
	if req.HeaderMediaURL != "" || req.HeaderMediaMimeType != "" || req.HeaderMediaFilename != "" {
		if req.HeaderMediaURL != "" {
			mediaURL = req.HeaderMediaURL
		}
		if req.HeaderMediaMimeType != "" {
			mediaMimeType = req.HeaderMediaMimeType
		}
		if req.HeaderMediaFilename != "" {
			mediaFilename = req.HeaderMediaFilename
		}
		return mediaURL, mediaMimeType, mediaFilename
	}

	if mediaURL == "" && templateHasRenderableHeaderMedia(template) {
		mediaURL = strings.TrimSpace(template.HeaderContent)
	}

	return mediaURL, mediaMimeType, mediaFilename
}

func isRemoteMediaURL(mediaURL string) bool {
	mediaURL = strings.ToLower(strings.TrimSpace(mediaURL))
	return strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://")
}

func (a *App) persistExternalMedia(mediaURL, mediaMimeType, mediaFilename string) (string, string, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(mediaURL)
	if err != nil {
		return "", "", "", fmt.Errorf("download media: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("download media: unexpected status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("read media: %w", err)
	}

	resolvedMimeType := strings.TrimSpace(mediaMimeType)
	if resolvedMimeType == "" {
		resolvedMimeType = strings.TrimSpace(resp.Header.Get("Content-Type"))
	}
	if idx := strings.Index(resolvedMimeType, ";"); idx >= 0 {
		resolvedMimeType = strings.TrimSpace(resolvedMimeType[:idx])
	}
	if resolvedMimeType == "" && len(data) > 0 {
		resolvedMimeType = http.DetectContentType(data)
	}
	if resolvedMimeType == "" {
		resolvedMimeType = "application/octet-stream"
	}

	resolvedFilename := strings.TrimSpace(mediaFilename)
	if resolvedFilename == "" {
		if parsedURL, err := url.Parse(mediaURL); err == nil {
			resolvedFilename = path.Base(parsedURL.Path)
		}
	}
	if resolvedFilename == "" || resolvedFilename == "." || resolvedFilename == "/" {
		resolvedFilename = "external-media" + getExtensionFromMimeType(resolvedMimeType)
	}

	localPath, err := a.saveMediaLocally(data, resolvedMimeType, resolvedFilename)
	if err != nil {
		return "", "", "", fmt.Errorf("save media locally: %w", err)
	}

	return localPath, resolvedMimeType, resolvedFilename, nil
}

func templateHasRenderableHeaderMedia(template *models.Template) bool {
	if template == nil {
		return false
	}

	switch strings.ToUpper(strings.TrimSpace(template.HeaderType)) {
	case "IMAGE", "VIDEO", "DOCUMENT":
		content := strings.TrimSpace(template.HeaderContent)
		return strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") || strings.HasPrefix(content, "/")
	default:
		return false
	}
}

// headerParamsWithFallback returns the TEXT-header parameter lookup map: the
// caller's header_params win, and template_params act as a fallback so a named
// header variable can be supplied alongside the body values.
func headerParamsWithFallback(headerParams, templateParams map[string]string) map[string]string {
	if len(headerParams) == 0 && len(templateParams) == 0 {
		return nil
	}
	merged := make(map[string]string, len(headerParams)+len(templateParams))
	for k, v := range templateParams {
		merged[k] = v
	}
	for k, v := range headerParams {
		if v != "" {
			merged[k] = v
		}
	}
	return merged
}
