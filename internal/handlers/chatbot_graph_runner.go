package handlers

import (
	"errors"
	"fmt"
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
