package handlers

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/zerodha/logf"
	"gorm.io/gorm"

	"github.com/shridarpatil/whatomate/internal/models"
)

// BackfillChatbotFlowGraph fills ChatbotFlow.Graph for every row where
// it is currently NULL by running stepsToGraph against the legacy
// chatbot_flow_steps table + canvas_layout column. Idempotent and safe
// to re-run.
//
// Called from main.go after RunMigrationWithProgress. Once every flow
// has Graph populated the dispatcher uses the v2 runner unconditionally.
// The legacy table and canvas_layout column are not dropped — they're
// quarantined as dead data until a future maintenance migration.
//
// The migration reads the legacy tables via raw queries so this package
// no longer needs Steps[] / CanvasLayout fields on the ChatbotFlow
// struct.
func BackfillChatbotFlowGraph(db *gorm.DB, lo logf.Logger) error {
	// Fresh installs never had the legacy schema — chatbot_flows lacks
	// the canvas_layout column and chatbot_flow_steps does not exist.
	// Skip in that case so a clean boot doesn't fail.
	mig := db.Migrator()
	if !mig.HasColumn(&models.ChatbotFlow{}, "canvas_layout") || !mig.HasTable("chatbot_flow_steps") {
		return nil
	}

	var pending []legacyFlowMeta
	err := db.Raw(`
		SELECT id, name, COALESCE(canvas_layout, '{}'::jsonb) AS canvas_layout
		FROM chatbot_flows
		WHERE (graph IS NULL OR graph::text = '{}')
		  AND deleted_at IS NULL
	`).Scan(&pending).Error
	if err != nil {
		return fmt.Errorf("load flows for graph backfill: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	converted, skipped := 0, 0
	for _, p := range pending {
		var steps []models.ChatbotFlowStep
		if err := db.Where("flow_id = ?", p.ID).Find(&steps).Error; err != nil {
			return fmt.Errorf("load steps for flow %s: %w", p.ID, err)
		}

		graph := stepsToGraph(steps, p.CanvasLayout)
		if graph == nil {
			skipped++
			lo.Warn("Chatbot flow graph backfill: skipping flow with no v2 mapping",
				"flow_id", p.ID, "name", p.Name, "step_count", len(steps))
			continue
		}
		if err := db.Model(&models.ChatbotFlow{}).Where("id = ?", p.ID).Update("graph", graph).Error; err != nil {
			return fmt.Errorf("save backfilled graph for flow %s: %w", p.ID, err)
		}
		converted++
	}

	lo.Info("Chatbot flow graph backfill complete",
		"converted", converted, "skipped", skipped, "total", len(pending))
	return nil
}

type legacyFlowMeta struct {
	ID           uuid.UUID
	Name         string
	CanvasLayout models.JSONB `gorm:"column:canvas_layout"`
}

// stepsToGraph converts a legacy slice of ChatbotFlowStep rows plus an
// optional canvas_layout into a v2 graph JSONB blob, mirroring the
// TypeScript converter in frontend/src/composables/useChatbotFlowConverter.ts.
//
// Returns nil if there are no steps or any step uses a message_type
// that has no v2 mapping.
func stepsToGraph(steps []models.ChatbotFlowStep, canvasLayout models.JSONB) models.JSONB {
	if len(steps) == 0 {
		return nil
	}

	for i := range steps {
		if !v2SupportedMessageType(steps[i].MessageType) {
			return nil
		}
	}

	sorted := make([]models.ChatbotFlowStep, len(steps))
	copy(sorted, steps)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].StepOrder < sorted[j].StepOrder
	})

	positions := extractCanvasPositions(canvasLayout)

	nodes := make([]map[string]any, 0, len(sorted))
	stepNames := make(map[string]struct{}, len(sorted))
	for _, s := range sorted {
		stepNames[s.StepName] = struct{}{}
	}

	for i, step := range sorted {
		// v1 text steps that capture a reply become v2 prompt nodes
		// (blocking + validating + storing), matching the editor's UX
		// intent. Trigger on an explicit input_type OR on store_as /
		// validation being configured — a step that only set store_as
		// (no input_type) still needs to capture its reply rather than
		// being flattened to a plain message that drops {{var}}.
		nodeType := messageTypeToNodeTypeGo(string(step.MessageType))
		if step.MessageType == models.FlowStepTypeText && textStepCapturesInput(&step) {
			nodeType = "prompt"
		}

		pos := positions[step.StepName]
		if pos == nil {
			pos = map[string]float64{"x": 300, "y": float64(i * 150)}
		}

		config := buildNodeConfig(nodeType, &step)

		nodes = append(nodes, map[string]any{
			"id":       step.StepName,
			"type":     nodeType,
			"label":    step.StepName,
			"position": map[string]any{"x": pos["x"], "y": pos["y"]},
			"config":   config,
		})
	}

	edges := buildEdges(sorted, stepNames)

	// Prepend a fixed `start` sentinel node so the editor always has a
	// non-deletable entry point. The first ordered step becomes the
	// target of start's default edge.
	const startID = "__start__"
	if len(sorted) > 0 {
		nodes = append([]map[string]any{{
			"id":       startID,
			"type":     "start",
			"label":    "Start",
			"position": map[string]any{"x": 100, "y": 100},
			"config":   map[string]any{},
		}}, nodes...)
		edges = append([]map[string]any{{
			"from":      startID,
			"to":        sorted[0].StepName,
			"condition": "default",
		}}, edges...)
	}

	return models.JSONB{
		"version":    2,
		"nodes":      nodes,
		"edges":      edges,
		"entry_node": startID,
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

// textStepCapturesInput reports whether a legacy text step expected and
// stored a user reply. Such steps map to a v2 prompt (which blocks + stores)
// rather than a plain message (which only sends), so {{var}} interpolation
// keeps working after migration.
func textStepCapturesInput(step *models.ChatbotFlowStep) bool {
	if step.InputType != "" && step.InputType != models.InputTypeNone {
		return true
	}
	return step.StoreAs != "" || step.ValidationRegex != ""
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
// of the legacy CanvasLayout JSONB.
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
	case "prompt":
		config["body"] = step.Message
		if step.ValidationRegex != "" {
			config["validation_regex"] = step.ValidationRegex
		}
		if step.ValidationError != "" {
			config["validation_error"] = step.ValidationError
		}
		if step.StoreAs != "" {
			config["store_as"] = step.StoreAs
		}
		if step.MaxRetries > 0 {
			config["max_retries"] = step.MaxRetries
		}
	case "buttons":
		config["body"] = step.Message
		config["buttons"] = jsonbArrayToSlice(step.Buttons)
		if step.StoreAs != "" {
			config["store_as"] = step.StoreAs
		}
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
