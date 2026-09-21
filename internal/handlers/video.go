package handlers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxWhatsAppVideoBytes = 15.5 * 1024 * 1024 // 15.5 MB safety threshold (Meta limit: 16 MB)

// compressVideoForWhatsApp recompresses an input video using server-side ffmpeg
// into a standardized H.264/AAC MP4 with faststart, targeting <15.5 MB so it complies
// with WhatsApp Cloud API's strict 16 MB limit and codec requirements.
func compressVideoForWhatsApp(inputData []byte) ([]byte, error) {
	if len(inputData) == 0 {
		return nil, fmt.Errorf("empty video data")
	}

	// Check if ffmpeg is available
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	// Write input data to temp file
	tmpIn, err := os.CreateTemp("", "wa-video-in-*.mp4")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer func() { _ = os.Remove(tmpIn.Name()) }()

	if _, err := tmpIn.Write(inputData); err != nil {
		_ = tmpIn.Close()
		return nil, fmt.Errorf("failed to write temp input file: %w", err)
	}
	_ = tmpIn.Close()

	// Temp output file
	tmpOutPath := filepath.Join(os.TempDir(), fmt.Sprintf("wa-video-out-%d.mp4", time.Now().UnixNano()))
	defer func() { _ = os.Remove(tmpOutPath) }()

	// Probe duration using ffprobe if available
	durationSec := probeVideoDuration(tmpIn.Name())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var args []string
	if durationSec > 0 && float64(len(inputData)) > maxWhatsAppVideoBytes {
		// Calculate target video bitrate to ensure the file fits comfortably within 14.5 MB
		targetBits := 14.5 * 1024 * 1024 * 8.0
		totalBitrate := targetBits / durationSec
		audioBitrate := 96000.0 // 96 kbps audio
		videoBitrate := totalBitrate - audioBitrate
		if videoBitrate < 150000 {
			videoBitrate = 150000 // minimum 150 kbps
		}

		vBitrateK := int(videoBitrate / 1000)

		// Choose scale resolution based on bitrate
		scaleFilter := "scale='min(1280,iw)':-2"
		if vBitrateK < 800 {
			scaleFilter = "scale='min(854,iw)':-2" // 480p for lower bitrates
		}
		if vBitrateK < 400 {
			scaleFilter = "scale='min(640,iw)':-2" // 360p for very low bitrates
		}

		args = []string{
			"-y",
			"-i", tmpIn.Name(),
			"-vf", scaleFilter,
			"-c:v", "libx264",
			"-preset", "fast",
			"-b:v", fmt.Sprintf("%dk", vBitrateK),
			"-maxrate", fmt.Sprintf("%dk", int(float64(vBitrateK)*1.3)),
			"-bufsize", fmt.Sprintf("%dk", vBitrateK*2),
			"-c:a", "aac",
			"-b:a", "96k",
			"-movflags", "+faststart",
			"-pix_fmt", "yuv420p",
			tmpOutPath,
		}
	} else {
		// Default single-pass CRF compression for high quality < 15.5 MB
		args = []string{
			"-y",
			"-i", tmpIn.Name(),
			"-vf", "scale='min(1280,iw)':-2",
			"-c:v", "libx264",
			"-preset", "fast",
			"-crf", "26",
			"-maxrate", "2200k",
			"-bufsize", "4400k",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
			"-pix_fmt", "yuv420p",
			tmpOutPath,
		}
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg transcoding failed: %w (stderr: %s)", err, stderr.String())
	}

	outData, err := os.ReadFile(tmpOutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read transcoded video: %w", err)
	}

	if len(outData) == 0 {
		return nil, fmt.Errorf("transcoded video is empty")
	}

	return outData, nil
}

// probeVideoDuration attempts to get duration in seconds using ffprobe or ffmpeg
func probeVideoDuration(filePath string) float64 {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err == nil {
		cmd := exec.Command(ffprobePath,
			"-v", "error",
			"-show_entries", "format=duration",
			"-of", "default=noprint_wrappers=1:nokey=1",
			filePath,
		)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			durStr := strings.TrimSpace(out.String())
			if dur, err := strconv.ParseFloat(durStr, 64); err == nil && dur > 0 {
				return dur
			}
		}
	}

	// Fallback to ffmpeg -i parsing duration
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err == nil {
		cmd := exec.Command(ffmpegPath, "-i", filePath)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		_ = cmd.Run()
		output := stderr.String()
		if idx := strings.Index(output, "Duration: "); idx != -1 {
			sub := output[idx+len("Duration: "):]
			if comma := strings.Index(sub, ","); comma != -1 {
				timeStr := strings.TrimSpace(sub[:comma])
				parts := strings.Split(timeStr, ":")
				if len(parts) == 3 {
					h, _ := strconv.ParseFloat(parts[0], 64)
					m, _ := strconv.ParseFloat(parts[1], 64)
					s, _ := strconv.ParseFloat(parts[2], 64)
					total := h*3600 + m*60 + s
					if total > 0 {
						return total
					}
				}
			}
		}
	}

	return 0
}
