package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"time"

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
	account   *models.WhatsAppAccount
	contact   *models.Contact
	session   *models.ChatbotSession
	userInput string
	buttonID  string
	consumed  bool
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
) error {
	graph, err := parseChatGraph(flow.Graph)
	if err != nil {
		return fmt.Errorf("parse chat graph: %w", err)
	}
	if graph == nil {
		return errors.New("flow has no v2 graph; legacy executor should have run")
	}

	ctx := &chatNodeCtx{
		account:   account,
		contact:   contact,
		session:   session,
		userInput: userInput,
		buttonID:  buttonID,
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

		res, err := a.executeChatNode(node, ctx)
		if err != nil {
			a.persistChatSession(session)
			return err
		}

		appendChatPath(session, node, res.outcome)

		if res.yield {
			// Stay at this node; next inbound resumes here.
			return a.persistChatSession(session)
		}

		next := graph.resolveEdge(node.ID, res.outcome)
		if next == "" {
			// No matching edge → terminal.
			session.Status = models.SessionStatusCompleted
			return a.persistChatSession(session)
		}
		session.CurrentStep = next
	}

	a.persistChatSession(session)
	return errChatGraphRunaway
}

// executeChatNode dispatches by node type. Phase 1 implements only
// message, buttons, and end. Other types return an error until their
// PR lands.
func (a *App) executeChatNode(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	switch node.Type {
	case ChatNodeMessage:
		return a.execChatMessage(node, ctx)
	case ChatNodeButtons:
		return a.execChatButtons(node, ctx)
	case ChatNodePrompt:
		return a.execChatPrompt(node, ctx)
	case ChatNodeAPICall:
		return a.execChatAPICall(node, ctx)
	case ChatNodeEnd:
		return a.execChatEnd(node, ctx)
	default:
		return nodeOutcome{outcome: "", yield: true},
			fmt.Errorf("chat node type %q not implemented in this phase", node.Type)
	}
}

// execChatMessage sends a text message and falls through.
// Config: { "message": "..." } or "text" for compatibility.
func (a *App) execChatMessage(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	text := stringFromConfig(node.Config, "message", "text")
	if text == "" {
		return nodeOutcome{outcome: "default"}, nil
	}
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
	buttons := buttonsFromConfig(node.Config)
	if len(buttons) == 0 {
		return nodeOutcome{}, fmt.Errorf("buttons node %q has no buttons configured", node.ID)
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
		if err := a.sendAndSaveTextMessage(ctx.account, ctx.contact, body); err != nil {
			return nodeOutcome{}, fmt.Errorf("send prompt: %w", err)
		}
		a.logSessionMessage(ctx.session.ID, models.DirectionOutgoing, body, node.ID)
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
//	  "response_mapping": { "customer_id": "data.id", "status": "data.status" }
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

	return nodeOutcome{outcome: "http:2xx"}, nil
}

// execChatEnd optionally sends a final message and returns an empty
// outcome. The runner sees no matching edge and marks the session
// completed.
// Config: { "message": "..." } (optional)
func (a *App) execChatEnd(node *ChatNode, ctx *chatNodeCtx) (nodeOutcome, error) {
	if msg := stringFromConfig(node.Config, "message"); msg != "" {
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
