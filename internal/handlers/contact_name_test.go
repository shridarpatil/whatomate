package handlers_test

import (
	"strings"
	"testing"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/utils"
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

// A masked-value rule estreita demais recusaria nomes reais com asterisco,
// como um rating de estrelas em nome de estabelecimento.
func TestUpdateContactName_AllowsStarInRealName(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "Ótica 5*"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var reloaded models.Contact
	require.NoError(t, app.DB.First(&reloaded, "id = ?", contact.ID).Error)
	assert.Equal(t, "Ótica 5*", reloaded.ProfileName)
}

// Guarda o caso "salvo sem editar": com máscara ligada, o contato mostra
// ****1234 na tela. Se o usuário mandar de volta exatamente esse valor, é o
// blind-save que perde o número real — precisa ser recusado mesmo sem bater
// no formato genérico de máscara.
func TestUpdateContactName_RejectsExactMaskedValue(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	org.Settings = models.JSONB{"mask_phone_numbers": true}
	require.NoError(t, app.DB.Save(org).Error)

	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	// Digits-only phone number so LooksLikePhoneNumber is deterministically
	// true — the default fixture's phone number can carry hex letters from
	// its uniqueness suffix and would flakily dodge the mask.
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithPhoneNumber("+5511987654321"))
	require.NoError(t, app.DB.Model(contact).Update("profile_name", contact.PhoneNumber).Error)
	masked := utils.MaskIfPhoneNumber(contact.PhoneNumber)
	require.NotEqual(t, contact.PhoneNumber, masked, "fixture phone number must look like a phone number for this test to mean anything")

	req := testutil.NewJSONRequest(t, map[string]any{"name": masked})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	var after models.Contact
	require.NoError(t, app.DB.First(&after, "id = ?", contact.ID).Error)
	assert.Equal(t, contact.PhoneNumber, after.ProfileName, "o nome real nao pode ter sido sobrescrito pela mascara")
}

// Um nome longo demais para o varchar(255) deve virar 400, nao um 500 com
// log de erro — e um problema de entrada do cliente, nao do servidor.
func TestUpdateContactName_RejectsTooLong(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": strings.Repeat("a", 300)})
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
