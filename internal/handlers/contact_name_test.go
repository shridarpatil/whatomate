package handlers_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestUpdateContactName_RequiresPermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "sem-renomear",
		[]string{"chat:read", "chat:write", "contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "Nome Novo"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestUpdateContactName_RenamesForPermittedUser(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "Maria da Silva"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var reloaded models.Contact
	require.NoError(t, app.DB.First(&reloaded, "id = ?", contact.ID).Error)
	assert.Equal(t, "Maria da Silva", reloaded.ProfileName)
}

// Nenhum outro campo do contato pode mudar. Um endpoint estreito que mexe
// em mais de um campo deixa de ser estreito.
func TestUpdateContactName_TouchesNothingElse(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	var before models.Contact
	require.NoError(t, app.DB.First(&before, "id = ?", contact.ID).Error)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "Outro Nome"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.UpdateContactName(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var after models.Contact
	require.NoError(t, app.DB.First(&after, "id = ?", contact.ID).Error)
	assert.Equal(t, before.PhoneNumber, after.PhoneNumber)
	assert.Equal(t, before.AssignedUserID, after.AssignedUserID)
	assert.Equal(t, before.TeamID, after.TeamID)
	assert.Equal(t, before.WhatsAppAccount, after.WhatsAppAccount)
}

// A permissão nova NÃO pode furar a visibilidade de conversa que a Fase 1
// construiu: renomear contato que o usuário não enxerga seria acesso lateral.
func TestUpdateContactName_DeniedForInvisibleContact(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	outsiderRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "renomeador-de-fora",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	outsider := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&outsiderRole.ID))
	enableStrictVisibility(t, app, org.ID)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", owner.ID).Error)

	var before models.Contact
	require.NoError(t, app.DB.First(&before, "id = ?", contact.ID).Error)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "Nao Deveria Renomear"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, outsider.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))

	var after models.Contact
	require.NoError(t, app.DB.First(&after, "id = ?", contact.ID).Error)
	assert.Equal(t, before.ProfileName, after.ProfileName, "o nome nao pode ter mudado")
}

// Guarda contra perda de dados: com mascara ligada, o nome exibido pode ser
// ****1234. Salvar isso gravaria a mascara por cima do numero real.
func TestUpdateContactName_RejectsMaskedValue(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "*******1234"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestUpdateContactName_RejectsEmpty(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "   "})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}
