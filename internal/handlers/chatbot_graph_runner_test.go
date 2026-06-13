package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))

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
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "", "opt_a", nil))

	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	require.NotNil(t, session.CompletedAt)

	p2 := chatGraphPath(t, session)
	require.Len(t, p2, 4)
	assert.Equal(t, "b1", p2[2]["node"])
	assert.Equal(t, "button:opt_a", p2[2]["outcome"])
	assert.Equal(t, "e1", p2[3]["node"])
}

// TestRunChatGraph_ButtonsStoreAs verifies a buttons node with store_as
// configured persists the tapped button's title into SessionData, so later
// nodes can interpolate it via {{var}} just like a prompt node.
func TestRunChatGraph_ButtonsStoreAs(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)

	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "buttons-store-as",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "b1",
			"nodes": []any{
				map[string]any{
					"id": "b1", "type": "buttons", "label": "choose",
					"config": map[string]any{
						"body":     "Pick one",
						"store_as": "topic",
						"buttons":  []any{map[string]any{"id": "opt_a", "title": "PIS vs Non PIS Account"}},
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

	// Trigger → b1 sends buttons and yields.
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	require.Equal(t, "b1", session.CurrentStep)

	// Click: WhatsApp delivers the title as text and the id as buttonID.
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "PIS vs Non PIS Account", "opt_a", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)

	assert.Equal(t, "PIS vs Non PIS Account", session.SessionData["topic"],
		"buttons store_as should persist the tapped button's title")
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
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "b1", session.CurrentStep)
	assert.Equal(t, models.SessionStatusActive, session.Status)

	// Second inbound: text instead of button click. Should re-send (stays
	// at b1, status still active, path has another b1 entry).
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "huh?", "", nil))
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
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	require.Equal(t, "b1", session.CurrentStep)

	// Click an unknown button. No edge matches, no default → session completes.
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "", "opt_z", nil))
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

	err := app.runChatGraph(account, contact, session, flow, "start", "", nil)
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

// newWhatsAppFlowFlow builds a one-node WA Flow graph that advances to
// an end node once the user submits the flow form.
func newWhatsAppFlowFlow(t *testing.T, app *App, org *models.Organization, account *models.WhatsAppAccount) *models.ChatbotFlow {
	t.Helper()
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "waflow",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "wa",
			"nodes": []any{
				map[string]any{"id": "wa", "type": "whatsapp_flow", "label": "form", "config": map[string]any{
					"flow_id": "meta-flow-123",
					"header":  "Welcome",
					"body":    "Please fill the form",
					"cta":     "Open",
				}},
				map[string]any{"id": "end", "type": "end"},
			},
			"edges": []any{
				map[string]any{"from": "wa", "to": "end", "condition": "default"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

// TestRunChatGraph_WhatsAppFlow_SendsFormOnFirstEntry: no flow response
// yet → executor sends the form and yields. CurrentStep stays on the
// node so the next inbound resumes here.
func TestRunChatGraph_WhatsAppFlow_SendsFormOnFirstEntry(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newWhatsAppFlowFlow(t, app, org, account)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "wa", session.CurrentStep, "should park at WA flow node")
	assert.Equal(t, models.SessionStatusActive, session.Status)
}

// TestRunChatGraph_WhatsAppFlow_ConsumesResponseAndAdvances: when a
// later inbound carries flow_response_data, the fields are merged into
// SessionData and the run advances via the default edge.
func TestRunChatGraph_WhatsAppFlow_ConsumesResponseAndAdvances(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newWhatsAppFlowFlow(t, app, org, account)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	require.Equal(t, "wa", session.CurrentStep)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "", "", map[string]any{
		"full_name": "Shri",
		"email":     "shri@example.com",
	}))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	assert.Equal(t, "Shri", session.SessionData["full_name"])
	assert.Equal(t, "shri@example.com", session.SessionData["email"])
}

// TestRunChatGraph_WhatsAppFlow_MissingFlowIDAdvancesGracefully: a
// misconfigured node logs and advances via default instead of stalling.
func TestRunChatGraph_WhatsAppFlow_MissingFlowIDAdvancesGracefully(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "wa-broken",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "wa",
			"nodes": []any{
				map[string]any{"id": "wa", "type": "whatsapp_flow", "config": map[string]any{}},
				map[string]any{"id": "end", "type": "end"},
			},
			"edges": []any{
				map[string]any{"from": "wa", "to": "end", "condition": "default"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
}

// TestRunChatGraph_Prompt_HappyPath: first inbound sends prompt + yields;
// second inbound validates, stores into SessionData, advances to terminal.
func TestRunChatGraph_Prompt_HappyPath(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newPromptFlow(t, app, org, account, `^[^@]+@[^@]+$`, 3)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "p1", session.CurrentStep, "should park at prompt on first inbound")

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "shri@example.com", "", nil))
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

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))

	// First invalid attempt
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "not-an-email", "", nil))
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

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))

	// First invalid → retry
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "x", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	require.Equal(t, "p1", session.CurrentStep)
	require.Equal(t, 1, session.StepRetries)

	// Second invalid → max_retries → end
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "y", "", nil))
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

// TestRunChatGraph_APICall_MessageTemplateSendsAfter2xx verifies the
// optional message_template path: on 2xx, after response_mapping
// populates SessionData, the templated message is rendered and sent.
// This is the path the converter uses to collapse v1 api_fetch (which
// bundled fetch + send) onto a single v2 api_call node.
func TestRunChatGraph_APICall_MessageTemplateSendsAfter2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"id": "cust-42"},
		})
	}))
	defer server.Close()

	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "api-with-message",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "api",
			"nodes": []any{
				map[string]any{"id": "api", "type": "api_call", "label": "fetch", "config": map[string]any{
					"url":              server.URL,
					"method":           "GET",
					"response_mapping": map[string]any{"customer_id": "data.id"},
					"message_template": "Hello {{customer_id}}!",
				}},
				map[string]any{"id": "end", "type": "end"},
			},
			"edges": []any{
				map[string]any{"from": "api", "to": "end", "condition": "http:2xx"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	assert.Equal(t, "cust-42", session.SessionData["customer_id"])

	// Verify the rendered message was logged on the session.
	var msgs []models.ChatbotSessionMessage
	require.NoError(t, app.DB.Where("session_id = ? AND direction = ?", session.ID, models.DirectionOutgoing).Find(&msgs).Error)
	rendered := ""
	for _, m := range msgs {
		if m.Message == "Hello cust-42!" {
			rendered = m.Message
		}
	}
	assert.Equal(t, "Hello cust-42!", rendered, "message_template should render with response_mapping vars and be logged")
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

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
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

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
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

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)

	path := chatGraphPath(t, session)
	require.GreaterOrEqual(t, len(path), 1)
	assert.Equal(t, "http:non2xx", path[0]["outcome"])
}

// runConditionFlow seeds session.SessionData with `seed`, runs the flow,
// reloads the session, and returns it for assertion.
func runConditionFlow(t *testing.T, app *App, account *models.WhatsAppAccount, contact *models.Contact, session *models.ChatbotSession, flow *models.ChatbotFlow, seed models.JSONB) *models.ChatbotSession {
	t.Helper()
	if seed != nil {
		session.SessionData = seed
		require.NoError(t, app.DB.Save(session).Error)
	}
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	return session
}

// newConditionFlowExpr builds a condition flow whose config uses a
// free-form expr-lang expression.
func newConditionFlowExpr(t *testing.T, app *App, org *models.Organization, account *models.WhatsAppAccount, expression string) *models.ChatbotFlow {
	t.Helper()
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "condition-expr",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "c1",
			"nodes": []any{
				map[string]any{"id": "c1", "type": "condition", "label": "check", "config": map[string]any{
					"expression": expression,
				}},
				map[string]any{"id": "yes", "type": "message", "config": map[string]any{"message": "matched"}},
				map[string]any{"id": "no", "type": "message", "config": map[string]any{"message": "not matched"}},
				map[string]any{"id": "end", "type": "end"},
			},
			"edges": []any{
				map[string]any{"from": "c1", "to": "yes", "condition": "true"},
				map[string]any{"from": "c1", "to": "no", "condition": "false"},
				map[string]any{"from": "yes", "to": "end", "condition": "default"},
				map[string]any{"from": "no", "to": "end", "condition": "default"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

func TestRunChatGraph_Condition_ExpressionAndOr(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newConditionFlowExpr(t, app, org, account, `status == "active" and (tier == "premium" or amount > 100)`)
	s := runConditionFlow(t, app, account, contact, session, flow, models.JSONB{
		"status": "active",
		"tier":   "free",
		"amount": float64(250),
	})
	path := chatGraphPath(t, s)
	assert.Equal(t, "true", path[0]["outcome"], "premium-or-high-amount AND active should be true")
}

func TestRunChatGraph_Condition_ExpressionNot(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newConditionFlowExpr(t, app, org, account, `not banned and status == "active"`)
	s := runConditionFlow(t, app, account, contact, session, flow, models.JSONB{
		"banned": false,
		"status": "active",
	})
	path := chatGraphPath(t, s)
	assert.Equal(t, "true", path[0]["outcome"])
}

func TestRunChatGraph_Condition_ExpressionMissingVariableIsFalse(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	// Referencing an undefined variable is allowed (returns nil), and
	// `== "x"` with nil evaluates false.
	flow := newConditionFlowExpr(t, app, org, account, `never_set == "x"`)
	s := runConditionFlow(t, app, account, contact, session, flow, nil)
	path := chatGraphPath(t, s)
	assert.Equal(t, "false", path[0]["outcome"])
}

func TestRunChatGraph_Condition_ExpressionSyntaxErrorIsFalse(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newConditionFlowExpr(t, app, org, account, `status ==`) // broken
	s := runConditionFlow(t, app, account, contact, session, flow, models.JSONB{"status": "active"})
	path := chatGraphPath(t, s)
	assert.Equal(t, "false", path[0]["outcome"], "compile error should yield false, not error the inbound")
}

func TestRunChatGraph_Condition_ExpressionContainsString(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	// expr-lang provides string helpers — see
	// https://expr-lang.org/docs/language-definition for the full list.
	flow := newConditionFlowExpr(t, app, org, account, `subject contains "billing"`)
	s := runConditionFlow(t, app, account, contact, session, flow, models.JSONB{"subject": "billing question"})
	path := chatGraphPath(t, s)
	assert.Equal(t, "true", path[0]["outcome"])
}

// TestEvaluateTimingSchedule covers the pure time-decision function so the
// timing node's behavior is testable without depending on time.Now().
func TestEvaluateTimingSchedule(t *testing.T) {
	// Mon 2026-03-02 10:30 — inside a 09:00-18:00 Monday schedule.
	now := time.Date(2026, 3, 2, 10, 30, 0, 0, time.UTC)
	in := []any{
		map[string]any{"day": "monday", "enabled": true, "start_time": "09:00", "end_time": "18:00"},
	}
	assert.Equal(t, "in_hours", evaluateTimingSchedule(now, in, nil))

	// Before the window.
	early := time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC)
	assert.Equal(t, "out_of_hours", evaluateTimingSchedule(early, in, nil))

	// After the window.
	late := time.Date(2026, 3, 2, 19, 0, 0, 0, time.UTC)
	assert.Equal(t, "out_of_hours", evaluateTimingSchedule(late, in, nil))

	// Day disabled.
	disabled := []any{
		map[string]any{"day": "monday", "enabled": false},
	}
	assert.Equal(t, "out_of_hours", evaluateTimingSchedule(now, disabled, nil))

	// Day not in schedule.
	otherDay := []any{
		map[string]any{"day": "sunday", "enabled": true, "start_time": "00:00", "end_time": "23:59"},
	}
	assert.Equal(t, "out_of_hours", evaluateTimingSchedule(now, otherDay, nil))

	// Malformed time strings — graceful.
	bad := []any{
		map[string]any{"day": "monday", "enabled": true, "start_time": "x", "end_time": "y"},
	}
	assert.Equal(t, "out_of_hours", evaluateTimingSchedule(now, bad, nil))

	// Empty schedule.
	assert.Equal(t, "out_of_hours", evaluateTimingSchedule(now, nil, nil))
}

// TestRunChatGraph_Timing_RoutesByCurrentTime exercises the full executor
// against a schedule that's guaranteed to cover "now" for the in_hours
// case, and a fully-disabled schedule for the out_of_hours case.
func TestRunChatGraph_Timing_RoutesByCurrentTime(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)

	// Today's weekday, 00:00-23:59 — always in hours regardless of when
	// the test runs.
	today := strings.ToLower(time.Now().Weekday().String())
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "timing-flow",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "t1",
			"nodes": []any{
				map[string]any{"id": "t1", "type": "timing", "label": "biz hours", "config": map[string]any{
					"schedule": []any{
						map[string]any{"day": today, "enabled": true, "start_time": "00:00", "end_time": "23:59"},
					},
				}},
				map[string]any{"id": "open", "type": "message", "config": map[string]any{"message": "We're open."}},
				map[string]any{"id": "closed", "type": "message", "config": map[string]any{"message": "Closed."}},
				map[string]any{"id": "end", "type": "end"},
			},
			"edges": []any{
				map[string]any{"from": "t1", "to": "open", "condition": "in_hours"},
				map[string]any{"from": "t1", "to": "closed", "condition": "out_of_hours"},
				map[string]any{"from": "open", "to": "end", "condition": "default"},
				map[string]any{"from": "closed", "to": "end", "condition": "default"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)

	path := chatGraphPath(t, session)
	require.GreaterOrEqual(t, len(path), 2)
	assert.Equal(t, "in_hours", path[0]["outcome"])
	assert.Equal(t, "open", path[1]["node"])
}

// newSetVariableFlow builds a two-node graph (set_variable → end) whose
// set_variable config is supplied by the caller.
func newSetVariableFlow(t *testing.T, app *App, org *models.Organization, account *models.WhatsAppAccount, set map[string]any) *models.ChatbotFlow {
	t.Helper()
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "set-var-flow",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "s1",
			"nodes": []any{
				map[string]any{"id": "s1", "type": "set_variable", "label": "set", "config": map[string]any{"set": set}},
				map[string]any{"id": "end", "type": "end"},
			},
			"edges": []any{
				map[string]any{"from": "s1", "to": "end", "condition": "default"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

func TestRunChatGraph_SetVariable_Constant(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newSetVariableFlow(t, app, org, account, map[string]any{
		"tier": "premium",
	})

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "premium", session.SessionData["tier"])
}

func TestRunChatGraph_SetVariable_TemplateReferencesExisting(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	session.SessionData = models.JSONB{"customer_name": "Shri"}
	require.NoError(t, app.DB.Save(session).Error)

	flow := newSetVariableFlow(t, app, org, account, map[string]any{
		"greeting": "Hello {{customer_name}}!",
	})

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "Hello Shri!", session.SessionData["greeting"])
}

func TestRunChatGraph_SetVariable_MultipleAtOnce(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newSetVariableFlow(t, app, org, account, map[string]any{
		"a": "1",
		"b": "2",
		"c": "3",
	})

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "1", session.SessionData["a"])
	assert.Equal(t, "2", session.SessionData["b"])
	assert.Equal(t, "3", session.SessionData["c"])
}

func TestRunChatGraph_SetVariable_EmptyConfigNoOp(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newSetVariableFlow(t, app, org, account, map[string]any{})

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
}

func TestRunChatGraph_SetVariable_NonStringStoredVerbatim(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newSetVariableFlow(t, app, org, account, map[string]any{
		"count":     float64(42),
		"is_active": true,
	})

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, float64(42), session.SessionData["count"])
	assert.Equal(t, true, session.SessionData["is_active"])
}

// newAIResponseFlow builds a two-node graph (ai_response → end).
func newAIResponseFlow(t *testing.T, app *App, org *models.Organization, account *models.WhatsAppAccount, promptTemplate string) *models.ChatbotFlow {
	t.Helper()
	cfg := map[string]any{}
	if promptTemplate != "" {
		cfg["prompt_template"] = promptTemplate
	}
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "ai-flow",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "ai",
			"nodes": []any{
				map[string]any{"id": "ai", "type": "ai_response", "label": "ask", "config": cfg},
				map[string]any{"id": "end", "type": "end"},
			},
			"edges": []any{
				map[string]any{"from": "ai", "to": "end", "condition": "default"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

func createChatbotSettings(t *testing.T, app *App, orgID uuid.UUID, accountName string, ai models.AIConfig) {
	t.Helper()
	s := models.ChatbotSettings{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: accountName,
		IsEnabled:       true,
		AI:              ai,
	}
	require.NoError(t, app.DB.Create(&s).Error)
}

// TestRunChatGraph_AIResponse_DisabledFallsThrough: when AI is disabled
// in settings, the node logs + advances via default without sending.
func TestRunChatGraph_AIResponse_DisabledFallsThrough(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	createChatbotSettings(t, app, org.ID, account.Name, models.AIConfig{Enabled: false})
	flow := newAIResponseFlow(t, app, org, account, "")

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "hi", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)

	path := chatGraphPath(t, session)
	require.GreaterOrEqual(t, len(path), 2)
	assert.Equal(t, "default", path[0]["outcome"])
}

// TestRunChatGraph_AIResponse_NoSettingsRowFallsThrough verifies the
// graceful path when no settings have been configured yet.
func TestRunChatGraph_AIResponse_NoSettingsRowFallsThrough(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	// Intentionally no createChatbotSettings call.
	flow := newAIResponseFlow(t, app, org, account, "")

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "hi", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
}

// TestRunChatGraph_AIResponse_MissingAPIKeyFallsThrough: provider set
// but no API key → graceful default outcome (no panic, no upstream call).
func TestRunChatGraph_AIResponse_MissingAPIKeyFallsThrough(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	createChatbotSettings(t, app, org.ID, account.Name, models.AIConfig{
		Enabled:  true,
		Provider: models.AIProviderOpenAI,
		APIKey:   "",
	})
	flow := newAIResponseFlow(t, app, org, account, "")

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "hi", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
}

// newTransferFlow builds a single-node graph (transfer) with caller-
// supplied config. Transfer is terminal so no outgoing edges.
func newTransferFlow(t *testing.T, app *App, org *models.Organization, account *models.WhatsAppAccount, cfg map[string]any) *models.ChatbotFlow {
	t.Helper()
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "transfer-flow",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "t1",
			"nodes": []any{
				map[string]any{"id": "t1", "type": "transfer", "label": "handoff", "config": cfg},
			},
			"edges": []any{},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

// TestRunChatGraph_Transfer_ToQueueCompletesSession verifies that a
// transfer node with no team_id creates a queue transfer and marks the
// session completed.
func TestRunChatGraph_Transfer_ToQueueCompletesSession(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newTransferFlow(t, app, org, account, map[string]any{
		"body":  "Connecting you to a human.",
		"notes": "context",
	})

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "help", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	require.NotNil(t, session.CompletedAt)

	var transfer models.AgentTransfer
	err := app.DB.Where("organization_id = ? AND contact_id = ? AND source = ?",
		org.ID, contact.ID, models.TransferSourceFlow).First(&transfer).Error
	require.NoError(t, err, "transfer row should be created")
	assert.Equal(t, models.TransferStatusActive, transfer.Status)
	assert.Nil(t, transfer.TeamID, "no team_id config → queue transfer")
}

// TestRunChatGraph_Transfer_ToSpecificTeam exercises the team path.
func TestRunChatGraph_Transfer_ToSpecificTeam(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	team := &models.Team{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     org.ID,
		Name:               "Test Team " + uuid.New().String()[:8],
		IsActive:           true,
		AssignmentStrategy: models.AssignmentStrategyRoundRobin,
	}
	require.NoError(t, app.DB.Create(team).Error)

	flow := newTransferFlow(t, app, org, account, map[string]any{
		"team_id": team.ID.String(),
	})

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "help", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)

	var transfer models.AgentTransfer
	err := app.DB.Where("organization_id = ? AND contact_id = ?", org.ID, contact.ID).First(&transfer).Error
	require.NoError(t, err)
	require.NotNil(t, transfer.TeamID)
	assert.Equal(t, team.ID, *transfer.TeamID)
}

// TestRunChatGraph_Transfer_GeneralStringRoutesToQueue: "_general" is a
// sentinel matching the legacy editor's "general queue" option.
func TestRunChatGraph_Transfer_GeneralStringRoutesToQueue(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newTransferFlow(t, app, org, account, map[string]any{
		"team_id": "_general",
	})

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "help", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)

	var transfer models.AgentTransfer
	err := app.DB.Where("organization_id = ? AND contact_id = ?", org.ID, contact.ID).First(&transfer).Error
	require.NoError(t, err)
	assert.Nil(t, transfer.TeamID)
}

// newWebhookFlow builds a webhook → end graph.
func newWebhookFlow(t *testing.T, app *App, org *models.Organization, account *models.WhatsAppAccount, url string) *models.ChatbotFlow {
	t.Helper()
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "webhook-flow",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "wh",
			"nodes": []any{
				map[string]any{"id": "wh", "type": "webhook", "label": "hook", "config": map[string]any{
					"url": url, "method": "POST", "body": `{"phone":"{{phone_number}}"}`,
				}},
				map[string]any{"id": "end", "type": "end"},
			},
			"edges": []any{
				map[string]any{"from": "wh", "to": "end", "condition": "default"},
			},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

func TestRunChatGraph_Webhook_SuccessAdvances(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newWebhookFlow(t, app, org, account, server.URL)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	assert.True(t, called, "webhook should fire")

	path := chatGraphPath(t, session)
	require.GreaterOrEqual(t, len(path), 1)
	assert.Equal(t, "default", path[0]["outcome"])
}

func TestRunChatGraph_Webhook_Non2xxStillAdvances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newWebhookFlow(t, app, org, account, server.URL)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status, "non-2xx should not block advancement")
}

func TestRunChatGraph_Webhook_NetworkErrorStillAdvances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	url := server.URL
	server.Close()

	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newWebhookFlow(t, app, org, account, url)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
}

// newGotoTargetFlow builds a one-node graph (message → end-by-no-edge)
// for use as a goto_flow target.
func newGotoTargetFlow(t *testing.T, app *App, org *models.Organization, account *models.WhatsAppAccount, name, messageText string) *models.ChatbotFlow {
	t.Helper()
	flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            name,
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "m1",
			"nodes": []any{
				map[string]any{"id": "m1", "type": "message", "label": "msg", "config": map[string]any{"message": messageText}},
			},
			"edges": []any{},
		},
	}
	require.NoError(t, app.DB.Create(flow).Error)
	return flow
}

// TestRunChatGraph_GotoFlow_JumpsAndRunsTarget verifies execution
// continues into the target flow within the same webhook run.
func TestRunChatGraph_GotoFlow_JumpsAndRunsTarget(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	target := newGotoTargetFlow(t, app, org, account, "target-flow", "From target")

	source := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "source-flow",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "g1",
			"nodes": []any{
				map[string]any{"id": "g1", "type": "goto_flow", "label": "jump", "config": map[string]any{"flow_id": target.ID.String()}},
			},
			"edges": []any{},
		},
	}
	require.NoError(t, app.DB.Create(source).Error)

	require.NoError(t, app.runChatGraph(account, contact, session, source, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	require.NotNil(t, session.CurrentFlowID)
	assert.Equal(t, target.ID, *session.CurrentFlowID, "should now be in target flow")
	assert.Equal(t, models.SessionStatusCompleted, session.Status, "target's terminal message ended the run")

	path := chatGraphPath(t, session)
	// Path order: executor's direct "goto_flow" jump entry, then the
	// runner's appendChatPath for the source g1 node, then the target's
	// entry node entry from appendChatPath after the reload.
	require.GreaterOrEqual(t, len(path), 3)
	assert.Equal(t, "goto_flow", path[0]["action"])
	assert.Equal(t, target.Name, path[0]["flow"])
	assert.Equal(t, "g1", path[1]["node"])
	assert.Equal(t, "m1", path[2]["node"], "target's entry node should run after the jump")
}

// TestRunChatGraph_GotoFlow_VariablesCarryForward checks SessionData
// survives the jump.
func TestRunChatGraph_GotoFlow_VariablesCarryForward(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	session.SessionData = models.JSONB{"customer_name": "Shri"}
	require.NoError(t, app.DB.Save(session).Error)

	target := newGotoTargetFlow(t, app, org, account, "target-vars", "ignored")
	source := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "source-vars",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "g1",
			"nodes": []any{
				map[string]any{"id": "g1", "type": "goto_flow", "label": "jump", "config": map[string]any{"flow_id": target.ID.String()}},
			},
			"edges": []any{},
		},
	}
	require.NoError(t, app.DB.Create(source).Error)

	require.NoError(t, app.runChatGraph(account, contact, session, source, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, "Shri", session.SessionData["customer_name"], "variables must survive the jump")
}

// TestRunChatGraph_GotoFlow_DisabledTargetIsTerminal verifies graceful
// handling — log + terminal, no panic.
func TestRunChatGraph_GotoFlow_DisabledTargetIsTerminal(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	target := newGotoTargetFlow(t, app, org, account, "disabled-target", "x")
	target.IsEnabled = false
	require.NoError(t, app.DB.Save(target).Error)

	source := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "src-disabled",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "g1",
			"nodes": []any{
				map[string]any{"id": "g1", "type": "goto_flow", "label": "jump", "config": map[string]any{"flow_id": target.ID.String()}},
			},
			"edges": []any{},
		},
	}
	require.NoError(t, app.DB.Create(source).Error)

	require.NoError(t, app.runChatGraph(account, contact, session, source, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status, "disabled target should terminate cleanly")
	// CurrentFlowID stays whatever it was before runChatGraph was called
	// (the dispatcher sets it; the runner doesn't on its own). What
	// matters here is that we did NOT switch to the disabled target.
	if session.CurrentFlowID != nil {
		assert.NotEqual(t, target.ID, *session.CurrentFlowID, "should not switch to disabled target")
	}
}

// TestRunChatGraph_GotoFlow_MissingFlowIDTerminates: malformed config →
// terminal, no error to dispatcher.
func TestRunChatGraph_GotoFlow_MissingFlowIDTerminates(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	source := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "src-missing",
		IsEnabled:       true,
		Graph: models.JSONB{
			"version":    2,
			"entry_node": "g1",
			"nodes": []any{
				map[string]any{"id": "g1", "type": "goto_flow", "label": "jump", "config": map[string]any{}},
			},
			"edges": []any{},
		},
	}
	require.NoError(t, app.DB.Create(source).Error)

	require.NoError(t, app.runChatGraph(account, contact, session, source, "start", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
}

// TestRunChatGraph_GotoFlow_CycleBoundedByMaxIterations: A→B→A cycle
// returns errChatGraphRunaway rather than hanging.
func TestRunChatGraph_GotoFlow_CycleBoundedByMaxIterations(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)

	// Two stub flows referencing each other.
	flowA := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "flow-a",
		IsEnabled:       true,
	}
	flowB := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "flow-b",
		IsEnabled:       true,
	}
	flowA.Graph = models.JSONB{
		"version":    2,
		"entry_node": "g",
		"nodes": []any{
			map[string]any{"id": "g", "type": "goto_flow", "config": map[string]any{"flow_id": flowB.ID.String()}},
		},
	}
	flowB.Graph = models.JSONB{
		"version":    2,
		"entry_node": "g",
		"nodes": []any{
			map[string]any{"id": "g", "type": "goto_flow", "config": map[string]any{"flow_id": flowA.ID.String()}},
		},
	}
	require.NoError(t, app.DB.Create(flowA).Error)
	require.NoError(t, app.DB.Create(flowB).Error)

	err := app.runChatGraph(account, contact, session, flowA, "start", "", nil)
	require.ErrorIs(t, err, errChatGraphRunaway)
}

// TestRunChatGraph_Prompt_NoRegexAcceptsAnything verifies the executor
// treats a prompt with no validation_regex as accept-all.
func TestRunChatGraph_Prompt_NoRegexAcceptsAnything(t *testing.T) {
	app, org, account, contact, session := newGraphTestFixtures(t)
	flow := newPromptFlow(t, app, org, account, "", 3)

	require.NoError(t, app.runChatGraph(account, contact, session, flow, "start", "", nil))
	require.NoError(t, app.runChatGraph(account, contact, session, flow, "literally anything", "", nil))
	require.NoError(t, app.DB.First(session, session.ID).Error)
	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	assert.Equal(t, "literally anything", session.SessionData["email"])
}
