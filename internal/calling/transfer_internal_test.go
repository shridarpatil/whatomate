package calling

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
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

// A click-to-call call belongs to the agent whose button the customer tapped.
// When that agent doesn't pick up, the call ends — offering it to the rest of
// the team would put a stranger on a line the customer asked a specific person
// for.
func TestHandleAssignedAgentTimeout_StickyCallEndsWithoutTeamFallback(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ac := &authCapture{}
	srv := ac.server(t)

	hub := websocket.NewHub(testutil.NopLogger())
	go hub.Run()

	m := &Manager{
		sessions: make(map[string]*CallSession),
		log:      testutil.NopLogger(),
		whatsapp: whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL),
		db:       db,
		wsHub:    hub,
	}

	orgID := uuid.New()
	agentID := uuid.New()
	teamID := uuid.New()
	session := &CallSession{
		ID:             "wacid." + uuid.NewString(),
		OrganizationID: orgID,
		AccountName:    "acct",
		WAAccount: &whatsapp.Account{
			PhoneID:     "phone-1",
			APIVersion:  "v18.0",
			AccessToken: "plaintext-access-token",
		},
		CallLogID:      uuid.New(),
		StickyAgentID:  &agentID,
		TransferID:     uuid.New(),
		TransferStatus: models.CallTransferStatusWaiting,
	}
	transfer := models.CallTransfer{BaseModel: models.BaseModel{ID: session.TransferID}, OrganizationID: orgID}

	// A team is targeted on purpose: even then, a sticky call must not roll over.
	m.handleAssignedAgentTimeout(session, transfer, orgCallingSettings{}, &teamID, map[string]any{})

	session.mu.Lock()
	status := session.TransferStatus
	session.mu.Unlock()
	assert.Equal(t, models.CallTransferStatusNoAnswer, status)

	ac.mu.Lock()
	defer ac.mu.Unlock()
	assert.Equal(t, 1, ac.hits, "the call must be terminated instead of rolling over to the team")
}

// Non-sticky calls keep today's behaviour: the agent who got first ring times
// out, and the call is offered to everyone else.
func TestHandleAssignedAgentTimeout_NonStickyCallFallsBackToOrg(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ac := &authCapture{}
	srv := ac.server(t)

	hub := websocket.NewHub(testutil.NopLogger())
	go hub.Run()

	m := &Manager{
		sessions: make(map[string]*CallSession),
		log:      testutil.NopLogger(),
		whatsapp: whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL),
		db:       db,
		wsHub:    hub,
	}

	orgID := uuid.New()
	bystander := websocket.NewClient(hub, nil, uuid.New(), orgID)
	hub.Register(bystander)

	session := &CallSession{
		ID:             "wacid." + uuid.NewString(),
		OrganizationID: orgID,
		AccountName:    "acct",
		CallLogID:      uuid.New(),
		TransferID:     uuid.New(),
		TransferStatus: models.CallTransferStatusWaiting,
	}
	transfer := models.CallTransfer{BaseModel: models.BaseModel{ID: session.TransferID}, OrganizationID: orgID}

	done := make(chan struct{})
	go func() {
		defer close(done)
		// No team targeted: the fallback is an org-wide broadcast.
		m.handleAssignedAgentTimeout(session, transfer, orgCallingSettings{TransferTimeoutSecs: 1}, nil, map[string]any{})
	}()

	select {
	case raw := <-bystander.SendChan():
		assert.Contains(t, string(raw), websocket.TypeCallTransferWaiting,
			"the rest of the org must be offered the call")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the org-wide fallback broadcast")
	}

	ac.mu.Lock()
	hits := ac.hits
	ac.mu.Unlock()
	assert.Zero(t, hits, "a non-sticky call must not be terminated on the first timeout")

	<-done
}

// A click-to-call call is never offered to anyone else, so it gets its own ring
// window instead of the rotation's "move on to the next agent" slot.
func TestFirstRingWindow_StickyCallUsesItsOwnSetting(t *testing.T) {
	m := &Manager{
		log:    testutil.NopLogger(),
		config: &config.CallingConfig{PerAgentTimeoutSecs: 15, StickyRingSecs: 30},
	}

	agentID := uuid.New()
	sticky := &CallSession{ID: "wacid.sticky", StickyAgentID: &agentID}
	organic := &CallSession{ID: "wacid.organic"}

	assert.Equal(t, 30*time.Second, m.firstRingWindow(sticky))
	assert.Equal(t, 15*time.Second, m.firstRingWindow(organic))
}

func TestFirstRingWindow_FallsBackWhenUnset(t *testing.T) {
	m := &Manager{log: testutil.NopLogger(), config: &config.CallingConfig{}}

	agentID := uuid.New()
	assert.Equal(t, 15*time.Second, m.firstRingWindow(&CallSession{StickyAgentID: &agentID}))
	assert.Equal(t, 15*time.Second, m.firstRingWindow(&CallSession{}))
}
