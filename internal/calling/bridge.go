package calling

import (
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// AudioBridge forwards RTP packets bidirectionally between two WebRTC tracks.
// It bridges the caller's remote track to the agent's local track, and vice versa.
type AudioBridge struct {
	stop          chan struct{}
	wg            sync.WaitGroup
	callerRec     *CallRecorder // records caller's audio (caller→agent direction), may be nil
	agentRec      *CallRecorder // records agent's audio (agent→caller direction), may be nil

	// lastCallerSeq and lastCallerTS track the last RTP sequence number and
	// timestamp forwarded to the caller's track (agent→caller direction).
	// Used to maintain RTP stream continuity when switching to hold music.
	lastCallerSeq uint16
	lastCallerTS  uint32
}

// NewAudioBridge creates a new audio bridge with optional per-direction recorders.
// Each direction gets its own recorder so the two independent Opus streams are
// kept in separate OGG files and can be merged correctly after the call.
func NewAudioBridge(callerRec, agentRec *CallRecorder) *AudioBridge {
	return &AudioBridge{
		stop:      make(chan struct{}),
		callerRec: callerRec,
		agentRec:  agentRec,
	}
}

// Start begins bidirectional RTP forwarding. It blocks until both directions end.
// Nil tracks are skipped to avoid panics when a PeerConnection never connected.
func (b *AudioBridge) Start(
	callerRemote *webrtc.TrackRemote, agentLocal *webrtc.TrackLocalStaticRTP,
	agentRemote *webrtc.TrackRemote, callerLocal *webrtc.TrackLocalStaticRTP,
) {
	// Caller audio → Agent speaker (record caller's voice)
	if callerRemote != nil && agentLocal != nil {
		b.wg.Add(1)
		go b.forward(callerRemote, agentLocal, b.callerRec, false)
	}

	// Agent mic → Caller speaker (record agent's voice, track seq/ts)
	if agentRemote != nil && callerLocal != nil {
		b.wg.Add(1)
		go b.forward(agentRemote, callerLocal, b.agentRec, true)
	}

	b.wg.Wait()
}

// forward reads RTP packets from src and writes them to dst until stopped.
// If rec is non-nil, the Opus payload of each packet is teed to it.
func (b *AudioBridge) forward(src *webrtc.TrackRemote, dst *webrtc.TrackLocalStaticRTP, rec *CallRecorder, trackSeq bool) {
	defer b.wg.Done()

	buf := make([]byte, 1500)
	for {
		select {
		case <-b.stop:
			return
		default:
		}

		n, _, err := src.Read(buf)
		if err != nil {
			return
		}

		if _, err := dst.Write(buf[:n]); err != nil {
			return
		}

		// Parse packet for recording and/or seq tracking.
		if rec != nil || trackSeq {
			pkt := &rtp.Packet{}
			if err := pkt.Unmarshal(buf[:n]); err == nil {
				if trackSeq {
					b.lastCallerSeq = pkt.SequenceNumber
					b.lastCallerTS = pkt.Timestamp
				}
				if rec != nil && len(pkt.Payload) > 0 {
					rec.WritePacket(pkt.Payload)
				}
			}
		}
	}
}

// Stop terminates both forwarding goroutines.
func (b *AudioBridge) Stop() {
	safeClose(b.stop)
}

// Wait blocks until both forwarding goroutines have exited.
func (b *AudioBridge) Wait() {
	b.wg.Wait()
}

// LastCallerSeq returns the last RTP sequence number and timestamp forwarded
// to the caller's track. Only valid after Stop()+Wait().
func (b *AudioBridge) LastCallerSeq() (uint16, uint32) {
	return b.lastCallerSeq, b.lastCallerTS
}
