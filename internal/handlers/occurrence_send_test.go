package handlers_test

import (
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// GATE 5. Fora da janela de 24h o envio é recusado com 422, antes de qualquer
// chamada à Meta.
func TestOccurrenceSendProtocol_RejectedOutsideServiceWindow(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", user.ID).Error)
	old := time.Now().Add(-30 * time.Hour)
	require.NoError(t, app.DB.Model(contact).Update("last_inbound_at", old).Error)

	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)
	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Fora da janela",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", occ.ID.String())

	require.NoError(t, app.SendOccurrenceProtocol(req))
	assert.Equal(t, fasthttp.StatusUnprocessableEntity, testutil.GetResponseStatusCode(req))

	var sent int64
	app.DB.Model(&models.OccurrenceEvent{}).
		Where("occurrence_id = ? AND type = ?", occ.ID, models.OccurrenceEventProtocolSent).
		Count(&sent)
	assert.EqualValues(t, 0, sent, "nenhum evento de envio pode ser gravado")
}

// Contato que nunca enviou mensagem também está fora da janela.
func TestOccurrenceSendProtocol_RejectedWhenNeverInbound(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", user.ID).Error)

	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)
	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Sem inbound",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", occ.ID.String())

	require.NoError(t, app.SendOccurrenceProtocol(req))
	assert.Equal(t, fasthttp.StatusUnprocessableEntity, testutil.GetResponseStatusCode(req))
}

// GATE 4 aplicado ao envio: quem não enxerga o contato não envia.
func TestOccurrenceSendProtocol_DeniedForInvisibleContact(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	outsiderRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "occ-send-outsider",
		[]string{"chat:read", "chat:write"})
	outsider := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&outsiderRole.ID))
	enableStrictVisibility(t, app, org.ID)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", owner.ID).Error)
	require.NoError(t, app.DB.Model(contact).Update("last_inbound_at", time.Now()).Error)

	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)
	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Privada",
		StageID: stage.ID, OpenedByUserID: owner.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, outsider.ID)
	testutil.SetPathParam(req, "id", occ.ID.String())

	require.NoError(t, app.SendOccurrenceProtocol(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// Dentro da janela: envia, grava o evento e a mensagem carrega o protocolo.
// Usa o mock de servidor WhatsApp já existente em messages_test.go (mesmo
// pacote handlers_test) — não é um mock novo.
func TestOccurrenceSendProtocol_SendsWithinWindow(t *testing.T) {
	mockServer := newMockWhatsAppServer()
	defer mockServer.close()

	app := newMsgTestApp(t, mockServer)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	account := createTestAccount(t, app, org.ID)

	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", user.ID).Error)
	require.NoError(t, app.DB.Model(contact).Update("last_inbound_at", time.Now()).Error)

	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)
	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Dentro da janela",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", occ.ID.String())

	require.NoError(t, app.SendOccurrenceProtocol(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	assert.Contains(t, string(testutil.GetResponseBody(req)), occ.ProtocolNumber)

	var evt models.OccurrenceEvent
	require.NoError(t, app.DB.Where("occurrence_id = ? AND type = ?",
		occ.ID, models.OccurrenceEventProtocolSent).First(&evt).Error)
	assert.Equal(t, occ.ProtocolNumber, evt.Content)

	// The send itself runs async (DefaultSendOptions); poll briefly for it to
	// land on the mock Meta server.
	require.Eventually(t, func() bool { return len(mockServer.sentMessages) == 1 },
		time.Second, 10*time.Millisecond)
	body := mockServer.sentMessages[0]["text"].(map[string]any)["body"].(string)
	assert.Contains(t, body, occ.ProtocolNumber)
}
