package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// RoleRequest represents the request body for creating/updating a role
type RoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsDefault   bool     `json:"is_default"`
	Permissions []string `json:"permissions"` // Format: ["resource:action", ...]
}

// RoleResponse represents the response for a role
type RoleResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
	IsDefault   bool      `json:"is_default"`
	Permissions []string  `json:"permissions"`
	UserCount   int64     `json:"user_count"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// PermissionResponse represents a permission in the API
type PermissionResponse struct {
	ID          uuid.UUID `json:"id"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	Key         string    `json:"key"` // "resource:action"
}

// ListRoles returns all roles for the organization
func (a *App) ListRoles(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	baseQuery := a.ScopeToOrg(a.DB, userID, orgID)
	if search != "" {
		baseQuery = baseQuery.Where("name ILIKE ?", "%"+search+"%")
	}

	// Get total count
	var total int64
	baseQuery.Model(&models.CustomRole{}).Count(&total)

	var roles []models.CustomRole
	if err := pg.Apply(baseQuery.
		Order("is_system DESC, name ASC")).
		Find(&roles).Error; err != nil {
		a.Log.Error("Failed to list roles", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list roles", nil, "")
	}

	// Load permissions via JOIN instead of GORM's Preload IN query
	rolePtrs := make([]*models.CustomRole, len(roles))
	for i := range roles {
		rolePtrs[i] = &roles[i]
	}
	if err := a.loadRolePermissions(rolePtrs...); err != nil {
		a.Log.Error("Failed to load role permissions", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list roles", nil, "")
	}

	// Convert to response format with user counts
	response := make([]RoleResponse, len(roles))
	for i, role := range roles {
		var userCount int64
		a.DB.Model(&models.User{}).Where("role_id = ?", role.ID).Count(&userCount)
		response[i] = roleToResponse(role, userCount)
	}

	return r.SendEnvelope(map[string]interface{}{
		"roles": response,
		"total": total,
		"page":  pg.Page,
		"limit": pg.Limit,
	})
}

// GetRole returns a single role
func (a *App) GetRole(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "role")
	if err != nil {
		return nil
	}

	var role models.CustomRole
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		First(&role).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Role not found", nil, "")
	}

	if err := a.loadRolePermissions(&role); err != nil {
		a.Log.Error("Failed to load role permissions", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to get role", nil, "")
	}

	var userCount int64
	a.DB.Model(&models.User{}).Where("role_id = ?", role.ID).Count(&userCount)

	return r.SendEnvelope(roleToResponse(role, userCount))
}

// CreateRole creates a new custom role
func (a *App) CreateRole(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req RoleRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate required fields
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Name is required", nil, "")
	}

	// Check if name already exists
	var existingRole models.CustomRole
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.Name).First(&existingRole).Error; err == nil {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Role with this name already exists", nil, "")
	}

	// Get permissions from database
	permissions, err := a.getPermissionsByKeys(req.Permissions)
	if err != nil {
		a.Log.Error("Failed to fetch permissions", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create role", nil, "")
	}

	role := models.CustomRole{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           req.Name,
		Description:    req.Description,
		IsSystem:       false,
		IsDefault:      req.IsDefault,
		Permissions:    permissions,
	}

	// If setting as default, unset other defaults (in a transaction)
	if req.IsDefault {
		if err := a.DB.Transaction(func(tx *gorm.DB) error {
			tx.Model(&models.CustomRole{}).
				Where("organization_id = ? AND is_default = ?", orgID, true).
				Update("is_default", false)
			return tx.Create(&role).Error
		}); err != nil {
			a.Log.Error("Failed to create role", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create role", nil, "")
		}
		return r.SendEnvelope(roleToResponse(role, 0))
	}

	if err := a.DB.Create(&role).Error; err != nil {
		a.Log.Error("Failed to create role", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create role", nil, "")
	}

	return r.SendEnvelope(roleToResponse(role, 0))
}

// UpdateRole updates a custom role
func (a *App) UpdateRole(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "role")
	if err != nil {
		return nil
	}

	var role models.CustomRole
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		First(&role).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Role not found", nil, "")
	}

	if err := a.loadRolePermissions(&role); err != nil {
		a.Log.Error("Failed to load role permissions", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update role", nil, "")
	}

	// System roles can only have their description updated
	var req RoleRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if role.IsSystem {
		// Check if user is super admin
		isSuperAdmin, _ := r.RequestCtx.UserValue("is_super_admin").(bool)

		// Only allow description updates for non-super admins
		if req.Description != "" {
			role.Description = req.Description
		}

		// Super admins can update permissions for system roles
		if isSuperAdmin && len(req.Permissions) > 0 {
			permissions, err := a.getPermissionsByKeys(req.Permissions)
			if err != nil {
				a.Log.Error("Failed to fetch permissions", "error", err)
				return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update role", nil, "")
			}
			if err := a.DB.Model(&role).Association("Permissions").Replace(permissions); err != nil {
				a.Log.Error("Failed to update role permissions", "error", err)
				return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update role", nil, "")
			}
			role.Permissions = permissions
		}

		if err := a.DB.Save(&role).Error; err != nil {
			a.Log.Error("Failed to update role", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update role", nil, "")
		}

		// Invalidate permissions cache for all users with this role
		if isSuperAdmin && len(req.Permissions) > 0 {
			a.InvalidateRolePermissionsCache(role.ID)
		}

		var userCount int64
		a.DB.Model(&models.User{}).Where("role_id = ?", role.ID).Count(&userCount)
		return r.SendEnvelope(roleToResponse(role, userCount))
	}

	// For custom roles, allow full updates
	if req.Name != "" {
		// Check if name already exists for another role
		var existingRole models.CustomRole
		if err := a.DB.Where("organization_id = ? AND name = ? AND id != ?", orgID, req.Name, id).First(&existingRole).Error; err == nil {
			return r.SendErrorEnvelope(fasthttp.StatusConflict, "Role with this name already exists", nil, "")
		}
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = req.Description
	}

	// Update permissions if provided
	if len(req.Permissions) > 0 {
		permissions, err := a.getPermissionsByKeys(req.Permissions)
		if err != nil {
			a.Log.Error("Failed to fetch permissions", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update role", nil, "")
		}
		// Replace associations
		if err := a.DB.Model(&role).Association("Permissions").Replace(permissions); err != nil {
			a.Log.Error("Failed to update role permissions", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update role", nil, "")
		}
		role.Permissions = permissions
	}

	// Handle default flag (in a transaction to prevent race conditions)
	if req.IsDefault && !role.IsDefault {
		role.IsDefault = true
		if err := a.DB.Transaction(func(tx *gorm.DB) error {
			tx.Model(&models.CustomRole{}).
				Where("organization_id = ? AND is_default = ? AND id != ?", orgID, true, role.ID).
				Update("is_default", false)
			return tx.Save(&role).Error
		}); err != nil {
			a.Log.Error("Failed to update role", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update role", nil, "")
		}
	} else {
		if !req.IsDefault && role.IsDefault {
			role.IsDefault = false
		}
		if err := a.DB.Save(&role).Error; err != nil {
			a.Log.Error("Failed to update role", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update role", nil, "")
		}
	}

	// Invalidate permissions cache for all users with this role
	a.InvalidateRolePermissionsCache(role.ID)

	var userCount int64
	a.DB.Model(&models.User{}).Where("role_id = ?", role.ID).Count(&userCount)
	return r.SendEnvelope(roleToResponse(role, userCount))
}

// DeleteRole deletes a custom role
func (a *App) DeleteRole(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "role")
	if err != nil {
		return nil
	}

	role, err := findByIDAndOrg[models.CustomRole](a.DB, r, id, orgID, "Role")
	if err != nil {
		return nil
	}

	// Cannot delete system roles
	if role.IsSystem {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot delete system roles", nil, "")
	}

	// Check if any users have this role
	var userCount int64
	a.DB.Model(&models.User{}).Where("role_id = ?", id).Count(&userCount)
	if userCount > 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot delete role with assigned users", nil, "")
	}

	// Delete the role (permissions associations will be cleared automatically)
	if err := a.DB.Delete(role).Error; err != nil {
		a.Log.Error("Failed to delete role", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete role", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Role deleted successfully"})
}

// ListPermissions returns all available permissions
func (a *App) ListPermissions(r *fastglue.Request) error {
	var permissions []models.Permission
	if err := a.DB.Order("resource ASC, action ASC").Find(&permissions).Error; err != nil {
		a.Log.Error("Failed to list permissions", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list permissions", nil, "")
	}

	response := make([]PermissionResponse, len(permissions))
	for i, p := range permissions {
		response[i] = PermissionResponse{
			ID:          p.ID,
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
			Key:         p.Resource + ":" + p.Action,
		}
	}

	return r.SendEnvelope(map[string]interface{}{
		"permissions": response,
	})
}

// Helper function to convert CustomRole to RoleResponse
func roleToResponse(role models.CustomRole, userCount int64) RoleResponse {
	permissions := make([]string, len(role.Permissions))
	for i, p := range role.Permissions {
		permissions[i] = p.Resource + ":" + p.Action
	}

	return RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		IsDefault:   role.IsDefault,
		Permissions: permissions,
		UserCount:   userCount,
		CreatedAt:   role.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   role.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// Helper function to get permissions by their keys
func (a *App) getPermissionsByKeys(keys []string) ([]models.Permission, error) {
	if len(keys) == 0 {
		return []models.Permission{}, nil
	}

	// Parse keys into resource:action pairs
	var conditions [][]string
	for _, key := range keys {
		if len(key) > 0 {
			parts := splitPermissionKey(key)
			if len(parts) == 2 {
				conditions = append(conditions, parts)
			}
		}
	}

	if len(conditions) == 0 {
		return []models.Permission{}, nil
	}

	var permissions []models.Permission
	query := a.DB.Model(&models.Permission{})

	// Build OR conditions for each permission
	for i, cond := range conditions {
		if i == 0 {
			query = query.Where("resource = ? AND action = ?", cond[0], cond[1])
		} else {
			query = query.Or("resource = ? AND action = ?", cond[0], cond[1])
		}
	}

	if err := query.Find(&permissions).Error; err != nil {
		return nil, err
	}

	return permissions, nil
}

// loadRolePermissions loads permissions for roles via JOIN instead of GORM's
// Preload, which generates a slow IN query with all permission UUIDs.
func (a *App) loadRolePermissions(roles ...*models.CustomRole) error {
	if len(roles) == 0 {
		return nil
	}
	roleIDs := make([]uuid.UUID, len(roles))
	roleMap := make(map[uuid.UUID]*models.CustomRole, len(roles))
	for i, r := range roles {
		roleIDs[i] = r.ID
		r.Permissions = []models.Permission{}
		roleMap[r.ID] = r
	}

	var results []struct {
		models.Permission
		CustomRoleID uuid.UUID `gorm:"column:custom_role_id"`
	}
	err := a.DB.Table("permissions").
		Select("permissions.*, role_permissions.custom_role_id").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.custom_role_id IN ?", roleIDs).
		Where("permissions.deleted_at IS NULL").
		Find(&results).Error
	if err != nil {
		return err
	}

	for _, r := range results {
		if role, ok := roleMap[r.CustomRoleID]; ok {
			role.Permissions = append(role.Permissions, r.Permission)
		}
	}
	return nil
}

// splitPermissionKey splits "resource:action" into ["resource", "action"]
func splitPermissionKey(key string) []string {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return nil
}
