package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
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
	Template   *models.Template
	BodyParams map[string]string // Parameter name -> value (supports both named and positional)

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

		// Get reply-to message ID if this is a reply
		var replyToMsgID string
		if req.ReplyToMessage != nil && req.ReplyToMessage.WhatsAppMessageID != "" {
			replyToMsgID = req.ReplyToMessage.WhatsAppMessageID
		}

		switch req.Type {
		case models.MessageTypeText:
			if replyToMsgID != "" {
				return a.WhatsApp.SendTextMessage(sendCtx, waAccount, req.Contact.PhoneNumber, req.Content, replyToMsgID)
			}
			return a.WhatsApp.SendTextMessage(sendCtx, waAccount, req.Contact.PhoneNumber, req.Content)

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
				return a.WhatsApp.SendImageMessage(sendCtx, waAccount, req.Contact.PhoneNumber, mediaID, req.Caption)
			case models.MessageTypeVideo:
				return a.WhatsApp.SendVideoMessage(sendCtx, waAccount, req.Contact.PhoneNumber, mediaID, req.Caption)
			case models.MessageTypeAudio:
				return a.WhatsApp.SendAudioMessage(sendCtx, waAccount, req.Contact.PhoneNumber, mediaID)
			default: // document
				return a.WhatsApp.SendDocumentMessage(sendCtx, waAccount, req.Contact.PhoneNumber, mediaID, req.MediaFilename, req.Caption)
			}

		case models.MessageTypeInteractive:
			switch req.InteractiveType {
			case "cta_url":
				return a.WhatsApp.SendCTAURLButton(sendCtx, waAccount, req.Contact.PhoneNumber, req.BodyText, req.ButtonText, req.URL)
			default: // "button" or "list"
				return a.WhatsApp.SendInteractiveButtons(sendCtx, waAccount, req.Contact.PhoneNumber, req.BodyText, req.Buttons)
			}

		case models.MessageTypeTemplate:
			if req.Template == nil {
				return "", fmt.Errorf("template is required for template messages")
			}
			return a.WhatsApp.SendTemplateMessage(sendCtx, waAccount, req.Contact.PhoneNumber, req.Template.Name, req.Template.Language, req.BodyParams)

		case models.MessageTypeFlow:
			if req.FlowID == "" {
				return "", fmt.Errorf("flow ID is required for flow messages")
			}
			return a.WhatsApp.SendFlowMessage(sendCtx, waAccount, req.Contact.PhoneNumber, req.FlowID, req.FlowHeader, req.BodyText, req.FlowCTA, req.FlowToken, req.FlowFirstScreen)

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
	return &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		AppID:       account.AppID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}
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

	case models.MessageTypeTemplate:
		if req.Template != nil {
			// Store actual rendered content instead of just template name
			content := replaceTemplateParams(req.Template.BodyContent, req.BodyParams)
			if content == "" {
				content = fmt.Sprintf("[Template: %s]", req.Template.DisplayName)
			}
			msg.Content = content
			msg.TemplateName = req.Template.Name
			msg.Metadata = models.JSONB{
				"template_name": req.Template.Name,
				"template_id":   req.Template.ID.String(),
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

// buildInteractiveData creates the InteractiveData JSONB for interactive messages
func (a *App) buildInteractiveData(req OutgoingMessageRequest) models.JSONB {
	switch req.InteractiveType {
	case "cta_url":
		return models.JSONB{
			"type":        "cta_url",
			"body":        req.BodyText,
			"button_text": req.ButtonText,
			"url":         req.URL,
		}
	case "list":
		rows := make([]interface{}, len(req.Buttons))
		for i, btn := range req.Buttons {
			rows[i] = map[string]string{"id": btn.ID, "title": btn.Title}
		}
		return models.JSONB{
			"type": "list",
			"body": req.BodyText,
			"rows": rows,
		}
	default: // "button"
		buttons := make([]interface{}, len(req.Buttons))
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
	if err != nil {
		a.DB.Model(msg).Updates(map[string]any{
			"status":        models.MessageStatusFailed,
			"error_message": err.Error(),
		})
		a.Log.Error("Failed to send message", "error", err, "message_id", msg.ID, "type", msg.MessageType)
		return
	}

	a.DB.Model(msg).Updates(map[string]any{
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
			Type: "message_status",
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

	payload := map[string]any{
		"id":           msg.ID,
		"contact_id":   contact.ID.String(),
		"direction":    msg.Direction,
		"message_type": msg.MessageType,
		"content":      map[string]string{"body": msg.Content},
		"status":       msg.Status,
		"created_at":   msg.CreatedAt,
		"updated_at":   msg.UpdatedAt,
	}

	// Add assigned user info
	if contact.AssignedUserID != nil {
		payload["assigned_user_id"] = contact.AssignedUserID.String()
	}
	payload["profile_name"] = contact.ProfileName

	// Add media fields
	if msg.MediaURL != "" {
		payload["media_url"] = msg.MediaURL
		payload["media_mime_type"] = msg.MediaMimeType
		payload["media_filename"] = msg.MediaFilename
	}

	// Add interactive data
	if msg.InteractiveData != nil {
		payload["interactive_data"] = msg.InteractiveData
	}

	// Add reply context
	if msg.IsReply && msg.ReplyToMessageID != nil {
		payload["is_reply"] = true
		payload["reply_to_message_id"] = msg.ReplyToMessageID.String()

		// Include reply preview for UI
		var replyToMsg models.Message
		if err := a.DB.First(&replyToMsg, msg.ReplyToMessageID).Error; err == nil {
			payload["reply_to_message"] = map[string]any{
				"id":           replyToMsg.ID.String(),
				"content":      replyToMsg.Content,
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
	ContactID      string                    `json:"contact_id"`
	PhoneNumber    string                    `json:"phone_number"`            // Alternative to contact_id - send to phone directly
	TemplateName   string                    `json:"template_name"`           // Template name
	TemplateID     string                    `json:"template_id"`             // Alternative: template UUID
	TemplateParams map[string]string         `json:"template_params"`         // Named or positional params
	Header         *TemplateHeaderRequest    `json:"header,omitempty"`        // Header parameters
	ButtonParams   []TemplateButtonParameter `json:"button_params,omitempty"` // Button parameters
	AccountName    string                    `json:"account_name"`            // Optional: specific WhatsApp account
}

// TemplateHeaderRequest represents header parameters for template message
type TemplateHeaderRequest struct {
	Type     string `json:"type"`               // text, image, video, document
	Value    string `json:"value,omitempty"`    // For type: text
	Link     string `json:"link,omitempty"`     // For type: image|video|document
	Filename string `json:"filename,omitempty"` // For type: document
}

// TemplateButtonParameter represents button parameters for template message
// Button index is determined by array position (first item = button 0, second = button 1, etc.)
type TemplateButtonParameter struct {
	Type  string `json:"type"`  // url or copy_code
	Value string `json:"value"` // Dynamic URL suffix or coupon code
}

// SendTemplateMessage sends a template message to a contact or phone number
func (a *App) SendTemplateMessage(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	var req SendTemplateMessageRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
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
		if err := a.DB.Where("id = ? AND organization_id = ?", templateID, orgID).First(&template).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Template not found", nil, "")
		}
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
	var phoneNumber string

	if req.ContactID != "" {
		cID, err := uuid.Parse(req.ContactID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
		}
		var c models.Contact
		if err := a.DB.Where("id = ? AND organization_id = ?", cID, orgID).First(&c).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
		}
		contact = &c
		phoneNumber = c.PhoneNumber
	} else {
		// Find or create contact from phone number
		phoneNumber = req.PhoneNumber
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

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if req.AccountName != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", req.AccountName, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
		}
	} else if template.WhatsAppAccount != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", template.WhatsAppAccount, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Template's WhatsApp account not found", nil, "")
		}
	} else if contact != nil && contact.WhatsAppAccount != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", contact.WhatsAppAccount, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Contact's WhatsApp account not found", nil, "")
		}
	} else {
		// Get default outgoing account
		if err := a.DB.Where("organization_id = ? AND is_default_outgoing = ?", orgID, true).First(&account).Error; err != nil {
			if err := a.DB.Where("organization_id = ?", orgID).First(&account).Error; err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No WhatsApp account configured", nil, "")
			}
		}
	}

	// Validate header if provided
	if req.Header != nil {
		if err := validateTemplateHeader(req.Header, &template); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
	}

	// Validate button parameters if provided
	if len(req.ButtonParams) > 0 {
		if err := validateTemplateButtons(req.ButtonParams, &template); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
	}

	// Extract parameter names and resolve values
	paramNames := ExtractParamNamesFromContent(template.BodyContent)
	bodyParams := ResolveParams(paramNames, req.TemplateParams)

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

	// Build components for template message
	components := buildTemplateComponents(req.Header, req.TemplateParams, req.ButtonParams, &template)

	// Send using WhatsApp client directly with components
	waAccount := a.toWhatsAppAccount(&account)
	ctx := context.Background()

	msgID, err := a.WhatsApp.SendTemplateMessageWithComponents(
		ctx,
		waAccount,
		phoneNumber,
		template.Name,
		template.Language,
		components,
	)

	if err != nil {
		a.Log.Error("Failed to send template message", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to send template message", nil, "")
	}

	// Create message record
	message := &models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New()},
		OrganizationID:    orgID,
		WhatsAppAccount:   account.Name,
		ContactID:         contact.ID,
		Direction:         models.DirectionOutgoing,
		MessageType:       models.MessageTypeTemplate,
		Status:            models.MessageStatusSent,
		WhatsAppMessageID: msgID,
		TemplateName:      template.Name,
		Content:           replaceTemplateParams(template.BodyContent, req.TemplateParams),
		SentByUserID:      &userID,
	}

	if err := a.DB.Create(message).Error; err != nil {
		a.Log.Error("Failed to create message record", "error", err)
	}

	// Broadcast via WebSocket
	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type: websocket.TypeNewMessage,
			Payload: map[string]interface{}{
				"message": message,
				"contact": contact,
			},
		})
	}

	// Update contact last message
	now := time.Now()
	preview := fmt.Sprintf("[Template: %s]", template.DisplayName)
	if template.DisplayName == "" {
		preview = fmt.Sprintf("[Template: %s]", template.Name)
	}
	a.DB.Model(contact).Updates(map[string]interface{}{
		"last_message_at":      &now,
		"last_message_preview": preview,
	})

	return r.SendEnvelope(map[string]any{
		"message_id":    message.ID,
		"status":        "sent",
		"template_name": template.Name,
		"phone_number":  phoneNumber,
	})
}

// ExtractParamNamesFromContent extracts parameter names from template content
// Supports both positional ({{1}}, {{2}}) and named ({{name}}, {{order_id}}) parameters
var templateParamPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func ExtractParamNamesFromContent(content string) []string {
	matches := templateParamPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var names []string
	for _, match := range matches {
		if len(match) > 1 {
			name := strings.TrimSpace(match[1])
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

// validateTemplateHeader validates header parameters against template definition
func validateTemplateHeader(header *TemplateHeaderRequest, template *models.Template) error {
	if header == nil {
		return nil
	}

	// Normalize header type
	headerType := strings.ToUpper(header.Type)

	// Check if template has a header
	if template.HeaderType == "" {
		return fmt.Errorf("template does not have a header, but header was provided")
	}

	// Validate header type matches template
	if headerType != template.HeaderType {
		return fmt.Errorf("header type mismatch: template expects '%s', got '%s'",
			template.HeaderType, headerType)
	}

	// Validate required fields based on type
	switch headerType {
	case "TEXT":
		if header.Value == "" {
			return fmt.Errorf("header value is required for text headers")
		}
	case "IMAGE", "VIDEO", "DOCUMENT":
		if header.Link == "" {
			return fmt.Errorf("header link is required for %s headers", strings.ToLower(headerType))
		}
		// For document, filename is optional but recommended
	default:
		return fmt.Errorf("invalid header type: %s", header.Type)
	}

	return nil
}

// validateTemplateButtons validates button parameters against template definition
func validateTemplateButtons(buttonParams []TemplateButtonParameter, template *models.Template) error {
	if len(buttonParams) == 0 {
		return nil
	}

	// Check if template has buttons
	if len(template.Buttons) == 0 {
		return fmt.Errorf("template does not have buttons, but button parameters were provided")
	}

	// Validate each button parameter (index is array position)
	for i, btnParam := range buttonParams {
		// Check button index is valid (based on array position)
		if i >= len(template.Buttons) {
			return fmt.Errorf("too many button parameters: provided %d, template has %d buttons", len(buttonParams), len(template.Buttons))
		}

		// Get the button definition from template using array index
		btnDef, ok := template.Buttons[i].(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid button definition at index %d", i)
		}

		btnType, _ := btnDef["type"].(string)
		btnType = strings.ToUpper(btnType)

		// Validate parameter type matches button type
		paramType := strings.ToLower(btnParam.Type)
		switch paramType {
		case "url":
			if btnType != "URL" {
				return fmt.Errorf("button at position %d is not a URL button (type: %s)", i, btnType)
			}
			// Check if URL has dynamic parameter
			btnURL, _ := btnDef["url"].(string)
			if !strings.Contains(btnURL, "{{") {
				return fmt.Errorf("button at position %d does not have a dynamic URL parameter", i)
			}
		case "copy_code":
			if btnType != "COPY_CODE" {
				return fmt.Errorf("button at position %d is not a COPY_CODE button (type: %s)", i, btnType)
			}
		default:
			return fmt.Errorf("invalid button parameter type: %s (must be 'url' or 'copy_code')", btnParam.Type)
		}

		// Validate value is provided
		if btnParam.Value == "" {
			return fmt.Errorf("button parameter value is required for button at position %d", i)
		}
	}

	return nil
}

// buildTemplateComponents builds WhatsApp API components array
func buildTemplateComponents(header *TemplateHeaderRequest, bodyParams map[string]string, buttonParams []TemplateButtonParameter, template *models.Template) []map[string]interface{} {
	components := []map[string]interface{}{}

	// Add header component if provided
	if header != nil {
		headerComp := map[string]interface{}{
			"type": "header",
		}

		headerType := strings.ToUpper(header.Type)
		switch headerType {
		case "TEXT":
			headerComp["parameters"] = []map[string]interface{}{
				{
					"type": "text",
					"text": header.Value,
				},
			}
		case "IMAGE":
			headerComp["parameters"] = []map[string]interface{}{
				{
					"type": "image",
					"image": map[string]string{
						"link": header.Link,
					},
				},
			}
		case "VIDEO":
			headerComp["parameters"] = []map[string]interface{}{
				{
					"type": "video",
					"video": map[string]string{
						"link": header.Link,
					},
				},
			}
		case "DOCUMENT":
			docParam := map[string]interface{}{
				"type": "document",
				"document": map[string]interface{}{
					"link": header.Link,
				},
			}
			if header.Filename != "" {
				docParam["document"].(map[string]interface{})["filename"] = header.Filename
			}
			headerComp["parameters"] = []map[string]interface{}{docParam}
		}

		components = append(components, headerComp)
	}

	// Add body component if parameters exist
	if len(bodyParams) > 0 {
		paramNames := ExtractParamNamesFromContent(template.BodyContent)
		resolvedParams := ResolveParams(paramNames, bodyParams)

		bodyParamsArray := []map[string]interface{}{}
		for _, value := range resolvedParams {
			bodyParamsArray = append(bodyParamsArray, map[string]interface{}{
				"type": "text",
				"text": value,
			})
		}

		components = append(components, map[string]interface{}{
			"type":       "body",
			"parameters": bodyParamsArray,
		})
	}

	// Add button components if parameters provided
	if len(buttonParams) > 0 {
		for i, btnParam := range buttonParams {
			btnComp := map[string]interface{}{
				"type":     "button",
				"sub_type": btnParam.Type,
				"index":    fmt.Sprintf("%d", i), // Use array position as index
			}

			switch strings.ToLower(btnParam.Type) {
			case "url":
				btnComp["parameters"] = []map[string]interface{}{
					{
						"type": "text",
						"text": btnParam.Value, // Dynamic URL suffix
					},
				}
			case "copy_code":
				btnComp["parameters"] = []map[string]interface{}{
					{
						"type":        "coupon_code",
						"coupon_code": btnParam.Value,
					},
				}
			}

			components = append(components, btnComp)
		}
	}

	return components
}

// replaceTemplateParams replaces {{1}}, {{2}}, {{name}}, etc. placeholders with actual values
func replaceTemplateParams(content string, params map[string]string) string {
	if content == "" || len(params) == 0 {
		return content
	}

	result := content
	// Extract param names from content to replace placeholders
	paramNames := ExtractParamNamesFromContent(content)
	for i, name := range paramNames {
		// Try to get value by name first (works for both named and positional)
		if val, ok := params[name]; ok {
			result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", name), val)
			continue
		}
		// Fall back to positional key (1-indexed)
		key := fmt.Sprintf("%d", i+1)
		if val, ok := params[key]; ok {
			result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", name), val)
		}
	}
	return result
}

// ResolveParams resolves both positional and named parameters to ordered values
func ResolveParams(paramNames []string, params map[string]string) []string {
	if len(paramNames) == 0 || len(params) == 0 {
		return nil
	}

	result := make([]string, len(paramNames))
	for i, name := range paramNames {
		// Try named key first
		if val, ok := params[name]; ok {
			result[i] = val
			continue
		}
		// Fall back to positional key (1-indexed)
		key := fmt.Sprintf("%d", i+1)
		if val, ok := params[key]; ok {
			result[i] = val
			continue
		}
		// Default to empty string
		result[i] = ""
	}
	return result
}
