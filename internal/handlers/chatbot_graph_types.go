package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/shridarpatil/whatomate/internal/flowgraph"
	"github.com/shridarpatil/whatomate/internal/models"
)

// ChatNodeType identifies the kind of node in a chatbot flow graph.
// Constants live in this package so the chat domain owns its vocabulary
// without coupling to internal/calling.
type ChatNodeType string

const (
	ChatNodeStart        ChatNodeType = "start"
	ChatNodeMessage      ChatNodeType = "message"
	ChatNodeButtons      ChatNodeType = "buttons"
	ChatNodePrompt       ChatNodeType = "prompt"
	ChatNodeAPICall      ChatNodeType = "api_call"
	ChatNodeCondition    ChatNodeType = "condition"
	ChatNodeTiming       ChatNodeType = "timing"
	ChatNodeSetVariable  ChatNodeType = "set_variable"
	ChatNodeAIResponse   ChatNodeType = "ai_response"
	ChatNodeTransfer     ChatNodeType = "transfer"
	ChatNodeWebhook      ChatNodeType = "webhook"
	ChatNodeGotoFlow     ChatNodeType = "goto_flow"
	ChatNodeWhatsAppFlow ChatNodeType = "whatsapp_flow"
	ChatNodeEnd          ChatNodeType = "end"
)

// ChatNode, ChatEdge and ChatGraph are the chatbot domain's views of the shared
// flow-graph types, specialized to ChatNodeType. Edge conditions for the chat
// engine include "default", "button:<id>", "input:<val>", "http:2xx",
// "http:non2xx", "validation_failed", "max_retries", "in_hours", "out_of_hours".
// Traversal (BuildMaps/Node/ResolveEdge) lives in internal/flowgraph.
type (
	ChatNode  = flowgraph.Node[ChatNodeType]
	ChatEdge  = flowgraph.Edge
	ChatGraph = flowgraph.Graph[ChatNodeType]
)

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
	g.BuildMaps()
	if g.Node(g.EntryNode) == nil {
		return nil, fmt.Errorf("entry_node %q not found in nodes", g.EntryNode)
	}
	return &g, nil
}
