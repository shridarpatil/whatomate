package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/contactutil"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"gorm.io/gorm"
)

// processCallWebhook handles a call webhook event for both incoming and outgoing calls.
// It creates/updates the CallLog and delegates to the CallManager for WebRTC handling.
func (a *App) processCallWebhook(phoneNumberID string, call any) {
	// The webhook handler passes an anonymous struct. Convert via JSON round-trip.
	type callEvent struct {
		ID         string `json:"id"`
		From       string `json:"from"`
		FromUserID string `json:"from_user_id,omitempty"` // BSUID
		To         string `json:"to"`
		ToUserID   string `json:"to_user_id,omitempty"` // BSUID
		Timestamp  string `json:"timestamp"`
		Type       string `json:"type"`
		Event      string `json:"event"`
		Direction  string `json:"direction,omitempty"`
		Session    *struct {
			SDPType string `json:"sdp_type"`
			SDP     string `json:"sdp"`
		} `json:"session,omitempty"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
		Duration int `json:"duration,omitempty"`
		// BizOpaqueCallbackData is the opaque string we set as `payload` on a
		// voice_call interactive button. Meta echoes it back here when the
		// customer taps the button. Carries `agent:<uuid>` for sticky routing;
		// parsed and acted on in a later PR — for now just logged so we can
		// confirm Meta is round-tripping the value.
		BizOpaqueCallbackData string `json:"biz_opaque_callback_data,omitempty"`
	}

	var ce callEvent
	b, _ := json.Marshal(call)
	if err := json.Unmarshal(b, &ce); err != nil {
		a.Log.Error("Failed to parse call event", "error", err)
		return
	}

	// Log raw payload to debug SDP and field mapping
	a.Log.Debug("Raw call webhook payload", "payload", string(b))

	// Surface the voice_call payload at info level so it shows up in
	// production logs ahead of sticky-routing landing. Quiet when the
	// caller didn't initiate via a voice_call button.
	if ce.BizOpaqueCallbackData != "" {
		a.Log.Info("Incoming call carries biz_opaque_callback_data",
			"call_id", ce.ID, "payload", ce.BizOpaqueCallbackData)
	}

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
		a.handleOrphanedOutgoingCallEvent(ce.ID, ce.Event, ce.Duration)
		return
	}

	// --- Incoming call flow ---

	// Look up the WhatsApp account
	account, err := a.getWhatsAppAccountCached(phoneNumberID)
	if err != nil {
		a.Log.Error("Failed to find WhatsApp account for call", "error", err, "phone_id", phoneNumberID)
		return
	}

	// Skip if phone number is missing (username user — BSUID-only calling not yet supported)
	if ce.From == "" {
		a.Log.Warn("Incoming call without phone number (username user), skipping",
			"bsuid", ce.FromUserID, "call_id", ce.ID)
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
		payload := map[string]any{
			"call_log_id":  callLog.ID.String(),
			"call_id":      ce.ID,
			"caller_phone": ce.From,
			"contact_id":   contact.ID.String(),
			"contact_name": contact.ProfileName,
			"ivr_flow_id":  callLog.IVRFlowID,
			"started_at":   now.Format(time.RFC3339),
		}
		// Sticky routing: if the customer clicked a voice_call button whose
		// payload tags the originating agent, ring just that agent. Falls
		// back to the org-wide broadcast on any failure (malformed payload,
		// wrong org, agent offline / unavailable).
		stickyAgentID := a.resolveStickyAgent(context.Background(), ce.BizOpaqueCallbackData, account.OrganizationID, contact.PhoneNumber)
		if stickyAgentID != nil {
			payload["sticky_agent_id"] = stickyAgentID.String()
			a.Log.Info("Sticky-routing incoming call to originating agent",
				"call_id", ce.ID, "agent_id", *stickyAgentID)
			a.WSHub.BroadcastToUser(account.OrganizationID, *stickyAgentID, websocket.WSMessage{
				Type:    websocket.TypeCallIncoming,
				Payload: payload,
			})
		} else {
			a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallIncoming, payload)
		}

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

		// Delegate to CallManager with the SDP offer. Resolve the sticky
		// agent again here — Meta echoes biz_opaque_callback_data on every
		// call event, so we don't need to plumb state across the ringing →
		// connect gap.
		if a.IsCallingEnabledForOrg(account.OrganizationID) && sdpOffer != "" {
			session := a.CallManager.GetSession(ce.ID)
			if session == nil {
				stickyAgentID := a.resolveStickyAgent(context.Background(), ce.BizOpaqueCallbackData, account.OrganizationID, contact.PhoneNumber)
				a.CallManager.HandleIncomingCall(account, contact, callLog, sdpOffer, stickyAgentID)
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

		updates := map[string]any{
			"status":   finalStatus,
			"ended_at": now,
			"duration": duration,
		}
		// Only set disconnected_by if not already set (agent hangup sets it first)
		if callLog.DisconnectedBy == "" {
			updates["disconnected_by"] = models.DisconnectedByClient
		}
		a.DB.Model(callLog).Updates(updates)

		// Notify CallManager to clean up
		if a.CallManager != nil {
			a.CallManager.EndCall(ce.ID)
		}

		disconnectedBy := string(callLog.DisconnectedBy)
		if disconnectedBy == "" {
			disconnectedBy = "client"
		}
		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallEnded, map[string]any{
			"call_id":         ce.ID,
			"contact_id":      contact.ID.String(),
			"status":          string(finalStatus),
			"duration":        duration,
			"ended_at":        now.Format(time.RFC3339),
			"disconnected_by": disconnectedBy,
		})

	case "missed", "unanswered":
		a.DB.Model(callLog).Updates(map[string]any{
			"status":          models.CallStatusMissed,
			"ended_at":        now,
			"disconnected_by": models.DisconnectedByClient,
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
				"status":          models.CallStatusFailed,
				"error_message":   ce.Error.Message,
				"ended_at":        now,
				"disconnected_by": models.DisconnectedBySystem,
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

	// Find the call-start IVR flow for this account (cached)
	if flow := a.CallManager.GetIVRFlowByConfig(account.OrganizationID, account.Name, "call_start"); flow != nil {
		callLog.IVRFlowID = &flow.ID
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
func (a *App) handleOrphanedOutgoingCallEvent(callID, event string, duration int) {
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
		// Only set disconnected_by if not already set (agent hangup sets it first)
		if callLog.DisconnectedBy == "" {
			updates["disconnected_by"] = models.DisconnectedByClient
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

	// Load the most recent permission request for this contact. If none exists
	// (e.g. the permission prompt was sent out-of-band by Meta, or the request
	// went out while outside business calling hours), create one from the reply
	// so the grant/decline is captured instead of being dropped.
	var permission models.CallPermission
	isNewPermission := false
	if err := a.DB.Where("organization_id = ? AND contact_id = ?", account.OrganizationID, contact.ID).
		Order("created_at DESC").
		First(&permission).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			a.Log.Error("Failed to load call permission for reply", "error", err, "contact_id", contact.ID)
			return
		}
		isNewPermission = true
		permission = models.CallPermission{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  account.OrganizationID,
			ContactID:       contact.ID,
			WhatsAppAccount: account.Name,
			Status:          models.CallPermissionPending,
		}
		a.Log.Info("No prior permission record for reply; creating one (out-of-band grant)", "contact_id", contact.ID)
	}

	now := time.Now()
	permission.RespondedAt = &now

	var expiresAt *time.Time
	if reply.Response == "accept" {
		permission.Status = models.CallPermissionAccepted
		if reply.ExpirationTimestamp > 0 {
			t := time.Unix(reply.ExpirationTimestamp, 0)
			expiresAt = &t
			permission.ExpiresAt = &t
		}
		a.Log.Info("Call permission accepted",
			"contact_id", contact.ID,
			"is_permanent", reply.IsPermanent,
			"expiration", reply.ExpirationTimestamp,
		)
	} else {
		permission.Status = models.CallPermissionDeclined
		a.Log.Info("Call permission declined", "contact_id", contact.ID)
	}

	if isNewPermission {
		if err := a.DB.Create(&permission).Error; err != nil {
			a.Log.Error("Failed to create call permission from reply", "error", err, "contact_id", contact.ID)
			return
		}
	} else {
		updates := map[string]any{
			"status":       permission.Status,
			"responded_at": now,
		}
		if expiresAt != nil {
			updates["expires_at"] = *expiresAt
		}
		a.DB.Model(&permission).Updates(updates)
	}

	// Broadcast permission update to agents via WebSocket
	wsPayload := map[string]any{
		"contact_id":    contact.ID,
		"contact_phone": contact.PhoneNumber,
		"contact_name":  contact.ProfileName,
		"status":        permission.Status,
	}
	if expiresAt != nil {
		wsPayload["expires_at"] = expiresAt.Format(time.RFC3339)
	}
	a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallPermissionUpdate, wsPayload)
}

// validateStickyAgent runs the per-call eligibility checks (same-org,
// IsActive, IsAvailable, online) on a candidate agent. Returns the id on
// pass, nil on fail (with the reason logged). Used by both sticky-agent
// sources in resolveStickyAgent.
func (a *App) validateStickyAgent(agentID, orgID uuid.UUID) *uuid.UUID {
	var user models.User
	if err := a.DB.Where(
		"id = ? AND organization_id = ? AND is_active = ? AND is_available = ?",
		agentID, orgID, true, true,
	).First(&user).Error; err != nil {
		a.Log.Info("Sticky-route skipped: agent not eligible",
			"agent_id", agentID, "org_id", orgID, "reason", err.Error())
		return nil
	}
	if a.WSHub == nil || !a.WSHub.IsUserOnline(orgID, agentID) {
		a.Log.Info("Sticky-route skipped: agent offline",
			"agent_id", agentID)
		return nil
	}
	return &agentID
}

// stickyCallKey returns the Redis key for a pending voice_call sticky
// route. Keyed by (org, caller-phone) because the incoming-call webhook
// gives us the phone before any contact lookup, and it's the same value
// the sender writes (contact.PhoneNumber).
func stickyCallKey(orgID uuid.UUID, phone string) string {
	return "vc_sticky:" + orgID.String() + ":" + phone
}

// MarkPendingStickyCall stores the originating agent id when an outbound
// voice_call button is sent, with a TTL matching the button's clickable
// lifetime. When the customer taps the button and our number rings, the
// call_webhook handler reads this back to route directly to the agent
// who sent it.
//
// Best-effort: a Redis failure logs and degrades to today's default
// (org-wide broadcast + IVR). Doesn't error out a successful send.
func (a *App) MarkPendingStickyCall(ctx context.Context, orgID uuid.UUID, phone string, agentID uuid.UUID, ttlMinutes int) {
	if a.Redis == nil || phone == "" {
		return
	}
	if ttlMinutes <= 0 {
		ttlMinutes = 15 // Meta's default for voice_call buttons
	}
	if err := a.Redis.Set(ctx, stickyCallKey(orgID, phone), agentID.String(),
		time.Duration(ttlMinutes)*time.Minute).Err(); err != nil {
		a.Log.Warn("Failed to mark pending sticky call in Redis",
			"error", err, "phone", phone)
	}
}

// findStickyAgentInRedis returns the agent id stored when the outbound
// voice_call button was sent (set by MarkPendingStickyCall), or nil if
// no key, expired, malformed, or Redis is unhealthy. Nil is a graceful
// signal — the caller falls through to today's default routing.
func (a *App) findStickyAgentInRedis(ctx context.Context, orgID uuid.UUID, phone string) *uuid.UUID {
	if a.Redis == nil || phone == "" {
		return nil
	}
	val, err := a.Redis.Get(ctx, stickyCallKey(orgID, phone)).Result()
	if err != nil {
		return nil
	}
	agentID, err := uuid.Parse(val)
	if err != nil {
		a.Log.Warn("Stored sticky-call agent id is malformed", "value", val)
		return nil
	}
	return &agentID
}

// resolveStickyAgent picks the agent (if any) who should receive this
// incoming call. Two sources are tried in order:
//
//  1. The voice_call button's `payload` echoed back by Meta. As of
//     2026-05, Meta does not surface this on the call webhook; we keep
//     the parsing for forward-compat in case they add it later.
//  2. Redis: a key set by MarkPendingStickyCall when the outbound
//     button was sent, with a TTL matching the button's clickable
//     lifetime.
//
// Either source's result goes through validateStickyAgent so the agent
// must still be in the same org, on-shift, and online. On any failure
// return nil and let the caller fall back to today's org-wide broadcast.
//
// Why Redis instead of a DB lookup: O(1) GET vs an unindexed JSONB scan
// of `messages`, and the TTL is enforced by Redis natively (no
// "last 60 min" window math).
func (a *App) resolveStickyAgent(ctx context.Context, rawPayload string, orgID uuid.UUID, callerPhone string) *uuid.UUID {
	// Source 1: Meta-echoed payload.
	if suffix, ok := strings.CutPrefix(rawPayload, "agent:"); ok {
		if agentID, err := uuid.Parse(suffix); err == nil {
			if validated := a.validateStickyAgent(agentID, orgID); validated != nil {
				a.Log.Info("Sticky-route: matched on Meta-echoed payload",
					"agent_id", agentID, "phone", callerPhone)
				return validated
			}
		} else {
			a.Log.Info("Sticky-route: malformed agent id in payload",
				"payload", rawPayload, "error", err)
		}
	}

	// Source 2: pending sticky-call key set when the button was sent.
	if originator := a.findStickyAgentInRedis(ctx, orgID, callerPhone); originator != nil {
		if validated := a.validateStickyAgent(*originator, orgID); validated != nil {
			a.Log.Info("Sticky-route: matched on Redis pending key",
				"agent_id", *originator, "phone", callerPhone)
			return validated
		}
	}

	return nil
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
