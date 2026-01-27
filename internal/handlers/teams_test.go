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

// createTeam creates a test team directly in the database.
func createTeam(t *testing.T, app *handlers.App, orgID uuid.UUID, name string) *models.Team {
	t.Helper()

	team := &models.Team{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     orgID,
		Name:               name,
		Description:        "Test team description",
		AssignmentStrategy: models.AssignmentStrategyRoundRobin,
		IsActive:           true,
	}
	require.NoError(t, app.DB.Create(team).Error)
	return team
}

// addTeamMember adds a user to a team directly in the database.
func addTeamMember(t *testing.T, app *handlers.App, teamID, userID uuid.UUID, role models.TeamRole) *models.TeamMember {
	t.Helper()

	member := &models.TeamMember{
		BaseModel: models.BaseModel{ID: uuid.New()},
		TeamID:    teamID,
		UserID:    userID,
		Role:      role,
	}
	require.NoError(t, app.DB.Create(member).Error)
	return member
}

// createAdminUser creates a user with an admin role that has all permissions.
func createAdminUser(t *testing.T, app *handlers.App, orgID uuid.UUID) *models.User {
	t.Helper()

	adminRole := testutil.CreateAdminRole(t, app.DB, orgID)
	return testutil.CreateTestUser(t, app.DB, orgID,
		testutil.WithRoleID(&adminRole.ID),
	)
}

// --- ListTeams Tests ---

func TestApp_ListTeams_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	// Create teams
	team1 := createTeam(t, app, org.ID, "Alpha Team")
	team2 := createTeam(t, app, org.ID, "Beta Team")

	// Add a member to team1 to verify member_count
	member := testutil.CreateTestUser(t, app.DB, org.ID)
	addTeamMember(t, app, team1.ID, member.ID, models.TeamRoleAgent)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListTeams(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Teams []handlers.TeamResponse `json:"teams"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Len(t, resp.Data.Teams, 2)

	// Teams should be sorted by name ASC
	assert.Equal(t, team1.Name, resp.Data.Teams[0].Name)
	assert.Equal(t, team2.Name, resp.Data.Teams[1].Name)

	// team1 has one member
	assert.Equal(t, 1, resp.Data.Teams[0].MemberCount)
	// team2 has no members
	assert.Equal(t, 0, resp.Data.Teams[1].MemberCount)
}

func TestApp_ListTeams_Empty(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.ListTeams(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Teams []handlers.TeamResponse `json:"teams"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Len(t, resp.Data.Teams, 0)
}

// --- GetTeam Tests ---

func TestApp_GetTeam_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	team := createTeam(t, app, org.ID, "Support Team")
	agent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithFullName("Agent Smith"))
	addTeamMember(t, app, team.ID, agent.ID, models.TeamRoleAgent)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", team.ID.String())

	err := app.GetTeam(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Team handlers.TeamResponse `json:"team"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, team.ID, resp.Data.Team.ID)
	assert.Equal(t, "Support Team", resp.Data.Team.Name)
	assert.Equal(t, 1, resp.Data.Team.MemberCount)
	// GetTeam includes members
	require.Len(t, resp.Data.Team.Members, 1)
	assert.Equal(t, agent.ID, resp.Data.Team.Members[0].UserID)
	assert.Equal(t, "Agent Smith", resp.Data.Team.Members[0].FullName)
}

func TestApp_GetTeam_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.GetTeam(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- CreateTeam Tests ---

func TestApp_CreateTeam_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	reqBody := handlers.TeamRequest{
		Name:               "New Team",
		Description:        "A brand new team",
		AssignmentStrategy: models.AssignmentStrategyLoadBalanced,
		IsActive:           true,
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTeam(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Team handlers.TeamResponse `json:"team"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, "New Team", resp.Data.Team.Name)
	assert.Equal(t, "A brand new team", resp.Data.Team.Description)
	assert.Equal(t, models.AssignmentStrategyLoadBalanced, resp.Data.Team.AssignmentStrategy)
	assert.True(t, resp.Data.Team.IsActive)
	assert.NotEqual(t, uuid.Nil, resp.Data.Team.ID)

	// Verify in DB
	var dbTeam models.Team
	require.NoError(t, app.DB.First(&dbTeam, "id = ?", resp.Data.Team.ID).Error)
	assert.Equal(t, "New Team", dbTeam.Name)
	assert.Equal(t, org.ID, dbTeam.OrganizationID)
}

func TestApp_CreateTeam_DefaultStrategy(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	// Omit assignment_strategy to verify default
	reqBody := handlers.TeamRequest{
		Name:     "Default Strategy Team",
		IsActive: true,
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTeam(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Team handlers.TeamResponse `json:"team"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, models.AssignmentStrategyRoundRobin, resp.Data.Team.AssignmentStrategy)
}

func TestApp_CreateTeam_MissingName(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	reqBody := handlers.TeamRequest{
		Name:        "",
		Description: "Team without a name",
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateTeam(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// --- UpdateTeam Tests ---

func TestApp_UpdateTeam_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	team := createTeam(t, app, org.ID, "Old Name")

	reqBody := handlers.TeamRequest{
		Name:               "Updated Name",
		Description:        "Updated description",
		AssignmentStrategy: models.AssignmentStrategyManual,
		IsActive:           true,
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", team.ID.String())

	err := app.UpdateTeam(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Team handlers.TeamResponse `json:"team"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, "Updated Name", resp.Data.Team.Name)
	assert.Equal(t, "Updated description", resp.Data.Team.Description)
	assert.Equal(t, models.AssignmentStrategyManual, resp.Data.Team.AssignmentStrategy)
	assert.True(t, resp.Data.Team.IsActive)

	// Verify in DB
	var dbTeam models.Team
	require.NoError(t, app.DB.First(&dbTeam, "id = ?", team.ID).Error)
	assert.Equal(t, "Updated Name", dbTeam.Name)
}

func TestApp_UpdateTeam_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	reqBody := handlers.TeamRequest{
		Name: "Updated Name",
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.UpdateTeam(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- DeleteTeam Tests ---

func TestApp_DeleteTeam_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	team := createTeam(t, app, org.ID, "Deletable Team")
	// Add a member to verify cascade cleanup
	member := testutil.CreateTestUser(t, app.DB, org.ID)
	addTeamMember(t, app, team.ID, member.ID, models.TeamRoleAgent)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", team.ID.String())

	err := app.DeleteTeam(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify team was deleted
	var dbTeam models.Team
	err = app.DB.First(&dbTeam, "id = ?", team.ID).Error
	assert.Error(t, err)

	// Verify team members were also deleted
	var memberCount int64
	app.DB.Model(&models.TeamMember{}).Where("team_id = ?", team.ID).Count(&memberCount)
	assert.Equal(t, int64(0), memberCount)
}

func TestApp_DeleteTeam_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.DeleteTeam(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- ListTeamMembers Tests ---

func TestApp_ListTeamMembers_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	team := createTeam(t, app, org.ID, "Members Team")

	agent1 := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithFullName("Agent One"))
	agent2 := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithFullName("Agent Two"))
	addTeamMember(t, app, team.ID, agent1.ID, models.TeamRoleAgent)
	addTeamMember(t, app, team.ID, agent2.ID, models.TeamRoleManager)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", team.ID.String())

	err := app.ListTeamMembers(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Members []handlers.TeamMemberResponse `json:"members"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Len(t, resp.Data.Members, 2)

	// Collect user IDs from the response
	memberUserIDs := make(map[uuid.UUID]bool)
	for _, m := range resp.Data.Members {
		memberUserIDs[m.UserID] = true
	}
	assert.True(t, memberUserIDs[agent1.ID])
	assert.True(t, memberUserIDs[agent2.ID])
}

func TestApp_ListTeamMembers_TeamNotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", uuid.New().String())

	err := app.ListTeamMembers(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}

// --- AddTeamMember Tests ---

func TestApp_AddTeamMember_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	team := createTeam(t, app, org.ID, "Add Member Team")
	newAgent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithFullName("New Agent"))

	reqBody := handlers.TeamMemberRequest{
		UserID: newAgent.ID.String(),
		Role:   models.TeamRoleAgent,
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", team.ID.String())

	err := app.AddTeamMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			Member handlers.TeamMemberResponse `json:"member"`
		} `json:"data"`
	}
	err = json.Unmarshal(testutil.GetResponseBody(req), &resp)
	require.NoError(t, err)

	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, newAgent.ID, resp.Data.Member.UserID)
	assert.Equal(t, "New Agent", resp.Data.Member.FullName)
	assert.Equal(t, models.TeamRoleAgent, resp.Data.Member.Role)

	// Verify in DB
	var dbMember models.TeamMember
	require.NoError(t, app.DB.Where("team_id = ? AND user_id = ?", team.ID, newAgent.ID).First(&dbMember).Error)
	assert.Equal(t, models.TeamRoleAgent, dbMember.Role)
}

func TestApp_AddTeamMember_DuplicateMember(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	team := createTeam(t, app, org.ID, "Duplicate Member Team")
	existingAgent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithFullName("Existing Agent"))
	addTeamMember(t, app, team.ID, existingAgent.ID, models.TeamRoleAgent)

	reqBody := handlers.TeamMemberRequest{
		UserID: existingAgent.ID.String(),
		Role:   models.TeamRoleAgent,
	}

	req := testutil.NewJSONRequest(t, reqBody)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", team.ID.String())

	err := app.AddTeamMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
}

// --- RemoveTeamMember Tests ---

func TestApp_RemoveTeamMember_Success(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	team := createTeam(t, app, org.ID, "Remove Member Team")
	agent := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithFullName("Removable Agent"))
	addTeamMember(t, app, team.ID, agent.ID, models.TeamRoleAgent)

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", team.ID.String())
	testutil.SetPathParam(req, "member_user_id", agent.ID.String())

	err := app.RemoveTeamMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	// Verify member was removed from DB
	var memberCount int64
	app.DB.Model(&models.TeamMember{}).Where("team_id = ? AND user_id = ?", team.ID, agent.ID).Count(&memberCount)
	assert.Equal(t, int64(0), memberCount)
}

func TestApp_RemoveTeamMember_NotFound(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := createAdminUser(t, app, org.ID)

	team := createTeam(t, app, org.ID, "Remove NotFound Team")

	req := testutil.NewGETRequest(t)
	req.RequestCtx.Request.Header.SetMethod("DELETE")
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", team.ID.String())
	testutil.SetPathParam(req, "member_user_id", uuid.New().String())

	err := app.RemoveTeamMember(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}
