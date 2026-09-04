package handlers_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// authedJSON builds a request with a JSON body, the given method, auth context
// and path id — the local equivalent of the not-yet-existing
// testutil.NewAuthedRequestWithBody, assembled from the primitives
// occurrences_test.go already uses (NewJSONRequest + SetAuthContext).
func authedJSON(t *testing.T, app *handlers.App, orgID, userID uuid.UUID, method string, occID uuid.UUID, body map[string]any) *fastglue.Request {
	t.Helper()
	req := testutil.NewJSONRequest(t, body)
	req.RequestCtx.Request.Header.SetMethod(method)
	testutil.SetAuthContext(req, orgID, userID)
	testutil.SetPathParam(req, "id", occID.String())
	return req
}

// authedGET is the GET equivalent of authedJSON.
func authedGET(t *testing.T, app *handlers.App, orgID, userID, occID uuid.UUID) *fastglue.Request {
	t.Helper()
	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, orgID, userID)
	testutil.SetPathParam(req, "id", occID.String())
	return req
}

// GATE 6. Entrar numa etapa de fechamento preenche closed_at; voltar limpa.
func TestOccurrences_CloseAndReopen(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", user.ID).Error)

	initial, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	var closing models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, true).
		First(&closing).Error)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Fecha e reabre",
		StageID: initial.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	// Fecha
	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": closing.ID.String()})
	require.NoError(t, app.ChangeOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var closed models.Occurrence
	require.NoError(t, app.DB.First(&closed, "id = ?", occ.ID).Error)
	require.NotNil(t, closed.ClosedAt, "closed_at deve ser preenchido na etapa de fechamento")

	// Reabre
	req2 := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": initial.ID.String()})
	require.NoError(t, app.ChangeOccurrenceStage(req2))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))

	var reopened models.Occurrence
	require.NoError(t, app.DB.First(&reopened, "id = ?", occ.ID).Error)
	assert.Nil(t, reopened.ClosedAt, "reabrir deve limpar closed_at")

	// Ambas as transições ficaram registradas
	var changes int64
	app.DB.Model(&models.OccurrenceEvent{}).
		Where("occurrence_id = ? AND type = ?", occ.ID, models.OccurrenceEventStageChange).
		Count(&changes)
	assert.EqualValues(t, 2, changes, "cada mudança de etapa vira um evento")
}

// Reenviar a etapa em que a ocorrência já está deve ser um no-op: nada de
// evento "X → X" espúrio, nada de re-carimbar closed_at, nada de evento
// "closed" duplicado.
func TestOccurrences_SameStageIsNoOp(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", user.ID).Error)

	initial, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	var closing models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, true).
		First(&closing).Error)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Reenvio da mesma etapa",
		StageID: initial.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	// Fecha.
	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": closing.ID.String()})
	require.NoError(t, app.ChangeOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var closed models.Occurrence
	require.NoError(t, app.DB.First(&closed, "id = ?", occ.ID).Error)
	require.NotNil(t, closed.ClosedAt)

	var changesBefore, closedEventsBefore int64
	app.DB.Model(&models.OccurrenceEvent{}).
		Where("occurrence_id = ? AND type = ?", occ.ID, models.OccurrenceEventStageChange).
		Count(&changesBefore)
	app.DB.Model(&models.OccurrenceEvent{}).
		Where("occurrence_id = ? AND type = ?", occ.ID, models.OccurrenceEventClosed).
		Count(&closedEventsBefore)

	// Garante que um novo time.Now() seria distinguível do primeiro closed_at.
	time.Sleep(10 * time.Millisecond)

	// Reenvia a mesma etapa de fechamento.
	req2 := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": closing.ID.String()})
	require.NoError(t, app.ChangeOccurrenceStage(req2))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))

	var afterNoop models.Occurrence
	require.NoError(t, app.DB.First(&afterNoop, "id = ?", occ.ID).Error)
	require.NotNil(t, afterNoop.ClosedAt)
	assert.True(t, closed.ClosedAt.Equal(*afterNoop.ClosedAt),
		"reenviar a mesma etapa não deve re-carimbar closed_at: antes=%v depois=%v",
		closed.ClosedAt, afterNoop.ClosedAt)

	var changesAfter, closedEventsAfter int64
	app.DB.Model(&models.OccurrenceEvent{}).
		Where("occurrence_id = ? AND type = ?", occ.ID, models.OccurrenceEventStageChange).
		Count(&changesAfter)
	app.DB.Model(&models.OccurrenceEvent{}).
		Where("occurrence_id = ? AND type = ?", occ.ID, models.OccurrenceEventClosed).
		Count(&closedEventsAfter)

	assert.Equal(t, changesBefore, changesAfter, "reenviar a mesma etapa não deve gravar stage_change")
	assert.Equal(t, closedEventsBefore, closedEventsAfter, "reenviar a mesma etapa não deve duplicar o evento closed")
}

// GATE 4. Cada endpoint carrega o próprio gate. Um só desprotegido já vaza.
// "outsider" mirrors occ-outsider in occurrences_test.go: has chat:read,
// chat:write, occurrences:read and occurrences:write, but no contacts:read
// and no conversations:view_all/view_team — so requireAuth lets them in, and
// only loadAuthorizedOccurrence's conversation gate can stop them.
func TestOccurrences_EveryEndpointIsAuthorized(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	outsiderRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "occ-outsider",
		[]string{"chat:read", "chat:write", "occurrences:read", "occurrences:write"})
	outsider := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&outsiderRole.ID))
	enableStrictVisibility(t, app, org.ID)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", owner.ID).Error)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Privada",
		StageID: stage.ID, OpenedByUserID: owner.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	cases := []struct {
		name    string
		handler func(*fastglue.Request) error
		method  string
		body    map[string]any
		pathID  uuid.UUID // most handlers take the occurrence id; ListContactOccurrences takes the contact id
	}{
		{"detalhe", app.GetOccurrence, "GET", nil, occ.ID},
		{"atualizar", app.UpdateOccurrence, "PUT", map[string]any{"title": "x"}, occ.ID},
		{"mudar etapa", app.ChangeOccurrenceStage, "PUT", map[string]any{"stage_id": stage.ID.String()}, occ.ID},
		{"listar eventos", app.ListOccurrenceEvents, "GET", nil, occ.ID},
		{"criar evento", app.CreateOccurrenceEvent, "POST", map[string]any{"content": "nota"}, occ.ID},
		{"enviar protocolo", app.SendOccurrenceProtocol, "POST", nil, occ.ID},
		// Not scoped by visibleOccurrences like ListOccurrences — relies entirely
		// on canViewConversation, so it deserves the same authorization check.
		{"listar ocorrências do contato", app.ListContactOccurrences, "GET", nil, contact.ID},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *fastglue.Request
			if tc.method == "GET" {
				req = authedGET(t, app, org.ID, outsider.ID, tc.pathID)
			} else {
				req = authedJSON(t, app, org.ID, outsider.ID, tc.method, tc.pathID, tc.body)
			}

			require.NoError(t, tc.handler(req))
			assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req),
				"%s deve recusar quem não enxerga o contato", tc.name)
		})
	}
}

// Nota adicionada aparece na timeline.
func TestOccurrences_NoteAppearsOnTimeline(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Com nota",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := authedJSON(t, app, org.ID, user.ID, "POST", occ.ID, map[string]any{"content": "Aguardando NF do fornecedor"})
	require.NoError(t, app.CreateOccurrenceEvent(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	list := authedGET(t, app, org.ID, user.ID, occ.ID)
	require.NoError(t, app.ListOccurrenceEvents(list))

	assert.Contains(t, string(testutil.GetResponseBody(list)), "Aguardando NF do fornecedor")
}

// O update sem assigned_user_id preserva o responsável; com "" remove.
func TestOccurrences_UpdateAssigneeAbsentVsEmpty(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	assignee := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Original",
		StageID: stage.ID, OpenedByUserID: user.ID, AssignedUserID: &assignee.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	// PUT without assigned_user_id: responsável must be preserved.
	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"title": "Atualizado"})
	require.NoError(t, app.UpdateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var afterAbsent models.Occurrence
	require.NoError(t, app.DB.First(&afterAbsent, "id = ?", occ.ID).Error)
	require.NotNil(t, afterAbsent.AssignedUserID, "PUT sem assigned_user_id não deve apagar o responsável")
	assert.Equal(t, assignee.ID, *afterAbsent.AssignedUserID)

	// PUT with assigned_user_id: "" must clear it.
	req2 := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{
		"title": "Atualizado", "assigned_user_id": "",
	})
	require.NoError(t, app.UpdateOccurrence(req2))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))

	var afterEmpty models.Occurrence
	require.NoError(t, app.DB.First(&afterEmpty, "id = ?", occ.ID).Error)
	assert.Nil(t, afterEmpty.AssignedUserID, `PUT com assigned_user_id:"" deve remover o responsável`)
}

// PUT sem description preserva o valor existente; PUT com description:""
// apaga; PUT com description preenchido grava o novo valor. Mesmo padrão de
// TestOccurrences_UpdateAssigneeAbsentVsEmpty.
func TestOccurrences_UpdateDescriptionAbsentVsEmpty(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Original",
		Description: "Descrição original", StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	// PUT without description: it must be preserved.
	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"title": "Atualizado"})
	require.NoError(t, app.UpdateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var afterAbsent models.Occurrence
	require.NoError(t, app.DB.First(&afterAbsent, "id = ?", occ.ID).Error)
	assert.Equal(t, "Descrição original", afterAbsent.Description,
		"PUT sem description não deve apagar a descrição existente")

	// PUT with description: "": it must be cleared.
	req2 := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{
		"title": "Atualizado", "description": "",
	})
	require.NoError(t, app.UpdateOccurrence(req2))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))

	var afterEmpty models.Occurrence
	require.NoError(t, app.DB.First(&afterEmpty, "id = ?", occ.ID).Error)
	assert.Empty(t, afterEmpty.Description, `PUT com description:"" deve apagar a descrição`)

	// PUT with description filled: regression check — a client that always
	// sends the field (any form that loads the record before editing) must
	// see no change in behavior.
	req3 := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{
		"title": "Atualizado", "description": "Nova descrição",
	})
	require.NoError(t, app.UpdateOccurrence(req3))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req3))

	var afterFilled models.Occurrence
	require.NoError(t, app.DB.First(&afterFilled, "id = ?", occ.ID).Error)
	assert.Equal(t, "Nova descrição", afterFilled.Description)
}
