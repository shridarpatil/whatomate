package handlers

import (
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
