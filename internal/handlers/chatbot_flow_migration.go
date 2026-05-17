package handlers

import (
	"fmt"
	"sort"

	"github.com/zerodha/logf"
	"gorm.io/gorm"

	"github.com/shridarpatil/whatomate/internal/models"
)

// BackfillChatbotFlowGraph fills ChatbotFlow.Graph for every row where
// it is currently NULL by running stepsToGraph against the legacy
// Steps[] + CanvasLayout fields. Idempotent and safe to re-run.
//
// Called from main.go after RunMigrationWithProgress, so the executor
// dispatcher (which prefers Graph when present) starts hitting v2 for
// every flow on the next request. Once every flow has Graph populated
// the legacy executor can be deleted in a follow-up commit.
//
// Flows whose stepsToGraph returns nil (any unsupported message_type
// remains, or zero steps) are skipped and logged at WARN.
func BackfillChatbotFlowGraph(db *gorm.DB, lo logf.Logger) error {
	var flows []models.ChatbotFlow
	if err := db.
		Preload("Steps").
		Where("graph IS NULL OR graph::text = '{}'").
		Find(&flows).Error; err != nil {
		return fmt.Errorf("load flows for graph backfill: %w", err)
	}

	if len(flows) == 0 {
		return nil
	}

	converted, skipped := 0, 0
	for i := range flows {
		flow := &flows[i]
		graph := stepsToGraph(flow)
		if graph == nil {
			skipped++
			lo.Warn("Chatbot flow graph backfill: skipping flow with no v2 mapping",
				"flow_id", flow.ID, "name", flow.Name, "step_count", len(flow.Steps))
			continue
		}
		if err := db.Model(flow).Update("graph", graph).Error; err != nil {
			return fmt.Errorf("save backfilled graph for flow %s: %w", flow.ID, err)
		}
		converted++
	}

	lo.Info("Chatbot flow graph backfill complete",
		"converted", converted, "skipped", skipped, "total", len(flows))
	return nil
}

// stepsToGraph converts a legacy ChatbotFlow.Steps[] + CanvasLayout into
// a v2 graph JSONB blob, mirroring the TypeScript converter in
// frontend/src/composables/useChatbotFlowConverter.ts (stepsToGraph +
// supporting maps).
//
// Returns nil if the flow is empty or contains any message_type that
// has no v2 mapping — callers can fall back to leaving Graph IS NULL.
// After the Phase 4 cutover this nil case should not occur in
// production because every legacy message_type has a v2 mapping.
func stepsToGraph(flow *models.ChatbotFlow) models.JSONB {
	if flow == nil || len(flow.Steps) == 0 {
		return nil
	}

	for i := range flow.Steps {
		if !v2SupportedMessageType(flow.Steps[i].MessageType) {
			return nil
		}
	}

	sorted := make([]models.ChatbotFlowStep, len(flow.Steps))
	copy(sorted, flow.Steps)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].StepOrder < sorted[j].StepOrder
	})

	positions := extractCanvasPositions(flow.CanvasLayout)

	nodes := make([]map[string]any, 0, len(sorted))
	stepNames := make(map[string]struct{}, len(sorted))
	for _, s := range sorted {
		stepNames[s.StepName] = struct{}{}
	}

	for i, step := range sorted {
		nodeType := messageTypeToNodeTypeGo(string(step.MessageType))
		pos := positions[step.StepName]
		if pos == nil {
			pos = map[string]float64{"x": 300, "y": float64(i * 150)}
		}

		config := buildNodeConfig(nodeType, &step)

		label := step.StepName
		// Backend FlowStep doesn't have a label field — fall back to
		// step_name. The editor will display a friendly default via the
		// fallbackLabel helper.

		nodes = append(nodes, map[string]any{
			"id":       step.StepName,
			"type":     nodeType,
			"label":    label,
			"position": map[string]any{"x": pos["x"], "y": pos["y"]},
			"config":   config,
		})
	}

	edges := buildEdges(sorted, stepNames)

	entry := ""
	if len(sorted) > 0 {
		entry = sorted[0].StepName
	}

	return models.JSONB{
		"version":    2,
		"nodes":      nodes,
		"edges":      edges,
		"entry_node": entry,
	}
}

// v2SupportedMessageType keeps the list of v1 → v2 mappings in lockstep
// with the TS V2_SUPPORTED_MESSAGE_TYPES set.
func v2SupportedMessageType(mt models.FlowStepType) bool {
	switch string(mt) {
	case "text", "buttons", "end", "condition", "timing", "goto_flow", "api_fetch", "whatsapp_flow", "transfer":
		return true
	}
	return false
}

func messageTypeToNodeTypeGo(messageType string) string {
	switch messageType {
	case "text":
		return "message"
	case "buttons":
		return "buttons"
	case "end":
		return "end"
	case "condition":
		return "condition"
	case "timing":
		return "timing"
	case "goto_flow":
		return "goto_flow"
	case "api_fetch":
		return "api_call"
	case "whatsapp_flow":
		return "whatsapp_flow"
	case "transfer":
		return "transfer"
	}
	return messageType
}

// extractCanvasPositions reads { node_positions: { name: {x, y} } } out
// of the legacy CanvasLayout JSONB. Returns nil-safe, defaults to empty.
func extractCanvasPositions(raw models.JSONB) map[string]map[string]float64 {
	out := map[string]map[string]float64{}
	if raw == nil {
		return out
	}
	positions, ok := raw["node_positions"].(map[string]any)
	if !ok {
		return out
	}
	for name, posAny := range positions {
		posMap, ok := posAny.(map[string]any)
		if !ok {
			continue
		}
		x, _ := posMap["x"].(float64)
		y, _ := posMap["y"].(float64)
		out[name] = map[string]float64{"x": x, "y": y}
	}
	return out
}

func buildNodeConfig(nodeType string, step *models.ChatbotFlowStep) map[string]any {
	config := map[string]any{}
	switch nodeType {
	case "message":
		config["message"] = step.Message
	case "buttons":
		config["body"] = step.Message
		config["buttons"] = jsonbArrayToSlice(step.Buttons)
	case "end":
		if step.Message != "" {
			config["message"] = step.Message
		}
	case "condition":
		expr, _ := getStringFromJSONB(step.InputConfig, "expression")
		config["expression"] = expr
	case "timing":
		if schedule, ok := step.InputConfig["schedule"].([]any); ok {
			config["schedule"] = schedule
		} else {
			config["schedule"] = []any{}
		}
	case "goto_flow":
		fid, _ := getStringFromJSONB(step.InputConfig, "flow_id")
		config["flow_id"] = fid
	case "api_call":
		ac := step.ApiConfig
		config["url"], _ = getStringFromJSONB(ac, "url")
		method, _ := getStringFromJSONB(ac, "method")
		if method == "" {
			method = "GET"
		}
		config["method"] = method
		if headers, ok := ac["headers"].(map[string]any); ok {
			config["headers"] = headers
		} else {
			config["headers"] = map[string]any{}
		}
		body, _ := getStringFromJSONB(ac, "body")
		config["body"] = body
		if mapping, ok := ac["response_mapping"].(map[string]any); ok {
			config["response_mapping"] = mapping
		} else {
			config["response_mapping"] = map[string]any{}
		}
		if fb, _ := getStringFromJSONB(ac, "fallback_message"); fb != "" {
			config["fallback_message"] = fb
		}
		if step.Message != "" {
			config["message_template"] = step.Message
		}
	case "whatsapp_flow":
		ic := step.InputConfig
		flowID, _ := getStringFromJSONB(ic, "whatsapp_flow_id")
		if flowID == "" {
			flowID, _ = getStringFromJSONB(ic, "flow_id")
		}
		config["flow_id"] = flowID
		header, _ := getStringFromJSONB(ic, "flow_header")
		if header == "" {
			header, _ = getStringFromJSONB(ic, "header")
		}
		config["header"] = header
		cta, _ := getStringFromJSONB(ic, "flow_cta")
		if cta == "" {
			cta, _ = getStringFromJSONB(ic, "cta")
		}
		config["cta"] = cta
		if step.Message != "" {
			config["body"] = step.Message
		}
	case "transfer":
		tc := step.TransferConfig
		if teamID, _ := getStringFromJSONB(tc, "team_id"); teamID != "" {
			config["team_id"] = teamID
		}
		if notes, _ := getStringFromJSONB(tc, "notes"); notes != "" {
			config["notes"] = notes
		}
		if step.Message != "" {
			config["body"] = step.Message
		}
	}
	return config
}

func buildEdges(sorted []models.ChatbotFlowStep, stepNames map[string]struct{}) []map[string]any {
	edges := make([]map[string]any, 0)

	for i, step := range sorted {
		var nextSequential string
		if i < len(sorted)-1 {
			nextSequential = sorted[i+1].StepName
		}

		mt := string(step.MessageType)
		switch mt {
		case "buttons":
			edges = append(edges, buttonsEdges(step, nextSequential, stepNames)...)
		case "condition":
			edges = append(edges, branchEdges(step, []string{"true", "false"}, stepNames)...)
		case "timing":
			edges = append(edges, branchEdges(step, []string{"in_hours", "out_of_hours"}, stepNames)...)
		case "transfer", "end", "goto_flow":
			// Terminal — no outgoing edges.
		default:
			// message / api_fetch / whatsapp_flow — sequential fallthrough.
			target := step.NextStep
			if target == "" {
				target = nextSequential
			}
			if target != "" {
				if _, ok := stepNames[target]; ok {
					edges = append(edges, map[string]any{
						"from": step.StepName, "to": target, "condition": "default",
					})
				}
			}
		}
	}
	return edges
}

func buttonsEdges(step models.ChatbotFlowStep, nextSequential string, stepNames map[string]struct{}) []map[string]any {
	edges := make([]map[string]any, 0)
	mapped := map[string]struct{}{}

	for buttonID, targetAny := range step.ConditionalNext {
		target, _ := targetAny.(string)
		if target == "" {
			continue
		}
		mapped[buttonID] = struct{}{}
		if _, ok := stepNames[target]; ok {
			edges = append(edges, map[string]any{
				"from": step.StepName, "to": target, "condition": "button:" + buttonID,
			})
		}
	}

	// Unmapped buttons fall through to the next sequential step.
	if nextSequential != "" {
		if _, ok := stepNames[nextSequential]; ok {
			for _, btn := range step.Buttons {
				btnMap, ok := btn.(map[string]any)
				if !ok {
					continue
				}
				id, _ := btnMap["id"].(string)
				if id == "" {
					continue
				}
				if _, already := mapped[id]; already {
					continue
				}
				edges = append(edges, map[string]any{
					"from": step.StepName, "to": nextSequential, "condition": "button:" + id,
				})
			}
		}
	}
	return edges
}

func branchEdges(step models.ChatbotFlowStep, handles []string, stepNames map[string]struct{}) []map[string]any {
	edges := make([]map[string]any, 0, len(handles))
	for _, h := range handles {
		target, _ := step.ConditionalNext[h].(string)
		if target == "" {
			continue
		}
		if _, ok := stepNames[target]; ok {
			edges = append(edges, map[string]any{
				"from": step.StepName, "to": target, "condition": h,
			})
		}
	}
	return edges
}

func jsonbArrayToSlice(arr models.JSONBArray) []any {
	out := make([]any, 0, len(arr))
	out = append(out, arr...)
	return out
}

func getStringFromJSONB(j models.JSONB, key string) (string, bool) {
	if j == nil {
		return "", false
	}
	if v, ok := j[key].(string); ok {
		return v, true
	}
	return "", false
}
