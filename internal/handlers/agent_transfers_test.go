package handlers_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// agentTransfersTestApp creates an App instance for agent transfers testing.
// It uses a transaction that gets rolled back after the test for isolation.
func agentTransfersTestApp(t *testing.T) *handlers.App {
	t.Helper()

	db := testutil.SetupTestDB(t)
	log := testutil.NopLogger()
	redis := testutil.SetupTestRedis(t)

	// Start a transaction for test isolation
	tx := db.Begin()
	t.Cleanup(func() {
		tx.Rollback()
	})

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:            testJWTSecret,
			AccessExpiryMins:  15,
			RefreshExpiryDays: 7,
		},
	}

	return &handlers.App{
		Config: cfg,
		DB:     tx, // Use transaction instead of raw db
		Log:    log,
		Redis:  redis,
	}
}

// createTestContact creates a test contact in the database.
func createTestContact(t *testing.T, app *handlers.App, orgID uuid.UUID) *models.Contact {
	t.Helper()

	uniqueID := uuid.New().String()[:8]
	contact := &models.Contact{
		OrganizationID: orgID,
		PhoneNumber:    "1234567890" + uniqueID[:4],
		ProfileName:    "Test Contact " + uniqueID,
	}
	require.NoError(t, app.DB.Create(contact).Error)
	return contact
}

// createTestAgent creates a test agent user in the database.
func createTestAgent(t *testing.T, app *handlers.App, orgID uuid.UUID) *models.User {
	t.Helper()

	uniqueID := uuid.New().String()[:8]
	agent := &models.User{
		OrganizationID: orgID,
		Email:          "agent-" + uniqueID + "@example.com",
		PasswordHash:   "hashed",
		FullName:       "Test Agent " + uniqueID,
		Role:           "agent",
		IsActive:       true,
		IsAvailable:    true,
	}
	require.NoError(t, app.DB.Create(agent).Error)
	return agent
}

// createTestTransfer creates a test agent transfer in the database.
func createTestTransfer(t *testing.T, app *handlers.App, orgID, contactID uuid.UUID, accountName, status string, agentID *uuid.UUID) *models.AgentTransfer {
	t.Helper()

	transfer := &models.AgentTransfer{
		OrganizationID:  orgID,
		ContactID:       contactID,
		WhatsAppAccount: accountName,
		PhoneNumber:     "1234567890",
		Status:          status,
		Source:          "manual",
		AgentID:         agentID,
		TransferredAt:   time.Now(),
	}
	require.NoError(t, app.DB.Create(transfer).Error)
	return transfer
}

// createTestTeam creates a test team with optional members.
func createTestTeam(t *testing.T, app *handlers.App, orgID uuid.UUID, memberIDs ...uuid.UUID) *models.Team {
	t.Helper()

	uniqueID := uuid.New().String()[:8]
	team := &models.Team{
		OrganizationID:     orgID,
		Name:               "Test Team " + uniqueID,
		IsActive:           true,
		AssignmentStrategy: "round_robin",
	}
	require.NoError(t, app.DB.Create(team).Error)

	for _, memberID := range memberIDs {
		member := &models.TeamMember{
			TeamID: team.ID,
			UserID: memberID,
			Role:   "agent",
		}
		require.NoError(t, app.DB.Create(member).Error)
	}

	return team
}

// setTransferAuthContext sets organization, user, and role in request context.
func setTransferAuthContext(req *fastglue.Request, orgID, userID uuid.UUID, role string) {
	req.RequestCtx.SetUserValue("organization_id", orgID)
	req.RequestCtx.SetUserValue("user_id", userID)
	req.RequestCtx.SetUserValue("role", role)
}

// --- ListAgentTransfers Tests ---

func TestApp_ListAgentTransfers_Success(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("list-transfers"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "list-transfers-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)

	// Create some transfers
	transfer1 := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)
	_ = createTestTransfer(t, app, org.ID, contact.ID, account.Name, "resumed", &agent.ID)

	req := testutil.NewGETRequest(t)
	setTransferAuthContext(req, org.ID, user.ID, "admin")

	err := app.ListAgentTransfers(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Transfers         []handlers.AgentTransferResponse `json:"transfers"`
			GeneralQueueCount int64                            `json:"general_queue_count"`
			TotalCount        int64                            `json:"total_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	assert.Equal(t, "success", result.Status)
	assert.Equal(t, int64(2), result.Data.TotalCount)
	assert.Len(t, result.Data.Transfers, 2)

	// First transfer should be the active unassigned one (FIFO)
	assert.Equal(t, transfer1.ID.String(), result.Data.Transfers[0].ID)
	assert.Equal(t, "active", result.Data.Transfers[0].Status)
}

func TestApp_ListAgentTransfers_FilterByStatus(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("filter-status"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "filter-status-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)

	// Create transfers with different statuses
	_ = createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)
	_ = createTestTransfer(t, app, org.ID, contact.ID, account.Name, "resumed", &agent.ID)

	req := testutil.NewGETRequest(t)
	setTransferAuthContext(req, org.ID, user.ID, "admin")
	testutil.SetQueryParam(req, "status", "active")

	err := app.ListAgentTransfers(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Transfers  []handlers.AgentTransferResponse `json:"transfers"`
			TotalCount int64                            `json:"total_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	assert.Equal(t, "success", result.Status)
	assert.Len(t, result.Data.Transfers, 1)
	assert.Equal(t, "active", result.Data.Transfers[0].Status)
}

func TestApp_ListAgentTransfers_AgentRoleFiltering(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	account := createTestWhatsAppAccount(t, app, org.ID, "agent-filter-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)

	// Create another agent
	otherAgent := createTestAgent(t, app, org.ID)

	// Create transfers: one assigned to agent, one to other agent, one unassigned
	_ = createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", &agent.ID)
	_ = createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", &otherAgent.ID)
	_ = createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil) // Unassigned (general queue)

	// Agent should only see their assigned transfers + general queue
	req := testutil.NewGETRequest(t)
	setTransferAuthContext(req, org.ID, agent.ID, "agent")

	err := app.ListAgentTransfers(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Transfers  []handlers.AgentTransferResponse `json:"transfers"`
			TotalCount int64                            `json:"total_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	// Agent sees their transfer + general queue (2), not the other agent's transfer
	assert.Equal(t, int64(2), result.Data.TotalCount)
}

func TestApp_ListAgentTransfers_Pagination(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("pagination"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "pagination-account")

	contact := createTestContact(t, app, org.ID)

	// Create multiple transfers
	for i := 0; i < 5; i++ {
		createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)
	}

	// Request with limit and offset
	req := testutil.NewGETRequest(t)
	setTransferAuthContext(req, org.ID, user.ID, "admin")
	testutil.SetQueryParam(req, "limit", "2")
	testutil.SetQueryParam(req, "offset", "1")

	err := app.ListAgentTransfers(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Transfers  []handlers.AgentTransferResponse `json:"transfers"`
			TotalCount int64                            `json:"total_count"`
			Limit      int                              `json:"limit"`
			Offset     int                              `json:"offset"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	assert.Equal(t, int64(5), result.Data.TotalCount)
	assert.Len(t, result.Data.Transfers, 2)
	assert.Equal(t, 2, result.Data.Limit)
	assert.Equal(t, 1, result.Data.Offset)
}

// --- CreateAgentTransfer Tests ---

func TestApp_CreateAgentTransfer_Success(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-transfer"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "create-transfer-account")

	contact := createTestContact(t, app, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id":       contact.ID.String(),
		"whatsapp_account": account.Name,
		"notes":            "Test transfer",
		"source":           "manual",
	})
	setTransferAuthContext(req, org.ID, user.ID, "admin")

	err := app.CreateAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Transfer handlers.AgentTransferResponse `json:"transfer"`
			Message  string                         `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "Transfer created successfully", result.Data.Message)
	assert.Equal(t, contact.ID.String(), result.Data.Transfer.ContactID)
	assert.Equal(t, "active", result.Data.Transfer.Status)
	assert.Equal(t, "manual", result.Data.Transfer.Source)
}

func TestApp_CreateAgentTransfer_WithAgent(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("create-with-agent"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "create-with-agent-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id":       contact.ID.String(),
		"whatsapp_account": account.Name,
		"agent_id":         agent.ID.String(),
		"notes":            "Assigned to specific agent",
	})
	setTransferAuthContext(req, org.ID, user.ID, "admin")

	err := app.CreateAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Transfer handlers.AgentTransferResponse `json:"transfer"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	assert.Equal(t, "success", result.Status)
	assert.NotNil(t, result.Data.Transfer.AgentID)
	assert.Equal(t, agent.ID.String(), *result.Data.Transfer.AgentID)
}

func TestApp_CreateAgentTransfer_ContactNotFound(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("contact-not-found"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "contact-not-found-account")

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id":       uuid.New().String(), // Non-existent contact
		"whatsapp_account": account.Name,
	})
	setTransferAuthContext(req, org.ID, user.ID, "admin")

	err := app.CreateAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	var result map[string]any
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))
	assert.Equal(t, "Contact not found", result["message"])
}

func TestApp_CreateAgentTransfer_DuplicateTransfer(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("duplicate-transfer"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "duplicate-transfer-account")

	contact := createTestContact(t, app, org.ID)

	// Create an existing active transfer
	createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id":       contact.ID.String(),
		"whatsapp_account": account.Name,
	})
	setTransferAuthContext(req, org.ID, user.ID, "admin")

	err := app.CreateAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))

	var result map[string]any
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))
	assert.Equal(t, "Contact already has an active transfer", result["message"])
}

func TestApp_CreateAgentTransfer_MissingContactID(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("missing-contact-id"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "missing-contact-id-account")

	req := testutil.NewJSONRequest(t, map[string]any{
		"whatsapp_account": account.Name,
	})
	setTransferAuthContext(req, org.ID, user.ID, "admin")

	err := app.CreateAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	var result map[string]any
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))
	assert.Equal(t, "contact_id is required", result["message"])
}

func TestApp_CreateAgentTransfer_AgentUnavailable(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("agent-unavailable"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "agent-unavailable-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)

	// Make agent unavailable
	require.NoError(t, app.DB.Model(agent).Update("is_available", false).Error)

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id":       contact.ID.String(),
		"whatsapp_account": account.Name,
		"agent_id":         agent.ID.String(),
	})
	setTransferAuthContext(req, org.ID, user.ID, "admin")

	err := app.CreateAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	var result map[string]any
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))
	assert.Equal(t, "Agent is currently away", result["message"])
}

// --- ResumeFromTransfer Tests ---

func TestApp_ResumeFromTransfer_Success(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("resume-success"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "resume-success-account")

	contact := createTestContact(t, app, org.ID)
	transfer := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)

	req := testutil.NewJSONRequest(t, nil)
	setTransferAuthContext(req, org.ID, user.ID, "admin")
	testutil.SetPathParam(req, "id", transfer.ID.String())

	err := app.ResumeFromTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	assert.Equal(t, "success", result.Status)
	assert.Contains(t, result.Data.Message, "resumed")

	// Verify transfer status updated
	var updatedTransfer models.AgentTransfer
	require.NoError(t, app.DB.First(&updatedTransfer, transfer.ID).Error)
	assert.Equal(t, "resumed", updatedTransfer.Status)
	assert.NotNil(t, updatedTransfer.ResumedAt)
	assert.Equal(t, user.ID, *updatedTransfer.ResumedBy)
}

func TestApp_ResumeFromTransfer_NotFound(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("resume-not-found"), "password", "admin", true)

	req := testutil.NewJSONRequest(t, nil)
	setTransferAuthContext(req, org.ID, user.ID, "admin")
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.ResumeFromTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	var result map[string]any
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))
	assert.Equal(t, "Transfer not found", result["message"])
}

func TestApp_ResumeFromTransfer_NotActive(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("resume-not-active"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "resume-not-active-account")

	contact := createTestContact(t, app, org.ID)
	transfer := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "resumed", nil) // Already resumed

	req := testutil.NewJSONRequest(t, nil)
	setTransferAuthContext(req, org.ID, user.ID, "admin")
	testutil.SetPathParam(req, "id", transfer.ID.String())

	err := app.ResumeFromTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	var result map[string]any
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))
	assert.Equal(t, "Transfer is not active", result["message"])
}

// --- AssignAgentTransfer Tests ---

func TestApp_AssignAgentTransfer_Success(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("assign-success"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "assign-success-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)
	transfer := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)

	req := testutil.NewJSONRequest(t, map[string]any{
		"agent_id": agent.ID.String(),
	})
	setTransferAuthContext(req, org.ID, user.ID, "admin")
	testutil.SetPathParam(req, "id", transfer.ID.String())

	err := app.AssignAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Message string     `json:"message"`
			AgentID *uuid.UUID `json:"agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "Transfer assigned successfully", result.Data.Message)
	assert.Equal(t, agent.ID, *result.Data.AgentID)

	// Verify transfer updated
	var updatedTransfer models.AgentTransfer
	require.NoError(t, app.DB.First(&updatedTransfer, transfer.ID).Error)
	assert.Equal(t, agent.ID, *updatedTransfer.AgentID)
}

func TestApp_AssignAgentTransfer_AgentSelfAssign(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	account := createTestWhatsAppAccount(t, app, org.ID, "self-assign-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)
	transfer := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)

	// Agent self-assigns (no agent_id in body means assign to self)
	req := testutil.NewJSONRequest(t, map[string]any{})
	setTransferAuthContext(req, org.ID, agent.ID, "agent")
	testutil.SetPathParam(req, "id", transfer.ID.String())

	err := app.AssignAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify transfer assigned to the agent
	var updatedTransfer models.AgentTransfer
	require.NoError(t, app.DB.First(&updatedTransfer, transfer.ID).Error)
	assert.Equal(t, agent.ID, *updatedTransfer.AgentID)
}

func TestApp_AssignAgentTransfer_AgentCannotAssignToOthers(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	account := createTestWhatsAppAccount(t, app, org.ID, "cannot-assign-others-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)
	otherAgent := createTestAgent(t, app, org.ID)
	transfer := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)

	// Agent tries to assign to another agent - should fail
	req := testutil.NewJSONRequest(t, map[string]any{
		"agent_id": otherAgent.ID.String(),
	})
	setTransferAuthContext(req, org.ID, agent.ID, "agent")
	testutil.SetPathParam(req, "id", transfer.ID.String())

	err := app.AssignAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))

	var result map[string]any
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))
	assert.Equal(t, "Agents cannot assign transfers to others", result["message"])
}

func TestApp_AssignAgentTransfer_NotActive(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	user := createTestUser(t, app, org.ID, uniqueEmail("assign-not-active"), "password", "admin", true)
	account := createTestWhatsAppAccount(t, app, org.ID, "assign-not-active-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)
	transfer := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "resumed", nil) // Not active

	req := testutil.NewJSONRequest(t, map[string]any{
		"agent_id": agent.ID.String(),
	})
	setTransferAuthContext(req, org.ID, user.ID, "admin")
	testutil.SetPathParam(req, "id", transfer.ID.String())

	err := app.AssignAgentTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	var result map[string]any
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))
	assert.Equal(t, "Transfer is not active", result["message"])
}

// --- PickNextTransfer Tests ---

func TestApp_PickNextTransfer_Success(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	account := createTestWhatsAppAccount(t, app, org.ID, "pick-success-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)

	// Create unassigned transfer in general queue
	transfer := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)

	req := testutil.NewJSONRequest(t, nil)
	setTransferAuthContext(req, org.ID, agent.ID, "agent")

	err := app.PickNextTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Message  string                          `json:"message"`
			Transfer *handlers.AgentTransferResponse `json:"transfer"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "Transfer picked successfully", result.Data.Message)
	assert.NotNil(t, result.Data.Transfer)
	assert.Equal(t, transfer.ID.String(), result.Data.Transfer.ID)
	assert.Equal(t, agent.ID.String(), *result.Data.Transfer.AgentID)

	// Verify transfer updated in DB
	var updatedTransfer models.AgentTransfer
	require.NoError(t, app.DB.First(&updatedTransfer, transfer.ID).Error)
	assert.Equal(t, agent.ID, *updatedTransfer.AgentID)
}

func TestApp_PickNextTransfer_EmptyQueue(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)

	agent := createTestAgent(t, app, org.ID)

	// No transfers in queue
	req := testutil.NewJSONRequest(t, nil)
	setTransferAuthContext(req, org.ID, agent.ID, "agent")

	err := app.PickNextTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Message  string `json:"message"`
			Transfer any    `json:"transfer"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "No transfers in queue", result.Data.Message)
	assert.Nil(t, result.Data.Transfer)
}

func TestApp_PickNextTransfer_FIFO(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	account := createTestWhatsAppAccount(t, app, org.ID, "pick-fifo-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)

	// Create multiple transfers with different times
	transfer1 := &models.AgentTransfer{
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     "1111111111",
		Status:          "active",
		Source:          "manual",
		TransferredAt:   time.Now().Add(-2 * time.Hour), // Oldest
	}
	require.NoError(t, app.DB.Create(transfer1).Error)

	transfer2 := &models.AgentTransfer{
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     "2222222222",
		Status:          "active",
		Source:          "manual",
		TransferredAt:   time.Now().Add(-1 * time.Hour), // Newer
	}
	require.NoError(t, app.DB.Create(transfer2).Error)

	req := testutil.NewJSONRequest(t, nil)
	setTransferAuthContext(req, org.ID, agent.ID, "agent")

	err := app.PickNextTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Transfer *handlers.AgentTransferResponse `json:"transfer"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	// Should pick the oldest transfer (FIFO)
	assert.Equal(t, transfer1.ID.String(), result.Data.Transfer.ID)
}

func TestApp_PickNextTransfer_TeamFiltering(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	account := createTestWhatsAppAccount(t, app, org.ID, "pick-team-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)

	// Create a team and add agent as member
	team := createTestTeam(t, app, org.ID, agent.ID)

	// Create transfer in team queue
	teamTransfer := &models.AgentTransfer{
		OrganizationID:  org.ID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     "1111111111",
		Status:          "active",
		Source:          "manual",
		TeamID:          &team.ID,
		TransferredAt:   time.Now(),
	}
	require.NoError(t, app.DB.Create(teamTransfer).Error)

	// Create transfer in general queue
	generalTransfer := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", nil)

	// Pick from team queue specifically
	req := testutil.NewJSONRequest(t, nil)
	setTransferAuthContext(req, org.ID, agent.ID, "agent")
	testutil.SetQueryParam(req, "team_id", team.ID.String())

	err := app.PickNextTransfer(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Transfer *handlers.AgentTransferResponse `json:"transfer"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &result))

	// Should pick from team queue, not general queue
	assert.Equal(t, teamTransfer.ID.String(), result.Data.Transfer.ID)
	assert.NotEqual(t, generalTransfer.ID.String(), result.Data.Transfer.ID)
}

// --- Cross-Organization Isolation Tests ---

func TestApp_AgentTransfers_CrossOrgIsolation(t *testing.T) {
	app := agentTransfersTestApp(t)

	// Create two organizations
	org1 := createTestOrganization(t, app)
	org2 := createTestOrganization(t, app)

	user1 := createTestUser(t, app, org1.ID, uniqueEmail("cross-org-1"), "password", "admin", true)
	user2 := createTestUser(t, app, org2.ID, uniqueEmail("cross-org-2"), "password", "admin", true)

	account1 := createTestWhatsAppAccount(t, app, org1.ID, "cross-org-account-1")
	account2 := createTestWhatsAppAccount(t, app, org2.ID, "cross-org-account-2")

	contact1 := createTestContact(t, app, org1.ID)
	contact2 := createTestContact(t, app, org2.ID)

	// Create transfers in each org
	transfer1 := createTestTransfer(t, app, org1.ID, contact1.ID, account1.Name, "active", nil)
	transfer2 := createTestTransfer(t, app, org2.ID, contact2.ID, account2.Name, "active", nil)

	// User1 should only see org1's transfers
	req1 := testutil.NewGETRequest(t)
	setTransferAuthContext(req1, org1.ID, user1.ID, "admin")

	err := app.ListAgentTransfers(req1)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req1))

	var result1 struct {
		Data struct {
			Transfers []handlers.AgentTransferResponse `json:"transfers"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req1), &result1))

	assert.Len(t, result1.Data.Transfers, 1)
	assert.Equal(t, transfer1.ID.String(), result1.Data.Transfers[0].ID)

	// User2 should only see org2's transfers
	req2 := testutil.NewGETRequest(t)
	setTransferAuthContext(req2, org2.ID, user2.ID, "admin")

	err = app.ListAgentTransfers(req2)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))

	var result2 struct {
		Data struct {
			Transfers []handlers.AgentTransferResponse `json:"transfers"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req2), &result2))

	assert.Len(t, result2.Data.Transfers, 1)
	assert.Equal(t, transfer2.ID.String(), result2.Data.Transfers[0].ID)

	// User1 cannot resume org2's transfer
	req3 := testutil.NewJSONRequest(t, nil)
	setTransferAuthContext(req3, org1.ID, user1.ID, "admin")
	testutil.SetPathParam(req3, "id", transfer2.ID.String())

	err = app.ResumeFromTransfer(req3)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req3))
}

// --- ReturnAgentTransfersToQueue Tests ---

func TestApp_ReturnAgentTransfersToQueue(t *testing.T) {
	app := agentTransfersTestApp(t)
	org := createTestOrganization(t, app)
	account := createTestWhatsAppAccount(t, app, org.ID, "return-to-queue-account")

	contact := createTestContact(t, app, org.ID)
	agent := createTestAgent(t, app, org.ID)

	// Create transfers assigned to the agent
	transfer1 := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", &agent.ID)
	transfer2 := createTestTransfer(t, app, org.ID, contact.ID, account.Name, "active", &agent.ID)

	// Return transfers to queue
	count := app.ReturnAgentTransfersToQueue(agent.ID, org.ID)

	assert.Equal(t, 2, count)

	// Verify transfers are unassigned
	var updatedTransfer1, updatedTransfer2 models.AgentTransfer
	require.NoError(t, app.DB.First(&updatedTransfer1, transfer1.ID).Error)
	require.NoError(t, app.DB.First(&updatedTransfer2, transfer2.ID).Error)

	assert.Nil(t, updatedTransfer1.AgentID)
	assert.Nil(t, updatedTransfer2.AgentID)
}
