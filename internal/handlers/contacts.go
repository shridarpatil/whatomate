package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// ContactResponse represents a contact with additional fields for the frontend
type ContactResponse struct {
	ID                 uuid.UUID  `json:"id"`
	PhoneNumber        string     `json:"phone_number"`
	Name               string     `json:"name"`
	ProfileName        string     `json:"profile_name"`
	AvatarURL          string     `json:"avatar_url"`
	Status             string     `json:"status"`
	Tags               []string   `json:"tags"`
	CustomFields       any        `json:"custom_fields"`
	LastMessageAt      *time.Time `json:"last_message_at"`
	LastMessagePreview string     `json:"last_message_preview"`
	UnreadCount        int        `json:"unread_count"`
	AssignedUserID     *uuid.UUID `json:"assigned_user_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// MessageResponse represents a message for the frontend
type MessageResponse struct {
	ID               uuid.UUID      `json:"id"`
	ContactID        uuid.UUID      `json:"contact_id"`
	Direction        string         `json:"direction"`
	MessageType      string         `json:"message_type"`
	Content          any            `json:"content"`
	MediaURL         string         `json:"media_url,omitempty"`
	MediaMimeType    string         `json:"media_mime_type,omitempty"`
	MediaFilename    string         `json:"media_filename,omitempty"`
	InteractiveData  models.JSONB   `json:"interactive_data,omitempty"`
	Status           string         `json:"status"`
	WAMID            string         `json:"wamid"`
	Error            string         `json:"error_message"`
	IsReply          bool           `json:"is_reply"`
	ReplyToMessageID *string        `json:"reply_to_message_id,omitempty"`
	ReplyToMessage   *ReplyPreview  `json:"reply_to_message,omitempty"`
	Reactions        []ReactionInfo `json:"reactions,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// ReplyPreview contains a preview of the replied-to message
type ReplyPreview struct {
	ID          string `json:"id"`
	Content     any    `json:"content"`
	MessageType string `json:"message_type"`
	Direction   string `json:"direction"`
}

// ReactionInfo represents a reaction on a message
type ReactionInfo struct {
	Emoji     string `json:"emoji"`
	FromPhone string `json:"from_phone,omitempty"`
	FromUser  string `json:"from_user,omitempty"`
}

// ListContacts returns all contacts for the organization
// Agents only see contacts assigned to them
func (a *App) ListContacts(r *fastglue.Request) error {
	orgID := r.RequestCtx.UserValue("organization_id").(uuid.UUID)
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	userRole, _ := r.RequestCtx.UserValue("role").(string)

	// Pagination
	page, _ := strconv.Atoi(string(r.RequestCtx.QueryArgs().Peek("page")))
	limit, _ := strconv.Atoi(string(r.RequestCtx.QueryArgs().Peek("limit")))
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	var contacts []models.Contact
	query := a.DB.Where("organization_id = ?", orgID)

	// Agents can only see contacts assigned to them
	if userRole == "agent" {
		query = query.Where("assigned_user_id = ?", userID)
	}

	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("phone_number LIKE ? OR profile_name LIKE ?", searchPattern, searchPattern)
	}

	// Order by last message time (most recent first)
	query = query.Order("last_message_at DESC NULLS LAST, created_at DESC")

	var total int64
	query.Model(&models.Contact{}).Count(&total)

	if err := query.Offset(offset).Limit(limit).Find(&contacts).Error; err != nil {
		a.Log.Error("Failed to list contacts", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list contacts", nil, "")
	}

	// Check if phone masking is enabled
	shouldMask := a.ShouldMaskPhoneNumbers(orgID)

	// Convert to response format
	response := make([]ContactResponse, len(contacts))
	for i, c := range contacts {
		// Count unread messages
		var unreadCount int64
		a.DB.Model(&models.Message{}).
			Where("contact_id = ? AND direction = ? AND status != ?", c.ID, "incoming", "read").
			Count(&unreadCount)

		tags := []string{}
		if c.Tags != nil {
			for _, t := range c.Tags {
				if s, ok := t.(string); ok {
					tags = append(tags, s)
				}
			}
		}

		phoneNumber := c.PhoneNumber
		profileName := c.ProfileName
		if shouldMask {
			phoneNumber = MaskPhoneNumber(phoneNumber)
			profileName = MaskIfPhoneNumber(profileName)
		}

		response[i] = ContactResponse{
			ID:                 c.ID,
			PhoneNumber:        phoneNumber,
			Name:               profileName,
			ProfileName:        profileName,
			Status:             "active",
			Tags:               tags,
			CustomFields:       c.Metadata,
			LastMessageAt:      c.LastMessageAt,
			LastMessagePreview: c.LastMessagePreview,
			UnreadCount:        int(unreadCount),
			AssignedUserID:     c.AssignedUserID,
			CreatedAt:          c.CreatedAt,
			UpdatedAt:          c.UpdatedAt,
		}
	}

	return r.SendEnvelope(map[string]any{
		"contacts": response,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// GetContact returns a single contact
// Agents can only access contacts assigned to them
func (a *App) GetContact(r *fastglue.Request) error {
	orgID := r.RequestCtx.UserValue("organization_id").(uuid.UUID)
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	userRole, _ := r.RequestCtx.UserValue("role").(string)
	contactIDStr := r.RequestCtx.UserValue("id").(string)

	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact ID", nil, "")
	}

	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)

	// Agents can only access their assigned contacts
	if userRole == "agent" {
		query = query.Where("assigned_user_id = ?", userID)
	}

	if err := query.First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Count unread messages
	var unreadCount int64
	a.DB.Model(&models.Message{}).
		Where("contact_id = ? AND direction = ? AND status != ?", contact.ID, "incoming", "read").
		Count(&unreadCount)

	tags := []string{}
	if contact.Tags != nil {
		for _, t := range contact.Tags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	phoneNumber := contact.PhoneNumber
	profileName := contact.ProfileName
	shouldMask := a.ShouldMaskPhoneNumbers(orgID)
	if shouldMask {
		phoneNumber = MaskPhoneNumber(phoneNumber)
		profileName = MaskIfPhoneNumber(profileName)
	}

	response := ContactResponse{
		ID:                 contact.ID,
		PhoneNumber:        phoneNumber,
		Name:               profileName,
		ProfileName:        profileName,
		Status:             "active",
		Tags:               tags,
		CustomFields:       contact.Metadata,
		LastMessageAt:      contact.LastMessageAt,
		LastMessagePreview: contact.LastMessagePreview,
		UnreadCount:        int(unreadCount),
		AssignedUserID:     contact.AssignedUserID,
		CreatedAt:          contact.CreatedAt,
		UpdatedAt:          contact.UpdatedAt,
	}

	return r.SendEnvelope(response)
}

// GetMessages returns messages for a contact
// Agents can only access messages for their assigned contacts
// Supports cursor-based pagination with before_id for loading older messages
func (a *App) GetMessages(r *fastglue.Request) error {
	orgID := r.RequestCtx.UserValue("organization_id").(uuid.UUID)
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	userRole, _ := r.RequestCtx.UserValue("role").(string)
	contactIDStr := r.RequestCtx.UserValue("id").(string)

	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact ID", nil, "")
	}

	// Verify contact belongs to org (and to agent if role is agent)
	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
	if userRole == "agent" {
		query = query.Where("assigned_user_id = ?", userID)
	}
	if err := query.First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Pagination parameters
	limit, _ := strconv.Atoi(string(r.RequestCtx.QueryArgs().Peek("limit")))
	beforeIDStr := string(r.RequestCtx.QueryArgs().Peek("before_id"))

	if limit < 1 || limit > 100 {
		limit = 50
	}

	// Build base query
	msgQuery := a.DB.Where("contact_id = ?", contactID)

	// Check if agent should only see current conversation
	if userRole == "agent" {
		settings, err := a.getChatbotSettingsCached(orgID, "")
		if err == nil {
			if settings.AgentCurrentConversationOnly {
				// Find the most recent session for this contact
				var session models.ChatbotSession
				if err := a.DB.Where("contact_id = ? AND organization_id = ?", contactID, orgID).
					Order("started_at DESC").First(&session).Error; err == nil {
					// Filter messages to only those from this session onwards
					msgQuery = msgQuery.Where("created_at >= ?", session.StartedAt)
				}
			}
		}
	}

	// Count total messages (with session filter if applied)
	var total int64
	msgQuery.Model(&models.Message{}).Count(&total)

	// Cursor-based pagination: load messages before a specific ID
	if beforeIDStr != "" {
		beforeID, err := uuid.Parse(beforeIDStr)
		if err == nil {
			// Get the created_at of the before_id message
			var beforeMsg models.Message
			if err := a.DB.Where("id = ?", beforeID).First(&beforeMsg).Error; err == nil {
				msgQuery = msgQuery.Where("created_at < ?", beforeMsg.CreatedAt)
			}
		}
		// For loading older messages, order DESC and limit, then reverse
		var messages []models.Message
		if err := msgQuery.Preload("ReplyToMessage").Order("created_at DESC").Limit(limit).Find(&messages).Error; err != nil {
			a.Log.Error("Failed to list messages", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list messages", nil, "")
		}
		// Reverse to get chronological order
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}

		response := a.buildMessagesResponse(messages)
		return r.SendEnvelope(map[string]any{
			"messages": response,
			"total":    total,
			"has_more": len(messages) == limit,
		})
	}

	// Default: load most recent messages (page 1)
	page, _ := strconv.Atoi(string(r.RequestCtx.QueryArgs().Peek("page")))
	if page < 1 {
		page = 1
	}

	// For chat, we want the most recent messages
	// Calculate offset from the end for pagination
	offset := int(total) - (page * limit)
	if offset < 0 {
		limit = limit + offset // Adjust limit if we're on the last page
		offset = 0
	}

	var messages []models.Message
	if err := msgQuery.Preload("ReplyToMessage").Order("created_at ASC").Offset(offset).Limit(limit).Find(&messages).Error; err != nil {
		a.Log.Error("Failed to list messages", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list messages", nil, "")
	}

	// Mark messages as read
	a.markMessagesAsRead(orgID, contactID, &contact)

	response := a.buildMessagesResponse(messages)
	return r.SendEnvelope(map[string]any{
		"messages": response,
		"total":    total,
		"page":     page,
		"limit":    limit,
		"has_more": offset > 0,
	})
}

// buildMessagesResponse converts messages to response format
func (a *App) buildMessagesResponse(messages []models.Message) []MessageResponse {
	response := make([]MessageResponse, len(messages))
	for i, m := range messages {
		var content any
		if m.MessageType == "text" {
			content = map[string]string{"body": m.Content}
		} else {
			content = map[string]string{"body": m.Content}
		}

		msgResp := MessageResponse{
			ID:              m.ID,
			ContactID:       m.ContactID,
			Direction:       m.Direction,
			MessageType:     m.MessageType,
			Content:         content,
			MediaURL:        m.MediaURL,
			MediaMimeType:   m.MediaMimeType,
			MediaFilename:   m.MediaFilename,
			InteractiveData: m.InteractiveData,
			Status:          m.Status,
			WAMID:           m.WhatsAppMessageID,
			Error:           m.ErrorMessage,
			IsReply:         m.IsReply,
			CreatedAt:       m.CreatedAt,
			UpdatedAt:       m.UpdatedAt,
		}

		if m.IsReply && m.ReplyToMessageID != nil {
			replyToID := m.ReplyToMessageID.String()
			msgResp.ReplyToMessageID = &replyToID
			if m.ReplyToMessage != nil {
				msgResp.ReplyToMessage = &ReplyPreview{
					ID:          m.ReplyToMessage.ID.String(),
					Content:     map[string]string{"body": m.ReplyToMessage.Content},
					MessageType: m.ReplyToMessage.MessageType,
					Direction:   m.ReplyToMessage.Direction,
				}
			}
		}

		if m.Metadata != nil {
			if reactionsRaw, ok := m.Metadata["reactions"]; ok {
				if reactionsArray, ok := reactionsRaw.([]interface{}); ok {
					for _, r := range reactionsArray {
						if rMap, ok := r.(map[string]interface{}); ok {
							emoji, _ := rMap["emoji"].(string)
							fromPhone, _ := rMap["from_phone"].(string)
							fromUser, _ := rMap["from_user"].(string)
							msgResp.Reactions = append(msgResp.Reactions, ReactionInfo{
								Emoji:     emoji,
								FromPhone: fromPhone,
								FromUser:  fromUser,
							})
						}
					}
				}
			}
		}

		response[i] = msgResp
	}
	return response
}

// markMessagesAsRead marks messages as read and sends read receipts
func (a *App) markMessagesAsRead(orgID uuid.UUID, contactID uuid.UUID, contact *models.Contact) {
	var unreadMessages []models.Message
	a.DB.Where("contact_id = ? AND direction = ? AND status != ?", contactID, "incoming", "read").
		Find(&unreadMessages)

	a.DB.Model(&models.Message{}).
		Where("contact_id = ? AND direction = ?", contactID, "incoming").
		Update("status", "read")

	a.DB.Model(contact).Update("is_read", true)

	if len(unreadMessages) > 0 && contact.WhatsAppAccount != "" {
		var account models.WhatsAppAccount
		if err := a.DB.Where("organization_id = ? AND name = ?", orgID, contact.WhatsAppAccount).First(&account).Error; err == nil {
			if account.AutoReadReceipt {
				go func() {
					waAccount := &whatsapp.Account{
						PhoneID:     account.PhoneID,
						AccessToken: account.AccessToken,
						APIVersion:  a.Config.WhatsApp.APIVersion,
					}
					for _, msg := range unreadMessages {
						if msg.WhatsAppMessageID != "" {
							if err := a.WhatsApp.MarkMessageRead(context.Background(), waAccount, msg.WhatsAppMessageID); err != nil {
								a.Log.Error("Failed to send read receipt", "error", err, "message_id", msg.WhatsAppMessageID)
							}
						}
					}
				}()
			}
		}
	}
}

// SendMessageRequest represents a send message request
type SendMessageRequest struct {
	Type    string `json:"type"`
	Content struct {
		Body string `json:"body"`
	} `json:"content"`
	ReplyToMessageID string `json:"reply_to_message_id,omitempty"`
}

// SendMessage sends a message to a contact
// Agents can only send messages to their assigned contacts
func (a *App) SendMessage(r *fastglue.Request) error {
	orgID := r.RequestCtx.UserValue("organization_id").(uuid.UUID)
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	userRole, _ := r.RequestCtx.UserValue("role").(string)
	contactIDStr := r.RequestCtx.UserValue("id").(string)

	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact ID", nil, "")
	}

	// Parse request body
	var req SendMessageRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Get contact (agents can only message their assigned contacts)
	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
	if userRole == "agent" {
		query = query.Where("assigned_user_id = ?", userID)
	}
	if err := query.First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if contact.WhatsAppAccount != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", contact.WhatsAppAccount, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
		}
	} else {
		// Get default outgoing account
		if err := a.DB.Where("organization_id = ? AND is_default_outgoing = ?", orgID, true).First(&account).Error; err != nil {
			// Fall back to any account
			if err := a.DB.Where("organization_id = ?", orgID).First(&account).Error; err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No WhatsApp account configured", nil, "")
			}
		}
	}

	// Create message record
	message := models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: account.Name,
		ContactID:       contactID,
		Direction:       "outgoing",
		MessageType:     req.Type,
		Content:         req.Content.Body,
		Status:          "pending",
		SentByUserID:    &userID,
	}

	// Handle reply context
	var replyToMessage *models.Message
	if req.ReplyToMessageID != "" {
		replyToID, err := uuid.Parse(req.ReplyToMessageID)
		if err == nil {
			var replyTo models.Message
			if err := a.DB.Where("id = ? AND contact_id = ?", replyToID, contactID).First(&replyTo).Error; err == nil {
				message.IsReply = true
				message.ReplyToMessageID = &replyToID
				replyToMessage = &replyTo
			}
		}
	}

	if err := a.DB.Create(&message).Error; err != nil {
		a.Log.Error("Failed to create message", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create message", nil, "")
	}

	// Send via WhatsApp API
	go a.sendWhatsAppMessage(&account, &contact, &message)

	// Update contact's last message
	now := time.Now()
	a.DB.Model(&contact).Updates(map[string]any{
		"last_message_at":      now,
		"last_message_preview": truncateString(req.Content.Body, 100),
	})

	response := MessageResponse{
		ID:          message.ID,
		ContactID:   message.ContactID,
		Direction:   message.Direction,
		MessageType: message.MessageType,
		Content:     map[string]string{"body": message.Content},
		Status:      message.Status,
		IsReply:     message.IsReply,
		CreatedAt:   message.CreatedAt,
		UpdatedAt:   message.UpdatedAt,
	}

	// Add reply context to response
	if message.IsReply && message.ReplyToMessageID != nil && replyToMessage != nil {
		replyToID := message.ReplyToMessageID.String()
		response.ReplyToMessageID = &replyToID
		response.ReplyToMessage = &ReplyPreview{
			ID:          replyToMessage.ID.String(),
			Content:     map[string]string{"body": replyToMessage.Content},
			MessageType: replyToMessage.MessageType,
			Direction:   replyToMessage.Direction,
		}
	}

	// Broadcast new outgoing message via WebSocket
	if a.WSHub != nil {
		wsPayload := map[string]any{
			"id":           message.ID,
			"contact_id":   message.ContactID,
			"direction":    message.Direction,
			"message_type": message.MessageType,
			"content":      map[string]string{"body": message.Content},
			"status":       message.Status,
			"created_at":   message.CreatedAt,
			"updated_at":   message.UpdatedAt,
			"is_reply":     message.IsReply,
		}
		if message.IsReply && message.ReplyToMessageID != nil && replyToMessage != nil {
			wsPayload["reply_to_message_id"] = message.ReplyToMessageID.String()
			wsPayload["reply_to_message"] = map[string]any{
				"id":           replyToMessage.ID.String(),
				"content":      map[string]string{"body": replyToMessage.Content},
				"message_type": replyToMessage.MessageType,
				"direction":    replyToMessage.Direction,
			}
		}
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type:    websocket.TypeNewMessage,
			Payload: wsPayload,
		})
	}

	return r.SendEnvelope(response)
}

// sendWhatsAppMessage sends a message via the WhatsApp Cloud API
func (a *App) sendWhatsAppMessage(account *models.WhatsAppAccount, contact *models.Contact, message *models.Message) {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", account.APIVersion, account.PhoneID)

	payload := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                contact.PhoneNumber,
		"type":              message.MessageType,
	}

	if message.MessageType == "text" {
		payload["text"] = map[string]any{
			"preview_url": false,
			"body":        message.Content,
		}
	}

	// Add reply context if this is a reply
	if message.IsReply && message.ReplyToMessageID != nil {
		var replyToMsg models.Message
		if err := a.DB.First(&replyToMsg, message.ReplyToMessageID).Error; err == nil && replyToMsg.WhatsAppMessageID != "" {
			payload["context"] = map[string]any{
				"message_id": replyToMsg.WhatsAppMessageID,
			}
		}
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		a.Log.Error("Failed to marshal message payload", "error", err)
		a.DB.Model(message).Updates(map[string]any{
			"status":        "failed",
			"error_message": "Failed to create request",
		})
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		a.Log.Error("Failed to create request", "error", err)
		a.DB.Model(message).Updates(map[string]any{
			"status":        "failed",
			"error_message": "Failed to create request",
		})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		a.Log.Error("Failed to send message", "error", err)
		a.DB.Model(message).Updates(map[string]any{
			"status":        "failed",
			"error_message": err.Error(),
		})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &errResp); err != nil {
			a.Log.Warn("Failed to parse error response", "error", err)
		}
		a.Log.Error("WhatsApp API error",
			"status", resp.StatusCode,
			"code", errResp.Error.Code,
			"message", errResp.Error.Message,
		)
		a.DB.Model(message).Updates(map[string]any{
			"status":        "failed",
			"error_message": errResp.Error.Message,
		})
		return
	}

	var result struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		a.Log.Warn("Failed to parse success response", "error", err)
	}

	if len(result.Messages) > 0 {
		a.DB.Model(message).Updates(map[string]any{
			"status":               "sent",
			"whats_app_message_id": result.Messages[0].ID,
		})
		a.Log.Info("Message sent successfully", "message_id", result.Messages[0].ID, "to", contact.PhoneNumber)

		// Dispatch webhook for message sent
		var sentByUserID string
		if message.SentByUserID != nil {
			sentByUserID = message.SentByUserID.String()
		}
		a.DispatchWebhook(account.OrganizationID, EventMessageSent, MessageEventData{
			MessageID:       message.ID.String(),
			ContactID:       contact.ID.String(),
			ContactPhone:    contact.PhoneNumber,
			ContactName:     contact.ProfileName,
			MessageType:     message.MessageType,
			Content:         message.Content,
			WhatsAppAccount: account.Name,
			Direction:       "outgoing",
			SentByUserID:    sentByUserID,
		})
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// SendMediaMessage sends a media message (image, document, video, audio) to a contact
func (a *App) SendMediaMessage(r *fastglue.Request) error {
	orgID := r.RequestCtx.UserValue("organization_id").(uuid.UUID)
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	userRole, _ := r.RequestCtx.UserValue("role").(string)

	// Parse multipart form
	form, err := r.RequestCtx.MultipartForm()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid multipart form", nil, "")
	}

	// Get contact ID from form
	contactIDValues := form.Value["contact_id"]
	if len(contactIDValues) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "contact_id is required", nil, "")
	}
	contactID, err := uuid.Parse(contactIDValues[0])
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact ID", nil, "")
	}

	// Get media type (image, document, video, audio)
	mediaType := "image"
	if typeValues := form.Value["type"]; len(typeValues) > 0 {
		mediaType = typeValues[0]
	}

	// Get caption (optional)
	caption := ""
	if captionValues := form.Value["caption"]; len(captionValues) > 0 {
		caption = captionValues[0]
	}

	// Get uploaded file
	files := form.File["file"]
	if len(files) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "file is required", nil, "")
	}
	fileHeader := files[0]

	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to read file", nil, "")
	}
	defer func() { _ = file.Close() }()

	// Read file data
	fileData, err := io.ReadAll(file)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to read file data", nil, "")
	}

	// Get MIME type
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Get contact (agents can only message their assigned contacts)
	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
	if userRole == "agent" {
		query = query.Where("assigned_user_id = ?", userID)
	}
	if err := query.First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if contact.WhatsAppAccount != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", contact.WhatsAppAccount, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
		}
	} else {
		// Get default outgoing account
		if err := a.DB.Where("organization_id = ? AND is_default_outgoing = ?", orgID, true).First(&account).Error; err != nil {
			if err := a.DB.Where("organization_id = ?", orgID).First(&account).Error; err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No WhatsApp account configured", nil, "")
			}
		}
	}

	// Save file locally
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	// Save locally first
	localPath, err := a.saveMediaLocally(fileData, mimeType, fileHeader.Filename)
	if err != nil {
		a.Log.Error("Failed to save media locally", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save media", nil, "")
	}

	// Create message record first
	message := models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: account.Name,
		ContactID:       contactID,
		Direction:       "outgoing",
		MessageType:     mediaType,
		Content:         caption,
		MediaURL:        localPath,
		MediaMimeType:   mimeType,
		MediaFilename:   fileHeader.Filename,
		Status:          "pending",
		SentByUserID:    &userID,
	}

	if err := a.DB.Create(&message).Error; err != nil {
		a.Log.Error("Failed to create message", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create message", nil, "")
	}

	// Upload to WhatsApp and send asynchronously
	go a.uploadAndSendMediaMessage(waAccount, &account, &contact, &message, fileData, mimeType, fileHeader.Filename, caption)

	// Update contact's last message
	now := time.Now()
	preview := "[" + mediaType + "]"
	if caption != "" {
		preview = caption
		if len(preview) > 100 {
			preview = preview[:97] + "..."
		}
	}
	a.DB.Model(&contact).Updates(map[string]any{
		"last_message_at":      now,
		"last_message_preview": preview,
	})

	response := MessageResponse{
		ID:            message.ID,
		ContactID:     message.ContactID,
		Direction:     message.Direction,
		MessageType:   message.MessageType,
		Content:       map[string]string{"body": message.Content},
		MediaURL:      message.MediaURL,
		MediaMimeType: message.MediaMimeType,
		MediaFilename: message.MediaFilename,
		Status:        message.Status,
		CreatedAt:     message.CreatedAt,
		UpdatedAt:     message.UpdatedAt,
	}

	// Broadcast new outgoing message via WebSocket
	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type: websocket.TypeNewMessage,
			Payload: map[string]any{
				"id":              message.ID.String(),
				"contact_id":     message.ContactID.String(),
				"direction":      message.Direction,
				"message_type":   message.MessageType,
				"content":        map[string]string{"body": message.Content},
				"media_url":      message.MediaURL,
				"media_mime_type": message.MediaMimeType,
				"media_filename": message.MediaFilename,
				"status":         message.Status,
				"created_at":     message.CreatedAt,
				"updated_at":     message.UpdatedAt,
			},
		})
	}

	return r.SendEnvelope(response)
}

// saveMediaLocally saves media data to local storage and returns the relative path
func (a *App) saveMediaLocally(data []byte, mimeType, filename string) (string, error) {
	// Determine subdirectory based on MIME type
	var subdir string
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		subdir = "images"
	case strings.HasPrefix(mimeType, "video/"):
		subdir = "videos"
	case strings.HasPrefix(mimeType, "audio/"):
		subdir = "audio"
	default:
		subdir = "documents"
	}

	// Ensure directory exists
	if err := a.ensureMediaDir(subdir); err != nil {
		return "", fmt.Errorf("failed to create media directory: %w", err)
	}

	// Get extension from MIME type or filename
	ext := getExtensionFromMimeType(mimeType)
	if ext == "" {
		// Try to get from filename
		if dotIdx := strings.LastIndex(filename, "."); dotIdx >= 0 {
			ext = filename[dotIdx:]
		} else {
			ext = ".bin"
		}
	}

	// Generate unique filename
	newFilename := uuid.New().String() + ext
	filePath := filepath.Join(a.getMediaStoragePath(), subdir, newFilename)

	// Save file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save media file: %w", err)
	}

	// Return relative path
	relativePath := filepath.Join(subdir, newFilename)
	a.Log.Info("Media saved locally", "path", relativePath, "size", len(data))

	return relativePath, nil
}

// SendReactionRequest represents a request to send a reaction
type SendReactionRequest struct {
	Emoji string `json:"emoji"` // Empty string to remove reaction
}

// SendReaction sends a reaction to a message
func (a *App) SendReaction(r *fastglue.Request) error {
	orgID := r.RequestCtx.UserValue("organization_id").(uuid.UUID)
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	userRole, _ := r.RequestCtx.UserValue("role").(string)
	contactIDStr := r.RequestCtx.UserValue("id").(string)
	messageIDStr := r.RequestCtx.UserValue("message_id").(string)

	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact ID", nil, "")
	}

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid message ID", nil, "")
	}

	// Parse request body
	var req SendReactionRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Get contact (agents can only react to messages in their assigned contacts)
	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
	if userRole == "agent" {
		query = query.Where("assigned_user_id = ?", userID)
	}
	if err := query.First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Get message
	var message models.Message
	if err := a.DB.Where("id = ? AND contact_id = ?", messageID, contactID).First(&message).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Message not found", nil, "")
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if contact.WhatsAppAccount != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", contact.WhatsAppAccount, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
		}
	} else {
		if err := a.DB.Where("organization_id = ? AND is_default_outgoing = ?", orgID, true).First(&account).Error; err != nil {
			if err := a.DB.Where("organization_id = ?", orgID).First(&account).Error; err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No WhatsApp account configured", nil, "")
			}
		}
	}

	// Parse existing reactions from Metadata
	var metadata map[string]interface{}
	if message.Metadata != nil {
		metadata = message.Metadata
	} else {
		metadata = make(map[string]interface{})
	}

	// Get or initialize reactions array
	type Reaction struct {
		Emoji     string `json:"emoji"`
		FromPhone string `json:"from_phone,omitempty"`
		FromUser  string `json:"from_user,omitempty"`
	}
	var reactions []Reaction
	if reactionsRaw, ok := metadata["reactions"]; ok {
		if reactionsArray, ok := reactionsRaw.([]interface{}); ok {
			for _, r := range reactionsArray {
				if rMap, ok := r.(map[string]interface{}); ok {
					emoji, _ := rMap["emoji"].(string)
					fromPhone, _ := rMap["from_phone"].(string)
					fromUser, _ := rMap["from_user"].(string)
					reactions = append(reactions, Reaction{
						Emoji:     emoji,
						FromPhone: fromPhone,
						FromUser:  fromUser,
					})
				}
			}
		}
	}

	// Remove existing reaction from this user (each user can only have one reaction)
	userIDStr := userID.String()
	var newReactions []Reaction
	for _, r := range reactions {
		if r.FromUser != userIDStr {
			newReactions = append(newReactions, r)
		}
	}

	// Add new reaction if emoji is not empty
	if req.Emoji != "" {
		newReactions = append(newReactions, Reaction{
			Emoji:    req.Emoji,
			FromUser: userIDStr,
		})
	}

	// Update metadata
	metadata["reactions"] = newReactions
	if err := a.DB.Model(&message).Update("metadata", metadata).Error; err != nil {
		a.Log.Error("Failed to update message reactions", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update reaction", nil, "")
	}

	// Send reaction to WhatsApp API
	go a.sendWhatsAppReaction(&account, &contact, &message, req.Emoji)

	// Broadcast via WebSocket
	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type: "reaction_update",
			Payload: map[string]any{
				"message_id": message.ID.String(),
				"contact_id": contact.ID.String(),
				"reactions":  newReactions,
			},
		})
	}

	return r.SendEnvelope(map[string]any{
		"message_id": message.ID.String(),
		"reactions":  newReactions,
	})
}

// sendWhatsAppReaction sends a reaction to WhatsApp
func (a *App) sendWhatsAppReaction(account *models.WhatsAppAccount, contact *models.Contact, message *models.Message, emoji string) {
	if message.WhatsAppMessageID == "" {
		a.Log.Warn("Cannot send reaction - message has no WhatsApp ID", "message_id", message.ID)
		return
	}

	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", account.APIVersion, account.PhoneID)

	payload := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                contact.PhoneNumber,
		"type":              "reaction",
		"reaction": map[string]any{
			"message_id": message.WhatsAppMessageID,
			"emoji":      emoji, // Empty string removes the reaction
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		a.Log.Error("Failed to marshal reaction payload", "error", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		a.Log.Error("Failed to create reaction request", "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		a.Log.Error("Failed to send reaction", "error", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		a.Log.Error("WhatsApp API reaction error", "status", resp.StatusCode, "body", string(body))
		return
	}

	a.Log.Info("Reaction sent successfully", "message_id", message.WhatsAppMessageID, "emoji", emoji)
}

// uploadAndSendMediaMessage uploads media to WhatsApp and sends the message
func (a *App) uploadAndSendMediaMessage(waAccount *whatsapp.Account, account *models.WhatsAppAccount, contact *models.Contact, message *models.Message, data []byte, mimeType, filename, caption string) {
	ctx := context.Background()

	// Upload media to WhatsApp
	mediaID, err := a.WhatsApp.UploadMedia(ctx, waAccount, data, mimeType, filename)
	if err != nil {
		a.Log.Error("Failed to upload media to WhatsApp", "error", err)
		a.DB.Model(message).Updates(map[string]any{
			"status":        "failed",
			"error_message": "Failed to upload media: " + err.Error(),
		})
		return
	}

	// Send the media message
	var wamID string
	switch message.MessageType {
	case "image":
		wamID, err = a.WhatsApp.SendImageMessage(ctx, waAccount, contact.PhoneNumber, mediaID, caption)
	case "document":
		wamID, err = a.WhatsApp.SendDocumentMessage(ctx, waAccount, contact.PhoneNumber, mediaID, filename, caption)
	case "video":
		wamID, err = a.WhatsApp.SendVideoMessage(ctx, waAccount, contact.PhoneNumber, mediaID, caption)
	case "audio":
		wamID, err = a.WhatsApp.SendAudioMessage(ctx, waAccount, contact.PhoneNumber, mediaID)
	default:
		err = fmt.Errorf("unsupported media type: %s", message.MessageType)
	}

	if err != nil {
		a.Log.Error("Failed to send media message", "error", err)
		a.DB.Model(message).Updates(map[string]any{
			"status":        "failed",
			"error_message": err.Error(),
		})
		return
	}

	// Update message with WhatsApp message ID
	a.DB.Model(message).Updates(map[string]any{
		"status":               "sent",
		"whats_app_message_id": wamID,
	})

	a.Log.Info("Media message sent", "message_id", message.ID, "wamid", wamID, "type", message.MessageType)

	// Dispatch webhook for message sent
	var sentByUserID string
	if message.SentByUserID != nil {
		sentByUserID = message.SentByUserID.String()
	}
	a.DispatchWebhook(account.OrganizationID, EventMessageSent, MessageEventData{
		MessageID:       message.ID.String(),
		ContactID:       contact.ID.String(),
		ContactPhone:    contact.PhoneNumber,
		ContactName:     contact.ProfileName,
		MessageType:     message.MessageType,
		Content:         caption,
		WhatsAppAccount: account.Name,
		Direction:       "outgoing",
		SentByUserID:    sentByUserID,
	})
}
