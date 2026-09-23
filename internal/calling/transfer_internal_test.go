package calling

import (
	"testing"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
)

// A browser hanging up sends a DTLS CloseNotify, and Pion responds by closing
// the PeerConnection itself (peerconnection.go internalOnCloseHandler), so the
// state we observe is Closed — never Disconnected or Failed. Treating Closed as
// "still up" leaves the caller's leg running until they hang up themselves.
func TestPeerGone_CoversCleanBrowserHangup(t *testing.T) {
	assert.True(t, peerGone(webrtc.PeerConnectionStateClosed), "clean hangup must tear the call down")
	assert.True(t, peerGone(webrtc.PeerConnectionStateFailed))
	assert.True(t, peerGone(webrtc.PeerConnectionStateDisconnected))

	assert.False(t, peerGone(webrtc.PeerConnectionStateNew))
	assert.False(t, peerGone(webrtc.PeerConnectionStateConnecting))
	assert.False(t, peerGone(webrtc.PeerConnectionStateConnected))
}
