package assignment_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/assignment"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- Test setup helpers ---

// newAssigner constructs an Assigner against the live test DB+Redis.
// Skips the test if either is unavailable.
func newAssigner(t *testing.T) (*assignment.Assigner, *gorm.DB) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set, skipping")
	}
	return assignment.New(db, rdb, testutil.NopLogger()), db
}

// createTeam creates a team with a strategy and member set, and returns the team
// + the agent users it contains. All members are added as TeamRoleAgent.
func createTeam(t *testing.T, db *gorm.DB, orgID uuid.UUID, strategy models.AssignmentStrategy, agentCount int, perAgentTimeout int) (*models.Team, []*models.User) {
	t.Helper()

	team := &models.Team{
		BaseModel:           models.BaseModel{ID: uuid.New()},
		OrganizationID:      orgID,
		Name:                "team-" + uuid.New().String()[:8],
		AssignmentStrategy:  strategy,
		PerAgentTimeoutSecs: perAgentTimeout,
		IsActive:            true,
	}
	require.NoError(t, db.Create(team).Error)

	users := make([]*models.User, 0, agentCount)
	for range agentCount {
		u := testutil.CreateTestUser(t, db, orgID)
		require.NoError(t, db.Create(&models.TeamMember{
			BaseModel: models.BaseModel{ID: uuid.New()},
			TeamID:    team.ID,
			UserID:    u.ID,
			Role:      models.TeamRoleAgent,
		}).Error)
		users = append(users, u)
	}
	return team, users
}

func setUnavailable(t *testing.T, db *gorm.DB, userID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", userID).Update("is_available", false).Error)
}

func setInactive(t *testing.T, db *gorm.DB, userID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", userID).Update("is_active", false).Error)
}

// --- GetTeamConfig (cache) ---

func TestGetTeamConfig_LoadsFromDBAndCaches(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, users := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 3, 20)

	cfg := a.GetTeamConfig(team.ID)
	require.NotNil(t, cfg)
	assert.Equal(t, models.AssignmentStrategyRoundRobin, cfg.Strategy)
	assert.Equal(t, 20, cfg.PerAgentTimeoutSecs)
	assert.Len(t, cfg.MemberIDs, len(users))

	// Subsequent call must come from cache; verify the cache key is populated.
	rdb := testutil.SetupTestRedis(t)
	require.NotNil(t, rdb)
	cached, err := rdb.Get(context.Background(), "team:assignment:"+team.ID.String()).Result()
	require.NoError(t, err)
	var roundtrip assignment.TeamConfig
	require.NoError(t, json.Unmarshal([]byte(cached), &roundtrip))
	assert.Equal(t, cfg.Strategy, roundtrip.Strategy)
	assert.ElementsMatch(t, cfg.MemberIDs, roundtrip.MemberIDs)
}

func TestGetTeamConfig_InactiveTeamReturnsNil(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, _ := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 1, 0)
	require.NoError(t, db.Model(team).Update("is_active", false).Error)

	assert.Nil(t, a.GetTeamConfig(team.ID))
}

func TestGetTeamConfig_OnlyAgentRoleMembers(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 2, 0)

	// Add a manager — must be excluded from MemberIDs.
	manager := testutil.CreateTestUser(t, db, org.ID)
	require.NoError(t, db.Create(&models.TeamMember{
		BaseModel: models.BaseModel{ID: uuid.New()},
		TeamID:    team.ID,
		UserID:    manager.ID,
		Role:      models.TeamRoleManager,
	}).Error)

	cfg := a.GetTeamConfig(team.ID)
	require.NotNil(t, cfg)
	require.Len(t, cfg.MemberIDs, len(agents))
	for _, id := range cfg.MemberIDs {
		assert.NotEqual(t, manager.ID, id, "manager must not appear in agent member set")
	}
}

func TestInvalidateTeamCache_RemovesCachedConfig(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, _ := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 1, 0)

	require.NotNil(t, a.GetTeamConfig(team.ID)) // populates cache
	a.InvalidateTeamCache(team.ID)

	rdb := testutil.SetupTestRedis(t)
	exists, err := rdb.Exists(context.Background(), "team:assignment:"+team.ID.String()).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

// --- AssignToTeam: round_robin ---

func TestAssignToTeam_RoundRobin_PicksOldestLastAssigned(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 3, 0)

	// Stamp last_assigned_at: agent[0] = oldest, agent[1] = middle, agent[2] = newest.
	now := time.Now()
	require.NoError(t, db.Model(&models.TeamMember{}).
		Where("team_id = ? AND user_id = ?", team.ID, agents[0].ID).
		Update("last_assigned_at", now.Add(-3*time.Hour)).Error)
	require.NoError(t, db.Model(&models.TeamMember{}).
		Where("team_id = ? AND user_id = ?", team.ID, agents[1].ID).
		Update("last_assigned_at", now.Add(-2*time.Hour)).Error)
	require.NoError(t, db.Model(&models.TeamMember{}).
		Where("team_id = ? AND user_id = ?", team.ID, agents[2].ID).
		Update("last_assigned_at", now.Add(-1*time.Hour)).Error)

	got := a.AssignToTeam(team.ID, org.ID, nil, nil)
	require.NotNil(t, got)
	assert.Equal(t, agents[0].ID, *got, "round-robin must pick agent with oldest last_assigned_at")

	// And last_assigned_at on the chosen agent should have advanced.
	var tm models.TeamMember
	require.NoError(t, db.Where("team_id = ? AND user_id = ?", team.ID, agents[0].ID).First(&tm).Error)
	require.NotNil(t, tm.LastAssignedAt)
	assert.True(t, tm.LastAssignedAt.After(now.Add(-3*time.Hour)), "last_assigned_at should be updated on assignment")
}

func TestAssignToTeam_RoundRobin_NullsFirst(t *testing.T) {
	// Members with NULL last_assigned_at must be picked before any member with a stamp.
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 2, 0)

	// agents[0] has a stamp, agents[1] is NULL → agents[1] should win.
	require.NoError(t, db.Model(&models.TeamMember{}).
		Where("team_id = ? AND user_id = ?", team.ID, agents[0].ID).
		Update("last_assigned_at", time.Now().Add(-time.Hour)).Error)

	got := a.AssignToTeam(team.ID, org.ID, nil, nil)
	require.NotNil(t, got)
	assert.Equal(t, agents[1].ID, *got, "NULLS FIRST: never-assigned agent must be picked first")
}

func TestAssignToTeam_RoundRobin_SkipsExcludedAgents(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 2, 0)

	got := a.AssignToTeam(team.ID, org.ID, []uuid.UUID{agents[0].ID}, nil)
	require.NotNil(t, got)
	assert.Equal(t, agents[1].ID, *got, "excluded agent must not be picked")
}

func TestAssignToTeam_RoundRobin_SkipsUnavailable(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 2, 0)

	setUnavailable(t, db, agents[0].ID)

	got := a.AssignToTeam(team.ID, org.ID, nil, nil)
	require.NotNil(t, got)
	assert.Equal(t, agents[1].ID, *got)
}

func TestAssignToTeam_RoundRobin_SkipsInactive(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 2, 0)

	setInactive(t, db, agents[0].ID)

	got := a.AssignToTeam(team.ID, org.ID, nil, nil)
	require.NotNil(t, got)
	assert.Equal(t, agents[1].ID, *got)
}

func TestAssignToTeam_RoundRobin_AllUnavailableReturnsNil(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 2, 0)

	for _, u := range agents {
		setUnavailable(t, db, u.ID)
	}

	got := a.AssignToTeam(team.ID, org.ID, nil, nil)
	assert.Nil(t, got)
}

func TestAssignToTeam_RoundRobin_AllExcludedReturnsNil(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 2, 0)

	exclude := []uuid.UUID{agents[0].ID, agents[1].ID}
	got := a.AssignToTeam(team.ID, org.ID, exclude, nil)
	assert.Nil(t, got)
}

func TestAssignToTeam_NonexistentTeamReturnsNil(t *testing.T) {
	a, _ := newAssigner(t)
	got := a.AssignToTeam(uuid.New(), uuid.New(), nil, nil)
	assert.Nil(t, got)
}

func TestAssignToTeam_TeamWithNoAgentsReturnsNil(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, _ := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 0, 0)

	got := a.AssignToTeam(team.ID, org.ID, nil, nil)
	assert.Nil(t, got)
}

// --- AssignToTeam: load_balanced ---

func TestAssignToTeam_LoadBalanced_PicksLowestLoad(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyLoadBalanced, 3, 0)

	// Provide a deterministic counter: agent[0]=5, agent[1]=2, agent[2]=8 → expect agent[1].
	counter := func(_ *gorm.DB, _ uuid.UUID, ids []uuid.UUID) map[uuid.UUID]int64 {
		return map[uuid.UUID]int64{
			agents[0].ID: 5,
			agents[1].ID: 2,
			agents[2].ID: 8,
		}
	}

	got := a.AssignToTeam(team.ID, org.ID, nil, counter)
	require.NotNil(t, got)
	assert.Equal(t, agents[1].ID, *got)
}

func TestAssignToTeam_LoadBalanced_TreatsMissingAsZero(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyLoadBalanced, 2, 0)

	// Counter only returns load for agent[0]; agent[1] missing → treated as 0.
	counter := func(_ *gorm.DB, _ uuid.UUID, _ []uuid.UUID) map[uuid.UUID]int64 {
		return map[uuid.UUID]int64{agents[0].ID: 3}
	}

	got := a.AssignToTeam(team.ID, org.ID, nil, counter)
	require.NotNil(t, got)
	assert.Equal(t, agents[1].ID, *got)
}

func TestAssignToTeam_LoadBalanced_AllUnavailableReturnsNil(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyLoadBalanced, 2, 0)
	for _, u := range agents {
		setUnavailable(t, db, u.ID)
	}

	called := false
	counter := func(_ *gorm.DB, _ uuid.UUID, _ []uuid.UUID) map[uuid.UUID]int64 {
		called = true
		return nil
	}

	got := a.AssignToTeam(team.ID, org.ID, nil, counter)
	assert.Nil(t, got)
	assert.False(t, called, "load counter should not be called when no agents are available")
}

// --- AssignToTeam: manual ---

func TestAssignToTeam_Manual_AlwaysReturnsNil(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, _ := createTeam(t, db, org.ID, models.AssignmentStrategyManual, 3, 0)

	got := a.AssignToTeam(team.ID, org.ID, nil, nil)
	assert.Nil(t, got, "manual strategy must never auto-assign")
}

// --- GetAvailableAgents (broadcast fallback) ---

func TestGetAvailableAgents_ReturnsAvailableMinusExcluded(t *testing.T) {
	a, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	team, agents := createTeam(t, db, org.ID, models.AssignmentStrategyRoundRobin, 4, 0)

	setUnavailable(t, db, agents[1].ID)

	got := a.GetAvailableAgents(team.ID, []uuid.UUID{agents[2].ID})
	// agents[0] available, [1] unavailable, [2] excluded, [3] available → expect [0] and [3].
	assert.ElementsMatch(t, []uuid.UUID{agents[0].ID, agents[3].ID}, got)
}

func TestGetAvailableAgents_NonexistentTeamReturnsNil(t *testing.T) {
	a, _ := newAssigner(t)
	assert.Nil(t, a.GetAvailableAgents(uuid.New(), nil))
}

// --- ResolvePerAgentTimeout ---

func TestResolvePerAgentTimeout(t *testing.T) {
	cases := []struct {
		name          string
		teamTimeout   int
		globalDefault int
		want          int
	}{
		{"team override wins", 30, 60, 30},
		{"team zero falls back to global", 0, 60, 60},
		{"both zero falls back to baseline", 0, 0, 15},
		{"negative team treated as unset", -5, 60, 60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := assignment.ResolvePerAgentTimeout(tc.teamTimeout, 0, tc.globalDefault)
			assert.Equal(t, tc.want, got)
		})
	}
}

// --- ChatLoadCounter / CallLoadCounter ---

func TestChatLoadCounter_CountsActiveTransfersPerAgent(t *testing.T) {
	_, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	a1 := testutil.CreateTestUser(t, db, org.ID)
	a2 := testutil.CreateTestUser(t, db, org.ID)
	contact := testutil.CreateTestContact(t, db, org.ID)

	// 2 active transfers for a1, 1 for a2, 1 completed for a1 (must not count).
	for range 2 {
		require.NoError(t, db.Create(&models.AgentTransfer{
			BaseModel:      models.BaseModel{ID: uuid.New()},
			OrganizationID: org.ID,
			ContactID:      contact.ID,
			AgentID:        &a1.ID,
			Status:         models.TransferStatusActive,
		}).Error)
	}
	require.NoError(t, db.Create(&models.AgentTransfer{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		ContactID:      contact.ID,
		AgentID:        &a2.ID,
		Status:         models.TransferStatusActive,
	}).Error)
	require.NoError(t, db.Create(&models.AgentTransfer{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		ContactID:      contact.ID,
		AgentID:        &a1.ID,
		Status:         models.TransferStatusResumed, // not "active" → must NOT count
	}).Error)

	loads := assignment.ChatLoadCounter(db, org.ID, []uuid.UUID{a1.ID, a2.ID})
	assert.Equal(t, int64(2), loads[a1.ID])
	assert.Equal(t, int64(1), loads[a2.ID])
}

func TestChatLoadCounter_CrossOrgIsolation(t *testing.T) {
	_, db := newAssigner(t)
	orgA := testutil.CreateTestOrganization(t, db)
	orgB := testutil.CreateTestOrganization(t, db)
	a1 := testutil.CreateTestUser(t, db, orgA.ID)
	contactA := testutil.CreateTestContact(t, db, orgA.ID)
	contactB := testutil.CreateTestContact(t, db, orgB.ID)

	require.NoError(t, db.Create(&models.AgentTransfer{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgA.ID,
		ContactID:      contactA.ID,
		AgentID:        &a1.ID,
		Status:         models.TransferStatusActive,
	}).Error)
	require.NoError(t, db.Create(&models.AgentTransfer{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgB.ID,
		ContactID:      contactB.ID,
		AgentID:        &a1.ID, // same agent ID, different org
		Status:         models.TransferStatusActive,
	}).Error)

	loads := assignment.ChatLoadCounter(db, orgA.ID, []uuid.UUID{a1.ID})
	assert.Equal(t, int64(1), loads[a1.ID], "load count must not leak across organizations")
}

func TestCallLoadCounter_OnlyWaitingAndConnected(t *testing.T) {
	_, db := newAssigner(t)
	org := testutil.CreateTestOrganization(t, db)
	agent := testutil.CreateTestUser(t, db, org.ID)
	contact := testutil.CreateTestContact(t, db, org.ID)

	callLog := &models.CallLog{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		ContactID:      contact.ID,
		Status:         models.CallStatusAnswered,
		CallerPhone:    contact.PhoneNumber,
	}
	require.NoError(t, db.Create(callLog).Error)

	mkTransfer := func(status models.CallTransferStatus) {
		require.NoError(t, db.Create(&models.CallTransfer{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  org.ID,
			CallLogID:       callLog.ID,
			ContactID:       contact.ID,
			WhatsAppCallID:  "wa-" + uuid.New().String()[:8],
			CallerPhone:     contact.PhoneNumber,
			WhatsAppAccount: "test-account",
			AgentID:         &agent.ID,
			Status:          status,
		}).Error)
	}
	mkTransfer(models.CallTransferStatusWaiting)
	mkTransfer(models.CallTransferStatusConnected)
	mkTransfer(models.CallTransferStatusCompleted) // must NOT count
	mkTransfer(models.CallTransferStatusAbandoned) // must NOT count

	loads := assignment.CallLoadCounter(db, org.ID, []uuid.UUID{agent.ID})
	assert.Equal(t, int64(2), loads[agent.ID], "only waiting + connected should be counted")
}
