package calling

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
)

// initiateTransfer starts the transfer flow: puts caller on hold, notifies agents via WebSocket.
func (m *Manager) initiateTransfer(session *CallSession, waAccount string, teamTarget string, ivrPath []map[string]string) {
	// Load org-level calling overrides once
	orgSettings := m.getOrgCallingSettings(session.OrganizationID)

	// Reuse the IVR player for hold music so RTP sequence numbers continue
	// from where the IVR left off. A new player starting at seq=0 would be
	// dropped by the receiver as "old" until seq exceeds the IVR high-water mark.
	session.mu.Lock()
	player := session.IVRPlayer
	if player == nil || player.IsStopped() {
		player = NewAudioPlayer(session.AudioTrack)
	}
	session.HoldPlayer = player
	session.mu.Unlock()

	go func() {
		_ = player.PlayFileLoop(orgSettings.HoldMusicFile)
	}()

	var teamID *uuid.UUID
	if teamTarget != "" {
		if parsed, err := uuid.Parse(teamTarget); err == nil {
			teamID = &parsed
		}
	}

	// Create CallTransfer record
	transfer := models.CallTransfer{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  session.OrganizationID,
		CallLogID:       session.CallLogID,
		WhatsAppCallID:  session.ID,
		CallerPhone:     session.CallerPhone,
		ContactID:       session.ContactID,
		WhatsAppAccount: waAccount,
		Status:          models.CallTransferStatusWaiting,
		TeamID:          teamID,
		TransferredAt:   time.Now(),
	}

	// Save IVR path
	if len(ivrPath) > 0 {
		transfer.IVRPath = models.JSONB{"steps": ivrPath}
	}

	if err := m.db.Create(&transfer).Error; err != nil {
		m.log.Error("Failed to create call transfer", "error", err, "call_id", session.ID)
		player.Stop()
		return
	}

	// Update call log status
	m.db.Model(&models.CallLog{}).
		Where("id = ?", session.CallLogID).
		Update("status", models.CallStatusTransferring)

	// Update session state
	session.mu.Lock()
	session.TransferID = transfer.ID
	session.TransferStatus = models.CallTransferStatusWaiting
	session.mu.Unlock()

	// Start timeout goroutine
	transferTimeout := orgSettings.TransferTimeoutSecs
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(transferTimeout)*time.Second)

	session.mu.Lock()
	session.TransferCancel = cancel
	session.mu.Unlock()

	go m.waitForTransferTimeout(ctx, session, transfer.ID)

	// Broadcast WebSocket event
	var teamIDStr string
	if teamID != nil {
		teamIDStr = teamID.String()
	}

	m.broadcastEvent(transfer.OrganizationID, websocket.TypeCallTransferWaiting, map[string]any{
		"id":               transfer.ID.String(),
		"call_log_id":      transfer.CallLogID.String(),
		"whatsapp_call_id": transfer.WhatsAppCallID,
		"caller_phone":     transfer.CallerPhone,
		"contact_id":       transfer.ContactID.String(),
		"whatsapp_account": transfer.WhatsAppAccount,
		"team_id":          teamIDStr,
		"transferred_at":   transfer.TransferredAt.Format(time.RFC3339),
	})

	m.log.Info("Call transfer initiated",
		"call_id", session.ID,
		"transfer_id", transfer.ID,
		"team_id", teamIDStr,
	)
}

// InitiateAgentTransfer allows a connected agent to transfer their active call
// to another team/agent. It tears down the current agent's bridge, puts the
// caller on hold, and creates a new CallTransfer record.
func (m *Manager) InitiateAgentTransfer(callLogID, initiatingAgentID uuid.UUID, teamID *uuid.UUID, targetAgentID *uuid.UUID) error {
	session := m.GetSessionByCallLogID(callLogID)
	if session == nil {
		return fmt.Errorf("no active session for call log %s", callLogID)
	}

	// Load org settings outside lock (DB query)
	orgSettings := m.getOrgCallingSettings(session.OrganizationID)

	session.mu.Lock()
	if session.TransferStatus == models.CallTransferStatusWaiting {
		session.mu.Unlock()
		return fmt.Errorf("call is already being transferred")
	}

	// Pick the correct caller track for hold music based on call direction.
	var holdTrack *webrtc.TrackLocalStaticRTP
	var callerRemote *webrtc.TrackRemote
	if session.Direction == models.CallDirectionOutgoing {
		holdTrack = session.WAAudioTrack
		callerRemote = session.WARemoteTrack
	} else {
		holdTrack = session.AudioTrack
		callerRemote = session.CallerRemoteTrack
	}

	if holdTrack == nil {
		session.mu.Unlock()
		return fmt.Errorf("no caller audio track available for hold music")
	}

	player := NewAudioPlayer(holdTrack)
	session.HoldPlayer = player

	// Snapshot and nil the agent-side resources so we can tear them down outside lock
	bridge := session.Bridge
	session.Bridge = nil
	agentPC := session.AgentPC
	session.AgentPC = nil
	session.AgentAudioTrack = nil
	session.AgentRemoteTrack = nil
	session.TransferID = uuid.Nil
	session.TransferStatus = models.CallTransferStatusWaiting
	session.BridgeStarted = make(chan struct{})
	session.mu.Unlock()

	// Stop bridge and close old agent PC outside lock.
	// Disable agentPC's OnConnectionStateChange BEFORE closing it to prevent
	// it from calling EndCall/EndTransfer which would destroy the session.
	if agentPC != nil {
		agentPC.OnConnectionStateChange(func(webrtc.PeerConnectionState) {})
	}
	if bridge != nil {
		bridge.Stop()
		bridge.Wait() // Wait for goroutines to finish so lastCallerSeq is final.

		// The bridge forwarded agent RTP with the agent's sequence numbers
		// (which are typically very high). Pion's Write() rewrites the SSRC
		// but preserves the original seq, so the receiver's high-water mark
		// is now at the agent's last seq. Advance the hold music player past
		// that point so the receiver doesn't drop hold music as "old".
		seq, ts := bridge.LastCallerSeq()
		if seq > 0 {
			player.SetSequence(seq, ts)
		}
	}
	if agentPC != nil {
		_ = agentPC.Close()
	}

	// Drain caller's remote track until the new bridge takes over.
	// After the bridge stops, nobody is reading from it and Pion's receive
	// buffer fills up, causing congestion feedback that degrades the
	// PeerConnection (including the ability to write hold music).
	if callerRemote != nil {
		go m.consumeAudioTrack(session, callerRemote)
	}

	// Start hold music now that the bridge is stopped and no longer writing
	// to the same track.
	m.log.Info("Starting hold music for agent transfer",
		"call_id", session.ID,
		"file", orgSettings.HoldMusicFile,
		"hold_track_nil", holdTrack == nil,
		"caller_remote_nil", callerRemote == nil,
		"bridge_was_nil", bridge == nil,
		"agent_pc_was_nil", agentPC == nil,
	)
	holdFile := orgSettings.HoldMusicFile
	go func() {
		m.log.Info("Hold music goroutine started", "call_id", session.ID, "file", holdFile)
		// Play first iteration manually to log packet count
		packets, err := player.PlayFile(holdFile)
		if err != nil {
			m.log.Error("Hold music first play failed",
				"error", err, "call_id", session.ID, "file", holdFile, "packets_sent", packets)
			return
		}
		m.log.Info("Hold music first loop done",
			"call_id", session.ID, "packets_sent", packets, "stopped", player.IsStopped())
		if player.IsStopped() {
			return
		}
		// Continue looping
		if err := player.PlayFileLoop(holdFile); err != nil {
			m.log.Error("Hold music playback failed during agent transfer",
				"error", err, "call_id", session.ID, "file", holdFile)
		} else {
			m.log.Info("Hold music stopped (no error)", "call_id", session.ID)
		}
	}()

	// Create CallTransfer record
	transfer := models.CallTransfer{
		BaseModel:         models.BaseModel{ID: uuid.New()},
		OrganizationID:    session.OrganizationID,
		CallLogID:         session.CallLogID,
		WhatsAppCallID:    session.ID,
		CallerPhone:       session.CallerPhone,
		ContactID:         session.ContactID,
		WhatsAppAccount:   session.AccountName,
		Status:            models.CallTransferStatusWaiting,
		TeamID:            teamID,
		InitiatingAgentID: &initiatingAgentID,
		TransferredAt:     time.Now(),
	}
	if targetAgentID != nil {
		transfer.AgentID = targetAgentID
	}

	if err := m.db.Create(&transfer).Error; err != nil {
		player.Stop()
		return fmt.Errorf("failed to create call transfer: %w", err)
	}

	// Update call log status
	m.db.Model(&models.CallLog{}).
		Where("id = ?", session.CallLogID).
		Update("status", models.CallStatusTransferring)

	// Update session state
	session.mu.Lock()
	session.TransferID = transfer.ID
	session.mu.Unlock()

	// Start timeout goroutine
	transferTimeout := orgSettings.TransferTimeoutSecs
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(transferTimeout)*time.Second)

	session.mu.Lock()
	session.TransferCancel = cancel
	session.mu.Unlock()

	go m.waitForTransferTimeout(ctx, session, transfer.ID)

	// Broadcast WebSocket event
	var teamIDStr string
	if teamID != nil {
		teamIDStr = teamID.String()
	}

	payload := map[string]any{
		"id":                  transfer.ID.String(),
		"call_log_id":        transfer.CallLogID.String(),
		"whatsapp_call_id":   transfer.WhatsAppCallID,
		"caller_phone":       transfer.CallerPhone,
		"contact_id":         transfer.ContactID.String(),
		"whatsapp_account":   transfer.WhatsAppAccount,
		"team_id":            teamIDStr,
		"initiating_agent_id": initiatingAgentID.String(),
		"transferred_at":     transfer.TransferredAt.Format(time.RFC3339),
	}

	if targetAgentID != nil {
		// Direct transfer: notify only the target agent
		m.wsHub.BroadcastToUser(session.OrganizationID, *targetAgentID, websocket.WSMessage{
			Type:    websocket.TypeCallTransferWaiting,
			Payload: payload,
		})
	} else {
		// Team transfer: broadcast to entire org
		m.broadcastEvent(session.OrganizationID, websocket.TypeCallTransferWaiting, payload)
	}

	m.log.Info("Agent-initiated call transfer started",
		"call_id", session.ID,
		"transfer_id", transfer.ID,
		"initiating_agent", initiatingAgentID,
		"team_id", teamIDStr,
	)

	return nil
}

// ConnectAgentToTransfer handles an agent accepting a transfer. It creates a WebRTC
// PeerConnection for the agent, performs SDP exchange, and starts the audio bridge.
func (m *Manager) ConnectAgentToTransfer(transferID, agentID uuid.UUID, sdpOffer string) (string, error) {
	// Find the session by transfer ID
	session := m.findSessionByTransferID(transferID)
	if session == nil {
		return "", fmt.Errorf("no active session for transfer %s", transferID)
	}

	session.mu.Lock()
	if session.TransferStatus != models.CallTransferStatusWaiting {
		session.mu.Unlock()
		return "", fmt.Errorf("transfer is not in waiting state: %s", session.TransferStatus)
	}
	// Claim the transfer atomically so a second agent gets rejected
	session.TransferStatus = models.CallTransferStatusConnected
	session.mu.Unlock()

	// Create PeerConnection for agent (reuses same codec config)
	agentPC, err := m.createPeerConnection()
	if err != nil {
		return "", fmt.Errorf("failed to create agent peer connection: %w", err)
	}

	// Create local audio track (server → agent: caller's voice will be forwarded here)
	agentAudioTrack, err := createOpusTrack(agentPC, "caller-audio")
	if err != nil {
		_ = agentPC.Close()
		return "", fmt.Errorf("failed to create agent audio track: %w", err)
	}

	// Channel to signal when agent's remote track (mic) is available
	agentTrackReady := make(chan *webrtc.TrackRemote, 1)

	agentPC.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if track.Codec().MimeType == webrtc.MimeTypeOpus {
			select {
			case agentTrackReady <- track:
			default:
			}
		}
	})

	// Handle agent connection state changes
	agentPC.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		m.log.Info("Agent peer connection state changed",
			"transfer_id", transferID,
			"state", state.String(),
		)
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateDisconnected {
			m.EndTransfer(transferID)
		}
	})

	// Set remote description (agent's offer)
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdpOffer,
	}
	if err := agentPC.SetRemoteDescription(offer); err != nil {
		_ = agentPC.Close()
		return "", fmt.Errorf("failed to set agent remote description: %w", err)
	}

	// Create answer
	answer, err := agentPC.CreateAnswer(nil)
	if err != nil {
		_ = agentPC.Close()
		return "", fmt.Errorf("failed to create agent SDP answer: %w", err)
	}

	if err := agentPC.SetLocalDescription(answer); err != nil {
		_ = agentPC.Close()
		return "", fmt.Errorf("failed to set agent local description: %w", err)
	}

	// Wait for ICE gathering (15s, consistent with other call flows)
	localDesc, err := waitForICEGathering(agentPC, 15*time.Second)
	if err != nil {
		_ = agentPC.Close()
		return "", fmt.Errorf("agent ICE gathering: %w", err)
	}

	// Store agent PC in session
	session.mu.Lock()
	session.AgentPC = agentPC
	session.AgentAudioTrack = agentAudioTrack
	session.mu.Unlock()

	// Wait for agent's audio track, then start bridge
	go m.completeTransferConnection(session, transferID, agentID, agentTrackReady)

	return localDesc.SDP, nil
}

// completeTransferConnection waits for the agent's audio track and starts the audio bridge.
func (m *Manager) completeTransferConnection(session *CallSession, transferID, agentID uuid.UUID, agentTrackReady chan *webrtc.TrackRemote) {
	// Wait for agent's mic track (up to 10 seconds)
	var agentRemoteTrack *webrtc.TrackRemote
	select {
	case track := <-agentTrackReady:
		agentRemoteTrack = track
	case <-time.After(10 * time.Second):
		m.log.Error("Timeout waiting for agent audio track", "transfer_id", transferID)
		m.EndTransfer(transferID)
		return
	}

	session.mu.Lock()
	session.AgentRemoteTrack = agentRemoteTrack
	session.mu.Unlock()

	// Stop hold music
	session.mu.Lock()
	if session.HoldPlayer != nil {
		session.HoldPlayer.Stop()
	}
	session.mu.Unlock()

	// Cancel transfer timeout
	session.mu.Lock()
	if session.TransferCancel != nil {
		session.TransferCancel()
	}
	session.mu.Unlock()

	// Signal that bridge is taking over the caller track
	session.mu.Lock()
	safeClose(session.BridgeStarted)
	session.mu.Unlock()

	// Update transfer status
	now := time.Now()
	m.db.Model(&models.CallTransfer{}).
		Where("id = ?", transferID).
		Updates(map[string]any{
			"status":       models.CallTransferStatusConnected,
			"agent_id":     agentID,
			"connected_at": now,
		})

	// Also set agent_id on the CallLog so the webhook "ended" handler
	// knows an agent was connected and doesn't mark the call as "missed".
	m.db.Model(&models.CallLog{}).
		Where("id = ?", session.CallLogID).
		Update("agent_id", agentID)

	session.mu.Lock()
	session.TransferStatus = models.CallTransferStatusConnected
	var callerRemote *webrtc.TrackRemote
	var callerLocal *webrtc.TrackLocalStaticRTP
	if session.Direction == models.CallDirectionOutgoing {
		callerRemote = session.WARemoteTrack
		callerLocal = session.WAAudioTrack
	} else {
		callerRemote = session.CallerRemoteTrack
		callerLocal = session.AudioTrack
	}
	agentLocal := session.AgentAudioTrack
	session.mu.Unlock()

	// Broadcast connected event
	m.broadcastEvent(session.OrganizationID, websocket.TypeCallTransferConnected, map[string]any{
		"id":           transferID.String(),
		"agent_id":     agentID.String(),
		"connected_at": now.Format(time.RFC3339),
	})

	m.log.Info("Call transfer connected",
		"transfer_id", transferID,
		"agent_id", agentID,
	)

	// Create recorder and start audio bridge (blocks until stopped)
	bridge := m.setupAudioBridge(session)
	bridge.Start(callerRemote, agentLocal, agentRemoteTrack, callerLocal)
}

// EndTransfer terminates an active transfer, cleans up resources, and updates the database.
func (m *Manager) EndTransfer(transferID uuid.UUID) {
	session := m.findSessionByTransferID(transferID)
	if session == nil {
		return
	}

	session.mu.Lock()
	if session.TransferStatus == models.CallTransferStatusCompleted {
		session.mu.Unlock()
		return
	}
	session.TransferStatus = models.CallTransferStatusCompleted

	// Snapshot and nil resources under lock so we can release before calling Stop/Close
	bridge := session.Bridge
	session.Bridge = nil
	holdPlayer := session.HoldPlayer
	session.HoldPlayer = nil
	transferCancel := session.TransferCancel
	session.TransferCancel = nil
	agentPC := session.AgentPC
	session.AgentPC = nil
	session.mu.Unlock()

	// Stop/close resources outside lock.
	// Save last RTP seq so the post-transfer IVR player can continue
	// from the correct sequence number. Use hold player first (always
	// present), then override with bridge if an agent was connected.
	if holdPlayer != nil {
		seq, ts := holdPlayer.Sequence()
		session.mu.Lock()
		session.LastRTPSeq = seq
		session.LastRTPTimestamp = ts
		session.mu.Unlock()
		holdPlayer.Stop()
	}
	if bridge != nil {
		bridge.Stop()
		bridge.Wait() // Wait for goroutines to finish so lastCallerSeq is final.
		seq, ts := bridge.LastCallerSeq()
		if seq > 0 {
			session.mu.Lock()
			session.LastRTPSeq = seq
			session.LastRTPTimestamp = ts
			session.mu.Unlock()
		}
	}
	if transferCancel != nil {
		transferCancel()
	}
	if agentPC != nil {
		_ = agentPC.Close()
	}

	// Calculate durations and update DB
	now := time.Now()
	var transfer models.CallTransfer
	if err := m.db.First(&transfer, transferID).Error; err != nil {
		m.log.Error("Failed to find transfer for completion", "error", err, "transfer_id", transferID)
		return
	}

	talkDuration := durationSince(transfer.ConnectedAt, now)
	holdDuration := 0
	if transfer.ConnectedAt != nil {
		holdDuration = int(transfer.ConnectedAt.Sub(transfer.TransferredAt).Seconds())
	} else {
		holdDuration = int(now.Sub(transfer.TransferredAt).Seconds())
	}

	m.db.Model(&transfer).Updates(map[string]any{
		"status":        models.CallTransferStatusCompleted,
		"completed_at":  now,
		"hold_duration": holdDuration,
		"talk_duration": talkDuration,
	})

	// Broadcast completed event
	m.broadcastEvent(session.OrganizationID, websocket.TypeCallTransferCompleted, map[string]any{
		"id":            transferID.String(),
		"hold_duration": holdDuration,
		"talk_duration": talkDuration,
		"completed_at":  now.Format(time.RFC3339),
	})

	m.log.Info("Call transfer completed",
		"transfer_id", transferID,
		"hold_duration", holdDuration,
		"talk_duration", talkDuration,
	)

	// If the IVR loop is waiting to resume after transfer, signal it
	// instead of tearing down the session.
	session.mu.Lock()
	transferDone := session.TransferDone
	session.TransferDone = nil
	session.mu.Unlock()

	if transferDone != nil {
		// Reset BridgeStarted and restart caller track consumption so
		// Pion's receive buffer doesn't fill up (same pattern as
		// InitiateAgentTransfer). Use consumeAudioWithDTMF so that
		// post-transfer IVR nodes (menu, gather) can receive DTMF.
		session.mu.Lock()
		session.BridgeStarted = make(chan struct{})
		callerRemote := session.CallerRemoteTrack
		session.mu.Unlock()
		if callerRemote != nil {
			go m.consumeAudioWithDTMF(session, callerRemote)
		}

		transferDone <- "completed"
	} else {
		// Terminal transfer — terminate the WhatsApp call and clean up.
		m.terminateCallBySession(session)
		m.cleanupSession(session.ID)
	}
}

// waitForTransferTimeout marks the transfer as no_answer if nobody accepts in time.
func (m *Manager) waitForTransferTimeout(ctx context.Context, session *CallSession, transferID uuid.UUID) {
	<-ctx.Done()

	// If the context was cancelled (not timed out), the transfer was accepted or ended
	if ctx.Err() != context.DeadlineExceeded {
		return
	}

	session.mu.Lock()
	if session.TransferStatus != models.CallTransferStatusWaiting {
		session.mu.Unlock()
		return
	}
	session.TransferStatus = models.CallTransferStatusNoAnswer
	session.mu.Unlock()

	now := time.Now()
	m.db.Model(&models.CallTransfer{}).
		Where("id = ?", transferID).
		Updates(map[string]any{
			"status":       models.CallTransferStatusNoAnswer,
			"completed_at": now,
		})

	// Mark call as disconnected by system (transfer timeout)
	m.db.Model(&models.CallLog{}).
		Where("id = ?", session.CallLogID).
		Update("disconnected_by", models.DisconnectedBySystem)

	// Stop hold music and save RTP seq for post-transfer IVR player
	session.mu.Lock()
	if session.HoldPlayer != nil {
		seq, ts := session.HoldPlayer.Sequence()
		session.LastRTPSeq = seq
		session.LastRTPTimestamp = ts
		session.HoldPlayer.Stop()
	}
	session.mu.Unlock()

	// Broadcast no_answer event
	m.broadcastEvent(session.OrganizationID, websocket.TypeCallTransferNoAnswer, map[string]any{
		"id":           transferID.String(),
		"completed_at": now.Format(time.RFC3339),
	})

	m.log.Info("Call transfer timed out", "transfer_id", transferID)

	// If the IVR loop is waiting to resume, signal it instead of cleaning up.
	session.mu.Lock()
	transferDone := session.TransferDone
	session.TransferDone = nil
	session.mu.Unlock()

	if transferDone != nil {
		// Restart caller track consumption with DTMF detection for
		// post-transfer IVR nodes.
		session.mu.Lock()
		session.BridgeStarted = make(chan struct{})
		callerRemote := session.CallerRemoteTrack
		session.mu.Unlock()
		if callerRemote != nil {
			go m.consumeAudioWithDTMF(session, callerRemote)
		}

		transferDone <- "no_answer"
	} else {
		m.cleanupSession(session.ID)
	}
}

// HandleCallerHangupDuringTransfer handles the case where the caller hangs up while waiting.
func (m *Manager) HandleCallerHangupDuringTransfer(session *CallSession) {
	session.mu.Lock()
	transferID := session.TransferID
	status := session.TransferStatus
	session.mu.Unlock()

	if transferID == uuid.Nil || status != models.CallTransferStatusWaiting {
		return
	}

	now := time.Now()
	m.db.Model(&models.CallTransfer{}).
		Where("id = ?", transferID).
		Updates(map[string]any{
			"status":       models.CallTransferStatusAbandoned,
			"completed_at": now,
		})

	// Mark call as disconnected by client (caller hung up during transfer)
	m.db.Model(&models.CallLog{}).
		Where("id = ?", session.CallLogID).
		Update("disconnected_by", models.DisconnectedByClient)

	// Stop hold music and cancel timeout
	session.mu.Lock()
	session.TransferStatus = models.CallTransferStatusAbandoned
	if session.HoldPlayer != nil {
		session.HoldPlayer.Stop()
	}
	if session.TransferCancel != nil {
		session.TransferCancel()
	}
	session.mu.Unlock()

	m.broadcastEvent(session.OrganizationID, websocket.TypeCallTransferAbandoned, map[string]any{
		"id":           transferID.String(),
		"completed_at": now.Format(time.RFC3339),
	})

	m.log.Info("Call transfer abandoned (caller hung up)", "transfer_id", transferID)

	// If the IVR loop is waiting to resume, signal it. The next node's audio
	// write will fail (caller disconnected), so the loop breaks naturally.
	session.mu.Lock()
	transferDone := session.TransferDone
	session.TransferDone = nil
	session.mu.Unlock()

	if transferDone != nil {
		transferDone <- "abandoned"
	} else {
		// Now that TransferStatus is no longer Waiting, cleanupSession will proceed.
		m.cleanupSession(session.ID)
	}
}

// findSessionByTransferID looks up a session by its transfer ID.
func (m *Manager) findSessionByTransferID(transferID uuid.UUID) *CallSession {
	m.mu.RLock()
	snapshot := make([]*CallSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		snapshot = append(snapshot, s)
	}
	m.mu.RUnlock()

	for _, s := range snapshot {
		s.mu.Lock()
		tid := s.TransferID
		s.mu.Unlock()
		if tid == transferID {
			return s
		}
	}
	return nil
}

