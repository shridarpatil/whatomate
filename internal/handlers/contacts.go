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
	Metadata           any        `json:"metadata"`
	LastMessageAt      *time.Time `json:"last_message_at"`
	LastMessagePreview string     `json:"last_message_preview"`
	UnreadCount        int        `json:"unread_count"`
	AssignedUserID     *uuid.UUID `json:"assigned_user_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// MessageResponse represents a message for the frontend
type MessageResponse struct {
	ID               uuid.UUID            `json:"id"`
	ContactID        uuid.UUID            `json:"contact_id"`
	Direction        models.Direction     `json:"direction"`
	MessageType      models.MessageType   `json:"message_type"`
	Content          any                  `json:"content"`
	MediaURL         string               `json:"media_url,omitempty"`
	MediaMimeType    string               `json:"media_mime_type,omitempty"`
	MediaFilename    string               `json:"media_filename,omitempty"`
	InteractiveData  models.JSONB         `json:"interactive_data,omitempty"`
	Status           models.MessageStatus `json:"status"`
	WAMID            string               `json:"wamid"`
	Error            string               `json:"error_message"`
	IsReply          bool                 `json:"is_reply"`
	ReplyToMessageID *string              `json:"reply_to_message_id,omitempty"`
	ReplyToMessage   *ReplyPreview        `json:"reply_to_message,omitempty"`
	Reactions        []ReactionInfo       `json:"reactions,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

// ReplyPreview contains a preview of the replied-to message
type ReplyPreview struct {
	ID          string             `json:"id"`
	Content     any                `json:"content"`
	MessageType models.MessageType `json:"message_type"`
	Direction   models.Direction   `json:"direction"`
}

// ReactionInfo represents a reaction on a message
type ReactionInfo struct {
	Emoji     string `json:"emoji"`
	FromPhone string `json:"from_phone,omitempty"`
	FromUser  string `json:"from_user,omitempty"`
}

// ListContacts returns all contacts for the organization
// Users without contacts:read permission only see contacts assigned to them
func (a *App) ListContacts(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Pagination
	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))
	tagsParam := string(r.RequestCtx.QueryArgs().Peek("tags"))

	var contacts []models.Contact
	query := a.ScopeToOrg(a.DB, userID, orgID)

	// Users without contacts:read permission can only see contacts assigned to them
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
		query = query.Where("assigned_user_id = ?", userID)
	}

	if search != "" {
		// Limit search string length to prevent abuse
		if len(search) > 1000 {
			search = search[:1000]
		}
		searchPattern := "%" + search + "%"
		// Use ILIKE for case-insensitive search on profile_name
		query = query.Where("phone_number LIKE ? OR profile_name ILIKE ?", searchPattern, searchPattern)
	}

	// Filter by tags (comma-separated, matches contacts that have ANY of the specified tags)
	if tagsParam != "" {
		tagList := strings.Split(tagsParam, ",")
		// Trim whitespace from each tag and build OR conditions
		// Using @> operator which leverages the GIN index on tags
		conditions := make([]string, 0, len(tagList))
		args := make([]any, 0, len(tagList))
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				// Use proper JSONB containment with explicit cast
				conditions = append(conditions, "tags @> ?::jsonb")
				tagJSON, _ := json.Marshal([]string{tag})
				args = append(args, string(tagJSON))
			}
		}
		if len(conditions) > 0 {
			query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
		}
	}

	// Order by last message time (most recent first)
	query = query.Order("last_message_at DESC NULLS LAST, created_at DESC")

	var total int64
	query.Model(&models.Contact{}).Count(&total)

	if err := query.Offset(pg.Offset).Limit(pg.Limit).Find(&contacts).Error; err != nil {
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
			Where("contact_id = ? AND direction = ? AND status != ?", c.ID, models.DirectionIncoming, models.MessageStatusRead).
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
			Metadata:           c.Metadata,
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
		"page":     pg.Page,
		"limit":    pg.Limit,
	})
}

// GetContact returns a single contact
// Users without contacts:read permission can only access contacts assigned to them
func (a *App) GetContact(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)

	// Users without contacts:read permission can only access their assigned contacts
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
		query = query.Where("assigned_user_id = ?", userID)
	}

	if err := query.First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Count unread messages
	var unreadCount int64
	a.DB.Model(&models.Message{}).
		Where("contact_id = ? AND direction = ? AND status != ?", contact.ID, models.DirectionIncoming, models.MessageStatusRead).
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
		Metadata:           contact.Metadata,
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
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	hasContactsReadPermission := a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID)

	// Verify contact belongs to org (and to user if no contacts:read permission)
	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
	if !hasContactsReadPermission {
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

	// Check if user without contacts:read should only see current conversation
	if !hasContactsReadPermission {
		settings, err := a.getChatbotSettingsCached(orgID, "")
		if err == nil {
			if settings.AgentAssignment.CurrentConversationOnly {
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
	// Preserve the original limit for the response; adjust a query-specific limit
	// when the remaining messages are fewer than the requested page size.
	responseLimit := limit
	queryLimit := limit
	offset := int(total) - (page * limit)
	if offset < 0 {
		queryLimit = limit + offset // Adjust limit if we're on the last page
		offset = 0
	}

	var messages []models.Message
	if err := msgQuery.Preload("ReplyToMessage").Order("created_at ASC").Offset(offset).Limit(queryLimit).Find(&messages).Error; err != nil {
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
		"limit":    responseLimit,
		"has_more": offset > 0,
	})
}

// buildMessagesResponse converts messages to response format
func (a *App) buildMessagesResponse(messages []models.Message) []MessageResponse {
	response := make([]MessageResponse, len(messages))
	for i, m := range messages {
		var content any
		if m.MessageType == models.MessageTypeText {
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
	a.DB.Where("contact_id = ? AND direction = ? AND status != ?", contactID, models.DirectionIncoming, models.MessageStatusRead).
		Find(&unreadMessages)

	a.DB.Model(&models.Message{}).
		Where("contact_id = ? AND direction = ?", contactID, models.DirectionIncoming).
		Update("status", models.MessageStatusRead)

	a.DB.Model(contact).Update("is_read", true)

	if len(unreadMessages) > 0 && contact.WhatsAppAccount != "" {
		var account models.WhatsAppAccount
		if err := a.DB.Where("organization_id = ? AND name = ?", orgID, contact.WhatsAppAccount).First(&account).Error; err == nil {
			if account.AutoReadReceipt {
				a.wg.Add(1)
				go func() {
					defer a.wg.Done()
					// Use timeout context for external API calls
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()

					waAccount := a.toWhatsAppAccount(&account)
					for _, msg := range unreadMessages {
						// Check if context was cancelled
						if ctx.Err() != nil {
							a.Log.Warn("Read receipt sending cancelled", "reason", ctx.Err())
							return
						}
						if msg.WhatsAppMessageID != "" {
							if err := a.WhatsApp.MarkMessageRead(ctx, waAccount, msg.WhatsAppMessageID); err != nil {
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
	Type    models.MessageType `json:"type"`
	Content struct {
		Body string `json:"body"`
	} `json:"content"`
	ReplyToMessageID string `json:"reply_to_message_id,omitempty"`

	// Interactive message fields (for type="interactive")
	Interactive *InteractiveContent `json:"interactive,omitempty"`
}

// InteractiveContent holds interactive message data
type InteractiveContent struct {
	Type       string           `json:"type"`                  // "button", "list", "cta_url"
	Body       string           `json:"body"`                  // Body text
	Buttons    []ButtonContent  `json:"buttons,omitempty"`     // For button type
	ButtonText string           `json:"button_text,omitempty"` // For cta_url type
	URL        string           `json:"url,omitempty"`         // For cta_url type
}

// ButtonContent represents a button in interactive messages
type ButtonContent struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SendMessage sends a message to a contact
// Agents can only send messages to their assigned contacts
func (a *App) SendMessage(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	// Parse request body
	var req SendMessageRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Get contact (users without full read permission can only message their assigned contacts)
	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
		query = query.Where("assigned_user_id = ?", userID)
	}
	if err := query.First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Get WhatsApp account
	account, err := a.resolveWhatsAppAccount(orgID, contact.WhatsAppAccount)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to resolve WhatsApp account", nil, "")
	}

	// Handle reply context
	var replyToMessage *models.Message
	if req.ReplyToMessageID != "" {
		replyToID, err := uuid.Parse(req.ReplyToMessageID)
		if err == nil {
			var replyTo models.Message
			if err := a.DB.Where("id = ? AND contact_id = ?", replyToID, contactID).First(&replyTo).Error; err == nil {
				replyToMessage = &replyTo
			}
		}
	}

	// Build request and send using unified sender
	msgReq := OutgoingMessageRequest{
		Account:        account,
		Contact:        &contact,
		Type:           req.Type,
		Content:        req.Content.Body,
		ReplyToMessage: replyToMessage,
	}

	// Handle interactive messages
	if req.Type == models.MessageTypeInteractive && req.Interactive != nil {
		msgReq.InteractiveType = req.Interactive.Type
		msgReq.BodyText = req.Interactive.Body
		msgReq.ButtonText = req.Interactive.ButtonText
		msgReq.URL = req.Interactive.URL

		// Convert buttons
		if len(req.Interactive.Buttons) > 0 {
			msgReq.Buttons = make([]whatsapp.Button, len(req.Interactive.Buttons))
			for i, btn := range req.Interactive.Buttons {
				msgReq.Buttons[i] = whatsapp.Button{
					ID:    btn.ID,
					Title: btn.Title,
				}
			}
		}
	}

	opts := DefaultSendOptions()
	opts.SentByUserID = &userID

	ctx := context.Background()
	message, err := a.SendOutgoingMessage(ctx, msgReq, opts)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to send message", nil, "")
	}

	// Build response
	response := MessageResponse{
		ID:              message.ID,
		ContactID:       message.ContactID,
		Direction:       message.Direction,
		MessageType:     message.MessageType,
		Content:         map[string]string{"body": message.Content},
		InteractiveData: message.InteractiveData,
		Status:          message.Status,
		IsReply:         message.IsReply,
		CreatedAt:       message.CreatedAt,
		UpdatedAt:       message.UpdatedAt,
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

	return r.SendEnvelope(response)
}

// resolveWhatsAppAccount gets the WhatsApp account for sending messages
func (a *App) resolveWhatsAppAccount(orgID uuid.UUID, accountName string) (*models.WhatsAppAccount, error) {
	var account models.WhatsAppAccount

	if accountName != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", accountName, orgID).First(&account).Error; err != nil {
			return nil, fmt.Errorf("WhatsApp account not found")
		}
		return &account, nil
	}

	// Get default outgoing account
	if err := a.DB.Where("organization_id = ? AND is_default_outgoing = ?", orgID, true).First(&account).Error; err != nil {
		// Fall back to any account
		if err := a.DB.Where("organization_id = ?", orgID).First(&account).Error; err != nil {
			return nil, fmt.Errorf("no WhatsApp account configured")
		}
	}
	return &account, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// SendMediaMessage sends a media message (image, document, video, audio) to a contact
func (a *App) SendMediaMessage(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

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

	// Get contact (users without full read permission can only message their assigned contacts)
	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
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

	// Save file locally first
	localPath, err := a.saveMediaLocally(fileData, mimeType, fileHeader.Filename)
	if err != nil {
		a.Log.Error("Failed to save media locally", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save media", nil, "")
	}

	// Build and send via unified message sender
	msgReq := OutgoingMessageRequest{
		Account:       &account,
		Contact:       &contact,
		Type:          models.MessageType(mediaType),
		MediaData:     fileData,
		MediaURL:      localPath,
		MediaMimeType: mimeType,
		MediaFilename: fileHeader.Filename,
		Caption:       caption,
	}

	opts := DefaultSendOptions()
	opts.SentByUserID = &userID

	ctx := context.Background()
	message, err := a.SendOutgoingMessage(ctx, msgReq, opts)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to send message", nil, "")
	}

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
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	messageIDStr := r.RequestCtx.UserValue("message_id").(string)

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid message ID", nil, "")
	}

	// Parse request body
	var req SendReactionRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Get contact (users without full read permission can only react to messages in their assigned contacts)
	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
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

	url := fmt.Sprintf("%s/%s/%s/messages", a.Config.WhatsApp.BaseURL, account.APIVersion, account.PhoneID)

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

	resp, err := a.HTTPClient.Do(req)
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

// AssignContactRequest represents the request to assign a contact to a user
type AssignContactRequest struct {
	UserID *uuid.UUID `json:"user_id"` // nil to unassign
}

// AssignContact assigns a contact to a user (agent)
// Only users with write permission can assign contacts
func (a *App) AssignContact(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Only users with write permission can assign contacts
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionWrite, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to assign contacts", nil, "")
	}

	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	var req AssignContactRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Get contact
	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	// If assigning to a user, verify they exist in the same org
	if req.UserID != nil {
		var user models.User
		if err := a.DB.Where("id = ? AND organization_id = ?", req.UserID, orgID).First(&user).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "User not found", nil, "")
		}
	}

	// Update contact assignment
	if err := a.DB.Model(contact).Update("assigned_user_id", req.UserID).Error; err != nil {
		a.Log.Error("Failed to assign contact", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to assign contact", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"message":          "Contact assigned successfully",
		"assigned_user_id": req.UserID,
	})
}

// ContactSessionDataResponse represents the session data for a contact's info panel
type ContactSessionDataResponse struct {
	SessionID   *uuid.UUID     `json:"session_id,omitempty"`
	FlowID      *uuid.UUID     `json:"flow_id,omitempty"`
	FlowName    string         `json:"flow_name,omitempty"`
	SessionData map[string]any `json:"session_data"`
	PanelConfig map[string]any `json:"panel_config"`
}

// GetContactSessionData returns session data and panel configuration for a contact
// Used by the contact info panel in the chat view
func (a *App) GetContactSessionData(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	// Verify contact belongs to org (users without full read permission can only access assigned contacts)
	var contact models.Contact
	query := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID)
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
		query = query.Where("assigned_user_id = ?", userID)
	}
	if err := query.First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	response := ContactSessionDataResponse{
		SessionData: make(map[string]any),
		PanelConfig: map[string]any{"sections": []any{}},
	}

	// Get the most recent completed or active session for this contact
	var session models.ChatbotSession
	err = a.DB.Where("contact_id = ? AND organization_id = ?", contactID, orgID).
		Where("status IN ?", []models.SessionStatus{models.SessionStatusActive, models.SessionStatusCompleted}).
		Order("created_at DESC").
		First(&session).Error

	if err == nil {
		response.SessionID = &session.ID
		response.FlowID = session.CurrentFlowID

		// Get the flow to retrieve panel config
		// First try current_flow_id, then fall back to _flow_id in session_data
		var flowID *uuid.UUID
		if session.CurrentFlowID != nil {
			flowID = session.CurrentFlowID
		} else if flowIDStr, ok := session.SessionData["_flow_id"].(string); ok {
			if parsedID, err := uuid.Parse(flowIDStr); err == nil {
				flowID = &parsedID
			}
		}

		if flowID != nil {
			// Use cached flow to avoid DB query
			flow, err := a.getChatbotFlowByIDCached(orgID, *flowID)
			if err == nil && flow != nil {
				response.FlowName = flow.Name
				response.FlowID = flowID

				// Use panel config directly from flow (it's already JSONB/map)
				if len(flow.PanelConfig) > 0 {
					response.PanelConfig = flow.PanelConfig

					// Only include session data for configured fields (reduce payload)
					if session.SessionData != nil {
						configuredKeys := make(map[string]bool)
						if sections, ok := flow.PanelConfig["sections"].([]any); ok {
							for _, sec := range sections {
								if section, ok := sec.(map[string]any); ok {
									if fields, ok := section["fields"].([]any); ok {
										for _, f := range fields {
											if field, ok := f.(map[string]any); ok {
												if key, ok := field["key"].(string); ok {
													configuredKeys[key] = true
												}
											}
										}
									}
								}
							}
						}
						// Copy only configured fields to response
						for key := range configuredKeys {
							if val, exists := session.SessionData[key]; exists {
								response.SessionData[key] = val
							}
						}
					}
				}
			}
		}
	}

	return r.SendEnvelope(response)
}

// UpdateContactTagsRequest represents the request body for updating contact tags
type UpdateContactTagsRequest struct {
	Tags []string `json:"tags"`
}

// UpdateContactTags updates the tags on a contact
func (a *App) UpdateContactTags(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permission - need contacts:write to update tags on contacts
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionWrite, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to update contact tags", nil, "")
	}

	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	var req UpdateContactTagsRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Get contact
	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	// Convert tags to JSONBArray
	tagsArray := make(models.JSONBArray, len(req.Tags))
	for i, tag := range req.Tags {
		tagsArray[i] = tag
	}

	// Update contact tags
	if err := a.DB.Model(contact).Update("tags", tagsArray).Error; err != nil {
		a.Log.Error("Failed to update contact tags", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update contact tags", nil, "")
	}

	// Reload contact to get updated tags
	if err := a.DB.First(contact, contactID).Error; err != nil {
		a.Log.Error("Failed to reload contact", "error", err)
	}

	// Build response with tag details
	tags := []string{}
	if contact.Tags != nil {
		for _, t := range contact.Tags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	return r.SendEnvelope(map[string]any{
		"message": "Contact tags updated",
		"tags":    tags,
	})
}

// CreateContactRequest represents the request body for creating a contact
type CreateContactRequest struct {
	PhoneNumber     string         `json:"phone_number"`
	ProfileName     string         `json:"profile_name"`
	WhatsAppAccount string         `json:"whatsapp_account"`
	Tags            []string       `json:"tags"`
	Metadata        map[string]any `json:"metadata"`
}

// CreateContact creates a new contact or restores a soft-deleted one
func (a *App) CreateContact(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionWrite, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to create contacts", nil, "")
	}

	var req CreateContactRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.PhoneNumber == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "phone_number is required", nil, "")
	}

	// Normalize phone number
	normalizedPhone := req.PhoneNumber
	if len(normalizedPhone) > 0 && normalizedPhone[0] == '+' {
		normalizedPhone = normalizedPhone[1:]
	}

	// Check if contact exists (including soft-deleted)
	var existingContact models.Contact
	if err := a.DB.Unscoped().Where("organization_id = ? AND phone_number = ?", orgID, normalizedPhone).First(&existingContact).Error; err == nil {
		// Contact exists
		if existingContact.DeletedAt.Valid {
			// Restore soft-deleted contact
			a.DB.Unscoped().Model(&existingContact).Update("deleted_at", nil)
			existingContact.DeletedAt.Valid = false
			// Update fields
			updates := map[string]any{}
			if req.ProfileName != "" {
				updates["profile_name"] = req.ProfileName
			}
			if req.WhatsAppAccount != "" {
				updates["whats_app_account"] = req.WhatsAppAccount
			}
			if req.Tags != nil {
				tagsArray := make(models.JSONBArray, len(req.Tags))
				for i, tag := range req.Tags {
					tagsArray[i] = tag
				}
				updates["tags"] = tagsArray
			}
			if req.Metadata != nil {
				updates["metadata"] = models.JSONB(req.Metadata)
			}
			if len(updates) > 0 {
				a.DB.Model(&existingContact).Updates(updates)
			}
			// Reload contact
			a.DB.First(&existingContact, existingContact.ID)
			return r.SendEnvelope(a.buildContactResponse(&existingContact, orgID))
		}
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Contact with this phone number already exists", nil, "")
	}

	// Create new contact
	contact := models.Contact{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		PhoneNumber:     normalizedPhone,
		ProfileName:     req.ProfileName,
		WhatsAppAccount: req.WhatsAppAccount,
	}

	if req.Tags != nil {
		tagsArray := make(models.JSONBArray, len(req.Tags))
		for i, tag := range req.Tags {
			tagsArray[i] = tag
		}
		contact.Tags = tagsArray
	}

	if req.Metadata != nil {
		contact.Metadata = models.JSONB(req.Metadata)
	}

	if err := a.DB.Create(&contact).Error; err != nil {
		a.Log.Error("Failed to create contact", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create contact", nil, "")
	}

	return r.SendEnvelope(a.buildContactResponse(&contact, orgID))
}

// UpdateContactRequest represents the request body for updating a contact
type UpdateContactRequest struct {
	ProfileName     *string         `json:"profile_name"`
	WhatsAppAccount *string         `json:"whatsapp_account"`
	Tags            []string        `json:"tags"`
	Metadata        *map[string]any `json:"metadata"`
	AssignedUserID  *uuid.UUID      `json:"assigned_user_id"`
}

// UpdateContact updates an existing contact
func (a *App) UpdateContact(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionWrite, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to update contacts", nil, "")
	}

	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	var req UpdateContactRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Get contact
	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	// Build updates map
	updates := map[string]any{}

	if req.ProfileName != nil {
		updates["profile_name"] = *req.ProfileName
	}
	if req.WhatsAppAccount != nil {
		updates["whats_app_account"] = *req.WhatsAppAccount
	}
	if req.Tags != nil {
		tagsArray := make(models.JSONBArray, len(req.Tags))
		for i, tag := range req.Tags {
			tagsArray[i] = tag
		}
		updates["tags"] = tagsArray
	}
	if req.Metadata != nil {
		updates["metadata"] = models.JSONB(*req.Metadata)
	}
	if req.AssignedUserID != nil {
		// Verify user exists in same org
		var user models.User
		if err := a.DB.Where("id = ? AND organization_id = ?", req.AssignedUserID, orgID).First(&user).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Assigned user not found", nil, "")
		}
		updates["assigned_user_id"] = req.AssignedUserID
	}

	if len(updates) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No fields to update", nil, "")
	}

	if err := a.DB.Model(contact).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update contact", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update contact", nil, "")
	}

	// Reload contact
	a.DB.First(contact, contactID)

	return r.SendEnvelope(a.buildContactResponse(contact, orgID))
}

// DeleteContact soft-deletes a contact
func (a *App) DeleteContact(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionDelete, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to delete contacts", nil, "")
	}

	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	// Get contact
	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	// Soft delete the contact
	if err := a.DB.Delete(contact).Error; err != nil {
		a.Log.Error("Failed to delete contact", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete contact", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"message": "Contact deleted successfully",
	})
}

// buildContactResponse creates a ContactResponse from a Contact model
func (a *App) buildContactResponse(contact *models.Contact, orgID uuid.UUID) ContactResponse {
	// Count unread messages
	var unreadCount int64
	a.DB.Model(&models.Message{}).
		Where("contact_id = ? AND direction = ? AND status != ?", contact.ID, models.DirectionIncoming, models.MessageStatusRead).
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

	return ContactResponse{
		ID:                 contact.ID,
		PhoneNumber:        phoneNumber,
		Name:               profileName,
		ProfileName:        profileName,
		Status:             "active",
		Tags:               tags,
		Metadata:           contact.Metadata,
		LastMessageAt:      contact.LastMessageAt,
		LastMessagePreview: contact.LastMessagePreview,
		UnreadCount:        int(unreadCount),
		AssignedUserID:     contact.AssignedUserID,
		CreatedAt:          contact.CreatedAt,
		UpdatedAt:          contact.UpdatedAt,
	}
}
