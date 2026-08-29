package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// GATE 2 (positivo). Quem enxerga a conversa cria a ocorrência.
func TestOccurrences_CreateAllowedForVisibleContact(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Troca de produto",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateOccurrence(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var occ models.Occurrence
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&occ).Error)
	assert.NotEmpty(t, occ.ProtocolNumber, "o protocolo deve ter sido gerado")
	assert.NotEqual(t, "", occ.StageID.String())
}

// GATE 2 (negativo). Quem NÃO enxerga a conversa recebe 403 — e nenhuma linha
// é criada. Este é o teste que teria pego os quatro vazamentos anteriores.
func TestOccurrences_CreateDeniedForInvisibleContact(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	outsiderRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "occ-outsider",
		[]string{"chat:read", "chat:write"})
	outsider := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&outsiderRole.ID))
	enableStrictVisibility(t, app, org.ID)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", owner.ID).Error)

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Não deveria abrir",
	})
	testutil.SetAuthContext(req, org.ID, outsider.ID)

	require.NoError(t, app.CreateOccurrence(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))

	var count int64
	app.DB.Model(&models.Occurrence{}).Where("contact_id = ?", contact.ID).Count(&count)
	assert.EqualValues(t, 0, count, "nenhuma ocorrência pode ter sido criada")
}

// GATE 2 (negativo, listagem). A lista não mostra ocorrência de contato invisível.
func TestOccurrences_ListHidesInvisibleContacts(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	outsiderRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "occ-outsider",
		[]string{"chat:read", "chat:write"})
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

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, outsider.ID)
	require.NoError(t, app.ListOccurrences(req))

	body := string(testutil.GetResponseBody(req))
	assert.NotContains(t, body, occ.ProtocolNumber,
		"a ocorrência de um contato invisível não pode aparecer na lista")
}

// GATE 3. A exceção do responsável: quem tem a ocorrência atribuída a enxerga,
// mesmo sem enxergar o contato. Sem isso dá para atribuir um caso que a pessoa
// não consegue abrir.
func TestOccurrences_AssigneeSeesOwnEvenWhenContactInvisible(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	assigneeRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "occ-assignee",
		[]string{"chat:read", "chat:write"})
	assignee := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&assigneeRole.ID))
	enableStrictVisibility(t, app, org.ID)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", owner.ID).Error)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Atribuída a mim",
		StageID: stage.ID, OpenedByUserID: owner.ID, AssignedUserID: &assignee.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, assignee.ID)
	require.NoError(t, app.ListOccurrences(req))

	body := string(testutil.GetResponseBody(req))
	assert.Contains(t, body, occ.ProtocolNumber,
		"o responsável precisa enxergar a própria ocorrência")
}

// assigned_user_id is client input: a UUID that names nobody must not become
// an orphan reference on the row. 400, and no occurrence created.
func TestOccurrences_CreateWithUnknownAssigneeIsRejected(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	unknownUserID := uuid.New().String()
	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Atribuir a ninguém",
		"assigned_user_id": unknownUserID,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateOccurrence(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	var count int64
	app.DB.Model(&models.Occurrence{}).Where("contact_id = ?", contact.ID).Count(&count)
	assert.EqualValues(t, 0, count, "nenhuma ocorrência pode ter sido criada")
}

// assigned_user_id de um usuário de OUTRA organização não pode ser aceito —
// não é uma questão de permissão do chamador, é dado inválido. 400, e
// nenhuma ocorrência criada.
func TestOccurrences_CreateWithForeignOrgAssigneeIsRejected(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	otherOrg := testutil.CreateTestOrganization(t, app.DB)
	otherAdminRole := testutil.CreateAdminRole(t, app.DB, otherOrg.ID)
	foreignUser := testutil.CreateTestUser(t, app.DB, otherOrg.ID, testutil.WithRoleID(&otherAdminRole.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Atribuir a usuário de outra org",
		"assigned_user_id": foreignUser.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateOccurrence(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	var count int64
	app.DB.Model(&models.Occurrence{}).Where("contact_id = ?", contact.ID).Count(&count)
	assert.EqualValues(t, 0, count, "nenhuma ocorrência pode ter sido criada")
}

// Task 9B, requirement 1: an occurrence must never be born orphaned. When the
// body doesn't bring assigned_user_id, the creator becomes the assignee.
func TestOccurrences_CreateDefaultsAssigneeToCreator(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Sem responsável informado",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateOccurrence(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var occ models.Occurrence
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&occ).Error)
	require.NotNil(t, occ.AssignedUserID, "a ocorrência não pode nascer órfã")
	assert.Equal(t, user.ID, *occ.AssignedUserID, "quem abriu deve ser o responsável padrão")
}

// Task 9B, requirement 1: an explicit assigned_user_id in the body still wins
// over the creator default.
func TestOccurrences_CreateWithExplicitAssigneeOverridesDefault(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	creator := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	otherAgentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	assignee := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&otherAgentRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Responsável explícito",
		"assigned_user_id": assignee.ID.String(),
	})
	testutil.SetAuthContext(req, org.ID, creator.ID)

	require.NoError(t, app.CreateOccurrence(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var occ models.Occurrence
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&occ).Error)
	require.NotNil(t, occ.AssignedUserID)
	assert.Equal(t, assignee.ID, *occ.AssignedUserID, "o responsável enviado deve prevalecer sobre o padrão")
}

// Sanity: the envelope actually round-trips a valid JSON body (guards against
// a handler that returns 200 with an empty/broken payload).
func TestOccurrences_ListEnvelopeShape(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)
	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Teste",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.ListOccurrences(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Occurrences []struct {
				ProtocolNumber string `json:"protocol_number"`
				ContactID      string `json:"contact_id"`
			} `json:"occurrences"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data.Occurrences, 1)
	assert.Equal(t, occ.ProtocolNumber, resp.Data.Occurrences[0].ProtocolNumber)
}
