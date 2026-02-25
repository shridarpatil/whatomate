package calling

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// AudioPlayer handles playing pre-recorded OGG/Opus audio into a WebRTC track.
// It maintains cumulative RTP sequence numbers and timestamps across multiple
// PlayFile calls so that receivers don't drop packets as duplicates.
type AudioPlayer struct {
	track          *webrtc.TrackLocalStaticRTP
	stop           chan struct{}
	sequenceNumber uint16
	timestamp      uint32
}

// NewAudioPlayer creates a new audio player for a WebRTC track.
func NewAudioPlayer(track *webrtc.TrackLocalStaticRTP) *AudioPlayer {
	return &AudioPlayer{
		track: track,
		stop:  make(chan struct{}),
	}
}

// PlayFile plays an OGG/Opus audio file into the WebRTC track.
// It parses the OGG container, splits pages into individual Opus packets
// using the segment table, and sends each as a properly timed RTP packet.
// Returns the number of RTP packets sent.
func (p *AudioPlayer) PlayFile(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close() //nolint:errcheck

	packets, err := readOpusPackets(file)
	if err != nil {
		return 0, fmt.Errorf("failed to read Opus packets: %w", err)
	}

	// Opus at 48kHz, 20ms frames = 960 samples per frame
	const samplesPerFrame = 960

	packetCount := 0

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for _, opusData := range packets {
		select {
		case <-p.stop:
			return packetCount, nil
		case <-ticker.C:
			rtpPkt := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    111, // Opus
					SequenceNumber: p.sequenceNumber,
					Timestamp:      p.timestamp,
					SSRC:           1,
				},
				Payload: opusData,
			}

			if err := p.track.WriteRTP(rtpPkt); err != nil {
				return packetCount, fmt.Errorf("failed to write RTP packet: %w", err)
			}

			p.sequenceNumber++
			p.timestamp += samplesPerFrame
			packetCount++
		}
	}

	return packetCount, nil
}

// Stop stops the current audio playback
func (p *AudioPlayer) Stop() {
	select {
	case <-p.stop:
		// Already stopped/closed
	default:
		close(p.stop)
	}
}

// IsStopped returns true if the player has been stopped.
func (p *AudioPlayer) IsStopped() bool {
	select {
	case <-p.stop:
		return true
	default:
		return false
	}
}

// PlayFileLoop plays an OGG/Opus audio file in a continuous loop until Stop() is called.
func (p *AudioPlayer) PlayFileLoop(filePath string) error {
	for {
		if _, err := p.PlayFile(filePath); err != nil {
			return err
		}
		// Check stop between loop iterations
		select {
		case <-p.stop:
			return nil
		default:
		}
	}
}

// PlaySilence sends silence packets for the specified duration.
// This keeps the RTP stream alive during pauses.
func (p *AudioPlayer) PlaySilence(duration time.Duration) {
	// Opus silence frame (a minimal valid Opus packet representing silence)
	silence := []byte{0xF8, 0xFF, 0xFE}

	const samplesPerFrame = 960

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.After(duration)
	for {
		select {
		case <-p.stop:
			return
		case <-deadline:
			return
		case <-ticker.C:
			packet := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    111,
					SequenceNumber: p.sequenceNumber,
					Timestamp:      p.timestamp,
					SSRC:           1,
				},
				Payload: silence,
			}
			if err := p.track.WriteRTP(packet); err != nil {
				return
			}
			p.sequenceNumber++
			p.timestamp += samplesPerFrame
		}
	}
}

// readOpusPackets parses an OGG stream and returns individual Opus packets,
// properly splitting multi-packet OGG pages using the segment table.
// Header pages (OpusHead, OpusTags) are skipped.
func readOpusPackets(r io.Reader) ([][]byte, error) {
	const oggPageHeaderLen = 27

	var packets [][]byte

	// Skip OpusHead and OpusTags by counting initial header pages
	headersSkipped := 0

	for {
		// Read the 27-byte OGG page header
		header := make([]byte, oggPageHeaderLen)
		if _, err := io.ReadFull(r, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, fmt.Errorf("failed to read page header: %w", err)
		}

		// Validate OGG signature
		if string(header[0:4]) != "OggS" {
			return nil, fmt.Errorf("invalid OGG page signature")
		}

		segmentsCount := int(header[26])

		// Read segment table
		segTable := make([]byte, segmentsCount)
		if _, err := io.ReadFull(r, segTable); err != nil {
			return nil, fmt.Errorf("failed to read segment table: %w", err)
		}

		// Calculate total payload size
		payloadSize := 0
		for _, s := range segTable {
			payloadSize += int(s)
		}

		// Read the full page payload
		payload := make([]byte, payloadSize)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, fmt.Errorf("failed to read page payload: %w", err)
		}

		// Skip first two pages (OpusHead and OpusTags)
		if headersSkipped < 2 {
			headersSkipped++
			continue
		}

		// Split payload into individual Opus packets using the segment table.
		// In OGG, a segment of 255 bytes means the packet continues in the
		// next segment. A segment < 255 bytes marks the end of a packet.
		offset := 0
		var currentPacket []byte

		for _, segSize := range segTable {
			size := int(segSize)
			if offset+size > len(payload) {
				break
			}
			currentPacket = append(currentPacket, payload[offset:offset+size]...)
			offset += size

			// Segment size < 255 means end of this Opus packet
			if segSize < 255 {
				if len(currentPacket) > 0 {
					pkt := make([]byte, len(currentPacket))
					copy(pkt, currentPacket)
					packets = append(packets, pkt)
				}
				currentPacket = currentPacket[:0]
			}
		}

		// Handle trailing packet (if last segment was exactly 255)
		if len(currentPacket) > 0 {
			pkt := make([]byte, len(currentPacket))
			copy(pkt, currentPacket)
			packets = append(packets, pkt)
		}
	}

	return packets, nil
}

