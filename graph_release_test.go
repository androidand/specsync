package specsync

import (
	"context"
	"fmt"
	"testing"
)

func TestGraph_BuildReleaseEdges_NoCommits(t *testing.T) {
	g := NewGraph()
	// Add only a spec node, no commits.
	g.AddNode(Node{
		Kind:  GraphKindSpec,
		ID:    graphNodeID(GraphKindSpec, "my-change"),
		Label: "My Change",
	})

	ghRunner := &mockGHRunner{responses: make(map[string]string)}
	ctx := context.Background()

	if err := g.BuildReleaseEdges(ctx, "owner/repo", ghRunner, nil); err != nil {
		t.Fatalf("BuildReleaseEdges: %v", err)
	}

	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildReleaseEdges_NoReleases(t *testing.T) {
	g := NewGraph()
	// Add a commit node.
	g.AddNode(Node{
		Kind:       GraphKindCommit,
		ID:         graphNodeID(GraphKindCommit, "abc123"),
		Label:      "abc123 feat: add feature",
		Hash:       "abc123",
		CommitDate: "2026-08-07T10:00:00Z",
	})

	// Mock gh release list returning empty.
	ghRunner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"--json", "tagName", "--repo", "owner/repo", "release", "list"}): "[]",
		},
	}
	ctx := context.Background()

	if err := g.BuildReleaseEdges(ctx, "owner/repo", ghRunner, nil); err != nil {
		t.Fatalf("BuildReleaseEdges: %v", err)
	}

	// Should have only the commit node.
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildReleaseEdges_CommitInRelease(t *testing.T) {
	g := NewGraph()
	// Add a commit node.
	g.AddNode(Node{
		Kind:       GraphKindCommit,
		ID:         graphNodeID(GraphKindCommit, "abc123"),
		Label:      "abc123 feat: add feature",
		Hash:       "abc123",
		CommitDate: "2026-08-07T10:00:00Z",
	})

	// Mock gh release list returning one release.
	releaseListOut := `[{"tagName":"v1.0.0"}]`
	// Mock git tag --contains returning the tag name (commit is in release).
	tagContainsOut := "v1.0.0"

	ghRunner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"--json", "tagName", "--repo", "owner/repo", "release", "list"}): releaseListOut,
		},
	}
	gitRunner := &mockGitRunner{
		responses: map[string]string{
			joinArgs([]string{"abc123", "tag", "--contains", "v1.0.0"}): tagContainsOut,
		},
	}
	ctx := context.Background()

	if err := g.BuildReleaseEdges(ctx, "owner/repo", ghRunner, gitRunner); err != nil {
		t.Fatalf("BuildReleaseEdges: %v", err)
	}

	// Should have: 1 commit + 1 release = 2 nodes, 1 edge.
	if g.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NodeCount())
		for _, n := range g.Nodes() {
			t.Logf("  node: %s %s", n.Kind, n.ID)
		}
	}
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
		for _, e := range g.Edges() {
			t.Logf("  edge: %s %s -> %s", e.Kind, e.From, e.To)
		}
	}

	// Check release node.
	releaseNode, ok := g.NodeByID(graphNodeID(GraphKindRelease, "v1.0.0"))
	if !ok {
		t.Fatal("release node not found")
	}
	if releaseNode.TagName != "v1.0.0" {
		t.Errorf("release tag = %q, want %q", releaseNode.TagName, "v1.0.0")
	}

	// Check commit↔release edge.
	relEdges := g.EdgesByKind(EdgeCommitRel)
	if len(relEdges) != 1 {
		t.Fatalf("expected 1 commit-release edge, got %d", len(relEdges))
	}
	if relEdges[0].Source != "git tag --contains" {
		t.Errorf("commit-release edge source = %q, want %q", relEdges[0].Source, "git tag --contains")
	}
}

func TestGraph_BuildReleaseEdges_CommitNotInRelease(t *testing.T) {
	g := NewGraph()
	// Add a commit node.
	g.AddNode(Node{
		Kind:       GraphKindCommit,
		ID:         graphNodeID(GraphKindCommit, "abc123"),
		Label:      "abc123 feat: add feature",
		Hash:       "abc123",
		CommitDate: "2026-08-07T10:00:00Z",
	})

	// Mock gh release list returning one release.
	releaseListOut := `[{"tagName":"v1.0.0"}]`
	// Mock git tag --contains returning empty (commit not in release).
	tagContainsOut := ""

	ghRunner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"--json", "tagName", "--repo", "owner/repo", "release", "list"}): releaseListOut,
		},
	}
	gitRunner := &mockGitRunner{
		responses: map[string]string{
			joinArgs([]string{"abc123", "tag", "--contains", "v1.0.0"}): tagContainsOut,
		},
	}
	ctx := context.Background()

	if err := g.BuildReleaseEdges(ctx, "owner/repo", ghRunner, gitRunner); err != nil {
		t.Fatalf("BuildReleaseEdges: %v", err)
	}

	// Should have only the commit node (no release edge).
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildReleaseEdges_GHError(t *testing.T) {
	g := NewGraph()
	// Add a commit node.
	g.AddNode(Node{
		Kind:       GraphKindCommit,
		ID:         graphNodeID(GraphKindCommit, "abc123"),
		Label:      "abc123 feat: add feature",
		Hash:       "abc123",
		CommitDate: "2026-08-07T10:00:00Z",
	})

	// gh returns an error.
	ghRunner := &mockGHRunner{
		errors: map[string]error{
			joinArgs([]string{"--json", "tagName", "--repo", "owner/repo", "release", "list"}): fmt.Errorf("gh auth failed"),
		},
	}
	ctx := context.Background()

	// Should not panic or error — gh errors are logged to stderr, not returned.
	if err := g.BuildReleaseEdges(ctx, "owner/repo", ghRunner, nil); err != nil {
		t.Fatalf("BuildReleaseEdges should not error on gh failures: %v", err)
	}

	// Graph should be unchanged.
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildReleaseEdges_MultipleCommits(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{
		Kind:       GraphKindCommit,
		ID:         graphNodeID(GraphKindCommit, "abc123"),
		Label:      "abc123 feat: add feature",
		Hash:       "abc123",
		CommitDate: "2026-08-07T10:00:00Z",
	})
	g.AddNode(Node{
		Kind:       GraphKindCommit,
		ID:         graphNodeID(GraphKindCommit, "def456"),
		Label:      "def456 fix: bug fix",
		Hash:       "def456",
		CommitDate: "2026-08-07T11:00:00Z",
	})

	// Mock gh release list returning one release.
	releaseListOut := `[{"tagName":"v1.0.0"}]`
	// Both commits are in the release.
	tagContainsABC := "v1.0.0"
	tagContainsDEF := "v1.0.0"

	ghRunner := &mockGHRunner{
		responses: map[string]string{
			joinArgs([]string{"--json", "tagName", "--repo", "owner/repo", "release", "list"}): releaseListOut,
		},
	}
	gitRunner := &mockGitRunner{
		responses: map[string]string{
			joinArgs([]string{"abc123", "tag", "--contains", "v1.0.0"}): tagContainsABC,
			joinArgs([]string{"def456", "tag", "--contains", "v1.0.0"}): tagContainsDEF,
		},
	}
	ctx := context.Background()

	if err := g.BuildReleaseEdges(ctx, "owner/repo", ghRunner, gitRunner); err != nil {
		t.Fatalf("BuildReleaseEdges: %v", err)
	}

	// Should have: 2 commits + 1 release = 3 nodes, 2 edges.
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
}

func TestGraph_BuildReleaseEdges_DeterministicOutput(t *testing.T) {
	// BuildReleaseEdges should produce the same graph regardless of map iteration order.
	for run := 0; run < 5; run++ {
		g := NewGraph()
		g.AddNode(Node{
			Kind:       GraphKindCommit,
			ID:         graphNodeID(GraphKindCommit, "abc123"),
			Label:      "abc123 feat: add feature",
			Hash:       "abc123",
			CommitDate: "2026-08-07T10:00:00Z",
		})

		releaseListOut := `[{"tagName":"v1.0.0"}]`
		tagContainsOut := "v1.0.0"

		ghRunner := &mockGHRunner{
			responses: map[string]string{
				joinArgs([]string{"--json", "tagName", "--repo", "owner/repo", "release", "list"}): releaseListOut,
			},
		}
		gitRunner := &mockGitRunner{
			responses: map[string]string{
				joinArgs([]string{"abc123", "tag", "--contains", "v1.0.0"}): tagContainsOut,
			},
		}
		ctx := context.Background()

		if err := g.BuildReleaseEdges(ctx, "owner/repo", ghRunner, gitRunner); err != nil {
			t.Fatalf("BuildReleaseEdges: %v", err)
		}

		// Verify deterministic node count.
		if g.NodeCount() != 2 {
			t.Errorf("run %d: expected 2 nodes, got %d", run, g.NodeCount())
		}
		if g.EdgeCount() != 1 {
			t.Errorf("run %d: expected 1 edge, got %d", run, g.EdgeCount())
		}
	}
}

// mockGitRunner is a test double for GitRunner.
type mockGitRunner struct {
	responses map[string]string
	errors    map[string]error
}

func (m *mockGitRunner) Run(ctx context.Context, args ...string) (string, error) {
	key := joinArgs(args)

	if err, ok := m.errors[key]; ok {
		return "", err
	}
	if out, ok := m.responses[key]; ok {
		return out, nil
	}
	return "", nil
}


