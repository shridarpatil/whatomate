package handlers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: pull out the nodes slice from the JSONB output.
func graphNodes(t *testing.T, g models.JSONB) []map[string]any {
	t.Helper()
	require.NotNil(t, g)
	raw, ok := g["nodes"].([]map[string]any)
	require.True(t, ok, "graph.nodes wrong type")
	return raw
}

func graphEdges(t *testing.T, g models.JSONB) []map[string]any {
	t.Helper()
	require.NotNil(t, g)
	raw, ok := g["edges"].([]map[string]any)
	require.True(t, ok, "graph.edges wrong type")
	return raw
}

func TestStepsToGraph_EmptyFlowReturnsNil(t *testing.T) {
	assert.Nil(t, stepsToGraph(nil))
	assert.Nil(t, stepsToGraph(&models.ChatbotFlow{}))
}

func TestStepsToGraph_TextStepProducesMessageNode(t *testing.T) {
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Steps: []models.ChatbotFlowStep{
			{StepName: "step_1", StepOrder: 1, MessageType: "text", Message: "Hi!"},
		},
	}
	g := stepsToGraph(flow)
	require.Equal(t, 2, g["version"])
	assert.Equal(t, "step_1", g["entry_node"])

	nodes := graphNodes(t, g)
	require.Len(t, nodes, 1)
	assert.Equal(t, "message", nodes[0]["type"])
	cfg, _ := nodes[0]["config"].(map[string]any)
	assert.Equal(t, "Hi!", cfg["message"])
}

func TestStepsToGraph_ButtonsStepEmitsPerButtonEdges(t *testing.T) {
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Steps: []models.ChatbotFlowStep{
			{
				StepName: "ask", StepOrder: 1, MessageType: "buttons", Message: "Pick",
				Buttons: models.JSONBArray{
					map[string]any{"id": "yes", "title": "Yes"},
					map[string]any{"id": "no", "title": "No"},
				},
				ConditionalNext: models.JSONB{
					"yes": "thanks",
					"no":  "bye",
				},
			},
			{StepName: "thanks", StepOrder: 2, MessageType: "text", Message: "Cool"},
			{StepName: "bye", StepOrder: 3, MessageType: "text", Message: "OK"},
		},
	}
	g := stepsToGraph(flow)
	edges := graphEdges(t, g)
	// Two button:* edges from ask, plus a default edge from thanks → bye.
	hasYes, hasNo := false, false
	for _, e := range edges {
		if e["from"] == "ask" && e["condition"] == "button:yes" && e["to"] == "thanks" {
			hasYes = true
		}
		if e["from"] == "ask" && e["condition"] == "button:no" && e["to"] == "bye" {
			hasNo = true
		}
	}
	assert.True(t, hasYes && hasNo, "buttons should emit button:<id> edges, got %v", edges)
}

func TestStepsToGraph_ApiFetchToApiCall(t *testing.T) {
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Steps: []models.ChatbotFlowStep{
			{
				StepName: "fetch", StepOrder: 1, MessageType: "api_fetch", Message: "Got {{customer_id}}",
				ApiConfig: models.JSONB{
					"url":              "https://x/api",
					"method":           "POST",
					"headers":          map[string]any{"Auth": "Bearer foo"},
					"body":             `{"a":1}`,
					"response_mapping": map[string]any{"customer_id": "data.id"},
				},
			},
		},
	}
	g := stepsToGraph(flow)
	nodes := graphNodes(t, g)
	require.Len(t, nodes, 1)
	assert.Equal(t, "api_call", nodes[0]["type"])
	cfg, _ := nodes[0]["config"].(map[string]any)
	assert.Equal(t, "https://x/api", cfg["url"])
	assert.Equal(t, "POST", cfg["method"])
	assert.Equal(t, "Got {{customer_id}}", cfg["message_template"])
	rm, _ := cfg["response_mapping"].(map[string]any)
	assert.Equal(t, "data.id", rm["customer_id"])
}

func TestStepsToGraph_WhatsAppFlowFieldRename(t *testing.T) {
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Steps: []models.ChatbotFlowStep{
			{
				StepName: "form", StepOrder: 1, MessageType: "whatsapp_flow", Message: "Open form",
				InputConfig: models.JSONB{
					"whatsapp_flow_id": "meta-1",
					"flow_header":      "Hello",
					"flow_cta":         "Continue",
				},
			},
		},
	}
	g := stepsToGraph(flow)
	cfg, _ := graphNodes(t, g)[0]["config"].(map[string]any)
	assert.Equal(t, "meta-1", cfg["flow_id"])
	assert.Equal(t, "Hello", cfg["header"])
	assert.Equal(t, "Continue", cfg["cta"])
	assert.Equal(t, "Open form", cfg["body"])
}

func TestStepsToGraph_TransferIsTerminal(t *testing.T) {
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Steps: []models.ChatbotFlowStep{
			{
				StepName: "t1", StepOrder: 1, MessageType: "transfer", Message: "Connecting…",
				TransferConfig: models.JSONB{"team_id": "team-uuid", "notes": "n"},
			},
			{StepName: "post", StepOrder: 2, MessageType: "text", Message: "should not connect"},
		},
	}
	g := stepsToGraph(flow)
	edges := graphEdges(t, g)
	for _, e := range edges {
		if e["from"] == "t1" {
			t.Fatalf("transfer should not emit outgoing edges, got %v", e)
		}
	}
}

func TestStepsToGraph_TimingEdgesUseInHoursOutOfHours(t *testing.T) {
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Steps: []models.ChatbotFlowStep{
			{
				StepName: "t1", StepOrder: 1, MessageType: "timing",
				ConditionalNext: models.JSONB{"in_hours": "open", "out_of_hours": "closed"},
				InputConfig:     models.JSONB{"schedule": []any{}},
			},
			{StepName: "open", StepOrder: 2, MessageType: "text", Message: "open"},
			{StepName: "closed", StepOrder: 3, MessageType: "text", Message: "closed"},
		},
	}
	edges := graphEdges(t, stepsToGraph(flow))
	conditions := map[string]string{}
	for _, e := range edges {
		if e["from"] == "t1" {
			conditions[e["condition"].(string)] = e["to"].(string)
		}
	}
	assert.Equal(t, "open", conditions["in_hours"])
	assert.Equal(t, "closed", conditions["out_of_hours"])
}

func TestStepsToGraph_ConditionExpressionPassthrough(t *testing.T) {
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Steps: []models.ChatbotFlowStep{
			{
				StepName: "c1", StepOrder: 1, MessageType: "condition",
				InputConfig: models.JSONB{"expression": `status == "active"`},
			},
		},
	}
	cfg, _ := graphNodes(t, stepsToGraph(flow))[0]["config"].(map[string]any)
	assert.Equal(t, `status == "active"`, cfg["expression"])
}

func TestStepsToGraph_GotoFlowTerminalWithFlowID(t *testing.T) {
	target := uuid.New()
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Steps: []models.ChatbotFlowStep{
			{
				StepName: "g1", StepOrder: 1, MessageType: "goto_flow",
				InputConfig: models.JSONB{"flow_id": target.String()},
			},
		},
	}
	g := stepsToGraph(flow)
	cfg, _ := graphNodes(t, g)[0]["config"].(map[string]any)
	assert.Equal(t, target.String(), cfg["flow_id"])
	for _, e := range graphEdges(t, g) {
		if e["from"] == "g1" {
			t.Fatalf("goto_flow should be terminal in the source flow")
		}
	}
}

func TestStepsToGraph_OrderingFollowsStepOrder(t *testing.T) {
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		// Intentionally out of order — sort by step_order should fix it.
		Steps: []models.ChatbotFlowStep{
			{StepName: "third", StepOrder: 3, MessageType: "text", Message: "3"},
			{StepName: "first", StepOrder: 1, MessageType: "text", Message: "1"},
			{StepName: "second", StepOrder: 2, MessageType: "text", Message: "2"},
		},
	}
	g := stepsToGraph(flow)
	assert.Equal(t, "first", g["entry_node"])
	nodes := graphNodes(t, g)
	require.Len(t, nodes, 3)
	assert.Equal(t, "first", nodes[0]["id"])
	assert.Equal(t, "second", nodes[1]["id"])
	assert.Equal(t, "third", nodes[2]["id"])
}

// TestBackfillChatbotFlowGraph_FillsNullGraphsAndLeavesOthersAlone
// verifies the startup migration against a real DB: legacy flows get
// Graph populated, already-v2 flows are untouched, idempotency holds.
func TestBackfillChatbotFlowGraph_FillsNullGraphsAndLeavesOthersAlone(t *testing.T) {
	app := newProcessorTestApp(t)
	org, account := createProcessorTestOrg(t, app)

	// Legacy: Steps[] only, no Graph.
	legacy := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "legacy",
		IsEnabled:       true,
	}
	require.NoError(t, app.DB.Create(legacy).Error)
	// Create steps separately so the FK is satisfied.
	require.NoError(t, app.DB.Create(&models.ChatbotFlowStep{
		BaseModel:   models.BaseModel{ID: uuid.New()},
		FlowID:      legacy.ID,
		StepName:    "step_1",
		StepOrder:   1,
		MessageType: "text",
		Message:     "Hello!",
	}).Error)

	// Already-v2: explicit Graph, no steps.
	preExisting := models.JSONB{
		"version":    2,
		"entry_node": "m1",
		"nodes": []any{
			map[string]any{"id": "m1", "type": "message", "config": map[string]any{"message": "hi"}},
		},
		"edges": []any{},
	}
	v2flow := &models.ChatbotFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  org.ID,
		WhatsAppAccount: account.Name,
		Name:            "already-v2",
		IsEnabled:       true,
		Graph:           preExisting,
	}
	require.NoError(t, app.DB.Create(v2flow).Error)

	require.NoError(t, BackfillChatbotFlowGraph(app.DB, app.Log))

	var legacyAfter models.ChatbotFlow
	require.NoError(t, app.DB.First(&legacyAfter, legacy.ID).Error)
	require.NotNil(t, legacyAfter.Graph)
	assert.EqualValues(t, 2, legacyAfter.Graph["version"])
	assert.Equal(t, "step_1", legacyAfter.Graph["entry_node"])

	var v2After models.ChatbotFlow
	require.NoError(t, app.DB.First(&v2After, v2flow.ID).Error)
	require.NotNil(t, v2After.Graph)
	// Unchanged: nodes[0].id should still be m1, not auto-generated.
	nodes, _ := v2After.Graph["nodes"].([]any)
	require.Len(t, nodes, 1)
	n0, _ := nodes[0].(map[string]any)
	assert.Equal(t, "m1", n0["id"])

	// Idempotent: re-running should be a no-op (no rows match the filter).
	require.NoError(t, BackfillChatbotFlowGraph(app.DB, app.Log))
}

// Compile-time witness that testutil is used (silences unused import
// warnings in IDEs when the file is read in isolation).
var _ = testutil.NopLogger

func TestStepsToGraph_CanvasPositionsApplied(t *testing.T) {
	flow := &models.ChatbotFlow{
		BaseModel: models.BaseModel{ID: uuid.New()},
		CanvasLayout: models.JSONB{
			"node_positions": map[string]any{
				"step_1": map[string]any{"x": 100.0, "y": 200.0},
			},
		},
		Steps: []models.ChatbotFlowStep{
			{StepName: "step_1", StepOrder: 1, MessageType: "text", Message: "hi"},
		},
	}
	pos, _ := graphNodes(t, stepsToGraph(flow))[0]["position"].(map[string]any)
	assert.Equal(t, 100.0, pos["x"])
	assert.Equal(t, 200.0, pos["y"])
}
