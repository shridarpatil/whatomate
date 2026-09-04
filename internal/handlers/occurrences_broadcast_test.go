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

func TestOccurrences_UpdateBroadcastsOccurrenceChanged(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Original",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"title": "Atualizado"})
	require.NoError(t, app.UpdateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	msg := readBroadcast(t, client, websocket.TypeOccurrenceChanged)
	payload, ok := msg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), payload["id"])
	assert.Equal(t, "Atualizado", payload["title"])
}

// assigned_user_id inexistente barra o UPDATE antes de qualquer escrita.
func TestOccurrences_UpdateFailureBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Original",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{
		"title": "Não deve emitir nada", "assigned_user_id": uuid.New().String(),
	})
	require.NoError(t, app.UpdateOccurrence(req))
	require.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}

// ChangeOccurrenceStage dispara os dois eventos, de tipos diferentes — não
// um "evento de WebSocket" contado uma vez. A mudança de etapa grava a
// ocorrência e o evento automático de timeline como duas escritas distintas,
// e cada uma tem seu próprio broadcast (spec §2, correção registrada).
func TestOccurrences_ChangeStageBroadcastsBothEventTypes(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var stages []models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, false).
		Order("position ASC").Find(&stages).Error)
	require.GreaterOrEqual(t, len(stages), 2,
		"precisa de duas etapas nao-fechadas para o teste nao se misturar com o evento closed")
	from, to := stages[0], stages[1]

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Muda de etapa",
		StageID: from.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": to.ID.String()})
	require.NoError(t, app.ChangeOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	changedMsg := readBroadcast(t, client, websocket.TypeOccurrenceChanged)
	changedPayload, ok := changedMsg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), changedPayload["id"])
	assert.Equal(t, to.ID.String(), changedPayload["stage_id"])

	eventMsg := readBroadcast(t, client, websocket.TypeOccurrenceEventCreated)
	eventPayload, ok := eventMsg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), eventPayload["occurrence_id"])
	assert.Equal(t, string(models.OccurrenceEventStageChange), eventPayload["type"])

	// Só os dois — a etapa de destino não é de fechamento, então nenhum
	// evento "closed" (e portanto nenhum terceiro broadcast) entra em jogo.
	assertNoBroadcast(t, client)
}

// Entrar numa etapa de fechamento também dispara exatamente os dois
// broadcasts (occurrence_changed + stage_change) — o evento "closed" que a
// própria etapa grava não ganha eco em tempo real (spec §4, revisado duas
// vezes), então nenhum terceiro broadcast pode aparecer aqui.
func TestOccurrences_ChangeStageToClosingBroadcastsStageChangeOnly(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	initial, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	var closing models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, true).
		First(&closing).Error)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Fecha com broadcast",
		StageID: initial.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": closing.ID.String()})
	require.NoError(t, app.ChangeOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	changedMsg := readBroadcast(t, client, websocket.TypeOccurrenceChanged)
	changedPayload, ok := changedMsg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), changedPayload["id"])
	assert.Equal(t, closing.ID.String(), changedPayload["stage_id"])

	eventMsg := readBroadcast(t, client, websocket.TypeOccurrenceEventCreated)
	eventPayload, ok := eventMsg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), eventPayload["occurrence_id"])
	assert.Equal(t, string(models.OccurrenceEventStageChange), eventPayload["type"])

	// O evento "closed" é gravado (ver TestOccurrences_CloseAndReopen) mas não
	// tem broadcast próprio — nenhum terceiro broadcast pode chegar aqui.
	assertNoBroadcast(t, client)
}

// Reenviar a etapa atual é no-op: nenhum broadcast de nenhum tipo.
func TestOccurrences_ChangeStageSameStageBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Mesma etapa",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": stage.ID.String()})
	require.NoError(t, app.ChangeOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}

// stage_id inexistente barra a operação antes de qualquer escrita.
func TestOccurrences_ChangeStageFailureBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Falha",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": uuid.New().String()})
	require.NoError(t, app.ChangeOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}
