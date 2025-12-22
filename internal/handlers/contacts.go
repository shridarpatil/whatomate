package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
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
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// MessageResponse represents a message for the frontend
type MessageResponse struct {
	ID              uuid.UUID    `json:"id"`
	ContactID       uuid.UUID    `json:"contact_id"`
	Direction       string       `json:"direction"`
	MessageType     string       `json:"message_type"`
	Content         any          `json:"content"`
	InteractiveData models.JSONB `json:"interactive_data,omitempty"`
	Status          string       `json:"status"`
	WAMID           string       `json:"wamid"`
	Error           string       `json:"error_message"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
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

		response[i] = ContactResponse{
			ID:                 c.ID,
			PhoneNumber:        c.PhoneNumber,
			Name:               c.ProfileName, // Use profile name as name
			ProfileName:        c.ProfileName,
			Status:             "active",
			Tags:               tags,
			CustomFields:       c.Metadata,
			LastMessageAt:      c.LastMessageAt,
			LastMessagePreview: c.LastMessagePreview,
			UnreadCount:        int(unreadCount),
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

	response := ContactResponse{
		ID:                 contact.ID,
		PhoneNumber:        contact.PhoneNumber,
		Name:               contact.ProfileName,
		ProfileName:        contact.ProfileName,
		Status:             "active",
		Tags:               tags,
		CustomFields:       contact.Metadata,
		LastMessageAt:      contact.LastMessageAt,
		LastMessagePreview: contact.LastMessagePreview,
		UnreadCount:        int(unreadCount),
		CreatedAt:          contact.CreatedAt,
		UpdatedAt:          contact.UpdatedAt,
	}

	return r.SendEnvelope(response)
}

// GetMessages returns messages for a contact
// Agents can only access messages for their assigned contacts
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

	// Pagination
	page, _ := strconv.Atoi(string(r.RequestCtx.QueryArgs().Peek("page")))
	limit, _ := strconv.Atoi(string(r.RequestCtx.QueryArgs().Peek("limit")))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	var messages []models.Message
	var total int64

	msgQuery := a.DB.Where("contact_id = ?", contactID)
	msgQuery.Model(&models.Message{}).Count(&total)

	// For chat, we want the most recent messages
	// Fetch in DESC order (newest first), then reverse for display
	// Calculate offset from the end for pagination
	offset := int(total) - (page * limit)
	if offset < 0 {
		limit = limit + offset // Adjust limit if we're on the last page
		offset = 0
	}

	if err := msgQuery.Order("created_at ASC").Offset(offset).Limit(limit).Find(&messages).Error; err != nil {
		a.Log.Error("Failed to list messages", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list messages", nil, "")
	}

	// Convert to response format
	response := make([]MessageResponse, len(messages))
	for i, m := range messages {
		// Parse content as JSON if it's text
		var content any
		if m.MessageType == "text" {
			content = map[string]string{"body": m.Content}
		} else {
			content = map[string]string{"body": m.Content}
		}

		response[i] = MessageResponse{
			ID:              m.ID,
			ContactID:       m.ContactID,
			Direction:       m.Direction,
			MessageType:     m.MessageType,
			Content:         content,
			InteractiveData: m.InteractiveData,
			Status:          m.Status,
			WAMID:           m.WhatsAppMessageID,
			Error:           m.ErrorMessage,
			CreatedAt:       m.CreatedAt,
			UpdatedAt:       m.UpdatedAt,
		}
	}

	// Mark incoming messages as read
	a.DB.Model(&models.Message{}).
		Where("contact_id = ? AND direction = ?", contactID, "incoming").
		Update("status", "read")

	// Update contact read status
	a.DB.Model(&contact).Update("is_read", true)

	return r.SendEnvelope(map[string]any{
		"messages": response,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// SendMessageRequest represents a send message request
type SendMessageRequest struct {
	Type    string `json:"type"`
	Content struct {
		Body string `json:"body"`
	} `json:"content"`
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
		CreatedAt:   message.CreatedAt,
		UpdatedAt:   message.UpdatedAt,
	}

	// Broadcast new outgoing message via WebSocket
	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type: websocket.TypeNewMessage,
			Payload: map[string]any{
				"id":           message.ID,
				"contact_id":   message.ContactID,
				"direction":    message.Direction,
				"message_type": message.MessageType,
				"content":      map[string]string{"body": message.Content},
				"status":       message.Status,
				"created_at":   message.CreatedAt,
				"updated_at":   message.UpdatedAt,
			},
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
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		json.Unmarshal(body, &errResp)
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
	json.Unmarshal(body, &result)

	if len(result.Messages) > 0 {
		a.DB.Model(message).Updates(map[string]any{
			"status":              "sent",
			"whatsapp_message_id": result.Messages[0].ID,
		})
		a.Log.Info("Message sent successfully", "message_id", result.Messages[0].ID, "to", contact.PhoneNumber)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
