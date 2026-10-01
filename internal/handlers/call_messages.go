package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
)

// missedCallPreview is what the contact list shows for a missed call.
const missedCallPreview = "[Missed call]"

// markCallSticky records which agent's click-to-call button produced this call.
// The terminate webhook reads it back to decide whether an unanswered call
// deserves a chat entry, long after the Redis key has expired.
func (a *App) markCallSticky(callLog *models.CallLog, agentID uuid.UUID) {
	if callLog.StickyAgentID != nil && *callLog.StickyAgentID == agentID {
		return
	}
	if err := a.DB.Model(callLog).Update("sticky_agent_id", agentID).Error; err != nil {
		a.Log.Error("Failed to record sticky agent on call log",
			"error", err, "call_log_id", callLog.ID, "agent_id", agentID)
		return
	}
	callLog.StickyAgentID = &agentID
}

// rejectUnreachableStickyCall ends a click-to-call call whose originating agent
// can't take it. Answering first would only park the customer on hold waiting
// for an agent who is never coming, and the call is never offered to anyone
// else, so there is nothing to wait for.
func (a *App) rejectUnreachableStickyCall(account *models.WhatsAppAccount, contact *models.Contact, callLog *models.CallLog, agentID uuid.UUID, now time.Time) {
	a.markCallSticky(callLog, agentID)

	if a.CallManager != nil {
		a.CallManager.RejectIncomingCall(context.Background(), account, callLog.WhatsAppCallID)
	}

	a.DB.Model(callLog).Updates(map[string]any{
		"status":          models.CallStatusMissed,
		"ended_at":        now,
		"disconnected_by": models.DisconnectedBySystem,
	})
	callLog.Status = models.CallStatusMissed

	a.recordMissedCallMessage(callLog, contact)

	a.Log.Info("Rejected click-to-call call: originating agent unreachable",
		"call_id", callLog.WhatsAppCallID, "agent_id", agentID)

	a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallEnded, map[string]any{
		"call_id":         callLog.WhatsAppCallID,
		"contact_id":      contact.ID.String(),
		"status":          string(models.CallStatusMissed),
		"duration":        0,
		"ended_at":        now.Format(time.RFC3339),
		"disconnected_by": string(models.DisconnectedBySystem),
	})
}

// recordMissedCallMessage drops a missed-call entry into the contact's chat so
// the agent sees it where they're already looking, rather than only in the call
// log. Covers every missed incoming call, not just click-to-call ones.
// Idempotent on call_log_id — Meta re-sends terminate events.
func (a *App) recordMissedCallMessage(callLog *models.CallLog, contact *models.Contact) {
	var existing int64
	if err := a.DB.Model(&models.Message{}).
		Where("organization_id = ? AND message_type = ? AND metadata->>'call_log_id' = ?",
			callLog.OrganizationID, models.MessageTypeCall, callLog.ID.String()).
		Count(&existing).Error; err != nil {
		a.Log.Error("Failed to check for an existing missed-call message",
			"error", err, "call_log_id", callLog.ID)
		return
	}
	if existing > 0 {
		return
	}

	metadata := models.JSONB{
		"call_log_id":  callLog.ID.String(),
		"call_status":  string(models.CallStatusMissed),
		"caller_phone": callLog.CallerPhone,
	}
	if callLog.StickyAgentID != nil {
		metadata["sticky_agent_id"] = callLog.StickyAgentID.String()
	}

	message := models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  callLog.OrganizationID,
		WhatsAppAccount: callLog.WhatsAppAccount,
		ContactID:       contact.ID,
		Direction:       models.DirectionIncoming,
		MessageType:     models.MessageTypeCall,
		Content:         "Missed call",
		Status:          models.MessageStatusReceived,
		Metadata:        metadata,
	}
	if err := a.DB.Create(&message).Error; err != nil {
		a.Log.Error("Failed to record missed-call message",
			"error", err, "call_log_id", callLog.ID)
		return
	}

	// Bump the conversation and leave it unread so the missed call is visible
	// in the contact list. LastInboundAt is deliberately untouched — a call
	// doesn't open the 24-hour messaging window.
	now := time.Now()
	a.DB.Model(contact).Updates(map[string]any{
		"last_message_at":      now,
		"last_message_preview": missedCallPreview,
		"is_read":              false,
	})
	contact.IsRead = false
	contact.LastMessageAt = &now
	contact.LastMessagePreview = missedCallPreview

	a.broadcastNewMessage(callLog.OrganizationID, &message, contact)
}
