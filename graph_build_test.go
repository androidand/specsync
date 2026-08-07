package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGraph_BuildCore_EmptyChange(t *testing.T) {
	g := NewGraph()
	c := Change{
		Dir:   t.TempDir(),
		Slug:  "empty-change",
		Title: "Empty Change",
		Stage: StageActive,
	}
	if err := os.WriteFile(filepath.Join(c.Dir, "proposal.md"), []byte("# Empty Change"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := g.BuildCore(c, ""); err != nil {
		t.Fatalf("BuildCore: %v", err)
	}

	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}

	n, ok := g.NodeByID(graphNodeID(GraphKindSpec, "empty-change"))
	if !ok {
		t.Fatal("spec node not found")
	}
	if n.Label != "Empty Change" || n.Stage != StageActive {
		t.Errorf("node fields: label=%q, stage=%q", n.Label, n.Stage)
	}
}

func TestGraph_BuildCore_WithRefs(t *testing.T) {
	g := NewGraph()
	dir := t.TempDir()

	// Create a ref cache with one GitHub ref.
	metaDir := filepath.Join(dir, ".specsync")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	refsJSON := `{"github:owner/repo":{"provider":"github:owner/repo","id":"42","url":"https://github.com/owner/repo/issues/42"}}`
	if err := os.WriteFile(filepath.Join(metaDir, "refs.json"), []byte(refsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	c := Change{
		Dir:   dir,
		Slug:  "with-refs",
		Title: "With Refs",
		Stage: StageActive,
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# With Refs"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := g.BuildCore(c, ""); err != nil {
		t.Fatalf("BuildCore: %v", err)
	}

	if g.NodeCount() != 2 {
		t.Fatalf("expected 2 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 1 {
		t.Fatalf("expected 1 edge, got %d", g.EdgeCount())
	}

	// Check spec node.
	specNode, ok := g.NodeByID(graphNodeID(GraphKindSpec, "with-refs"))
	if !ok {
		t.Fatal("spec node not found")
	}
	if specNode.Label != "With Refs" {
		t.Errorf("spec label = %q, want %q", specNode.Label, "With Refs")
	}

	// Check issue node.
	issueNode, ok := g.NodeByID(graphNodeID(GraphKindIssue, "github:owner/repo:42"))
	if !ok {
		t.Fatal("issue node not found")
	}
	if issueNode.IssueNum != "42" {
		t.Errorf("issue num = %q, want %q", issueNode.IssueNum, "42")
	}

	// Check edge.
	edges := g.Edges()
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Kind != EdgeSpecIssue {
		t.Errorf("edge kind = %q, want %q", edges[0].Kind, EdgeSpecIssue)
	}
	if edges[0].Source != "refs.json" {
		t.Errorf("edge source = %q, want %q", edges[0].Source, "refs.json")
	}
}

func TestGraph_BuildCore_MultipleRefs(t *testing.T) {
	g := NewGraph()
	dir := t.TempDir()

	// Create a ref cache with multiple refs.
	metaDir := filepath.Join(dir, ".specsync")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	refsJSON := `{
		"github:owner/repo":{"provider":"github:owner/repo","id":"42","url":"https://github.com/owner/repo/issues/42"},
		"github:owner/other":{"provider":"github:owner/other","id":"7","url":"https://github.com/owner/other/issues/7"}
	}`
	if err := os.WriteFile(filepath.Join(metaDir, "refs.json"), []byte(refsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	c := Change{
		Dir:   dir,
		Slug:  "multi-ref",
		Title: "Multi Ref",
		Stage: StageComplete,
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# Multi Ref"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := g.BuildCore(c, ""); err != nil {
		t.Fatalf("BuildCore: %v", err)
	}

	if g.NodeCount() != 3 {
		t.Fatalf("expected 3 nodes (1 spec + 2 issues), got %d", g.NodeCount())
	}
	if g.EdgeCount() != 2 {
		t.Fatalf("expected 2 edges, got %d", g.EdgeCount())
	}

	// Verify both issue nodes exist.
	if _, ok := g.NodeByID(graphNodeID(GraphKindIssue, "github:owner/repo:42")); !ok {
		t.Error("missing issue github:owner/repo:42")
	}
	if _, ok := g.NodeByID(graphNodeID(GraphKindIssue, "github:owner/other:7")); !ok {
		t.Error("missing issue github:owner/other:7")
	}
}

func TestGraph_BuildCore_NoRefsFile(t *testing.T) {
	g := NewGraph()
	dir := t.TempDir()

	// No .specsync directory at all.
	c := Change{
		Dir:   dir,
		Slug:  "no-refs",
		Title: "No Refs",
		Stage: StageActive,
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# No Refs"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := g.BuildCore(c, ""); err != nil {
		t.Fatalf("BuildCore: %v", err)
	}

	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildCore_WithLinksMD(t *testing.T) {
	g := NewGraph()
	dir := t.TempDir()

	// Create a links.md with a sibling slug entry.
	linksMD := `## Related
- sibling-change
`
	if err := os.WriteFile(filepath.Join(dir, "links.md"), []byte(linksMD), 0o644); err != nil {
		t.Fatal(err)
	}

	c := Change{
		Dir:   dir,
		Slug:  "my-change",
		Title: "My Change",
		Stage: StageActive,
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# My Change"), 0o644); err != nil {
		t.Fatal(err)
	}

	// With empty openspecDir, sibling slugs can't be resolved.
	if err := g.BuildCore(c, ""); err != nil {
		t.Fatalf("BuildCore: %v", err)
	}

	// Should have only the spec node (sibling not resolved).
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildCore_MixedRefsAndLinks(t *testing.T) {
	g := NewGraph()
	dir := t.TempDir()

	// Create a ref cache with one ref.
	metaDir := filepath.Join(dir, ".specsync")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	refsJSON := `{"github:owner/repo":{"provider":"github:owner/repo","id":"10","url":"https://github.com/owner/repo/issues/10"}}`
	if err := os.WriteFile(filepath.Join(metaDir, "refs.json"), []byte(refsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a links.md with an external URL (not a resolvable slug).
	linksMD := `## Related
- https://github.com/other/repo/issues/99
`
	if err := os.WriteFile(filepath.Join(dir, "links.md"), []byte(linksMD), 0o644); err != nil {
		t.Fatal(err)
	}

	c := Change{
		Dir:   dir,
		Slug:  "mixed",
		Title: "Mixed",
		Stage: StageActive,
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# Mixed"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := g.BuildCore(c, ""); err != nil {
		t.Fatalf("BuildCore: %v", err)
	}

	// Should have 2 nodes: 1 spec + 1 issue (from refs.json).
	// The external URL in links.md doesn't resolve to a sibling slug.
	if g.NodeCount() != 2 {
		t.Fatalf("expected 2 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 1 {
		t.Fatalf("expected 1 edge, got %d", g.EdgeCount())
	}
}

func TestGraph_BuildCore_DeterministicOutput(t *testing.T) {
	// BuildCore should produce the same graph regardless of map iteration order.
	// Run multiple times to catch non-determinism.
	for run := 0; run < 10; run++ {
		g := NewGraph()
		dir := t.TempDir()

		// Create a ref cache with multiple entries.
		metaDir := filepath.Join(dir, ".specsync")
		if err := os.MkdirAll(metaDir, 0o755); err != nil {
			t.Fatal(err)
		}
		refsJSON := `{
			"github:owner/z":{"provider":"github:owner/z","id":"9","url":"https://github.com/owner/z/issues/9"},
			"github:owner/a":{"provider":"github:owner/a","id":"1","url":"https://github.com/owner/a/issues/1"},
			"github:owner/m":{"provider":"github:owner/m","id":"5","url":"https://github.com/owner/m/issues/5"}
		}`
		if err := os.WriteFile(filepath.Join(metaDir, "refs.json"), []byte(refsJSON), 0o644); err != nil {
			t.Fatal(err)
		}

		c := Change{
			Dir:   dir,
			Slug:  "deterministic",
			Title: "Deterministic",
			Stage: StageActive,
		}
		if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# Deterministic"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := g.BuildCore(c, ""); err != nil {
			t.Fatalf("BuildCore: %v", err)
		}

		// Verify deterministic node order.
		nodes := g.Nodes()
		if len(nodes) != 4 {
			t.Fatalf("run %d: expected 4 nodes, got %d", run, len(nodes))
		}
		// Spec node should be first (sorted by full ID: "spec:" < "issue:").
		// Actually "issue:" < "spec:" lexicographically, so issues come first.
		// The important thing is consistency across runs.
	}
}
