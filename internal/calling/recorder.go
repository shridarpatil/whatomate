package calling

import (
	"encoding/binary"
	"os"
	"sync"
)

// oggCRCTable is the pre-computed lookup table for OGG's direct (non-reflected)
// CRC-32 using polynomial 0x04C11DB7.
var oggCRCTable [256]uint32

func init() {
	for i := 0; i < 256; i++ {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if r&0x80000000 != 0 {
				r = (r << 1) ^ 0x04C11DB7
			} else {
				r <<= 1
			}
		}
		oggCRCTable[i] = r
	}
}

// CallRecorder writes interleaved Opus packets from both call directions
// into a single-channel OGG/Opus file. Packets are expected to arrive at
// ~20ms intervals (960 samples at 48kHz). The recorder is safe for
// concurrent use from multiple goroutines (the two bridge directions).
type CallRecorder struct {
	mu            sync.Mutex
	file          *os.File
	path          string
	granulePos    uint64
	pageSeqNo     uint32
	packetCount   int
	stopped       bool

	// Buffer packets into OGG pages (flush every N packets)
	pageBuf       [][]byte
	pageBufBytes  int
}

const (
	samplesPerFrame20ms = 960  // 48kHz * 20ms
	maxPagePackets      = 48   // ~960ms per page — keeps pages reasonable
	oggPageHeaderLen    = 27
)

// NewCallRecorder creates a new recorder writing to a temp file.
// Returns nil if the temp file cannot be created.
func NewCallRecorder() (*CallRecorder, error) {
	f, err := os.CreateTemp("", "call-recording-*.ogg")
	if err != nil {
		return nil, err
	}

	r := &CallRecorder{
		file: f,
		path: f.Name(),
	}

	// Write OpusHead and OpusTags header pages
	if err := r.writeHeaders(); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}

	return r, nil
}

// WritePacket adds an Opus packet to the recording. Thread-safe.
func (r *CallRecorder) WritePacket(opusData []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		return
	}

	pkt := make([]byte, len(opusData))
	copy(pkt, opusData)
	r.pageBuf = append(r.pageBuf, pkt)
	r.pageBufBytes += len(pkt)
	r.packetCount++

	if len(r.pageBuf) >= maxPagePackets {
		r.flushPage(false)
	}
}

// Stop finalizes the OGG file and returns the path to the recording.
// After Stop, WritePacket calls are no-ops.
func (r *CallRecorder) Stop() (string, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		return r.path, r.packetCount
	}
	r.stopped = true

	// Flush remaining packets as the final page
	if len(r.pageBuf) > 0 {
		r.flushPage(true)
	}

	_ = r.file.Close()
	return r.path, r.packetCount
}

// PacketCount returns the number of packets written so far.
func (r *CallRecorder) PacketCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.packetCount
}

// writeHeaders writes the two required OGG header pages:
// Page 0: OpusHead (ID header)
// Page 1: OpusTags (comment header)
func (r *CallRecorder) writeHeaders() error {
	// OpusHead: https://www.rfc-editor.org/rfc/rfc7845#section-5.1
	opusHead := make([]byte, 19)
	copy(opusHead[0:8], "OpusHead")
	opusHead[8] = 1   // version
	opusHead[9] = 1   // channel count (mono)
	binary.LittleEndian.PutUint16(opusHead[10:12], 0)     // pre-skip
	binary.LittleEndian.PutUint32(opusHead[12:16], 48000) // input sample rate
	binary.LittleEndian.PutUint16(opusHead[16:18], 0)     // output gain
	opusHead[18] = 0 // channel mapping family

	if err := r.writePage(opusHead, true, false, 0); err != nil {
		return err
	}

	// OpusTags: https://www.rfc-editor.org/rfc/rfc7845#section-5.2
	vendor := "whatomate"
	opusTags := make([]byte, 8+4+len(vendor)+4)
	copy(opusTags[0:8], "OpusTags")
	binary.LittleEndian.PutUint32(opusTags[8:12], uint32(len(vendor)))
	copy(opusTags[12:12+len(vendor)], vendor)
	binary.LittleEndian.PutUint32(opusTags[12+len(vendor):], 0) // no comments

	if err := r.writePage(opusTags, false, false, 0); err != nil {
		return err
	}

	return nil
}

// flushPage writes buffered packets as a single OGG page.
func (r *CallRecorder) flushPage(lastPage bool) {
	if len(r.pageBuf) == 0 {
		return
	}

	// Build segment table: each packet's size encoded as 255-byte segments
	var segTable []byte
	for _, pkt := range r.pageBuf {
		remaining := len(pkt)
		for remaining >= 255 {
			segTable = append(segTable, 255)
			remaining -= 255
		}
		segTable = append(segTable, byte(remaining))
	}

	// Advance granule position: each packet represents one 20ms frame
	r.granulePos += uint64(len(r.pageBuf)) * samplesPerFrame20ms

	// Concatenate packet data
	payload := make([]byte, 0, r.pageBufBytes)
	for _, pkt := range r.pageBuf {
		payload = append(payload, pkt...)
	}

	// Build OGG page header
	headerSize := oggPageHeaderLen + len(segTable)
	page := make([]byte, headerSize+len(payload))

	copy(page[0:4], "OggS")     // capture pattern
	page[4] = 0                 // version
	page[5] = 0                 // header type flags
	if lastPage {
		page[5] |= 0x04 // end of stream
	}
	binary.LittleEndian.PutUint64(page[6:14], r.granulePos)
	binary.LittleEndian.PutUint32(page[14:18], 0) // serial number (stream 0)
	binary.LittleEndian.PutUint32(page[18:22], r.pageSeqNo)
	// checksum placeholder (bytes 22-25) — will be filled below
	page[26] = byte(len(segTable))

	copy(page[oggPageHeaderLen:], segTable)
	copy(page[headerSize:], payload)

	// Compute CRC32 with the checksum field zeroed
	binary.LittleEndian.PutUint32(page[22:26], 0)
	checksum := oggCRC32(page)
	binary.LittleEndian.PutUint32(page[22:26], checksum)

	if _, err := r.file.Write(page); err != nil {
		return
	}
	r.pageSeqNo++

	// Clear buffer
	r.pageBuf = r.pageBuf[:0]
	r.pageBufBytes = 0
}

// writePage writes a single OGG page with the given payload (for header pages).
func (r *CallRecorder) writePage(payload []byte, bos bool, eos bool, granule uint64) error {
	// Single-segment page
	segTable := []byte{}
	remaining := len(payload)
	for remaining >= 255 {
		segTable = append(segTable, 255)
		remaining -= 255
	}
	segTable = append(segTable, byte(remaining))

	headerSize := oggPageHeaderLen + len(segTable)
	page := make([]byte, headerSize+len(payload))

	copy(page[0:4], "OggS")
	page[4] = 0 // version
	page[5] = 0
	if bos {
		page[5] |= 0x02
	}
	if eos {
		page[5] |= 0x04
	}
	binary.LittleEndian.PutUint64(page[6:14], granule)
	binary.LittleEndian.PutUint32(page[14:18], 0) // serial number
	binary.LittleEndian.PutUint32(page[18:22], r.pageSeqNo)
	page[26] = byte(len(segTable))

	copy(page[oggPageHeaderLen:], segTable)
	copy(page[headerSize:], payload)

	binary.LittleEndian.PutUint32(page[22:26], 0)
	checksum := oggCRC32(page)
	binary.LittleEndian.PutUint32(page[22:26], checksum)

	if _, err := r.file.Write(page); err != nil {
		return err
	}
	r.pageSeqNo++
	return nil
}

// oggCRC32 computes the OGG-specific CRC32 checksum.
// OGG uses a direct (non-reflected) CRC-32 with polynomial 0x04C11DB7,
// which differs from Go's standard crc32 package (reflected algorithm).
func oggCRC32(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc = (crc << 8) ^ oggCRCTable[(crc>>24)^uint32(b)]
	}
	return crc
}
