package handlers

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/contactutil"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
)

// processCallWebhook handles a call webhook event for both incoming and outgoing calls.
// It creates/updates the CallLog and delegates to the CallManager for WebRTC handling.
func (a *App) processCallWebhook(phoneNumberID string, call interface{}) {
	// The webhook handler passes an anonymous struct. Convert via JSON round-trip.
	type callEvent struct {
		ID        string `json:"id"`
		From      string `json:"from"`
		To        string `json:"to"`
		Timestamp string `json:"timestamp"`
		Type      string `json:"type"`
		Event     string `json:"event"`
		Direction string `json:"direction,omitempty"`
		Session   *struct {
			SDPType string `json:"sdp_type"`
			SDP     string `json:"sdp"`
		} `json:"session,omitempty"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
		Duration int `json:"duration,omitempty"`
	}

	var ce callEvent
	b, _ := json.Marshal(call)
	if err := json.Unmarshal(b, &ce); err != nil {
		a.Log.Error("Failed to parse call event", "error", err)
		return
	}

	// Log raw payload to debug SDP and field mapping
	a.Log.Debug("Raw call webhook payload", "payload", string(b))

	// Check if this call_id belongs to an existing outgoing session
	if a.CallManager != nil {
		session := a.CallManager.GetSession(ce.ID)
		if session != nil && session.Direction == models.CallDirectionOutgoing {
			sdp := ""
			if ce.Session != nil {
				sdp = ce.Session.SDP
			}
			a.CallManager.HandleOutgoingCallWebhook(ce.ID, ce.Event, sdp)
			return
		}
	}

	// Handle business-initiated events when session is already cleaned up
	// (e.g., terminate webhook arrives after PeerConnection closed)
	if ce.Direction == "BUSINESS_INITIATED" {
		a.handleOrphanedOutgoingCallEvent(phoneNumberID, ce.ID, ce.Event, ce.Duration)
		return
	}

	// --- Incoming call flow ---

	// Look up the WhatsApp account
	account, err := a.getWhatsAppAccountCached(phoneNumberID)
	if err != nil {
		a.Log.Error("Failed to find WhatsApp account for call", "error", err, "phone_id", phoneNumberID)
		return
	}

	// Get or create the contact
	contact, _, _ := contactutil.GetOrCreateContact(a.DB, account.OrganizationID, ce.From, "")

	if contact == nil {
		a.Log.Error("Failed to get or create contact for call", "phone", ce.From)
		return
	}

	now := time.Now()

	// Ensure a CallLog exists for this call. WhatsApp may send "connect" as the
	// first event (skipping "ringing"), so we create the record on demand.
	callLog := a.getOrCreateCallLog(account, contact, ce.ID, ce.From, now)
	if callLog == nil {
		return
	}

	switch ce.Event {
	case "ringing":
		// Broadcast incoming call via WebSocket (no SDP yet, WebRTC starts on "connect")
		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallIncoming, map[string]any{
			"call_log_id":  callLog.ID.String(),
			"call_id":      ce.ID,
			"caller_phone": ce.From,
			"contact_id":   contact.ID.String(),
			"contact_name": contact.ProfileName,
			"ivr_flow_id":  callLog.IVRFlowID,
			"started_at":   now.Format(time.RFC3339),
		})

	case "connect":
		// "connect" carries the SDP offer from the consumer in session.sdp.
		// Extract SDP and start WebRTC negotiation.
		sdpOffer := ""
		if ce.Session != nil && ce.Session.SDPType == "offer" {
			sdpOffer = ce.Session.SDP
		}

		// Update call status to answered
		a.DB.Model(callLog).Updates(map[string]any{
			"status":      models.CallStatusAnswered,
			"answered_at": now,
		})

		// Delegate to CallManager with the SDP offer
		if a.IsCallingEnabledForOrg(account.OrganizationID) && sdpOffer != "" {
			session := a.CallManager.GetSession(ce.ID)
			if session == nil {
				a.CallManager.HandleIncomingCall(account, contact, callLog, sdpOffer)
			} else {
				a.CallManager.HandleCallEvent(ce.ID, ce.Event)
			}
		}

		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallAnswered, map[string]any{
			"call_id":     ce.ID,
			"contact_id":  contact.ID.String(),
			"answered_at": now.Format(time.RFC3339),
		})

	case "in_call":
		// Update call status to answered
		a.DB.Model(callLog).Updates(map[string]any{
			"status":      models.CallStatusAnswered,
			"answered_at": now,
		})

		// Notify CallManager if session exists
		if a.IsCallingEnabledForOrg(account.OrganizationID) {
			if session := a.CallManager.GetSession(ce.ID); session != nil {
				a.CallManager.HandleCallEvent(ce.ID, ce.Event)
			}
		}

		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallAnswered, map[string]any{
			"call_id":     ce.ID,
			"contact_id":  contact.ID.String(),
			"answered_at": now.Format(time.RFC3339),
		})

	case "ended", "terminate":
		// Calculate duration and determine final status.
		// Re-read the call log to get the latest agent_id (may have been set
		// by transfer acceptance after our initial read).
		a.DB.First(callLog, callLog.ID)

		duration := 0
		if callLog.AnsweredAt != nil {
			duration = int(now.Sub(*callLog.AnsweredAt).Seconds())
		}

		// For incoming calls that were pre-accepted for WebRTC but never reached
		// an agent (no transfer connected), mark as missed instead of completed.
		finalStatus := models.CallStatusCompleted
		if callLog.Direction == models.CallDirectionIncoming && callLog.AgentID == nil &&
			callLog.Status != models.CallStatusTransferring {
			finalStatus = models.CallStatusMissed
		}

		a.DB.Model(callLog).Updates(map[string]any{
			"status":   finalStatus,
			"ended_at": now,
			"duration": duration,
		})

		// Notify CallManager to clean up
		if a.CallManager != nil {
			a.CallManager.EndCall(ce.ID)
		}

		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallEnded, map[string]any{
			"call_id":    ce.ID,
			"contact_id": contact.ID.String(),
			"status":     string(finalStatus),
			"duration":   duration,
			"ended_at":   now.Format(time.RFC3339),
		})

	case "missed", "unanswered":
		a.DB.Model(callLog).Updates(map[string]any{
			"status":   models.CallStatusMissed,
			"ended_at": now,
		})

		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallEnded, map[string]any{
			"call_id":    ce.ID,
			"contact_id": contact.ID.String(),
			"status":     string(models.CallStatusMissed),
			"ended_at":   now.Format(time.RFC3339),
		})

	default:
		a.Log.Warn("Unknown call event", "event", ce.Event, "call_id", ce.ID)
	}

	// Handle error in call event
	if ce.Error != nil {
		a.DB.Model(&models.CallLog{}).
			Where("whatsapp_call_id = ? AND organization_id = ?", ce.ID, account.OrganizationID).
			Updates(map[string]any{
				"status":        models.CallStatusFailed,
				"error_message": ce.Error.Message,
				"ended_at":      now,
			})
	}
}

// getOrCreateCallLog finds an existing CallLog by WhatsApp call ID, or creates one
// if it doesn't exist. This handles cases where WhatsApp skips the "ringing" event
// and sends "connect" as the first event.
func (a *App) getOrCreateCallLog(account *models.WhatsAppAccount, contact *models.Contact, callID, callerPhone string, now time.Time) *models.CallLog {
	var callLog models.CallLog
	err := a.DB.Where("whatsapp_call_id = ? AND organization_id = ?", callID, account.OrganizationID).
		First(&callLog).Error
	if err == nil {
		return &callLog
	}

	// Create a new call log
	callLog = models.CallLog{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  account.OrganizationID,
		WhatsAppAccount: account.Name,
		ContactID:       contact.ID,
		WhatsAppCallID:  callID,
		CallerPhone:     callerPhone,
		Status:          models.CallStatusRinging,
		StartedAt:       &now,
	}

	// Find the call-start IVR flow for this account (must also be enabled)
	var ivrFlow models.IVRFlow
	if err := a.DB.Where("organization_id = ? AND whatsapp_account = ? AND is_call_start = ? AND is_active = ? AND deleted_at IS NULL",
		account.OrganizationID, account.Name, true, true).First(&ivrFlow).Error; err == nil {
		callLog.IVRFlowID = &ivrFlow.ID
	}

	if err := a.DB.Create(&callLog).Error; err != nil {
		a.Log.Error("Failed to create call log", "error", err)
		return nil
	}

	a.Log.Info("Created call log", "call_id", callID, "call_log_id", callLog.ID)
	return &callLog
}

// handleOrphanedOutgoingCallEvent handles business-initiated call webhooks
// when the session has already been cleaned up (e.g., terminate arrives after
// PeerConnection closed). Updates the call log and broadcasts WebSocket events.
func (a *App) handleOrphanedOutgoingCallEvent(phoneNumberID, callID, event string, duration int) {
	// Find the call log by WhatsApp call ID
	var callLog models.CallLog
	if err := a.DB.Where("whatsapp_call_id = ?", callID).First(&callLog).Error; err != nil {
		a.Log.Debug("No call log found for orphaned outgoing event", "call_id", callID, "event", event)
		return
	}

	now := time.Now()

	switch event {
	case "terminate":
		finalStatus := models.CallStatusCompleted
		if callLog.AnsweredAt == nil {
			finalStatus = models.CallStatusMissed
		}

		updates := map[string]any{
			"status":   finalStatus,
			"ended_at": now,
		}
		if duration > 0 {
			updates["duration"] = duration
		}
		a.DB.Model(&callLog).Updates(updates)

		a.broadcastCallEvent(callLog.OrganizationID, websocket.TypeOutgoingCallEnded, map[string]any{
			"call_log_id": callLog.ID.String(),
			"call_id":     callID,
			"status":      string(finalStatus),
			"duration":    duration,
			"ended_at":    now.Format(time.RFC3339),
		})

		a.Log.Info("Handled orphaned outgoing call terminate", "call_id", callID, "duration", duration)
	default:
		a.Log.Debug("Ignoring orphaned outgoing call event", "call_id", callID, "event", event)
	}
}

// processCallStatusWebhook handles business-initiated call status webhooks
// (RINGING, ACCEPTED, REJECTED) that arrive in the statuses array under field="calls".
func (a *App) processCallStatusWebhook(status WebhookStatus) {
	if a.CallManager == nil {
		return
	}

	// Map uppercase status to event name used by HandleOutgoingCallWebhook
	var event string
	switch status.Status {
	case "RINGING":
		event = "ringing"
	case "ACCEPTED":
		event = "accepted"
	case "REJECTED":
		event = "rejected"
	default:
		a.Log.Warn("Unknown call status", "status", status.Status, "call_id", status.ID)
		return
	}

	a.CallManager.HandleOutgoingCallWebhook(status.ID, event, "")
}

// CallPermissionReplyData holds the parsed call_permission_reply webhook data.
type CallPermissionReplyData struct {
	Response            string `json:"response"`
	IsPermanent         bool   `json:"is_permanent"`
	ExpirationTimestamp int64  `json:"expiration_timestamp,omitempty"`
	ResponseSource      string `json:"response_source"`
}

// processCallPermissionReply handles the call_permission_reply interactive webhook.
// Updates the CallPermission record in the DB when the user accepts or rejects.
func (a *App) processCallPermissionReply(phoneNumberID, fromPhone string, reply *CallPermissionReplyData) {
	account, err := a.getWhatsAppAccountCached(phoneNumberID)
	if err != nil {
		a.Log.Error("Failed to find account for call permission reply", "error", err)
		return
	}

	// Find the most recent pending permission for this contact
	var contact models.Contact
	if err := a.DB.Where("organization_id = ? AND phone_number = ?", account.OrganizationID, fromPhone).
		First(&contact).Error; err != nil {
		a.Log.Warn("No contact found for call permission reply", "phone", fromPhone)
		return
	}

	var permission models.CallPermission
	if err := a.DB.Where("organization_id = ? AND contact_id = ?", account.OrganizationID, contact.ID).
		Order("created_at DESC").
		First(&permission).Error; err != nil {
		a.Log.Warn("No permission record found for call permission reply", "contact_id", contact.ID)
		return
	}

	now := time.Now()
	updates := map[string]any{
		"responded_at": now,
	}

	if reply.Response == "accept" {
		updates["status"] = models.CallPermissionAccepted
		a.Log.Info("Call permission accepted",
			"contact_id", contact.ID,
			"is_permanent", reply.IsPermanent,
			"expiration", reply.ExpirationTimestamp,
		)
	} else {
		updates["status"] = models.CallPermissionDeclined
		a.Log.Info("Call permission declined", "contact_id", contact.ID)
	}

	a.DB.Model(&permission).Updates(updates)
}

// broadcastCallEvent sends a call event to all connected clients in an organization
func (a *App) broadcastCallEvent(orgID uuid.UUID, eventType string, payload map[string]any) {
	if a.WSHub == nil {
		return
	}
	a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
		Type:    eventType,
		Payload: payload,
	})
}
