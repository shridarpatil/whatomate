package handlers_test

import (
	"encoding/json"
	"testing"
	"time"

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
		[]string{"chat:read", "chat:write", "occurrences:read", "occurrences:write"})
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
		[]string{"chat:read", "chat:write", "occurrences:read"})
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

// A borda é inclusiva: fechada exatamente no instante do corte entra.
// O relógio é fixo no teste para o caso de borda não ficar intermitente.
func TestOccurrences_ClosedSinceIncludesBoundary(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	cut := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Na borda",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))
	require.NoError(t, app.DB.Model(&models.Occurrence{}).
		Where("id = ?", occ.ID).Update("closed_at", cut).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", cut.Format(time.RFC3339))
	require.NoError(t, app.ListOccurrences(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Occurrences []struct {
				ID string `json:"id"`
			} `json:"occurrences"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data.Occurrences, 1)
	assert.Equal(t, occ.ID.String(), resp.Data.Occurrences[0].ID)
}

// Fechada antes do corte fica de fora.
func TestOccurrences_ClosedSinceExcludesOlder(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	cut := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Velha",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))
	require.NoError(t, app.DB.Model(&models.Occurrence{}).
		Where("id = ?", occ.ID).Update("closed_at", cut.Add(-time.Second)).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", cut.Format(time.RFC3339))
	require.NoError(t, app.ListOccurrences(req))

	var resp struct {
		Data struct {
			Occurrences []struct {
				ID string `json:"id"`
			} `json:"occurrences"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Empty(t, resp.Data.Occurrences)
}

// Aberta (closed_at NULL) nunca entra, por mais antigo que seja o corte.
func TestOccurrences_ClosedSinceExcludesOpen(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Aberta",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", "2000-01-01T00:00:00Z")
	require.NoError(t, app.ListOccurrences(req))

	var resp struct {
		Data struct {
			Occurrences []struct {
				ID string `json:"id"`
			} `json:"occurrences"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Empty(t, resp.Data.Occurrences)
}

// Valor impossível de interpretar é recusado, não ignorado: ignorar
// transformaria a coluna de fechadas na lista inteira.
func TestOccurrences_ClosedSinceRejectsInvalidValue(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", "ontem")
	require.NoError(t, app.ListOccurrences(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// closed_since combina com stage_id: cada coluna do quadro é uma etapa.
func TestOccurrences_ClosedSinceCombinesWithStageFilter(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var stages []models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ?", org.ID).
		Order("position ASC").Find(&stages).Error)
	require.GreaterOrEqual(t, len(stages), 2)
	wanted, other := stages[0], stages[1]

	cut := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	// Duas ocorrências na etapa "wanted": uma dentro da janela (deve aparecer)
	// e uma fora dela (deve ser excluída por closed_since). Uma terceira, na
	// etapa "other", fica dentro da janela mas deve ser excluída por stage_id.
	// Só assim o teste falha se qualquer uma das duas metades do filtro sumir.
	type seed struct {
		stage    models.OccurrenceStage
		closedAt time.Time
		title    string
	}
	inWindow := models.Occurrence{}
	for _, s := range []seed{
		{wanted, cut.Add(time.Hour), "Dentro da janela, etapa certa"},
		{wanted, cut.Add(-time.Hour), "Fora da janela, etapa certa"},
		{other, cut.Add(time.Hour), "Dentro da janela, etapa errada"},
	} {
		occ := models.Occurrence{
			OrganizationID: org.ID, ContactID: contact.ID, Title: s.title,
			StageID: s.stage.ID, OpenedByUserID: user.ID,
		}
		require.NoError(t, app.CreateOccurrenceForTest(&occ))
		require.NoError(t, app.DB.Model(&models.Occurrence{}).
			Where("id = ?", occ.ID).Update("closed_at", s.closedAt).Error)
		if s.stage.ID == wanted.ID && s.closedAt.Equal(cut.Add(time.Hour)) {
			inWindow = occ
		}
	}

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", cut.Format(time.RFC3339))
	testutil.SetQueryParam(req, "stage_id", wanted.ID.String())
	require.NoError(t, app.ListOccurrences(req))

	var resp struct {
		Data struct {
			Occurrences []struct {
				ID      string `json:"id"`
				StageID string `json:"stage_id"`
			} `json:"occurrences"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data.Occurrences, 1)
	assert.Equal(t, inWindow.ID.String(), resp.Data.Occurrences[0].ID,
		"deve ser a ocorrência fechada dentro da janela, na etapa certa")
	assert.Equal(t, wanted.ID.String(), resp.Data.Occurrences[0].StageID)
}
