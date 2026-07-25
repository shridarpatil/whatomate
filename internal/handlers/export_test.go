package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
)

// TransitionContactStatusForTest exposes transitionContactStatus to the
// external handlers_test package.
func (a *App) TransitionContactStatusForTest(
	contact *models.Contact,
	to models.ContactStatus,
	from []models.ContactStatus,
	actorID *uuid.UUID,
) (bool, error) {
	return a.transitionContactStatus(contact, to, from, actorID)
}

// SenderNameForBroadcastForTest exposes senderNameForBroadcast to the external
// handlers_test package, so its result can be pinned against senderName() —
// the REST-side producer of the same sent_by_user_name field.
func (a *App) SenderNameForBroadcastForTest(msg *models.Message) string {
	return a.senderNameForBroadcast(msg)
}

// SenderNameForTest exposes senderName to the external handlers_test package.
func SenderNameForTest(m *models.Message) string {
	return senderName(m)
}

// CreateAgentInitiatedTransferForTest exposes createAgentInitiatedTransfer.
func (a *App) CreateAgentInitiatedTransferForTest(account *models.WhatsAppAccount, contact *models.Contact, agentID uuid.UUID) {
	a.createAgentInitiatedTransfer(account, contact, agentID)
}

// ReleaseContactForTest exposes releaseContact.
func (a *App) ReleaseContactForTest(contact *models.Contact, actorID *uuid.UUID, reason string) error {
	return a.releaseContact(contact, actorID, reason)
}

// CanViewConversationForTest / CanInteractWithConversationForTest expose the
// authorization functions to the external test package.
func (a *App) CanViewConversationForTest(userID, orgID uuid.UUID, contact *models.Contact) bool {
	return a.canViewConversation(userID, orgID, contact)
}
func (a *App) CanInteractWithConversationForTest(userID, orgID uuid.UUID, contact *models.Contact) bool {
	return a.canInteractWithConversation(userID, orgID, contact)
}

// ScopeVisibleConversationsForTest exposes scopeVisibleConversations to the
// external handlers_test package.
func (a *App) ScopeVisibleConversationsForTest(q *gorm.DB, userID, orgID uuid.UUID) *gorm.DB {
	return a.scopeVisibleConversations(q, userID, orgID)
}

// CanViewTeamMemberForTest exposes canViewTeamMember to the external
// handlers_test package.
func (a *App) CanViewTeamMemberForTest(viewerID, ownerID uuid.UUID) bool {
	return a.canViewTeamMember(viewerID, ownerID)
}
