# CRM: PUT sem full-replace da descrição, e tempo real por WebSocket — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the PUT that silently wipes `description` on `Occurrence`, and add WebSocket real-time updates (`occurrence_changed`, `occurrence_event_created`) to the CRM's Kanban board and case-detail page.

**Architecture:** Backend: `Description` becomes `*string` on `UpdateOccurrenceRequest` (same pattern already used for `AssignedUserID`); `CreateOccurrence`, `UpdateOccurrence`, `ChangeOccurrenceStage` and `CreateOccurrenceEvent` each broadcast via the existing `Hub.BroadcastToOrg`, reusing `OccurrenceResponse`/`OccurrenceEventResponse` as the payloads — no new WS-specific DTOs. Frontend: `websocket.ts` gains two subscribe/unsubscribe methods mirroring the existing `onCampaignStatsUpdate` pattern; `OccurrenceBoard.vue` and `OccurrenceDetailView.vue` (both of which hold their state in local component refs, not the Pinia store) subscribe directly on mount and unsubscribe on unmount.

**Tech Stack:** Go 1.25, fastglue, GORM, Postgres; Vue 3 `<script setup>` + TypeScript + Pinia; Playwright E2E.

## Global Constraints

- Every occurrence broadcast fires strictly **after** persistence succeeds — never before, never on a failed write.
- No new WebSocket-specific payload struct: broadcasts reuse `OccurrenceResponse` (already returned by REST) and `OccurrenceEventResponse` (already returned by REST), both defined in `internal/handlers/occurrences.go`.
- No changes to `visibleOccurrences`, `resolveAssignee`, `loadAuthorizedOccurrence`, `set_contact`, or `BroadcastToContact` semantics.
- `OccurrencesView.vue` (the list) subscribes to nothing — it is explicitly out of scope.
- Source spec: `docs/superpowers/specs/2026-09-03-crm-put-e-tempo-real-design.md`.

**Two refinements this plan makes to the approved spec, both required for it to actually work, neither changing what was approved in spirit:**

1. **`OccurrenceEventResponse` gains an `occurrence_id` field.** The spec says "no new DTO — reuse what REST already returns," but REST scopes both endpoints that return this struct by URL (`/occurrences/{id}/events`), so the struct itself never needed to say which occurrence it belongs to. `occurrence_event_created` broadcasts fan out to the whole organization via `BroadcastToOrg`, and the spec's own §2 requires the detail page to "process only when `occurrence_id` matches the one open on screen" — that check is impossible without the field. Adding it is additive and harmless to existing REST consumers.
2. **The "closed" `OccurrenceEvent` created inside `ChangeOccurrenceStage` when a case enters a closing stage does NOT get its own `occurrence_event_created` broadcast.** The spec's §4 verification is explicit and was reviewed twice by the user: "ChangeOccurrenceStage dispara **exatamente um** `occurrence_changed` e **exatamente um** `occurrence_event_created`." The code writes up to two `OccurrenceEvent` rows on a closing transition (`stage_change` + `closed`); only the `stage_change` one broadcasts. The `closed` row is still written to the database unchanged — it just doesn't get a live echo, and shows up on the next `fetchEvents`.

---

## Task 1: Backend — PUT `description` stops wiping the field when omitted

**Files:**
- Modify: `internal/handlers/occurrences.go:292-297` (struct), `:398-401` (updates map)
- Test: `internal/handlers/occurrences_detail_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `UpdateOccurrenceRequest.Description` becomes `*string` — later tasks in this plan do not touch it, but any other future caller of this struct must know it is now a pointer.

- [ ] **Step 1: Write the failing test**

Append to `internal/handlers/occurrences_detail_test.go` (same file that already has `TestOccurrences_UpdateAssigneeAbsentVsEmpty`, whose pattern this mirrors):

```go
// PUT sem description preserva o valor existente; PUT com description:""
// apaga; PUT com description preenchido grava o novo valor. Mesmo padrão de
// TestOccurrences_UpdateAssigneeAbsentVsEmpty.
func TestOccurrences_UpdateDescriptionAbsentVsEmpty(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Original",
		Description: "Descrição original", StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	// PUT without description: it must be preserved.
	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"title": "Atualizado"})
	require.NoError(t, app.UpdateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var afterAbsent models.Occurrence
	require.NoError(t, app.DB.First(&afterAbsent, "id = ?", occ.ID).Error)
	assert.Equal(t, "Descrição original", afterAbsent.Description,
		"PUT sem description não deve apagar a descrição existente")

	// PUT with description: "": it must be cleared.
	req2 := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{
		"title": "Atualizado", "description": "",
	})
	require.NoError(t, app.UpdateOccurrence(req2))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req2))

	var afterEmpty models.Occurrence
	require.NoError(t, app.DB.First(&afterEmpty, "id = ?", occ.ID).Error)
	assert.Empty(t, afterEmpty.Description, `PUT com description:"" deve apagar a descrição`)

	// PUT with description filled: regression check — a client that always
	// sends the field (any form that loads the record before editing) must
	// see no change in behavior.
	req3 := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{
		"title": "Atualizado", "description": "Nova descrição",
	})
	require.NoError(t, app.UpdateOccurrence(req3))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req3))

	var afterFilled models.Occurrence
	require.NoError(t, app.DB.First(&afterFilled, "id = ?", occ.ID).Error)
	assert.Equal(t, "Nova descrição", afterFilled.Description)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handlers/... -run TestOccurrences_UpdateDescriptionAbsentVsEmpty -v`
Expected: FAIL on the first assertion — `Descrição original` becomes `""`, because `Description` is currently a plain `string` and an omitted JSON field decodes to the zero value, indistinguishable from an explicit `""`.

- [ ] **Step 3: Write minimal implementation**

In `internal/handlers/occurrences.go`, change the struct at line 292:

```go
// Before:
type UpdateOccurrenceRequest struct {
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Priority       string  `json:"priority"`
	AssignedUserID *string `json:"assigned_user_id"`
}

// After:
type UpdateOccurrenceRequest struct {
	Title          string  `json:"title"`
	Description    *string `json:"description"`
	Priority       string  `json:"priority"`
	AssignedUserID *string `json:"assigned_user_id"`
}
```

And the `updates` map inside `UpdateOccurrence` at line 398:

```go
// Before:
	updates := map[string]any{
		"title":       req.Title,
		"description": req.Description,
	}

// After:
	updates := map[string]any{
		"title": req.Title,
	}
	// req.Description != nil is what says the field participated in the
	// request at all, mirroring req.AssignedUserID below. A PUT that omits
	// description leaves it untouched; only an explicit "" clears it.
	if req.Description != nil {
		updates["description"] = *req.Description
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handlers/... -run TestOccurrences_UpdateDescriptionAbsentVsEmpty -v`
Expected: PASS

- [ ] **Step 5: Run the full occurrences test suite to check for regressions**

Run: `go test ./internal/handlers/... -run TestOccurrences -v`
Expected: PASS (all existing tests, including `TestOccurrences_UpdateAssigneeAbsentVsEmpty`, still pass — this task did not touch assignee handling)

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/occurrences.go internal/handlers/occurrences_detail_test.go
git commit -m "fix(crm): PUT sem description nao apaga mais o campo"
```

---

## Task 2: Backend — WebSocket test harness, and `occurrence_changed` on `CreateOccurrence`

**Files:**
- Modify: `internal/websocket/messages.go` (new type constants)
- Modify: `internal/handlers/occurrences.go` (import + broadcast in `CreateOccurrence`)
- Create: `internal/handlers/occurrences_broadcast_test.go`

**Interfaces:**
- Consumes: `websocket.Hub.BroadcastToOrg(orgID uuid.UUID, msg websocket.WSMessage)`, `websocket.NewHub(log logf.Logger) *Hub`, `websocket.NewClient(hub *Hub, conn *websocket.Conn, userID, orgID uuid.UUID) *Client` (conn may be `nil` for tests), `Hub.Register(*Client)`, `Hub.GetClientCount() int`, `Client.SendChan() <-chan []byte` — all already exported by `internal/websocket` (see `internal/websocket/websocket_test.go`'s `newTestHub`/`newTestClient` for the same pattern used there). `testutil.NopLogger() logf.Logger`.
- Produces: `websocket.TypeOccurrenceChanged` and `websocket.TypeOccurrenceEventCreated` string constants (used by every later task in this plan); test helpers `newTestHubWithClient(t, orgID) (*websocket.Hub, *websocket.Client)`, `readBroadcast(t, client, expectedType) websocket.WSMessage`, `assertNoBroadcast(t, client)` in package `handlers_test` (used by Tasks 3-5).

- [ ] **Step 1: Add the two new WebSocket message types**

In `internal/websocket/messages.go`, insert after the `Permission types` block (after `TypePermissionsUpdated = "permissions_updated"`):

```go
	// Occurrence types
	TypeOccurrenceChanged      = "occurrence_changed"
	TypeOccurrenceEventCreated = "occurrence_event_created"
```

- [ ] **Step 2: Write the test harness and the first failing test**

Create `internal/handlers/occurrences_broadcast_test.go`:

```go
package handlers_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// newTestHubWithClient starts a real Hub — the same type App.WSHub points at
// in production — and registers a single client in orgID, so a test can
// assert on exactly what a client in that org receives. Mirrors
// newTestHub/newTestClient in internal/websocket/websocket_test.go;
// duplicated here rather than imported because handlers_test is a different
// package and the websocket package already exports everything this needs
// (NewHub, NewClient, Register, GetClientCount, SendChan).
func newTestHubWithClient(t *testing.T, orgID uuid.UUID) (*websocket.Hub, *websocket.Client) {
	t.Helper()
	hub := websocket.NewHub(testutil.NopLogger())
	go hub.Run()

	client := websocket.NewClient(hub, nil, uuid.New(), orgID)
	hub.Register(client)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() != 1 {
		time.Sleep(5 * time.Millisecond)
	}
	require.Equal(t, 1, hub.GetClientCount(), "client failed to register on the test hub")

	return hub, client
}

// readBroadcast reads the next message off client's send channel and asserts
// its type. Fails the test if nothing arrives within 2s.
func readBroadcast(t *testing.T, client *websocket.Client, expectedType string) websocket.WSMessage {
	t.Helper()
	select {
	case data := <-client.SendChan():
		var msg websocket.WSMessage
		require.NoError(t, json.Unmarshal(data, &msg))
		assert.Equal(t, expectedType, msg.Type)
		return msg
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for a %q broadcast", expectedType)
		return websocket.WSMessage{}
	}
}

// assertNoBroadcast fails the test if any message arrives within the window.
// Short on purpose: it only runs after the handler call under test has
// already returned, so there is nothing legitimate left to wait for.
func assertNoBroadcast(t *testing.T, client *websocket.Client) {
	t.Helper()
	select {
	case data := <-client.SendChan():
		t.Fatalf("expected no broadcast, got: %s", string(data))
	case <-time.After(150 * time.Millisecond):
	}
}

// CreateOccurrence broadcasts occurrence_changed, with the persisted
// occurrence_id and stage_id — the two fields the Kanban board uses to
// decide between updating, moving between columns, or just incrementing a
// counter (see occurrences.go's occurrenceToResponse / the design spec's §2).
func TestOccurrences_CreateBroadcastsOccurrenceChanged(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Broadcast na criação",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var occ models.Occurrence
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&occ).Error)

	msg := readBroadcast(t, client, websocket.TypeOccurrenceChanged)
	payload, ok := msg.Payload.(map[string]any)
	require.True(t, ok, "payload deve ser um objeto")
	assert.Equal(t, occ.ID.String(), payload["id"], "occurrence_id do broadcast deve bater com o persistido")
	assert.Equal(t, occ.StageID.String(), payload["stage_id"], "stage_id do broadcast deve bater com o persistido")
}

// A falha de validação (assignee inexistente) barra a criação antes de
// qualquer persistência — e, por isso, não pode disparar broadcast nenhum.
func TestOccurrences_CreateFailureBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(), "title": "Não deve emitir nada",
		"assigned_user_id": uuid.New().String(),
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateOccurrence(req))
	require.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/handlers/... -run TestOccurrences_Create.*Broadcast -v`
Expected: `TestOccurrences_CreateBroadcastsOccurrenceChanged` FAILs with a timeout in `readBroadcast` (no broadcast is sent yet). `TestOccurrences_CreateFailureBroadcastsNothing` currently PASSes vacuously (nothing broadcasts because nothing broadcasts at all yet) — that is expected and will stay true after Step 4 too, it just becomes meaningful instead of trivial.

- [ ] **Step 4: Wire the broadcast into `CreateOccurrence`**

In `internal/handlers/occurrences.go`, add the import:

```go
import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)
```

Then change the end of `CreateOccurrence`:

```go
// Before:
	occ.Stage = stage
	occ.Contact = contact
	return r.SendEnvelope(occurrenceToResponse(occ))
}

// After:
	occ.Stage = stage
	occ.Contact = contact
	resp := occurrenceToResponse(occ)

	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type:    websocket.TypeOccurrenceChanged,
			Payload: resp,
		})
	}

	return r.SendEnvelope(resp)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/handlers/... -run TestOccurrences_Create.*Broadcast -v`
Expected: PASS

- [ ] **Step 6: Run the full occurrences suite**

Run: `go test ./internal/handlers/... -run TestOccurrences -v`
Expected: PASS (no regressions — `App.WSHub` is `nil` in every other existing test, and every write path already guards on `a.WSHub != nil`, per the existing `typing_test.go` precedent)

- [ ] **Step 7: Commit**

```bash
git add internal/websocket/messages.go internal/handlers/occurrences.go internal/handlers/occurrences_broadcast_test.go
git commit -m "feat(crm): broadcast occurrence_changed na criacao de ocorrencia"
```

---

## Task 3: Backend — `occurrence_changed` on `UpdateOccurrence`

**Files:**
- Modify: `internal/handlers/occurrences.go` (end of `UpdateOccurrence`)
- Modify: `internal/handlers/occurrences_broadcast_test.go` (append tests)

**Interfaces:**
- Consumes: `newTestHubWithClient`, `readBroadcast`, `assertNoBroadcast` from Task 2. `authedJSON` from `internal/handlers/occurrences_detail_test.go` (same package, already in scope).
- Produces: nothing new for later tasks.

- [ ] **Step 1: Write the failing tests**

Append to `internal/handlers/occurrences_broadcast_test.go`:

```go
func TestOccurrences_UpdateBroadcastsOccurrenceChanged(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Original",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"title": "Atualizado"})
	require.NoError(t, app.UpdateOccurrence(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	msg := readBroadcast(t, client, websocket.TypeOccurrenceChanged)
	payload, ok := msg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), payload["id"])
	assert.Equal(t, "Atualizado", payload["title"])
}

// assigned_user_id inexistente barra o UPDATE antes de qualquer escrita.
func TestOccurrences_UpdateFailureBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Original",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{
		"title": "Não deve emitir nada", "assigned_user_id": uuid.New().String(),
	})
	require.NoError(t, app.UpdateOccurrence(req))
	require.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/... -run TestOccurrences_Update.*Broadcast -v`
Expected: `TestOccurrences_UpdateBroadcastsOccurrenceChanged` FAILs on timeout (no broadcast wired yet).

- [ ] **Step 3: Wire the broadcast into `UpdateOccurrence`**

In `internal/handlers/occurrences.go`, change the end of `UpdateOccurrence`:

```go
// Before:
	a.DB.Preload("Stage").Preload("AssignedUser").First(occ, occ.ID)
	return r.SendEnvelope(occurrenceToResponse(*occ))
}

// After:
	a.DB.Preload("Stage").Preload("AssignedUser").First(occ, occ.ID)
	resp := occurrenceToResponse(*occ)

	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type:    websocket.TypeOccurrenceChanged,
			Payload: resp,
		})
	}

	return r.SendEnvelope(resp)
}
```

(This replaces the return statement at the end of `UpdateOccurrence` — the function's earlier `req.AssignedUserID != nil` block and the assignment-event `a.DB.Create` above it are unchanged.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handlers/... -run TestOccurrences_Update.*Broadcast -v`
Expected: PASS

- [ ] **Step 5: Run the full occurrences suite**

Run: `go test ./internal/handlers/... -run TestOccurrences -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/occurrences.go internal/handlers/occurrences_broadcast_test.go
git commit -m "feat(crm): broadcast occurrence_changed na edicao de ocorrencia"
```

---

## Task 4: Backend — `occurrence_changed` + `occurrence_event_created` on `ChangeOccurrenceStage`

**Files:**
- Modify: `internal/handlers/occurrences.go` (`OccurrenceEventResponse` struct, `ListOccurrenceEvents`, `ChangeOccurrenceStage`)
- Modify: `internal/handlers/occurrences_broadcast_test.go` (append tests)

**Interfaces:**
- Consumes: same test harness as Task 3.
- Produces: `OccurrenceEventResponse.OccurrenceID uuid.UUID` (`json:"occurrence_id"`) — Task 5 must populate it too when it builds this struct for `CreateOccurrenceEvent`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/handlers/occurrences_broadcast_test.go`:

```go
// ChangeOccurrenceStage dispara os dois eventos, de tipos diferentes — não
// um "evento de WebSocket" contado uma vez. A mudança de etapa grava a
// ocorrência e o evento automático de timeline como duas escritas distintas,
// e cada uma tem seu próprio broadcast (spec §2, correção registrada).
func TestOccurrences_ChangeStageBroadcastsBothEventTypes(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var stages []models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, false).
		Order("position ASC").Find(&stages).Error)
	require.GreaterOrEqual(t, len(stages), 2,
		"precisa de duas etapas nao-fechadas para o teste nao se misturar com o evento closed")
	from, to := stages[0], stages[1]

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Muda de etapa",
		StageID: from.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": to.ID.String()})
	require.NoError(t, app.ChangeOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	changedMsg := readBroadcast(t, client, websocket.TypeOccurrenceChanged)
	changedPayload, ok := changedMsg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), changedPayload["id"])
	assert.Equal(t, to.ID.String(), changedPayload["stage_id"])

	eventMsg := readBroadcast(t, client, websocket.TypeOccurrenceEventCreated)
	eventPayload, ok := eventMsg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), eventPayload["occurrence_id"])
	assert.Equal(t, string(models.OccurrenceEventStageChange), eventPayload["type"])

	// Só os dois — a etapa de destino não é de fechamento, então nenhum
	// evento "closed" (e portanto nenhum terceiro broadcast) entra em jogo.
	assertNoBroadcast(t, client)
}

// Reenviar a etapa atual é no-op: nenhum broadcast de nenhum tipo.
func TestOccurrences_ChangeStageSameStageBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Mesma etapa",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": stage.ID.String()})
	require.NoError(t, app.ChangeOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}

// stage_id inexistente barra a operação antes de qualquer escrita.
func TestOccurrences_ChangeStageFailureBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Falha",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "PUT", occ.ID, map[string]any{"stage_id": uuid.New().String()})
	require.NoError(t, app.ChangeOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/... -run TestOccurrences_ChangeStage.*Broadcast -v`
Expected: `TestOccurrences_ChangeStageBroadcastsBothEventTypes` FAILs on timeout; the other two currently pass vacuously.

- [ ] **Step 3: Add `OccurrenceID` to `OccurrenceEventResponse` and populate it in `ListOccurrenceEvents`**

In `internal/handlers/occurrences.go`, change the struct:

```go
// Before:
// OccurrenceEventResponse is the API shape of a timeline entry.
type OccurrenceEventResponse struct {
	ID            uuid.UUID  `json:"id"`
	Type          string     `json:"type"`
	Content       string     `json:"content"`
	Metadata      any        `json:"metadata"`
	CreatedByID   *uuid.UUID `json:"created_by_id,omitempty"`
	CreatedByName string     `json:"created_by_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// After:
// OccurrenceEventResponse is the API shape of a timeline entry.
//
// OccurrenceID is redundant for both REST endpoints that return this struct
// (both are already scoped to one occurrence by the URL) but is required by
// occurrence_event_created WebSocket broadcasts, which fan out to the whole
// organization via BroadcastToOrg and need a way to say which occurrence's
// timeline they belong to.
type OccurrenceEventResponse struct {
	ID            uuid.UUID  `json:"id"`
	OccurrenceID  uuid.UUID  `json:"occurrence_id"`
	Type          string     `json:"type"`
	Content       string     `json:"content"`
	Metadata      any        `json:"metadata"`
	CreatedByID   *uuid.UUID `json:"created_by_id,omitempty"`
	CreatedByName string     `json:"created_by_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}
```

And in `ListOccurrenceEvents`:

```go
// Before:
	for i, e := range events {
		result[i] = OccurrenceEventResponse{
			ID:          e.ID,
			Type:        string(e.Type),
			Content:     e.Content,
			Metadata:    e.Metadata,
			CreatedByID: e.CreatedByID,
			CreatedAt:   e.CreatedAt,
		}

// After:
	for i, e := range events {
		result[i] = OccurrenceEventResponse{
			ID:           e.ID,
			OccurrenceID: e.OccurrenceID,
			Type:         string(e.Type),
			Content:      e.Content,
			Metadata:     e.Metadata,
			CreatedByID:  e.CreatedByID,
			CreatedAt:    e.CreatedAt,
		}
```

- [ ] **Step 4: Wire both broadcasts into `ChangeOccurrenceStage`**

Replace the tail of `ChangeOccurrenceStage`, from the successful `Updates` call onward:

```go
// Before:
	if err := a.DB.Model(occ).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to change occurrence stage", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to change stage", nil, "")
	}

	eventType := models.OccurrenceEventStageChange
	a.DB.Create(&models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           eventType,
		Content:        from.Name + " → " + target.Name,
		Metadata:       models.JSONB{"from_stage_id": from.ID.String(), "to_stage_id": target.ID.String()},
		CreatedByID:    &userID,
	})

	if target.IsClosing {
		a.DB.Create(&models.OccurrenceEvent{
			OrganizationID: orgID,
			OccurrenceID:   occ.ID,
			Type:           models.OccurrenceEventClosed,
			CreatedByID:    &userID,
		})
	}

	occ.Stage = target
	return r.SendEnvelope(occurrenceToResponse(*occ))
}

// After:
	if err := a.DB.Model(occ).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to change occurrence stage", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to change stage", nil, "")
	}
	occ.Stage = target

	resp := occurrenceToResponse(*occ)
	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type:    websocket.TypeOccurrenceChanged,
			Payload: resp,
		})
	}

	stageChangeEvent := models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           models.OccurrenceEventStageChange,
		Content:        from.Name + " → " + target.Name,
		Metadata:       models.JSONB{"from_stage_id": from.ID.String(), "to_stage_id": target.ID.String()},
		CreatedByID:    &userID,
	}
	a.DB.Create(&stageChangeEvent)
	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type: websocket.TypeOccurrenceEventCreated,
			Payload: OccurrenceEventResponse{
				ID:           stageChangeEvent.ID,
				OccurrenceID: stageChangeEvent.OccurrenceID,
				Type:         string(stageChangeEvent.Type),
				Content:      stageChangeEvent.Content,
				Metadata:     stageChangeEvent.Metadata,
				CreatedByID:  stageChangeEvent.CreatedByID,
				CreatedAt:    stageChangeEvent.CreatedAt,
			},
		})
	}

	if target.IsClosing {
		// Sem broadcast aqui: a spec fixa "exatamente um
		// occurrence_event_created" por mudança de etapa (§4, revisado duas
		// vezes). O evento automático "closed" continua sendo gravado — só
		// não tem eco em tempo real; a tela de detalhe volta a mostrá-lo no
		// próximo fetchEvents.
		a.DB.Create(&models.OccurrenceEvent{
			OrganizationID: orgID,
			OccurrenceID:   occ.ID,
			Type:           models.OccurrenceEventClosed,
			CreatedByID:    &userID,
		})
	}

	return r.SendEnvelope(resp)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/handlers/... -run TestOccurrences_ChangeStage -v`
Expected: PASS, including the pre-existing `TestOccurrences_CloseAndReopen` and `TestOccurrences_SameStageIsNoOp`.

- [ ] **Step 6: Run the full occurrences suite**

Run: `go test ./internal/handlers/... -run TestOccurrences -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/handlers/occurrences.go internal/handlers/occurrences_broadcast_test.go
git commit -m "feat(crm): broadcast occurrence_changed e occurrence_event_created na mudanca de etapa"
```

---

## Task 5: Backend — `occurrence_event_created` on `CreateOccurrenceEvent` (manual note)

**Files:**
- Modify: `internal/handlers/occurrences.go` (end of `CreateOccurrenceEvent`)
- Modify: `internal/handlers/occurrences_broadcast_test.go` (append tests)

**Interfaces:**
- Consumes: `OccurrenceEventResponse.OccurrenceID` from Task 4; same test harness as Task 3.
- Produces: nothing new for later tasks.

- [ ] **Step 1: Write the failing tests**

Append to `internal/handlers/occurrences_broadcast_test.go`:

```go
func TestOccurrences_CreateEventBroadcastsOccurrenceEventCreated(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Nota manual",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "POST", occ.ID, map[string]any{"content": "Cliente ligou de volta"})
	require.NoError(t, app.CreateOccurrenceEvent(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	msg := readBroadcast(t, client, websocket.TypeOccurrenceEventCreated)
	payload, ok := msg.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, occ.ID.String(), payload["occurrence_id"])
	assert.Equal(t, string(models.OccurrenceEventNote), payload["type"])
	assert.Equal(t, "Cliente ligou de volta", payload["content"])
}

// content vazio barra a criação antes de qualquer escrita.
func TestOccurrences_CreateEventFailureBroadcastsNothing(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Conteúdo vazio",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	hub, client := newTestHubWithClient(t, org.ID)
	app.WSHub = hub

	req := authedJSON(t, app, org.ID, user.ID, "POST", occ.ID, map[string]any{"content": ""})
	require.NoError(t, app.CreateOccurrenceEvent(req))
	require.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))

	assertNoBroadcast(t, client)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/... -run TestOccurrences_CreateEvent.*Broadcast -v`
Expected: `TestOccurrences_CreateEventBroadcastsOccurrenceEventCreated` FAILs on timeout.

- [ ] **Step 3: Wire the broadcast into `CreateOccurrenceEvent`**

```go
// Before:
	event := models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           models.OccurrenceEventNote,
		Content:        req.Content,
		CreatedByID:    &userID,
	}
	if err := a.DB.Create(&event).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create event", nil, "")
	}

	return r.SendEnvelope(OccurrenceEventResponse{
		ID:          event.ID,
		Type:        string(event.Type),
		Content:     event.Content,
		CreatedByID: event.CreatedByID,
		CreatedAt:   event.CreatedAt,
	})
}

// After:
	event := models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           models.OccurrenceEventNote,
		Content:        req.Content,
		CreatedByID:    &userID,
	}
	if err := a.DB.Create(&event).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create event", nil, "")
	}

	resp := OccurrenceEventResponse{
		ID:           event.ID,
		OccurrenceID: event.OccurrenceID,
		Type:         string(event.Type),
		Content:      event.Content,
		CreatedByID:  event.CreatedByID,
		CreatedAt:    event.CreatedAt,
	}

	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type:    websocket.TypeOccurrenceEventCreated,
			Payload: resp,
		})
	}

	return r.SendEnvelope(resp)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handlers/... -run TestOccurrences_CreateEvent -v`
Expected: PASS

- [ ] **Step 5: Run the entire handlers package test suite**

Run: `go test ./internal/handlers/... -v`
Expected: PASS. This is the last backend task — this run is the full regression check for Tasks 1-5 together.

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/occurrences.go internal/handlers/occurrences_broadcast_test.go
git commit -m "feat(crm): broadcast occurrence_event_created na nota manual"
```

---

## Task 6: Frontend — `websocket.ts` subscribe API for occurrence events

**Files:**
- Modify: `frontend/src/services/websocket.ts` (new message types, callback arrays, dispatch, public subscribe methods)
- Modify: `frontend/src/services/api.ts` (`OccurrenceEvent` interface gains `occurrence_id`)

**Interfaces:**
- Consumes: `Occurrence` and `OccurrenceEvent` types already exported from `frontend/src/services/api.ts`.
- Produces: `wsService.onOccurrenceChanged(callback: (payload: Occurrence) => void): () => void` and `wsService.onOccurrenceEventCreated(callback: (payload: OccurrenceEvent) => void): () => void` — both used by Tasks 7 and 8. Mirrors the existing `wsService.onCampaignStatsUpdate` (see `frontend/src/views/settings/CampaignsView.vue` for the consumer-side pattern this plan follows).

There is no unit-test runner configured in this frontend (no Vitest, no `*.spec.ts`/`*.test.ts` outside `frontend/e2e/`) — verification for this task is a build/typecheck pass, plus the end-to-end proof in Task 9.

- [ ] **Step 1: Add `occurrence_id` to the `OccurrenceEvent` TypeScript interface**

In `frontend/src/services/api.ts`:

```ts
// Before:
export interface OccurrenceEvent {
  id: string
  type: 'opened' | 'note' | 'stage_change' | 'assignment' | 'protocol_sent' | 'closed'
  content: string
  metadata: Record<string, unknown> | null
  created_by_id?: string
  created_by_name?: string
  created_at: string
}

// After:
export interface OccurrenceEvent {
  id: string
  occurrence_id: string
  type: 'opened' | 'note' | 'stage_change' | 'assignment' | 'protocol_sent' | 'closed'
  content: string
  metadata: Record<string, unknown> | null
  created_by_id?: string
  created_by_name?: string
  created_at: string
}
```

- [ ] **Step 2: Add the message type constants**

In `frontend/src/services/websocket.ts`, after the existing conversation-note constants:

```ts
// Conversation note types
const WS_TYPE_CONVERSATION_NOTE_CREATED = 'conversation_note_created'
const WS_TYPE_CONVERSATION_NOTE_UPDATED = 'conversation_note_updated'
const WS_TYPE_CONVERSATION_NOTE_DELETED = 'conversation_note_deleted'

// Occurrence types
const WS_TYPE_OCCURRENCE_CHANGED = 'occurrence_changed'
const WS_TYPE_OCCURRENCE_EVENT_CREATED = 'occurrence_event_created'
```

- [ ] **Step 3: Add the callback arrays**

Next to the existing `campaignStatsCallbacks` field declaration:

```ts
// Before:
  private campaignStatsCallbacks: ((payload: any) => void)[] = []

// After:
  private campaignStatsCallbacks: ((payload: any) => void)[] = []
  private occurrenceChangedCallbacks: ((payload: any) => void)[] = []
  private occurrenceEventCreatedCallbacks: ((payload: any) => void)[] = []
```

- [ ] **Step 4: Dispatch the two new message types**

In `handleMessage`'s `switch`, right after the conversation-note cases:

```ts
// Before:
        case WS_TYPE_CONVERSATION_NOTE_DELETED:
          useNotesStore().onNoteDeleted(message.payload.id)
          break
        default:

// After:
        case WS_TYPE_CONVERSATION_NOTE_DELETED:
          useNotesStore().onNoteDeleted(message.payload.id)
          break
        case WS_TYPE_OCCURRENCE_CHANGED:
          this.handleOccurrenceChanged(message.payload)
          break
        case WS_TYPE_OCCURRENCE_EVENT_CREATED:
          this.handleOccurrenceEventCreated(message.payload)
          break
        default:
```

- [ ] **Step 5: Add the private handlers and public subscribe methods**

Right after `handleCampaignStatsUpdate`:

```ts
// Before:
  private handleCampaignStatsUpdate(payload: any) {
    // Notify all registered callbacks
    this.campaignStatsCallbacks.forEach(callback => callback(payload))
  }

// After:
  private handleCampaignStatsUpdate(payload: any) {
    // Notify all registered callbacks
    this.campaignStatsCallbacks.forEach(callback => callback(payload))
  }

  private handleOccurrenceChanged(payload: any) {
    this.occurrenceChangedCallbacks.forEach(callback => callback(payload))
  }

  private handleOccurrenceEventCreated(payload: any) {
    this.occurrenceEventCreatedCallbacks.forEach(callback => callback(payload))
  }
```

And right after `onCampaignStatsUpdate`:

```ts
// Before:
  onCampaignStatsUpdate(callback: (payload: any) => void) {
    this.campaignStatsCallbacks.push(callback)
    // Return unsubscribe function
    return () => {
      const index = this.campaignStatsCallbacks.indexOf(callback)
      if (index > -1) {
        this.campaignStatsCallbacks.splice(index, 1)
      }
    }
  }

// After:
  onCampaignStatsUpdate(callback: (payload: any) => void) {
    this.campaignStatsCallbacks.push(callback)
    // Return unsubscribe function
    return () => {
      const index = this.campaignStatsCallbacks.indexOf(callback)
      if (index > -1) {
        this.campaignStatsCallbacks.splice(index, 1)
      }
    }
  }

  onOccurrenceChanged(callback: (payload: any) => void) {
    this.occurrenceChangedCallbacks.push(callback)
    return () => {
      const index = this.occurrenceChangedCallbacks.indexOf(callback)
      if (index > -1) {
        this.occurrenceChangedCallbacks.splice(index, 1)
      }
    }
  }

  onOccurrenceEventCreated(callback: (payload: any) => void) {
    this.occurrenceEventCreatedCallbacks.push(callback)
    return () => {
      const index = this.occurrenceEventCreatedCallbacks.indexOf(callback)
      if (index > -1) {
        this.occurrenceEventCreatedCallbacks.splice(index, 1)
      }
    }
  }
```

- [ ] **Step 6: Typecheck**

Run (from `frontend/`): `npm run typecheck`
Expected: no new errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/services/websocket.ts frontend/src/services/api.ts
git commit -m "feat(crm): assinatura de eventos de ocorrencia no wsService"
```

---

## Task 7: Frontend — `OccurrenceBoard.vue` live updates

**Files:**
- Modify: `frontend/src/components/crm/OccurrenceBoard.vue`

**Interfaces:**
- Consumes: `wsService.onOccurrenceChanged` / return value (unsubscribe fn) from Task 6; `ColumnState`, `sortByOpenedAtDesc`, `columns` already defined in this file.
- Produces: nothing consumed elsewhere.

There is no unit test for this component (no Vitest configured) — verification is manual, via the dev server, in this task's last step, plus the automated proof in Task 9's E2E tests.

- [ ] **Step 1: Import `onUnmounted` and `wsService`**

```ts
// Before:
import { ref, onMounted } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { useRouter } from 'vue-router'
import draggable from 'vuedraggable'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { useOccurrencesStore } from '@/stores/occurrences'
import type { Occurrence, OccurrenceStage } from '@/services/api'
import OccurrenceCard from './OccurrenceCard.vue'

// After:
import { ref, onMounted, onUnmounted } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { useRouter } from 'vue-router'
import draggable from 'vuedraggable'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { useOccurrencesStore } from '@/stores/occurrences'
import { wsService } from '@/services/websocket'
import type { Occurrence, OccurrenceStage } from '@/services/api'
import OccurrenceCard from './OccurrenceCard.vue'
```

- [ ] **Step 2: Add the live-update handler**

Right after `hasMore` (before `onDragStart`):

```ts
function hasMore(col: ColumnState): boolean {
  return col.items.length < col.total
}

/**
 * Aplica um occurrence_changed vindo de qualquer origem (criação, edição ou
 * mudança de etapa, por este cliente ou por outro) contra as colunas já
 * carregadas.
 *
 * Uma ocorrência já conhecida é atualizada no lugar, ou movida de coluna se a
 * etapa mudou — mantendo a ordenação por opened_at, o mesmo invariante que o
 * arrastar-e-soltar garante depois de um movimento bem-sucedido. Uma
 * ocorrência desconhecida só soma no total do cabeçalho, sem inserir cartão
 * fora da ordem de paginação (spec §2, regra da criação).
 *
 * ponytail: o payload não diz se a origem foi CreateOccurrence ou
 * UpdateOccurrence/ChangeOccurrenceStage — as três emitem o mesmo tipo de
 * mensagem. Uma ocorrência desconhecida por estar apenas fora da página
 * carregada (não por ser nova) também cai no ramo "soma o total", o que pode
 * super-contar por um até o próximo "Load More" ou recarregamento da coluna,
 * que sempre resincroniza `total` pela resposta do servidor. Sem esse sinal
 * extra no payload não há como distinguir os dois casos no cliente.
 */
function handleOccurrenceChanged(payload: Occurrence) {
  const targetCol = columns.value.find(c => c.stage.id === payload.stage_id)
  if (!targetCol) return // etapa sem coluna carregada ainda

  let sourceCol: ColumnState | null = null
  let sourceIdx = -1
  for (const col of columns.value) {
    const idx = col.items.findIndex(i => i.id === payload.id)
    if (idx !== -1) {
      sourceCol = col
      sourceIdx = idx
      break
    }
  }

  if (!sourceCol) {
    targetCol.total += 1
    return
  }

  if (sourceCol.stage.id === targetCol.stage.id) {
    sourceCol.items.splice(sourceIdx, 1, payload)
    sortByOpenedAtDesc(sourceCol.items)
    return
  }

  sourceCol.items.splice(sourceIdx, 1)
  sourceCol.total = Math.max(0, sourceCol.total - 1)
  targetCol.items.push(payload)
  targetCol.total += 1
  sortByOpenedAtDesc(targetCol.items)
}

let unsubscribeOccurrenceChanged: (() => void) | null = null
```

- [ ] **Step 3: Subscribe on mount, unsubscribe on unmount**

```ts
// Before:
onMounted(async () => {
  if (store.stages.length === 0) await store.fetchStages()
  await loadAll()
})

// After:
onMounted(async () => {
  if (store.stages.length === 0) await store.fetchStages()
  await loadAll()
  unsubscribeOccurrenceChanged = wsService.onOccurrenceChanged(handleOccurrenceChanged)
})

onUnmounted(() => {
  unsubscribeOccurrenceChanged?.()
})
```

- [ ] **Step 4: Manual verification with the dev server**

Start the frontend (`npm run dev` from `frontend/`, backend already running per this repo's usual dev setup) and open the board in two browser windows logged in as two different agents:
1. In window A, drag a card to another column. Confirm it moves in window B without a reload.
2. In window B, create a new occurrence (via the chat contact panel) whose stage lands in a column window A already has open. Confirm window A's column header count goes up by one and no new card appears until "Load More" or a reload.

Expected: both behaviors match. Fix any discrepancy before moving on — this is the primary risk point in this task, since it can't be typechecked into correctness.

- [ ] **Step 5: Typecheck**

Run (from `frontend/`): `npm run typecheck`
Expected: no new errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/crm/OccurrenceBoard.vue
git commit -m "feat(crm): quadro atualiza em tempo real via WebSocket"
```

---

## Task 8: Frontend — `OccurrenceDetailView.vue` live updates

**Files:**
- Modify: `frontend/src/views/crm/OccurrenceDetailView.vue`

**Interfaces:**
- Consumes: `wsService.onOccurrenceChanged` / `wsService.onOccurrenceEventCreated` from Task 6; `occurrence` ref, `store.events` (from `useOccurrencesStore`), `occurrenceId` computed — all already defined in this file.
- Produces: nothing consumed elsewhere.

- [ ] **Step 1: Import `onUnmounted` and `wsService`**

```ts
// Before:
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import { IconButton } from '@/components/shared'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { occurrencesService, type Occurrence } from '@/services/api'
import { useOccurrencesStore } from '@/stores/occurrences'
import { useUsersStore } from '@/stores/users'

// After:
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import { IconButton } from '@/components/shared'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { occurrencesService, type Occurrence, type OccurrenceEvent } from '@/services/api'
import { useOccurrencesStore } from '@/stores/occurrences'
import { useUsersStore } from '@/stores/users'
import { wsService } from '@/services/websocket'
```

- [ ] **Step 2: Add the live-update handlers**

Right after `loadOccurrence` (before `handleStageChange`):

```ts
async function loadOccurrence() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const res = await occurrencesService.get(occurrenceId.value)
    occurrence.value = res.data.data
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

/** Só reage a eventos da própria ocorrência aberta — o resto é ignorado. */
function handleOccurrenceChanged(payload: Occurrence) {
  if (occurrence.value && payload.id === occurrence.value.id) {
    occurrence.value = payload
  }
}

function handleOccurrenceEventCreated(payload: OccurrenceEvent) {
  if (!occurrence.value || payload.occurrence_id !== occurrence.value.id) return
  // Evita um flash de item duplicado quando esta própria aba é a origem: o
  // submit de nota já recarrega store.events do REST logo em seguida.
  if (store.events.some(e => e.id === payload.id)) return
  store.events.push(payload)
}

let unsubscribeOccurrenceChanged: (() => void) | null = null
let unsubscribeOccurrenceEventCreated: (() => void) | null = null
```

- [ ] **Step 3: Subscribe on mount, unsubscribe on unmount**

```ts
// Before:
onMounted(async () => {
  await Promise.all([
    store.fetchStages(),
    loadOccurrence(),
    store.fetchEvents(occurrenceId.value),
    usersStore.users.length === 0 ? usersStore.fetchUsers().catch(() => {}) : Promise.resolve(),
  ])
})

// After:
onMounted(async () => {
  await Promise.all([
    store.fetchStages(),
    loadOccurrence(),
    store.fetchEvents(occurrenceId.value),
    usersStore.users.length === 0 ? usersStore.fetchUsers().catch(() => {}) : Promise.resolve(),
  ])

  unsubscribeOccurrenceChanged = wsService.onOccurrenceChanged(handleOccurrenceChanged)
  unsubscribeOccurrenceEventCreated = wsService.onOccurrenceEventCreated(handleOccurrenceEventCreated)
})

onUnmounted(() => {
  unsubscribeOccurrenceChanged?.()
  unsubscribeOccurrenceEventCreated?.()
})
```

- [ ] **Step 4: Manual verification with the dev server**

With two browser windows logged in as two different agents, both on the same occurrence's detail page (`/crm/occurrences/:id`):
1. In window A, change the title (via the API or another surface — the detail view itself doesn't expose a title editor, so use `curl`/Postman/`PUT /api/occurrences/:id`, or edit assignee/stage from the UI, which does exist here). Confirm window B updates without a reload.
2. In window A, add a note. Confirm it appears in window B's timeline without a reload.
3. Open a second, different occurrence's detail page in window B instead, add a note to the first occurrence from window A. Confirm window B's timeline does not change.

Expected: all three behaviors match.

- [ ] **Step 5: Typecheck**

Run (from `frontend/`): `npm run typecheck`
Expected: no new errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/crm/OccurrenceDetailView.vue
git commit -m "feat(crm): detalhe da ocorrencia atualiza em tempo real via WebSocket"
```

---

## Task 9: E2E — Playwright coverage for realtime CRM updates

**Files:**
- Create: `frontend/e2e/tests/crm/occurrence-realtime.spec.ts`

**Interfaces:**
- Consumes: `OccurrencesPage` (`frontend/e2e/pages/OccurrencesPage.ts`, already has `gotoList`, `switchToBoard`, `boardColumn`, `boardColumnCount`, `boardCard`, `cardInColumn`, `listView`), `ApiHelper` (`frontend/e2e/helpers/api.ts`, already has `loginAsAdmin`, `createContact`, generic `get`/`post`/`put`), `loginAsAdmin` and `createTestScope` (`frontend/e2e/helpers.ts` / `frontend/e2e/framework`).
- Produces: nothing consumed elsewhere — this is the last task.

The "other agent" side of every scenario below is a plain REST call through `ApiHelper`, not a second browser context. `BroadcastToOrg` does not distinguish the origin of a write, so this proves exactly the same delivery path with far less flakiness than replaying a real drag-and-drop gesture in a second window — and drag-and-drop itself already has full coverage in `occurrence-board.spec.ts`. This mirrors how that same suite already treats bulk-created occurrences (`api.post('/api/occurrences', ...)` in a loop) as equivalent to "another agent."

- [ ] **Step 1: Write the test file**

Create `frontend/e2e/tests/crm/occurrence-realtime.spec.ts`:

```ts
import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { createTestScope } from '../../framework'
import { OccurrencesPage } from '../../pages'

const scope = createTestScope('crm-realtime')

async function createContact(api: ApiHelper): Promise<string> {
  await api.loginAsAdmin()
  const contact = await api.createContact(scope.phone(), scope.name('contact'))
  return contact.id
}

async function createOccurrenceViaApi(api: ApiHelper, contactId: string, title: string) {
  const res = await api.post('/api/occurrences', { contact_id: contactId, title })
  if (!res.ok()) throw new Error(`Failed to create occurrence: ${await res.text()}`)
  return (await res.json()).data as { id: string; protocol_number: string; stage_id: string }
}

/**
 * Tempo real no CRM (spec 2026-09-03-crm-put-e-tempo-real-design.md, §2 e
 * §4). A ação "de outro agente" é sempre uma chamada de API direta — ver a
 * nota no topo deste arquivo do plano de implementação para o porquê.
 */
test.describe('CRM realtime', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('a stage change from another agent moves the card on an open board without a reload', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    const occ = await createOccurrenceViaApi(api, contactId, scope.name('live-move'))

    const occurrencesPage = new OccurrencesPage(page)
    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()
    await expect(occurrencesPage.boardCard(occ.protocol_number)).toBeVisible()
    // Dá tempo do WebSocket desta página terminar de conectar antes da
    // mudança de etapa sair pela API.
    await page.waitForTimeout(1000)

    const stageRes = await api.get('/api/occurrence-stages')
    if (!stageRes.ok()) throw new Error(`Failed to fetch stages: ${await stageRes.text()}`)
    const { stages } = (await stageRes.json()).data as { stages: Array<{ id: string; name: string }> }
    const target = stages.find(s => s.name === 'Em análise')
    if (!target) throw new Error('etapa "Em análise" não encontrada no pipeline padrão')

    const moveRes = await api.put(`/api/occurrences/${occ.id}/stage`, { stage_id: target.id })
    if (!moveRes.ok()) throw new Error(`Failed to change stage: ${await moveRes.text()}`)

    await expect(occurrencesPage.cardInColumn('Em análise', occ.protocol_number)).toBeVisible({ timeout: 10000 })
    await expect(occurrencesPage.cardInColumn('Aberto', occ.protocol_number)).toBeHidden()
  })

  test('an occurrence created by another agent only bumps the column total, not the card list', async ({ page, request }) => {
    const api = new ApiHelper(request)

    const occurrencesPage = new OccurrencesPage(page)
    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()
    await expect(occurrencesPage.boardColumn('Aberto')).toBeVisible()
    const before = Number(await occurrencesPage.boardColumnCount('Aberto').innerText())
    await page.waitForTimeout(1000)

    const contactId = await createContact(api)
    const occ = await createOccurrenceViaApi(api, contactId, scope.name('counter-only'))

    await expect(occurrencesPage.boardColumnCount('Aberto')).toHaveText(String(before + 1), { timeout: 10000 })
    await expect(occurrencesPage.boardCard(occ.protocol_number)).toBeHidden()
  })

  test('a title change from another agent updates an open detail page without a reload', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    const occ = await createOccurrenceViaApi(api, contactId, scope.name('before-title'))

    await page.goto(`/crm/occurrences/${occ.id}`)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    const updateRes = await api.put(`/api/occurrences/${occ.id}`, {
      title: 'Titulo mudou ao vivo', priority: 'normal',
    })
    if (!updateRes.ok()) throw new Error(`Failed to update occurrence: ${await updateRes.text()}`)

    await expect(page.getByRole('heading', { name: 'Titulo mudou ao vivo' })).toBeVisible({ timeout: 10000 })
  })

  test('an event on a different occurrence does not touch the open detail page', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    const watchedTitle = scope.name('watched')
    const watched = await createOccurrenceViaApi(api, contactId, watchedTitle)
    const other = await createOccurrenceViaApi(api, contactId, scope.name('other'))

    await page.goto(`/crm/occurrences/${watched.id}`)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    const noteRes = await api.post(`/api/occurrences/${other.id}/events`, { content: 'Nota na ocorrência errada' })
    if (!noteRes.ok()) throw new Error(`Failed to add note: ${await noteRes.text()}`)

    // Dá tempo do broadcast chegar (ou não) antes de verificar que nada mudou.
    await page.waitForTimeout(1500)
    await expect(page.getByText('Nota na ocorrência errada')).toHaveCount(0)
    await expect(page.getByRole('heading', { name: watchedTitle })).toBeVisible()
  })

  test('the list view ignores realtime events entirely', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)

    const occurrencesPage = new OccurrencesPage(page)
    await occurrencesPage.gotoList()
    await expect(occurrencesPage.listView).toBeVisible()
    await page.waitForTimeout(1000)

    const occ = await createOccurrenceViaApi(api, contactId, scope.name('list-no-realtime'))

    // A lista não processa nenhum evento de WebSocket (spec §2, "Na lista:
    // nada") — só aparece depois de uma navegação real, que recarrega do REST.
    await page.waitForTimeout(1500)
    await expect(page.getByText(occ.protocol_number)).toHaveCount(0)

    await occurrencesPage.gotoList()
    await expect(page.getByText(occ.protocol_number)).toBeVisible()
  })
})
```

- [ ] **Step 2: Run the suite**

Run (from `frontend/`): `npx playwright test crm/occurrence-realtime.spec.ts`
Expected: all 5 tests PASS. If the backend dev server was started before Tasks 1-5 landed, restart it first.

- [ ] **Step 3: Run the full CRM E2E suite to check for regressions**

Run (from `frontend/`): `npx playwright test crm/`
Expected: PASS — `occurrence-board.spec.ts`, `occurrence-permissions.spec.ts`, `occurrences.spec.ts` and the new `occurrence-realtime.spec.ts` all green.

- [ ] **Step 4: Commit**

```bash
git add frontend/e2e/tests/crm/occurrence-realtime.spec.ts
git commit -m "test(crm): cobertura E2E para atualizacoes em tempo real"
```

---

## After all tasks land

Merge `feature/crm-put-tempo-real` into `development`, then open the PR the user asked for ("faça o PR") — target `development` → `main` in `ivankoelho/whatc`. That PR will bundle everything on `development` since it last synced with `origin/development`, not just this plan's changes, so review the accumulated diff before drafting the PR body.
