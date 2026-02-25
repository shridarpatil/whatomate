package calling

import (
	"github.com/pion/webrtc/v4"
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
		if endBit && (lastEvent != eventID || !lastEndBit) {
			if digit, ok := dtmfDigits[eventID]; ok {
				m.log.Info("DTMF digit detected",
					"call_id", session.ID,
					"digit", string(digit),
					"event_id", eventID,
				)

				// Non-blocking send to buffer
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

		lastEvent = eventID
		lastEndBit = endBit
	}
}
