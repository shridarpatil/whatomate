package handlers_test

import (
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SetSLADeadlines ---

func TestSetSLADeadlines_AllFieldsSet(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)

	transfer := &models.AgentTransfer{}
	settings := &models.ChatbotSettings{
		SLA: models.SLAConfig{
			Enabled:           true,
			ResponseMinutes:   10,
			ResolutionMinutes: 60,
			EscalationMinutes: 20,
			AutoCloseHours:    24,
		},
	}

	before := time.Now()
	app.SetSLADeadlines(transfer, settings)
	after := time.Now()

	require.NotNil(t, transfer.SLA.ResponseDeadline)
	require.NotNil(t, transfer.SLA.ResolutionDeadline)
	require.NotNil(t, transfer.SLA.EscalationAt)
	require.NotNil(t, transfer.SLA.ExpiresAt)

	// Response deadline should be ~10 minutes from now
	assert.True(t, transfer.SLA.ResponseDeadline.After(before.Add(9*time.Minute)))
	assert.True(t, transfer.SLA.ResponseDeadline.Before(after.Add(11*time.Minute)))

	// Resolution deadline should be ~60 minutes from now
	assert.True(t, transfer.SLA.ResolutionDeadline.After(before.Add(59*time.Minute)))
	assert.True(t, transfer.SLA.ResolutionDeadline.Before(after.Add(61*time.Minute)))

	// Escalation deadline should be ~20 minutes from now
	assert.True(t, transfer.SLA.EscalationAt.After(before.Add(19*time.Minute)))
	assert.True(t, transfer.SLA.EscalationAt.Before(after.Add(21*time.Minute)))

	// Expiry deadline should be ~24 hours from now
	assert.True(t, transfer.SLA.ExpiresAt.After(before.Add(23*time.Hour)))
	assert.True(t, transfer.SLA.ExpiresAt.Before(after.Add(25*time.Hour)))
}

func TestSetSLADeadlines_DisabledSLA(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)

	transfer := &models.AgentTransfer{}
	settings := &models.ChatbotSettings{
		SLA: models.SLAConfig{
			Enabled:           false,
			ResponseMinutes:   10,
			ResolutionMinutes: 60,
			EscalationMinutes: 20,
			AutoCloseHours:    24,
		},
	}

	app.SetSLADeadlines(transfer, settings)

	assert.Nil(t, transfer.SLA.ResponseDeadline)
	assert.Nil(t, transfer.SLA.ResolutionDeadline)
	assert.Nil(t, transfer.SLA.EscalationAt)
	assert.Nil(t, transfer.SLA.ExpiresAt)
}

func TestSetSLADeadlines_PartialConfig(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)

	transfer := &models.AgentTransfer{}
	settings := &models.ChatbotSettings{
		SLA: models.SLAConfig{
			Enabled:         true,
			ResponseMinutes: 15,
			// ResolutionMinutes, EscalationMinutes, AutoCloseHours all zero
		},
	}

	app.SetSLADeadlines(transfer, settings)

	assert.NotNil(t, transfer.SLA.ResponseDeadline)
	assert.Nil(t, transfer.SLA.ResolutionDeadline)
	assert.Nil(t, transfer.SLA.EscalationAt)
	assert.Nil(t, transfer.SLA.ExpiresAt)
}

// --- UpdateSLAOnPickup ---

func TestUpdateSLAOnPickup_WithinDeadline(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)

	futureDeadline := time.Now().Add(30 * time.Minute)
	transfer := &models.AgentTransfer{
		SLA: models.SLATracking{
			ResponseDeadline: &futureDeadline,
		},
	}

	before := time.Now()
	app.UpdateSLAOnPickup(transfer)

	require.NotNil(t, transfer.SLA.PickedUpAt)
	assert.False(t, transfer.SLA.PickedUpAt.Before(before))
	assert.False(t, transfer.SLA.Breached, "SLA should not be breached when picked up before deadline")
	assert.Nil(t, transfer.SLA.BreachedAt)
}

func TestUpdateSLAOnPickup_AfterDeadline(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)

	pastDeadline := time.Now().Add(-10 * time.Minute)
	transfer := &models.AgentTransfer{
		SLA: models.SLATracking{
			ResponseDeadline: &pastDeadline,
		},
	}

	app.UpdateSLAOnPickup(transfer)

	require.NotNil(t, transfer.SLA.PickedUpAt)
	assert.True(t, transfer.SLA.Breached, "SLA should be breached when picked up after deadline")
	require.NotNil(t, transfer.SLA.BreachedAt)
}

func TestUpdateSLAOnPickup_NoDeadline(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)

	transfer := &models.AgentTransfer{
		SLA: models.SLATracking{
			ResponseDeadline: nil,
		},
	}

	app.UpdateSLAOnPickup(transfer)

	require.NotNil(t, transfer.SLA.PickedUpAt)
	assert.False(t, transfer.SLA.Breached, "SLA should not be breached when no deadline is set")
	assert.Nil(t, transfer.SLA.BreachedAt)
}

// --- UpdateSLAOnFirstResponse ---

func TestUpdateSLAOnFirstResponse_SetsTimestamp(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)

	transfer := &models.AgentTransfer{}

	before := time.Now()
	app.UpdateSLAOnFirstResponse(transfer)

	require.NotNil(t, transfer.SLA.FirstResponseAt)
	assert.False(t, transfer.SLA.FirstResponseAt.Before(before))
}

func TestUpdateSLAOnFirstResponse_SkipsIfAlreadyResponded(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)

	originalTime := time.Now().Add(-1 * time.Hour)
	transfer := &models.AgentTransfer{
		SLA: models.SLATracking{
			FirstResponseAt: &originalTime,
		},
	}

	app.UpdateSLAOnFirstResponse(transfer)

	// Should not overwrite the original time
	assert.Equal(t, originalTime.Unix(), transfer.SLA.FirstResponseAt.Unix())
}

// --- UpdateContactChatbotMessage ---

func TestUpdateContactChatbotMessage_SetsTimestampAndResetsReminder(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Set reminder_sent to true first so we can verify it gets reset
	require.NoError(t, app.DB.Model(contact).Update("chatbot_reminder_sent", true).Error)

	before := time.Now()
	app.UpdateContactChatbotMessage(contact.ID)

	// Reload the contact from DB
	var updated models.Contact
	require.NoError(t, app.DB.Where("id = ?", contact.ID).First(&updated).Error)

	require.NotNil(t, updated.ChatbotLastMessageAt)
	assert.False(t, updated.ChatbotLastMessageAt.Before(before))
	assert.False(t, updated.ChatbotReminderSent, "reminder_sent should be reset to false")
}

// --- ClearContactChatbotTracking ---

func TestClearContactChatbotTracking_ClearsFields(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Set chatbot tracking fields first
	now := time.Now()
	require.NoError(t, app.DB.Model(contact).Updates(map[string]interface{}{
		"chatbot_last_message_at": now,
		"chatbot_reminder_sent":   true,
	}).Error)

	// Verify they were set
	var before models.Contact
	require.NoError(t, app.DB.Where("id = ?", contact.ID).First(&before).Error)
	require.NotNil(t, before.ChatbotLastMessageAt)
	require.True(t, before.ChatbotReminderSent)

	app.ClearContactChatbotTracking(contact.ID)

	// Reload and verify cleared
	var after models.Contact
	require.NoError(t, app.DB.Where("id = ?", contact.ID).First(&after).Error)

	assert.Nil(t, after.ChatbotLastMessageAt, "chatbot_last_message_at should be nil after clearing")
	assert.False(t, after.ChatbotReminderSent, "chatbot_reminder_sent should be false after clearing")
}

func TestClearContactChatbotTracking_NopWhenAlreadyClear(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Contact starts with nil chatbot tracking fields by default
	app.ClearContactChatbotTracking(contact.ID)

	// Should still be nil/false without errors
	var after models.Contact
	require.NoError(t, app.DB.Where("id = ?", contact.ID).First(&after).Error)
	assert.Nil(t, after.ChatbotLastMessageAt)
	assert.False(t, after.ChatbotReminderSent)
}
