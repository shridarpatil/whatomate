package calling

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
)

// initiateTransfer starts the transfer flow: puts caller on hold, notifies agents via WebSocket.
func (m *Manager) initiateTransfer(session *CallSession, waAccount string, teamTarget string, ivrPath []map[string]string) {
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

	// Start hold music on the caller's audio track
	holdFile := filepath.Join(m.config.AudioDir, m.config.HoldMusicFile)
	player := NewAudioPlayer(session.AudioTrack)

	session.mu.Lock()
	session.HoldPlayer = player
	session.mu.Unlock()

	go func() {
		_ = player.PlayFileLoop(holdFile)
	}()

	// Start timeout goroutine (use org-level override if set)
	transferTimeout := m.getOrgTransferTimeout(session.OrganizationID)
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

	m.broadcastTransferEvent(transfer.OrganizationID, websocket.TypeCallTransferWaiting, map[string]any{
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
	agentAudioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"caller-audio",
	)
	if err != nil {
		_ = agentPC.Close()
		return "", fmt.Errorf("failed to create agent audio track: %w", err)
	}

	if _, err := agentPC.AddTrack(agentAudioTrack); err != nil {
		_ = agentPC.Close()
		return "", fmt.Errorf("failed to add agent audio track: %w", err)
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

	// Wait for ICE gathering
	gatherComplete := webrtc.GatheringCompletePromise(agentPC)
	select {
	case <-gatherComplete:
	case <-time.After(5 * time.Second):
		_ = agentPC.Close()
		return "", fmt.Errorf("ICE gathering timed out for agent")
	}

	localDesc := agentPC.LocalDescription()
	if localDesc == nil {
		_ = agentPC.Close()
		return "", fmt.Errorf("no local description available for agent")
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
	close(session.BridgeStarted)
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
	callerRemote := session.CallerRemoteTrack
	callerLocal := session.AudioTrack
	agentLocal := session.AgentAudioTrack
	session.mu.Unlock()

	// Broadcast connected event
	m.broadcastTransferEvent(session.OrganizationID, websocket.TypeCallTransferConnected, map[string]any{
		"id":           transferID.String(),
		"agent_id":     agentID.String(),
		"connected_at": now.Format(time.RFC3339),
	})

	m.log.Info("Call transfer connected",
		"transfer_id", transferID,
		"agent_id", agentID,
	)

	// Create recorder and start audio bridge (blocks until stopped)
	recorder := m.newRecorderIfEnabled()
	bridge := NewAudioBridge(recorder)
	session.mu.Lock()
	session.Bridge = bridge
	session.Recorder = recorder
	session.mu.Unlock()

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

	// Stop bridge
	if session.Bridge != nil {
		session.Bridge.Stop()
	}

	// Stop hold music
	if session.HoldPlayer != nil {
		session.HoldPlayer.Stop()
	}

	// Cancel timeout
	if session.TransferCancel != nil {
		session.TransferCancel()
	}

	// Close agent PC
	if session.AgentPC != nil {
		_ = session.AgentPC.Close()
	}

	session.mu.Unlock()

	// Calculate durations and update DB
	now := time.Now()
	var transfer models.CallTransfer
	if err := m.db.First(&transfer, transferID).Error; err != nil {
		m.log.Error("Failed to find transfer for completion", "error", err, "transfer_id", transferID)
		return
	}

	holdDuration := 0
	talkDuration := 0
	if transfer.ConnectedAt != nil {
		holdDuration = int(transfer.ConnectedAt.Sub(transfer.TransferredAt).Seconds())
		talkDuration = int(now.Sub(*transfer.ConnectedAt).Seconds())
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
	m.broadcastTransferEvent(session.OrganizationID, websocket.TypeCallTransferCompleted, map[string]any{
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

	// Terminate the WhatsApp call so the caller's phone also disconnects
	var account models.WhatsAppAccount
	if err := m.db.Where("organization_id = ? AND name = ?", session.OrganizationID, session.AccountName).
		First(&account).Error; err == nil {
		waAccount := &whatsapp.Account{
			PhoneID:     account.PhoneID,
			BusinessID:  account.BusinessID,
			APIVersion:  account.APIVersion,
			AccessToken: account.AccessToken,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := m.whatsapp.TerminateCall(ctx, waAccount, session.ID); err != nil {
			m.log.Error("Failed to terminate WhatsApp call after transfer end", "error", err, "call_id", session.ID)
		}
	}

	// Clean up the whole call session
	m.cleanupSession(session.ID)
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

	// Stop hold music
	session.mu.Lock()
	if session.HoldPlayer != nil {
		session.HoldPlayer.Stop()
	}
	session.mu.Unlock()

	// Broadcast no_answer event
	m.broadcastTransferEvent(session.OrganizationID, websocket.TypeCallTransferNoAnswer, map[string]any{
		"id":           transferID.String(),
		"completed_at": now.Format(time.RFC3339),
	})

	m.log.Info("Call transfer timed out", "transfer_id", transferID)

	// Clean up the session (terminates WhatsApp call via cleanupSession)
	m.cleanupSession(session.ID)
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

	m.broadcastTransferEvent(session.OrganizationID, websocket.TypeCallTransferAbandoned, map[string]any{
		"id":           transferID.String(),
		"completed_at": now.Format(time.RFC3339),
	})

	m.log.Info("Call transfer abandoned (caller hung up)", "transfer_id", transferID)
}

// findSessionByTransferID looks up a session by its transfer ID.
func (m *Manager) findSessionByTransferID(transferID uuid.UUID) *CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sessions {
		s.mu.Lock()
		tid := s.TransferID
		s.mu.Unlock()
		if tid == transferID {
			return s
		}
	}
	return nil
}

// broadcastTransferEvent sends a transfer event via WebSocket.
func (m *Manager) broadcastTransferEvent(orgID uuid.UUID, eventType string, payload map[string]any) {
	if m.wsHub == nil {
		return
	}
	m.wsHub.BroadcastToOrg(orgID, websocket.WSMessage{
		Type:    eventType,
		Payload: payload,
	})
}
