package handlers_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// newTestHubWithClient starts a real Hub — the same type App.WSHub points at
// in production — and registers a single client in orgID, so a test can
// assert on exactly what a client in that org receives. Mirrors
// newTestHub/newTestClient in internal/websocket/websocket_test.go;
// duplicated here rather than imported because handlers_test is a different
// package and the websocket package already exports everything this needs
// (NewHub, NewClient, Register, GetClientCount, SendChan).
func newTestHubWithClient(t *testing.T, orgID uuid.UUID) (*websocket.Hub, *websocket.Client) {
	t.Helper()
	hub := websocket.NewHub(testutil.NopLogger())
	go hub.Run()

	client := websocket.NewClient(hub, nil, uuid.New(), orgID)
	hub.Register(client)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() != 1 {
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, 1, hub.GetClientCount(), "client failed to register on the test hub")

	return hub, client
}

// readBroadcast reads the next message off client's send channel and asserts
// its type. Fails the test if nothing arrives within 2s.
func readBroadcast(t *testing.T, client *websocket.Client, expectedType string) websocket.WSMessage {
	t.Helper()
	select {
	case data := <-client.SendChan():
		var msg websocket.WSMessage
		require.NoError(t, json.Unmarshal(data, &msg))
		assert.Equal(t, expectedType, msg.Type)
		return msg
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for a %q broadcast", expectedType)
		return websocket.WSMessage{}
	}
}

// assertNoBroadcast fails the test if any message arrives within the window.
// Short on purpose: it only runs after the handler call under test has
// already returned, so there is nothing legitimate left to wait for.
func assertNoBroadcast(t *testing.T, client *websocket.Client) {
	t.Helper()
	select {
	case data := <-client.SendChan():
		t.Fatalf("expected no broadcast, got: %s", string(data))
	case <-time.After(150 * time.Millisecond):
	}
}

// CreateOccurrence broadcasts occurrence_changed, with the persisted
// occurrence_id and stage_id — the two fields the Kanban board uses to
// decide between updating, moving between columns, or just incrementing a
// counter (see occurrences.go's occurrenceToResponse / the design spec's §2).
func TestOccurrences_CreateBroadcastsOccurrenceChanged(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Broadcast na criação",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var occ models.Occurrence
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&occ).Error)

	msg := readBroadcast(t, client, websocket.TypeOccurrenceChanged)
	payload, ok := msg.Payload.(map[string]any)
	require.True(t, ok, "payload deve ser um objeto")
	assert.Equal(t, occ.ID.String(), payload["id"], "occurrence_id do broadcast deve bater com o persistido")
	assert.Equal(t, occ.StageID.String(), payload["stage_id"], "stage_id do broadcast deve bater com o persistido")
}

// A falha de validação (assignee inexistente) barra a criação antes de
// qualquer persistência — e, por isso, não pode disparar broadcast nenhum.
func TestOccurrences_CreateFailureBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Não deve emitir nada",
		"assigned_user_id": uuid.New().String(),
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateOccurrence(req))
	require.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}
