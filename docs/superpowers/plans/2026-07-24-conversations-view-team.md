# conversations:view_team (Team-Scoped Visibility) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in `conversations:view_team` permission so a supervisor sees and acts on every conversation owned by agents in the teams they belong to — and nothing outside them.

**Architecture:** Strictly additive change to the existing strict-visibility rule. One reusable primitive defines "owner is in my team scope" — `canViewTeamMember` in Go and the equivalent `viewTeamScope` subquery in SQL. Only two branches (active-transfer-to-agent and carteira) gain an extra OR condition, gated by the permission. `authorizeConversation` (function) and `scopeVisibleConversations` (SQL) stay mirrored, guarded by the existing `TestVisibilityScopeMatchesFunction` oracle.

**Tech Stack:** Go 1.25, GORM, PostgreSQL. Tests with testify, harness in `internal/handlers/*_test.go` and `test/testutil`.

**Spec:** `docs/superpowers/specs/2026-07-24-view-team-supervisor-design.md`

## Global Constraints

- Change is **strictly additive**: the six existing scope branches (A–F) and the existing function branches stay semantically identical. (spec §4)
- `conversations:view_all` continues to short-circuit with global access **before** any `view_team` consideration. `view_team` is only consulted when the user lacks `view_all`. (spec §4)
- The new permission is added to `DefaultPermissions()` but to **no** system role in `SystemRolePermissions()`. It stays inert until an admin assigns it. (spec §9)
- "Owner is in my team scope" is defined in exactly **one** place per side: `canViewTeamMember` (Go) and `viewTeamScope` (SQL). They must denote the same set; the oracle test guards it. (spec §5)
- "Active membership" = a `team_members` row not soft-deleted (GORM default scope). No special-casing needed today. (spec §5)
- Constant: `ActionViewTeam = "view_team"`. Permission key: `conversations:view_team`.
- All new visibility behavior applies only in **strict** mode (`strict_conversation_visibility = true`); the flag-off path is untouched.

---

### Task 1: Add the `conversations:view_team` permission to the catalog

**Files:**
- Modify: `internal/models/roles.go` (add `ActionViewTeam` constant near `ActionViewAll`; add one `DefaultPermissions()` entry)
- Test: `internal/models/models_test.go`

**Interfaces:**
- Produces: constant `models.ActionViewTeam` (value `"view_team"`); a `DefaultPermissions()` entry `{Resource: ResourceConversations, Action: ActionViewTeam}`.

- [ ] **Step 1: Write the failing test**

Add to `internal/models/models_test.go`:

```go
func TestViewTeamPermissionInCatalogButNotDefaultRoles(t *testing.T) {
	// It must exist in the catalog so admins can assign it.
	found := false
	for _, p := range models.DefaultPermissions() {
		if p.Resource == models.ResourceConversations && p.Action == models.ActionViewTeam {
			found = true
		}
	}
	assert.True(t, found, "conversations:view_team must be in DefaultPermissions")

	// It must NOT be granted to manager or agent by default.
	roles := models.SystemRolePermissions()
	assert.NotContains(t, roles["manager"], "conversations:view_team")
	assert.NotContains(t, roles["agent"], "conversations:view_team")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/models/ -run TestViewTeamPermissionInCatalogButNotDefaultRoles -count=1`
Expected: FAIL — `ActionViewTeam` undefined (compile error) or `found` is false.

- [ ] **Step 3: Add the constant**

In `internal/models/roles.go`, in the action `const` block (currently ends with `ActionViewAll = "view_all"`), add:

```go
	ActionViewAll = "view_all"
	ActionViewTeam = "view_team"
```

- [ ] **Step 4: Add the catalog entry**

In `internal/models/roles.go`, `DefaultPermissions()`, immediately after the existing conversations line:

```go
		// Conversations
		{Resource: ResourceConversations, Action: ActionViewAll, Description: "View and act on all conversations, including those assigned to other agents"},
		{Resource: ResourceConversations, Action: ActionViewTeam, Description: "View and act on all conversations of the teams the user belongs to"},
```

Do **not** modify `SystemRolePermissions()`. (The `admin` mapping uses `allPermissions`, which now includes this one — harmless, since admin already has `view_all`. `manager`/`agent` lists are explicit and unchanged.)

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/models/ -run TestViewTeamPermissionInCatalogButNotDefaultRoles -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/models/roles.go internal/models/models_test.go
git commit -m "feat(rbac): add conversations:view_team permission to the catalog"
```

---

### Task 2: Add the `canViewTeamMember` primitive (Go)

**Files:**
- Modify: `internal/handlers/conversation_visibility.go` (add the helper near `userInTeam`)
- Test: `internal/handlers/conversation_visibility_test.go`

**Interfaces:**
- Produces: `func (a *App) canViewTeamMember(viewerID, ownerID uuid.UUID) bool` — true iff `viewerID` and `ownerID` have an active membership in a common team (true when `viewerID == ownerID` and the viewer has at least one team).

- [ ] **Step 1: Write the failing test**

Add to `internal/handlers/conversation_visibility_test.go`:

```go
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
```

Add the test-only exported wrapper to `internal/handlers/export_test.go` (this file already exposes `CanViewConversationForTest`):

```go
func (a *App) CanViewTeamMemberForTest(viewerID, ownerID uuid.UUID) bool {
	return a.canViewTeamMember(viewerID, ownerID)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handlers/ -run TestCanViewTeamMember -count=1`
Expected: FAIL — `canViewTeamMember` / `CanViewTeamMemberForTest` undefined.

- [ ] **Step 3: Implement the helper**

In `internal/handlers/conversation_visibility.go`, add after `userInTeam`:

```go
// canViewTeamMember reports whether viewerID may see conversations owned by
// ownerID by virtue of team scope: there is at least one team in which BOTH
// have an active membership. It is the single Go definition of "owner is in my
// team scope" — its SQL twin is the viewTeamScope subquery in
// scopeVisibleConversations. Keep the two in sync (TestVisibilityScopeMatchesFunction).
func (a *App) canViewTeamMember(viewerID, ownerID uuid.UUID) bool {
	var count int64
	a.DB.Model(&models.TeamMember{}).
		Where("user_id = ? AND team_id IN (?)", ownerID,
			a.DB.Model(&models.TeamMember{}).Select("team_id").Where("user_id = ?", viewerID)).
		Count(&count)
	return count > 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handlers/ -run TestCanViewTeamMember -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/conversation_visibility.go internal/handlers/conversation_visibility_test.go internal/handlers/export_test.go
git commit -m "feat(visibility): add canViewTeamMember team-scope primitive"
```

---

### Task 3: Widen the function and SQL for `view_team` (atomic, oracle-guarded)

This task changes `authorizeConversation` and `scopeVisibleConversations` **together** — they must stay mirrored, and the oracle test fails if only one is changed.

**Files:**
- Modify: `internal/handlers/conversation_visibility.go` (`authorizeConversation` branches A & D; `scopeVisibleConversations` add G & H)
- Test: `internal/handlers/conversation_visibility_test.go`

**Interfaces:**
- Consumes: `canViewTeamMember` (Task 2); `models.ActionViewTeam` (Task 1); existing `a.HasPermission`, `activeTransferFor`, `userInTeam`.
- Produces: `view_team` holders see team-mate-owned conversations via function + SQL.

- [ ] **Step 1: Write the failing behavior test**

Add to `internal/handlers/conversation_visibility_test.go`. Helper first:

```go
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

	teamStore := createTeamWithMember(t, app, org.ID, agentA.ID)     // store team, agentA in it
	teamOther := createTeamWithMember(t, app, org.ID, agentB.ID)     // other store team, agentB in it
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

	// 3. carteira of agentB (other team) -> supervisor does NOT see
	carteiraB := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(carteiraB).Update("assigned_user_id", agentB.ID).Error)
	assert.False(t, app.CanViewConversationForTest(supID, org.ID, load(carteiraB)))

	// 4. multi-team supervisor (both stores) sees both agents' carteiras
	multiSupID := makeViewTeamSupervisor(t, app, org.ID, teamStore.ID, teamOther.ID)
	assert.True(t, app.CanViewConversationForTest(multiSupID, org.ID, load(carteiraA)))
	assert.True(t, app.CanViewConversationForTest(multiSupID, org.ID, load(carteiraB)))

	// 5. owner with no team -> not visible to the supervisor
	teamless := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	orphan := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(orphan).Update("assigned_user_id", teamless.ID).Error)
	assert.False(t, app.CanViewConversationForTest(supID, org.ID, load(orphan)))

	// 6. supervisor with view_team but NO team -> no extra access (cannot see agentA's carteira)
	teamlessSupID := makeViewTeamSupervisor(t, app, org.ID) // no teams
	assert.False(t, app.CanViewConversationForTest(teamlessSupID, org.ID, load(carteiraA)))

	// 7. regression: a plain agent (no view_team) does NOT see a co-member's carteira
	assert.False(t, app.CanViewConversationForTest(agentA.ID, org.ID, load(carteiraB)))

	// 8. view_all + view_team together: view_all wins (sees a foreign-team conversation).
	bothRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "both",
		[]string{"conversations:view_all", "conversations:view_team", "chat:read"})
	both := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&bothRole.ID))
	assert.True(t, app.CanViewConversationForTest(both.ID, org.ID, load(carteiraB)),
		"view_all still grants global access when view_team is also present")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handlers/ -run TestTeamScopedVisibility -count=1`
Expected: FAIL — assertions 1, 2, 4 are false (supervisor cannot yet see team-mate-owned conversations).

- [ ] **Step 3: Widen the function branches A and D**

In `internal/handlers/conversation_visibility.go`, in `authorizeConversation`, inside the strict block, right **after** the `view_all` check and **before** `transfer, hasActive := ...`, add:

```go
	// view_team widens exactly two branches (A: transfer-to-agent, D: carteira)
	// to "owner shares a team with me". Only consulted for non-view_all users.
	hasViewTeam := a.HasPermission(userID, models.ResourceConversations, models.ActionViewTeam, orgID)
```

Then change branch **A** (the `transfer.AgentID != nil` case):

```go
		case transfer.AgentID != nil:
			ok := *transfer.AgentID == userID ||
				(hasViewTeam && a.canViewTeamMember(userID, *transfer.AgentID))
			return conversationAccess{canView: ok, canInteract: ok}
```

And change branch **D** (the `contact.AssignedUserID != nil` block):

```go
	if contact.AssignedUserID != nil {
		ok := *contact.AssignedUserID == userID ||
			(hasViewTeam && a.canViewTeamMember(userID, *contact.AssignedUserID))
		return conversationAccess{canView: ok, canInteract: ok}
	}
```

Leave branches B, C, E, F unchanged.

- [ ] **Step 4: Add the mirrored G and H disjuncts in SQL**

In `internal/handlers/conversation_visibility.go`, in `scopeVisibleConversations`, replace the single `return query.Where(a.DB.Where(...)...)` at the end of the strict block with a built-up condition plus the gated additions. The existing `myTeams`, `activeSub`, `activeAgentMine`, `activeTeamMine`, `activeGeneral`, `acctDefault` definitions stay as-is above.

```go
	cond := a.DB.
		Where("id IN (?)", activeAgentMine).                                     // A
		Or("id IN (?)", activeTeamMine).                                         // B
		Or(a.DB.Where("id IN (?)", activeGeneral).Where(acctDefault, myTeams)).  // C
		Or(a.DB.Where("id NOT IN (?)", activeSub).Where("assigned_user_id = ?", userID)). // D
		Or(a.DB.Where("id NOT IN (?)", activeSub).
			Where("assigned_user_id IS NULL AND team_id IN (?)", myTeams)). // E
		Or(a.DB.Where("id NOT IN (?)", activeSub).
			Where("assigned_user_id IS NULL AND team_id IS NULL").Where(acctDefault, myTeams)) // F

	// view_team: the team-scoped analogues of A and D, gated by the permission.
	// viewTeamScope = users who share a team with me (the SQL twin of
	// canViewTeamMember). Emitted only when the user holds view_team.
	if a.HasPermission(userID, models.ResourceConversations, models.ActionViewTeam, orgID) {
		viewTeamScope := a.DB.Model(&models.TeamMember{}).Select("user_id").
			Where("team_id IN (?)", myTeams)
		activeAgentTeam := a.DB.Model(&models.AgentTransfer{}).Select("contact_id").
			Where("organization_id = ? AND status = ? AND agent_id IN (?)",
				orgID, models.TransferStatusActive, viewTeamScope)
		cond = cond.
			Or("id IN (?)", activeAgentTeam). // G: active transfer to a team-mate agent
			Or(a.DB.Where("id NOT IN (?)", activeSub).
				Where("assigned_user_id IN (?)", viewTeamScope)) // H: carteira of a team-mate agent
	}

	return query.Where(cond)
```

- [ ] **Step 5: Run the behavior test to verify it passes**

Run: `go test ./internal/handlers/ -run TestTeamScopedVisibility -count=1`
Expected: PASS

- [ ] **Step 6: Extend the oracle test with a view_team viewer**

The oracle (`TestVisibilityScopeMatchesFunction`) derives `expected` from the function and `got` from the SQL, so a new viewer with `view_team` over the same fixtures auto-checks that function and SQL agree. Add a sibling test that reuses the same shape but with a `view_team` supervisor sharing a team with `otherAgent`:

```go
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
```

- [ ] **Step 7: Run the oracle + behavior tests**

Run: `go test ./internal/handlers/ -run 'TestVisibilityScopeMatchesFunction|TestTeamScopedVisibility|TestCanViewTeamMember' -count=1`
Expected: PASS (both oracle variants and the behavior test).

- [ ] **Step 8: Commit**

```bash
git add internal/handlers/conversation_visibility.go internal/handlers/conversation_visibility_test.go
git commit -m "feat(visibility): team-scoped view via conversations:view_team"
```

---

### Task 4: Audit `view_all` usages and run full regression

Confirms no other authorization site needs a parallel `view_team` branch, and that nothing existing changed.

**Files:**
- Modify: none expected. If the audit finds a site that gates access on `view_all` outside `scopeVisibleConversations`/`authorizeConversation` and would leak or over-hide for a `view_team` user, add a task before merging (out of scope here — record the finding).

**Interfaces:**
- Consumes: everything from Tasks 1–3.

- [ ] **Step 1: List every `view_all` usage**

Run: `grep -rn "ActionViewAll\|view_all" internal/handlers --include=*.go | grep -v _test.go`
Expected: a short list. For each hit, confirm it is either (a) inside `authorizeConversation`/`scopeVisibleConversations` (already handled), or (b) a genuinely global-only concern (e.g., an org-wide export or an "act on anyone's conversation" gate) where team-scope is intentionally **not** granted. Record the classification in the commit message.

- [ ] **Step 2: Confirm the flag-off path is untouched**

Run: `grep -n "StrictConversationVisibility" internal/handlers/conversation_visibility.go`
Expected: the early `if settings == nil || !...StrictConversationVisibility` returns are unchanged — `view_team` logic lives only in the strict branch.

- [ ] **Step 3: Run the full handlers + models suites**

Run: `go test ./internal/handlers/ ./internal/models/ -count=1`
Expected: PASS, except the two known pre-existing environment failures (`TestApp_ServeMedia_RejectsSymlink` — Windows symlink privilege; and the flaky `TestUpdateContactChatbotMessage_SetsTimestampAndResetsReminder`). Confirm no visibility/permission test fails.

- [ ] **Step 4: Build and vet**

Run: `go build ./... && go vet ./internal/handlers/`
Expected: no output (success).

- [ ] **Step 5: Commit the audit note**

```bash
git commit --allow-empty -m "chore(visibility): audit view_all sites; view_team is additive and strict-only

<paste the classified grep results here>"
```

---

## Self-Review

**Spec coverage:**
- §3 Semantics (view_team scoped to teams, view+act) → Task 3 (branches grant `canView==canInteract`; scope via `canViewTeamMember`).
- §4 Additive / view_all precedence → Task 3 (branches A/D only; `hasViewTeam` computed after view_all short-circuit) + Task 4 Step 2.
- §5 Single primitive + active membership + invariant → Task 2 (`canViewTeamMember`) + Task 3 (`viewTeamScope` twin) + oracle.
- §6.1/§6.2 function + SQL changes → Task 3 Steps 3–4.
- §6.3 edge cases (owner no team, supervisor no team, scope-follows-owner) → Task 3 test cases 5, 6 (owner/supervisor no team); scope-follows-owner is inherent to keying on `assigned_user_id`/`agent_id`.
- §6.4 mirroring/oracle → Task 3 Step 6.
- §8 tests (oracle + behavior + 0-team + view_all-wins) → Task 3 Step 1 (cases 1–8, incl. #6 supervisor-no-team, #8 view_all-wins) + Step 6 (oracle variant).
- §9 rollout (catalog, not in system roles, seeding) → Task 1 + `SeedPermissionsAndRoles` idempotency (existing).

**Placeholder scan:** none — all steps contain concrete code and commands. The one intentional fill-in is the grep output pasted into Task 4 Step 5's commit message.

**Type consistency:** `canViewTeamMember(viewerID, ownerID uuid.UUID) bool` and `CanViewTeamMemberForTest` used consistently across Tasks 2–3. `models.ActionViewTeam` string `"view_team"` and key `conversations:view_team` consistent across Tasks 1, 3, and tests. `viewTeamScope` is a local subquery in `scopeVisibleConversations`, matching the spec name.

**Gap check:** no remaining gaps. Every spec §8 test case maps to a step (behavior cases 1–8 in Task 3 Step 1; oracle agreement in Step 6; full regression in Task 4). The `view_team + view_all` precedence case is Task 3 Step 1 case 8.
