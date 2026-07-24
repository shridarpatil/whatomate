package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
)

// conversationAccess is the single authorization decision for a conversation,
// computed once from the contact's state. canView and canInteract are separate
// concepts even though cycle 2 derives both from the same rule — see the spec
// section "A função central".
type conversationAccess struct {
	canView     bool
	canInteract bool
}

// authorizeConversation is the ONLY place the visibility rule lives.
//
// Precedence invariant: Contact.AssignedUserID (carteira) is consulted only
// when there is no active AgentTransfer for the contact. An active transfer
// always wins, so a queued/closed/transferred conversation is never governed
// by a stale carteira pointer.
func (a *App) authorizeConversation(userID, orgID uuid.UUID, contact *models.Contact) conversationAccess {
	settings, _ := a.getChatbotSettingsCached(orgID, "")

	// Flag off (default): preserve today's behaviour exactly — contacts:read
	// sees all, otherwise only own/assigned (the old assigned-contact scope).
	if settings == nil || !settings.AgentAssignment.StrictConversationVisibility {
		if a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
			return conversationAccess{canView: true, canInteract: true}
		}
		ok := a.userOwnsContact(userID, orgID, contact)
		return conversationAccess{canView: ok, canInteract: ok}
	}

	// Strict mode.
	if a.HasPermission(userID, models.ResourceConversations, models.ActionViewAll, orgID) {
		return conversationAccess{canView: true, canInteract: true}
	}

	// view_team widens exactly two branches (A: transfer-to-agent, D: carteira)
	// to "owner shares a team with me". Only consulted for non-view_all users.
	hasViewTeam := a.HasPermission(userID, models.ResourceConversations, models.ActionViewTeam, orgID)

	// Active transfer is the primary authority.
	transfer, hasActive := a.activeTransferFor(orgID, contact.ID)
	if hasActive {
		switch {
		case transfer.AgentID != nil:
			ok := *transfer.AgentID == userID ||
				(hasViewTeam && a.canViewTeamMember(userID, *transfer.AgentID))
			return conversationAccess{canView: ok, canInteract: ok}
		case transfer.TeamID != nil:
			ok := a.userInTeam(userID, *transfer.TeamID)
			return conversationAccess{canView: ok, canInteract: ok}
		default:
			// Active general-queue transfer (no agent, no team): fall back to
			// the account default team, else view_all only.
			if team := a.accountDefaultTeamID(orgID, contact); team != nil {
				ok := a.userInTeam(userID, *team)
				return conversationAccess{canView: ok, canInteract: ok}
			}
			return conversationAccess{canView: false, canInteract: false}
		}
	}

	// No active transfer: carteira governs (more specific than any team).
	if contact.AssignedUserID != nil {
		ok := *contact.AssignedUserID == userID ||
			(hasViewTeam && a.canViewTeamMember(userID, *contact.AssignedUserID))
		return conversationAccess{canView: ok, canInteract: ok}
	}

	// No carteira: effective team = flow-set team, else account default team.
	effTeam := contact.TeamID
	if effTeam == nil {
		effTeam = a.accountDefaultTeamID(orgID, contact)
	}
	if effTeam != nil {
		ok := a.userInTeam(userID, *effTeam)
		return conversationAccess{canView: ok, canInteract: ok}
	}

	// No transfer, no carteira, no team: view_all only.
	return conversationAccess{canView: false, canInteract: false}
}

func (a *App) canViewConversation(userID, orgID uuid.UUID, contact *models.Contact) bool {
	return a.authorizeConversation(userID, orgID, contact).canView
}

// CanViewConversationByID loads the contact org-scoped and reports whether the
// user may view its conversation. Used to authorize WebSocket delivery.
func (a *App) CanViewConversationByID(userID, orgID, contactID uuid.UUID) bool {
	var contact models.Contact
	if err := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID).First(&contact).Error; err != nil {
		return false
	}
	return a.canViewConversation(userID, orgID, &contact)
}

func (a *App) canInteractWithConversation(userID, orgID uuid.UUID, contact *models.Contact) bool {
	return a.authorizeConversation(userID, orgID, contact).canInteract
}

// activeTransferFor returns the contact's active transfer, if any.
func (a *App) activeTransferFor(orgID, contactID uuid.UUID) (models.AgentTransfer, bool) {
	var t models.AgentTransfer
	err := a.DB.Where("organization_id = ? AND contact_id = ? AND status = ?",
		orgID, contactID, models.TransferStatusActive).
		Order("transferred_at DESC").First(&t).Error
	if err != nil {
		return models.AgentTransfer{}, false
	}
	return t, true
}

// userInTeam reports whether the user is a member of the team.
func (a *App) userInTeam(userID, teamID uuid.UUID) bool {
	var count int64
	a.DB.Model(&models.TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Count(&count)
	return count > 0
}

// canViewTeamMember reports whether viewerID may see conversations owned by
// ownerID by virtue of team scope: there is at least one team in which BOTH
// have an active membership. It is the single Go definition of "owner is in my
// team scope" — its SQL twin is the viewTeamScope subquery in
// scopeVisibleConversations. Keep the two in sync (TestVisibilityScopeMatchesFunction).
func (a *App) canViewTeamMember(viewerID, ownerID uuid.UUID) bool {
	var count int64
	a.DB.Model(&models.TeamMember{}).
		Where("user_id = ? AND team_id IN (?)", ownerID,
			a.DB.Model(&models.TeamMember{}).Select("team_id").Where("user_id = ?", viewerID)).
		Count(&count)
	return count > 0
}

// accountDefaultTeamID returns the default team configured on the contact's
// WhatsApp account, or nil. Used only in strict mode as the last team signal
// before falling back to view_all-only.
func (a *App) accountDefaultTeamID(orgID uuid.UUID, contact *models.Contact) *uuid.UUID {
	if contact == nil {
		return nil
	}
	return a.getAccountDefaultTeamCached(orgID, contact.WhatsAppAccount)
}

// scopeVisibleConversations is the SQL translation of authorizeConversation.canView
// (see spec §"A função central"). It must return exactly the contacts for which
// canViewConversation is true — TestVisibilityScopeMatchesFunction guards that.
// It is the single scope now used at every listing/read/action site.
func (a *App) scopeVisibleConversations(query *gorm.DB, userID, orgID uuid.UUID) *gorm.DB {
	settings, _ := a.getChatbotSettingsCached(orgID, "")

	// Flag off: preserve the old assigned-contact scope exactly.
	if settings == nil || !settings.AgentAssignment.StrictConversationVisibility {
		if a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
			return query
		}
		return query.Where("assigned_user_id = ? OR id IN (?)",
			userID,
			a.DB.Model(&models.AgentTransfer{}).Select("contact_id").
				Where("agent_id = ? AND organization_id = ? AND status = ?",
					userID, orgID, models.TransferStatusActive),
		)
	}

	// Strict: view_all sees everything.
	if a.HasPermission(userID, models.ResourceConversations, models.ActionViewAll, orgID) {
		return query
	}

	myTeams := a.DB.Model(&models.TeamMember{}).Select("team_id").Where("user_id = ?", userID)
	activeSub := a.DB.Model(&models.AgentTransfer{}).Select("contact_id").
		Where("organization_id = ? AND status = ?", orgID, models.TransferStatusActive)
	activeAgentMine := a.DB.Model(&models.AgentTransfer{}).Select("contact_id").
		Where("organization_id = ? AND status = ? AND agent_id = ?", orgID, models.TransferStatusActive, userID)
	activeTeamMine := a.DB.Model(&models.AgentTransfer{}).Select("contact_id").
		Where("organization_id = ? AND status = ? AND agent_id IS NULL AND team_id IN (?)",
			orgID, models.TransferStatusActive, myTeams)
	activeGeneral := a.DB.Model(&models.AgentTransfer{}).Select("contact_id").
		Where("organization_id = ? AND status = ? AND agent_id IS NULL AND team_id IS NULL",
			orgID, models.TransferStatusActive)

	// The contact's WhatsApp account default team is one of my teams.
	acctDefault := `EXISTS (SELECT 1 FROM whatsapp_accounts wa
		WHERE wa.name = contacts.whats_app_account
		  AND wa.organization_id = contacts.organization_id
		  AND wa.default_team_id IN (?))`

	cond := a.DB.
		Where("id IN (?)", activeAgentMine).                                              // A: active transfer to me
		Or("id IN (?)", activeTeamMine).                                                  // B: active team queue, my team
		Or(a.DB.Where("id IN (?)", activeGeneral).Where(acctDefault, myTeams)).           // C: general queue + account default mine
		Or(a.DB.Where("id NOT IN (?)", activeSub).Where("assigned_user_id = ?", userID)). // D: carteira mine
		Or(a.DB.Where("id NOT IN (?)", activeSub).
			Where("assigned_user_id IS NULL AND team_id IN (?)", myTeams)). // E: flow team mine
		Or(a.DB.Where("id NOT IN (?)", activeSub).
			Where("assigned_user_id IS NULL AND team_id IS NULL").Where(acctDefault, myTeams)) // F: account default mine

	// view_team: the team-scoped analogues of A and D, gated by the permission.
	// viewTeamScope = users who share a team with me (the SQL twin of
	// canViewTeamMember). Emitted only when the user holds view_team.
	if a.HasPermission(userID, models.ResourceConversations, models.ActionViewTeam, orgID) {
		viewTeamScope := a.DB.Model(&models.TeamMember{}).Select("user_id").
			Where("team_id IN (?)", myTeams)
		activeAgentTeam := a.DB.Model(&models.AgentTransfer{}).Select("contact_id").
			Where("organization_id = ? AND status = ? AND agent_id IN (?)",
				orgID, models.TransferStatusActive, viewTeamScope)
		cond = cond.
			Or("id IN (?)", activeAgentTeam). // G: active transfer to a team-mate agent
			Or(a.DB.Where("id NOT IN (?)", activeSub).
				Where("assigned_user_id IN (?)", viewTeamScope)) // H: carteira of a team-mate agent
	}

	return query.Where(cond)
}

// userOwnsContact mirrors the old assigned-contact "mine" condition, for
// the flag-off path: the contact is assigned to the user, or an active transfer
// is assigned to them.
func (a *App) userOwnsContact(userID, orgID uuid.UUID, contact *models.Contact) bool {
	if contact.AssignedUserID != nil && *contact.AssignedUserID == userID {
		return true
	}
	var count int64
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND contact_id = ? AND agent_id = ? AND status = ?",
			orgID, contact.ID, userID, models.TransferStatusActive).
		Count(&count)
	return count > 0
}
