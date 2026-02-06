package handlers

import (
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// TeamRequest represents create/update team request
type TeamRequest struct {
	Name               string                   `json:"name" validate:"required"`
	Description        string                   `json:"description"`
	AssignmentStrategy models.AssignmentStrategy `json:"assignment_strategy"` // round_robin, load_balanced, manual
	IsActive           bool                     `json:"is_active"`
}

// TeamMemberRequest represents add member request
type TeamMemberRequest struct {
	UserID string          `json:"user_id" validate:"required"`
	Role   models.TeamRole `json:"role"` // manager, agent
}

// TeamResponse represents team in API response
type TeamResponse struct {
	ID                 uuid.UUID                 `json:"id"`
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	AssignmentStrategy models.AssignmentStrategy `json:"assignment_strategy"`
	IsActive           bool                      `json:"is_active"`
	MemberCount        int                       `json:"member_count"`
	Members            []TeamMemberResponse      `json:"members,omitempty"`
	CreatedAt          time.Time                 `json:"created_at"`
	UpdatedAt          time.Time                 `json:"updated_at"`
}

// TeamMemberResponse represents team member in API response
type TeamMemberResponse struct {
	ID             uuid.UUID       `json:"id"`
	UserID         uuid.UUID       `json:"user_id"`
	FullName       string          `json:"full_name"`
	Email          string          `json:"email"`
	Role           models.TeamRole `json:"role"` // manager, agent
	IsAvailable    bool            `json:"is_available"`
	LastAssignedAt *time.Time      `json:"last_assigned_at,omitempty"`
}

// ListTeams returns teams based on user access
func (a *App) ListTeams(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))
	var teams []models.Team
	var total int64

	// Users with teams:read permission can see all teams, others see only their teams
	if a.HasPermission(userID, models.ResourceTeams, models.ActionRead, orgID) {
		baseQuery := a.ScopeToOrg(a.DB, userID, orgID)
		if search != "" {
			baseQuery = baseQuery.Where("name ILIKE ?", "%"+search+"%")
		}
		baseQuery.Model(&models.Team{}).Count(&total)
		if err := pg.Apply(baseQuery.
			Preload("Members").Preload("Members.User").
			Order("name ASC")).Find(&teams).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list teams", nil, "")
		}
	} else {
		// Users only see teams they belong to
		baseQuery := a.ScopeToOrg(a.DB.Joins("JOIN team_members ON team_members.team_id = teams.id"), userID, orgID).
			Where("team_members.user_id = ?", userID)
		if search != "" {
			baseQuery = baseQuery.Where("teams.name ILIKE ?", "%"+search+"%")
		}
		baseQuery.Model(&models.Team{}).Count(&total)
		if err := pg.Apply(baseQuery.
			Preload("Members").Preload("Members.User").
			Order("teams.name ASC")).Find(&teams).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list teams", nil, "")
		}
	}

	// Build response
	response := make([]TeamResponse, len(teams))
	for i, t := range teams {
		response[i] = buildTeamResponse(&t, false)
	}

	return r.SendEnvelope(map[string]interface{}{
		"teams": response,
		"total": total,
		"page":  pg.Page,
		"limit": pg.Limit,
	})
}

// GetTeam returns a single team with members
func (a *App) GetTeam(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	teamID, err := parsePathUUID(r, "id", "team")
	if err != nil {
		return nil
	}

	var team models.Team
	if err := a.DB.Where("id = ? AND organization_id = ?", teamID, orgID).
		Preload("Members").Preload("Members.User").
		First(&team).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Team not found", nil, "")
	}

	// Check access: users with teams:read permission can see all teams, otherwise must be a member
	if !a.HasPermission(userID, models.ResourceTeams, models.ActionRead, orgID) {
		hasAccess := false
		for _, m := range team.Members {
			if m.UserID == userID {
				hasAccess = true
				break
			}
		}
		if !hasAccess {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Access denied", nil, "")
		}
	}

	return r.SendEnvelope(map[string]interface{}{"team": buildTeamResponse(&team, true)})
}

// CreateTeam creates a new team
func (a *App) CreateTeam(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceTeams, models.ActionWrite); err != nil {
		return nil
	}

	var req TeamRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Team name is required", nil, "")
	}

	// Validate assignment strategy
	strategy := req.AssignmentStrategy
	if strategy == "" {
		strategy = models.AssignmentStrategyRoundRobin
	}
	if strategy != models.AssignmentStrategyRoundRobin && strategy != models.AssignmentStrategyLoadBalanced && strategy != models.AssignmentStrategyManual {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid assignment strategy", nil, "")
	}

	team := models.Team{
		OrganizationID:     orgID,
		Name:               req.Name,
		Description:        req.Description,
		AssignmentStrategy: strategy,
		IsActive:           true,
	}

	if err := a.DB.Create(&team).Error; err != nil {
		a.Log.Error("Failed to create team", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create team", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{"team": buildTeamResponse(&team, false)})
}

// UpdateTeam updates a team
func (a *App) UpdateTeam(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	teamID, err := parsePathUUID(r, "id", "team")
	if err != nil {
		return nil
	}

	var team models.Team
	if err := a.DB.Where("id = ? AND organization_id = ?", teamID, orgID).
		Preload("Members").First(&team).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Team not found", nil, "")
	}

	// Check access: users with teams:write permission OR team managers can update
	if !a.HasPermission(userID, models.ResourceTeams, models.ActionWrite, orgID) {
		isManager := false
		for _, m := range team.Members {
			if m.UserID == userID && m.Role == models.TeamRoleManager {
				isManager = true
				break
			}
		}
		if !isManager {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
		}
	}

	var req TeamRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Update fields
	if req.Name != "" {
		team.Name = req.Name
	}
	team.Description = req.Description
	team.IsActive = req.IsActive

	if req.AssignmentStrategy != "" {
		if req.AssignmentStrategy != models.AssignmentStrategyRoundRobin && req.AssignmentStrategy != models.AssignmentStrategyLoadBalanced && req.AssignmentStrategy != models.AssignmentStrategyManual {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid assignment strategy", nil, "")
		}
		team.AssignmentStrategy = req.AssignmentStrategy
	}

	if err := a.DB.Save(&team).Error; err != nil {
		a.Log.Error("Failed to update team", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update team", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{"team": buildTeamResponse(&team, false)})
}

// DeleteTeam deletes a team
func (a *App) DeleteTeam(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceTeams, models.ActionDelete); err != nil {
		return nil
	}

	teamID, err := parsePathUUID(r, "id", "team")
	if err != nil {
		return nil
	}

	// Delete team members first
	if err := a.DB.Where("team_id = ?", teamID).Delete(&models.TeamMember{}).Error; err != nil {
		a.Log.Error("Failed to delete team members", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete team", nil, "")
	}

	// Delete team
	result := a.DB.Where("id = ? AND organization_id = ?", teamID, orgID).Delete(&models.Team{})
	if result.Error != nil {
		a.Log.Error("Failed to delete team", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete team", nil, "")
	}

	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Team not found", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Team deleted"})
}

// ListTeamMembers lists members of a team
func (a *App) ListTeamMembers(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	teamID, err := parsePathUUID(r, "id", "team")
	if err != nil {
		return nil
	}

	// Verify team exists and user has access
	var team models.Team
	if err := a.DB.Where("id = ? AND organization_id = ?", teamID, orgID).
		Preload("Members").Preload("Members.User").
		First(&team).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Team not found", nil, "")
	}

	// Check access: users with teams:read permission can see all, otherwise must be a member
	if !a.HasPermission(userID, models.ResourceTeams, models.ActionRead, orgID) {
		hasAccess := false
		for _, m := range team.Members {
			if m.UserID == userID {
				hasAccess = true
				break
			}
		}
		if !hasAccess {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Access denied", nil, "")
		}
	}

	members := make([]TeamMemberResponse, len(team.Members))
	for i, m := range team.Members {
		members[i] = TeamMemberResponse{
			ID:             m.ID,
			UserID:         m.UserID,
			FullName:       m.User.FullName,
			Email:          m.User.Email,
			Role:           m.Role,
			IsAvailable:    m.User.IsAvailable,
			LastAssignedAt: m.LastAssignedAt,
		}
	}

	return r.SendEnvelope(map[string]interface{}{"members": members})
}

// AddTeamMember adds a member to a team
func (a *App) AddTeamMember(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	teamID, err := parsePathUUID(r, "id", "team")
	if err != nil {
		return nil
	}

	// Verify team exists
	var team models.Team
	if err := a.DB.Where("id = ? AND organization_id = ?", teamID, orgID).
		Preload("Members").First(&team).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Team not found", nil, "")
	}

	hasWritePermission := a.HasPermission(userID, models.ResourceTeams, models.ActionWrite, orgID)

	// Check access: users with teams:write permission OR team managers can add members
	if !hasWritePermission {
		isManager := false
		for _, m := range team.Members {
			if m.UserID == userID && m.Role == models.TeamRoleManager {
				isManager = true
				break
			}
		}
		if !isManager {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
		}
	}

	var req TeamMemberRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	memberUserID, err := uuid.Parse(req.UserID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid user ID", nil, "")
	}

	// Verify user exists in org
	user, err := findByIDAndOrg[models.User](a.DB, r, memberUserID, orgID, "User")
	if err != nil {
		return nil
	}

	// Check if already a member
	var existingMember models.TeamMember
	if err := a.DB.Where("team_id = ? AND user_id = ?", teamID, memberUserID).First(&existingMember).Error; err == nil {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "User is already a member of this team", nil, "")
	}

	// Validate role
	role := req.Role
	if role == "" {
		role = models.TeamRoleAgent
	}
	if role != models.TeamRoleManager && role != models.TeamRoleAgent {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid role. Must be 'manager' or 'agent'", nil, "")
	}

	// Only users with teams:write permission can add managers
	if !hasWritePermission && role == models.TeamRoleManager {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions to add managers", nil, "")
	}

	member := models.TeamMember{
		TeamID: teamID,
		UserID: memberUserID,
		Role:   role,
	}

	if err := a.DB.Create(&member).Error; err != nil {
		a.Log.Error("Failed to add team member", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to add member", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{"member": TeamMemberResponse{
		ID:          member.ID,
		UserID:      member.UserID,
		FullName:    user.FullName,
		Email:       user.Email,
		Role:        member.Role,
		IsAvailable: user.IsAvailable,
	}})
}

// RemoveTeamMember removes a member from a team
func (a *App) RemoveTeamMember(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	teamID, err := parsePathUUID(r, "id", "team")
	if err != nil {
		return nil
	}

	memberUserID, err := parsePathUUID(r, "member_user_id", "user")
	if err != nil {
		return nil
	}

	// Verify team exists
	var team models.Team
	if err := a.DB.Where("id = ? AND organization_id = ?", teamID, orgID).
		Preload("Members").First(&team).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Team not found", nil, "")
	}

	hasWritePermission := a.HasPermission(userID, models.ResourceTeams, models.ActionWrite, orgID)

	// Check access: users with teams:write permission OR team managers can remove members
	if !hasWritePermission {
		isManager := false
		for _, m := range team.Members {
			if m.UserID == userID && m.Role == models.TeamRoleManager {
				isManager = true
				break
			}
		}
		if !isManager {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
		}

		// Team managers cannot remove other managers
		for _, m := range team.Members {
			if m.UserID == memberUserID && m.Role == models.TeamRoleManager {
				return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions to remove managers", nil, "")
			}
		}
	}

	result := a.DB.Where("team_id = ? AND user_id = ?", teamID, memberUserID).Delete(&models.TeamMember{})
	if result.Error != nil {
		a.Log.Error("Failed to remove team member", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to remove member", nil, "")
	}

	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Member not found in team", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Member removed from team"})
}

// Helper function to build team response
func buildTeamResponse(team *models.Team, includeMembers bool) TeamResponse {
	resp := TeamResponse{
		ID:                 team.ID,
		Name:               team.Name,
		Description:        team.Description,
		AssignmentStrategy: team.AssignmentStrategy,
		IsActive:           team.IsActive,
		MemberCount:        len(team.Members),
		CreatedAt:          team.CreatedAt,
		UpdatedAt:          team.UpdatedAt,
	}

	if includeMembers && len(team.Members) > 0 {
		resp.Members = make([]TeamMemberResponse, len(team.Members))
		for i, m := range team.Members {
			resp.Members[i] = TeamMemberResponse{
				ID:             m.ID,
				UserID:         m.UserID,
				Role:           m.Role,
				LastAssignedAt: m.LastAssignedAt,
			}
			if m.User != nil {
				resp.Members[i].FullName = m.User.FullName
				resp.Members[i].Email = m.User.Email
				resp.Members[i].IsAvailable = m.User.IsAvailable
			}
		}
	}

	return resp
}
