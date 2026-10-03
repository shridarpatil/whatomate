package handlers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/contactutil"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

func withMediaDir(dir string) appOption {
	return func(a *handlers.App) {
		a.Config.Storage.LocalPath = dir
	}
}

func createChatMessage(t *testing.T, db *gorm.DB, contact *models.Contact, account string, mutate func(*models.Message)) *models.Message {
	t.Helper()
	msg := &models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  contact.OrganizationID,
		WhatsAppAccount: account,
		ContactID:       contact.ID,
		Direction:       models.DirectionIncoming,
		MessageType:     models.MessageTypeText,
		Content:         "hello",
		Status:          models.MessageStatusDelivered,
	}
	if mutate != nil {
		mutate(msg)
	}
	require.NoError(t, db.Create(msg).Error)
	return msg
}

func writeMediaFile(t *testing.T, dir, rel string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte("data"), 0o644))
}

func countRows(db *gorm.DB, model any, query string, args ...any) int64 {
	var n int64
	db.Unscoped().Model(model).Where(query, args...).Count(&n)
	return n
}

func TestApp_DeleteConversation_RemovesChatHistory(t *testing.T) {
	mediaDir := t.TempDir()
	app := newTestApp(t, withMediaDir(mediaDir))
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "chat-deleter", []string{"contacts:delete"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
	other := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
	require.NoError(t, app.DB.Model(contact).Updates(map[string]any{
		"last_message_preview": "hello", "is_read": false,
	}).Error)

	// Media: one file owned by this chat only, one campaign header shared with another contact.
	writeMediaFile(t, mediaDir, "images/own.jpg")
	writeMediaFile(t, mediaDir, "images/shared.jpg")

	first := createChatMessage(t, app.DB, contact, account.Name, func(m *models.Message) {
		m.MessageType = models.MessageTypeImage
		m.MediaURL = "images/own.jpg"
	})
	reply := createChatMessage(t, app.DB, contact, account.Name, func(m *models.Message) {
		m.IsReply = true
		m.ReplyToMessageID = &first.ID
	})
	campaignMsg := createChatMessage(t, app.DB, contact, account.Name, func(m *models.Message) {
		m.Direction = models.DirectionOutgoing
		m.MessageType = models.MessageTypeTemplate
		m.MediaURL = "images/shared.jpg"
	})
	otherMsg := createChatMessage(t, app.DB, other, account.Name, func(m *models.Message) {
		m.Direction = models.DirectionOutgoing
		m.MessageType = models.MessageTypeTemplate
		m.MediaURL = "images/shared.jpg"
	})

	template := testutil.CreateTestTemplate(t, app.DB, org.ID, account.Name)
	campaign := &models.BulkMessageCampaign{
		BaseModel:            models.BaseModel{ID: uuid.New()},
		OrganizationID:       org.ID,
		WhatsAppAccount:      account.Name,
		Name:                 "promo",
		TemplateID:           template.ID,
		HeaderMediaLocalPath: "images/shared.jpg",
		CreatedBy:            user.ID,
	}
	require.NoError(t, app.DB.Create(campaign).Error)
	recipient := &models.BulkMessageRecipient{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		CampaignID:  campaign.ID,
		PhoneNumber: contact.PhoneNumber,
		Status:      models.MessageStatusDelivered,
		MessageID:   &campaignMsg.ID,
	}
	require.NoError(t, app.DB.Create(recipient).Error)

	require.NoError(t, app.DB.Create(&models.ConversationNote{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID,
		ContactID: contact.ID, CreatedByID: user.ID, Content: "vip",
	}).Error)
	session := &models.ChatbotSession{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, ContactID: contact.ID,
		WhatsAppAccount: account.Name, PhoneNumber: contact.PhoneNumber, Status: models.SessionStatusActive,
	}
	require.NoError(t, app.DB.Create(session).Error)
	require.NoError(t, app.DB.Create(&models.ChatbotSessionMessage{
		BaseModel: models.BaseModel{ID: uuid.New()}, SessionID: session.ID,
		Direction: models.DirectionIncoming, Message: "hi",
	}).Error)
	transfer := &models.AgentTransfer{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID, ContactID: contact.ID,
		WhatsAppAccount: account.Name, PhoneNumber: contact.PhoneNumber, Status: models.TransferStatusActive,
	}
	require.NoError(t, app.DB.Create(transfer).Error)

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())

	require.NoError(t, app.DeleteConversation(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		DeletedMessages int64 `json:"deleted_messages"`
	}
	testutil.ParseEnvelopeResponse(t, req, &resp)
	assert.Equal(t, int64(3), resp.DeletedMessages)

	assert.Zero(t, countRows(app.DB, &models.Message{}, "contact_id = ?", contact.ID))
	assert.Zero(t, countRows(app.DB, &models.ConversationNote{}, "contact_id = ?", contact.ID))
	assert.Zero(t, countRows(app.DB, &models.ChatbotSession{}, "contact_id = ?", contact.ID))
	assert.Zero(t, countRows(app.DB, &models.ChatbotSessionMessage{}, "session_id = ?", session.ID))
	assert.Equal(t, int64(1), countRows(app.DB, &models.Message{}, "id = ?", otherMsg.ID), "other chats are untouched")
	assert.Zero(t, countRows(app.DB, &models.Message{}, "id = ?", reply.ID))

	var gotRecipient models.BulkMessageRecipient
	require.NoError(t, app.DB.First(&gotRecipient, "id = ?", recipient.ID).Error)
	assert.Nil(t, gotRecipient.MessageID, "campaign recipient is kept but unlinked")
	assert.Equal(t, models.MessageStatusDelivered, gotRecipient.Status)

	var gotTransfer models.AgentTransfer
	require.NoError(t, app.DB.First(&gotTransfer, "id = ?", transfer.ID).Error)
	assert.Equal(t, models.TransferStatusResumed, gotTransfer.Status)
	require.NotNil(t, gotTransfer.ResumedBy)
	assert.Equal(t, user.ID, *gotTransfer.ResumedBy)

	var gotContact models.Contact
	require.NoError(t, app.DB.Unscoped().First(&gotContact, "id = ?", contact.ID).Error)
	assert.True(t, gotContact.DeletedAt.Valid, "contact is soft-deleted")
	assert.Empty(t, gotContact.LastMessagePreview)
	assert.Nil(t, gotContact.LastMessageAt)
	assert.True(t, gotContact.IsRead)

	assert.NoFileExists(t, filepath.Join(mediaDir, "images/own.jpg"))
	assert.FileExists(t, filepath.Join(mediaDir, "images/shared.jpg"), "file still referenced elsewhere is kept")
}

func TestApp_DeleteConversation_RequiresContactsDeletePermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "chat-rw", []string{"chat:read", "chat:write", "contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
	createChatMessage(t, app.DB, contact, account.Name, nil)

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())

	require.NoError(t, app.DeleteConversation(req))
	testutil.AssertErrorResponse(t, req, fasthttp.StatusForbidden, "permission to delete chats")
	assert.Equal(t, int64(1), countRows(app.DB, &models.Message{}, "contact_id = ?", contact.ID))
}

func TestApp_DeleteConversation_CrossOrgIsolation(t *testing.T) {
	app := newTestApp(t)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)
	roleB := testutil.CreateTestRoleWithKeys(t, app.DB, orgB.ID, "chat-deleter", []string{"contacts:delete"})
	userB := testutil.CreateTestUser(t, app.DB, orgB.ID, testutil.WithRoleID(&roleB.ID))
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, orgA.ID)
	contactA := testutil.CreateTestContactWith(t, app.DB, orgA.ID, testutil.WithContactAccount(account.Name))
	createChatMessage(t, app.DB, contactA, account.Name, nil)

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, orgB.ID, userB.ID)
	testutil.SetPathParam(req, "id", contactA.ID.String())

	require.NoError(t, app.DeleteConversation(req))
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
	assert.Equal(t, int64(1), countRows(app.DB, &models.Message{}, "contact_id = ?", contactA.ID))
}

func TestApp_DeleteConversation_ContactReturnsWithEmptyHistory(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "chat-deleter", []string{"contacts:delete"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
	createChatMessage(t, app.DB, contact, account.Name, nil)

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())
	require.NoError(t, app.DeleteConversation(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// The customer writes again: the same contact comes back, without the old messages.
	restored, created, err := contactutil.GetOrCreateContact(app.DB, org.ID, contact.PhoneNumber, "")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, contact.ID, restored.ID)
	assert.False(t, restored.DeletedAt.Valid)
	assert.Zero(t, countRows(app.DB, &models.Message{}, "contact_id = ?", contact.ID))
}
