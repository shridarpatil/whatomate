package handlers_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// --- Helper functions ---

// createTestMessage creates a message in the database for analytics testing.
func createTestMessage(t *testing.T, app *handlers.App, orgID, contactID uuid.UUID, direction models.Direction, createdAt time.Time) *models.Message {
	t.Helper()

	msg := &models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New(), CreatedAt: createdAt},
		OrganizationID:  orgID,
		ContactID:       contactID,
		WhatsAppAccount: "test-account",
		Direction:       direction,
		MessageType:     models.MessageTypeText,
		Content:         "Test message",
		Status:          models.MessageStatusSent,
	}
	require.NoError(t, app.DB.Create(msg).Error)
	return msg
}

// createTestChatbotSession creates a chatbot session in the database.
func createTestChatbotSession(t *testing.T, app *handlers.App, orgID, contactID uuid.UUID, createdAt time.Time) *models.ChatbotSession {
	t.Helper()

	session := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New(), CreatedAt: createdAt},
		OrganizationID:  orgID,
		ContactID:       contactID,
		WhatsAppAccount: "test-account",
		PhoneNumber:     "+1234567890",
		Status:          "active",
		StartedAt:       createdAt,
		LastActivityAt:  createdAt,
	}
	require.NoError(t, app.DB.Create(session).Error)
	return session
}

// createAnalyticsTestCampaign creates a bulk message campaign with a specific creation time.
func createAnalyticsTestCampaign(t *testing.T, app *handlers.App, orgID, createdBy uuid.UUID, status string, createdAt time.Time) *models.BulkMessageCampaign {
	t.Helper()

	templateID := uuid.New()
	// Create a minimal template for the foreign key
	tmpl := &models.Template{
		BaseModel:       models.BaseModel{ID: templateID},
		OrganizationID:  orgID,
		WhatsAppAccount: "test-account",
		Name:            "campaign-template-" + uuid.New().String()[:8],
		MetaTemplateID:  "meta-" + uuid.New().String()[:8],
		Category:        "MARKETING",
		Language:        "en",
		Status:          string(models.TemplateStatusApproved),
		BodyContent:     "Hello",
	}
	require.NoError(t, app.DB.Create(tmpl).Error)

	campaign := &models.BulkMessageCampaign{
		BaseModel:       models.BaseModel{ID: uuid.New(), CreatedAt: createdAt},
		OrganizationID:  orgID,
		WhatsAppAccount: "test-account",
		Name:            "Test Campaign " + uuid.New().String()[:8],
		TemplateID:      templateID,
		Status:          models.CampaignStatus(status),
		CreatedBy:       createdBy,
	}
	require.NoError(t, app.DB.Create(campaign).Error)
	return campaign
}

// createTestAgentTransfer creates an agent transfer in the database.
func createTestAgentTransfer(t *testing.T, app *handlers.App, orgID, contactID uuid.UUID, agentID *uuid.UUID, status models.TransferStatus, source models.TransferSource, transferredAt time.Time, resumedAt *time.Time) *models.AgentTransfer {
	t.Helper()

	transfer := &models.AgentTransfer{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		ContactID:       contactID,
		WhatsAppAccount: "test-account",
		PhoneNumber:     "+1234567890",
		Status:          status,
		Source:          source,
		AgentID:         agentID,
		TransferredAt:   transferredAt,
		ResumedAt:       resumedAt,
	}
	require.NoError(t, app.DB.Create(transfer).Error)
	return transfer
}

// createTestTeamWithAgent creates a team and adds the user as an agent member.
func createTestTeamWithAgent(t *testing.T, app *handlers.App, orgID, userID uuid.UUID) (*models.Team, *models.TeamMember) {
	t.Helper()

	team := &models.Team{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           "Test Team " + uuid.New().String()[:8],
		IsActive:       true,
	}
	require.NoError(t, app.DB.Create(team).Error)

	member := &models.TeamMember{
		BaseModel: models.BaseModel{ID: uuid.New()},
		TeamID:    team.ID,
		UserID:    userID,
		Role:      models.TeamRoleAgent,
	}
	require.NoError(t, app.DB.Create(member).Error)

	return team, member
}

// --- GetDashboardStats Tests ---

func TestApp_GetDashboardStats_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("dash-stats")), testutil.WithPassword("password"))

	// Create test data within the current month
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	now := time.Now().UTC()
	createTestMessage(t, app, org.ID, contact.ID, models.DirectionIncoming, now.Add(-1*time.Hour))
	createTestMessage(t, app, org.ID, contact.ID, models.DirectionOutgoing, now.Add(-30*time.Minute))
	createTestChatbotSession(t, app, org.ID, contact.ID, now.Add(-2*time.Hour))
	createAnalyticsTestCampaign(t, app, org.ID, user.ID, "completed", now.Add(-3*time.Hour))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetDashboardStats(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Stats          handlers.DashboardStats            `json:"stats"`
			RecentMessages []handlers.RecentMessageResponse   `json:"recent_messages"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, int64(2), resp.Data.Stats.TotalMessages)
	assert.Equal(t, int64(1), resp.Data.Stats.ChatbotSessions)
	assert.Equal(t, int64(1), resp.Data.Stats.CampaignsSent)
	assert.Len(t, resp.Data.RecentMessages, 2)
}

func TestApp_GetDashboardStats_WithDateFilters(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("dash-date")), testutil.WithPassword("password"))

	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Create messages in January 2025
	jan15 := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	jan20 := time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC)
	// Create a message outside the date range
	feb10 := time.Date(2025, 2, 10, 12, 0, 0, 0, time.UTC)

	createTestMessage(t, app, org.ID, contact.ID, models.DirectionIncoming, jan15)
	createTestMessage(t, app, org.ID, contact.ID, models.DirectionOutgoing, jan20)
	createTestMessage(t, app, org.ID, contact.ID, models.DirectionIncoming, feb10)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "from", "2025-01-01")
	testutil.SetQueryParam(req, "to", "2025-01-31")

	err := app.GetDashboardStats(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Stats handlers.DashboardStats `json:"stats"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	// Only 2 messages fall within the Jan 1-31 range
	assert.Equal(t, int64(2), resp.Data.Stats.TotalMessages)
}

func TestApp_GetDashboardStats_InvalidFromDate(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("dash-bad-from")), testutil.WithPassword("password"))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "from", "not-a-date")
	testutil.SetQueryParam(req, "to", "2025-01-31")

	err := app.GetDashboardStats(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetDashboardStats_InvalidToDate(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("dash-bad-to")), testutil.WithPassword("password"))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "from", "2025-01-01")
	testutil.SetQueryParam(req, "to", "invalid")

	err := app.GetDashboardStats(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetDashboardStats_Unauthorized(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context set

	err := app.GetDashboardStats(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

func TestApp_GetDashboardStats_EmptyData(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithEmail(testutil.UniqueEmail("dash-empty")), testutil.WithPassword("password"))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetDashboardStats(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Stats          handlers.DashboardStats          `json:"stats"`
			RecentMessages []handlers.RecentMessageResponse `json:"recent_messages"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, int64(0), resp.Data.Stats.TotalMessages)
	assert.Equal(t, int64(0), resp.Data.Stats.TotalContacts)
	assert.Equal(t, int64(0), resp.Data.Stats.ChatbotSessions)
	assert.Equal(t, int64(0), resp.Data.Stats.CampaignsSent)
	assert.Equal(t, float64(0), resp.Data.Stats.MessagesChange)
	assert.Empty(t, resp.Data.RecentMessages)
}

// --- GetAgentAnalytics Tests ---

func TestApp_GetAgentAnalytics_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics Agent", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("agent-analytics")),
		testutil.WithPassword("password"),
		testutil.WithRoleID(&role.ID),
	)

	// Create a team and add user as agent so calculateAllAgentStats finds them
	createTestTeamWithAgent(t, app, org.ID, user.ID)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	now := time.Now().UTC()
	resumedAt := now.Add(-30 * time.Minute)

	createTestAgentTransfer(t, app, org.ID, contact.ID, &user.ID,
		models.TransferStatusResumed, models.TransferSourceManual,
		now.Add(-2*time.Hour), &resumedAt)
	createTestAgentTransfer(t, app, org.ID, contact.ID, &user.ID,
		models.TransferStatusActive, models.TransferSourceFlow,
		now.Add(-1*time.Hour), nil)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetAgentAnalytics(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.AgentAnalyticsResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	// User has analytics permission so sees summary + all agent stats + my_stats
	assert.NotNil(t, resp.Data.MyStats)
	assert.NotNil(t, resp.Data.AgentStats)
}

func TestApp_GetAgentAnalytics_EmptyData(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics Empty", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("agent-empty")),
		testutil.WithPassword("password"),
		testutil.WithRoleID(&role.ID),
	)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetAgentAnalytics(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.AgentAnalyticsResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, int64(0), resp.Data.Summary.TotalTransfersHandled)
	assert.Equal(t, int64(0), resp.Data.Summary.ActiveTransfers)
	assert.NotNil(t, resp.Data.TrendData)
	assert.Empty(t, resp.Data.TrendData)
}

func TestApp_GetAgentAnalytics_Unauthorized(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context

	err := app.GetAgentAnalytics(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

func TestApp_GetAgentAnalytics_AgentSeesOwnStats(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	// Create a user without analytics permission (agent-level user)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("agent-own")),
		testutil.WithPassword("password"),
	)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	now := time.Now().UTC()
	resumedAt := now.Add(-10 * time.Minute)

	createTestAgentTransfer(t, app, org.ID, contact.ID, &user.ID,
		models.TransferStatusResumed, models.TransferSourceManual,
		now.Add(-1*time.Hour), &resumedAt)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetAgentAnalytics(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data handlers.AgentAnalyticsResponse `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	// User without analytics permission sees only their own stats
	assert.NotNil(t, resp.Data.MyStats)
	assert.Nil(t, resp.Data.AgentStats)
	assert.Equal(t, user.ID.String(), resp.Data.MyStats.AgentID)
}

// --- GetAgentDetails Tests ---

func TestApp_GetAgentDetails_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics Detail", false, false, perms)
	adminUser := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("detail-admin")),
		testutil.WithPassword("password"),
		testutil.WithRoleID(&role.ID),
	)

	agentUser := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("detail-agent")),
		testutil.WithPassword("password"),
		testutil.WithFullName("Agent Smith"),
	)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	now := time.Now().UTC()
	resumedAt := now.Add(-15 * time.Minute)

	createTestAgentTransfer(t, app, org.ID, contact.ID, &agentUser.ID,
		models.TransferStatusResumed, models.TransferSourceManual,
		now.Add(-1*time.Hour), &resumedAt)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, adminUser.ID)
	testutil.SetPathParam(req, "id", agentUser.ID.String())

	err := app.GetAgentDetails(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Agent     handlers.AgentPerformanceStats `json:"agent"`
			TrendData []handlers.TrendPoint          `json:"trend_data"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, agentUser.ID.String(), resp.Data.Agent.AgentID)
	assert.Equal(t, "Agent Smith", resp.Data.Agent.AgentName)
	assert.Equal(t, int64(1), resp.Data.Agent.TransfersHandled)
}

func TestApp_GetAgentDetails_NotFound(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics NotFound", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("detail-notfound")),
		testutil.WithPassword("password"),
		testutil.WithRoleID(&role.ID),
	)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetAgentDetails(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

func TestApp_GetAgentDetails_InvalidID(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics InvalidID", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("detail-invalid")),
		testutil.WithPassword("password"),
		testutil.WithRoleID(&role.ID),
	)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", "not-a-uuid")

	err := app.GetAgentDetails(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestApp_GetAgentDetails_NoPermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// User without analytics permission
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("detail-noperm")),
		testutil.WithPassword("password"),
	)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetAgentDetails(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_GetAgentDetails_Unauthorized(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetAgentDetails(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

// --- GetAgentComparison Tests ---

func TestApp_GetAgentComparison_Success(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics Compare", false, false, perms)
	adminUser := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("compare-admin")),
		testutil.WithPassword("password"),
		testutil.WithRoleID(&role.ID),
	)

	agent1 := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("compare-agent1")),
		testutil.WithPassword("password"),
		testutil.WithFullName("Agent One"),
	)
	agent2 := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("compare-agent2")),
		testutil.WithPassword("password"),
		testutil.WithFullName("Agent Two"),
	)

	// Add agents to a team so calculateAllAgentStats can find them
	createTestTeamWithAgent(t, app, org.ID, agent1.ID)
	createTestTeamWithAgent(t, app, org.ID, agent2.ID)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	now := time.Now().UTC()
	resumedAt := now.Add(-20 * time.Minute)

	createTestAgentTransfer(t, app, org.ID, contact.ID, &agent1.ID,
		models.TransferStatusResumed, models.TransferSourceManual,
		now.Add(-2*time.Hour), &resumedAt)
	createTestAgentTransfer(t, app, org.ID, contact.ID, &agent2.ID,
		models.TransferStatusResumed, models.TransferSourceFlow,
		now.Add(-1*time.Hour), &resumedAt)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, adminUser.ID)

	err := app.GetAgentComparison(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Agents []handlers.AgentPerformanceStats `json:"agents"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Len(t, resp.Data.Agents, 2)
}

func TestApp_GetAgentComparison_NoPermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// User without analytics permission
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("compare-noperm")),
		testutil.WithPassword("password"),
	)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetAgentComparison(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestApp_GetAgentComparison_Unauthorized(t *testing.T) {
	app := newTestApp(t)

	req := testutil.NewGETRequest(t)
	// No auth context

	err := app.GetAgentComparison(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, testutil.GetResponseStatusCode(req))
}

func TestApp_GetAgentComparison_EmptyAgents(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	perms := getAnalyticsPermissions(t, app)
	role := testutil.CreateTestRoleExact(t, app.DB, org.ID, "Analytics EmptyCmp", false, false, perms)
	user := testutil.CreateTestUser(t, app.DB, org.ID,
		testutil.WithEmail(testutil.UniqueEmail("compare-empty")),
		testutil.WithPassword("password"),
		testutil.WithRoleID(&role.ID),
	)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetAgentComparison(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Agents []handlers.AgentPerformanceStats `json:"agents"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Empty(t, resp.Data.Agents)
}
