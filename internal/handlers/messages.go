package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/templateutil"
	"github.com/shridarpatil/whatomate/internal/utils"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
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
	InteractiveType string            // "button", "list", "cta_url"
	BodyText        string            // Body text for interactive messages
	Buttons         []whatsapp.Button // For button/list messages
	ButtonText      string            // For CTA URL button
	URL             string            // For CTA URL button

	// Template messages
	Template        *models.Template
	BodyParams      map[string]string // Parameter name -> value (supports both named and positional)
	HeaderMediaID   string            // WhatsApp media ID for template header (IMAGE/VIDEO/DOCUMENT)
	ButtonURLParams map[string]string // Button index (as string) -> dynamic URL param value

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
			default: // "button" or "list"
				return a.WhatsApp.SendInteractiveButtons(sendCtx, waAccount, rcpt, req.BodyText, req.Buttons)
			}

		case models.MessageTypeTemplate:
			if req.Template == nil {
				return "", fmt.Errorf("template is required for template messages")
			}
			components := whatsapp.BuildTemplateComponents(req.BodyParams, req.Template.HeaderType, req.HeaderMediaID)
			// Add auto-generated button components (Flow needs flow_token)
			flowComponents := whatsapp.AutoButtonComponents(req.Template.Buttons)
			components = append(components, flowComponents...)
			// Add URL/COPY_CODE button components with dynamic params
			buttonComponents := whatsapp.ButtonURLParamsToComponents(req.ButtonURLParams, req.Template.Buttons)
			components = append(components, buttonComponents...)
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
				"template_name": req.Template.Name,
				"template_id":   req.Template.ID.String(),
			}
			// Store header media so it renders in the chat bubble
			if req.MediaURL != "" {
				msg.MediaURL = req.MediaURL
				msg.MediaMimeType = req.MediaMimeType
			}
			// Store template buttons so they render in the chat bubble
			if len(req.Template.Buttons) > 0 {
				msg.InteractiveData = a.buildInteractiveData(req)
			}
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
	case "list":
		rows := make([]any, len(req.Buttons))
		for i, btn := range req.Buttons {
			rows[i] = map[string]string{"id": btn.ID, "title": btn.Title}
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
		"media_url":        msg.MediaURL,
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
	ContactID      string            `json:"contact_id"`
	PhoneNumber    string            `json:"phone_number"`    // Alternative to contact_id - send to phone directly
	TemplateName   string            `json:"template_name"`   // Template name
	TemplateID     string            `json:"template_id"`     // Alternative: template UUID
	TemplateParams map[string]string `json:"template_params"` // Named or positional params
	ButtonParams   map[string]string `json:"button_params"`   // Button index -> dynamic URL param value
	AccountName    string            `json:"account_name"`    // Optional: specific WhatsApp account

	// Header media for templates with IMAGE/VIDEO/DOCUMENT headers.
	// Three options (in priority order):
	//   1. header_media_id  — pre-uploaded WhatsApp media ID (skip upload)
	//   2. header_media_url — URL to fetch the media from (server downloads & uploads to WhatsApp)
	//   3. multipart header_file — raw file upload via multipart/form-data
	HeaderMediaID  string `json:"header_media_id"`  // Already-uploaded WhatsApp media ID
	HeaderMediaURL string `json:"header_media_url"` // URL to download media from
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
		if v := form.Value["template_name"]; len(v) > 0 {
			req.TemplateName = v[0]
		}
		if v := form.Value["template_id"]; len(v) > 0 {
			req.TemplateID = v[0]
		}
		if v := form.Value["account_name"]; len(v) > 0 {
			req.AccountName = v[0]
		}
		// Parse template_params from JSON string
		if v := form.Value["template_params"]; len(v) > 0 && v[0] != "" {
			if err := json.Unmarshal([]byte(v[0]), &req.TemplateParams); err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid template_params JSON", nil, "")
			}
		}
		// Parse button_params from JSON string
		if v := form.Value["button_params"]; len(v) > 0 && v[0] != "" {
			if err := json.Unmarshal([]byte(v[0]), &req.ButtonParams); err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid button_params JSON", nil, "")
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
		var c models.Contact
		err := a.DB.Where("phone_number = ? AND organization_id = ?", phoneNumber, orgID).First(&c).Error
		if err != nil {
			// Contact not found, create new one
			c = models.Contact{
				BaseModel:      models.BaseModel{ID: uuid.New()},
				OrganizationID: orgID,
				PhoneNumber:    phoneNumber,
			}
			if err := a.DB.Create(&c).Error; err != nil {
				a.Log.Error("Failed to create contact", "error", err, "phone", phoneNumber)
				return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create contact", nil, "")
			}
			a.Log.Info("Contact created from API", "contact_id", c.ID, "phone", phoneNumber)
		}
		contact = &c
	}

	// Determine which WhatsApp account to use (explicit > template > contact > default)
	accountName := req.AccountName
	if accountName == "" {
		accountName = template.WhatsAppAccount
	}
	if accountName == "" && contact != nil {
		accountName = contact.WhatsAppAccount
	}

	account, err := a.resolveWhatsAppAccount(orgID, accountName)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	// Extract parameter names and resolve values
	paramNames := templateutil.ExtParamNames(template.BodyContent)
	bodyParams := templateutil.ResolveParamsFromMap(paramNames, req.TemplateParams)

	// Validate that all required parameters are provided
	if len(paramNames) > 0 {
		var missingParams []string
		for i, name := range paramNames {
			if i >= len(bodyParams) || bodyParams[i] == "" {
				missingParams = append(missingParams, name)
			}
		}
		if len(missingParams) > 0 {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
				fmt.Sprintf("Missing template parameters: %s. Expected parameters: %v", strings.Join(missingParams, ", "), paramNames),
				nil, "")
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
			mediaID, err := a.WhatsApp.UploadMedia(context.Background(), waAcct, headerMediaData, headerMimeType, "header")
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
	buttonParams := req.ButtonParams
	if strings.EqualFold(template.Category, "AUTHENTICATION") && len(buttonParams) == 0 {
		if code, ok := req.TemplateParams["1"]; ok && code != "" {
			for i, raw := range template.Buttons {
				if btn, ok := raw.(map[string]any); ok {
					btnType, _ := btn["type"].(string)
					if strings.EqualFold(btnType, "OTP") {
						if buttonParams == nil {
							buttonParams = make(map[string]string)
						}
						buttonParams[fmt.Sprintf("%d", i)] = code
						break
					}
				}
			}
		}
	}

	// Send using unified message sender
	msgReq := OutgoingMessageRequest{
		Account:         account,
		Contact:         contact,
		Type:            models.MessageTypeTemplate,
		Template:        &template,
		BodyParams:      req.TemplateParams,
		HeaderMediaID:   headerMediaID,
		MediaURL:        headerLocalPath,
		MediaMimeType:   headerMimeType,
		ButtonURLParams: buttonParams,
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
		Status:          message.Status,
		IsReply:         message.IsReply,
		WhatsAppAccount: message.WhatsAppAccount,
		CreatedAt:       message.CreatedAt,
		UpdatedAt:       message.UpdatedAt,
	}
	return r.SendEnvelope(response)
}

