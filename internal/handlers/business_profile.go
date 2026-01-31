package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// GetBusinessProfile returns the business profile for a WhatsApp account
func (a *App) GetBusinessProfile(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid account ID", nil, "")
	}

	var account models.WhatsAppAccount
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Account not found", nil, "")
	}

	// Create a context for the request
	ctx := r.RequestCtx

	// Call the WhatsApp client
	profile, err := a.WhatsApp.GetBusinessProfile(ctx, a.toWhatsAppAccount(&account))
	if err != nil {
		a.Log.Error("Failed to get business profile", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to get business profile: "+err.Error(), nil, "")
	}

	return r.SendEnvelope(profile)
}

// UpdateBusinessProfile updates the business profile for a WhatsApp account
func (a *App) UpdateBusinessProfile(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid account ID", nil, "")
	}

	var account models.WhatsAppAccount
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Account not found", nil, "")
	}

	var input whatsapp.BusinessProfileInput
	if err := r.Decode(&input, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	ctx := r.RequestCtx
	waAccount := a.toWhatsAppAccount(&account)

	if err := a.WhatsApp.UpdateBusinessProfile(ctx, waAccount, input); err != nil {
		a.Log.Error("Failed to update business profile", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update business profile: "+err.Error(), nil, "")
	}

	// Re-fetch to ensure we have the latest state
	profile, err := a.WhatsApp.GetBusinessProfile(ctx, waAccount)
	if err != nil {
		// If re-fetch fails, just return success message
		return r.SendEnvelope(map[string]string{"message": "Profile updated successfully"})
	}

	return r.SendEnvelope(profile)
}

// UpdateProfilePicture handles the profile picture upload
func (a *App) UpdateProfilePicture(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid account ID", nil, "")
	}

	var account models.WhatsAppAccount
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Account not found", nil, "")
	}

	// 1. Get the file from request
	fileHeader, err := r.RequestCtx.FormFile("file")
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Missing file", nil, "")
	}

	// 2. Open and read file
	file, err := fileHeader.Open()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to open file", nil, "")
	}
	defer file.Close()

	fileSize := fileHeader.Size
	fileContent := make([]byte, fileSize)
	_, err = file.Read(fileContent)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to read file", nil, "")
	}

	ctx := r.RequestCtx
	waAccount := a.toWhatsAppAccount(&account)

	// Upload to Meta to get handle
	handle, err := a.WhatsApp.UploadProfilePicture(ctx, waAccount, fileContent, fileHeader.Header.Get("Content-Type"))
	if err != nil {
		a.Log.Error("Failed to upload profile picture", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to upload to Meta: "+err.Error(), nil, "")
	}

	// Update Business Profile with the handle
	input := whatsapp.BusinessProfileInput{
		MessagingProduct:     "whatsapp",
		ProfilePictureHandle: handle,
	}

	err = a.WhatsApp.UpdateBusinessProfile(ctx, waAccount, input)

	if err != nil {
		a.Log.Error("Failed to update profile request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Uploaded but failed to set profile: "+err.Error(), nil, "")
	}

	return r.SendEnvelope(map[string]string{
		"message": "Profile picture updated successfully",
		"handle":  handle,
	})
}
