package websocket

import (
	"time"

	"github.com/google/uuid"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Message types
const (
	TypeAuth           = "auth"
	TypeNewMessage     = "new_message"
	TypeStatusUpdate   = "status_update"
	TypeReactionUpdate = "reaction_update"
	TypeContactUpdate  = "contact_update"
	// TypeContactStatusChanged carries ContactStatusChangedPayload
	TypeContactStatusChanged = "contact_status_changed"
	// TypeAgentTyping carries AgentTypingPayload
	TypeAgentTyping = "agent_typing"
	TypeSetContact  = "set_contact"
	TypePing        = "ping"
	TypePong        = "pong"

	// Agent transfer types
	TypeAgentTransfer       = "agent_transfer"
	TypeAgentTransferResume = "agent_transfer_resume"
	TypeAgentTransferAssign = "agent_transfer_assign"
	TypeTransferEscalation  = "transfer_escalation"
	TypeTransferExpired     = "transfer_expired"
	TypeTransferEscalated   = "transfer_escalated"

	// Campaign types
	TypeCampaignStatsUpdate = "campaign_stats_update"

	// Permission types
	TypePermissionsUpdated = "permissions_updated"

	// Occurrence types
	TypeOccurrenceChanged      = "occurrence_changed"
	TypeOccurrenceEventCreated = "occurrence_event_created"

	// Conversation note types
	TypeConversationNoteCreated = "conversation_note_created"
	TypeConversationNoteUpdated = "conversation_note_updated"
	TypeConversationNoteDeleted = "conversation_note_deleted"

	// Call types
	TypeCallIncoming = "call_incoming"
	TypeCallAnswered = "call_answered"
	TypeCallEnded    = "call_ended"

	// Call transfer types
	TypeCallTransferWaiting    = "call_transfer_waiting"
	TypeCallTransferConnected  = "call_transfer_connected"
	TypeCallTransferCompleted  = "call_transfer_completed"
	TypeCallTransferAbandoned  = "call_transfer_abandoned"
	TypeCallTransferNoAnswer   = "call_transfer_no_answer"
	TypeCallTransferReassigned = "call_transfer_reassigned"

	// Call hold types
	TypeCallHold    = "call_hold"
	TypeCallResumed = "call_resumed"

	// Outgoing call types
	TypeOutgoingCallInitiated = "outgoing_call_initiated"
	TypeOutgoingCallRinging   = "outgoing_call_ringing"
	TypeOutgoingCallAnswered  = "outgoing_call_answered"
	TypeOutgoingCallRejected  = "outgoing_call_rejected"
	TypeOutgoingCallEnded     = "outgoing_call_ended"

	// Call permission types
	TypeCallPermissionUpdate = "call_permission_update"
)

// BroadcastMessage represents a message to be broadcast to clients
type BroadcastMessage struct {
	OrgID     uuid.UUID
	UserID    uuid.UUID // Optional: only send to specific user
	ContactID uuid.UUID // Optional: only send to users viewing this contact
	Message   WSMessage

	// RequireContactMatch restricts delivery to clients that have explicitly
	// selected ContactID. Without it, clients with no contact selected also
	// receive the message — the historical behaviour BroadcastToContact relies on.
	RequireContactMatch bool

	// IgnoreContactFilter skips the currentContact interest-filter so an
	// authorized client receives the event even while viewing a different
	// conversation. The authorization gate still applies. Used for new_message.
	IgnoreContactFilter bool
}

// ContactStatusChangedPayload is the payload for contact_status_changed events.
// Typed struct on purpose — a bare map would compile against Payload's `any`
// and only fail at runtime in the UI.
type ContactStatusChangedPayload struct {
	ContactID       uuid.UUID  `json:"contact_id"`
	OldStatus       string     `json:"old_status"`
	NewStatus       string     `json:"new_status"`
	ChangedByUserID *uuid.UUID `json:"changed_by_user_id,omitempty"`
	ChangedAt       time.Time  `json:"changed_at"`
}

// AgentTypingPayload is the payload for agent_typing events.
type AgentTypingPayload struct {
	ContactID uuid.UUID `json:"contact_id"`
	UserID    uuid.UUID `json:"user_id"`
	UserName  string    `json:"user_name"`
	At        time.Time `json:"at"`
}

// AuthPayload is the payload for auth messages from client
type AuthPayload struct {
	Token string `json:"token"`
}

// SetContactPayload is the payload for set_contact messages from client
type SetContactPayload struct {
	ContactID string `json:"contact_id"`
}

// StatusUpdatePayload is the payload for status_update messages
type StatusUpdatePayload struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}
