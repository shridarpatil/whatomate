package handlers_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestOccurrencePermissions_ListRequiresOccurrencesRead(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "sem-crm",
		[]string{"chat:read", "chat:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.ListOccurrences(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestOccurrencePermissions_ReadOnlyCannotCreate(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "leitura",
		[]string{"occurrences:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(),
		"title":      "Nao deve criar",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.CreateOccurrence(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	// A role "leitura" também não tem contacts:read nem conversations:view_all,
	// então um 403 sozinho não prova nada: podia vir do gate de visibilidade de
	// conversa em vez do requireAuth(ActionWrite). Fixa a origem pela mensagem.
	body := string(testutil.GetResponseBody(req))
	assert.Contains(t, body, "Insufficient permissions")
}

// Listar etapas é parte de USAR o CRM: o quadro não renderiza sem elas.
// Não pode exigir permissão administrativa.
func TestOccurrencePermissions_ListStagesNeedsOnlyOccurrencesRead(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"occurrences:read", "occurrences:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.ListOccurrenceStages(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

// O caso central da spec: quem usa o CRM não administra o funil.
// É caracterização, não regressão — já passa hoje, porque as mutações de
// etapa exigem settings.general:write. Existe para travar a separação
// depois da troca, e fica vermelho se alguém ligar administração de funil
// a occurrences:write.
func TestOccurrencePermissions_CRMUserCannotAdministerStages(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"occurrences:read", "occurrences:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":  "Etapa proibida",
		"color": "#000000",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.CreateOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestOccurrencePermissions_StageAdminCanCreateStage(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "gestor-funil",
		[]string{"occurrences:read", "occurrences.stages:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":  "Etapa permitida",
		"color": "#123456",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.CreateOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

// Apagar exige a ação de exclusão, não a de escrita.
func TestOccurrencePermissions_StageWriteCannotDelete(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var stage models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, false).
		Where("is_initial = ?", false).First(&stage).Error)

	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "so-escrita",
		[]string{"occurrences:read", "occurrences.stages:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewRequest(t)
	testutil.SetPathParam(req, "id", stage.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.DeleteOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestOccurrencePermissions_SendProtocolRequiresOccurrencesWrite(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "leitura",
		[]string{"occurrences:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.SendOccurrenceProtocol(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
	// Same shape as ReadOnlyCannotCreate above: pin the denial to requireAuth,
	// not the conversation-visibility gate this role would also fail.
	body := string(testutil.GetResponseBody(req))
	assert.Contains(t, body, "Insufficient permissions")
}
