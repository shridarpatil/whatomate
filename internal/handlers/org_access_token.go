package handlers

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/models"
	"gorm.io/gorm"
)

// Errors returned by ResolveOrgUser.
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserInactive      = errors.New("user account is disabled")
	ErrOrgNotFound       = errors.New("organization not found")
	ErrNotOrgMember      = errors.New("user is not a member of this organization")
	ErrOrgLookupInternal = errors.New("failed to load organization access")
)

// ResolveOrgUser loads an active user as they would act inside orgID, with
// exactly the rules SwitchOrg applies: super admins may use any existing
// organisation (keeping their own role); everyone else needs a membership row
// and takes that row's role, never the home-org role. The returned user has
// OrganizationID and RoleID set for orgID and is ready for MintAccessToken.
func (a *App) ResolveOrgUser(userID, orgID uuid.UUID) (*models.User, error) {
	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, ErrOrgLookupInternal
	}
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	var orgCount int64
	if err := a.DB.Model(&models.Organization{}).Where("id = ?", orgID).Count(&orgCount).Error; err != nil {
		return nil, ErrOrgLookupInternal
	}
	if orgCount == 0 {
		return nil, ErrOrgNotFound
	}

	if !user.IsSuperAdmin {
		var userOrg models.UserOrganization
		if err := a.DB.Where("user_id = ? AND organization_id = ?", userID, orgID).First(&userOrg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrNotOrgMember
			}
			return nil, ErrOrgLookupInternal
		}
		// Same rule as SwitchOrg: the membership row is the only source of the
		// role in that organisation.
		user.RoleID = userOrg.RoleID
	}
	user.OrganizationID = orgID
	return &user, nil
}

// MintAccessToken signs a short-lived access JWT carrying the same claims a
// browser session would get for user (as returned by ResolveOrgUser). Used by
// the MCP connector to dispatch tool calls through the normal /api chain.
func (a *App) MintAccessToken(user *models.User, ttl time.Duration) (string, error) {
	return a.signAccessToken(user, ttl)
}
