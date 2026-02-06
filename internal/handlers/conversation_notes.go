package handlers

import (
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// ConversationNoteRequest represents the request body for creating/updating a note.
type ConversationNoteRequest struct {
	Content string `json:"content"`
}

// ConversationNoteResponse represents the API response for a conversation note.
type ConversationNoteResponse struct {
	ID            uuid.UUID `json:"id"`
	ContactID     uuid.UUID `json:"contact_id"`
	CreatedByID   uuid.UUID `json:"created_by_id"`
	CreatedByName string    `json:"created_by_name"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ListConversationNotes returns paginated notes for a contact (latest at bottom).
func (a *App) ListConversationNotes(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceChat, models.ActionRead); err != nil {
		return nil
	}

	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	pg := parsePaginationWithDefaults(r, 30, 100)
	limit := pg.Limit

	query := a.DB.Where("organization_id = ? AND contact_id = ?", orgID, contactID)

	// Get total count
	var total int64
	query.Model(&models.ConversationNote{}).Count(&total)

	// Cursor-based pagination: load notes before a specific ID
	beforeIDStr := string(r.RequestCtx.QueryArgs().Peek("before"))
	if beforeIDStr != "" {
		beforeID, err := uuid.Parse(beforeIDStr)
		if err == nil {
			var beforeNote models.ConversationNote
			if err := a.DB.Where("id = ?", beforeID).First(&beforeNote).Error; err == nil {
				query = query.Where("created_at < ?", beforeNote.CreatedAt)
			}
		}
	}

	// Fetch DESC then reverse to get chronological order (oldest first, latest last)
	var notes []models.ConversationNote
	if err := query.Preload("CreatedBy").
		Order("created_at DESC").
		Limit(limit).
		Find(&notes).Error; err != nil {
		a.Log.Error("Failed to list conversation notes", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to list notes", nil, "")
	}

	// Reverse to chronological order (oldest first)
	for i, j := 0, len(notes)-1; i < j; i, j = i+1, j-1 {
		notes[i], notes[j] = notes[j], notes[i]
	}

	result := make([]ConversationNoteResponse, len(notes))
	for i, n := range notes {
		result[i] = noteToResponse(n)
	}

	return r.SendEnvelope(map[string]any{
		"notes":    result,
		"total":    total,
		"has_more": len(notes) == limit,
	})
}

// CreateConversationNote creates a new note on a contact.
func (a *App) CreateConversationNote(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceChat, models.ActionWrite); err != nil {
		return nil
	}

	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	var req ConversationNoteRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Content == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "content is required", nil, "")
	}

	note := models.ConversationNote{
		OrganizationID: orgID,
		ContactID:      contactID,
		CreatedByID:    userID,
		Content:        req.Content,
	}

	if err := a.DB.Create(&note).Error; err != nil {
		a.Log.Error("Failed to create conversation note", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create note", nil, "")
	}

	// Load the creator relation for the response
	var user models.User
	a.DB.First(&user, "id = ?", userID)
	note.CreatedBy = &user

	resp := noteToResponse(note)

	// Broadcast via WebSocket
	if a.WSHub != nil {
		a.WSHub.BroadcastToContact(orgID, contactID, websocket.WSMessage{
			Type:    websocket.TypeConversationNoteCreated,
			Payload: resp,
		})
	}

	return r.SendEnvelope(resp)
}

// UpdateConversationNote updates an existing note (creator only).
func (a *App) UpdateConversationNote(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceChat, models.ActionWrite); err != nil {
		return nil
	}

	_, err = parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	noteID, err := parsePathUUID(r, "note_id", "note")
	if err != nil {
		return nil
	}

	note, err := findByIDAndOrg[models.ConversationNote](a.DB, r, noteID, orgID, "Note")
	if err != nil {
		return nil
	}

	// Only the creator can update their own notes
	if note.CreatedByID != userID {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You can only edit your own notes", nil, "")
	}

	var req ConversationNoteRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Content == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "content is required", nil, "")
	}

	note.Content = req.Content
	if err := a.DB.Save(note).Error; err != nil {
		a.Log.Error("Failed to update conversation note", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to update note", nil, "")
	}

	// Load the creator relation for the response
	var user models.User
	a.DB.First(&user, "id = ?", note.CreatedByID)
	note.CreatedBy = &user

	resp := noteToResponse(*note)

	// Broadcast via WebSocket
	if a.WSHub != nil {
		a.WSHub.BroadcastToContact(orgID, note.ContactID, websocket.WSMessage{
			Type:    websocket.TypeConversationNoteUpdated,
			Payload: resp,
		})
	}

	return r.SendEnvelope(resp)
}

// DeleteConversationNote deletes a note (creator only).
func (a *App) DeleteConversationNote(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceChat, models.ActionWrite); err != nil {
		return nil
	}

	_, err = parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	noteID, err := parsePathUUID(r, "note_id", "note")
	if err != nil {
		return nil
	}

	note, err := findByIDAndOrg[models.ConversationNote](a.DB, r, noteID, orgID, "Note")
	if err != nil {
		return nil
	}

	// Only the creator can delete their own notes
	if note.CreatedByID != userID {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You can only delete your own notes", nil, "")
	}

	contactID := note.ContactID

	if err := a.DB.Delete(note).Error; err != nil {
		a.Log.Error("Failed to delete conversation note", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to delete note", nil, "")
	}

	// Broadcast via WebSocket
	if a.WSHub != nil {
		a.WSHub.BroadcastToContact(orgID, contactID, websocket.WSMessage{
			Type: websocket.TypeConversationNoteDeleted,
			Payload: map[string]any{
				"id":         noteID,
				"contact_id": contactID,
			},
		})
	}

	return r.SendEnvelope(map[string]string{"message": "Note deleted"})
}

func noteToResponse(n models.ConversationNote) ConversationNoteResponse {
	createdByName := ""
	if n.CreatedBy != nil {
		createdByName = n.CreatedBy.FullName
	}
	return ConversationNoteResponse{
		ID:            n.ID,
		ContactID:     n.ContactID,
		CreatedByID:   n.CreatedByID,
		CreatedByName: createdByName,
		Content:       n.Content,
		CreatedAt:     n.CreatedAt,
		UpdatedAt:     n.UpdatedAt,
	}
}
