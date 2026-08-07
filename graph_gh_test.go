package specsync

import (
	"context"
	"fmt"
	"testing"
)

// mockGHRunner is a test double for GHRunner.
type mockGHRunner struct {
	// responses maps a sorted gh command (joined by space) to output.
	responses map[string]string
	// errors maps a sorted gh command to an error.
	errors map[string]error
}

func (m *mockGHRunner) Run(ctx context.Context, args ...string) (string, error) {
	key := joinArgs(args)

	if err, ok := m.errors[key]; ok {
		return "", err
	}
	if out, ok := m.responses[key]; ok {
		return out, nil
	}
	return "", nil
}

func joinArgs(args []string) string {
	s := make([]string, len(args))
	copy(s, args)
	// Sort for deterministic key.
	sortStrings(s)
	return joinStrings(s, " ")
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

func joinStrings(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	r := s[0]
	for i := 1; i < len(s); i++ {
		r += sep + s[i]
	}
	return r
}

func TestGraph_BuildGHEdges_NoIssues(t *testing.T) {
	g := NewGraph()
	// Add only a spec node, no issues.
	g.AddNode(Node{
		Kind:  GraphKindSpec,
		ID:    graphNodeID(GraphKindSpec, "my-change"),
		Label: "My Change",
	})

	runner := &mockGHRunner{responses: make(map[string]string)}
	ctx := context.Background()

	if err := g.BuildGHEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildGHEdges: %v", err)
	}

	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildGHEdges_IssueWithPRs(t *testing.T) {
	g := NewGraph()
	// Add an issue node (ID already prefixed by graphNodeID).
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       graphNodeID(GraphKindIssue, "github:owner/repo:42"),
		Label:    "https://github.com/owner/repo/issues/42",
		IssueNum: "42",
	})

	// Mock gh api repos/owner/repo/issues/42 returning one PR.
	prListOut := `[{"number":100,"title":"Fix login","mergedAt":"2026-08-07T10:00:00Z"}]`
	runner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"api", "repos/owner/repo/issues/42", "--jq", ".pull_requests // [] | .[] | {number: .number, title: .title, mergedAt: .merged_at}"}): prListOut,
		},
	}
	ctx := context.Background()

	if err := g.BuildGHEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildGHEdges: %v", err)
	}

	// Should have: 1 issue + 1 PR = 2 nodes, 1 edge.
	if g.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NodeCount())
		for _, n := range g.Nodes() {
			t.Logf("  node: %s %s", n.Kind, n.ID)
		}
	}
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}

	// Check PR node.
	prEdges := g.EdgesByKind(EdgeIssuePR)
	if len(prEdges) != 1 {
		t.Fatalf("expected 1 issue-pr edge, got %d", len(prEdges))
	}
	if prEdges[0].From != "issue:github:owner/repo:42" {
		t.Errorf("issue-pr edge from = %q, want %q", prEdges[0].From, "issue:github:owner/repo:42")
	}
}

func TestGraph_BuildGHEdges_PRWithMergeCommit(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       graphNodeID(GraphKindIssue, "github:owner/repo:42"),
		Label:    "https://github.com/owner/repo/issues/42",
		IssueNum: "42",
	})

	commitSHA := "abc123def456789"
	prListOut := `[{"number":100,"title":"Fix login","mergedAt":"2026-08-07T10:00:00Z"}]`
	mergeCommitOut := fmt.Sprintf(`{"mergeCommit":{"oid":"%s"}}`, commitSHA)

	runner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"api", "repos/owner/repo/issues/42", "--jq", ".pull_requests // [] | .[] | {number: .number, title: .title, mergedAt: .merged_at}"}): prListOut,
			joinArgs([]string{"--json", "mergeCommit", "--repo", "owner/repo", "pr", "view", "100"}):    mergeCommitOut,
		},
	}
	ctx := context.Background()

	if err := g.BuildGHEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildGHEdges: %v", err)
	}

	// Should have: 1 issue + 1 PR + 1 commit = 3 nodes, 2 edges.
	if g.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", g.NodeCount())
		for _, n := range g.Nodes() {
			t.Logf("  node: %s %s", n.Kind, n.ID)
		}
	}
	if g.EdgeCount() != 2 {
		t.Errorf("expected 2 edges, got %d", g.EdgeCount())
		for _, e := range g.Edges() {
			t.Logf("  edge: %s %s -> %s", e.Kind, e.From, e.To)
		}
	}

	// Check commit node.
	commitEdges := g.EdgesByKind(EdgePRCommit)
	if len(commitEdges) != 1 {
		t.Fatalf("expected 1 pr-commit edge, got %d", len(commitEdges))
	}
	if commitEdges[0].Source != "gh pr view --json mergeCommit" {
		t.Errorf("pr-commit edge source = %q, want %q", commitEdges[0].Source, "gh pr view --json mergeCommit")
	}

	// Verify commit node has correct hash.
	commitNode, ok := g.NodeByID(graphNodeID(GraphKindCommit, commitSHA))
	if !ok {
		t.Fatal("commit node not found")
	}
	if commitNode.Hash != commitSHA {
		t.Errorf("commit hash = %q, want %q", commitNode.Hash, commitSHA)
	}
}

func TestGraph_BuildGHEdges_PRNotMerged(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       graphNodeID(GraphKindIssue, "github:owner/repo:42"),
		Label:    "https://github.com/owner/repo/issues/42",
		IssueNum: "42",
	})

	prListOut := `[{"number":100,"title":"WIP feature","mergedAt":""}]`
	// gh pr view returns empty mergeCommit for unmerged PR.
	mergeCommitOut := `{"mergeCommit":{}}`

	runner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"api", "repos/owner/repo/issues/42", "--jq", ".pull_requests // [] | .[] | {number: .number, title: .title, mergedAt: .merged_at}"}): prListOut,
			joinArgs([]string{"--json", "mergeCommit", "--repo", "owner/repo", "pr", "view", "100"}):    mergeCommitOut,
		},
	}
	ctx := context.Background()

	if err := g.BuildGHEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildGHEdges: %v", err)
	}

	// Should have: 1 issue + 1 PR = 2 nodes (no commit for unmerged PR).
	if g.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildGHEdges_NoPRs(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       graphNodeID(GraphKindIssue, "github:owner/repo:42"),
		Label:    "https://github.com/owner/repo/issues/42",
		IssueNum: "42",
	})

	// gh issue view returns empty.
	runner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"api", "repos/owner/repo/issues/42", "--jq", ".pull_requests // [] | .[] | {number: .number, title: .title, mergedAt: .merged_at}"}): `{"pullRequests":[]}`,
		},
	}
	ctx := context.Background()

	if err := g.BuildGHEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildGHEdges: %v", err)
	}

	// Should have only the issue node.
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildGHEdges_GHError(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       graphNodeID(GraphKindIssue, "github:owner/repo:42"),
		Label:    "https://github.com/owner/repo/issues/42",
		IssueNum: "42",
	})

	// gh returns an error.
	runner := &mockGHRunner{
		errors: map[string]error{
			joinArgs([]string{"api", "repos/owner/repo/issues/42", "--jq", ".pull_requests // [] | .[] | {number: .number, title: .title, mergedAt: .merged_at}"}): fmt.Errorf("gh auth failed"),
		},
	}
	ctx := context.Background()

	// Should not panic or error — gh errors are logged to stderr, not returned.
	if err := g.BuildGHEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildGHEdges should not error on gh failures: %v", err)
	}

	// Graph should be unchanged.
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildGHEdges_MultipleIssues(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       graphNodeID(GraphKindIssue, "github:owner/repo:42"),
		Label:    "https://github.com/owner/repo/issues/42",
		IssueNum: "42",
	})
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       graphNodeID(GraphKindIssue, "github:owner/repo:43"),
		Label:    "https://github.com/owner/repo/issues/43",
		IssueNum: "43",
	})

	prList42 := `[{"number":100,"title":"Fix login","mergedAt":"2026-08-07T10:00:00Z"}]`
	prList43 := `[{"number":101,"title":"Add logout","mergedAt":"2026-08-07T11:00:00Z"}]`
	mergeCommit100 := `{"mergeCommit":{"oid":"abc123"}}`
	mergeCommit101 := `{"mergeCommit":{"oid":"def456"}}`

	runner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"api", "repos/owner/repo/issues/42", "--jq", ".pull_requests // [] | .[] | {number: .number, title: .title, mergedAt: .merged_at}"}): prList42,
			joinArgs([]string{"api", "repos/owner/repo/issues/43", "--jq", ".pull_requests // [] | .[] | {number: .number, title: .title, mergedAt: .merged_at}"}): prList43,
			joinArgs([]string{"--json", "mergeCommit", "--repo", "owner/repo", "pr", "view", "100"}):    mergeCommit100,
			joinArgs([]string{"--json", "mergeCommit", "--repo", "owner/repo", "pr", "view", "101"}):    mergeCommit101,
		},
	}
	ctx := context.Background()

	if err := g.BuildGHEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildGHEdges: %v", err)
	}

	// Should have: 2 issues + 2 PRs + 2 commits = 6 nodes, 4 edges.
	if g.NodeCount() != 6 {
		t.Errorf("expected 6 nodes, got %d", g.NodeCount())
		for _, n := range g.Nodes() {
			t.Logf("  node: %s %s", n.Kind, n.ID)
		}
	}
	if g.EdgeCount() != 4 {
		t.Errorf("expected 4 edges, got %d", g.EdgeCount())
		for _, e := range g.Edges() {
			t.Logf("  edge: %s %s -> %s", e.Kind, e.From, e.To)
		}
	}
}

func TestGraph_BuildGHEdges_NilRunner(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       graphNodeID(GraphKindIssue, "github:owner/repo:42"),
		Label:    "https://github.com/owner/repo/issues/42",
		IssueNum: "42",
	})

	ctx := context.Background()

	// Nil runner should use default (real gh), which will fail in tests.
	// That's fine — we just verify it doesn't panic.
	_ = g.BuildGHEdges(ctx, "owner/repo", nil)
}

func TestExtractIssueNum(t *testing.T) {
	tests := []struct {
		id     string
		expect string
	}{
		{"issue:github:owner/repo:42", "42"},
		{"issue:github:42", "42"},
		{"issue:github:owner/repo:123", "123"},
		{"spec:my-change", ""},
		{"", ""},
	}
	for _, tc := range tests {
		got := extractIssueNum(tc.id)
		if got != tc.expect {
			t.Errorf("extractIssueNum(%q) = %q, want %q", tc.id, got, tc.expect)
		}
	}
}

func TestGraph_BuildGHEdges_DeterministicOutput(t *testing.T) {
	// BuildGHEdges should produce the same graph regardless of map iteration order.
	for run := 0; run < 5; run++ {
		g := NewGraph()
		g.AddNode(Node{
			Kind:     GraphKindIssue,
			ID:       graphNodeID(GraphKindIssue, "github:owner/repo:42"),
			Label:    "https://github.com/owner/repo/issues/42",
			IssueNum: "42",
		})

		prListOut := `[{"number":100,"title":"Fix login","mergedAt":"2026-08-07T10:00:00Z"}]`
		mergeCommitOut := `{"mergeCommit":{"oid":"abc123"}}`

		runner := &mockGHRunner{
			responses: map[string]string{
				joinArgs([]string{"api", "repos/owner/repo/issues/42", "--jq", ".pull_requests // [] | .[] | {number: .number, title: .title, mergedAt: .merged_at}"}): prListOut,
				joinArgs([]string{"--json", "mergeCommit", "--repo", "owner/repo", "pr", "view", "100"}):    mergeCommitOut,
			},
		}
		ctx := context.Background()

		if err := g.BuildGHEdges(ctx, "owner/repo", runner); err != nil {
			t.Fatalf("BuildGHEdges: %v", err)
		}

		// Verify deterministic node count.
		if g.NodeCount() != 3 {
			t.Errorf("run %d: expected 3 nodes, got %d", run, g.NodeCount())
		}
		if g.EdgeCount() != 2 {
			t.Errorf("run %d: expected 2 edges, got %d", run, g.EdgeCount())
		}
	}
}
