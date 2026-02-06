package handlers

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/database"
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
	orgID, err := a.getOrgID(r)
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
	orgID, err := a.getOrgID(r)
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

// ListOrganizations returns all organizations (super admin or users with organizations:read)
func (a *App) ListOrganizations(r *fastglue.Request) error {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Super admins or users with organizations:read permission
	if !a.IsSuperAdmin(userID) && !a.HasPermission(userID, models.ResourceOrganizations, models.ActionRead) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
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
	orgID, err := a.getOrgID(r)
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

// CreateOrganizationRequest represents the request body for creating an organization
type CreateOrganizationRequest struct {
	Name string `json:"name"`
}

// CreateOrganization creates a new organization
func (a *App) CreateOrganization(r *fastglue.Request) error {
	_, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceOrganizations, models.ActionWrite); err != nil {
		return nil
	}

	var req CreateOrganizationRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Organization name is required", nil, "")
	}

	// Start transaction
	tx := a.DB.Begin()
	if tx.Error != nil {
		a.Log.Error("Failed to begin transaction", "error", tx.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create organization", nil, "")
	}

	org := models.Organization{
		Name:     req.Name,
		Slug:     generateSlug(req.Name),
		Settings: models.JSONB{},
	}

	if err := tx.Create(&org).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to create organization", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create organization", nil, "")
	}

	// Seed system roles for the new organization
	if err := database.SeedSystemRolesForOrg(tx, org.ID); err != nil {
		tx.Rollback()
		a.Log.Error("Failed to seed system roles", "error", err, "org_id", org.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create organization", nil, "")
	}

	// Create default chatbot settings
	chatbotSettings := models.ChatbotSettings{
		OrganizationID:     org.ID,
		IsEnabled:          false,
		SessionTimeoutMins: 30,
	}
	if err := tx.Create(&chatbotSettings).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to create chatbot settings", "error", err, "org_id", org.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create organization", nil, "")
	}

	// Get admin role for this org and add the creator as admin
	var adminRole models.CustomRole
	if err := tx.Where("organization_id = ? AND name = ? AND is_system = ?", org.ID, "admin", true).First(&adminRole).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to find admin role", "error", err, "org_id", org.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create organization", nil, "")
	}

	userOrg := models.UserOrganization{
		UserID:         userID,
		OrganizationID: org.ID,
		RoleID:         &adminRole.ID,
		IsDefault:      false,
	}
	if err := tx.Create(&userOrg).Error; err != nil {
		tx.Rollback()
		a.Log.Error("Failed to add creator to organization", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create organization", nil, "")
	}

	if err := tx.Commit().Error; err != nil {
		a.Log.Error("Failed to commit transaction", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create organization", nil, "")
	}

	a.Log.Info("Created organization", "org_id", org.ID, "org_name", org.Name, "created_by", userID)

	return r.SendEnvelope(OrganizationResponse{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// MemberResponse represents an organization member in API responses
type MemberResponse struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	RoleID         *uuid.UUID `json:"role_id,omitempty"`
	RoleName       string     `json:"role_name,omitempty"`
	IsDefault      bool       `json:"is_default"`
	Email          string     `json:"email"`
	FullName       string     `json:"full_name"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      string     `json:"created_at"`
}

// ListOrganizationMembers returns all members of the current organization
func (a *App) ListOrganizationMembers(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceOrganizations, models.ActionRead); err != nil {
		return nil
	}

	var userOrgs []models.UserOrganization
	if err := a.DB.Where("organization_id = ?", orgID).
		Preload("User").
		Preload("Role").
		Find(&userOrgs).Error; err != nil {
		a.Log.Error("Failed to list organization members", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list members", nil, "")
	}

	response := make([]MemberResponse, 0, len(userOrgs))
	for _, uo := range userOrgs {
		item := MemberResponse{
			ID:             uo.ID,
			UserID:         uo.UserID,
			OrganizationID: uo.OrganizationID,
			RoleID:         uo.RoleID,
			IsDefault:      uo.IsDefault,
			CreatedAt:      uo.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if uo.User != nil {
			item.Email = uo.User.Email
			item.FullName = uo.User.FullName
			item.IsActive = uo.User.IsActive
		}
		if uo.Role != nil {
			item.RoleName = uo.Role.Name
		}
		response = append(response, item)
	}

	return r.SendEnvelope(map[string]interface{}{
		"members": response,
	})
}

// AddMemberRequest represents the request body for adding a member to an organization
type AddMemberRequest struct {
	UserID uuid.UUID  `json:"user_id"`
	RoleID *uuid.UUID `json:"role_id"`
}

// AddOrganizationMember adds an existing user to the current organization
func (a *App) AddOrganizationMember(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceOrganizations, models.ActionAssign); err != nil {
		return nil
	}

	var req AddMemberRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.UserID == uuid.Nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "user_id is required", nil, "")
	}

	// Validate target user exists
	var targetUser models.User
	if err := a.DB.Where("id = ?", req.UserID).First(&targetUser).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	// Check if already a member
	var existingCount int64
	a.DB.Model(&models.UserOrganization{}).
		Where("user_id = ? AND organization_id = ?", req.UserID, orgID).
		Count(&existingCount)
	if existingCount > 0 {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "User is already a member of this organization", nil, "")
	}

	// Determine role
	var roleID *uuid.UUID
	if req.RoleID != nil {
		// Validate role exists and belongs to org
		var role models.CustomRole
		if err := a.DB.Where("id = ? AND organization_id = ?", req.RoleID, orgID).First(&role).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid role", nil, "")
		}
		roleID = req.RoleID
	} else {
		// Use org's default role
		var defaultRole models.CustomRole
		if err := a.DB.Where("organization_id = ? AND is_default = ?", orgID, true).First(&defaultRole).Error; err == nil {
			roleID = &defaultRole.ID
		}
	}

	userOrg := models.UserOrganization{
		UserID:         req.UserID,
		OrganizationID: orgID,
		RoleID:         roleID,
		IsDefault:      false,
	}

	if err := a.DB.Create(&userOrg).Error; err != nil {
		a.Log.Error("Failed to add organization member", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to add member", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Member added successfully"})
}

// RemoveOrganizationMember removes a user from the current organization
func (a *App) RemoveOrganizationMember(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceOrganizations, models.ActionAssign); err != nil {
		return nil
	}

	targetUserID, err := parsePathUUID(r, "user_id", "user")
	if err != nil {
		return nil
	}

	// Cannot remove self
	if targetUserID == userID {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot remove yourself from the organization", nil, "")
	}

	result := a.DB.Where("user_id = ? AND organization_id = ?", targetUserID, orgID).
		Delete(&models.UserOrganization{})
	if result.Error != nil {
		a.Log.Error("Failed to remove organization member", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to remove member", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Member not found in this organization", nil, "")
	}

	// Invalidate removed user's permission cache
	a.InvalidateUserPermissionsCache(targetUserID)

	return r.SendEnvelope(map[string]string{"message": "Member removed successfully"})
}

// UpdateMemberRoleRequest represents the request body for updating a member's role
type UpdateMemberRoleRequest struct {
	RoleID uuid.UUID `json:"role_id"`
}

// UpdateOrganizationMemberRole updates a member's role in the current organization
func (a *App) UpdateOrganizationMemberRole(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceOrganizations, models.ActionAssign); err != nil {
		return nil
	}

	targetUserID, err := parsePathUUID(r, "user_id", "user")
	if err != nil {
		return nil
	}

	var req UpdateMemberRoleRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.RoleID == uuid.Nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "role_id is required", nil, "")
	}

	// Validate role exists and belongs to org
	var role models.CustomRole
	if err := a.DB.Where("id = ? AND organization_id = ?", req.RoleID, orgID).First(&role).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid role", nil, "")
	}

	// Update the user's role in this org
	result := a.DB.Model(&models.UserOrganization{}).
		Where("user_id = ? AND organization_id = ?", targetUserID, orgID).
		Update("role_id", req.RoleID)
	if result.Error != nil {
		a.Log.Error("Failed to update member role", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update member role", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Member not found in this organization", nil, "")
	}

	// Invalidate permission cache
	a.InvalidateUserPermissionsCache(targetUserID)

	return r.SendEnvelope(map[string]string{"message": "Member role updated successfully"})
}
