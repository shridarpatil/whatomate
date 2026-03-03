package calling

import (
	"context"
	"fmt"
	"os"
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
	CurrentMenu     *IVRMenuNode
	IVRFlow         *models.IVRFlow
	IVRPlayer       *AudioPlayer // persists across goto_flow for RTP continuity
	DTMFBuffer      chan byte
	StartedAt       time.Time

	// Recording
	Recorder *CallRecorder

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

// IVRMenuNode represents a node in the IVR menu tree (parsed from JSONB)
type IVRMenuNode struct {
	Greeting            string                 `json:"greeting"`
	GreetingText        string                 `json:"greeting_text,omitempty"`
	Options             map[string]IVROption   `json:"options"`
	TimeoutSeconds      int                    `json:"timeout_seconds"`
	MaxRetries          int                    `json:"max_retries"`
	InvalidInputMessage string                 `json:"invalid_input_message"`
	Parent              *IVRMenuNode           `json:"-"`
}

// IVROption represents a single option in an IVR menu
type IVROption struct {
	Label  string       `json:"label"`
	Action string       `json:"action"` // transfer, submenu, repeat, parent, hangup, goto_flow
	Target string       `json:"target,omitempty"`
	Menu   *IVRMenuNode `json:"menu,omitempty"`
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
	if exists {
		delete(m.sessions, callID)
	}
	m.mu.Unlock()

	if !exists {
		return
	}

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
	recorder := session.Recorder
	session.Recorder = nil

	session.mu.Unlock()

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
		m.broadcastTransferEvent(orgID, websocket.TypeCallTransferAbandoned, map[string]any{
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
	if recorder != nil {
		go m.finalizeRecording(orgID, callLogID, recorder)
	}

	m.log.Info("Call session cleaned up", "call_id", callID)
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

// finalizeRecording stops the recorder, uploads the OGG file to S3, and updates the CallLog.
func (m *Manager) finalizeRecording(orgID, callLogID uuid.UUID, recorder *CallRecorder) {
	path, packetCount := recorder.Stop()
	defer func() { _ = os.Remove(path) }()

	if packetCount == 0 {
		return
	}

	// Calculate duration: each packet is 20ms, but both directions interleave,
	// so actual call duration ≈ packetCount * 20ms / 2 (two directions).
	durationSecs := (packetCount * 20) / 2 / 1000

	s3Key := fmt.Sprintf("recordings/%s/%s.ogg", orgID.String(), callLogID.String())

	f, err := os.Open(path)
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
		"packets", packetCount,
		"duration_secs", durationSecs,
	)
}
