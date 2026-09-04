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
	// CreateOccurrence defaults the assignee to its creator, so this must not
	// be silently empty (Finding 2: the broadcast used to skip the
	// AssignedUser preload that occurrenceToResponse needs).
	assert.NotEmpty(t, payload["assigned_user_name"], "assigned_user_name nao deve vir vazio no broadcast")
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
	// Finding 3: the stage-change event broadcast used to leave
	// created_by_name empty.
	assert.NotEmpty(t, eventPayload["created_by_name"], "created_by_name nao deve vir vazio no broadcast")

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

func TestOccurrences_CreateEventBroadcastsOccurrenceEventCreated(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Nota manual",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "POST", occ.ID, map[string]any{"content": "Cliente ligou de volta"})
	require.NoError(t, app.CreateOccurrenceEvent(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	msg := readBroadcast(t, client, websocket.TypeOccurrenceEventCreated)
	payload, ok := msg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), payload["occurrence_id"])
	assert.Equal(t, string(models.OccurrenceEventNote), payload["type"])
	assert.Equal(t, "Cliente ligou de volta", payload["content"])
	// Finding 3: CreateOccurrenceEvent's broadcast used to leave
	// created_by_name empty.
	assert.NotEmpty(t, payload["created_by_name"], "created_by_name nao deve vir vazio no broadcast")
}

// content vazio barra a criação antes de qualquer escrita.
func TestOccurrences_CreateEventFailureBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Conteúdo vazio",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "POST", occ.ID, map[string]any{"content": ""})
	require.NoError(t, app.CreateOccurrenceEvent(req))
	require.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}

// The whole point of switching off BroadcastToOrg: a client with no
// relationship to the contact must not receive the occurrence's data.
func TestOccurrences_CreateBroadcastDeniesUnauthorizedViewer(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	creator := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	hub := websocket.NewHub(testutil.NopLogger())
	go hub.Run()
	// Only the creator (who becomes the default assignee) is "authorized".
	hub.SetConversationAuthorizer(func(userID, orgID, contactID uuid.UUID) bool {
		return userID == creator.ID
	})
	authorized := websocket.NewClient(hub, nil, creator.ID, org.ID)
	stranger := websocket.NewClient(hub, nil, uuid.New(), org.ID)
	hub.Register(authorized)
	hub.Register(stranger)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() != 2 {
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, 2, hub.GetClientCount())
	app.WSHub = hub

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Só autorizado recebe",
	})
	testutil.SetAuthContext(req, org.ID, creator.ID)
	require.NoError(t, app.CreateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	readBroadcast(t, authorized, websocket.TypeOccurrenceChanged)
	assertNoBroadcast(t, stranger)
}

// The assignee exception: a case assigned to you is yours to see even when
// your general visibility doesn't cover the contact — loadAuthorizedOccurrence
// already grants this on the REST path; the broadcast must grant it too.
func TestOccurrences_CreateBroadcastReachesAssigneeDespiteNoGeneralAuthorization(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	creator := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	assignee := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	hub := websocket.NewHub(testutil.NopLogger())
	go hub.Run()
	// Nobody passes the general check -- the assignee fan-out is the only
	// thing that can deliver this broadcast.
	hub.SetConversationAuthorizer(func(userID, orgID, contactID uuid.UUID) bool { return false })
	assigneeClient := websocket.NewClient(hub, nil, assignee.ID, org.ID)
	hub.Register(assigneeClient)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() != 1 {
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, 1, hub.GetClientCount())
	app.WSHub = hub

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Responsável recebe mesmo sem autorização geral",
		"assigned_user_id": assignee.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, creator.ID)
	require.NoError(t, app.CreateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	readBroadcast(t, assigneeClient, websocket.TypeOccurrenceChanged)
}

// The prior fix (authorization gate + assignee fan-out) must not deliver the
// SAME broadcast twice to a client that is both an authorized viewer AND the
// assignee — occurrence_changed's board handler increments a counter for an
// unseen occurrence, so a duplicate is a real, visible bug (double-counting),
// not a cosmetic one.
func TestOccurrences_CreateBroadcastDeliversExactlyOnceWhenAssigneeIsAlsoAuthorized(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	creator := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	hub := websocket.NewHub(testutil.NopLogger())
	go hub.Run()
	// The creator (who becomes the default assignee) is ALSO generally
	// authorized -- this is the case that used to double-deliver.
	hub.SetConversationAuthorizer(func(userID, orgID, contactID uuid.UUID) bool {
		return userID == creator.ID
	})
	client := websocket.NewClient(hub, nil, creator.ID, org.ID)
	hub.Register(client)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() != 1 {
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, 1, hub.GetClientCount())
	app.WSHub = hub

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Autorizado e responsável ao mesmo tempo",
	})
	testutil.SetAuthContext(req, org.ID, creator.ID)
	require.NoError(t, app.CreateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	readBroadcast(t, client, websocket.TypeOccurrenceChanged)
	// If delivery happened twice, this second message is exactly what would
	// arrive -- assertNoBroadcast fails the test if it does.
	assertNoBroadcast(t, client)
}
