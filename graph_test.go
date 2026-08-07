package specsync

import (
	"testing"
)

func TestNewGraph_Empty(t *testing.T) {
	g := NewGraph()
	if g.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
	if got := g.Nodes(); got != nil {
		t.Errorf("expected nil nodes slice, got %v", got)
	}
	if got := g.Edges(); got != nil {
		t.Errorf("expected nil edges slice, got %v", got)
	}
}

func TestGraph_AddNode(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{Kind: GraphKindSpec, ID: "spec:foo", Label: "foo"})
	g.AddNode(Node{Kind: GraphKindIssue, ID: "issue:42", Label: "#42"})
	if g.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NodeCount())
	}
	n, ok := g.NodeByID("spec:foo")
	if !ok {
		t.Fatal("node spec:foo not found")
	}
	if n.Label != "foo" {
		t.Errorf("expected label 'foo', got %q", n.Label)
	}
}

func TestGraph_AddNode_DuplicateIgnored(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{Kind: GraphKindSpec, ID: "spec:foo", Label: "first"})
	g.AddNode(Node{Kind: GraphKindSpec, ID: "spec:foo", Label: "second"})
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node (duplicate ignored), got %d", g.NodeCount())
	}
	n, _ := g.NodeByID("spec:foo")
	if n.Label != "first" {
		t.Errorf("expected label 'first', got %q", n.Label)
	}
}

func TestGraph_AddEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{Kind: GraphKindSpec, ID: "spec:foo", Label: "foo"})
	g.AddNode(Node{Kind: GraphKindIssue, ID: "issue:42", Label: "#42"})
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "spec:foo", To: "issue:42", Source: "refs.json"})
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}
}

func TestGraph_AddEdge_DuplicateIgnored(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "a", To: "b", Source: "x"})
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "a", To: "b", Source: "y"})
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge (duplicate ignored), got %d", g.EdgeCount())
	}
}

func TestGraph_AddEdge_DifferentKindNotDuplicate(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "a", To: "b", Source: "x"})
	g.AddEdge(Edge{Kind: EdgeSpecSpec, From: "a", To: "b", Source: "y"})
	if g.EdgeCount() != 2 {
		t.Errorf("expected 2 edges (different kinds), got %d", g.EdgeCount())
	}
}

func TestGraph_Nodes_DeterministicOrder(t *testing.T) {
	g := NewGraph()
	// Add in random order.
	g.AddNode(Node{Kind: GraphKindCommit, ID: "commit:zzz", Label: "zzz"})
	g.AddNode(Node{Kind: GraphKindSpec, ID: "spec:alpha", Label: "alpha"})
	g.AddNode(Node{Kind: GraphKindIssue, ID: "issue:10", Label: "#10"})
	g.AddNode(Node{Kind: GraphKindIssue, ID: "issue:5", Label: "#5"})
	g.AddNode(Node{Kind: GraphKindSpec, ID: "spec:beta", Label: "beta"})
	g.AddNode(Node{Kind: GraphKindRelease, ID: "release:v1.0", Label: "v1.0"})

	nodes := g.Nodes()
	if len(nodes) != 6 {
		t.Fatalf("expected 6 nodes, got %d", len(nodes))
	}

	// Verify deterministic ordering: sorted by full ID lexicographically.
	expectedIDs := []string{
		"commit:zzz",
		"issue:10", // '1' < '5' in lexicographic order
		"issue:5",
		"release:v1.0",
		"spec:alpha",
		"spec:beta",
	}
	for i, wantID := range expectedIDs {
		if nodes[i].ID != wantID {
			t.Errorf("node[%d]: expected ID %q, got %q", i, wantID, nodes[i].ID)
		}
	}
}

func TestGraph_Edges_DeterministicOrder(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{Kind: EdgePRCommit, From: "pr:1", To: "commit:abc"})
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "spec:foo", To: "issue:42"})
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "spec:bar", To: "issue:43"})
	g.AddEdge(Edge{Kind: EdgeCommitRel, From: "commit:abc", To: "release:v1"})

	edges := g.Edges()
	if len(edges) != 4 {
		t.Fatalf("expected 4 edges, got %d", len(edges))
	}

	// Verify ordering: commit-release < pr-commit < spec-issue (alphabetical by kind).
	// Within same kind, sorted by from, then to.
	expected := []struct {
		kind EdgeKind
		from string
		to   string
	}{
		{EdgeCommitRel, "commit:abc", "release:v1"},
		{EdgePRCommit, "pr:1", "commit:abc"},
		{EdgeSpecIssue, "spec:bar", "issue:43"},
		{EdgeSpecIssue, "spec:foo", "issue:42"},
	}
	for i, want := range expected {
		if edges[i].Kind != want.kind || edges[i].From != want.from || edges[i].To != want.to {
			t.Errorf("edge[%d]: expected %s %s->%s, got %s %s->%s",
				i, want.kind, want.from, want.to, edges[i].Kind, edges[i].From, edges[i].To)
		}
	}
}

func TestGraph_EdgesByKind(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "spec:a", To: "issue:1"})
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "spec:b", To: "issue:2"})
	g.AddEdge(Edge{Kind: EdgePRCommit, From: "pr:1", To: "commit:abc"})

	issueEdges := g.EdgesByKind(EdgeSpecIssue)
	if len(issueEdges) != 2 {
		t.Errorf("expected 2 spec-issue edges, got %d", len(issueEdges))
	}
	if issueEdges[0].From != "spec:a" || issueEdges[1].From != "spec:b" {
		t.Errorf("expected sorted spec-issue edges, got %v", issueEdges)
	}

	prEdges := g.EdgesByKind(EdgePRCommit)
	if len(prEdges) != 1 {
		t.Errorf("expected 1 pr-commit edge, got %d", len(prEdges))
	}

	emptyEdges := g.EdgesByKind(EdgeCommitRel)
	if emptyEdges != nil {
		t.Errorf("expected nil slice for non-existent kind, got %v", emptyEdges)
	}
}

func TestGraph_NodeByID_NotFound(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{Kind: GraphKindSpec, ID: "spec:foo", Label: "foo"})
	_, ok := g.NodeByID("spec:bar")
	if ok {
		t.Error("expected not found for non-existent node")
	}
}

func TestGraph_NodeFields(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:       GraphKindSpec,
		ID:         "spec:my-change",
		Label:      "My Change",
		Slug:       "my-change",
		Stage:      StageActive,
		CommitDate: "2026-08-07",
	})
	n, ok := g.NodeByID("spec:my-change")
	if !ok {
		t.Fatal("node not found")
	}
	if n.Slug != "my-change" {
		t.Errorf("expected slug 'my-change', got %q", n.Slug)
	}
	if n.Stage != StageActive {
		t.Errorf("expected stage 'active', got %q", n.Stage)
	}
	if n.CommitDate != "2026-08-07" {
		t.Errorf("expected commit date '2026-08-07', got %q", n.CommitDate)
	}
}

func TestGraph_IssueNode(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       "issue:42",
		Label:    "#42: Fix login",
		IssueNum: "42",
	})
	n, _ := g.NodeByID("issue:42")
	if n.IssueNum != "42" {
		t.Errorf("expected IssueNum '42', got %q", n.IssueNum)
	}
}

func TestGraph_PRNode(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:     GraphKindPR,
		ID:       "pr:100",
		Label:    "#100: Merge feature",
		IssueNum: "100",
	})
	n, _ := g.NodeByID("pr:100")
	if n.IssueNum != "100" {
		t.Errorf("expected IssueNum '100', got %q", n.IssueNum)
	}
}

func TestGraph_CommitNode(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:       GraphKindCommit,
		ID:         "commit:abc123",
		Label:      "abc123 feat: add graph",
		Hash:       "abc123def456",
		CommitDate: "2026-08-07T12:00:00Z",
	})
	n, _ := g.NodeByID("commit:abc123")
	if n.Hash != "abc123def456" {
		t.Errorf("expected Hash 'abc123def456', got %q", n.Hash)
	}
}

func TestGraph_ReleaseNode(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:    GraphKindRelease,
		ID:      "release:v1.2.0",
		Label:   "v1.2.0",
		TagName: "v1.2.0",
	})
	n, _ := g.NodeByID("release:v1.2.0")
	if n.TagName != "v1.2.0" {
		t.Errorf("expected TagName 'v1.2.0', got %q", n.TagName)
	}
}

func TestGraphKind_String(t *testing.T) {
	tests := []struct {
		kind GraphKind
		want string
	}{
		{GraphKindSpec, "spec"},
		{GraphKindIssue, "issue"},
		{GraphKindCommit, "commit"},
		{GraphKindPR, "pr"},
		{GraphKindRelease, "release"},
	}
	for _, tc := range tests {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("GraphKind(%q).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestEdgeKind_String(t *testing.T) {
	tests := []struct {
		kind EdgeKind
		want string
	}{
		{EdgeSpecSpec, "spec-spec"},
		{EdgeSpecIssue, "spec-issue"},
		{EdgeIssuePR, "issue-pr"},
		{EdgePRCommit, "pr-commit"},
		{EdgeCommitRel, "commit-release"},
		{EdgeWorkFiles, "work-files"},
	}
	for _, tc := range tests {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("EdgeKind(%q).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestGraph_DeterministicAcrossRuns(t *testing.T) {
	// Add nodes in different orders and verify the output is the same.
	orders := [][]Node{
		{
			{Kind: GraphKindRelease, ID: "release:v1"},
			{Kind: GraphKindSpec, ID: "spec:alpha"},
			{Kind: GraphKindCommit, ID: "commit:zzz"},
			{Kind: GraphKindIssue, ID: "issue:1"},
		},
		{
			{Kind: GraphKindCommit, ID: "commit:zzz"},
			{Kind: GraphKindIssue, ID: "issue:1"},
			{Kind: GraphKindSpec, ID: "spec:alpha"},
			{Kind: GraphKindRelease, ID: "release:v1"},
		},
		{
			{Kind: GraphKindSpec, ID: "spec:alpha"},
			{Kind: GraphKindRelease, ID: "release:v1"},
			{Kind: GraphKindCommit, ID: "commit:zzz"},
			{Kind: GraphKindIssue, ID: "issue:1"},
		},
	}

	var first []Node
	for i, nodes := range orders {
		g := NewGraph()
		for _, n := range nodes {
			g.AddNode(n)
		}
		got := g.Nodes()
		if i == 0 {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Errorf("order %d: expected %d nodes, got %d", i, len(first), len(got))
			continue
		}
		for j := range got {
			if got[j].ID != first[j].ID {
				t.Errorf("order %d, node[%d]: expected %s, got %s", i, j, first[j].ID, got[j].ID)
			}
		}
	}
}

func TestGraph_EdgesDeterministicAcrossRuns(t *testing.T) {
	edges := [][]Edge{
		{
			{Kind: EdgePRCommit, From: "pr:1", To: "commit:a"},
			{Kind: EdgeSpecIssue, From: "spec:x", To: "issue:1"},
		},
		{
			{Kind: EdgeSpecIssue, From: "spec:x", To: "issue:1"},
			{Kind: EdgePRCommit, From: "pr:1", To: "commit:a"},
		},
	}

	var first []Edge
	for i, edgeList := range edges {
		g := NewGraph()
		for _, e := range edgeList {
			g.AddEdge(e)
		}
		got := g.Edges()
		if i == 0 {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Errorf("order %d: expected %d edges, got %d", i, len(first), len(got))
			continue
		}
		for j := range got {
			if got[j].Kind != first[j].Kind || got[j].From != first[j].From || got[j].To != first[j].To {
				t.Errorf("order %d, edge[%d]: expected %s %s->%s, got %s %s->%s",
					i, j, first[j].Kind, first[j].From, first[j].To,
					got[j].Kind, got[j].From, got[j].To)
			}
		}
	}
}

func TestGraph_AddEdge_DifferentFromTo(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "a", To: "b"})
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "b", To: "a"})
	if g.EdgeCount() != 2 {
		t.Errorf("expected 2 edges (reversed from/to), got %d", g.EdgeCount())
	}
}

func TestGraph_Edge_SourcePreserved(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{Kind: EdgeSpecIssue, From: "spec:foo", To: "issue:42", Source: "refs.json"})
	edges := g.Edges()
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Source != "refs.json" {
		t.Errorf("expected Source 'refs.json', got %q", edges[0].Source)
	}
}

func TestGraph_NodeByID_PartialMatch(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{Kind: GraphKindSpec, ID: "spec:foo", Label: "foo"})
	// Partial ID match should not return the node.
	_, ok := g.NodeByID("spec:fo")
	if ok {
		t.Error("expected not found for partial ID match")
	}
}
