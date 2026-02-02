package handlers

import (
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
)

// UserRequest represents the request body for creating/updating a user
type UserRequest struct {
	Email        string     `json:"email"`
	Password     string     `json:"password"`
	FullName     string     `json:"full_name"`
	RoleID       *uuid.UUID `json:"role_id"`
	IsActive     *bool      `json:"is_active"`
	IsSuperAdmin *bool      `json:"is_super_admin"`
}

// UserResponse represents the response for a user (without sensitive data)
type UserResponse struct {
	ID             uuid.UUID    `json:"id"`
	Email          string       `json:"email"`
	FullName       string       `json:"full_name"`
	RoleID         *uuid.UUID   `json:"role_id,omitempty"`
	Role           *RoleInfo    `json:"role,omitempty"`
	IsActive       bool         `json:"is_active"`
	IsAvailable    bool         `json:"is_available"`
	IsSuperAdmin   bool         `json:"is_super_admin"`
	OrganizationID uuid.UUID    `json:"organization_id"`
	Settings       models.JSONB `json:"settings,omitempty"`
	CreatedAt      string       `json:"created_at"`
	UpdatedAt      string       `json:"updated_at"`
}

// PermissionInfo represents permission info in role response
type PermissionInfo struct {
	ID          uuid.UUID `json:"id"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	Description string    `json:"description,omitempty"`
}

// RoleInfo represents role info in user response
type RoleInfo struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	IsSystem    bool             `json:"is_system"`
	Permissions []PermissionInfo `json:"permissions"`
}

// UserSettingsRequest represents notification/settings preferences
type UserSettingsRequest struct {
	EmailNotifications bool `json:"email_notifications"`
	NewMessageAlerts   bool `json:"new_message_alerts"`
	CampaignUpdates    bool `json:"campaign_updates"`
}

// ChangePasswordRequest represents the request body for changing password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ListUsers returns all users for the organization
func (a *App) ListUsers(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceUsers, models.ActionRead); err != nil {
		return nil
	}

	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	baseQuery := a.ScopeToOrg(a.DB, userID, orgID)
	if search != "" {
		baseQuery = baseQuery.Where("full_name ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Get total count
	var total int64
	baseQuery.Model(&models.User{}).Count(&total)

	var users []models.User
	if err := pg.Apply(baseQuery.
		Preload("Role").
		Order("created_at DESC")).
		Find(&users).Error; err != nil {
		a.Log.Error("Failed to list users", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list users", nil, "")
	}

	// Convert to response format (hide sensitive data)
	response := make([]UserResponse, len(users))
	for i, user := range users {
		response[i] = userToResponse(user)
	}

	return r.SendEnvelope(map[string]interface{}{
		"users": response,
		"total": total,
		"page":  pg.Page,
		"limit": pg.Limit,
	})
}

// GetUser returns a single user
func (a *App) GetUser(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "user")
	if err != nil {
		return nil
	}

	var user models.User
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		Preload("Role").
		First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	return r.SendEnvelope(userToResponse(user))
}

// CreateUser creates a new user (admin only)
func (a *App) CreateUser(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceUsers, models.ActionWrite); err != nil {
		return nil
	}

	var req UserRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Email, password, and full_name are required", nil, "")
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
		// No role specified, use default role
		var defaultRole models.CustomRole
		if err := a.DB.Where("organization_id = ? AND is_default = ?", orgID, true).First(&defaultRole).Error; err == nil {
			roleID = &defaultRole.ID
		}
	}

	// Check if email already exists
	var existingUser models.User
	if err := a.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Email already exists", nil, "")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("Failed to hash password", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create user", nil, "")
	}

	user := models.User{
		OrganizationID: orgID,
		Email:          req.Email,
		PasswordHash:   string(hashedPassword),
		FullName:       req.FullName,
		RoleID:         roleID,
		IsActive:       true,
	}

	// Only superadmins can create other superadmins
	if req.IsSuperAdmin != nil && *req.IsSuperAdmin {
		if !a.IsSuperAdmin(userID) {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Only super admins can create super admins", nil, "")
		}
		user.IsSuperAdmin = true
	}

	if err := a.DB.Create(&user).Error; err != nil {
		a.Log.Error("Failed to create user", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create user", nil, "")
	}

	// Load role for response
	a.DB.Preload("Role").First(&user, user.ID)

	return r.SendEnvelope(userToResponse(user))
}

// UpdateUser updates a user
func (a *App) UpdateUser(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	currentUserID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	id, err := parsePathUUID(r, "id", "user")
	if err != nil {
		return nil
	}

	// Users can update themselves, others need users:write permission
	if currentUserID != id && !a.HasPermission(currentUserID, models.ResourceUsers, models.ActionWrite) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).Preload("Role").First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	var req UserRequest
	if err := r.Decode(&req, "json"); err != nil {
		a.Log.Error("UpdateUser: Failed to decode request", "error", err, "body", string(r.RequestCtx.PostBody()))
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Only users with users:write permission can change roles
	if req.RoleID != nil && !a.HasPermission(currentUserID, models.ResourceUsers, models.ActionWrite) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions to change roles", nil, "")
	}

	// Update fields if provided
	if req.Email != "" {
		var existingUser models.User
		if err := a.DB.Where("email = ? AND id != ?", req.Email, id).First(&existingUser).Error; err == nil {
			return r.SendErrorEnvelope(fasthttp.StatusConflict, "Email already exists", nil, "")
		}
		user.Email = req.Email
	}
	if req.FullName != "" {
		user.FullName = req.FullName
	}
	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			a.Log.Error("Failed to hash password", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update user", nil, "")
		}
		user.PasswordHash = string(hashedPassword)
	}

	// Handle role update
	roleChanged := false
	if req.RoleID != nil {
		// Validate role exists and belongs to org
		var newRole models.CustomRole
		if err := a.DB.Where("id = ? AND organization_id = ?", req.RoleID, orgID).First(&newRole).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid role", nil, "")
		}
		// Prevent self-demotion from admin
		if currentUserID == id && user.Role != nil && user.Role.Name == "admin" && newRole.Name != "admin" {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot demote yourself", nil, "")
		}
		if user.RoleID == nil || *user.RoleID != *req.RoleID {
			roleChanged = true
		}
		user.RoleID = req.RoleID
		user.Role = nil // Clear the preloaded role to prevent GORM from using the old association
	}

	if req.IsActive != nil {
		// Prevent user from deactivating themselves
		if currentUserID == id && !*req.IsActive {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot deactivate yourself", nil, "")
		}
		user.IsActive = *req.IsActive
	}

	// Handle super admin update - only superadmins can change this
	if req.IsSuperAdmin != nil {
		if !a.IsSuperAdmin(currentUserID) {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Only super admins can modify super admin status", nil, "")
		}
		// Prevent removing own super admin status
		if currentUserID == id && !*req.IsSuperAdmin && user.IsSuperAdmin {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot remove your own super admin status", nil, "")
		}
		user.IsSuperAdmin = *req.IsSuperAdmin
	}

	if err := a.DB.Save(&user).Error; err != nil {
		a.Log.Error("Failed to update user", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update user", nil, "")
	}

	// Invalidate permissions cache if role changed
	if roleChanged {
		a.InvalidateUserPermissionsCache(user.ID)
	}

	// Load role for response
	a.DB.Preload("Role").First(&user, user.ID)

	return r.SendEnvelope(userToResponse(user))
}

// DeleteUser deletes a user
func (a *App) DeleteUser(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	currentUserID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !a.HasPermission(currentUserID, models.ResourceUsers, models.ActionDelete) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
	}

	id, err := parsePathUUID(r, "id", "user")
	if err != nil {
		return nil
	}

	// Prevent user from deleting themselves
	if currentUserID == id {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot delete yourself", nil, "")
	}

	// Check if user exists
	var user models.User
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).Preload("Role").First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	// Check if this is the last admin (user with admin role)
	if user.Role != nil && user.Role.Name == "admin" {
		var adminRole models.CustomRole
		if err := a.DB.Where("organization_id = ? AND name = ? AND is_system = ?", orgID, "admin", true).First(&adminRole).Error; err == nil {
			var adminCount int64
			a.DB.Model(&models.User{}).Where("organization_id = ? AND role_id = ?", orgID, adminRole.ID).Count(&adminCount)
			if adminCount <= 1 {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot delete the last admin", nil, "")
			}
		}
	}

	result := a.DB.Where("id = ? AND organization_id = ?", id, orgID).Delete(&models.User{})
	if result.Error != nil {
		a.Log.Error("Failed to delete user", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete user", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "User deleted successfully"})
}

// GetCurrentUser returns the current authenticated user's details
func (a *App) GetCurrentUser(r *fastglue.Request) error {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", userID).
		Preload("Role").
		First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	// Load permissions from cache
	if user.Role != nil && user.RoleID != nil {
		cachedPerms, err := a.GetRolePermissionsCached(*user.RoleID)
		if err == nil {
			// Convert cached permission strings back to Permission objects
			permissions := make([]models.Permission, 0, len(cachedPerms))
			for _, p := range cachedPerms {
				parts := splitPermission(p)
				if len(parts) == 2 {
					permissions = append(permissions, models.Permission{
						Resource: parts[0],
						Action:   parts[1],
					})
				}
			}
			user.Role.Permissions = permissions
		}
	}

	return r.SendEnvelope(userToResponse(user))
}

// splitPermission splits a "resource:action" string
func splitPermission(p string) []string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == ':' {
			return []string{p[:i], p[i+1:]}
		}
	}
	return nil
}

// UpdateCurrentUserSettings updates the current user's notification/preferences settings
func (a *App) UpdateCurrentUserSettings(r *fastglue.Request) error {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	var req UserSettingsRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Initialize settings if nil
	if user.Settings == nil {
		user.Settings = make(models.JSONB)
	}

	// Update notification settings
	user.Settings["email_notifications"] = req.EmailNotifications
	user.Settings["new_message_alerts"] = req.NewMessageAlerts
	user.Settings["campaign_updates"] = req.CampaignUpdates

	if err := a.DB.Save(&user).Error; err != nil {
		a.Log.Error("Failed to update user settings", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update settings", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{
		"message":  "Settings updated successfully",
		"settings": user.Settings,
	})
}

// ChangePassword changes the current user's password
func (a *App) ChangePassword(r *fastglue.Request) error {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	var req ChangePasswordRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate required fields
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Current password and new password are required", nil, "")
	}

	// Validate new password length
	if len(req.NewPassword) < 6 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "New password must be at least 6 characters", nil, "")
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Current password is incorrect", nil, "")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("Failed to hash password", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to change password", nil, "")
	}

	user.PasswordHash = string(hashedPassword)
	if err := a.DB.Save(&user).Error; err != nil {
		a.Log.Error("Failed to update password", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to change password", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Password changed successfully"})
}

// Helper function to convert User to UserResponse
func userToResponse(user models.User) UserResponse {
	resp := UserResponse{
		ID:             user.ID,
		Email:          user.Email,
		FullName:       user.FullName,
		RoleID:         user.RoleID,
		IsActive:       user.IsActive,
		IsAvailable:    user.IsAvailable,
		IsSuperAdmin:   user.IsSuperAdmin,
		OrganizationID: user.OrganizationID,
		Settings:       user.Settings,
		CreatedAt:      user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	// Include role info if loaded
	if user.Role != nil {
		roleInfo := &RoleInfo{
			ID:          user.Role.ID,
			Name:        user.Role.Name,
			Description: user.Role.Description,
			IsSystem:    user.Role.IsSystem,
			Permissions: []PermissionInfo{},
		}

		// Include permissions if loaded
		for _, p := range user.Role.Permissions {
			roleInfo.Permissions = append(roleInfo.Permissions, PermissionInfo{
				ID:          p.ID,
				Resource:    p.Resource,
				Action:      p.Action,
				Description: p.Description,
			})
		}

		resp.Role = roleInfo
	}

	return resp
}

// AvailabilityRequest represents the request body for updating availability
type AvailabilityRequest struct {
	IsAvailable bool `json:"is_available"`
}

// UpdateAvailability updates the current user's availability status (away/available)
func (a *App) UpdateAvailability(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	var req AvailabilityRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Only log if status is actually changing
	if user.IsAvailable != req.IsAvailable {
		now := time.Now()

		// End the previous availability log (if exists)
		a.DB.Model(&models.UserAvailabilityLog{}).
			Where("user_id = ? AND ended_at IS NULL", userID).
			Update("ended_at", now)

		// Create new availability log
		log := models.UserAvailabilityLog{
			UserID:         userID,
			OrganizationID: orgID,
			IsAvailable:    req.IsAvailable,
			StartedAt:      now,
		}
		if err := a.DB.Create(&log).Error; err != nil {
			a.Log.Error("Failed to create availability log", "error", err)
			// Continue anyway - logging failure shouldn't block availability update
		}
	}

	user.IsAvailable = req.IsAvailable

	if err := a.DB.Save(&user).Error; err != nil {
		a.Log.Error("Failed to update availability", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update availability", nil, "")
	}

	status := "available"
	transfersReturned := 0
	if !req.IsAvailable {
		status = "away"
		// Return agent's active transfers to queue when going away
		transfersReturned = a.ReturnAgentTransfersToQueue(userID, orgID)
	}

	// Get the current break start time if away
	var breakStartedAt *time.Time
	if !req.IsAvailable {
		var currentLog models.UserAvailabilityLog
		if err := a.DB.Where("user_id = ? AND is_available = false AND ended_at IS NULL", userID).
			Order("started_at DESC").First(&currentLog).Error; err == nil {
			breakStartedAt = &currentLog.StartedAt
		}
	}

	return r.SendEnvelope(map[string]interface{}{
		"message":             "Availability updated successfully",
		"is_available":        user.IsAvailable,
		"status":              status,
		"break_started_at":    breakStartedAt,
		"transfers_to_queue":  transfersReturned,
	})
}
