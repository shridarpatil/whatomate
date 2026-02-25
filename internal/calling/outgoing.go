package calling

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
)

// InitiateOutgoingCall sets up WebRTC between the agent browser and
// the WhatsApp Cloud API, and places the outgoing call.
// Returns the call log ID and the SDP answer for the agent's browser.
func (m *Manager) InitiateOutgoingCall(
	orgID, agentID, contactID uuid.UUID,
	contactPhone, accountName string,
	waAccount *whatsapp.Account,
	agentSDPOffer string,
) (uuid.UUID, string, error) {

	now := time.Now()

	// 1. Create CallLog
	callLog := models.CallLog{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: accountName,
		ContactID:       contactID,
		CallerPhone:     contactPhone,
		Direction:       models.CallDirectionOutgoing,
		Status:          models.CallStatusInitiating,
		AgentID:         &agentID,
		StartedAt:       &now,
	}
	if err := m.db.Create(&callLog).Error; err != nil {
		return uuid.Nil, "", fmt.Errorf("failed to create call log: %w", err)
	}

	// 2. Create agent PeerConnection
	agentPC, err := m.createPeerConnection()
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("failed to create agent PC: %w", err)
	}

	// 3. Add local audio track for server → agent
	agentLocalTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"server-to-agent",
	)
	if err != nil {
		_ = agentPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to create agent local track: %w", err)
	}
	if _, err := agentPC.AddTrack(agentLocalTrack); err != nil {
		_ = agentPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to add agent local track: %w", err)
	}

	// Create session early so OnTrack can reference it
	session := &CallSession{
		OrganizationID: orgID,
		AccountName:    accountName,
		CallerPhone:    contactPhone,
		ContactID:      contactID,
		CallLogID:      callLog.ID,
		Status:         models.CallStatusInitiating,
		StartedAt:      now,
		Direction:      models.CallDirectionOutgoing,
		AgentID:        agentID,
		TargetPhone:    contactPhone,
		SDPAnswerReady: make(chan string, 1),
		BridgeStarted:  make(chan struct{}),
	}

	// 4. Capture agent's remote track
	agentPC.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		m.log.Info("Received agent remote track",
			"call_log_id", callLog.ID,
			"codec", track.Codec().MimeType,
		)
		if track.Codec().MimeType == "audio/telephone-event" {
			return
		}
		session.mu.Lock()
		session.AgentRemoteTrack = track
		session.mu.Unlock()

		// Consume until bridge takes over
		go m.consumeAudioTrack(session, track)
	})

	agentPC.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		m.log.Info("Agent PC state changed",
			"call_log_id", callLog.ID,
			"state", state.String(),
		)
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateDisconnected {
			if session.ID != "" {
				m.EndCall(session.ID)
			}
		}
	})

	// 5. Set agent's SDP offer as remote description
	if err := agentPC.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  agentSDPOffer,
	}); err != nil {
		_ = agentPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to set agent remote desc: %w", err)
	}

	// 6. Create SDP answer for agent
	agentAnswer, err := agentPC.CreateAnswer(nil)
	if err != nil {
		_ = agentPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to create agent answer: %w", err)
	}
	if err := agentPC.SetLocalDescription(agentAnswer); err != nil {
		_ = agentPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to set agent local desc: %w", err)
	}

	// Wait for ICE gathering
	gatherComplete := webrtc.GatheringCompletePromise(agentPC)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	select {
	case <-gatherComplete:
	case <-ctx.Done():
		_ = agentPC.Close()
		return uuid.Nil, "", fmt.Errorf("agent ICE gathering timed out")
	}

	agentSDP := agentPC.LocalDescription()
	if agentSDP == nil {
		_ = agentPC.Close()
		return uuid.Nil, "", fmt.Errorf("no agent local description")
	}

	// 7. Create WhatsApp PeerConnection
	waPC, err := m.createPeerConnection()
	if err != nil {
		_ = agentPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to create WA PC: %w", err)
	}

	// 8. Add local audio track for server → WhatsApp
	waLocalTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"server-to-wa",
	)
	if err != nil {
		_ = agentPC.Close()
		_ = waPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to create WA local track: %w", err)
	}
	if _, err := waPC.AddTrack(waLocalTrack); err != nil {
		_ = agentPC.Close()
		_ = waPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to add WA local track: %w", err)
	}

	// 9. Capture WhatsApp's remote track → start bridge
	waPC.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		m.log.Info("Received WA remote track",
			"call_log_id", callLog.ID,
			"codec", track.Codec().MimeType,
		)
		if track.Codec().MimeType == "audio/telephone-event" {
			return
		}
		session.mu.Lock()
		session.WARemoteTrack = track
		agentRemote := session.AgentRemoteTrack
		session.mu.Unlock()

		// Start audio bridge when both remote tracks are ready
		if agentRemote != nil {
			m.startOutgoingBridge(session, track, agentRemote, agentLocalTrack, waLocalTrack)
		}
	})

	waPC.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		m.log.Info("WA PC state changed",
			"call_log_id", callLog.ID,
			"state", state.String(),
		)
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateDisconnected {
			if session.ID != "" {
				m.EndCall(session.ID)
			}
		}
	})

	// 10. Create SDP offer for WhatsApp
	waOffer, err := waPC.CreateOffer(nil)
	if err != nil {
		_ = agentPC.Close()
		_ = waPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to create WA offer: %w", err)
	}
	if err := waPC.SetLocalDescription(waOffer); err != nil {
		_ = agentPC.Close()
		_ = waPC.Close()
		return uuid.Nil, "", fmt.Errorf("failed to set WA local desc: %w", err)
	}

	waGatherComplete := webrtc.GatheringCompletePromise(waPC)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	select {
	case <-waGatherComplete:
	case <-ctx2.Done():
		_ = agentPC.Close()
		_ = waPC.Close()
		return uuid.Nil, "", fmt.Errorf("WA ICE gathering timed out")
	}

	waLocalDesc := waPC.LocalDescription()
	if waLocalDesc == nil {
		_ = agentPC.Close()
		_ = waPC.Close()
		return uuid.Nil, "", fmt.Errorf("no WA local description")
	}

	// 11. Call WhatsApp API to initiate the call
	callCtx, callCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer callCancel()

	callID, err := m.whatsapp.InitiateCall(callCtx, waAccount, contactPhone, waLocalDesc.SDP)
	if err != nil {
		_ = agentPC.Close()
		_ = waPC.Close()
		// Update call log as failed
		m.db.Model(&callLog).Updates(map[string]any{
			"status":        models.CallStatusFailed,
			"error_message": err.Error(),
			"ended_at":      time.Now(),
		})
		return uuid.Nil, "", fmt.Errorf("failed to initiate call via API: %w", err)
	}

	// Update call log with WhatsApp call ID
	m.db.Model(&callLog).Update("whatsapp_call_id", callID)

	// 12. Complete session setup
	session.ID = callID
	session.PeerConnection = agentPC // agent's PC
	session.AgentPC = agentPC
	session.AgentAudioTrack = agentLocalTrack
	session.WAPeerConn = waPC
	session.WAAudioTrack = waLocalTrack

	// 13. Store session
	m.mu.Lock()
	m.sessions[callID] = session
	m.mu.Unlock()

	// 14. Wait for SDP answer from webhook
	go m.waitForWASDPAnswer(session, waPC)

	// 15. Broadcast event
	m.broadcastOutgoingEvent(orgID, websocket.TypeOutgoingCallInitiated, map[string]any{
		"call_log_id":    callLog.ID.String(),
		"call_id":        callID,
		"contact_id":     contactID.String(),
		"contact_phone":  contactPhone,
		"agent_id":       agentID.String(),
		"started_at":     now.Format(time.RFC3339),
	})

	return callLog.ID, agentSDP.SDP, nil
}

// waitForWASDPAnswer waits for the SDP answer from the webhook and sets it
// on the WhatsApp PeerConnection.
func (m *Manager) waitForWASDPAnswer(session *CallSession, waPC *webrtc.PeerConnection) {
	select {
	case sdpAnswer := <-session.SDPAnswerReady:
		if err := waPC.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  sdpAnswer,
		}); err != nil {
			m.log.Error("Failed to set WA remote description",
				"error", err,
				"call_id", session.ID,
			)
		} else {
			m.log.Info("WA SDP answer set successfully", "call_id", session.ID)
		}
	case <-time.After(30 * time.Second):
		m.log.Error("Timed out waiting for WA SDP answer", "call_id", session.ID)
		m.cleanupSession(session.ID)
	}
}

// HandleOutgoingCallWebhook processes webhook events for outgoing calls.
func (m *Manager) HandleOutgoingCallWebhook(callID, event, sdpAnswer string) {
	m.mu.RLock()
	session, exists := m.sessions[callID]
	m.mu.RUnlock()

	if !exists {
		m.log.Warn("Outgoing call webhook for unknown session", "call_id", callID, "event", event)
		return
	}

	// Deliver SDP answer if present
	if sdpAnswer != "" {
		select {
		case session.SDPAnswerReady <- sdpAnswer:
			m.log.Info("Delivered SDP answer to session", "call_id", callID)
		default:
			m.log.Warn("SDP answer channel full, ignoring", "call_id", callID)
		}
	}

	now := time.Now()

	switch event {
	case "ringing":
		session.mu.Lock()
		session.Status = models.CallStatusRinging
		session.mu.Unlock()

		m.db.Model(&models.CallLog{}).
			Where("id = ?", session.CallLogID).
			Update("status", models.CallStatusRinging)

		m.broadcastOutgoingEvent(session.OrganizationID, websocket.TypeOutgoingCallRinging, map[string]any{
			"call_log_id":   session.CallLogID.String(),
			"call_id":       callID,
			"contact_id":    session.ContactID.String(),
			"contact_phone": session.TargetPhone,
		})

	case "accepted", "in_call", "connect":
		session.mu.Lock()
		session.Status = models.CallStatusAnswered
		session.mu.Unlock()

		m.db.Model(&models.CallLog{}).
			Where("id = ?", session.CallLogID).
			Updates(map[string]any{
				"status":      models.CallStatusAnswered,
				"answered_at": now,
			})

		m.broadcastOutgoingEvent(session.OrganizationID, websocket.TypeOutgoingCallAnswered, map[string]any{
			"call_log_id":   session.CallLogID.String(),
			"call_id":       callID,
			"contact_id":    session.ContactID.String(),
			"contact_phone": session.TargetPhone,
			"answered_at":   now.Format(time.RFC3339),
		})

	case "rejected":
		m.db.Model(&models.CallLog{}).
			Where("id = ?", session.CallLogID).
			Updates(map[string]any{
				"status":   models.CallStatusRejected,
				"ended_at": now,
			})

		m.broadcastOutgoingEvent(session.OrganizationID, websocket.TypeOutgoingCallRejected, map[string]any{
			"call_log_id":   session.CallLogID.String(),
			"call_id":       callID,
			"contact_id":    session.ContactID.String(),
			"contact_phone": session.TargetPhone,
		})

		m.cleanupSession(callID)

	case "ended", "terminated", "terminate":
		// Calculate duration
		var callLog models.CallLog
		if err := m.db.Where("id = ?", session.CallLogID).First(&callLog).Error; err == nil {
			duration := 0
			if callLog.AnsweredAt != nil {
				duration = int(now.Sub(*callLog.AnsweredAt).Seconds())
			}
			m.db.Model(&callLog).Updates(map[string]any{
				"status":   models.CallStatusCompleted,
				"ended_at": now,
				"duration": duration,
			})
		}

		m.broadcastOutgoingEvent(session.OrganizationID, websocket.TypeOutgoingCallEnded, map[string]any{
			"call_log_id":   session.CallLogID.String(),
			"call_id":       callID,
			"contact_id":    session.ContactID.String(),
			"contact_phone": session.TargetPhone,
			"ended_at":      now.Format(time.RFC3339),
		})

		m.cleanupSession(callID)
	}
}

// HangupOutgoingCall terminates an outgoing call initiated by an agent.
func (m *Manager) HangupOutgoingCall(callLogID, agentID uuid.UUID) error {
	session := m.GetSessionByCallLogID(callLogID)
	if session == nil {
		return fmt.Errorf("session not found for call log %s", callLogID)
	}

	if session.Direction != models.CallDirectionOutgoing {
		return fmt.Errorf("not an outgoing call")
	}

	// Terminate via WhatsApp API
	waAccount := &whatsapp.Account{
		PhoneID:     "", // will be looked up from the stored session
		AccessToken: "",
	}

	// Look up account from DB
	var account models.WhatsAppAccount
	if err := m.db.Where("organization_id = ? AND name = ?", session.OrganizationID, session.AccountName).
		First(&account).Error; err == nil {
		waAccount.PhoneID = account.PhoneID
		waAccount.BusinessID = account.BusinessID
		waAccount.APIVersion = account.APIVersion
		waAccount.AccessToken = account.AccessToken
	}

	if waAccount.AccessToken != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := m.whatsapp.TerminateCall(ctx, waAccount, session.ID); err != nil {
			m.log.Error("Failed to terminate outgoing call via API", "error", err, "call_id", session.ID)
		}
	}

	now := time.Now()

	// Calculate duration
	var callLog models.CallLog
	if err := m.db.Where("id = ?", callLogID).First(&callLog).Error; err == nil {
		duration := 0
		if callLog.AnsweredAt != nil {
			duration = int(now.Sub(*callLog.AnsweredAt).Seconds())
		}
		m.db.Model(&callLog).Updates(map[string]any{
			"status":   models.CallStatusCompleted,
			"ended_at": now,
			"duration": duration,
		})
	}

	m.broadcastOutgoingEvent(session.OrganizationID, websocket.TypeOutgoingCallEnded, map[string]any{
		"call_log_id":   callLogID.String(),
		"call_id":       session.ID,
		"contact_id":    session.ContactID.String(),
		"contact_phone": session.TargetPhone,
		"ended_at":      now.Format(time.RFC3339),
	})

	m.cleanupSession(session.ID)
	return nil
}

// startOutgoingBridge starts bidirectional audio forwarding between the agent
// and WhatsApp remote tracks.
func (m *Manager) startOutgoingBridge(
	session *CallSession,
	waRemote *webrtc.TrackRemote,
	agentRemote *webrtc.TrackRemote,
	agentLocal *webrtc.TrackLocalStaticRTP,
	waLocal *webrtc.TrackLocalStaticRTP,
) {
	// Signal that bridge is taking over
	select {
	case <-session.BridgeStarted:
	default:
		close(session.BridgeStarted)
	}

	recorder := m.newRecorderIfEnabled()
	bridge := NewAudioBridge(recorder)
	session.mu.Lock()
	session.Bridge = bridge
	session.Recorder = recorder
	session.mu.Unlock()

	m.log.Info("Starting outgoing call audio bridge", "call_id", session.ID)

	// WA audio → Agent speaker, Agent mic → WA speaker
	go bridge.Start(waRemote, agentLocal, agentRemote, waLocal)
}

// broadcastOutgoingEvent broadcasts an outgoing call event via WebSocket.
func (m *Manager) broadcastOutgoingEvent(orgID uuid.UUID, eventType string, payload map[string]any) {
	if m.wsHub == nil {
		return
	}
	m.wsHub.BroadcastToOrg(orgID, websocket.WSMessage{
		Type:    eventType,
		Payload: payload,
	})
}
