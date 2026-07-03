package calling

import (
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// AudioBridge forwards RTP packets bidirectionally between two WebRTC tracks.
// It bridges the caller's remote track to the agent's local track, and vice versa.
type AudioBridge struct {
	stop      chan struct{}
	wg        sync.WaitGroup
	callerRec *CallRecorder // records caller's audio (caller→agent direction), may be nil
	agentRec  *CallRecorder // records agent's audio (agent→caller direction), may be nil

	// lastCallerSeq and lastCallerTS track the last RTP sequence number and
	// timestamp forwarded to the caller's track (agent→caller direction).
	// Used to maintain RTP stream continuity when switching to hold music.
	lastCallerSeq uint16
	lastCallerTS  uint32

	// seqOffset / tsOffset are added to agent→caller RTP packets so the
	// receiver sees a continuous stream after hold music. Without this,
	// the agent's low sequence numbers are discarded as "old" because the
	// hold music player advanced the receiver's high-water mark.
	seqOffset     uint16
	tsOffset      uint32
	firstAgentSeq bool // true until the first agent packet sets the base
	agentBaseSeq  uint16
	agentBaseTS   uint32

	// mu guards stopped and callerAttached. callerSlot (buffered, cap 1)
	// hands a late caller→agent leg to the slot goroutine reserved by Start,
	// so Start is the single owner of wg.Add and AttachCaller never touches
	// the WaitGroup after Wait may have begun.
	mu             sync.Mutex
	stopped        bool
	callerAttached bool
	callerSlot     chan callerLeg
}

// callerLeg is a late caller→agent forwarding request delivered to the slot
// goroutine reserved by Start (see AttachCaller).
type callerLeg struct {
	src *webrtc.TrackRemote
	dst *webrtc.TrackLocalStaticRTP
}

// NewAudioBridge creates a new audio bridge with optional per-direction recorders.
// Each direction gets its own recorder so the two independent Opus streams are
// kept in separate OGG files and can be merged correctly after the call.
func NewAudioBridge(callerRec, agentRec *CallRecorder) *AudioBridge {
	return &AudioBridge{
		stop:       make(chan struct{}),
		callerRec:  callerRec,
		agentRec:   agentRec,
		callerSlot: make(chan callerLeg, 1),
	}
}

// SeedSequence sets the starting sequence/timestamp for the agent→caller
// direction so that packets continue past the hold music high-water mark.
func (b *AudioBridge) SeedSequence(seq uint16, ts uint32) {
	b.seqOffset = seq
	b.tsOffset = ts
	b.firstAgentSeq = true
}

// Start begins bidirectional RTP forwarding. It blocks until the bridge is
// stopped (or both directions end after the caller leg has been launched or
// attached). Nil tracks are skipped to avoid panics when a PeerConnection
// never connected; when the caller leg cannot be launched yet, a slot
// goroutine is reserved for it so AttachCaller can feed it later without
// ever calling wg.Add itself — all Adds happen here, before Wait.
func (b *AudioBridge) Start(
	callerRemote *webrtc.TrackRemote, agentLocal *webrtc.TrackLocalStaticRTP,
	agentRemote *webrtc.TrackRemote, callerLocal *webrtc.TrackLocalStaticRTP,
) {
	// Caller audio → Agent speaker (record caller's voice)
	if callerRemote != nil && agentLocal != nil {
		b.mu.Lock()
		b.callerAttached = true
		b.mu.Unlock()
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			b.forward(callerRemote, agentLocal, b.callerRec, false)
		}()
	} else {
		// Reserve the caller slot: on incoming calls the caller's track often
		// arrives only after the agent answers. Holding a WaitGroup slot here
		// means AttachCaller never races Start's wg.Wait, even if the agent
		// leg exits first.
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			select {
			case <-b.stop:
			case leg := <-b.callerSlot:
				b.forward(leg.src, leg.dst, b.callerRec, false)
			}
		}()
	}

	// Agent mic → Caller speaker (record agent's voice, track seq/ts)
	if agentRemote != nil && callerLocal != nil {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			b.forward(agentRemote, callerLocal, b.agentRec, true)
		}()
	}

	b.wg.Wait()
}

// AttachCaller hands the caller→agent leg to the bridge after Start() has
// already begun. On incoming calls the caller's media track frequently
// arrives only once the agent answers — after the bridge was started with a
// nil callerRemote — so no caller→agent goroutine exists and audio is
// one-way. The peer's OnTrack handler calls this when the track finally
// lands. Idempotent and a no-op once the bridge is stopped. It never touches
// the WaitGroup: the leg runs on the slot goroutine reserved by Start, so it
// cannot race Start's wg.Wait regardless of when the other legs exit.
func (b *AudioBridge) AttachCaller(callerRemote *webrtc.TrackRemote, agentLocal *webrtc.TrackLocalStaticRTP) {
	if callerRemote == nil || agentLocal == nil {
		return
	}
	b.mu.Lock()
	if b.stopped || b.callerAttached {
		b.mu.Unlock()
		return
	}
	b.callerAttached = true
	b.mu.Unlock()

	// Buffered (cap 1) and gated by callerAttached, so at most one leg is
	// ever sent and this never blocks. If Stop() won the race above, the slot
	// goroutine already exited via b.stop and the value is simply discarded
	// with the bridge; if it picks the leg anyway, forward exits on its first
	// stop-channel check.
	b.callerSlot <- callerLeg{src: callerRemote, dst: agentLocal}
}

// forward reads RTP packets from src and writes them to dst until stopped.
// If rec is non-nil, the Opus payload of each packet is teed to it.
// When trackSeq is true and a sequence offset has been seeded via SeedSequence,
// the agent's RTP seq/ts are rewritten to continue past the hold music
// high-water mark so the receiver doesn't discard them as old.
func (b *AudioBridge) forward(src *webrtc.TrackRemote, dst *webrtc.TrackLocalStaticRTP, rec *CallRecorder, trackSeq bool) {
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

		// Rewrite seq/ts for agent→caller direction when seeded
		if trackSeq && b.firstAgentSeq {
			pkt := &rtp.Packet{}
			if err := pkt.Unmarshal(buf[:n]); err == nil {
				b.agentBaseSeq = pkt.SequenceNumber
				b.agentBaseTS = pkt.Timestamp
				b.firstAgentSeq = false
			}
		}

		if trackSeq && b.seqOffset > 0 {
			pkt := &rtp.Packet{}
			if err := pkt.Unmarshal(buf[:n]); err == nil {
				pkt.SequenceNumber = b.seqOffset + (pkt.SequenceNumber - b.agentBaseSeq) + 1
				pkt.Timestamp = b.tsOffset + (pkt.Timestamp - b.agentBaseTS) + 960

				b.lastCallerSeq = pkt.SequenceNumber
				b.lastCallerTS = pkt.Timestamp

				rewritten, err := pkt.Marshal()
				if err == nil {
					if _, err := dst.Write(rewritten); err != nil {
						return
					}
				}

				if rec != nil && len(pkt.Payload) > 0 {
					rec.WritePacket(pkt.Payload)
				}
				continue
			}
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

// Stop terminates all forwarding goroutines and releases the caller slot if
// it was never fed. After Stop, AttachCaller is a guaranteed no-op.
func (b *AudioBridge) Stop() {
	b.mu.Lock()
	b.stopped = true
	b.mu.Unlock()
	safeClose(b.stop)
}

// Wait blocks until all forwarding goroutines (and the reserved caller slot,
// if any) have exited. Call Stop first.
func (b *AudioBridge) Wait() {
	b.wg.Wait()
}

// LastCallerSeq returns the last RTP sequence number and timestamp forwarded
// to the caller's track. Only valid after Stop()+Wait().
func (b *AudioBridge) LastCallerSeq() (uint16, uint32) {
	return b.lastCallerSeq, b.lastCallerTS
}
