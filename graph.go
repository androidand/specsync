package specsync

// GraphKind is the type of a work-graph node.
type GraphKind string

const (
	GraphKindSpec    GraphKind = "spec"    // OpenSpec change folder
	GraphKindIssue   GraphKind = "issue"   // tracker issue
	GraphKindCommit  GraphKind = "commit"  // git commit
	GraphKindPR      GraphKind = "pr"      // pull request
	GraphKindRelease GraphKind = "release" // release / tag
)

// String returns the lowercase kind name.
func (k GraphKind) String() string { return string(k) }

// Node is one artifact in the work graph. ID is unique across kinds (it is
// kind-prefixed); Label is human-facing.
type Node struct {
	Kind  GraphKind
	ID    string
	Label string

	// Optional fields for specific node kinds.
	Slug       string    // change slug (GraphKindSpec)
	IssueNum   string    // issue/PR number (GraphKindIssue, GraphKindPR)
	Hash       string    // commit hash (GraphKindCommit)
	TagName    string    // release tag name (GraphKindRelease)
	Stage      Stage     // workflow stage for spec nodes
	CommitDate string    // commit date for GraphKindCommit
}

// EdgeKind is the type of a work-graph edge.
type EdgeKind string

const (
	EdgeSpecSpec   EdgeKind = "spec-spec"    // links.md (asserted)
	EdgeSpecIssue  EdgeKind = "spec-issue"   // body marker / refs.json (asserted)
	EdgeIssuePR    EdgeKind = "issue-pr"     // gh issue/PR linkage
	EdgePRCommit   EdgeKind = "pr-commit"    // PR merge commit
	EdgeCommitRel  EdgeKind = "commit-release" // git tag --contains / gh release
	EdgeWorkFiles  EdgeKind = "work-files"   // changed files of linked PRs/commits
)

// String returns the lowercase kind name.
func (k EdgeKind) String() string { return string(k) }

// Edge is a directed edge between two nodes with the provenance that established it.
type Edge struct {
	Kind   EdgeKind
	From   string // node ID
	To     string // node ID
	Source string // optional: where this edge was asserted (e.g. "links.md", "refs.json")
}

// Graph is an in-memory work graph built on demand from asserted evidence.
// Nodes and edges are indexed by ID for fast lookup; sorted slices provide
// deterministic, stable-order output.
type Graph struct {
	nodes   map[string]Node // keyed by Node.ID
	edges   []Edge
	nodeIDs []string // sorted node IDs for deterministic iteration
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]Node),
		edges: make([]Edge, 0),
	}
}

// AddNode inserts a node. Duplicate IDs are silently ignored (idempotent).
func (g *Graph) AddNode(n Node) {
	if g.nodes == nil {
		g.nodes = make(map[string]Node)
	}
	if _, exists := g.nodes[n.ID]; exists {
		return
	}
	g.nodes[n.ID] = n
}

// AddEdge inserts a directed edge. Duplicate (from, to, kind) tuples are
// silently ignored (idempotent).
func (g *Graph) AddEdge(e Edge) {
	if g.edges == nil {
		g.edges = make([]Edge, 0)
	}
	for _, existing := range g.edges {
		if existing.From == e.From && existing.To == e.To && existing.Kind == e.Kind {
			return
		}
	}
	g.edges = append(g.edges, e)
}

// NodeByID returns the node with the given ID, or (zero value, false) if absent.
func (g *Graph) NodeByID(id string) (Node, bool) {
	n, ok := g.nodes[id]
	return n, ok
}

// Nodes returns all nodes in deterministic order: sorted by kind, then by ID.
func (g *Graph) Nodes() []Node {
	if g.nodeIDs == nil {
		g.nodeIDs = sortedNodeIDs(g.nodes)
	}
	if len(g.nodeIDs) == 0 {
		return nil
	}
	result := make([]Node, 0, len(g.nodeIDs))
	for _, id := range g.nodeIDs {
		result = append(result, g.nodes[id])
	}
	return result
}

// Edges returns all edges in deterministic order: sorted by kind, then from, then to.
func (g *Graph) Edges() []Edge {
	if len(g.edges) == 0 {
		return nil
	}
	edges := make([]Edge, len(g.edges))
	copy(edges, g.edges)
	sortEdges(edges)
	return edges
}

// EdgesByKind returns edges filtered to the given kind, in deterministic order.
func (g *Graph) EdgesByKind(kind EdgeKind) []Edge {
	var out []Edge
	for _, e := range g.edges {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	sortEdges(out)
	return out
}

// EdgeCount returns the number of edges in the graph.
func (g *Graph) EdgeCount() int {
	return len(g.edges)
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int {
	return len(g.nodes)
}

// sortEdges sorts edges in place by kind, from, then to for deterministic output.
func sortEdges(edges []Edge) {
	for i := 1; i < len(edges); i++ {
		key := edges[i]
		j := i - 1
		for j >= 0 && edgeLess(key, edges[j]) {
			edges[j+1] = edges[j]
			j--
		}
		edges[j+1] = key
	}
}

func edgeLess(a, b Edge) bool {
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.From != b.From {
		return a.From < b.From
	}
	return a.To < b.To
}

// sortedNodeIDs returns node IDs sorted by kind, then by ID.
func sortedNodeIDs(nodes map[string]Node) []string {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	// Insertion sort — small N, stable, deterministic.
	for i := 1; i < len(ids); i++ {
		key := ids[i]
		keyKey := nodeSortKey(nodes[key])
		j := i - 1
		for j >= 0 && nodeSortKey(nodes[ids[j]]) > keyKey {
			ids[j+1] = ids[j]
			j--
		}
		ids[j+1] = key
	}
	return ids
}

// nodeSortKey returns a composite sort key for a node: kind + ID.
func nodeSortKey(n Node) string {
	return string(n.Kind) + ":" + n.ID
}

// graphNodeID builds a kind-prefixed ID unique across all node kinds.
func graphNodeID(kind GraphKind, raw string) string {
	return string(kind) + ":" + raw
}
