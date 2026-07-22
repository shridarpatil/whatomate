package tts_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shridarpatil/whatomate/internal/tts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBin writes a small shell script that emulates piper or opusenc by
// touching its --output_file (or last positional arg for opusenc) with a
// minimal payload. Returns the absolute path to the script.
func fakeBin(t *testing.T, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary trick assumes /bin/sh")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0755))
	return path
}

// fakePiper writes a script that, given --output_file <path>, writes a fake
// WAV header to that file.
func fakePiper(t *testing.T) string {
	return fakeBin(t, "piper", `
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output_file) out="$2"; shift 2;;
    *) shift;;
  esac
done
# Read stdin so the test harness can verify text was piped in (we don't use it here).
cat > /dev/null
printf "RIFF\x24\x00\x00\x00WAVE" > "$out"
`)
}

// fakeOpusenc writes a script that copies its second positional arg's input
// to its third positional arg's output (treats them as in/out filenames).
func fakeOpusenc(t *testing.T) string {
	return fakeBin(t, "opusenc", `
# opusenc [opts...] in.wav out.ogg — last two non-option args.
prev=""
last=""
for a in "$@"; do
  prev="$last"
  last="$a"
done
in="$prev"
out="$last"
printf "OggS" > "$out"
test -f "$in"
`)
}

func TestPiperTTS_Generate_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	tts := &tts.PiperTTS{
		BinaryPath:    fakePiper(t),
		ModelPath:     filepath.Join(dir, "model.onnx"), // contents irrelevant — fake piper ignores
		OpusencBinary: fakeOpusenc(t),
		AudioDir:      dir,
	}

	got, err := tts.Generate("hello world")
	require.NoError(t, err)
	assert.True(t, len(got) > 0)
	assert.True(t, filepath.IsAbs(filepath.Join(dir, got)))

	full := filepath.Join(dir, got)
	stat, err := os.Stat(full)
	require.NoError(t, err)
	assert.False(t, stat.IsDir())
	assert.Greater(t, stat.Size(), int64(0))
}

func TestPiperTTS_Generate_DeterministicFilename(t *testing.T) {
	dir := t.TempDir()
	tts := &tts.PiperTTS{
		BinaryPath:    fakePiper(t),
		ModelPath:     "model",
		OpusencBinary: fakeOpusenc(t),
		AudioDir:      dir,
	}

	a, err := tts.Generate("the quick brown fox")
	require.NoError(t, err)
	b, err := tts.Generate("the quick brown fox")
	require.NoError(t, err)
	assert.Equal(t, a, b, "same text → same filename (cache key)")

	c, err := tts.Generate("different text")
	require.NoError(t, err)
	assert.NotEqual(t, a, c, "different text → different filename")
}

func TestPiperTTS_Generate_CachesExistingFile(t *testing.T) {
	dir := t.TempDir()
	// Seed the cache: write a fake output for what we know the hash will be.
	tts1 := &tts.PiperTTS{
		BinaryPath:    fakePiper(t),
		ModelPath:     "model",
		OpusencBinary: fakeOpusenc(t),
		AudioDir:      dir,
	}
	first, err := tts1.Generate("cached")
	require.NoError(t, err)
	cachedPath := filepath.Join(dir, first)
	stat1, err := os.Stat(cachedPath)
	require.NoError(t, err)

	// Replace the file with custom content, then call Generate again.
	require.NoError(t, os.WriteFile(cachedPath, []byte("custom-cached-content"), 0644))
	stat2, err := os.Stat(cachedPath)
	require.NoError(t, err)

	// Use a piper that would FAIL if invoked — proves cache hit short-circuits.
	tts2 := &tts.PiperTTS{
		BinaryPath:    fakeBin(t, "piper", "exit 1"),
		ModelPath:     "model",
		OpusencBinary: fakeBin(t, "opusenc", "exit 1"),
		AudioDir:      dir,
	}
	second, err := tts2.Generate("cached")
	require.NoError(t, err, "cache hit should not invoke piper or opusenc")
	assert.Equal(t, first, second)

	// Content untouched (proves we did not regenerate).
	stat3, err := os.Stat(cachedPath)
	require.NoError(t, err)
	assert.Equal(t, stat2.ModTime(), stat3.ModTime())
	_ = stat1
}

func TestPiperTTS_Generate_PiperFailureSurfacesStderr(t *testing.T) {
	dir := t.TempDir()
	tts := &tts.PiperTTS{
		BinaryPath:    fakeBin(t, "piper", `echo "model not found" >&2; exit 2`),
		ModelPath:     "missing-model.onnx",
		OpusencBinary: fakeOpusenc(t),
		AudioDir:      dir,
	}

	_, err := tts.Generate("anything")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "piper TTS failed")
	assert.Contains(t, err.Error(), "model not found")
}

func TestPiperTTS_Generate_OpusencFailureCleansUpWavTemp(t *testing.T) {
	dir := t.TempDir()
	tts := &tts.PiperTTS{
		BinaryPath:    fakePiper(t),
		ModelPath:     "model",
		OpusencBinary: fakeBin(t, "opusenc", `echo "bad bitrate" >&2; exit 1`),
		AudioDir:      dir,
	}

	_, err := tts.Generate("anything")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opusenc failed")
	assert.Contains(t, err.Error(), "bad bitrate")

	// No final .ogg should remain.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".ogg", "no final ogg file should be left behind on opusenc failure")
		// .tmp.wav should also have been cleaned up by deferred remove.
		assert.NotContains(t, e.Name(), ".tmp.wav")
	}
}

func TestPiperTTS_Generate_OpusencDefaultIsUsedWhenEmpty(t *testing.T) {
	// If OpusencBinary is empty, the code defaults to "opusenc" from PATH.
	// Stand up a fake opusenc on PATH to verify this branch.
	dir := t.TempDir()
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "opusenc"), []byte(`#!/bin/sh
prev=""
last=""
for a in "$@"; do prev="$last"; last="$a"; done
printf "OggS" > "$last"
`), 0755))
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	tts := &tts.PiperTTS{
		BinaryPath:    fakePiper(t),
		ModelPath:     "model",
		OpusencBinary: "", // intentionally empty → uses "opusenc" from PATH
		AudioDir:      dir,
	}

	got, err := tts.Generate("default opusenc")
	require.NoError(t, err)
	full := filepath.Join(dir, got)
	_, err = os.Stat(full)
	require.NoError(t, err)
}
