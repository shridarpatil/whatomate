package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/shridarpatil/whatomate/internal/models"
)

// ChatNodeType identifies the kind of node in a chatbot flow graph.
// Constants live in this package so the chat domain owns its vocabulary
// without coupling to internal/calling.
type ChatNodeType string

const (
	ChatNodeStart       ChatNodeType = "start"
	ChatNodeMessage     ChatNodeType = "message"
	ChatNodeButtons     ChatNodeType = "buttons"
	ChatNodePrompt      ChatNodeType = "prompt"
	ChatNodeAPICall     ChatNodeType = "api_call"
	ChatNodeCondition   ChatNodeType = "condition"
	ChatNodeTiming      ChatNodeType = "timing"
	ChatNodeSetVariable ChatNodeType = "set_variable"
	ChatNodeAIResponse  ChatNodeType = "ai_response"
	ChatNodeTransfer    ChatNodeType = "transfer"
	ChatNodeWebhook     ChatNodeType = "webhook"
	ChatNodeGotoFlow     ChatNodeType = "goto_flow"
	ChatNodeWhatsAppFlow ChatNodeType = "whatsapp_flow"
	ChatNodeEnd          ChatNodeType = "end"
)

// ChatNodePosition is the visual editor placement.
type ChatNodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ChatNode is a single node in a v2 chatbot flow graph.
type ChatNode struct {
	ID       string           `json:"id"`
	Type     ChatNodeType     `json:"type"`
	Label    string           `json:"label"`
	Position ChatNodePosition `json:"position"`
	Config   map[string]any   `json:"config"`
}

// ChatEdge connects two nodes. Condition examples: "default",
// "button:<id>", "input:<val>", "http:2xx", "http:non2xx",
// "validation_failed", "max_retries", "in_hours", "out_of_hours".
type ChatEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition"`
}

// ChatGraph is the top-level structure stored in ChatbotFlow.Graph.
type ChatGraph struct {
	Version   int        `json:"version"`
	Nodes     []ChatNode `json:"nodes"`
	Edges     []ChatEdge `json:"edges"`
	EntryNode string     `json:"entry_node"`

	nodeMap map[string]*ChatNode
	edgeMap map[string][]ChatEdge
}

// parseChatGraph decodes a raw JSONB blob into a ChatGraph and builds the
// runtime lookup maps. Returns (nil, nil) when raw is nil — caller treats
// that as "no graph, use legacy Steps".
func parseChatGraph(raw models.JSONB) (*ChatGraph, error) {
	if raw == nil {
		return nil, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal graph: %w", err)
	}
	var g ChatGraph
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("unmarshal graph: %w", err)
	}
	if g.Version != 2 {
		return nil, fmt.Errorf("unsupported graph version %d (want 2)", g.Version)
	}
	if g.EntryNode == "" {
		return nil, fmt.Errorf("graph missing entry_node")
	}
	g.buildMaps()
	if g.nodeMap[g.EntryNode] == nil {
		return nil, fmt.Errorf("entry_node %q not found in nodes", g.EntryNode)
	}
	return &g, nil
}

func (g *ChatGraph) buildMaps() {
	g.nodeMap = make(map[string]*ChatNode, len(g.Nodes))
	g.edgeMap = make(map[string][]ChatEdge, len(g.Edges))
	for i := range g.Nodes {
		g.nodeMap[g.Nodes[i].ID] = &g.Nodes[i]
	}
	for _, e := range g.Edges {
		g.edgeMap[e.From] = append(g.edgeMap[e.From], e)
	}
}

func (g *ChatGraph) getNode(id string) *ChatNode {
	return g.nodeMap[id]
}

// resolveEdge returns the target node ID for a given outcome from fromID.
// Exact condition match wins; otherwise falls back to a "default" edge.
// Returns "" when no edge matches (terminal).
func (g *ChatGraph) resolveEdge(fromID, outcome string) string {
	var def string
	for _, e := range g.edgeMap[fromID] {
		if e.Condition == outcome {
			return e.To
		}
		if e.Condition == "default" {
			def = e.To
		}
	}
	return def
}
