package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// enableStrictVisibility flips the org flag on and clears the settings cache.
//
// Test orgs have no ChatbotSettings row by default (unlike production, where
// one is created on signup), so this ensures one exists before updating it —
// otherwise the flag flip is a no-op UPDATE against zero rows.
func enableStrictVisibility(t *testing.T, app *handlers.App, orgID uuid.UUID) {
	t.Helper()
	settings := &models.ChatbotSettings{OrganizationID: orgID}
	require.NoError(t, app.DB.Where("organization_id = ? AND whats_app_account = ?", orgID, "").
		FirstOrCreate(settings).Error)
	require.NoError(t, app.DB.Model(&models.ChatbotSettings{}).
		Where("id = ?", settings.ID).
		Update("strict_conversation_visibility", true).Error)
	app.InvalidateChatbotSettingsCache(orgID)
}

// activeTransfer creates an active transfer for a contact.
func activeTransfer(t *testing.T, app *handlers.App, orgID, contactID uuid.UUID, agentID, teamID *uuid.UUID) {
	t.Helper()
	require.NoError(t, app.DB.Create(&models.AgentTransfer{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		ContactID:      contactID,
		PhoneNumber:    "+550000",
		Status:         models.TransferStatusActive,
		Source:         models.TransferSourceManual,
		AgentID:        agentID,
		TeamID:         teamID,
	}).Error)
}

// createTeamWithMember creates a team and adds userID as an agent member.
func createTeamWithMember(t *testing.T, app *handlers.App, orgID, userID uuid.UUID) *models.Team {
	t.Helper()
	team := &models.Team{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		Name:           "Test Team",
		IsActive:       true,
	}
	require.NoError(t, app.DB.Create(team).Error)
	require.NoError(t, app.DB.Create(&models.TeamMember{
		BaseModel: models.BaseModel{ID: uuid.New()},
		TeamID:    team.ID,
		UserID:    userID,
		Role:      models.TeamRoleAgent,
	}).Error)
	return team
}

func TestAuthorizeConversation(t *testing.T) {
	t.Parallel()

	t.Run("flag off preserves current behaviour", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
		agent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		other := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		activeTransfer(t, app, org.ID, contact.ID, &agent.ID, nil)

		// Flag off: an "other" agent with contacts:read still sees it (today's behaviour).
		assert.True(t, app.CanViewConversationForTest(other.ID, org.ID, contact))
	})

	t.Run("strict: assigned agent sees and interacts, other agent does not", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID) // agent role: no view_all
		agent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		other := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		activeTransfer(t, app, org.ID, contact.ID, &agent.ID, nil)
		enableStrictVisibility(t, app, org.ID)

		assert.True(t, app.CanViewConversationForTest(agent.ID, org.ID, contact))
		assert.True(t, app.CanInteractWithConversationForTest(agent.ID, org.ID, contact))
		assert.False(t, app.CanViewConversationForTest(other.ID, org.ID, contact))
		assert.False(t, app.CanInteractWithConversationForTest(other.ID, org.ID, contact))
	})

	t.Run("strict: view_all sees any conversation", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		managerRole := testutil.CreateAdminRole(t, app.DB, org.ID) // admin role has all perms incl view_all
		agent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		manager := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&managerRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		activeTransfer(t, app, org.ID, contact.ID, &agent.ID, nil)
		enableStrictVisibility(t, app, org.ID)

		assert.True(t, app.CanViewConversationForTest(manager.ID, org.ID, contact))
	})

	t.Run("strict: team queue visible to team members only", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		member := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		outsider := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		team := createTeamWithMember(t, app, org.ID, member.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		activeTransfer(t, app, org.ID, contact.ID, nil, &team.ID) // queued to team, no agent
		enableStrictVisibility(t, app, org.ID)

		assert.True(t, app.CanViewConversationForTest(member.ID, org.ID, contact))
		assert.False(t, app.CanViewConversationForTest(outsider.ID, org.ID, contact))
	})

	t.Run("strict: general queue (no team, no account default) is view_all only", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		anyAgent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		activeTransfer(t, app, org.ID, contact.ID, nil, nil) // general queue
		enableStrictVisibility(t, app, org.ID)

		assert.False(t, app.CanViewConversationForTest(anyAgent.ID, org.ID, contact))
	})

	t.Run("strict: general queue falls back to account default team", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		member := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		outsider := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		team := createTeamWithMember(t, app, org.ID, member.ID)
		acct := &models.WhatsAppAccount{
			BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID,
			Name: "gq-" + uuid.New().String()[:8], PhoneID: "p", BusinessID: "b",
			AccessToken: "t", DefaultTeamID: &team.ID,
		}
		require.NoError(t, app.DB.Create(acct).Error)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		require.NoError(t, app.DB.Model(contact).Update("whats_app_account", acct.Name).Error)
		contact.WhatsAppAccount = acct.Name
		activeTransfer(t, app, org.ID, contact.ID, nil, nil) // general queue, no team
		enableStrictVisibility(t, app, org.ID)

		assert.True(t, app.CanViewConversationForTest(member.ID, org.ID, contact))
		assert.False(t, app.CanViewConversationForTest(outsider.ID, org.ID, contact))
	})

	t.Run("strict: flow team (Contact.TeamID) scopes to that team only", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		member := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		outsider := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		team := createTeamWithMember(t, app, org.ID, member.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID) // no transfer, no carteira
		require.NoError(t, app.DB.Model(contact).Update("team_id", team.ID).Error)
		contact.TeamID = &team.ID
		enableStrictVisibility(t, app, org.ID)

		assert.True(t, app.CanViewConversationForTest(member.ID, org.ID, contact))
		assert.False(t, app.CanViewConversationForTest(outsider.ID, org.ID, contact))
	})

	t.Run("strict: account default team scopes a teamless conversation", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		member := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		outsider := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		team := createTeamWithMember(t, app, org.ID, member.ID)
		acct := &models.WhatsAppAccount{
			BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID,
			Name: "fin-" + uuid.New().String()[:8], PhoneID: "p", BusinessID: "b",
			AccessToken: "t", DefaultTeamID: &team.ID,
		}
		require.NoError(t, app.DB.Create(acct).Error)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		require.NoError(t, app.DB.Model(contact).Update("whats_app_account", acct.Name).Error)
		contact.WhatsAppAccount = acct.Name
		enableStrictVisibility(t, app, org.ID)

		assert.True(t, app.CanViewConversationForTest(member.ID, org.ID, contact))
		assert.False(t, app.CanViewConversationForTest(outsider.ID, org.ID, contact))
	})

	t.Run("strict: teamless with no account default is view_all only", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		anyAgent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID) // no transfer/carteira/team/account default
		enableStrictVisibility(t, app, org.ID)

		assert.False(t, app.CanViewConversationForTest(anyAgent.ID, org.ID, contact))
	})

	t.Run("strict: carteira wins over flow team", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		teamMember := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		team := createTeamWithMember(t, app, org.ID, teamMember.ID)
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		require.NoError(t, app.DB.Model(contact).Updates(map[string]any{"assigned_user_id": owner.ID, "team_id": team.ID}).Error)
		contact.AssignedUserID = &owner.ID
		contact.TeamID = &team.ID
		enableStrictVisibility(t, app, org.ID)

		assert.True(t, app.CanViewConversationForTest(owner.ID, org.ID, contact))
		assert.False(t, app.CanViewConversationForTest(teamMember.ID, org.ID, contact), "carteira is more specific than flow team")
	})

	t.Run("strict: carteira governs only without an active transfer", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		other := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", owner.ID).Error)
		contact.AssignedUserID = &owner.ID
		enableStrictVisibility(t, app, org.ID)

		assert.True(t, app.CanViewConversationForTest(owner.ID, org.ID, contact))
		assert.False(t, app.CanViewConversationForTest(other.ID, org.ID, contact))
	})

	t.Run("strict: active transfer wins over carteira (precedence)", func(t *testing.T) {
		app := newTestApp(t)
		org := testutil.CreateTestOrganization(t, app.DB)
		agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
		serving := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		carteira := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
		contact := testutil.CreateTestContact(t, app.DB, org.ID)
		require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", carteira.ID).Error)
		contact.AssignedUserID = &carteira.ID
		activeTransfer(t, app, org.ID, contact.ID, &serving.ID, nil)
		enableStrictVisibility(t, app, org.ID)

		// The active transfer's agent governs; the carteira agent does not.
		assert.True(t, app.CanViewConversationForTest(serving.ID, org.ID, contact))
		assert.False(t, app.CanViewConversationForTest(carteira.ID, org.ID, contact))
	})
}

// TestVisibilityScopeMatchesFunction is the anti-divergence guard: the SQL scope
// must return exactly the contacts for which canViewConversation is true.
func TestVisibilityScopeMatchesFunction(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	viewer := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	otherAgent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	team := createTeamWithMember(t, app, org.ID, viewer.ID)

	// One contact per branch of the tree.
	assignedToViewer := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, assignedToViewer.ID, &viewer.ID, nil)

	assignedToOther := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, assignedToOther.ID, &otherAgent.ID, nil)

	teamQueue := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, teamQueue.ID, nil, &team.ID)

	generalQueue := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, generalQueue.ID, nil, nil)

	carteira := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(carteira).Update("assigned_user_id", otherAgent.ID).Error)

	idle := testutil.CreateTestContact(t, app.DB, org.ID) // no transfer, no carteira

	flowTeamMine := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(flowTeamMine).Update("team_id", team.ID).Error) // team has viewer

	flowTeamOther := testutil.CreateTestContact(t, app.DB, org.ID)
	otherTeam := createTeamWithMember(t, app, org.ID, otherAgent.ID)
	require.NoError(t, app.DB.Model(flowTeamOther).Update("team_id", otherTeam.ID).Error)

	acctMine := &models.WhatsAppAccount{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID,
		Name: "am-" + uuid.New().String()[:8], PhoneID: "p", BusinessID: "b",
		AccessToken: "t", DefaultTeamID: &team.ID,
	}
	require.NoError(t, app.DB.Create(acctMine).Error)
	acctDefaultMine := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(acctDefaultMine).Update("whats_app_account", acctMine.Name).Error)

	// Branch C positive: an active general-queue transfer (agent_id NULL,
	// team_id NULL) on a contact whose WhatsApp account's default_team_id IS
	// the viewer's team.
	generalQueueAcctMine := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(generalQueueAcctMine).Update("whats_app_account", acctMine.Name).Error)
	activeTransfer(t, app, org.ID, generalQueueAcctMine.ID, nil, nil)

	// Branch F negative: no transfer, no carteira, no flow team, and the
	// contact's account default_team_id is a team the viewer is NOT in.
	otherTeamForAcct := createTeamWithMember(t, app, org.ID, otherAgent.ID)
	acctOther := &models.WhatsAppAccount{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID,
		Name: "ao-" + uuid.New().String()[:8], PhoneID: "p2", BusinessID: "b2",
		AccessToken: "t", DefaultTeamID: &otherTeamForAcct.ID,
	}
	require.NoError(t, app.DB.Create(acctOther).Error)
	acctDefaultOther := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(acctDefaultOther).Update("whats_app_account", acctOther.Name).Error)

	enableStrictVisibility(t, app, org.ID)

	// All contacts in the org.
	all := []*models.Contact{
		assignedToViewer, assignedToOther, teamQueue, generalQueue, carteira, idle,
		flowTeamMine, flowTeamOther, acctDefaultMine,
		generalQueueAcctMine, acctDefaultOther,
	}

	// Expected set per the function.
	expected := map[uuid.UUID]bool{}
	for _, c := range all {
		var fresh models.Contact
		require.NoError(t, app.DB.First(&fresh, "id = ?", c.ID).Error)
		if app.CanViewConversationForTest(viewer.ID, org.ID, &fresh) {
			expected[c.ID] = true
		}
	}

	// Actual set from the SQL scope.
	var visible []models.Contact
	q := app.ScopeVisibleConversationsForTest(
		app.DB.Where("organization_id = ?", org.ID), viewer.ID, org.ID)
	require.NoError(t, q.Find(&visible).Error)

	got := map[uuid.UUID]bool{}
	for i := range visible {
		got[visible[i].ID] = true
	}

	assert.Equal(t, expected, got,
		"scopeVisibleConversations must return exactly the contacts canViewConversation allows")

	// Pin the two branches added for general-queue-with-account-default (C)
	// and its negative counterpart (F, teamless with a foreign account
	// default): a future change that made both the function and the SQL deny
	// (or both allow) branch C, or both allow (or both deny) the branch F
	// negative, would still pass the set-equality assert above by construction
	// — these explicit checks guard against losing that coverage silently.
	assert.True(t, got[generalQueueAcctMine.ID], "branch C: general queue + account default mine must be visible")
	assert.False(t, got[acctDefaultOther.ID], "branch F negative: foreign account default team must not be visible")
}

func TestListContacts_StrictVisibility(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	agent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	other := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))

	mine := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, mine.ID, &agent.ID, nil)
	theirs := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, theirs.ID, &other.ID, nil)

	enableStrictVisibility(t, app, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, agent.ID)
	require.NoError(t, app.ListContacts(req))

	var resp struct {
		Data struct {
			Contacts []handlers.ContactResponse `json:"contacts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))

	ids := map[string]bool{}
	for _, c := range resp.Data.Contacts {
		ids[c.ID.String()] = true
	}
	assert.True(t, ids[mine.ID.String()], "agent sees own conversation")
	assert.False(t, ids[theirs.ID.String()], "agent must not see another agent's conversation")
}

func TestAssignAgentTransfer_403OnAnotherAgentsConversation(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	agentA := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	agentB := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	// Contact X is served by agent A (active transfer).
	activeTransfer(t, app, org.ID, contact.ID, &agentA.ID, nil)
	enableStrictVisibility(t, app, org.ID)

	var transfer models.AgentTransfer
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&transfer).Error)

	// Agent B self-assigns onto A's active conversation — must be 403.
	req := testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetAuthContext(req, org.ID, agentB.ID)
	testutil.SetPathParam(req, "id", transfer.ID.String())

	require.NoError(t, app.AssignAgentTransfer(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req),
		"agent B must not take over agent A's active conversation")

	// The transfer's agent is unchanged.
	var stored models.AgentTransfer
	require.NoError(t, app.DB.First(&stored, "id = ?", transfer.ID).Error)
	require.NotNil(t, stored.AgentID)
	assert.Equal(t, agentA.ID, *stored.AgentID, "assignment must be untouched")
}

func TestUnassignTransfer_403OnAnotherAgentsConversation(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	agentA := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	agentB := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	activeTransfer(t, app, org.ID, contact.ID, &agentA.ID, nil)
	enableStrictVisibility(t, app, org.ID)

	var transfer models.AgentTransfer
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&transfer).Error)

	// Agent B tries to kick A's conversation back to the queue — must be 403.
	req := testutil.NewJSONRequest(t, nil)
	testutil.SetAuthContext(req, org.ID, agentB.ID)
	testutil.SetPathParam(req, "id", transfer.ID.String())

	require.NoError(t, app.UnassignTransfer(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req),
		"agent B must not unassign agent A's active conversation")

	var stored models.AgentTransfer
	require.NoError(t, app.DB.First(&stored, "id = ?", transfer.ID).Error)
	require.NotNil(t, stored.AgentID)
	assert.Equal(t, agentA.ID, *stored.AgentID, "assignment must be untouched")
}

func TestListAgentTransfers_StrictVisibility(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	managerRole := testutil.CreateAdminRole(t, app.DB, org.ID) // view_all
	agentA := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	agentB := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	manager := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&managerRole.ID))

	contactA := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, contactA.ID, &agentA.ID, nil)
	contactB := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, contactB.ID, &agentB.ID, nil)

	enableStrictVisibility(t, app, org.ID)

	listFor := func(uid uuid.UUID) map[string]bool {
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, uid)
		require.NoError(t, app.ListAgentTransfers(req))
		var resp struct {
			Data struct {
				Transfers []handlers.AgentTransferResponse `json:"transfers"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
		ids := map[string]bool{}
		for _, tr := range resp.Data.Transfers {
			ids[tr.ContactID] = true
		}
		return ids
	}

	bSees := listFor(agentB.ID)
	assert.True(t, bSees[contactB.ID.String()], "agent B sees B's transfer")
	assert.False(t, bSees[contactA.ID.String()], "agent B must not see A's transfer")

	mgrSees := listFor(manager.ID)
	assert.True(t, mgrSees[contactA.ID.String()], "view_all manager sees A's transfer")
	assert.True(t, mgrSees[contactB.ID.String()], "view_all manager sees B's transfer")
}

func TestAssignContact_403WithoutVisibility(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	// A custom role with contacts:write but NO conversations:view_all.
	writerRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "contact-writer",
		[]string{"contacts:write"})
	ownerA := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	userB := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&writerRole.ID))

	// Carteira-only contact owned by A (no active transfer).
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", ownerA.ID).Error)
	enableStrictVisibility(t, app, org.ID)

	// B (contacts:write, no view_all) tries to reassign the contact to self.
	req := testutil.NewJSONRequest(t, map[string]any{"user_id": userB.ID.String()})
	testutil.SetAuthContext(req, org.ID, userB.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())

	require.NoError(t, app.AssignContact(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req),
		"a contacts:write user without view_all cannot reassign a conversation they can't view")

	var stored models.Contact
	require.NoError(t, app.DB.First(&stored, "id = ?", contact.ID).Error)
	require.NotNil(t, stored.AssignedUserID)
	assert.Equal(t, ownerA.ID, *stored.AssignedUserID, "assignment must be untouched")
}

func TestExportContacts_StrictVisibility(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	// contacts:export but NO view_all.
	exporterRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "strict-exporter",
		[]string{"contacts:export"})
	exporter := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&exporterRole.ID))
	other := testutil.CreateTestUser(t, app.DB, org.ID)

	mine := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithPhoneNumber("+15551111"))
	require.NoError(t, app.DB.Model(mine).Update("assigned_user_id", exporter.ID).Error)
	theirs := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithPhoneNumber("+15552222"))
	require.NoError(t, app.DB.Model(theirs).Update("assigned_user_id", other.ID).Error)

	enableStrictVisibility(t, app, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{
		"table":   "contacts",
		"columns": []string{"phone_number", "profile_name"},
	})
	req.RequestCtx.Request.Header.SetMethod("POST")
	testutil.SetAuthContext(req, org.ID, exporter.ID)

	require.NoError(t, app.ExportData(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
	csv := string(testutil.GetResponseBody(req))

	assert.Contains(t, csv, mine.PhoneNumber, "exporter sees own carteira contact")
	assert.NotContains(t, csv, theirs.PhoneNumber,
		"strict mode: a no-view_all exporter must not export another agent's conversation")
}

func TestSendMessage_403AfterTransfer(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	source := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	dest := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithContactAccount(account.Name))

	// Initially served by source.
	activeTransfer(t, app, org.ID, contact.ID, &source.ID, nil)
	enableStrictVisibility(t, app, org.ID)

	// Transfer to dest: close source's transfer, open dest's (mirror the real
	// reassignment: only one active transfer at a time).
	require.NoError(t, app.DB.Model(&models.AgentTransfer{}).
		Where("contact_id = ? AND status = ?", contact.ID, models.TransferStatusActive).
		Update("status", models.TransferStatusResumed).Error)
	activeTransfer(t, app, org.ID, contact.ID, &dest.ID, nil)

	// Source now tries to send — must be 403.
	req := testutil.NewJSONRequest(t, map[string]any{"content": map[string]string{"body": "hi"}})
	testutil.SetAuthContext(req, org.ID, source.ID)
	testutil.SetPathParam(req, "id", contact.ID.String())
	require.NoError(t, app.SendMessage(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req),
		"the source agent loses interaction access immediately at transfer")
}

// createChatbotSession creates a chatbot session for a contact.
func createChatbotSession(t *testing.T, app *handlers.App, orgID, contactID uuid.UUID) *models.ChatbotSession {
	t.Helper()
	session := &models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		ContactID:       contactID,
		WhatsAppAccount: "default",
		PhoneNumber:     "+550000",
		Status:          models.SessionStatusCompleted,
	}
	require.NoError(t, app.DB.Create(session).Error)
	return session
}

func TestGetChatbotSession_404WhenNotVisible(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	agentA := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	agentB := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))

	// Contact X served by agent A (active transfer), so B cannot view it.
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, contact.ID, &agentA.ID, nil)
	session := createChatbotSession(t, app, org.ID, contact.ID)
	enableStrictVisibility(t, app, org.ID)

	getFor := func(uid uuid.UUID) int {
		req := testutil.NewGETRequest(t)
		testutil.SetAuthContext(req, org.ID, uid)
		testutil.SetPathParam(req, "id", session.ID.String())
		require.NoError(t, app.GetChatbotSession(req))
		return testutil.GetResponseStatusCode(req)
	}

	assert.Equal(t, fasthttp.StatusNotFound, getFor(agentB.ID),
		"agent B must not read the transcript of A's conversation")
	assert.Equal(t, fasthttp.StatusOK, getFor(agentA.ID),
		"the assigned agent A can read the session")
}

func TestListChatbotSessions_StrictVisibility(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	agentA := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	agentB := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))

	contactX := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, contactX.ID, &agentA.ID, nil)
	sessionX := createChatbotSession(t, app, org.ID, contactX.ID)

	contactY := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, contactY.ID, &agentB.ID, nil)
	sessionY := createChatbotSession(t, app, org.ID, contactY.ID)

	enableStrictVisibility(t, app, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, agentB.ID)
	require.NoError(t, app.ListChatbotSessions(req))

	var resp struct {
		Data struct {
			Sessions []models.ChatbotSession `json:"sessions"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))

	ids := map[uuid.UUID]bool{}
	for _, s := range resp.Data.Sessions {
		ids[s.ID] = true
	}
	assert.True(t, ids[sessionY.ID], "agent B sees B's session")
	assert.False(t, ids[sessionX.ID], "agent B must not see A's session")
}

func TestAssignAgentTransfer_TeamMemberCanPickUp(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	member := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	team := createTeamWithMember(t, app, org.ID, member.ID)

	// Transfer QUEUED to team T: active, no agent.
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, contact.ID, nil, &team.ID)
	enableStrictVisibility(t, app, org.ID)

	var transfer models.AgentTransfer
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&transfer).Error)

	// The team member self-assigns (empty body = "assign to me").
	req := testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetAuthContext(req, org.ID, member.ID)
	testutil.SetPathParam(req, "id", transfer.ID.String())

	require.NoError(t, app.AssignAgentTransfer(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req),
		"a member of the queued team may pick up the transfer")

	var stored models.AgentTransfer
	require.NoError(t, app.DB.First(&stored, "id = ?", transfer.ID).Error)
	require.NotNil(t, stored.AgentID)
	assert.Equal(t, member.ID, *stored.AgentID, "the transfer is now assigned to the picking-up member")
}

func TestSendMessage_MultiTenantIsolation(t *testing.T) {
	app := newTestApp(t)
	orgX := testutil.CreateTestOrganization(t, app.DB)
	orgY := testutil.CreateTestOrganization(t, app.DB)
	// Manager of X with view_all (admin role has it).
	adminRole := testutil.CreateAdminRole(t, app.DB, orgX.ID)
	managerX := testutil.CreateTestUser(t, app.DB, orgX.ID, testutil.WithRoleID(&adminRole.ID))
	// A contact in Y.
	contactY := testutil.CreateTestContact(t, app.DB, orgY.ID)
	enableStrictVisibility(t, app, orgX.ID)
	enableStrictVisibility(t, app, orgY.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"content": map[string]string{"body": "hi"}})
	testutil.SetAuthContext(req, orgX.ID, managerX.ID) // acting as org X
	testutil.SetPathParam(req, "id", contactY.ID.String())
	require.NoError(t, app.SendMessage(req))
	code := testutil.GetResponseStatusCode(req)
	assert.True(t, code == fasthttp.StatusNotFound || code == fasthttp.StatusForbidden,
		"view_all in org X must never reach a contact in org Y, got %d", code)
}

func TestCanViewTeamMember(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateAgentRole(t, app.DB, org.ID)
	viewer := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	mate := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	stranger := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	teamless := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	team := createTeamWithMember(t, app, org.ID, viewer.ID)
	require.NoError(t, app.DB.Create(&models.TeamMember{
		BaseModel: models.BaseModel{ID: uuid.New()}, TeamID: team.ID, UserID: mate.ID,
	}).Error)
	// stranger is in a different team
	_ = createTeamWithMember(t, app, org.ID, stranger.ID)

	assert.True(t, app.CanViewTeamMemberForTest(viewer.ID, mate.ID), "shares a team")
	assert.True(t, app.CanViewTeamMemberForTest(viewer.ID, viewer.ID), "self (has a team)")
	assert.False(t, app.CanViewTeamMemberForTest(viewer.ID, stranger.ID), "different team")
	assert.False(t, app.CanViewTeamMemberForTest(viewer.ID, teamless.ID), "owner has no team")
	assert.False(t, app.CanViewTeamMemberForTest(teamless.ID, mate.ID), "viewer has no team")
}

// makeViewTeamSupervisor creates a supervisor role (view_team) + user, added as
// a member of each given team.
func makeViewTeamSupervisor(t *testing.T, app *handlers.App, orgID uuid.UUID, teamIDs ...uuid.UUID) uuid.UUID {
	t.Helper()
	role := testutil.CreateTestRoleWithKeys(t, app.DB, orgID, "loja-sup",
		[]string{"chat:read", "chat:write", "chat.assign:write",
			"conversations:view_team", "transfers:read", "transfers:write"})
	sup := testutil.CreateTestUser(t, app.DB, orgID, testutil.WithRoleID(&role.ID))
	for _, tid := range teamIDs {
		require.NoError(t, app.DB.Create(&models.TeamMember{
			BaseModel: models.BaseModel{ID: uuid.New()}, TeamID: tid, UserID: sup.ID,
		}).Error)
	}
	return sup.ID
}

func TestTeamScopedVisibility(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	agentA := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	agentB := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))

	teamStore := createTeamWithMember(t, app, org.ID, agentA.ID) // store team, agentA in it
	teamOther := createTeamWithMember(t, app, org.ID, agentB.ID) // other store team, agentB in it
	enableStrictVisibility(t, app, org.ID)

	// supervisor of the store: view_team + member of teamStore (shares team with agentA, not agentB)
	supID := makeViewTeamSupervisor(t, app, org.ID, teamStore.ID)

	load := func(c *models.Contact) *models.Contact {
		var fresh models.Contact
		require.NoError(t, app.DB.First(&fresh, "id = ?", c.ID).Error)
		return &fresh
	}

	// 1. carteira of agentA (same team) -> supervisor sees AND interacts
	carteiraA := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(carteiraA).Update("assigned_user_id", agentA.ID).Error)
	assert.True(t, app.CanViewConversationForTest(supID, org.ID, load(carteiraA)))
	assert.True(t, app.CanInteractWithConversationForTest(supID, org.ID, load(carteiraA)))

	// 2. active transfer to agentA (same team) -> supervisor sees
	transferA := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, transferA.ID, &agentA.ID, nil)
	assert.True(t, app.CanViewConversationForTest(supID, org.ID, load(transferA)))

	// 3. active transfer to agentB (other team) -> supervisor does NOT see (branch G negative)
	transferB := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, transferB.ID, &agentB.ID, nil)
	assert.False(t, app.CanViewConversationForTest(supID, org.ID, load(transferB)))

	// 4. active transfer to a teamless agent -> supervisor does NOT see (branch G negative)
	agentTeamless := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	transferTeamless := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, transferTeamless.ID, &agentTeamless.ID, nil)
	assert.False(t, app.CanViewConversationForTest(supID, org.ID, load(transferTeamless)))

	// 5. carteira of agentB (other team) -> supervisor does NOT see
	carteiraB := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(carteiraB).Update("assigned_user_id", agentB.ID).Error)
	assert.False(t, app.CanViewConversationForTest(supID, org.ID, load(carteiraB)))

	// 6. multi-team supervisor (both stores) sees both agents' carteiras
	multiSupID := makeViewTeamSupervisor(t, app, org.ID, teamStore.ID, teamOther.ID)
	assert.True(t, app.CanViewConversationForTest(multiSupID, org.ID, load(carteiraA)))
	assert.True(t, app.CanViewConversationForTest(multiSupID, org.ID, load(carteiraB)))

	// 7. owner with no team -> not visible to the supervisor
	teamless := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	orphan := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(orphan).Update("assigned_user_id", teamless.ID).Error)
	assert.False(t, app.CanViewConversationForTest(supID, org.ID, load(orphan)))

	// 8. supervisor with view_team but NO team -> no extra access (cannot see agentA's carteira)
	teamlessSupID := makeViewTeamSupervisor(t, app, org.ID) // no teams
	assert.False(t, app.CanViewConversationForTest(teamlessSupID, org.ID, load(carteiraA)))

	// 9. regression: a plain agent (no view_team) does NOT see a co-member's carteira
	assert.False(t, app.CanViewConversationForTest(agentA.ID, org.ID, load(carteiraB)))

	// 10. view_all + view_team together: view_all wins (sees a foreign-team conversation).
	bothRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "both",
		[]string{"conversations:view_all", "conversations:view_team", "chat:read"})
	both := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&bothRole.ID))
	assert.True(t, app.CanViewConversationForTest(both.ID, org.ID, load(carteiraB)),
		"view_all still grants global access when view_team is also present")
}

// TestVisibilityScopeMatchesFunction_ViewTeam is the oracle guard for the
// view_team disjuncts (G, H): the SQL scope must match the function exactly
// for a viewer who holds view_team and shares a team with another agent.
func TestVisibilityScopeMatchesFunction_ViewTeam(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	otherAgent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	team := createTeamWithMember(t, app, org.ID, otherAgent.ID)

	// supervisor shares `team` with otherAgent and holds view_team.
	supID := makeViewTeamSupervisor(t, app, org.ID, team.ID)

	// Contacts across the branches, several owned by otherAgent (a team-mate).
	transferToMate := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, transferToMate.ID, &otherAgent.ID, nil) // G
	carteiraMate := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(carteiraMate).Update("assigned_user_id", otherAgent.ID).Error) // H
	teamQueue := testutil.CreateTestContact(t, app.DB, org.ID)
	activeTransfer(t, app, org.ID, teamQueue.ID, nil, &team.ID) // B
	idle := testutil.CreateTestContact(t, app.DB, org.ID)       // none

	// A contact owned by a stranger in another team (must NOT be visible).
	stranger := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	_ = createTeamWithMember(t, app, org.ID, stranger.ID)
	carteiraStranger := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(carteiraStranger).Update("assigned_user_id", stranger.ID).Error)

	enableStrictVisibility(t, app, org.ID)

	all := []*models.Contact{transferToMate, carteiraMate, teamQueue, idle, carteiraStranger}
	expected := map[uuid.UUID]bool{}
	for _, c := range all {
		var fresh models.Contact
		require.NoError(t, app.DB.First(&fresh, "id = ?", c.ID).Error)
		if app.CanViewConversationForTest(supID, org.ID, &fresh) {
			expected[c.ID] = true
		}
	}

	var visible []models.Contact
	q := app.ScopeVisibleConversationsForTest(
		app.DB.Where("organization_id = ?", org.ID), supID, org.ID)
	require.NoError(t, q.Find(&visible).Error)
	got := map[uuid.UUID]bool{}
	for i := range visible {
		got[visible[i].ID] = true
	}

	assert.Equal(t, expected, got,
		"view_team: SQL scope must equal the function for a team supervisor")
	// Sanity: the supervisor sees the team-mate's owned conversations, not the stranger's.
	assert.True(t, got[transferToMate.ID])
	assert.True(t, got[carteiraMate.ID])
	assert.False(t, got[carteiraStranger.ID])
}

func TestContactAndAccountTeamColumns(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	team := createTeamWithMember(t, app, org.ID, user.ID)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("team_id", team.ID).Error)
	var freshContact models.Contact
	require.NoError(t, app.DB.First(&freshContact, "id = ?", contact.ID).Error)
	require.NotNil(t, freshContact.TeamID)
	assert.Equal(t, team.ID, *freshContact.TeamID)

	acct := &models.WhatsAppAccount{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: org.ID,
		Name: "acct-" + uuid.New().String()[:8], PhoneID: "p", BusinessID: "b",
		AccessToken: "t", DefaultTeamID: &team.ID,
	}
	require.NoError(t, app.DB.Create(acct).Error)
	var freshAcct models.WhatsAppAccount
	require.NoError(t, app.DB.First(&freshAcct, "id = ?", acct.ID).Error)
	require.NotNil(t, freshAcct.DefaultTeamID)
	assert.Equal(t, team.ID, *freshAcct.DefaultTeamID)
}

func TestReleaseContact_ClearsTeamID(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	team := &models.Team{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "Test Team",
		IsActive:       true,
	}
	require.NoError(t, app.DB.Create(team).Error)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("team_id", team.ID).Error)
	contact.TeamID = &team.ID

	require.NoError(t, app.ReleaseContactForTest(contact, nil, "test"))

	var fresh models.Contact
	require.NoError(t, app.DB.First(&fresh, "id = ?", contact.ID).Error)
	assert.Nil(t, fresh.TeamID, "release must clear the effective team")
}

// TestAccountDefaultTeam_VisibilityReflectsHandlerUpdate proves the account
// default-team cache is invalidated by the UpdateAccount handler: after an
// admin changes a number's default team, visibility reflects it immediately.
// With Redis present a missing invalidation would keep the stale (primed)
// verdict and fail the final assertion; without Redis the DB is read each time.
func TestAccountDefaultTeam_VisibilityReflectsHandlerUpdate(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	viewer := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	admin := createAdminUser(t, app, org.ID)
	teamMine := createTeamWithMember(t, app, org.ID, viewer.ID)
	teamOther := createTeamWithMember(t, app, org.ID, admin.ID) // viewer is NOT a member

	account := testutil.CreateTestWhatsAppAccount(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(&models.WhatsAppAccount{}).
		Where("id = ?", account.ID).Update("default_team_id", teamMine.ID).Error)

	contact := testutil.CreateTestContact(t, app.DB, org.ID) // no transfer, no carteira, no flow team
	require.NoError(t, app.DB.Model(contact).Update("whats_app_account", account.Name).Error)
	contact.WhatsAppAccount = account.Name

	enableStrictVisibility(t, app, org.ID)

	// Account default team = viewer's team → visible (primes the cache).
	require.True(t, app.CanViewConversationForTest(viewer.ID, org.ID, contact))

	// Admin repoints the account's default team to one the viewer is not in.
	req := testutil.NewJSONRequest(t, map[string]any{"default_team_id": teamOther.ID.String()})
	testutil.SetAuthContext(req, org.ID, admin.ID)
	testutil.SetPathParam(req, "id", account.ID.String())
	require.NoError(t, app.UpdateAccount(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// The handler must have invalidated the cache: the viewer loses access.
	assert.False(t, app.CanViewConversationForTest(viewer.ID, org.ID, contact),
		"after the account's default team changes, visibility must follow (cache invalidated)")
}

// TestCreateContact_FormattedPhoneMatchesExisting guards the manual-create path:
// a differently-formatted form of an existing number must resolve to the same
// contact (409 conflict), not create a duplicate.
func TestCreateContact_FormattedPhoneMatchesExisting(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	admin := createAdminUser(t, app, org.ID)

	// Existing contact stored digits-only, as inbound webhooks store it.
	testutil.CreateTestContactWith(t, app.DB, org.ID, testutil.WithPhoneNumber("5511955554444"))

	req := testutil.NewJSONRequest(t, map[string]any{
		"phone_number": "+55 (11) 95555-4444",
		"profile_name": "Dup",
	})
	testutil.SetAuthContext(req, org.ID, admin.ID)
	require.NoError(t, app.CreateContact(req))
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req),
		"a formatted form of an existing number must conflict, not create a duplicate")

	var count int64
	app.DB.Model(&models.Contact{}).Where("organization_id = ?", org.ID).Count(&count)
	assert.Equal(t, int64(1), count, "no duplicate contact created from a formatted number")
}
