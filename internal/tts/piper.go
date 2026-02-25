package tts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// PiperTTS generates OGG/Opus audio files from text using the Piper TTS CLI
// and opusenc for WAV→OGG/Opus conversion. No cgo dependency required.
type PiperTTS struct {
	BinaryPath    string // path to piper executable
	ModelPath     string // path to .onnx model file
	OpusencBinary string // path to opusenc (defaults to "opusenc")
	AudioDir      string // output directory for generated files
}

// Generate converts text to an OGG/Opus audio file. Returns the filename.
// Uses SHA256 hash of text as the filename for caching — same text produces the same file.
func (p *PiperTTS) Generate(text string) (string, error) {
	hash := sha256Short(text)
	filename := "tts_" + hash + ".ogg"
	outPath := filepath.Join(p.AudioDir, filename)

	// Cache hit — file already exists
	if fileExists(outPath) {
		return filename, nil
	}

	// Ensure output directory exists
	if err := os.MkdirAll(p.AudioDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create audio directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Step 1: Generate WAV using piper
	wavPath := outPath + ".tmp.wav"
	defer func() { _ = os.Remove(wavPath) }() // clean up temp WAV

	piperCmd := exec.CommandContext(ctx, p.BinaryPath,
		"--model", p.ModelPath,
		"--output_file", wavPath,
		"--length_scale", "1.0",
	)
	piperCmd.Stdin = bytes.NewReader([]byte(text))

	var piperStderr bytes.Buffer
	piperCmd.Stderr = &piperStderr

	if err := piperCmd.Run(); err != nil {
		return "", fmt.Errorf("piper TTS failed: %w (stderr: %s)", err, piperStderr.String())
	}

	// Verify WAV was created
	if !fileExists(wavPath) {
		return "", fmt.Errorf("piper did not produce output file")
	}

	// Step 2: Convert WAV → OGG/Opus using opusenc
	opusenc := p.OpusencBinary
	if opusenc == "" {
		opusenc = "opusenc"
	}

	// Write to a temp file first, then rename for atomicity
	tmpOgg := outPath + ".tmp.ogg"
	defer func() { _ = os.Remove(tmpOgg) }() // clean up on error

	encCmd := exec.CommandContext(ctx, opusenc,
		"--bitrate", "24",
		"--quiet",
		wavPath,
		tmpOgg,
	)

	var encStderr bytes.Buffer
	encCmd.Stderr = &encStderr

	if err := encCmd.Run(); err != nil {
		return "", fmt.Errorf("opusenc failed: %w (stderr: %s)", err, encStderr.String())
	}

	// Atomic rename
	if err := os.Rename(tmpOgg, outPath); err != nil {
		return "", fmt.Errorf("failed to finalize audio file: %w", err)
	}

	return filename, nil
}

// sha256Short returns the first 16 hex characters of the SHA256 hash of s.
func sha256Short(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

// fileExists returns true if the file at path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
