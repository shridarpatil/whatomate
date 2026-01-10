package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// Stub handlers - not yet implemented

// Contact handlers
func (a *App) CreateContact(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}

func (a *App) UpdateContact(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}

func (a *App) DeleteContact(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}

// AssignContactRequest represents the request to assign a contact to a user
type AssignContactRequest struct {
	UserID *uuid.UUID `json:"user_id"` // nil to unassign
}

// AssignContact assigns a contact to a user (agent)
// Only admin and manager can assign contacts
func (a *App) AssignContact(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Only admin and manager can assign contacts
	role, _ := r.RequestCtx.UserValue("role").(models.Role)
	if role == models.RoleAgent {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Only admin and manager can assign contacts", nil, "")
	}

	contactIDStr := r.RequestCtx.UserValue("id").(string)
	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact ID", nil, "")
	}

	var req AssignContactRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Get contact
	var contact models.Contact
	if err := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID).First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// If assigning to a user, verify they exist in the same org
	if req.UserID != nil {
		var user models.User
		if err := a.DB.Where("id = ? AND organization_id = ?", req.UserID, orgID).First(&user).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "User not found", nil, "")
		}
	}

	// Update contact assignment
	if err := a.DB.Model(&contact).Update("assigned_user_id", req.UserID).Error; err != nil {
		a.Log.Error("Failed to assign contact", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to assign contact", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"message":          "Contact assigned successfully",
		"assigned_user_id": req.UserID,
	})
}

// Message handlers
func (a *App) SendTemplateMessage(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}

// SendMediaMessage is implemented in contacts.go

func (a *App) MarkMessageRead(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}

// Analytics handlers
func (a *App) GetMessageAnalytics(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}

func (a *App) GetChatbotAnalytics(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}
