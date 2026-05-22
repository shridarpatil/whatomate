package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
)

// maxChatGraphIterations bounds non-blocking node chains within a single
// inbound message. A graph that loops through non-blocking nodes forever
// returns errChatGraphRunaway instead of wedging the webhook goroutine.
const maxChatGraphIterations = 100

var errChatGraphRunaway = errors.New("chat graph: too many non-blocking nodes in a single inbound (cycle?)")

// chatNodeCtx carries the per-inbound execution state for a single
// runChatGraph call. userInput is the message text; buttonID is the
// payload of an interactive button reply (empty for text messages).
// consumed flips to true after the first blocking node treats the input
// as its outcome, so a later blocking node in the same run doesn't see
// stale input.
type chatNodeCtx struct {
	account          *models.WhatsAppAccount
	contact          *models.Contact
	session          *models.ChatbotSession
	userInput        string
	buttonID         string
	flowResponseData map[string]any // form fields from a WhatsApp Flow submission
	consumed         bool
}

// nodeOutcome is the return value of a node executor.
//   - outcome is the edge condition used to pick the next node ("default",
//     "button:foo", "http:2xx", ...). Empty means "no edge needed", and
//     when paired with yield=false collapses the session as terminal.
//   - yield=true means "stay at this node; persist state and return so the
//     next inbound resumes here." Used by blocking nodes that haven't
//     received their input yet (e.g. buttons sent, awaiting click).
//   - yield=false means "advance via resolveEdge(node, outcome)". Used by
//     non-blocking nodes (message, set_variable, ...) AND by blocking
//     nodes that have just consumed their input (e.g. buttons + buttonID).
type nodeOutcome struct {
	outcome string
	yield   bool
}

// runChatGraph executes the v2 graph for a session against a single inbound
// message. It chains through non-blocking nodes and stops at the first
// blocking node (or at a terminal node with no outgoing edges).
//
// On entry:
//   - If session.CurrentStep is empty, execution starts at graph.EntryNode
//     and userInput/buttonID are treated as the trigger that started the
//     flow (not input to the entry node).
//   - Otherwise, execution resumes at session.CurrentStep with the input
//     applied to that node.
func (a *App) runChatGraph(
	account *models.WhatsAppAccount,
	contact *models.Contact,
	session *models.ChatbotSession,
	flow *models.ChatbotFlow,
	userInput string,
	buttonID string,
	flowResponseData map[string]any,
) error {
	graph, err := parseChatGraph(flow.Graph)
	if err != nil {
		return fmt.Errorf("parse chat graph: %w", err)
	}
	if graph == nil {
		return errors.New("flow has no v2 graph; legacy executor should have run")
	}

	ctx := &chatNodeCtx{
		account:          account,
		contact:          contact,
		session:          session,
		userInput:        userInput,
		buttonID:         buttonID,
		flowResponseData: flowResponseData,
	}

	// Seed built-in template variables so {{phone_number}} / {{contact_name}}
	// work in any outgoing message without needing an upstream api_call.
	if session.SessionData == nil {
		session.SessionData = models.JSONB{}
	}
	session.SessionData["phone_number"] = session.PhoneNumber
	if contact != nil {
		session.SessionData["contact_name"] = contact.ProfileName
	}

	if session.CurrentStep == "" {
		session.CurrentStep = graph.EntryNode
		// Trigger input is not "for" the entry node — clear it so we don't
		// double-count the trigger keyword as user input.
		ctx.userInput = ""
		ctx.buttonID = ""
	}

	for range maxChatGraphIterations {
		node := graph.getNode(session.CurrentStep)
		if node == nil {
			a.Log.Error("chat graph node not found",
				"session", session.ID, "node_id", session.CurrentStep, "flow", flow.ID)
			return fmt.Errorf("node %q not found", session.CurrentStep)
		}

		// Skip condition: if the node config has a truthy expression,
		// short-circuit through the default edge without executing the
		// node. Authored from the editor's per-node Advanced section.
		if expr := stringFromConfig(node.Config, "skip_condition"); expr != "" {
			matched, err := evaluateConditionExpression(expr, session.SessionData)
			if err != nil {
				a.Log.Warn("skip_condition failed; ignoring",
					"node", node.ID, "session", session.ID, "expression", expr, "error", err)
			} else if matched {
				appendChatPath(session, node, "skipped")
				next := graph.resolveEdge(node.ID, "default")
				if next == "" {
					session.Status = models.SessionStatusCompleted
					return a.persistChatSession(session)
				}
				session.CurrentStep = next
				continue
			}
		}

		res, err := a.executeChatNode(node, ctx)
		if err != nil {
			_ = a.persistChatSession(session)
			return err
		}

		appendChatPath(session, node, res.outcome)

		if res.yield {
			// Stay at this node; next inbound resumes here.
			return a.persistChatSession(session)
		}

		// goto_flow may have switched the session to a different flow.
		// Reload graph + flow and continue at the new entry node within
		// the same run, mirroring IVR's executeGotoFlow recursion. The
		// outer loop's max-iteration guard prevents A→B→A pathologies.
		if session.CurrentFlowID != nil && *session.CurrentFlowID != flow.ID {
			newFlow, err := a.getChatbotFlowByIDCached(account.OrganizationID, *session.CurrentFlowID)
			if err != nil {
				_ = a.persistChatSession(session)
				return fmt.Errorf("goto_flow: load target: %w", err)
			}
			if newFlow.Graph == nil {
				_ = a.persistChatSession(session)
				return errors.New("goto_flow: target flow has no v2 graph")
			}
			newGraph, err := parseChatGraph(newFlow.Graph)
			if err != nil {
				_ = a.persistChatSession(session)
				return fmt.Errorf("goto_flow: parse target graph: %w", err)
			}
			flow = newFlow
			graph = newGraph
			session.CurrentStep = newGraph.EntryNode
			continue
		}

		next := graph.resolveEdge(node.ID, res.outcome)
		if next == "" {
			// No matching edge → terminal.
			session.Status = models.SessionStatusCompleted
			return a.persistChatSession(session)
		}
		session.CurrentStep = next
	}

	_ = a.persistChatSession(session)
	return errChatGraphRunaway
}

// executeChatNode dispatches by node type. Phase 1 implements only
// message, buttons, and end. Other types return an error until their
// PR lands.
func (a *App) executeChatNode(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	switch node.Type {
	case ChatNodeStart:
		// Always-present entry sentinel. No side effect — falls through.
		_ = node
		_ = ctx
		return nodeOutcome{outcome: "default"}, nil
	case ChatNodeMessage:
		return a.execChatMessage(node, ctx)
	case ChatNodeButtons:
		return a.execChatButtons(node, ctx)
	case ChatNodePrompt:
		return a.execChatPrompt(node, ctx)
	case ChatNodeAPICall:
		return a.execChatAPICall(node, ctx)
	case ChatNodeCondition:
		return a.execChatCondition(node, ctx)
	case ChatNodeTiming:
		return a.execChatTiming(node, ctx)
	case ChatNodeSetVariable:
		return a.execChatSetVariable(node, ctx)
	case ChatNodeAIResponse:
		return a.execChatAIResponse(node, ctx)
	case ChatNodeTransfer:
		return a.execChatTransfer(node, ctx)
	case ChatNodeWebhook:
		return a.execChatWebhook(node, ctx)
	case ChatNodeGotoFlow:
		return a.execChatGotoFlow(node, ctx)
	case ChatNodeWhatsAppFlow:
		return a.execChatWhatsAppFlow(node, ctx)
	case ChatNodeEnd:
		return a.execChatEnd(node, ctx)
	default:
		return nodeOutcome{outcome: "", yield: true},
			fmt.Errorf("chat node type %q not implemented in this phase", node.Type)
	}
}

// execChatMessage sends a text message and falls through. The message
// body is rendered with processTemplate against SessionData so authors
// can interpolate captured variables (e.g. "Hi {{customer_name}}").
// Config: { "message": "..." } or "text" for compatibility.
func (a *App) execChatMessage(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	text := stringFromConfig(node.Config, "message", "text")
	if text == "" {
		return nodeOutcome{outcome: "default"}, nil
	}
	text = processTemplate(text, ctx.session.SessionData)
	if err := a.sendAndSaveTextMessage(ctx.account, ctx.contact, text); err != nil {
		return nodeOutcome{}, fmt.Errorf("send message: %w", err)
	}
	a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, text, node.ID)
	return nodeOutcome{outcome: "default"}, nil
}

// execChatButtons sends interactive buttons on first entry (yielding to
// wait for a click); on a later inbound that carries a buttonID, consumes
// the selection and returns "button:<id>" so the runner can resolve the
// next edge and advance.
// Config: { "body": "...", "buttons": [{ "id": "...", "title": "..." }, ...] }
func (a *App) execChatButtons(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	if !ctx.consumed && ctx.buttonID != "" {
		ctx.consumed = true
		return nodeOutcome{outcome: "button:" + ctx.buttonID}, nil
	}

	body := stringFromConfig(node.Config, "body", "message", "text")
	if body == "" {
		body = node.Label
	}
	body = processTemplate(body, ctx.session.SessionData)
	buttons := buttonsFromConfig(node.Config)
	if len(buttons) == 0 {
		return nodeOutcome{}, fmt.Errorf("buttons node %q has no buttons configured", node.ID)
	}
	// Template each button's user-facing fields so authors can
	// interpolate variables into titles / urls / phone numbers too.
	for _, b := range buttons {
		for _, key := range []string{"title", "url", "phone_number"} {
			if s, ok := b[key].(string); ok && s != "" {
				b[key] = processTemplate(s, ctx.session.SessionData)
			}
		}
	}
	if err := a.sendAndSaveInteractiveButtons(ctx.account, ctx.contact, body, buttons); err != nil {
		return nodeOutcome{}, fmt.Errorf("send buttons: %w", err)
	}
	a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, body, node.ID)
	return nodeOutcome{yield: true}, nil
}

// execChatPrompt asks the user for input. On first entry (no userInput),
// sends the prompt body and yields to wait for a reply. On a later inbound,
// validates ctx.userInput against an optional regex:
//   - valid (or no regex): stores the input in SessionData under store_as,
//     resets StepRetries, returns outcome="default" so the runner advances.
//   - invalid + StepRetries+1 < max_retries: sends the validation error,
//     yields to re-prompt (the same node will fire again on next inbound).
//   - invalid + StepRetries+1 >= max_retries: returns outcome="max_retries"
//     so the runner can route to an error branch (or terminate if none).
//
// Config:
//
//	{
//	  "body": "...",                 // prompt sent on first entry
//	  "validation_regex": "...",     // optional; default = accept anything
//	  "validation_error": "...",     // optional; default fallback message
//	  "store_as": "var_name",        // optional; persists input into SessionData
//	  "max_retries": 3               // optional; default 3
//	}
func (a *App) execChatPrompt(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	body := stringFromConfig(node.Config, "body", "message", "text")

	// No input yet → send prompt and wait.
	if !ctx.consumed && ctx.userInput == "" {
		if body == "" {
			return nodeOutcome{}, fmt.Errorf("prompt node %q has no body configured", node.ID)
		}
		rendered := processTemplate(body, ctx.session.SessionData)
		if err := a.sendAndSaveTextMessage(ctx.account, ctx.contact, rendered); err != nil {
			return nodeOutcome{}, fmt.Errorf("send prompt: %w", err)
		}
		a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, rendered, node.ID)
		return nodeOutcome{yield: true}, nil
	}

	if ctx.consumed {
		// Input was already consumed by an earlier blocking node in this
		// run — defensive guard. Treat as fresh entry.
		return nodeOutcome{yield: true}, nil
	}

	ctx.consumed = true
	input := ctx.userInput

	validationRegex := stringFromConfig(node.Config, "validation_regex")
	if validationRegex != "" {
		re, err := regexp.Compile(validationRegex)
		if err != nil {
			a.Log.Error("prompt node has invalid regex",
				"node", node.ID, "regex", validationRegex, "error", err)
			// Skip validation rather than failing the user-facing flow.
		} else if !re.MatchString(input) {
			return a.handleChatPromptInvalid(node, ctx)
		}
	}

	// Valid → persist + advance.
	if storeAs := stringFromConfig(node.Config, "store_as"); storeAs != "" {
		if ctx.session.SessionData == nil {
			ctx.session.SessionData = models.JSONB{}
		}
		ctx.session.SessionData[storeAs] = input
	}
	ctx.session.StepRetries = 0
	return nodeOutcome{outcome: "default"}, nil
}

func (a *App) handleChatPromptInvalid(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	ctx.session.StepRetries++

	maxRetries := intFromConfig(node.Config, "max_retries", 3)
	if ctx.session.StepRetries >= maxRetries {
		ctx.session.StepRetries = 0
		return nodeOutcome{outcome: "max_retries"}, nil
	}

	errorMsg := stringFromConfig(node.Config, "validation_error")
	if errorMsg == "" {
		errorMsg = "Invalid input. Please try again."
	}
	errorMsg = processTemplate(errorMsg, ctx.session.SessionData)
	if err := a.sendAndSaveTextMessage(ctx.account, ctx.contact, errorMsg); err != nil {
		return nodeOutcome{}, fmt.Errorf("send validation error: %w", err)
	}
	a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, errorMsg, node.ID)
	return nodeOutcome{yield: true}, nil
}

// execChatAPICall fires an HTTP request defined in node.Config and routes
// via "http:2xx" / "http:non2xx" outcomes. Mirrors fetchApiResponse's
// approach to template interpolation (seeds {{phone_number}}) and
// response_mapping (extracted keys are merged into SessionData so later
// nodes can reference them through processTemplate).
//
// Non-blocking — the runner immediately advances via resolveEdge after
// this returns. Network errors are mapped to "http:non2xx" so the graph
// can route to a fallback path; logged for visibility.
//
// Config:
//
//	{
//	  "url":     "https://api.example.com/lookup?phone={{phone_number}}",
//	  "method":  "POST",
//	  "headers": { "Authorization": "Bearer {{token}}" },
//	  "body":    "{\"phone\":\"{{phone_number}}\"}",
//	  "response_mapping": { "customer_id": "data.id", "status": "data.status" },
//	  // Optional. If set, a 2xx response renders this template against
//	  // SessionData (post-response_mapping) and sends it to the user.
//	  // Lets the same node act as v1's "fetch + send templated message"
//	  // pattern without forcing authors to chain a separate message node.
//	  "message_template": "Hello {{customer_id}}!"
//	}
func (a *App) execChatAPICall(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	cfgJSONB := models.JSONB(node.Config)

	if ctx.session.SessionData == nil {
		ctx.session.SessionData = models.JSONB{}
	}
	sessionData := ctx.session.SessionData
	sessionData["phone_number"] = ctx.session.PhoneNumber

	replaceVar := func(s string) string { return processTemplate(s, sessionData) }
	respBody, statusCode, err := a.executeConfiguredAPI(cfgJSONB, replaceVar)
	if err != nil {
		a.Log.Error("api_call node request failed",
			"node", node.ID, "session", ctx.session.ID, "error", err)
		return nodeOutcome{outcome: "http:non2xx"}, nil
	}

	if statusCode < 200 || statusCode >= 300 {
		return nodeOutcome{outcome: "http:non2xx"}, nil
	}

	// 2xx: optionally extract response_mapping → SessionData.
	if mapping, ok := node.Config["response_mapping"].(map[string]any); ok && len(mapping) > 0 {
		var jsonResp map[string]any
		if err := json.Unmarshal(respBody, &jsonResp); err == nil {
			mappingStrings := make(map[string]string, len(mapping))
			for varName, path := range mapping {
				if pathStr, ok := path.(string); ok {
					mappingStrings[varName] = pathStr
				}
			}
			extracted := extractResponseMapping(jsonResp, mappingStrings)
			maps.Copy(sessionData, extracted)
		}
	}

	// Optionally render and send a message after the fetch. Mirrors v1
	// api_fetch's bundled "fetch + send" behavior so the converter can
	// keep collapsing api_fetch steps onto a single api_call node.
	if tmpl := stringFromConfig(node.Config, "message_template"); tmpl != "" {
		rendered := processTemplate(tmpl, sessionData)
		if rendered != "" {
			if err := a.sendAndSaveTextMessage(ctx.account, ctx.contact, rendered); err != nil {
				a.Log.Error("api_call node failed to send message_template",
					"node", node.ID, "session", ctx.session.ID, "error", err)
				// Still advance via http:2xx — the data fetch succeeded.
			} else {
				a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, rendered, node.ID)
			}
		}
	}

	return nodeOutcome{outcome: "http:2xx"}, nil
}

// execChatCondition is a pure-branch node: evaluates a free-form
// boolean expression against SessionData and returns "true" or "false"
// so an edge can route accordingly. No message is sent.
//
// Config:
//
//	{
//	  "expression": "status == \"active\" and (tier == \"premium\" or amount > 100)"
//	}
//
// The expression is evaluated by github.com/expr-lang/expr. SessionData
// keys are available as top-level identifiers. Unknown identifiers
// resolve to nil (so `status == "active"` evaluates false when status
// isn't set, rather than throwing). Compile/runtime errors map to
// outcome "false" so the inbound webhook doesn't error.
func (a *App) execChatCondition(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	expression := stringFromConfig(node.Config, "expression")
	if expression == "" {
		a.Log.Warn("condition node missing expression",
			"node", node.ID, "session", ctx.session.ID)
		return nodeOutcome{outcome: "false"}, nil
	}

	matched, err := evaluateConditionExpression(expression, ctx.session.SessionData)
	if err != nil {
		a.Log.Warn("condition node expression failed",
			"node", node.ID, "session", ctx.session.ID, "expression", expression, "error", err)
		return nodeOutcome{outcome: "false"}, nil
	}
	if matched {
		return nodeOutcome{outcome: "true"}, nil
	}
	return nodeOutcome{outcome: "false"}, nil
}

// evaluateConditionExpression compiles + runs a boolean expression via
// expr-lang/expr against SessionData. The result is coerced to bool —
// non-bool truthy values count as true (matches expr's natural casting).
func evaluateConditionExpression(expression string, data models.JSONB) (bool, error) {
	env := make(map[string]any, len(data))
	maps.Copy(env, data)

	program, err := expr.Compile(expression, expr.Env(env), expr.AllowUndefinedVariables())
	if err != nil {
		return false, fmt.Errorf("compile: %w", err)
	}
	out, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("run: %w", err)
	}
	switch v := out.(type) {
	case bool:
		return v, nil
	case nil:
		return false, nil
	case string:
		return v != "" && strings.ToLower(v) != "false", nil
	case float64:
		return v != 0, nil
	case int:
		return v != 0, nil
	}
	// Anything else (slices, maps) counts as truthy if non-nil.
	return out != nil, nil
}

// execChatTiming routes "in_hours" / "out_of_hours" based on a per-day
// schedule. Mirrors IVR's executeTiming (internal/calling/ivr.go:539).
// Non-blocking; no message sent.
//
// Config:
//
//	{
//	  "schedule": [
//	    { "day": "monday", "enabled": true,  "start_time": "09:00", "end_time": "18:00" },
//	    { "day": "sunday", "enabled": false }
//	  ]
//	}
//
// Days not listed in the schedule are treated as out_of_hours, matching
// IVR's behavior.
func (a *App) execChatTiming(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	rawSchedule, _ := node.Config["schedule"].([]any)
	outcome := evaluateTimingSchedule(time.Now(), rawSchedule, a.Log)
	_ = ctx // ctx unused — included for symmetry with other executors
	return nodeOutcome{outcome: outcome}, nil
}

// evaluateTimingSchedule is the pure decision function, factored out for
// unit-testing with a fixed clock. Returns "in_hours" or "out_of_hours".
func evaluateTimingSchedule(now time.Time, schedule []any, log scheduleLogger) string {
	dayName := strings.ToLower(now.Weekday().String())
	for _, item := range schedule {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		day, _ := entry["day"].(string)
		if strings.ToLower(day) != dayName {
			continue
		}
		enabled, _ := entry["enabled"].(bool)
		if !enabled {
			return "out_of_hours"
		}
		startStr, _ := entry["start_time"].(string)
		endStr, _ := entry["end_time"].(string)
		startTime, err1 := time.Parse("15:04", startStr)
		endTime, err2 := time.Parse("15:04", endStr)
		if err1 != nil || err2 != nil {
			if log != nil {
				log.Warn("timing node has invalid time format",
					"start", startStr, "end", endStr)
			}
			return "out_of_hours"
		}
		nowMinutes := now.Hour()*60 + now.Minute()
		startMinutes := startTime.Hour()*60 + startTime.Minute()
		endMinutes := endTime.Hour()*60 + endTime.Minute()
		if nowMinutes >= startMinutes && nowMinutes < endMinutes {
			return "in_hours"
		}
		return "out_of_hours"
	}
	// Day not configured — treat as out of hours.
	return "out_of_hours"
}

// scheduleLogger is the subset of the app logger evaluateTimingSchedule
// needs. Defined locally to avoid pulling logf into the test imports.
type scheduleLogger interface {
	Warn(msg string, args ...any)
}

// execChatSetVariable assigns one or more values into SessionData. Each
// value runs through processTemplate against current SessionData first,
// so authors can compose new variables from existing ones (e.g.
// "greeting" = "Hello {{customer_name}}!"). Non-blocking; outcome
// "default".
//
// Config:
//
//	{
//	  "set": {
//	    "greeting":  "Hello {{customer_name}}!",
//	    "tier":      "premium"
//	  }
//	}
func (a *App) execChatSetVariable(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	assignments, _ := node.Config["set"].(map[string]any)
	if len(assignments) == 0 {
		return nodeOutcome{outcome: "default"}, nil
	}

	if ctx.session.SessionData == nil {
		ctx.session.SessionData = models.JSONB{}
	}
	for name, raw := range assignments {
		if name == "" {
			continue
		}
		tmpl, ok := raw.(string)
		if !ok {
			// Non-string assignments are stored verbatim — useful for
			// numbers or booleans authored directly in the editor.
			ctx.session.SessionData[name] = raw
			continue
		}
		ctx.session.SessionData[name] = processTemplate(tmpl, ctx.session.SessionData)
	}
	return nodeOutcome{outcome: "default"}, nil
}

// execChatAIResponse invokes the configured LLM provider via the
// existing generateAIResponse helper, sends the answer back to the user,
// and falls through. Reuses the org's chatbot settings (provider,
// model, api key, system prompt) so authors don't have to duplicate
// credentials per node.
//
// Input to the LLM is, in priority order:
//  1. config.prompt_template — runs through processTemplate, useful when
//     the AI should respond to a structured request rather than the raw
//     user text (e.g. "Summarise the customer's situation: {{summary}}").
//  2. ctx.userInput — the user's latest message.
//
// Outcome is always "default". AI failures, empty replies, or AI being
// disabled all advance via the default edge and log a warning — the
// graph author can route to a fallback message there.
func (a *App) execChatAIResponse(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	settings, err := a.getChatbotSettingsCached(ctx.account.OrganizationID, ctx.account.Name)
	if err != nil {
		a.Log.Error("ai_response node failed to load chatbot settings",
			"node", node.ID, "session", ctx.session.ID, "error", err)
		return nodeOutcome{outcome: "default"}, nil
	}
	if !settings.AI.Enabled || settings.AI.Provider == "" || settings.AI.APIKey == "" {
		a.Log.Warn("ai_response node hit but AI not configured",
			"node", node.ID, "session", ctx.session.ID,
			"ai_enabled", settings.AI.Enabled, "has_provider", settings.AI.Provider != "")
		return nodeOutcome{outcome: "default"}, nil
	}

	userMessage := ctx.userInput
	if tmpl := stringFromConfig(node.Config, "prompt_template", "prompt"); tmpl != "" {
		if ctx.session.SessionData == nil {
			ctx.session.SessionData = models.JSONB{}
		}
		userMessage = processTemplate(tmpl, ctx.session.SessionData)
	}

	answer, err := a.generateAIResponse(settings, ctx.session, userMessage)
	if err != nil {
		a.Log.Error("ai_response node generateAIResponse failed",
			"node", node.ID, "session", ctx.session.ID, "error", err)
		return nodeOutcome{outcome: "default"}, nil
	}
	if answer == "" {
		a.Log.Warn("ai_response node got empty answer from provider",
			"node", node.ID, "session", ctx.session.ID)
		return nodeOutcome{outcome: "default"}, nil
	}

	if err := a.sendAndSaveTextMessage(ctx.account, ctx.contact, answer); err != nil {
		return nodeOutcome{}, fmt.Errorf("send ai response: %w", err)
	}
	a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, answer, node.ID)
	return nodeOutcome{outcome: "default"}, nil
}

// execChatTransfer hands the session off to a human team or the general
// queue. Optionally sends a body message first (e.g. "Connecting you to
// an agent…"), then creates the transfer row via the same helpers the
// legacy flow uses (createTransferToTeam / createTransferToQueue, source
// = TransferSourceFlow), and marks the session completed.
//
// Terminal: returns yield=true so the runner stops and persists. No
// outgoing edges are expected on this node.
//
// Config:
//
//	{
//	  "body":    "Connecting you to a human…",  // optional
//	  "team_id": "<uuid>",                       // empty or "_general" = queue
//	  "notes":   "Last seen {{last_query}}"      // optional; templated
//	}
func (a *App) execChatTransfer(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	if body := stringFromConfig(node.Config, "body", "message", "text"); body != "" {
		message := processTemplate(body, ctx.session.SessionData)
		if err := a.sendAndSaveTextMessage(ctx.account, ctx.contact, message); err != nil {
			a.Log.Error("transfer node failed to send body",
				"node", node.ID, "session", ctx.session.ID, "error", err)
		} else {
			a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, message, node.ID)
		}
	}

	notes := ""
	if rawNotes := stringFromConfig(node.Config, "notes"); rawNotes != "" {
		notes = processTemplate(rawNotes, ctx.session.SessionData)
	}

	teamIDStr := stringFromConfig(node.Config, "team_id")
	if teamIDStr != "" && teamIDStr != "_general" {
		if parsed, err := uuid.Parse(teamIDStr); err == nil {
			a.createTransferToTeam(ctx.account, ctx.contact, parsed, notes, models.TransferSourceFlow)
		} else {
			a.Log.Warn("transfer node has invalid team_id, falling back to queue",
				"node", node.ID, "team_id", teamIDStr, "error", err)
			a.createTransferToQueue(ctx.account, ctx.contact, models.TransferSourceFlow)
		}
	} else {
		a.createTransferToQueue(ctx.account, ctx.contact, models.TransferSourceFlow)
	}

	ctx.session.Status = models.SessionStatusCompleted
	return nodeOutcome{yield: true}, nil
}

// execChatWebhook fires a best-effort HTTP request. Unlike api_call, the
// response is discarded — success, non-2xx, and network errors all
// advance via the "default" edge. Use api_call when the flow needs to
// branch on the response or capture data.
//
// Non-blocking; the call is synchronous to keep test semantics simple
// but the flow does not depend on the outcome.
//
// Config (same shape as api_call minus response_mapping):
//
//	{
//	  "url":     "https://example.com/hook?phone={{phone_number}}",
//	  "method":  "POST",
//	  "headers": { "Authorization": "Bearer …" },
//	  "body":    "{\"event\":\"flow_completed\"}"
//	}
func (a *App) execChatWebhook(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	if ctx.session.SessionData == nil {
		ctx.session.SessionData = models.JSONB{}
	}
	sessionData := ctx.session.SessionData
	sessionData["phone_number"] = ctx.session.PhoneNumber

	replaceVar := func(s string) string { return processTemplate(s, sessionData) }
	_, statusCode, err := a.executeConfiguredAPI(models.JSONB(node.Config), replaceVar)
	switch {
	case err != nil:
		a.Log.Warn("webhook node request errored (continuing)",
			"node", node.ID, "session", ctx.session.ID, "error", err)
	case statusCode < 200 || statusCode >= 300:
		a.Log.Warn("webhook node returned non-2xx (continuing)",
			"node", node.ID, "session", ctx.session.ID, "status", statusCode)
	}
	return nodeOutcome{outcome: "default"}, nil
}

// execChatGotoFlow jumps execution to another flow within the same
// organization + WhatsApp account. SessionData (variables, path) is
// carried forward — this is the whole point of sub-flows.
//
// Mirrors IVR's executeGotoFlow (internal/calling/ivr.go:495). Not a
// "call" — there's no return stack — so once the target flow ends, the
// session ends. The runner detects the CurrentFlowID change and reloads
// the graph + entry node within the same webhook run.
//
// Config:
//
//	{ "flow_id": "<uuid>" }
//
// Misconfiguration (missing/invalid id, target disabled, cross-account)
// is logged and terminates the source flow gracefully rather than
// erroring the inbound webhook.
func (a *App) execChatGotoFlow(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	targetIDStr := stringFromConfig(node.Config, "flow_id")
	if targetIDStr == "" {
		a.Log.Warn("goto_flow node missing flow_id",
			"node", node.ID, "session", ctx.session.ID)
		return nodeOutcome{}, nil
	}
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		a.Log.Warn("goto_flow node has invalid flow_id",
			"node", node.ID, "flow_id", targetIDStr, "error", err)
		return nodeOutcome{}, nil
	}

	target, err := a.getChatbotFlowByIDCached(ctx.account.OrganizationID, targetID)
	if err != nil || target == nil {
		a.Log.Warn("goto_flow target not found",
			"node", node.ID, "flow_id", targetID, "error", err)
		return nodeOutcome{}, nil
	}
	if !target.IsEnabled {
		a.Log.Warn("goto_flow target is disabled",
			"node", node.ID, "flow_id", targetID)
		return nodeOutcome{}, nil
	}
	if target.WhatsAppAccount != ctx.session.WhatsAppAccount {
		a.Log.Warn("goto_flow target belongs to a different WA account; refusing",
			"node", node.ID, "target_account", target.WhatsAppAccount,
			"session_account", ctx.session.WhatsAppAccount)
		return nodeOutcome{}, nil
	}
	if target.Graph == nil {
		a.Log.Warn("goto_flow target has no v2 graph",
			"node", node.ID, "flow_id", targetID)
		return nodeOutcome{}, nil
	}

	// Record the jump on the path for audit visibility.
	if ctx.session.SessionData == nil {
		ctx.session.SessionData = models.JSONB{}
	}
	path, _ := ctx.session.SessionData["__path__"].([]any)
	path = append(path, map[string]any{
		"action":  "goto_flow",
		"flow":    target.Name,
		"flow_id": target.ID.String(),
	})
	ctx.session.SessionData["__path__"] = path

	// Signal the switch — the runner detects CurrentFlowID change and
	// reloads + resets CurrentStep to the new entry node.
	ctx.session.CurrentFlowID = &target.ID

	return nodeOutcome{outcome: "goto"}, nil
}

// execChatWhatsAppFlow sends an interactive WhatsApp Flow form on first
// entry (yields to wait for the user's submission). On a later inbound
// that carries a parsed flow_response_data, merges those fields into
// SessionData and advances via the "default" edge.
//
// Config:
//
//	{
//	  "flow_id": "<meta_flow_id>",   // required
//	  "header":  "Header text",       // optional, templated
//	  "body":    "Body text",         // optional, templated
//	  "cta":     "Open form"           // optional CTA label
//	}
//
// Submitted fields land in SessionData keyed by their form field names.
// A misconfigured node logs + advances via default so the conversation
// doesn't dead-end.
func (a *App) execChatWhatsAppFlow(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	if !ctx.consumed && len(ctx.flowResponseData) > 0 {
		ctx.consumed = true
		if ctx.session.SessionData == nil {
			ctx.session.SessionData = models.JSONB{}
		}
		maps.Copy(ctx.session.SessionData, ctx.flowResponseData)
		return nodeOutcome{outcome: "default"}, nil
	}

	flowID := stringFromConfig(node.Config, "flow_id")
	if flowID == "" {
		a.Log.Warn("whatsapp_flow node missing flow_id",
			"node", node.ID, "session", ctx.session.ID)
		return nodeOutcome{outcome: "default"}, nil
	}

	body := processTemplate(stringFromConfig(node.Config, "body", "message", "text"), ctx.session.SessionData)
	header := processTemplate(stringFromConfig(node.Config, "header"), ctx.session.SessionData)
	cta := processTemplate(stringFromConfig(node.Config, "cta"), ctx.session.SessionData)

	// Look up the first screen — same pattern as the legacy executor so
	// existing WhatsAppFlow rows continue to work.
	firstScreen := ""
	var waFlow models.WhatsAppFlow
	if err := a.DB.Where("meta_flow_id = ?", flowID).First(&waFlow).Error; err == nil {
		if len(waFlow.Screens) > 0 {
			if screenMap, ok := waFlow.Screens[0].(map[string]any); ok {
				if id, ok := screenMap["id"].(string); ok {
					firstScreen = id
				}
			}
		}
		if firstScreen == "" && waFlow.FlowJSON != nil {
			if screens, ok := waFlow.FlowJSON["screens"].([]any); ok && len(screens) > 0 {
				if screenMap, ok := screens[0].(map[string]any); ok {
					if id, ok := screenMap["id"].(string); ok {
						firstScreen = id
					}
				}
			}
		}
	}

	flowToken := fmt.Sprintf("chatbot_%s_%s_%d", ctx.session.ID.String(), node.ID, time.Now().UnixNano())
	if err := a.sendAndSaveFlowMessage(ctx.account, ctx.contact, flowID, header, body, cta, flowToken, firstScreen); err != nil {
		return nodeOutcome{}, fmt.Errorf("send whatsapp_flow: %w", err)
	}
	a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, body, node.ID)
	return nodeOutcome{yield: true}, nil
}

// execChatEnd optionally sends a final message and returns an empty
// outcome. The runner sees no matching edge and marks the session
// completed.
// Config: { "message": "..." } (optional)
func (a *App) execChatEnd(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	if msg := stringFromConfig(node.Config, "message"); msg != "" {
		msg = processTemplate(msg, ctx.session.SessionData)
		if err := a.sendAndSaveTextMessage(ctx.account, ctx.contact, msg); err != nil {
			return nodeOutcome{}, fmt.Errorf("send end message: %w", err)
		}
		a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, msg, node.ID)
	}
	return nodeOutcome{}, nil
}

// persistChatSession writes the running session state back to the DB.
// Variables, current node, and the __path__ trail all live in SessionData
// + dedicated columns. Called after every yield and on the completion path.
func (a *App) persistChatSession(s *models.ChatbotSession) error {
	s.LastActivityAt = time.Now()
	if s.Status == models.SessionStatusCompleted && s.CompletedAt == nil {
		now := time.Now()
		s.CompletedAt = &now
	}
	if err := a.DB.Save(s).Error; err != nil {
		a.Log.Error("persist chat session", "session", s.ID, "error", err)
		return err
	}
	return nil
}

// appendChatPath records the executed node + outcome in SessionData["__path__"].
// The shape mirrors IVRContext.Path so frontends and audit tooling can
// render either domain's trail with the same code.
func appendChatPath(s *models.ChatbotSession, node *ChatNode, outcome string) {
	if s.SessionData == nil {
		s.SessionData = models.JSONB{}
	}
	entry := map[string]any{
		"node":    node.ID,
		"type":    string(node.Type),
		"label":   node.Label,
		"outcome": outcome,
	}
	path, _ := s.SessionData["__path__"].([]any)
	path = append(path, entry)
	s.SessionData["__path__"] = path
}

// stringFromConfig returns the first non-empty string at any of the given keys.
func stringFromConfig(cfg map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := cfg[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// intFromConfig returns an int value from config at the given key, falling
// back to def. JSON numbers decode as float64 in map[string]any, so accept
// both float64 and int.
func intFromConfig(cfg map[string]any, key string, def int) int {
	switch v := cfg[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

// buttonsFromConfig normalizes node.Config["buttons"] into the shape the
// existing sendAndSaveInteractiveButtons helper expects.
// Accepts: [{"id": "...", "title": "...", "type": "..."(optional)}, ...]
func buttonsFromConfig(cfg map[string]any) []map[string]any {
	raw, ok := cfg["buttons"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
