package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCallWebhookTestApp builds a minimal App with DB, Redis and a running
// WebSocket hub — everything sticky routing touches.
func newCallWebhookTestApp(t *testing.T) *App {
	t.Helper()
	db := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set, skipping test")
	}
	log := testutil.NopLogger()
	hub := websocket.NewHub(log)
	go hub.Run()

	return &App{DB: db, Log: log, Redis: rdb, WSHub: hub}
}

// bringUserOnline registers a connection-less client so IsUserOnline reports true.
func bringUserOnline(t *testing.T, hub *websocket.Hub, orgID, userID uuid.UUID) {
	t.Helper()
	hub.Register(websocket.NewClient(hub, nil, userID, orgID))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.IsUserOnline(orgID, userID) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the user to come online")
}

func createTestCallLog(t *testing.T, app *App, orgID, contactID uuid.UUID, accountName, phone string, mutate func(*models.CallLog)) *models.CallLog {
	t.Helper()
	now := time.Now()
	callLog := &models.CallLog{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: accountName,
		ContactID:       contactID,
		WhatsAppCallID:  "wacid." + uuid.NewString(),
		CallerPhone:     phone,
		Direction:       models.CallDirectionIncoming,
		Status:          models.CallStatusRinging,
		StartedAt:       &now,
	}
	if mutate != nil {
		mutate(callLog)
	}
	require.NoError(t, app.DB.Create(callLog).Error)
	return callLog
}

// --- resolveStickyRoute ---

func TestResolveStickyRoute_OrganicCallHasNoRoute(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	assert.Nil(t, app.resolveStickyRoute(context.Background(), "", org.ID, "+15550001111"))
}

func TestResolveStickyRoute_MalformedPayloadWithoutRedisKeyHasNoRoute(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	assert.Nil(t, app.resolveStickyRoute(context.Background(), "agent:not-a-uuid", org.ID, "+15550001112"))
}

func TestResolveStickyRoute_MetaPayloadMatchesEligibleAgent(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	bringUserOnline(t, app.WSHub, org.ID, agent.ID)

	route := app.resolveStickyRoute(context.Background(), "agent:"+agent.ID.String(), org.ID, "+15550001113")

	require.NotNil(t, route)
	assert.Equal(t, agent.ID, route.AgentID)
	assert.True(t, route.Eligible)
}

func TestResolveStickyRoute_RedisKeyMatchesEligibleAgent(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	bringUserOnline(t, app.WSHub, org.ID, agent.ID)

	const phone = "+15550001114"
	app.MarkPendingStickyCall(context.Background(), org.ID, phone, agent.ID, 15)

	route := app.resolveStickyRoute(context.Background(), "", org.ID, phone)

	require.NotNil(t, route)
	assert.Equal(t, agent.ID, route.AgentID)
	assert.True(t, route.Eligible)
}

// An offline or off-shift agent must still yield a route. Returning nil would
// leak the call into the org-wide broadcast, which is exactly what click-to-call
// calls must never do.
func TestResolveStickyRoute_OfflineAgentStaysStickyButIneligible(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)

	const phone = "+15550001115"
	app.MarkPendingStickyCall(context.Background(), org.ID, phone, agent.ID, 15)

	route := app.resolveStickyRoute(context.Background(), "", org.ID, phone)

	require.NotNil(t, route)
	assert.Equal(t, agent.ID, route.AgentID)
	assert.False(t, route.Eligible)
}

func TestResolveStickyRoute_UnavailableAgentStaysStickyButIneligible(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(agent).Update("is_available", false).Error)
	bringUserOnline(t, app.WSHub, org.ID, agent.ID)

	route := app.resolveStickyRoute(context.Background(), "agent:"+agent.ID.String(), org.ID, "+15550001116")

	require.NotNil(t, route)
	assert.Equal(t, agent.ID, route.AgentID)
	assert.False(t, route.Eligible)
}

// A payload from another org must not route — and must not be treated as sticky
// either, since it tells us nothing about this org's call.
func TestResolveStickyRoute_ForeignOrgAgentHasNoRoute(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	otherOrg := testutil.CreateTestOrganization(t, app.DB)
	agent := testutil.CreateTestUser(t, app.DB, otherOrg.ID)

	route := app.resolveStickyRoute(context.Background(), "agent:"+agent.ID.String(), org.ID, "+15550001117")

	assert.Nil(t, route)
}

// --- stickyRouteForCall ---

// The Redis key only lives as long as the button's TTL. Once a call is in
// flight the agent recorded on the call log keeps it sticky, so a mid-call
// expiry can't leak the call into the team's queue.
func TestStickyRouteForCall_SurvivesExpiredRedisKey(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	bringUserOnline(t, app.WSHub, org.ID, agent.ID)

	callLog := createTestCallLog(t, app, org.ID, contact.ID, account.Name, contact.PhoneNumber, func(cl *models.CallLog) {
		cl.StickyAgentID = &agent.ID
	})

	// No payload, no Redis key — only the call log remembers.
	route := app.stickyRouteForCall(context.Background(), "", org.ID, contact.PhoneNumber, callLog)

	require.NotNil(t, route)
	assert.Equal(t, agent.ID, route.AgentID)
	assert.True(t, route.Eligible)
}

// An organic call has nothing recorded, so it keeps today's routing.
func TestStickyRouteForCall_OrganicCallHasNoRoute(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
	callLog := createTestCallLog(t, app, org.ID, contact.ID, account.Name, contact.PhoneNumber, nil)

	assert.Nil(t, app.stickyRouteForCall(context.Background(), "", org.ID, contact.PhoneNumber, callLog))
}

// --- recordMissedCallMessage ---

func TestRecordMissedCallMessage_AddsChatEntryAndBumpsContact(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	callLog := createTestCallLog(t, app, org.ID, contact.ID, account.Name, contact.PhoneNumber, func(cl *models.CallLog) {
		cl.StickyAgentID = &agent.ID
		cl.Status = models.CallStatusMissed
	})

	app.recordMissedCallMessage(callLog, contact)

	var msg models.Message
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&msg).Error)
	assert.Equal(t, models.MessageTypeCall, msg.MessageType)
	assert.Equal(t, models.DirectionIncoming, msg.Direction)
	assert.Equal(t, callLog.ID.String(), msg.Metadata["call_log_id"])
	assert.Equal(t, account.Name, msg.WhatsAppAccount)

	var updated models.Contact
	require.NoError(t, app.DB.First(&updated, contact.ID).Error)
	assert.False(t, updated.IsRead, "a missed call should leave the conversation unread")
	assert.NotNil(t, updated.LastMessageAt)
	assert.Contains(t, updated.LastMessagePreview, "Missed call")
}

// Meta re-sends terminate events; a second delivery must not post a second
// missed-call bubble.
func TestRecordMissedCallMessage_IsIdempotent(t *testing.T) {
	app := newCallWebhookTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))
	callLog := createTestCallLog(t, app, org.ID, contact.ID, account.Name, contact.PhoneNumber, nil)

	app.recordMissedCallMessage(callLog, contact)
	app.recordMissedCallMessage(callLog, contact)

	var count int64
	require.NoError(t, app.DB.Model(&models.Message{}).Where("contact_id = ?", contact.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
