package handlers

import (
	"net/mail"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/audit"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Superadmin portal handlers. All endpoints under /api/admin are global
// (not org-scoped) and restricted to super admins via requireSuperAdmin.

// requireSuperAdmin extracts the current user and verifies superadmin status
// using the DB-backed check (not JWT claims, so revocation applies immediately).
// Returns the user ID on success; otherwise sends a 401/403 envelope and
// returns errEnvelopeSent.
func (a *App) requireSuperAdmin(r *fastglue.Request) (uuid.UUID, error) {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		_ = r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
		return uuid.Nil, errEnvelopeSent
	}
	if !a.IsSuperAdmin(userID) {
		_ = r.SendErrorEnvelope(fasthttp.StatusForbidden, "Super admin access required", nil, "")
		return uuid.Nil, errEnvelopeSent
	}
	return userID, nil
}

// AdminOrganizationResponse represents an organization in the superadmin portal.
type AdminOrganizationResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug,omitempty"`
	UserCount    int64     `json:"user_count"`
	AccountCount int64     `json:"account_count"`
	CreatedAt    string    `json:"created_at"`
}

// AdminListOrganizations returns all organizations with usage stats.
func (a *App) AdminListOrganizations(r *fastglue.Request) error {
	if _, err := a.requireSuperAdmin(r); err != nil {
		return nil
	}

	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	query := a.DB.Model(&models.Organization{})
	if search != "" {
		query = query.Where("name ILIKE ? OR slug ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var orgs []models.Organization
	if err := pg.Apply(query.Order("name ASC")).Find(&orgs).Error; err != nil {
		a.Log.Error("Failed to list organizations", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list organizations", nil, "")
	}

	orgIDs := make([]uuid.UUID, len(orgs))
	for i, org := range orgs {
		orgIDs[i] = org.ID
	}

	type orgCount struct {
		OrganizationID uuid.UUID
		Count          int64
	}
	userCounts := make(map[uuid.UUID]int64, len(orgs))
	accountCounts := make(map[uuid.UUID]int64, len(orgs))
	if len(orgIDs) > 0 {
		var counts []orgCount
		a.DB.Model(&models.UserOrganization{}).
			Select("organization_id, COUNT(*) AS count").
			Where("organization_id IN ?", orgIDs).
			Group("organization_id").
			Scan(&counts)
		for _, c := range counts {
			userCounts[c.OrganizationID] = c.Count
		}

		counts = nil
		a.DB.Model(&models.WhatsAppAccount{}).
			Select("organization_id, COUNT(*) AS count").
			Where("organization_id IN ?", orgIDs).
			Group("organization_id").
			Scan(&counts)
		for _, c := range counts {
			accountCounts[c.OrganizationID] = c.Count
		}
	}

	response := make([]AdminOrganizationResponse, len(orgs))
	for i, org := range orgs {
		response[i] = AdminOrganizationResponse{
			ID:           org.ID,
			Name:         org.Name,
			Slug:         org.Slug,
			UserCount:    userCounts[org.ID],
			AccountCount: accountCounts[org.ID],
			CreatedAt:    org.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return r.SendEnvelope(map[string]any{
		"organizations": response,
		"total":         total,
		"page":          pg.Page,
		"limit":         pg.Limit,
	})
}

// AdminCreateOrganizationRequest represents the request body for creating an
// organization together with its default administrator user.
type AdminCreateOrganizationRequest struct {
	Name          string `json:"name"`
	AdminFullName string `json:"admin_full_name"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
}

// AdminCreateOrganization creates a new organization along with its default
// administrator user in one step. The superadmin is not added as a member.
func (a *App) AdminCreateOrganization(r *fastglue.Request) error {
	userID, err := a.requireSuperAdmin(r)
	if err != nil {
		return nil
	}

	var req AdminCreateOrganizationRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Organization name is required", nil, "")
	}
	if req.AdminFullName == "" || req.AdminEmail == "" || req.AdminPassword == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Administrator full name, email, and password are required", nil, "")
	}
	if _, err := mail.ParseAddress(req.AdminEmail); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid administrator email format", nil, "")
	}
	// Reject duplicate emails before provisioning so a failed user creation
	// doesn't leave behind an orphaned organization.
	var existingUser models.User
	if err := a.DB.Where("email = ?", req.AdminEmail).First(&existingUser).Error; err == nil {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Email already exists", nil, "")
	}

	org, err := a.provisionOrganization(req.Name, userID, false)
	if err != nil {
		a.Log.Error("Failed to create organization", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create organization", nil, "")
	}

	var adminRole models.CustomRole
	if err := a.DB.Where("organization_id = ? AND name = ? AND is_system = ?", org.ID, "admin", true).First(&adminRole).Error; err != nil {
		a.Log.Error("Failed to find admin role for new organization", "error", err, "org_id", org.ID)
		a.DB.Delete(org)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create organization", nil, "")
	}

	adminUser, status, msg := a.createUserInOrg(org.ID, userID, UserRequest{
		Email:    req.AdminEmail,
		Password: req.AdminPassword,
		FullName: req.AdminFullName,
		RoleID:   &adminRole.ID,
	}, false)
	if adminUser == nil {
		// Roll back the org so the superadmin can simply retry.
		a.DB.Delete(org)
		return r.SendErrorEnvelope(status, msg, nil, "")
	}

	a.Log.Info("Created organization via admin portal", "org_id", org.ID, "org_name", org.Name,
		"admin_email", adminUser.Email, "created_by", userID)
	audit.LogAudit(a.DB, org.ID, userID, audit.GetUserName(a.DB, userID),
		"organization", org.ID, models.AuditActionCreated, nil,
		map[string]any{"name": org.Name, "slug": org.Slug, "admin_email": adminUser.Email})

	return r.SendEnvelope(AdminOrganizationResponse{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		UserCount: 1,
		CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// AdminUpdateOrganization renames an organization.
func (a *App) AdminUpdateOrganization(r *fastglue.Request) error {
	userID, err := a.requireSuperAdmin(r)
	if err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "organization")
	if err != nil {
		return nil
	}

	var req CreateOrganizationRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Organization name is required", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", id).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	oldName := org.Name
	org.Name = req.Name
	if err := a.DB.Model(&org).Update("name", req.Name).Error; err != nil {
		a.Log.Error("Failed to update organization", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update organization", nil, "")
	}

	audit.LogAudit(a.DB, org.ID, userID, audit.GetUserName(a.DB, userID),
		"organization", org.ID, models.AuditActionUpdated,
		map[string]any{"name": oldName}, map[string]any{"name": org.Name})

	return r.SendEnvelope(AdminOrganizationResponse{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// AdminRoleResponse represents a role option for the admin create-user dialog.
type AdminRoleResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsSystem    bool      `json:"is_system"`
	IsDefault   bool      `json:"is_default"`
}

// AdminListOrgRoles returns the roles of a specific organization.
func (a *App) AdminListOrgRoles(r *fastglue.Request) error {
	if _, err := a.requireSuperAdmin(r); err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "organization")
	if err != nil {
		return nil
	}

	var roles []models.CustomRole
	if err := a.DB.Where("organization_id = ?", id).Order("name ASC").Find(&roles).Error; err != nil {
		a.Log.Error("Failed to list organization roles", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list roles", nil, "")
	}

	response := make([]AdminRoleResponse, len(roles))
	for i, role := range roles {
		response[i] = AdminRoleResponse{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			IsSystem:    role.IsSystem,
			IsDefault:   role.IsDefault,
		}
	}

	return r.SendEnvelope(map[string]any{"roles": response})
}

// AdminUserResponse represents a user in the superadmin portal.
type AdminUserResponse struct {
	UserResponse
	OrganizationName string `json:"organization_name,omitempty"`
	OrgCount         int64  `json:"org_count"`
}

// AdminListUsers returns all users across all organizations.
func (a *App) AdminListUsers(r *fastglue.Request) error {
	if _, err := a.requireSuperAdmin(r); err != nil {
		return nil
	}

	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))
	orgIDParam := string(r.RequestCtx.QueryArgs().Peek("organization_id"))
	isActiveParam := string(r.RequestCtx.QueryArgs().Peek("is_active"))

	buildQuery := func() *gorm.DB {
		q := a.DB.Model(&models.User{}).Where("users.deleted_at IS NULL")
		if orgIDParam != "" {
			if orgUUID, err := uuid.Parse(orgIDParam); err == nil {
				q = q.Joins("JOIN user_organizations ON user_organizations.user_id = users.id AND user_organizations.organization_id = ? AND user_organizations.deleted_at IS NULL", orgUUID)
			}
		}
		if search != "" {
			q = q.Where("users.full_name ILIKE ? OR users.email ILIKE ?", "%"+search+"%", "%"+search+"%")
		}
		if isActiveParam == "true" || isActiveParam == "false" {
			q = q.Where("users.is_active = ?", isActiveParam == "true")
		}
		return q
	}

	var total int64
	buildQuery().Count(&total)

	var users []models.User
	if err := pg.Apply(buildQuery().Select("users.*").Order("users.created_at DESC")).
		Preload("Role").
		Find(&users).Error; err != nil {
		a.Log.Error("Failed to list users", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list users", nil, "")
	}

	// Resolve home org names and membership counts in two batched queries.
	userIDs := make([]uuid.UUID, len(users))
	homeOrgIDs := make([]uuid.UUID, 0, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
		homeOrgIDs = append(homeOrgIDs, u.OrganizationID)
	}

	orgNames := make(map[uuid.UUID]string, len(users))
	orgCounts := make(map[uuid.UUID]int64, len(users))
	if len(users) > 0 {
		var orgs []models.Organization
		a.DB.Where("id IN ?", homeOrgIDs).Find(&orgs)
		for _, org := range orgs {
			orgNames[org.ID] = org.Name
		}

		type userCount struct {
			UserID uuid.UUID
			Count  int64
		}
		var counts []userCount
		a.DB.Model(&models.UserOrganization{}).
			Select("user_id, COUNT(*) AS count").
			Where("user_id IN ?", userIDs).
			Group("user_id").
			Scan(&counts)
		for _, c := range counts {
			orgCounts[c.UserID] = c.Count
		}
	}

	response := make([]AdminUserResponse, len(users))
	for i, user := range users {
		response[i] = AdminUserResponse{
			UserResponse:     userToResponse(user),
			OrganizationName: orgNames[user.OrganizationID],
			OrgCount:         orgCounts[user.ID],
		}
	}

	return r.SendEnvelope(map[string]any{
		"users": response,
		"total": total,
		"page":  pg.Page,
		"limit": pg.Limit,
	})
}

// AdminCreateUserRequest represents the request body for creating a user in
// any organization from the superadmin portal.
type AdminCreateUserRequest struct {
	OrganizationID uuid.UUID  `json:"organization_id"`
	Email          string     `json:"email"`
	Password       string     `json:"password"`
	FullName       string     `json:"full_name"`
	RoleID         *uuid.UUID `json:"role_id"`
}

// AdminCreateUser creates a user directly in the specified organization.
// Superadmin status can never be granted through this endpoint.
func (a *App) AdminCreateUser(r *fastglue.Request) error {
	userID, err := a.requireSuperAdmin(r)
	if err != nil {
		return nil
	}

	var req AdminCreateUserRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.OrganizationID == uuid.Nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "organization_id is required", nil, "")
	}
	var org models.Organization
	if err := a.DB.Where("id = ?", req.OrganizationID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	user, status, msg := a.createUserInOrg(req.OrganizationID, userID, UserRequest{
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
		RoleID:   req.RoleID,
	}, false)
	if user == nil {
		return r.SendErrorEnvelope(status, msg, nil, "")
	}

	return r.SendEnvelope(AdminUserResponse{
		UserResponse:     userToResponse(*user),
		OrganizationName: org.Name,
		OrgCount:         1,
	})
}

// AdminSetUserStatusRequest represents the request body for activating or
// deactivating a user.
type AdminSetUserStatusRequest struct {
	IsActive *bool `json:"is_active"`
}

// AdminSetUserStatus activates or deactivates any user.
func (a *App) AdminSetUserStatus(r *fastglue.Request) error {
	userID, err := a.requireSuperAdmin(r)
	if err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "user")
	if err != nil {
		return nil
	}

	var req AdminSetUserStatusRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.IsActive == nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "is_active is required", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	if !*req.IsActive {
		if user.ID == userID {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot deactivate your own account", nil, "")
		}
		if user.IsSuperAdmin {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Cannot deactivate another super admin", nil, "")
		}
	}

	oldSnapshot := userAuditSnapshot(&user)
	user.IsActive = *req.IsActive
	if err := a.DB.Model(&user).Update("is_active", *req.IsActive).Error; err != nil {
		a.Log.Error("Failed to update user status", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update user status", nil, "")
	}

	a.InvalidateUserPermissionsCache(user.ID)

	audit.LogAudit(a.DB, user.OrganizationID, userID, audit.GetUserName(a.DB, userID),
		"user", user.ID, models.AuditActionUpdated, oldSnapshot, userAuditSnapshot(&user))

	return r.SendEnvelope(userToResponse(user))
}

// AdminResetPasswordRequest represents the request body for resetting a
// user's password from the superadmin portal.
type AdminResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// AdminResetUserPassword sets a new password for any user without requiring
// the current password.
func (a *App) AdminResetUserPassword(r *fastglue.Request) error {
	userID, err := a.requireSuperAdmin(r)
	if err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "user")
	if err != nil {
		return nil
	}

	var req AdminResetPasswordRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if len(req.NewPassword) < 6 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "New password must be at least 6 characters", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("Failed to hash password", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to reset password", nil, "")
	}

	if err := a.DB.Model(&user).Update("password_hash", string(hashedPassword)).Error; err != nil {
		a.Log.Error("Failed to reset password", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to reset password", nil, "")
	}

	audit.LogAudit(a.DB, user.OrganizationID, userID, audit.GetUserName(a.DB, userID),
		"user", user.ID, models.AuditActionUpdated, nil, nil,
		map[string]any{"field": "password", "old_value": "***", "new_value": "***"})

	return r.SendEnvelope(map[string]any{"success": true})
}
