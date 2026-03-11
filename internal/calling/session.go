package calling

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/storage"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/zerodha/logf"
	"gorm.io/gorm"
)

// CallSession represents an active call with its WebRTC state
type CallSession struct {
	ID              string // WhatsApp call_id
	OrganizationID  uuid.UUID
	AccountName     string
	CallerPhone     string
	ContactID       uuid.UUID
	CallLogID       uuid.UUID
	Status          models.CallStatus
	PeerConnection  *webrtc.PeerConnection
	AudioTrack      *webrtc.TrackLocalStaticRTP
	IVRGraph        *IVRFlowGraph
	IVRCtx          *IVRContext
	IVRFlow         *models.IVRFlow
	IVRPlayer       *AudioPlayer // persists across goto_flow for RTP continuity
	DTMFBuffer      chan byte
	StartedAt       time.Time

	// Recording (one per direction for correct OGG/Opus playback)
	CallerRecorder *CallRecorder // caller's audio stream
	AgentRecorder  *CallRecorder // agent's audio stream

	// Transfer fields
	TransferID        uuid.UUID
	TransferStatus    models.CallTransferStatus
	AgentPC           *webrtc.PeerConnection
	AgentAudioTrack   *webrtc.TrackLocalStaticRTP
	CallerRemoteTrack *webrtc.TrackRemote
	AgentRemoteTrack  *webrtc.TrackRemote
	Bridge            *AudioBridge
	HoldPlayer        *AudioPlayer
	TransferCancel    context.CancelFunc
	BridgeStarted     chan struct{} // closed when bridge takes over caller track
	TransferDone      chan string   // outcome sent when transfer ends; nil = terminal
	LastRTPSeq        uint16       // last RTP seq from bridge, for post-transfer player
	LastRTPTimestamp   uint32       // last RTP timestamp from bridge

	// Ringback (outgoing calls)
	RingbackPlayer *AudioPlayer

	// Outgoing call fields
	Direction      models.CallDirection
	AgentID        uuid.UUID
	TargetPhone    string
	WAPeerConn     *webrtc.PeerConnection           // WhatsApp-side PC (outgoing only)
	WAAudioTrack   *webrtc.TrackLocalStaticRTP       // server→WhatsApp audio track
	WARemoteTrack  *webrtc.TrackRemote               // WhatsApp's remote audio track
	SDPAnswerReady chan string                        // webhook delivers SDP answer here

	mu sync.Mutex
}

// IVRNodeType identifies the kind of applet in an IVR flow graph.
type IVRNodeType string

const (
	IVRNodeGreeting     IVRNodeType = "greeting"
	IVRNodeMenu         IVRNodeType = "menu"
	IVRNodeGather       IVRNodeType = "gather"
	IVRNodeHTTPCallback IVRNodeType = "http_callback"
	IVRNodeTransfer     IVRNodeType = "transfer"
	IVRNodeGotoFlow     IVRNodeType = "goto_flow"
	IVRNodeTiming       IVRNodeType = "timing"
	IVRNodeHangup       IVRNodeType = "hangup"
)

// IVRNodePosition stores the (x,y) position for the visual editor.
type IVRNodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// IVRNode represents a single node (applet) in an IVR flow graph.
type IVRNode struct {
	ID       string                 `json:"id"`
	Type     IVRNodeType            `json:"type"`
	Label    string                 `json:"label"`
	Position IVRNodePosition        `json:"position"`
	Config   map[string]interface{} `json:"config"`
}

// IVREdge connects two nodes in the flow graph.
type IVREdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition"` // default, digit:N, timeout, max_retries, http:2xx, http:non2xx, in_hours, out_of_hours
}

// IVRFlowGraph is the top-level structure stored in IVRFlow.Menu (version 2).
type IVRFlowGraph struct {
	Version   int       `json:"version"`
	Nodes     []IVRNode `json:"nodes"`
	Edges     []IVREdge `json:"edges"`
	EntryNode string    `json:"entry_node"`

	// Runtime lookup maps — populated by buildMaps()
	nodeMap map[string]*IVRNode  // id → node
	edgeMap map[string][]IVREdge // from-node-id → outgoing edges
}

// buildMaps populates the runtime lookup maps for fast traversal.
func (g *IVRFlowGraph) buildMaps() {
	g.nodeMap = make(map[string]*IVRNode, len(g.Nodes))
	g.edgeMap = make(map[string][]IVREdge, len(g.Edges))
	for i := range g.Nodes {
		g.nodeMap[g.Nodes[i].ID] = &g.Nodes[i]
	}
	for _, e := range g.Edges {
		g.edgeMap[e.From] = append(g.edgeMap[e.From], e)
	}
}

// getNode returns the node with the given ID, or nil.
func (g *IVRFlowGraph) getNode(id string) *IVRNode {
	return g.nodeMap[id]
}

// resolveEdge finds the next node ID for a given outcome.
// It tries an exact condition match first, then falls back to "default".
func (g *IVRFlowGraph) resolveEdge(fromID, outcome string) string {
	edges := g.edgeMap[fromID]
	var defaultTarget string
	for _, e := range edges {
		if e.Condition == outcome {
			return e.To
		}
		if e.Condition == "default" {
			defaultTarget = e.To
		}
	}
	return defaultTarget
}

// IVRContext holds runtime state during IVR flow execution.
type IVRContext struct {
	Variables   map[string]string
	CallerPhone string
	CallID      string
	CurrentNode string
	Path        []map[string]string
}

// Manager manages active call sessions
type Manager struct {
	sessions map[string]*CallSession
	mu       sync.RWMutex
	log      logf.Logger
	whatsapp *whatsapp.Client
	db       *gorm.DB
	wsHub    *websocket.Hub
	config   *config.CallingConfig
	s3       *storage.S3Client // nil when recording is disabled
}

// NewManager creates a new call session manager
func NewManager(cfg *config.CallingConfig, s3Client *storage.S3Client, db *gorm.DB, waClient *whatsapp.Client, wsHub *websocket.Hub, log logf.Logger) *Manager {
	// Apply defaults for server-level config
	if cfg.AudioDir == "" {
		cfg.AudioDir = "./audio"
	}
	if cfg.HoldMusicFile == "" {
		cfg.HoldMusicFile = "hold.ogg"
	}
	if cfg.MaxCallDuration <= 0 {
		cfg.MaxCallDuration = 3600
	}
	if cfg.TransferTimeoutSecs <= 0 {
		cfg.TransferTimeoutSecs = 60
	}

	return &Manager{
		sessions: make(map[string]*CallSession),
		log:      log,
		whatsapp: waClient,
		db:       db,
		wsHub:    wsHub,
		config:   cfg,
		s3:       s3Client,
	}
}

// HandleIncomingCall processes a new incoming call and starts WebRTC negotiation.
// The sdpOffer parameter is the consumer's SDP offer received from the webhook's
// session.sdp field in the "connect" event.
func (m *Manager) HandleIncomingCall(account *models.WhatsAppAccount, contact *models.Contact, callLog *models.CallLog, sdpOffer string) {
	session := &CallSession{
		ID:             callLog.WhatsAppCallID,
		OrganizationID: account.OrganizationID,
		AccountName:    account.Name,
		CallerPhone:    contact.PhoneNumber,
		ContactID:      contact.ID,
		CallLogID:      callLog.ID,
		Status:         models.CallStatusRinging,
		DTMFBuffer:     make(chan byte, 32),
		StartedAt:      time.Now(),
		BridgeStarted:  make(chan struct{}),
	}

	// Load IVR flow if assigned
	if callLog.IVRFlowID != nil {
		var flow models.IVRFlow
		if err := m.db.First(&flow, callLog.IVRFlowID).Error; err == nil {
			session.IVRFlow = &flow
		}
	}

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	m.log.Info("Call session created",
		"call_id", session.ID,
		"caller", session.CallerPhone,
		"has_sdp_offer", sdpOffer != "",
	)

	// Start WebRTC negotiation using the consumer's SDP offer
	go m.negotiateWebRTC(session, account, sdpOffer)
}

// HandleCallEvent processes a call lifecycle event (in_call, ended, etc.)
func (m *Manager) HandleCallEvent(callID, event string) {
	m.mu.RLock()
	session, exists := m.sessions[callID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	session.mu.Lock()
	var action string
	var transferID uuid.UUID

	switch event {
	case "in_call", "connect":
		session.Status = models.CallStatusAnswered
	case "ended", "terminate", "missed", "unanswered":
		switch session.TransferStatus {
		case models.CallTransferStatusWaiting:
			action = "hangup_transfer"
		case models.CallTransferStatusConnected:
			action = "end_transfer"
			transferID = session.TransferID
		default:
			session.Status = models.CallStatusCompleted
			action = "cleanup"
		}
	}
	session.mu.Unlock()

	switch action {
	case "hangup_transfer":
		m.HandleCallerHangupDuringTransfer(session)
	case "end_transfer":
		m.EndTransfer(transferID)
	case "cleanup":
		go m.cleanupSession(callID)
	}
}

// EndCall terminates a call session and cleans up resources
func (m *Manager) EndCall(callID string) {
	m.cleanupSession(callID)
}

// GetSession returns a call session by ID
func (m *Manager) GetSession(callID string) *CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[callID]
}

// GetSessionByCallLogID returns a call session by its CallLog ID
func (m *Manager) GetSessionByCallLogID(callLogID uuid.UUID) *CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sessions {
		if s.CallLogID == callLogID {
			return s
		}
	}
	return nil
}

// orgCallingSettings holds per-org calling overrides resolved from a single DB query.
type orgCallingSettings struct {
	TransferTimeoutSecs int
	HoldMusicFile       string
	RingbackFile        string
}

// getOrgCallingSettings loads org-level calling overrides with a single DB query,
// falling back to global config defaults for any missing values.
func (m *Manager) getOrgCallingSettings(orgID uuid.UUID) orgCallingSettings {
	s := orgCallingSettings{
		TransferTimeoutSecs: m.config.TransferTimeoutSecs,
		HoldMusicFile:       filepath.Join(m.config.AudioDir, m.config.HoldMusicFile),
	}
	if m.config.RingbackFile != "" {
		s.RingbackFile = filepath.Join(m.config.AudioDir, m.config.RingbackFile)
	}

	var org models.Organization
	if err := m.db.Where("id = ?", orgID).First(&org).Error; err != nil || org.Settings == nil {
		return s
	}

	if v, ok := org.Settings["transfer_timeout_secs"].(float64); ok && v > 0 {
		s.TransferTimeoutSecs = int(v)
	}
	if v, ok := org.Settings["hold_music_file"].(string); ok && v != "" {
		s.HoldMusicFile = filepath.Join(m.config.AudioDir, v)
	}
	if v, ok := org.Settings["ringback_file"].(string); ok && v != "" {
		s.RingbackFile = filepath.Join(m.config.AudioDir, v)
	}

	return s
}

// getOrgRingback returns the ringback file path for a session's organization.
func (m *Manager) getOrgRingback(orgID uuid.UUID) string {
	return m.getOrgCallingSettings(orgID).RingbackFile
}

// cleanupSession removes a session and releases WebRTC resources
func (m *Manager) cleanupSession(callID string) {
	m.mu.Lock()
	session, exists := m.sessions[callID]
	if !exists {
		m.mu.Unlock()
		return
	}

	// If a transfer is in the "waiting" state the agent's PC is being torn
	// down intentionally. Don't destroy the whole session — the caller-side
	// (or WA-side) PeerConnection must stay alive for hold music.
	session.mu.Lock()
	if session.TransferStatus == models.CallTransferStatusWaiting {
		session.mu.Unlock()
		m.mu.Unlock()
		m.log.Info("Skipping cleanup — transfer in waiting state", "call_id", callID)
		return
	}
	session.mu.Unlock()

	delete(m.sessions, callID)
	m.mu.Unlock()

	// Snapshot state and resources under lock, then release before calling external methods
	session.mu.Lock()

	// Transfer state snapshot for DB updates
	transferID := session.TransferID
	transferStatus := session.TransferStatus
	callLogID := session.CallLogID
	orgID := session.OrganizationID

	if transferID != uuid.Nil && transferStatus == models.CallTransferStatusWaiting {
		session.TransferStatus = models.CallTransferStatusAbandoned
	}

	// Snapshot and nil resources to prevent double-close
	bridge := session.Bridge
	session.Bridge = nil
	holdPlayer := session.HoldPlayer
	session.HoldPlayer = nil
	ringbackPlayer := session.RingbackPlayer
	session.RingbackPlayer = nil
	ivrPlayer := session.IVRPlayer
	session.IVRPlayer = nil
	transferCancel := session.TransferCancel
	session.TransferCancel = nil
	agentPC := session.AgentPC
	session.AgentPC = nil
	waPeerConn := session.WAPeerConn
	session.WAPeerConn = nil
	peerConn := session.PeerConnection
	session.PeerConnection = nil
	dtmfBuffer := session.DTMFBuffer
	session.DTMFBuffer = nil
	callerRec := session.CallerRecorder
	session.CallerRecorder = nil
	agentRec := session.AgentRecorder
	session.AgentRecorder = nil
	transferDone := session.TransferDone
	session.TransferDone = nil

	session.mu.Unlock()

	// Close TransferDone to unblock any waiting IVR goroutine
	if transferDone != nil {
		close(transferDone)
	}

	// DB operations and broadcasts (outside lock)
	if transferID != uuid.Nil && transferStatus == models.CallTransferStatusWaiting {
		now := time.Now()
		m.db.Model(&models.CallTransfer{}).
			Where("id = ? AND status = ?", transferID, models.CallTransferStatusWaiting).
			Updates(map[string]any{
				"status":       models.CallTransferStatusAbandoned,
				"completed_at": now,
			})
		m.db.Model(&models.CallLog{}).
			Where("id = ?", callLogID).
			Update("disconnected_by", models.DisconnectedByClient)
		m.broadcastEvent(orgID, websocket.TypeCallTransferAbandoned, map[string]any{
			"id":           transferID.String(),
			"completed_at": now.Format(time.RFC3339),
		})
		m.log.Info("Transfer marked abandoned during cleanup", "transfer_id", transferID, "call_id", callID)
	}

	// Stop resources (outside lock)
	if bridge != nil {
		bridge.Stop()
	}
	if holdPlayer != nil {
		holdPlayer.Stop()
	}
	if ringbackPlayer != nil {
		ringbackPlayer.Stop()
	}
	if ivrPlayer != nil {
		ivrPlayer.Stop()
	}
	if transferCancel != nil {
		transferCancel()
	}
	if agentPC != nil {
		if err := agentPC.Close(); err != nil {
			m.log.Error("Failed to close agent peer connection", "error", err, "call_id", callID)
		}
	}

	// Close WhatsApp peer connection (outgoing calls)
	if waPeerConn != nil {
		if err := waPeerConn.Close(); err != nil {
			m.log.Error("Failed to close WA peer connection", "error", err, "call_id", callID)
		}
	}

	// Close caller peer connection
	if peerConn != nil {
		if err := peerConn.Close(); err != nil {
			m.log.Error("Failed to close peer connection", "error", err, "call_id", callID)
		}
	}

	// Close DTMF buffer channel
	if dtmfBuffer != nil {
		close(dtmfBuffer)
	}

	// Finalize recording (async — don't block cleanup)
	if callerRec != nil || agentRec != nil {
		go m.finalizeRecording(orgID, callLogID, callerRec, agentRec)
	}

	m.log.Info("Call session cleaned up", "call_id", callID)
}

// --- Shared helpers to reduce duplication across calling files ---

// broadcastEvent broadcasts a call event via WebSocket to an organization.
func (m *Manager) broadcastEvent(orgID uuid.UUID, eventType string, payload map[string]any) {
	if m.wsHub == nil {
		return
	}
	m.wsHub.BroadcastToOrg(orgID, websocket.WSMessage{
		Type:    eventType,
		Payload: payload,
	})
}

// setupAudioBridge creates per-direction recorders (if enabled), builds an
// AudioBridge, and assigns everything to the session under its lock.
// If recorders already exist on the session (e.g. after a transfer), they are
// reused so the entire call is captured in continuous files.
func (m *Manager) setupAudioBridge(session *CallSession) *AudioBridge {
	session.mu.Lock()
	callerRec := session.CallerRecorder
	agentRec := session.AgentRecorder
	session.mu.Unlock()

	if callerRec == nil {
		callerRec = m.newRecorderIfEnabled()
	}
	if agentRec == nil {
		agentRec = m.newRecorderIfEnabled()
	}

	bridge := NewAudioBridge(callerRec, agentRec)
	session.mu.Lock()
	session.Bridge = bridge
	session.CallerRecorder = callerRec
	session.AgentRecorder = agentRec
	session.mu.Unlock()
	return bridge
}

// safeClose closes a channel only if it hasn't already been closed.
func safeClose(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}

// terminateCall terminates an active call via the WhatsApp API.
func (m *Manager) terminateCall(session *CallSession, waAccount *whatsapp.Account) {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := m.whatsapp.TerminateCall(c, waAccount, session.ID); err != nil {
		m.log.Error("Failed to terminate call via API", "error", err, "call_id", session.ID)
	}
}

// terminateCallBySession looks up the WhatsApp account from the DB and
// terminates the call. Used when only the session is available.
func (m *Manager) terminateCallBySession(session *CallSession) {
	var account models.WhatsAppAccount
	if err := m.db.Where("organization_id = ? AND name = ?", session.OrganizationID, session.AccountName).
		First(&account).Error; err != nil {
		m.log.Error("Failed to look up account for call termination", "error", err, "call_id", session.ID)
		return
	}
	waAccount := account.ToWAAccount()
	if waAccount.AccessToken != "" {
		m.terminateCall(session, waAccount)
	}
}

// durationSince calculates seconds elapsed since a given time, returning 0 if
// the pointer is nil.
func durationSince(from *time.Time, now time.Time) int {
	if from == nil {
		return 0
	}
	return int(now.Sub(*from).Seconds())
}

// newRecorderIfEnabled creates a CallRecorder if recording is enabled, or returns nil.
func (m *Manager) newRecorderIfEnabled() *CallRecorder {
	if !m.config.RecordingEnabled || m.s3 == nil {
		return nil
	}
	rec, err := NewCallRecorder()
	if err != nil {
		m.log.Error("Failed to create call recorder", "error", err)
		return nil
	}
	return rec
}

// finalizeRecording stops both per-direction recorders, merges them into a
// single OGG/Opus file using FFmpeg, uploads to S3, and updates the CallLog.
func (m *Manager) finalizeRecording(orgID, callLogID uuid.UUID, callerRec, agentRec *CallRecorder) {
	var callerPath, agentPath string
	var callerCount, agentCount int

	if callerRec != nil {
		callerPath, callerCount = callerRec.Stop()
		defer func() { _ = os.Remove(callerPath) }()
	}
	if agentRec != nil {
		agentPath, agentCount = agentRec.Stop()
		defer func() { _ = os.Remove(agentPath) }()
	}

	maxCount := callerCount
	if agentCount > maxCount {
		maxCount = agentCount
	}
	if maxCount == 0 {
		return
	}

	// Duration from the longer stream (each packet = 20ms)
	durationSecs := (maxCount * 20) / 1000

	// Merge the two direction files into one using FFmpeg.
	// If only one direction was recorded, use it directly.
	var uploadPath string
	switch {
	case callerCount > 0 && agentCount > 0:
		merged, err := mergeRecordings(callerPath, agentPath)
		if err != nil {
			m.log.Error("Failed to merge recordings, uploading caller only",
				"error", err, "call_log_id", callLogID)
			uploadPath = callerPath
		} else {
			defer func() { _ = os.Remove(merged) }()
			uploadPath = merged
		}
	case callerCount > 0:
		uploadPath = callerPath
	default:
		uploadPath = agentPath
	}

	s3Key := fmt.Sprintf("recordings/%s/%s.ogg", orgID.String(), callLogID.String())

	f, err := os.Open(uploadPath)
	if err != nil {
		m.log.Error("Failed to open recording file", "error", err, "call_log_id", callLogID)
		return
	}
	defer f.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.s3.Upload(ctx, s3Key, f, "audio/ogg"); err != nil {
		m.log.Error("Failed to upload recording to S3", "error", err, "call_log_id", callLogID)
		return
	}

	m.db.Model(&models.CallLog{}).
		Where("id = ?", callLogID).
		Updates(map[string]any{
			"recording_s3_key":    s3Key,
			"recording_duration": durationSecs,
		})

	m.log.Info("Recording uploaded",
		"call_log_id", callLogID,
		"s3_key", s3Key,
		"caller_packets", callerCount,
		"agent_packets", agentCount,
		"duration_secs", durationSecs,
	)
}

// mergeRecordings uses FFmpeg to mix two mono OGG/Opus files into one.
func mergeRecordings(file1, file2 string) (string, error) {
	out, err := os.CreateTemp("", "call-merged-*.ogg")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	outPath := out.Name()
	_ = out.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", file1,
		"-i", file2,
		"-filter_complex", "amix=inputs=2:duration=longest",
		"-c:a", "libopus",
		"-y", outPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("ffmpeg: %w: %s", err, output)
	}

	return outPath, nil
}
