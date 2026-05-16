package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGraphTestFixtures sets up the common state used across graph-runner tests:
// app, org, account, contact, and an active session. Caller adds the flow.
func newGraphTestFixtures(t *testing.T) (
	app *App,
	org *models.Organization,
	account *models.WhatsAppAccount,
	contact *models.Contact,
	session *models.ChatbotSession,
) {
	t.Helper()
	app = newProcessorTestApp(t)
	org, account = createProcessorTestOrg(t, app)
	contact = testutil.CreateTestContact(t, app.DB, org.ID)

	session = &models.ChatbotSession{
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
	return app, org, account, contact, session
}

// chatGraphPath extracts the recorded __path__ entries from session data
// as []map[string]any for assertion.
func chatGraphPath(t *testing.T, s *models.ChatbotSession) []map[string]any {
	t.Helper()
	raw, ok := s.SessionData["__path__"].([]any)
	require.True(t, ok, "session.SessionData[__path__] not set or wrong type")
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		require.True(t, ok, "path entry not a map")
		out = append(out, entry)
	}
	return out
}

// TestRunChatGraph_GoldenPath walks a three-node flow end-to-end:
// message → buttons → end, with the user clicking a button.
func TestRunChatGraph_GoldenPath(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)

	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "golden",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "m1",
			"nodes": []any{
				map[string]any{
					"id": "m1", "type": "message", "label": "greet",
					"config": map[string]any{"message": "Hello!"},
				},
				map[string]any{
					"id": "b1", "type": "buttons", "label": "choose",
					"config": map[string]any{
						"body": "Pick one",
						"buttons": []any{
							map[string]any{"id": "opt_a", "title": "A"},
							map[string]any{"id": "opt_b", "title": "B"},
						},
					},
				},
				map[string]any{
					"id": "e1", "type": "end", "label": "done",
					"config": map[string]any{"message": "Thanks!"},
				},
			},
			"edges": []any{
				map[string]any{"from": "m1", "to": "b1", "condition": "default"},
				map[string]any{"from": "b1", "to": "e1", "condition": "button:opt_a"},
				map[string]any{"from": "b1", "to": "e1", "condition": "button:opt_b"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)

	// Run 1: trigger arrives. Entry m1 (non-blocking) → b1 (blocking, yields).
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))

	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "b1", session.CurrentStep, "should be parked at buttons node")
	assert.Equal(t, models.SessionStatusActive, session.Status)

	p1 := chatGraphPath(t, session)
	require.Len(t, p1, 2)
	assert.Equal(t, "m1", p1[0]["node"])
	assert.Equal(t, "default", p1[0]["outcome"])
	assert.Equal(t, "b1", p1[1]["node"])
	assert.Equal(t, "", p1[1]["outcome"])

	// Run 2: user clicks button opt_a. b1 consumes → e1 → terminal.
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "", "opt_a"))

	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	require.NotNil(t, session.CompletedAt)

	p2 := chatGraphPath(t, session)
	require.Len(t, p2, 4)
	assert.Equal(t, "b1", p2[2]["node"])
	assert.Equal(t, "button:opt_a", p2[2]["outcome"])
	assert.Equal(t, "e1", p2[3]["node"])
}

// TestRunChatGraph_ButtonsRePromptOnText verifies that a text reply (no
// buttonID) at a buttons node re-sends the prompt instead of advancing.
func TestRunChatGraph_ButtonsRePromptOnText(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)

	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "buttons-only",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "b1",
			"nodes": []any{
				map[string]any{
					"id": "b1", "type": "buttons", "label": "choose",
					"config": map[string]any{
						"body":    "Pick one",
						"buttons": []any{map[string]any{"id": "opt_a", "title": "A"}},
					},
				},
				map[string]any{"id": "e1", "type": "end", "label": "done"},
			},
			"edges": []any{
				map[string]any{"from": "b1", "to": "e1", "condition": "button:opt_a"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)

	// First inbound: trigger. Lands on b1, sends buttons, yields.
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "b1", session.CurrentStep)
	assert.Equal(t, models.SessionStatusActive, session.Status)

	// Second inbound: text instead of button click. Should re-send (stays
	// at b1, status still active, path has another b1 entry).
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "huh?", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "b1", session.CurrentStep)
	assert.Equal(t, models.SessionStatusActive, session.Status)

	path := chatGraphPath(t, session)
	require.GreaterOrEqual(t, len(path), 2, "should have at least two b1 visits")
	assert.Equal(t, "b1", path[len(path)-1]["node"])
}

// TestRunChatGraph_UnknownButtonEndsFlow verifies that a click on a button
// with no matching edge (and no "default" edge) terminates the flow rather
// than tight-looping.
func TestRunChatGraph_UnknownButtonEndsFlow(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)

	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "unknown-button",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "b1",
			"nodes": []any{
				map[string]any{
					"id": "b1", "type": "buttons", "label": "choose",
					"config": map[string]any{
						"body":    "Pick one",
						"buttons": []any{map[string]any{"id": "opt_a", "title": "A"}},
					},
				},
				map[string]any{"id": "e1", "type": "end", "label": "done"},
			},
			"edges": []any{
				map[string]any{"from": "b1", "to": "e1", "condition": "button:opt_a"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)

	// Get to b1 and yield.
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	require.Equal(t, "b1", session.CurrentStep)

	// Click an unknown button. No edge matches, no default → session completes.
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "", "opt_z"))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
}

// TestParseChatGraph_InvalidGraph rejects malformed graphs at parse time
// so the runner never has to defend against them.
func TestParseChatGraph_InvalidGraph(t *testing.T) {
	cases := []struct {
		name string
		raw  models.JSONB
	}{
		{"nil-treated-as-no-graph", nil}, // returns (nil, nil)
		{"wrong-version", models.JSONB{"version": 1, "entry_node": "x"}},
		{"missing-entry", models.JSONB{"version": 2, "entry_node": ""}},
		{"entry-not-in-nodes", models.JSONB{
			"version":    2,
			"entry_node": "missing",
			"nodes":      []any{map[string]any{"id": "a", "type": "end"}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := parseChatGraph(tc.raw)
			if tc.name == "nil-treated-as-no-graph" {
				assert.NoError(t, err)
				assert.Nil(t, g)
				return
			}
			assert.Error(t, err)
			assert.Nil(t, g)
		})
	}
}

// TestRunChatGraph_RunawayCycle ensures a non-blocking cycle is bounded
// by the iteration guard rather than hanging the webhook goroutine.
func TestRunChatGraph_RunawayCycle(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)

	// Two message nodes that point at each other → infinite chain.
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "cycle",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "a",
			"nodes": []any{
				map[string]any{"id": "a", "type": "message", "config": map[string]any{"message": "A"}},
				map[string]any{"id": "b", "type": "message", "config": map[string]any{"message": "B"}},
			},
			"edges": []any{
				map[string]any{"from": "a", "to": "b", "condition": "default"},
				map[string]any{"from": "b", "to": "a", "condition": "default"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)

	err := app.runChatGraph(account, contact, session, flow, "start", "")
	require.ErrorIs(t, err, errChatGraphRunaway)
}

// newPromptFlow builds a two-node graph (prompt → end) with an optional
// regex + max_retries on the prompt. Used by the prompt-node test suite.
func newPromptFlow(t *testing.T, app *App, org *models.Organization, account *models.WhatsAppAccount, regex string, maxRetries int) *models.ChatbotFlow {
	t.Helper()
	cfg := map[string]any{
		"body":     "What's your email?",
		"store_as": "email",
	}
	if regex != "" {
		cfg["validation_regex"] = regex
		cfg["validation_error"] = "Not a valid email, try again."
	}
	if maxRetries > 0 {
		cfg["max_retries"] = maxRetries
	}
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "prompt-flow",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "p1",
			"nodes": []any{
				map[string]any{"id": "p1", "type": "prompt", "label": "ask", "config": cfg},
				map[string]any{"id": "e1", "type": "end", "label": "done"},
			},
			"edges": []any{
				map[string]any{"from": "p1", "to": "e1", "condition": "default"},
				map[string]any{"from": "p1", "to": "e1", "condition": "max_retries"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

// TestRunChatGraph_Prompt_HappyPath: first inbound sends prompt + yields;
// second inbound validates, stores into SessionData, advances to terminal.
func TestRunChatGraph_Prompt_HappyPath(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newPromptFlow(t, app, org, account, `^[^@]+@[^@]+$`, 3)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "p1", session.CurrentStep, "should park at prompt on first inbound")

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "shri@example.com", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	assert.Equal(t, "shri@example.com", session.SessionData["email"], "input should be stored under store_as")
	assert.Equal(t, 0, session.StepRetries, "retries should reset on valid input")
}

// TestRunChatGraph_Prompt_RetryOnInvalid: invalid input re-sends the error
// and stays at the prompt node.
func TestRunChatGraph_Prompt_RetryOnInvalid(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newPromptFlow(t, app, org, account, `^[^@]+@[^@]+$`, 3)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))

	// First invalid attempt
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "not-an-email", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "p1", session.CurrentStep, "should stay at prompt on invalid")
	assert.Equal(t, 1, session.StepRetries)
	assert.Equal(t, models.SessionStatusActive, session.Status)
	_, stored := session.SessionData["email"]
	assert.False(t, stored, "invalid input must not be stored")
}

// TestRunChatGraph_Prompt_MaxRetriesRoutesToEdge: once retries reach max,
// the runner advances via the max_retries edge instead of looping.
func TestRunChatGraph_Prompt_MaxRetriesRoutesToEdge(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newPromptFlow(t, app, org, account, `^[^@]+@[^@]+$`, 2) // 2 strikes

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))

	// First invalid → retry
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "x", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	require.Equal(t, "p1", session.CurrentStep)
	require.Equal(t, 1, session.StepRetries)

	// Second invalid → max_retries → end
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "y", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
}

// newAPICallFlow builds a three-node graph (api_call → message → end)
// where the api_call's outgoing edges route to differently-labelled
// message nodes for 2xx vs non-2xx, making it easy to assert which
// branch ran.
func newAPICallFlow(t *testing.T, app *App, org *models.Organization, account *models.WhatsAppAccount, apiURL string, mapping map[string]any) *models.ChatbotFlow {
	t.Helper()
	cfg := map[string]any{
		"url":    apiURL,
		"method": "GET",
	}
	if mapping != nil {
		cfg["response_mapping"] = mapping
	}
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "api-call-flow",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "api",
			"nodes": []any{
				map[string]any{"id": "api", "type": "api_call", "label": "fetch", "config": cfg},
				map[string]any{"id": "ok", "type": "message", "label": "success", "config": map[string]any{"message": "ok"}},
				map[string]any{"id": "bad", "type": "message", "label": "error", "config": map[string]any{"message": "boom"}},
				map[string]any{"id": "end", "type": "end", "label": "done"},
			},
			"edges": []any{
				map[string]any{"from": "api", "to": "ok", "condition": "http:2xx"},
				map[string]any{"from": "api", "to": "bad", "condition": "http:non2xx"},
				map[string]any{"from": "ok", "to": "end", "condition": "default"},
				map[string]any{"from": "bad", "to": "end", "condition": "default"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

// TestRunChatGraph_APICall_2xxRoutesAndMapsResponse verifies the 2xx
// branch fires AND response_mapping pulls fields into SessionData.
func TestRunChatGraph_APICall_2xxRoutesAndMapsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"id": "cust-42", "status": "active"},
		})
	}))
	defer server.Close()

	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newAPICallFlow(t, app, org, account, server.URL, map[string]any{
		"customer_id": "data.id",
		"status":      "data.status",
	})

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	assert.Equal(t, "cust-42", session.SessionData["customer_id"])
	assert.Equal(t, "active", session.SessionData["status"])

	path := chatGraphPath(t, session)
	require.GreaterOrEqual(t, len(path), 2)
	assert.Equal(t, "api", path[0]["node"])
	assert.Equal(t, "http:2xx", path[0]["outcome"])
	assert.Equal(t, "ok", path[1]["node"], "should advance via http:2xx edge to success branch")
}

// TestRunChatGraph_APICall_Non2xxRoutesToErrorBranch verifies the
// http:non2xx outcome.
func TestRunChatGraph_APICall_Non2xxRoutesToErrorBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newAPICallFlow(t, app, org, account, server.URL, nil)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)

	path := chatGraphPath(t, session)
	require.GreaterOrEqual(t, len(path), 2)
	assert.Equal(t, "api", path[0]["node"])
	assert.Equal(t, "http:non2xx", path[0]["outcome"])
	assert.Equal(t, "bad", path[1]["node"], "should advance via http:non2xx edge to error branch")
}

// TestRunChatGraph_APICall_NetworkErrorRoutesNon2xx verifies that a
// connection failure (server closed) maps to http:non2xx rather than
// returning an error up to the dispatcher.
func TestRunChatGraph_APICall_NetworkErrorRoutesNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	url := server.URL
	server.Close() // shut it down so the request fails

	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newAPICallFlow(t, app, org, account, url, nil)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)

	path := chatGraphPath(t, session)
	require.GreaterOrEqual(t, len(path), 1)
	assert.Equal(t, "http:non2xx", path[0]["outcome"])
}

// TestRunChatGraph_Prompt_NoRegexAcceptsAnything verifies the executor
// treats a prompt with no validation_regex as accept-all.
func TestRunChatGraph_Prompt_NoRegexAcceptsAnything(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newPromptFlow(t, app, org, account, "", 3)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", ""))
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "literally anything", ""))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	assert.Equal(t, "literally anything", session.SessionData["email"])
}
