package calling

import (
	"github.com/pion/webrtc/v4"
	"github.com/zerodha/logf"
)

// DTMF digit mapping from RFC 4733 event IDs to characters
var dtmfDigits = map[byte]byte{
	0:  '0',
	1:  '1',
	2:  '2',
	3:  '3',
	4:  '4',
	5:  '5',
	6:  '6',
	7:  '7',
	8:  '8',
	9:  '9',
	10: '*',
	11: '#',
}

// decodeDTMFEvent checks whether an RTP telephone-event represents a new DTMF
// digit press using end-bit debouncing. Returns the digit and true if a new
// press is detected, or (0, false) otherwise. Caller must pass pointers to
// persistent state variables that track the previous event.
func decodeDTMFEvent(eventID byte, endBit bool, lastEvent *byte, lastEndBit *bool) (byte, bool) {
	defer func() { *lastEvent = eventID; *lastEndBit = endBit }()
	if endBit && (*lastEvent != eventID || !*lastEndBit) {
		if digit, ok := dtmfDigits[eventID]; ok {
			return digit, true
		}
	}
	return 0, false
}

// sendDTMFDigit sends a decoded digit to the session's DTMF buffer (non-blocking).
func sendDTMFDigit(session *CallSession, digit byte, log logf.Logger) {
	select {
	case session.DTMFBuffer <- digit:
	default:
		log.Warn("DTMF buffer full, dropping digit",
			"call_id", session.ID,
			"digit", string(digit),
		)
	}
}

// handleDTMFTrack reads RTP telephone-event packets and extracts DTMF digits.
// RFC 4733 telephone-event RTP payload format:
//
//	0                   1                   2                   3
//	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     event     |E|R| volume    |          duration             |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// The E-bit (end bit) is set on the last packet of a DTMF event.
// We only emit the digit when we see the end bit to avoid duplicates.
func (m *Manager) handleDTMFTrack(session *CallSession, track *webrtc.TrackRemote) {
	buf := make([]byte, 1500)
	var lastEvent byte = 0xFF // impossible event ID as sentinel
	var lastEndBit bool

	for {
		n, _, err := track.Read(buf)
		if err != nil {
			m.log.Debug("DTMF track read ended", "call_id", session.ID, "error", err)
			return
		}

		if n < 4 {
			continue // Too short for a telephone-event payload
		}

		// Parse the RTP payload (after the RTP header is stripped by pion)
		eventID := buf[0]
		endBit := (buf[1] & 0x80) != 0

		// Debounce: only emit on the first end-bit packet for each event
		if digit, ok := decodeDTMFEvent(eventID, endBit, &lastEvent, &lastEndBit); ok {
			m.log.Info("DTMF digit detected",
				"call_id", session.ID,
				"digit", string(digit),
				"event_id", eventID,
			)
			sendDTMFDigit(session, digit, m.log)
		}
	}
}
