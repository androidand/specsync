package specsync

import (
	"context"
	"fmt"
	"testing"
)

func TestGraph_BuildWorkFileEdges_NoPRsOrCommits(t *testing.T) {
	g := NewGraph()
	// Add only a spec node.
	g.AddNode(Node{
		Kind:  GraphKindSpec,
		ID:    graphNodeID(GraphKindSpec, "my-change"),
		Label: "My Change",
	})

	runner := &mockGHRunner{responses: make(map[string]string)}
	ctx := context.Background()

	if err := g.BuildWorkFileEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildWorkFileEdges: %v", err)
	}

	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildWorkFileEdges_PRWithFiles(t *testing.T) {
	g := NewGraph()
	// Add a PR node.
	g.AddNode(Node{
		Kind:     GraphKindPR,
		ID:       graphNodeID(GraphKindPR, "owner/repo:100"),
		Label:    "#100: Fix login",
		IssueNum: "100",
	})

	// Mock gh pr view --json files returning changed files.
	prFilesOut := `{"files":[{"path":"src/login.ts"},{"path":"src/auth.ts"}]}`

	runner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"--json", "files", "--repo", "owner/repo", "pr", "view", "100"}): prFilesOut,
		},
	}
	ctx := context.Background()

	if err := g.BuildWorkFileEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildWorkFileEdges: %v", err)
	}

	// Should have: 1 PR + 2 file edges = 1 node, 2 edges.
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 2 {
		t.Errorf("expected 2 edges, got %d", g.EdgeCount())
	}

	// Check work↔files edges.
	fileEdges := g.EdgesByKind(EdgeWorkFiles)
	if len(fileEdges) != 2 {
		t.Fatalf("expected 2 work-files edges, got %d", len(fileEdges))
	}

	// Verify edge sources.
	for _, e := range fileEdges {
		if e.Source != "gh pr view --json files" {
			t.Errorf("work-files edge source = %q, want %q", e.Source, "gh pr view --json files")
		}
	}
}

func TestGraph_BuildWorkFileEdges_CommitWithFiles(t *testing.T) {
	g := NewGraph()
	// Add a commit node.
	g.AddNode(Node{
		Kind:       GraphKindCommit,
		ID:         graphNodeID(GraphKindCommit, "abc123"),
		Label:      "abc123 feat: add feature",
		Hash:       "abc123",
		CommitDate: "2026-08-07T10:00:00Z",
	})

	// Mock git diff-tree returning changed files.
	commitFilesOut := "src/feature.ts\nsrc/feature_test.ts"

	runner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"abc123", "diff-tree", "--no-commit-id", "-r", "--name-only"}): commitFilesOut,
		},
	}
	ctx := context.Background()

	if err := g.BuildWorkFileEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildWorkFileEdges: %v", err)
	}

	// Should have: 1 commit + 2 file edges = 1 node, 2 edges.
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 2 {
		t.Errorf("expected 2 edges, got %d", g.EdgeCount())
	}

	// Check work↔files edges.
	fileEdges := g.EdgesByKind(EdgeWorkFiles)
	if len(fileEdges) != 2 {
		t.Fatalf("expected 2 work-files edges, got %d", len(fileEdges))
	}

	// Verify edge sources.
	for _, e := range fileEdges {
		if e.Source != "git diff-tree --name-only" {
			t.Errorf("work-files edge source = %q, want %q", e.Source, "git diff-tree --name-only")
		}
	}
}

func TestGraph_BuildWorkFileEdges_NoFiles(t *testing.T) {
	g := NewGraph()
	// Add a PR node.
	g.AddNode(Node{
		Kind:     GraphKindPR,
		ID:       graphNodeID(GraphKindPR, "owner/repo:100"),
		Label:    "#100: Empty PR",
		IssueNum: "100",
	})

	// Mock gh pr view --json files returning empty.
	prFilesOut := `{"files":[]}`

	runner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"--json", "files", "--repo", "owner/repo", "pr", "view", "100"}): prFilesOut,
		},
	}
	ctx := context.Background()

	if err := g.BuildWorkFileEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildWorkFileEdges: %v", err)
	}

	// Should have only the PR node (no file edges).
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildWorkFileEdges_GHError(t *testing.T) {
	g := NewGraph()
	// Add a PR node.
	g.AddNode(Node{
		Kind:     GraphKindPR,
		ID:       graphNodeID(GraphKindPR, "owner/repo:100"),
		Label:    "#100: Fix login",
		IssueNum: "100",
	})

	// gh returns an error.
	runner := &mockGHRunner{
		errors: map[string]error{
			joinArgs([]string{"--json", "files", "--repo", "owner/repo", "pr", "view", "100"}): fmt.Errorf("gh auth failed"),
		},
	}
	ctx := context.Background()

	// Should not panic or error — gh errors are logged to stderr, not returned.
	if err := g.BuildWorkFileEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildWorkFileEdges should not error on gh failures: %v", err)
	}

	// Graph should be unchanged.
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildWorkFileEdges_MixedPRsAndCommits(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:     GraphKindPR,
		ID:       graphNodeID(GraphKindPR, "owner/repo:100"),
		Label:    "#100: Fix login",
		IssueNum: "100",
	})
	g.AddNode(Node{
		Kind:       GraphKindCommit,
		ID:         graphNodeID(GraphKindCommit, "abc123"),
		Label:      "abc123 feat: add feature",
		Hash:       "abc123",
		CommitDate: "2026-08-07T10:00:00Z",
	})

	// Mock gh pr view --json files.
	prFilesOut := `{"files":[{"path":"src/login.ts"}]}`
	// Mock git diff-tree.
	commitFilesOut := "src/feature.ts"

	runner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"--json", "files", "--repo", "owner/repo", "pr", "view", "100"}): prFilesOut,
			joinArgs([]string{"abc123", "diff-tree", "--no-commit-id", "-r", "--name-only"}):    commitFilesOut,
		},
	}
	ctx := context.Background()

	if err := g.BuildWorkFileEdges(ctx, "owner/repo", runner); err != nil {
		t.Fatalf("BuildWorkFileEdges: %v", err)
	}

	// Should have: 1 PR + 1 commit + 2 file edges = 2 nodes, 2 edges.
	if g.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NodeCount())
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
}

func TestExtractPRNum(t *testing.T) {
	tests := []struct {
		id     string
		expect string
	}{
		{"pr:owner/repo:100", "100"},
		{"pr:owner/repo:1", "1"},
		{"spec:my-change", ""},
		{"", ""},
	}
	for _, tc := range tests {
		got := extractPRNum(tc.id)
		if got != tc.expect {
			t.Errorf("extractPRNum(%q) = %q, want %q", tc.id, got, tc.expect)
		}
	}
}

func TestGraph_BuildWorkFileEdges_DeterministicOutput(t *testing.T) {
	// BuildWorkFileEdges should produce the same graph regardless of map iteration order.
	for run := 0; run < 5; run++ {
		g := NewGraph()
		g.AddNode(Node{
			Kind:     GraphKindPR,
			ID:       graphNodeID(GraphKindPR, "owner/repo:100"),
			Label:    "#100: Fix login",
			IssueNum: "100",
		})

		prFilesOut := `{"files":[{"path":"src/login.ts"},{"path":"src/auth.ts"}]}`

		runner := &mockGHRunner{
			responses: map[string]string{
				joinArgs([]string{"--json", "files", "--repo", "owner/repo", "pr", "view", "100"}): prFilesOut,
			},
		}
		ctx := context.Background()

		if err := g.BuildWorkFileEdges(ctx, "owner/repo", runner); err != nil {
			t.Fatalf("BuildWorkFileEdges: %v", err)
		}

		// Verify deterministic edge count.
		if g.EdgeCount() != 2 {
			t.Errorf("run %d: expected 2 edges, got %d", run, g.EdgeCount())
		}
	}
}
