package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newProcessorTestApp creates a minimal App suitable for chatbot processor tests.
// It connects to the test database and Redis, provides a mock WhatsApp client,
// and uses a no-op logger.
func newProcessorTestApp(t *testing.T) *App {
	t.Helper()
	db := testutil.SetupTestDB(t)
	log := testutil.NopLogger()

	// Mock WhatsApp API server that accepts all requests.
	waServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"messages": []map[string]string{{"id": "wamid.mock_" + uuid.New().String()[:8]}},
		})
	}))
	t.Cleanup(waServer.Close)

	app := &App{
		DB:       db,
		Log:      log,
		WhatsApp: whatsapp.NewWithBaseURL(log, waServer.URL),
	}
	if rdb := testutil.SetupTestRedis(t); rdb != nil {
		app.Redis = rdb
	}
	return app
}

// createProcessorTestOrg creates an organization and WhatsApp account for processor tests.
func createProcessorTestOrg(t *testing.T, app *App) (*models.Organization, *models.WhatsAppAccount) {
	t.Helper()
	org := testutil.CreateTestOrganization(t, app.DB)
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	return org, account
}

// =============================================================================
// matchKeywordRules
// =============================================================================

func TestMatchKeywordRules_ExactMatch(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "exact-hello",
		Keywords:        models.StringArray{"hello"},
		MatchType:       models.MatchTypeExact,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"body": "Hello response"},
		Priority:        10,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(rule).Error)

	resp, matched := app.matchKeywordRules(org.ID, account.Name, "hello")
	assert.True(t, matched)
	require.NotNil(t, resp)
	assert.Equal(t, "Hello response", resp.Body)

	// Different case should also match (case insensitive by default)
	resp2, matched2 := app.matchKeywordRules(org.ID, account.Name, "HELLO")
	assert.True(t, matched2)
	require.NotNil(t, resp2)
	assert.Equal(t, "Hello response", resp2.Body)

	// Partial should NOT match exact
	_, matched3 := app.matchKeywordRules(org.ID, account.Name, "hello world")
	assert.False(t, matched3)
}

func TestMatchKeywordRules_ExactMatch_CaseSensitive(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "exact-case",
		Keywords:        models.StringArray{"Hello"},
		MatchType:       models.MatchTypeExact,
		CaseSensitive:   true,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"body": "Case match"},
		Priority:        10,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(rule).Error)

	_, matched := app.matchKeywordRules(org.ID, account.Name, "Hello")
	assert.True(t, matched)

	_, matched2 := app.matchKeywordRules(org.ID, account.Name, "hello")
	assert.False(t, matched2)
}

func TestMatchKeywordRules_ContainsMatch(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "contains-help",
		Keywords:        models.StringArray{"help"},
		MatchType:       models.MatchTypeContains,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"body": "Help response"},
		Priority:        10,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(rule).Error)

	resp, matched := app.matchKeywordRules(org.ID, account.Name, "I need help please")
	assert.True(t, matched)
	require.NotNil(t, resp)
	assert.Equal(t, "Help response", resp.Body)

	_, matched2 := app.matchKeywordRules(org.ID, account.Name, "HELP ME")
	assert.True(t, matched2)

	_, matched3 := app.matchKeywordRules(org.ID, account.Name, "goodbye")
	assert.False(t, matched3)
}

func TestMatchKeywordRules_StartsWithMatch(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "starts-with-hi",
		Keywords:        models.StringArray{"hi"},
		MatchType:       models.MatchTypeStartsWith,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"body": "Hi response"},
		Priority:        10,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(rule).Error)

	resp, matched := app.matchKeywordRules(org.ID, account.Name, "hi there")
	assert.True(t, matched)
	require.NotNil(t, resp)
	assert.Equal(t, "Hi response", resp.Body)

	_, matched2 := app.matchKeywordRules(org.ID, account.Name, "say hi")
	assert.False(t, matched2)
}

func TestMatchKeywordRules_RegexMatch(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "regex-order",
		Keywords:        models.StringArray{`order\s*#?\d+`},
		MatchType:       models.MatchTypeRegex,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"body": "Order lookup"},
		Priority:        10,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(rule).Error)

	resp, matched := app.matchKeywordRules(org.ID, account.Name, "I have order #12345")
	assert.True(t, matched)
	require.NotNil(t, resp)
	assert.Equal(t, "Order lookup", resp.Body)

	_, matched2 := app.matchKeywordRules(org.ID, account.Name, "where is my package")
	assert.False(t, matched2)
}

func TestMatchKeywordRules_NoMatch(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "nope",
		Keywords:        models.StringArray{"specific-keyword"},
		MatchType:       models.MatchTypeExact,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"body": "reply"},
		Priority:        10,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(rule).Error)

	resp, matched := app.matchKeywordRules(org.ID, account.Name, "random message")
	assert.False(t, matched)
	assert.Nil(t, resp)
}

func TestMatchKeywordRules_Priority(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	// Lower priority rule
	lowRule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "low-priority",
		Keywords:        models.StringArray{"test"},
		MatchType:       models.MatchTypeContains,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"body": "Low priority"},
		Priority:        5,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(lowRule).Error)

	// Higher priority rule
	highRule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "high-priority",
		Keywords:        models.StringArray{"test"},
		MatchType:       models.MatchTypeContains,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"body": "High priority"},
		Priority:        20,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(highRule).Error)

	// The higher priority rule should be returned (rules are ORDER BY priority DESC)
	resp, matched := app.matchKeywordRules(org.ID, account.Name, "this is a test")
	assert.True(t, matched)
	require.NotNil(t, resp)
	assert.Equal(t, "High priority", resp.Body)
}

func TestMatchKeywordRules_DisabledRuleIgnored(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "disabled",
		Keywords:        models.StringArray{"disabled"},
		MatchType:       models.MatchTypeExact,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{"body": "Should not match"},
		Priority:        10,
		IsEnabled:       true, // Create as enabled first
	}
	require.NoError(t, app.DB.Create(rule).Error)
	// Explicitly disable: GORM skips zero-value bools with default:true on INSERT.
	require.NoError(t, app.DB.Model(rule).Update("is_enabled", false).Error)

	_, matched := app.matchKeywordRules(org.ID, account.Name, "disabled")
	assert.False(t, matched)
}

func TestMatchKeywordRules_TransferType(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "agent",
		Keywords:        models.StringArray{"agent"},
		MatchType:       models.MatchTypeExact,
		ResponseType:    models.ResponseTypeTransfer,
		ResponseContent: models.JSONB{"body": "Connecting you to an agent..."},
		Priority:        10,
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(rule).Error)

	resp, matched := app.matchKeywordRules(org.ID, account.Name, "agent")
	assert.True(t, matched)
	require.NotNil(t, resp)
	assert.Equal(t, models.ResponseTypeTransfer, resp.ResponseType)
	assert.Equal(t, "Connecting you to an agent...", resp.Body)
}

func TestMatchKeywordRules_WithButtons(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	rule := &models.KeywordRule{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "menu",
		Keywords:        models.StringArray{"menu"},
		MatchType:       models.MatchTypeExact,
		ResponseType:    models.ResponseTypeText,
		ResponseContent: models.JSONB{
			"body": "Choose an option:",
			"buttons": []interface{}{
				map[string]interface{}{"id": "opt1", "title": "Option 1"},
				map[string]interface{}{"id": "opt2", "title": "Option 2"},
			},
		},
		Priority:  10,
		IsEnabled: true,
	}
	require.NoError(t, app.DB.Create(rule).Error)

	resp, matched := app.matchKeywordRules(org.ID, account.Name, "menu")
	assert.True(t, matched)
	require.NotNil(t, resp)
	assert.Equal(t, "Choose an option:", resp.Body)
	assert.Len(t, resp.Buttons, 2)
}

// =============================================================================
// getOrCreateSession
// =============================================================================

func TestGetOrCreateSession_NewSession(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	session, isNew := app.getOrCreateSession(org.ID, contact.ID, account.Name, contact.PhoneNumber, 30)
	assert.True(t, isNew)
	require.NotNil(t, session)
	assert.Equal(t, models.SessionStatusActive, session.Status)
	assert.Equal(t, org.ID, session.OrganizationID)
	assert.Equal(t, contact.ID, session.ContactID)
	assert.Equal(t, account.Name, session.WhatsAppAccount)

	// Verify it was persisted
	var dbSession models.ChatbotSession
	require.NoError(t, app.DB.First(&dbSession, session.ID).Error)
	assert.Equal(t, models.SessionStatusActive, dbSession.Status)
}

func TestGetOrCreateSession_ExistingSession(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Create an active session
	existing := models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusActive,
		SessionData:     models.JSONB{"key": "value"},
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
	}
	require.NoError(t, app.DB.Create(&existing).Error)

	session, isNew := app.getOrCreateSession(org.ID, contact.ID, account.Name, contact.PhoneNumber, 30)
	assert.False(t, isNew)
	require.NotNil(t, session)
	assert.Equal(t, existing.ID, session.ID)
}

func TestGetOrCreateSession_ExpiredSession(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Create an expired session (last activity 60 minutes ago, timeout is 30 minutes)
	expired := models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusActive,
		SessionData:     models.JSONB{},
		StartedAt:       time.Now().Add(-60 * time.Minute),
		LastActivityAt:  time.Now().Add(-60 * time.Minute),
	}
	require.NoError(t, app.DB.Create(&expired).Error)

	session, isNew := app.getOrCreateSession(org.ID, contact.ID, account.Name, contact.PhoneNumber, 30)
	assert.True(t, isNew)
	require.NotNil(t, session)
	assert.NotEqual(t, expired.ID, session.ID, "should create a new session, not return expired one")
}

// =============================================================================
// isWithinBusinessHours
// =============================================================================

func TestIsWithinBusinessHours_WithinHours(t *testing.T) {
	app := newProcessorTestApp(t)
	now := time.Now()
	dayOfWeek := float64(now.Weekday())

	hours := models.JSONBArray{
		map[string]interface{}{
			"day":        dayOfWeek,
			"enabled":    true,
			"start_time": "00:00",
			"end_time":   "23:59",
		},
	}

	result := app.isWithinBusinessHours(hours)
	assert.True(t, result)
}

func TestIsWithinBusinessHours_OutsideHours(t *testing.T) {
	app := newProcessorTestApp(t)
	now := time.Now()
	dayOfWeek := float64(now.Weekday())

	// Set hours to a time window that has definitely passed
	// Use a very narrow window in the past
	hours := models.JSONBArray{
		map[string]interface{}{
			"day":        dayOfWeek,
			"enabled":    true,
			"start_time": "00:00",
			"end_time":   "00:01",
		},
	}

	// This will only be true if running at midnight; for all practical purposes it tests false
	currentTime := now.Format("15:04")
	if currentTime > "00:01" {
		result := app.isWithinBusinessHours(hours)
		assert.False(t, result)
	}
}

func TestIsWithinBusinessHours_DayDisabled(t *testing.T) {
	app := newProcessorTestApp(t)
	now := time.Now()
	dayOfWeek := float64(now.Weekday())

	hours := models.JSONBArray{
		map[string]interface{}{
			"day":        dayOfWeek,
			"enabled":    false,
			"start_time": "00:00",
			"end_time":   "23:59",
		},
	}

	result := app.isWithinBusinessHours(hours)
	assert.False(t, result)
}

func TestIsWithinBusinessHours_NoMatchingDay(t *testing.T) {
	app := newProcessorTestApp(t)
	now := time.Now()
	// Use a different day of the week
	otherDay := float64((int(now.Weekday()) + 1) % 7)

	hours := models.JSONBArray{
		map[string]interface{}{
			"day":        otherDay,
			"enabled":    true,
			"start_time": "00:00",
			"end_time":   "23:59",
		},
	}

	result := app.isWithinBusinessHours(hours)
	assert.False(t, result)
}

func TestIsWithinBusinessHours_EmptyHours(t *testing.T) {
	app := newProcessorTestApp(t)

	result := app.isWithinBusinessHours(models.JSONBArray{})
	assert.False(t, result)
}

// =============================================================================
// shouldSkipStep
// =============================================================================

func TestShouldSkipStep_NoCondition(t *testing.T) {
	app := newProcessorTestApp(t)

	step := &models.ChatbotFlowStep{
		StepName:      "test_step",
		SkipCondition: "",
	}

	result := app.shouldSkipStep(step, map[string]interface{}{})
	assert.False(t, result)
}

func TestShouldSkipStep_ConditionTrue(t *testing.T) {
	app := newProcessorTestApp(t)

	step := &models.ChatbotFlowStep{
		StepName:      "test_step",
		SkipCondition: "status == 'vip'",
	}

	result := app.shouldSkipStep(step, map[string]interface{}{"status": "vip"})
	assert.True(t, result)
}

func TestShouldSkipStep_ConditionFalse(t *testing.T) {
	app := newProcessorTestApp(t)

	step := &models.ChatbotFlowStep{
		StepName:      "test_step",
		SkipCondition: "status == 'vip'",
	}

	result := app.shouldSkipStep(step, map[string]interface{}{"status": "regular"})
	assert.False(t, result)
}

func TestShouldSkipStep_ComplexConditionAND(t *testing.T) {
	app := newProcessorTestApp(t)

	step := &models.ChatbotFlowStep{
		StepName:      "test_step",
		SkipCondition: "status == 'vip' AND country == 'US'",
	}

	// Both conditions true
	result := app.shouldSkipStep(step, map[string]interface{}{
		"status":  "vip",
		"country": "US",
	})
	assert.True(t, result)

	// One condition false
	result2 := app.shouldSkipStep(step, map[string]interface{}{
		"status":  "vip",
		"country": "UK",
	})
	assert.False(t, result2)
}

func TestShouldSkipStep_ComplexConditionOR(t *testing.T) {
	app := newProcessorTestApp(t)

	step := &models.ChatbotFlowStep{
		StepName:      "test_step",
		SkipCondition: "status == 'vip' OR status == 'premium'",
	}

	result := app.shouldSkipStep(step, map[string]interface{}{"status": "premium"})
	assert.True(t, result)

	result2 := app.shouldSkipStep(step, map[string]interface{}{"status": "regular"})
	assert.False(t, result2)
}

func TestShouldSkipStep_MissingVariable(t *testing.T) {
	app := newProcessorTestApp(t)

	step := &models.ChatbotFlowStep{
		StepName:      "test_step",
		SkipCondition: "nonexistent == 'value'",
	}

	result := app.shouldSkipStep(step, map[string]interface{}{})
	assert.False(t, result)
}

// =============================================================================
// startFlow
// =============================================================================

func TestStartFlow_UpdatesSession(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	session := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusActive,
		SessionData:     models.JSONB{},
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
	}
	require.NoError(t, app.DB.Create(session).Error)

	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "Test Flow",
		IsEnabled:       true,
		Steps:           []models.ChatbotFlowStep{},
	}
	require.NoError(t, app.DB.Create(flow).Error)

	// startFlow with no steps should call completeFlow
	app.startFlow(account, session, contact, flow)

	// Verify session was updated: since there are no steps, completeFlow is called
	var dbSession models.ChatbotSession
	require.NoError(t, app.DB.First(&dbSession, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, dbSession.Status)
}

func TestStartFlow_WithSteps(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	session := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusActive,
		SessionData:     models.JSONB{},
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
	}
	require.NoError(t, app.DB.Create(session).Error)

	flowID := uuid.New()
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: flowID},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "Flow With Steps",
		InitialMessage:  "Welcome to the flow!",
		IsEnabled:       true,
		Steps: []models.ChatbotFlowStep{
			{
				BaseModel:   models.BaseModel{ID: uuid.New()},
				FlowID:      flowID,
				StepName:    "step1",
				StepOrder:   1,
				Message:     "What is your name?",
				MessageType: models.FlowStepTypeText,
				InputType:   models.InputTypeText,
				StoreAs:     "name",
			},
		},
	}
	// Flow must exist in DB for the session FK constraint on CurrentFlowID.
	require.NoError(t, app.DB.Create(flow).Error)

	// startFlow sets current_flow_id and current_step on the session
	app.startFlow(account, session, contact, flow)

	var dbSession models.ChatbotSession
	require.NoError(t, app.DB.First(&dbSession, session.ID).Error)
	assert.NotNil(t, dbSession.CurrentFlowID)
	assert.Equal(t, flowID, *dbSession.CurrentFlowID)
	assert.Equal(t, "step1", dbSession.CurrentStep)
	assert.Equal(t, models.SessionStatusActive, dbSession.Status)
}

// =============================================================================
// completeFlow
// =============================================================================

func TestCompleteFlow_UpdatesSession(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	flow := &models.ChatbotFlow{
		BaseModel:         models.BaseModel{ID: uuid.New()},
		OrganizationID:    org.ID,
		WhatsAppAccount:   account.Name,
		Name:              "Test Flow",
		CompletionMessage: "Thank you {{name}}!",
		IsEnabled:         true,
	}
	require.NoError(t, app.DB.Create(flow).Error)

	session := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusActive,
		CurrentFlowID:   &flow.ID,
		CurrentStep:     "step1",
		SessionData:     models.JSONB{"name": "John"},
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
	}
	require.NoError(t, app.DB.Create(session).Error)

	app.completeFlow(account, session, contact, flow)

	// Session should be completed
	var dbSession models.ChatbotSession
	require.NoError(t, app.DB.First(&dbSession, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, dbSession.Status)
	assert.Equal(t, "", dbSession.CurrentStep)
	assert.NotNil(t, dbSession.CompletedAt)
}

// =============================================================================
// exitFlow
// =============================================================================

func TestExitFlow_UpdatesSession(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "Exit Test Flow",
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(flow).Error)

	session := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusActive,
		CurrentFlowID:   &flow.ID,
		CurrentStep:     "step2",
		StepRetries:     2,
		SessionData:     models.JSONB{},
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
	}
	require.NoError(t, app.DB.Create(session).Error)

	app.exitFlow(session)

	var dbSession models.ChatbotSession
	require.NoError(t, app.DB.First(&dbSession, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, dbSession.Status)
	assert.Equal(t, "", dbSession.CurrentStep)
	assert.Equal(t, 0, dbSession.StepRetries)
	assert.NotNil(t, dbSession.CompletedAt)
}

// =============================================================================
// saveIncomingMessage
// =============================================================================

func TestSaveIncomingMessage_TextMessage(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	waMsgID := "wamid." + uuid.New().String()[:16]
	app.saveIncomingMessage(account, contact, waMsgID, "text", "Hello from test", nil, "")

	// Verify message was saved
	var msg models.Message
	require.NoError(t, app.DB.Where("whats_app_message_id = ?", waMsgID).First(&msg).Error)
	assert.Equal(t, models.DirectionIncoming, msg.Direction)
	assert.Equal(t, models.MessageTypeText, msg.MessageType)
	assert.Equal(t, "Hello from test", msg.Content)
	assert.Equal(t, contact.ID, msg.ContactID)
	assert.Equal(t, account.Name, msg.WhatsAppAccount)
	assert.Equal(t, models.MessageStatusReceived, msg.Status)

	// Verify contact was updated
	var dbContact models.Contact
	require.NoError(t, app.DB.First(&dbContact, contact.ID).Error)
	assert.NotNil(t, dbContact.LastMessageAt)
	assert.Equal(t, "Hello from test", dbContact.LastMessagePreview)
	assert.False(t, dbContact.IsRead)
}

func TestSaveIncomingMessage_WithMedia(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	waMsgID := "wamid." + uuid.New().String()[:16]
	media := &MediaInfo{
		MediaURL:      "/uploads/test-image.jpg",
		MediaMimeType: "image/jpeg",
		MediaFilename: "photo.jpg",
	}
	app.saveIncomingMessage(account, contact, waMsgID, "image", "Look at this", media, "")

	var msg models.Message
	require.NoError(t, app.DB.Where("whats_app_message_id = ?", waMsgID).First(&msg).Error)
	assert.Equal(t, "/uploads/test-image.jpg", msg.MediaURL)
	assert.Equal(t, "image/jpeg", msg.MediaMimeType)
	assert.Equal(t, "photo.jpg", msg.MediaFilename)

	// Non-text messages show type in preview
	var dbContact models.Contact
	require.NoError(t, app.DB.First(&dbContact, contact.ID).Error)
	assert.Equal(t, "[image]", dbContact.LastMessagePreview)
}

func TestSaveIncomingMessage_WithReplyContext(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Create original message to reply to
	originalWAMID := "wamid.original_" + uuid.New().String()[:8]
	originalMsg := models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New()},
		OrganizationID:    org.ID,
		WhatsAppAccount:   account.Name,
		ContactID:         contact.ID,
		WhatsAppMessageID: originalWAMID,
		Direction:         models.DirectionOutgoing,
		MessageType:       models.MessageTypeText,
		Content:           "Original message",
		Status:            models.MessageStatusReceived,
	}
	require.NoError(t, app.DB.Create(&originalMsg).Error)

	// Save reply message
	replyWAMID := "wamid.reply_" + uuid.New().String()[:8]
	app.saveIncomingMessage(account, contact, replyWAMID, "text", "Reply to your message", nil, originalWAMID)

	var replyMsg models.Message
	require.NoError(t, app.DB.Where("whats_app_message_id = ?", replyWAMID).First(&replyMsg).Error)
	assert.True(t, replyMsg.IsReply)
	require.NotNil(t, replyMsg.ReplyToMessageID)
	assert.Equal(t, originalMsg.ID, *replyMsg.ReplyToMessageID)
}

func TestSaveIncomingMessage_LongContent(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Create a message with content longer than 100 characters
	longContent := ""
	for i := 0; i < 120; i++ {
		longContent += "x"
	}
	waMsgID := "wamid." + uuid.New().String()[:16]
	app.saveIncomingMessage(account, contact, waMsgID, "text", longContent, nil, "")

	var dbContact models.Contact
	require.NoError(t, app.DB.First(&dbContact, contact.ID).Error)
	// Preview should be truncated to 97 chars + "..."
	assert.Len(t, dbContact.LastMessagePreview, 100)
	assert.True(t, len(dbContact.LastMessagePreview) <= 100)
}

// =============================================================================
// replaceVariables
// =============================================================================

func TestReplaceVariables_Basic(t *testing.T) {
	app := newProcessorTestApp(t)

	result := app.replaceVariables("Hello {{name}}, your order is {{order_id}}", models.JSONB{
		"name":     "John",
		"order_id": "12345",
	})
	assert.Equal(t, "Hello John, your order is 12345", result)
}

func TestReplaceVariables_NilData(t *testing.T) {
	app := newProcessorTestApp(t)

	result := app.replaceVariables("Hello {{name}}", nil)
	assert.Equal(t, "Hello {{name}}", result)
}

func TestReplaceVariables_MissingVariable(t *testing.T) {
	app := newProcessorTestApp(t)

	result := app.replaceVariables("Hello {{name}}", models.JSONB{})
	assert.Equal(t, "Hello {{name}}", result)
}

// =============================================================================
// logSessionMessage
// =============================================================================

func TestLogSessionMessage(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	session := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.SessionStatusActive,
		SessionData:     models.JSONB{},
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
	}
	require.NoError(t, app.DB.Create(session).Error)

	app.logSessionMessage(session.ID, models.DirectionIncoming, "test message", "greeting")

	var msgs []models.ChatbotSessionMessage
	require.NoError(t, app.DB.Where("session_id = ?", session.ID).Find(&msgs).Error)
	require.Len(t, msgs, 1)
	assert.Equal(t, "test message", msgs[0].Message)
	assert.Equal(t, "greeting", msgs[0].StepName)
	assert.Equal(t, models.DirectionIncoming, msgs[0].Direction)
}

// =============================================================================
// matchFlowTrigger
// =============================================================================

func TestMatchFlowTrigger_Match(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "Order Flow",
		TriggerKeywords: models.StringArray{"order", "buy"},
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(flow).Error)

	result := app.matchFlowTrigger(org.ID, account.Name, "I want to order")
	require.NotNil(t, result)
	assert.Equal(t, flow.ID, result.ID)

	// No match
	noMatch := app.matchFlowTrigger(org.ID, account.Name, "hello there")
	assert.Nil(t, noMatch)
}

// =============================================================================
// evaluateExpression (package-level, not on App)
// =============================================================================

func TestEvaluateExpression_SimpleEquality(t *testing.T) {
	assert.True(t, evaluateExpression("status == 'active'", map[string]interface{}{"status": "active"}))
	assert.False(t, evaluateExpression("status == 'active'", map[string]interface{}{"status": "inactive"}))
}

func TestEvaluateExpression_NotEquals(t *testing.T) {
	assert.True(t, evaluateExpression("status != 'inactive'", map[string]interface{}{"status": "active"}))
	assert.False(t, evaluateExpression("status != 'active'", map[string]interface{}{"status": "active"}))
}

func TestEvaluateExpression_ANDOperator(t *testing.T) {
	data := map[string]interface{}{"a": "1", "b": "2"}
	assert.True(t, evaluateExpression("a == '1' AND b == '2'", data))
	assert.False(t, evaluateExpression("a == '1' AND b == '3'", data))
}

func TestEvaluateExpression_OROperator(t *testing.T) {
	data := map[string]interface{}{"a": "1", "b": "2"}
	assert.True(t, evaluateExpression("a == '1' OR b == '3'", data))
	assert.True(t, evaluateExpression("a == '9' OR b == '2'", data))
	assert.False(t, evaluateExpression("a == '9' OR b == '9'", data))
}

func TestEvaluateExpression_Parentheses(t *testing.T) {
	data := map[string]interface{}{"a": "1", "b": "2", "c": "3"}
	assert.True(t, evaluateExpression("(a == '1' OR b == '9') AND c == '3'", data))
	assert.False(t, evaluateExpression("(a == '9' OR b == '9') AND c == '3'", data))
}

func TestEvaluateExpression_EmptyExpression(t *testing.T) {
	assert.False(t, evaluateExpression("", map[string]interface{}{}))
}
