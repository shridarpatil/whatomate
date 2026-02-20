package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSLATestApp creates a minimal App for SLA processor internal tests.
func newSLATestApp(t *testing.T) *App {
	t.Helper()
	db := testutil.SetupTestDB(t)
	log := testutil.NopLogger()
	hub := websocket.NewHub(log)
	go hub.Run()

	app := &App{
		DB:    db,
		Log:   log,
		WSHub: hub,
	}
	if rdb := testutil.SetupTestRedis(t); rdb != nil {
		app.Redis = rdb
	}
	return app
}

// createSLATestTransfer creates an active agent transfer in the DB with the given SLA fields.
func createSLATestTransfer(t *testing.T, app *App, orgID, contactID, agentID uuid.UUID, accountName string, sla models.SLATracking) *models.AgentTransfer {
	t.Helper()
	transfer := &models.AgentTransfer{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		ContactID:       contactID,
		AgentID:         &agentID,
		WhatsAppAccount: accountName,
		PhoneNumber:     "+1234567890",
		Status:          models.TransferStatusActive,
		SLA:             sla,
	}
	require.NoError(t, app.DB.Create(transfer).Error)
	return transfer
}

// createTestAgentMessage creates an outgoing message from the given agent for the given contact.
func createTestAgentMessage(t *testing.T, app *App, orgID, contactID, agentID uuid.UUID, accountName string, sentAt time.Time) {
	t.Helper()
	msg := &models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New(), CreatedAt: sentAt},
		OrganizationID:  orgID,
		ContactID:       contactID,
		WhatsAppAccount: accountName,
		Direction:       models.DirectionOutgoing,
		MessageType:     models.MessageTypeText,
		Content:         "agent reply",
		SentByUserID:    &agentID,
		Status:          models.MessageStatusSent,
	}
	require.NoError(t, app.DB.Create(msg).Error)
}

// --- agentRespondedSince ---

func TestAgentRespondedSince_TrueWhenMessageAfterTimestamp(t *testing.T) {
	app := newSLATestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	since := time.Now().Add(-10 * time.Minute)
	// Agent sent a message 5 minutes ago (after since)
	createTestAgentMessage(t, app, org.ID, contact.ID, agent.ID, account.Name, time.Now().Add(-5*time.Minute))

	proc := NewSLAProcessor(app, time.Minute)
	transfer := models.AgentTransfer{
		ContactID: contact.ID,
		AgentID:   &agent.ID,
	}

	assert.True(t, proc.agentRespondedSince(transfer, since))
}

func TestAgentRespondedSince_FalseWhenNoMessages(t *testing.T) {
	app := newSLATestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)

	proc := NewSLAProcessor(app, time.Minute)
	transfer := models.AgentTransfer{
		ContactID: contact.ID,
		AgentID:   &agent.ID,
	}

	assert.False(t, proc.agentRespondedSince(transfer, time.Now().Add(-1*time.Hour)))
}

func TestAgentRespondedSince_FalseWhenNoAgent(t *testing.T) {
	app := newSLATestApp(t)

	proc := NewSLAProcessor(app, time.Minute)
	transfer := models.AgentTransfer{
		ContactID: uuid.New(),
		AgentID:   nil,
	}

	assert.False(t, proc.agentRespondedSince(transfer, time.Now().Add(-1*time.Hour)))
}

func TestAgentRespondedSince_FalseWhenMessageBeforeTimestamp(t *testing.T) {
	app := newSLATestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	// Agent sent a message 20 minutes ago
	createTestAgentMessage(t, app, org.ID, contact.ID, agent.ID, account.Name, time.Now().Add(-20*time.Minute))

	proc := NewSLAProcessor(app, time.Minute)
	transfer := models.AgentTransfer{
		ContactID: contact.ID,
		AgentID:   &agent.ID,
	}

	// Check since 10 minutes ago — the message at -20m is before that
	assert.False(t, proc.agentRespondedSince(transfer, time.Now().Add(-10*time.Minute)))
}

// --- autoCloseExpiredTransfers: skipped when agent active ---

func TestSLAAutoCloseSkippedWhenAgentActive(t *testing.T) {
	app := newSLATestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	autoCloseHours := 2
	// Transfer created 3 hours ago, expired 1 hour ago
	expiresAt := time.Now().Add(-1 * time.Hour)
	transfer := createSLATestTransfer(t, app, org.ID, contact.ID, agent.ID, account.Name, models.SLATracking{
		ExpiresAt: &expiresAt,
	})

	// Agent sent a message 30 minutes ago (after transfer was created)
	createTestAgentMessage(t, app, org.ID, contact.ID, agent.ID, account.Name, time.Now().Add(-30*time.Minute))

	settings := models.ChatbotSettings{
		OrganizationID: org.ID,
		SLA: models.SLAConfig{
			Enabled:        true,
			AutoCloseHours: autoCloseHours,
		},
	}

	proc := NewSLAProcessor(app, time.Minute)
	proc.autoCloseExpiredTransfers(org.ID, settings, time.Now())

	// Reload transfer — should still be active with extended expiry
	var updated models.AgentTransfer
	require.NoError(t, app.DB.Where("id = ?", transfer.ID).First(&updated).Error)

	assert.Equal(t, models.TransferStatusActive, updated.Status, "transfer should still be active")
	require.NotNil(t, updated.SLA.ExpiresAt)
	assert.True(t, updated.SLA.ExpiresAt.After(time.Now().Add(time.Duration(autoCloseHours-1)*time.Hour)),
		"expires_at should be extended into the future")
}

// --- autoCloseExpiredTransfers: fires when no agent response ---

func TestSLAAutoCloseFiresWhenNoAgentResponse(t *testing.T) {
	app := newSLATestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	// Transfer expired 1 hour ago, no agent messages at all
	expiresAt := time.Now().Add(-1 * time.Hour)
	transfer := createSLATestTransfer(t, app, org.ID, contact.ID, agent.ID, account.Name, models.SLATracking{
		ExpiresAt: &expiresAt,
	})

	settings := models.ChatbotSettings{
		OrganizationID: org.ID,
		SLA: models.SLAConfig{
			Enabled:        true,
			AutoCloseHours: 2,
		},
	}

	proc := NewSLAProcessor(app, time.Minute)
	proc.autoCloseExpiredTransfers(org.ID, settings, time.Now())

	// Reload transfer — should be expired
	var updated models.AgentTransfer
	require.NoError(t, app.DB.Where("id = ?", transfer.ID).First(&updated).Error)

	assert.Equal(t, models.TransferStatusExpired, updated.Status, "transfer should be expired")
}

// --- escalateTransfers: skipped when agent active ---

func TestSLAEscalationSkippedWhenAgentActive(t *testing.T) {
	app := newSLATestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	escalationMinutes := 30
	// Escalation was due 5 minutes ago
	escalationAt := time.Now().Add(-5 * time.Minute)
	transfer := createSLATestTransfer(t, app, org.ID, contact.ID, agent.ID, account.Name, models.SLATracking{
		EscalationAt:    &escalationAt,
		EscalationLevel: 0,
	})

	// Agent sent a message 10 minutes ago (after the escalation was originally set)
	createTestAgentMessage(t, app, org.ID, contact.ID, agent.ID, account.Name, time.Now().Add(-10*time.Minute))

	settings := models.ChatbotSettings{
		OrganizationID: org.ID,
		SLA: models.SLAConfig{
			Enabled:           true,
			EscalationMinutes: escalationMinutes,
		},
	}

	proc := NewSLAProcessor(app, time.Minute)
	proc.escalateTransfers(org.ID, settings, time.Now())

	// Reload transfer — should still be at escalation level 0 with extended deadline
	var updated models.AgentTransfer
	require.NoError(t, app.DB.Where("id = ?", transfer.ID).First(&updated).Error)

	assert.Equal(t, 0, updated.SLA.EscalationLevel, "escalation level should not increase")
	require.NotNil(t, updated.SLA.EscalationAt)
	assert.True(t, updated.SLA.EscalationAt.After(time.Now().Add(time.Duration(escalationMinutes-1)*time.Minute)),
		"escalation_at should be extended into the future")
}

// --- escalateTransfers: fires when no agent response ---

func TestSLAEscalationFiresWhenNoAgentResponse(t *testing.T) {
	app := newSLATestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	agent := testutil.CreateTestUser(t, app.DB, org.ID)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)

	// Escalation was due 5 minutes ago, no agent messages
	escalationAt := time.Now().Add(-5 * time.Minute)
	transfer := createSLATestTransfer(t, app, org.ID, contact.ID, agent.ID, account.Name, models.SLATracking{
		EscalationAt:    &escalationAt,
		EscalationLevel: 0,
	})

	settings := models.ChatbotSettings{
		OrganizationID: org.ID,
		SLA: models.SLAConfig{
			Enabled:           true,
			EscalationMinutes: 30,
		},
	}

	proc := NewSLAProcessor(app, time.Minute)
	proc.escalateTransfers(org.ID, settings, time.Now())

	// Reload transfer — should be escalated to level 1
	var updated models.AgentTransfer
	require.NoError(t, app.DB.Where("id = ?", transfer.ID).First(&updated).Error)

	assert.Equal(t, 1, updated.SLA.EscalationLevel, "escalation level should increase to 1")
	require.NotNil(t, updated.SLA.EscalatedAt)
}
