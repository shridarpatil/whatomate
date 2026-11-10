package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postWebhook(t *testing.T, app *App, body string) {
	t.Helper()
	req := testutil.NewRequest(t)
	req.RequestCtx.Request.Header.SetMethod("POST")
	req.RequestCtx.Request.Header.SetContentType("application/json")
	req.RequestCtx.Request.SetBody([]byte(body))
	require.NoError(t, app.WebhookHandler(req))
}

func TestWebhookHandler_history(t *testing.T) {
	app := webhookTestApp(t)

	uid := uuid.New().String()[:8]
	org := models.Organization{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "hist-org-" + uid, Slug: "hist-org-" + uid}
	require.NoError(t, app.DB.Create(&org).Error)
	account := models.WhatsAppAccount{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "hist-acct-" + uid,
		PhoneID:        "phone-hist-" + uid,
		BusinessID:     "biz-hist-" + uid,
		AccessToken:    "token",
		IsSMB:          true,
	}
	require.NoError(t, app.DB.Create(&account).Error)

	customer := "1650555" + uid[:4]
	envelope := func(history string) string {
		return `{"object":"whatsapp_business_account","entry":[{"id":"` + account.BusinessID + `","changes":[{"field":"history","value":{
			"messaging_product":"whatsapp",
			"metadata":{"display_phone_number":"15550001111","phone_number_id":"` + account.PhoneID + `"},
			"history":` + history + `}}]}]}`
	}

	// Two chunks, sent out of order; one message repeated across chunks.
	postWebhook(t, app, envelope(`[
		{"metadata":{"phase":0,"chunk_order":2,"progress":100},"threads":[{"id":"`+customer+`","context":{"wa_id":"`+customer+`"},"messages":[
			{"from":"15550001111","id":"wamid.hist-out-`+uid+`","timestamp":"1739300000","type":"text","text":{"body":"Thanks for your order!"},"history_context":{"status":"READ"}},
			{"from":"`+customer+`","id":"wamid.hist-media-`+uid+`","timestamp":"1739300100","type":"media_placeholder","history_context":{"status":"DELIVERED"}}
		]}]},
		{"metadata":{"phase":0,"chunk_order":1,"progress":50},"threads":[{"id":"`+customer+`","context":{"wa_id":"`+customer+`"},"messages":[
			{"from":"`+customer+`","id":"wamid.hist-in-`+uid+`","timestamp":"1739299000","type":"text","text":{"body":"Where is my order?"},"history_context":{"status":"READ"}},
			{"from":"15550001111","id":"wamid.hist-out-`+uid+`","timestamp":"1739300000","type":"text","text":{"body":"Thanks for your order!"},"history_context":{"status":"READ"}}
		]}]}
	]`))

	var msgs []models.Message
	assert.Eventually(t, func() bool {
		msgs = nil
		app.DB.Where("organization_id = ?", org.ID).Order("created_at").Find(&msgs)
		return len(msgs) == 3
	}, 3*time.Second, 20*time.Millisecond)
	require.Len(t, msgs, 3)

	assert.Equal(t, models.DirectionIncoming, msgs[0].Direction)
	assert.Equal(t, models.MessageStatusReceived, msgs[0].Status)
	assert.Equal(t, "Where is my order?", msgs[0].Content)
	assert.Equal(t, time.Unix(1739299000, 0).UTC(), msgs[0].CreatedAt.UTC())

	assert.Equal(t, models.DirectionOutgoing, msgs[1].Direction)
	assert.Equal(t, models.MessageStatusRead, msgs[1].Status)

	assert.Equal(t, models.DirectionIncoming, msgs[2].Direction)
	assert.Equal(t, historyMediaPlaceholder, msgs[2].Content)

	var contact models.Contact
	require.NoError(t, app.DB.Where("organization_id = ? AND phone_number = ?", org.ID, customer).First(&contact).Error)
	require.NotNil(t, contact.LastMessageAt)
	assert.Equal(t, time.Unix(1739300100, 0).UTC(), contact.LastMessageAt.UTC())

	// Declined history sharing: logged, nothing imported, no panic.
	postWebhook(t, app, envelope(`[{"errors":[{"code":2593109,"title":"History sync is turned off","message":"History sync is turned off"}]}]`))
	time.Sleep(100 * time.Millisecond)
	var count int64
	app.DB.Model(&models.Message{}).Where("organization_id = ?", org.ID).Count(&count)
	assert.Equal(t, int64(3), count)
}

func TestHistoryStatus(t *testing.T) {
	assert.Equal(t, models.MessageStatusRead, historyStatus("READ"))
	assert.Equal(t, models.MessageStatusRead, historyStatus("PLAYED"))
	assert.Equal(t, models.MessageStatusDelivered, historyStatus("DELIVERED"))
	assert.Equal(t, models.MessageStatusFailed, historyStatus("ERROR"))
	assert.Equal(t, models.MessageStatusPending, historyStatus("PENDING"))
	assert.Equal(t, models.MessageStatusSent, historyStatus(""))
}
