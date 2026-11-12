package tts_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/shridarpatil/whatomate/internal/tts"
	"github.com/stretchr/testify/require"
)

func TestPiperConcurrentIdenticalRequestsPublishOneReusableResult(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WHATOMATE_TTS_TEST_GATE", t.TempDir())
	// Every request reaches both converters before any final output is published.
	// After all encoders finish writing, the old shared temporary name can be
	// renamed successfully by only one request. No scheduling delay is assumed.
	barrier := `
barrier() {
 phase="$1"
 : > "$WHATOMATE_TTS_TEST_GATE/$phase-$$"
 while :; do
  ready=0
  for marker in "$WHATOMATE_TTS_TEST_GATE/$phase-"*; do
   if [ -f "$marker" ]; then ready=$((ready + 1)); fi
  done
  if [ "$ready" -eq 8 ]; then return; fi
  sleep 0.01
 done
}
`
	p := &tts.PiperTTS{
		AudioDir:  dir,
		ModelPath: "model",
		BinaryPath: fakeBin(t, "piper", barrier+`
while [ "$#" -gt 0 ]; do
 case "$1" in --output_file) out="$2"; shift 2;; *) shift;; esac
done
cat > /dev/null
barrier piper
printf RIFF > "$out"
`),
		OpusencBinary: fakeBin(t, "opusenc", barrier+`
prev=""; last=""
for argument in "$@"; do prev="$last"; last="$argument"; done
barrier encoder
test -f "$prev" || exit 3
printf OggS > "$last"
barrier encoded
`),
	}
	start := make(chan struct{})
	results := make(chan error, 8)
	names := make(chan string, 8)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			name, err := p.Generate("Concurrent example")
			results <- err
			names <- name
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(names)
	for err := range results {
		require.NoError(t, err)
	}
	var expected string
	for name := range names {
		if expected == "" {
			expected = name
		}
		require.Equal(t, expected, name)
	}
	data, err := os.ReadFile(filepath.Join(dir, expected))
	require.NoError(t, err)
	require.Equal(t, "OggS", string(data))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "temporary conversion paths must be removed")
}
