// Package flowgraph holds the shared v2 flow-graph data model and traversal
// used by both the chatbot engine (internal/handlers) and the IVR engine
// (internal/calling). Each subsystem supplies its own node-type enum via the
// generic type parameter and keeps its own executor; only the graph shape and
// edge resolution are shared here.
package flowgraph

// Position is the visual-editor placement of a node.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Edge connects two nodes. Condition carries the routing outcome, e.g.
// "default", "button:<id>", "digit:N", "http:2xx", "in_hours".
type Edge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition"`
}

// Node is a single node in a v2 flow graph. T is the domain-specific node-type
// enum (a ~string newtype) so each subsystem owns its vocabulary.
type Node[T ~string] struct {
	ID       string         `json:"id"`
	Type     T              `json:"type"`
	Label    string         `json:"label"`
	Position Position       `json:"position"`
	Config   map[string]any `json:"config"`
}

// Graph is the top-level v2 flow-graph structure. After unmarshaling a Graph
// from JSON, call BuildMaps to populate the runtime lookup indexes.
type Graph[T ~string] struct {
	Version   int       `json:"version"`
	Nodes     []Node[T] `json:"nodes"`
	Edges     []Edge    `json:"edges"`
	EntryNode string    `json:"entry_node"`

	nodeMap map[string]*Node[T] // id → node
	edgeMap map[string][]Edge   // from-node-id → outgoing edges
}

// BuildMaps populates the runtime lookup indexes for fast traversal.
func (g *Graph[T]) BuildMaps() {
	g.nodeMap = make(map[string]*Node[T], len(g.Nodes))
	g.edgeMap = make(map[string][]Edge, len(g.Edges))
	for i := range g.Nodes {
		g.nodeMap[g.Nodes[i].ID] = &g.Nodes[i]
	}
	for _, e := range g.Edges {
		g.edgeMap[e.From] = append(g.edgeMap[e.From], e)
	}
}

// Node returns the node with the given ID, or nil.
func (g *Graph[T]) Node(id string) *Node[T] {
	return g.nodeMap[id]
}

// OutgoingEdges returns the edges leaving the given node.
func (g *Graph[T]) OutgoingEdges(fromID string) []Edge {
	return g.edgeMap[fromID]
}

// ResolveEdge returns the target node ID for a given outcome leaving fromID.
// An exact condition match wins; otherwise it falls back to a "default" edge.
// Returns "" when no edge matches (terminal).
func (g *Graph[T]) ResolveEdge(fromID, outcome string) string {
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
