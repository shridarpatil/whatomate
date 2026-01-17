package handlers

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// OrganizationSettings represents the settings structure
type OrganizationSettings struct {
	MaskPhoneNumbers bool   `json:"mask_phone_numbers"`
	Timezone         string `json:"timezone"`
	DateFormat       string `json:"date_format"`
}

// GetOrganizationSettings returns the organization settings
func (a *App) GetOrganizationSettings(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	// Parse settings from JSONB
	settings := OrganizationSettings{
		MaskPhoneNumbers: false,
		Timezone:         "UTC",
		DateFormat:       "YYYY-MM-DD",
	}

	if org.Settings != nil {
		if v, ok := org.Settings["mask_phone_numbers"].(bool); ok {
			settings.MaskPhoneNumbers = v
		}
		if v, ok := org.Settings["timezone"].(string); ok && v != "" {
			settings.Timezone = v
		}
		if v, ok := org.Settings["date_format"].(string); ok && v != "" {
			settings.DateFormat = v
		}
	}

	return r.SendEnvelope(map[string]interface{}{
		"settings": settings,
		"name":     org.Name,
	})
}

// UpdateOrganizationSettings updates the organization settings
func (a *App) UpdateOrganizationSettings(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req struct {
		MaskPhoneNumbers *bool   `json:"mask_phone_numbers"`
		Timezone         *string `json:"timezone"`
		DateFormat       *string `json:"date_format"`
		Name             *string `json:"name"`
	}

	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	// Update settings
	if org.Settings == nil {
		org.Settings = models.JSONB{}
	}

	if req.MaskPhoneNumbers != nil {
		org.Settings["mask_phone_numbers"] = *req.MaskPhoneNumbers
	}
	if req.Timezone != nil {
		org.Settings["timezone"] = *req.Timezone
	}
	if req.DateFormat != nil {
		org.Settings["date_format"] = *req.DateFormat
	}
	if req.Name != nil && *req.Name != "" {
		org.Name = *req.Name
	}

	if err := a.DB.Save(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update settings", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{
		"message": "Settings updated successfully",
	})
}

// MaskPhoneNumber masks a phone number showing only last 4 digits
func MaskPhoneNumber(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	masked := ""
	for i := 0; i < len(phone)-4; i++ {
		masked += "*"
	}
	return masked + phone[len(phone)-4:]
}

// LooksLikePhoneNumber checks if a string looks like a phone number
// (mostly digits, optionally with common phone formatting characters)
func LooksLikePhoneNumber(s string) bool {
	if len(s) < 7 {
		return false
	}
	digitCount := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			digitCount++
		}
	}
	// If at least 7 digits and more than 70% of the string is digits
	return digitCount >= 7 && float64(digitCount)/float64(len(s)) > 0.7
}

// MaskIfPhoneNumber masks a string if it looks like a phone number
func MaskIfPhoneNumber(s string) string {
	if LooksLikePhoneNumber(s) {
		return MaskPhoneNumber(s)
	}
	return s
}

// ShouldMaskPhoneNumbers checks if phone masking is enabled for the organization
func (a *App) ShouldMaskPhoneNumbers(orgID interface{}) bool {
	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return false
	}

	if org.Settings != nil {
		if v, ok := org.Settings["mask_phone_numbers"].(bool); ok {
			return v
		}
	}
	return false
}

// OrganizationResponse represents an organization in API responses
type OrganizationResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug,omitempty"`
	CreatedAt string    `json:"created_at"`
}

// ListOrganizations returns all organizations (super admin only)
func (a *App) ListOrganizations(r *fastglue.Request) error {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Only super admins can list all organizations
	if !a.IsSuperAdmin(userID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Only super admins can access all organizations", nil, "")
	}

	var orgs []models.Organization
	if err := a.DB.Order("name ASC").Find(&orgs).Error; err != nil {
		a.Log.Error("Failed to list organizations", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list organizations", nil, "")
	}

	response := make([]OrganizationResponse, len(orgs))
	for i, org := range orgs {
		response[i] = OrganizationResponse{
			ID:        org.ID,
			Name:      org.Name,
			Slug:      org.Slug,
			CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return r.SendEnvelope(map[string]any{
		"organizations": response,
	})
}

// GetCurrentOrganization returns the current user's organization details
func (a *App) GetCurrentOrganization(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	return r.SendEnvelope(OrganizationResponse{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}
