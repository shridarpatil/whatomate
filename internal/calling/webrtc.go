package calling

import (
	"context"
	"fmt"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
)

// negotiateWebRTC handles the SDP exchange and sets up WebRTC media.
//
// Per the WhatsApp Business Calling API (user-initiated calls):
//  1. Webhook "connect" delivers the consumer's SDP offer (in session.sdp)
//  2. Business creates a PeerConnection and sets the offer as remote description
//  3. Business creates an SDP answer
//  4. Business sends pre_accept with session: { sdp_type: "answer", sdp: "<SDP>" }
//  5. Business sends accept with the same session object
//  6. WebRTC media flows
func (m *Manager) negotiateWebRTC(session *CallSession, account *models.WhatsAppAccount, sdpOffer string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	// Create peer connection with Opus codec
	pc, err := m.createPeerConnection()
	if err != nil {
		m.log.Error("Failed to create peer connection", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	session.mu.Lock()
	session.PeerConnection = pc
	session.mu.Unlock()

	// Add local audio track for IVR playback / server→caller audio
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"ivr-audio",
	)
	if err != nil {
		m.log.Error("Failed to create audio track", "error", err)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	_, err = pc.AddTrack(audioTrack)
	if err != nil {
		m.log.Error("Failed to add audio track", "error", err)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	session.mu.Lock()
	session.AudioTrack = audioTrack
	session.mu.Unlock()

	// Register handler for incoming audio (caller's voice + DTMF)
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		m.log.Info("Received remote track",
			"call_id", session.ID,
			"codec", track.Codec().MimeType,
			"payload_type", track.PayloadType(),
		)

		// Check if this is a dedicated telephone-event track (DTMF)
		if track.Codec().MimeType == "audio/telephone-event" {
			go m.handleDTMFTrack(session, track)
			return
		}

		// Store the caller's remote track for potential audio bridge use
		session.mu.Lock()
		session.CallerRemoteTrack = track
		session.mu.Unlock()

		// Consume audio and detect inline DTMF (telephone-event packets
		// arrive on the same m-line as audio with a different payload type).
		go m.consumeAudioWithDTMF(session, track)
	})

	// Channel to signal when the WebRTC connection is established
	connected := make(chan struct{})

	// Handle connection state changes
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		m.log.Info("Peer connection state changed",
			"call_id", session.ID,
			"state", state.String(),
		)
		switch state {
		case webrtc.PeerConnectionStateConnected:
			select {
			case <-connected:
			default:
				close(connected)
			}
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateDisconnected:
			m.EndCall(session.ID)
		}
	})

	// Step 1: Set the consumer's SDP offer as remote description
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdpOffer,
	}); err != nil {
		m.log.Error("Failed to set remote description (consumer offer)", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	// Step 2: Create SDP answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		m.log.Error("Failed to create SDP answer", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		m.log.Error("Failed to set local description (answer)", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	// Wait for ICE gathering to complete
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gatherComplete:
		// ICE gathering complete
	case <-ctx.Done():
		m.log.Error("ICE gathering timed out", "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	localDesc := pc.LocalDescription()
	if localDesc == nil {
		m.log.Error("No local description available", "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	sdpAnswer := localDesc.SDP

	// Step 3: Pre-accept with our SDP answer
	if err := m.whatsapp.PreAcceptCall(ctx, waAccount, session.ID, sdpAnswer); err != nil {
		m.log.Error("Failed to pre-accept call", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	// Step 4: Accept with the same SDP answer
	if err := m.whatsapp.AcceptCall(ctx, waAccount, session.ID, sdpAnswer); err != nil {
		m.log.Error("Failed to accept call via API", "error", err, "call_id", session.ID)
		return
	}

	session.mu.Lock()
	session.Status = models.CallStatusAnswered
	session.mu.Unlock()

	m.log.Info("Call accepted with WebRTC, waiting for media connection", "call_id", session.ID)

	// Wait for the WebRTC media connection to be established before starting IVR.
	// ICE connectivity checks run after the SDP exchange; we must wait for them
	// to complete before audio can flow.
	select {
	case <-connected:
		m.log.Info("WebRTC media connected", "call_id", session.ID)
	case <-time.After(15 * time.Second):
		m.log.Error("WebRTC media connection timed out", "call_id", session.ID)
		m.terminateCall(session, waAccount)
		return
	}

	// Brief delay to let the media path stabilize before sending audio
	time.Sleep(500 * time.Millisecond)

	// Start IVR flow if configured
	if session.IVRFlow != nil {
		go m.runIVRFlow(session, waAccount)
	}
}

// createPeerConnection creates a new WebRTC peer connection with Opus codec support
func (m *Manager) createPeerConnection() (*webrtc.PeerConnection, error) {
	iceServers := make([]webrtc.ICEServer, 0, len(m.config.ICEServers))
	for _, s := range m.config.ICEServers {
		ice := webrtc.ICEServer{URLs: s.URLs}
		if s.Username != "" {
			ice.Username = s.Username
			ice.Credential = s.Credential
			ice.CredentialType = webrtc.ICECredentialTypePassword
		}
		iceServers = append(iceServers, ice)
	}

	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	// Force all media through TURN relay when direct UDP is not available.
	if m.config.RelayOnly {
		config.ICETransportPolicy = webrtc.ICETransportPolicyRelay
	}

	mediaEngine := &webrtc.MediaEngine{}

	// Register Opus codec
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to register Opus codec: %w", err)
	}

	// Register telephone-event codec for DTMF (RFC 4733)
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  "audio/telephone-event",
			ClockRate: 8000,
		},
		PayloadType: 101,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to register telephone-event codec: %w", err)
	}

	// Configure UDP port range and build API
	settingEngine := webrtc.SettingEngine{}
	portMin := m.config.UDPPortMin
	portMax := m.config.UDPPortMax
	if portMin == 0 {
		portMin = 10000
	}
	if portMax == 0 {
		portMax = 10100
	}
	if err := settingEngine.SetEphemeralUDPPortRange(portMin, portMax); err != nil {
		return nil, fmt.Errorf("failed to set UDP port range: %w", err)
	}

	// On cloud/AWS, map private IP to public IP so ICE candidates
	// advertise the reachable address instead of the internal one.
	if m.config.PublicIP != "" {
		if err := settingEngine.SetICEAddressRewriteRules(webrtc.ICEAddressRewriteRule{
			External:        []string{m.config.PublicIP},
			AsCandidateType: webrtc.ICECandidateTypeHost,
		}); err != nil {
			return nil, fmt.Errorf("failed to set ICE address rewrite rules: %w", err)
		}
	}

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)
	return api.NewPeerConnection(config)
}

// consumeAudioTrack reads and discards RTP packets to keep the stream active.
// It exits when the bridge takes over (BridgeStarted channel is closed) or on error.
func (m *Manager) consumeAudioTrack(session *CallSession, track *webrtc.TrackRemote) {
	buf := make([]byte, 1500)
	for {
		select {
		case <-session.BridgeStarted:
			// Bridge is taking over reading from this track
			return
		default:
		}

		_, _, err := track.Read(buf)
		if err != nil {
			return
		}
	}
}

// consumeAudioWithDTMF reads RTP packets from the audio track, detecting
// inline telephone-event (DTMF) packets that share the same m-line.
// WhatsApp sends both Opus audio and telephone-event on a single track.
// In pion v4, a new OnTrack may fire for telephone-event, but we also
// handle the case where DTMF arrives on the same track.
func (m *Manager) consumeAudioWithDTMF(session *CallSession, track *webrtc.TrackRemote) {
	audioPT := track.PayloadType()
	var lastDTMFEvent byte = 0xFF
	var lastEndBit bool
	packetCount := 0

	m.log.Info("Consuming audio with inline DTMF detection",
		"call_id", session.ID,
		"audio_pt", audioPT,
	)

	for {
		select {
		case <-session.BridgeStarted:
			return
		default:
		}

		pkt, _, err := track.ReadRTP()
		if err != nil {
			m.log.Debug("Audio track read ended", "call_id", session.ID, "error", err)
			return
		}

		packetCount++

		// Log every 500th packet and any non-audio packet for debugging
		if pkt.PayloadType != uint8(audioPT) {
			m.log.Info("Non-audio RTP packet received",
				"call_id", session.ID,
				"payload_type", pkt.PayloadType,
				"payload_len", len(pkt.Payload),
				"audio_pt", audioPT,
			)

			// Telephone-event DTMF payload is 4 bytes
			if len(pkt.Payload) >= 4 {
				eventID := pkt.Payload[0]
				endBit := (pkt.Payload[1] & 0x80) != 0

				if endBit && (lastDTMFEvent != eventID || !lastEndBit) {
					if digit, ok := dtmfDigits[eventID]; ok {
						m.log.Info("DTMF digit detected (inline)",
							"call_id", session.ID,
							"digit", string(digit),
							"event_id", eventID,
						)

						select {
						case session.DTMFBuffer <- digit:
						default:
							m.log.Warn("DTMF buffer full, dropping digit",
								"call_id", session.ID,
								"digit", string(digit),
							)
						}
					}
				}

				lastDTMFEvent = eventID
				lastEndBit = endBit
			}
		} else if packetCount == 1 {
			m.log.Debug("First audio packet received",
				"call_id", session.ID,
				"payload_type", pkt.PayloadType,
			)
		}
	}
}

// rejectCall sends a reject action via the WhatsApp API
func (m *Manager) rejectCall(ctx context.Context, account *whatsapp.Account, callID string) {
	if err := m.whatsapp.RejectCall(ctx, account, callID); err != nil {
		m.log.Error("Failed to reject call", "error", err, "call_id", callID)
	}
}
