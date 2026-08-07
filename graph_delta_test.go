package specsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGraph_BuildDeltaAnnotations_NoSpecs(t *testing.T) {
	g := NewGraph()
	// Add only an issue node.
	g.AddNode(Node{
		Kind:     GraphKindIssue,
		ID:       graphNodeID(GraphKindIssue, "github:owner/repo:42"),
		Label:    "https://github.com/owner/repo/issues/42",
		IssueNum: "42",
	})

	ctx := context.Background()

	// Should not error even without spec nodes.
	if err := g.BuildDeltaAnnotations(ctx); err != nil {
		t.Fatalf("BuildDeltaAnnotations: %v", err)
	}

	// Graph should be unchanged.
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
}

func TestGraph_BuildDeltaAnnotations_NoOpenSpec(t *testing.T) {
	g := NewGraph()
	// Add a spec node.
	g.AddNode(Node{
		Kind:  GraphKindSpec,
		ID:    graphNodeID(GraphKindSpec, "my-change"),
		Label: "My Change",
		Slug:  "my-change",
	})

	// Temporarily remove openspec from PATH.
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", "/usr/bin:/bin")
	defer os.Setenv("PATH", originalPath)

	ctx := context.Background()

	// Should not error when openspec is not available.
	if err := g.BuildDeltaAnnotations(ctx); err != nil {
		t.Fatalf("BuildDeltaAnnotations: %v", err)
	}

	// Graph should be unchanged.
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
}

func TestGraph_BuildDeltaAnnotations_WithDeltas(t *testing.T) {
	// Create a fake openscript that returns delta JSON.
	tmpDir := t.TempDir()
	fakeOpenScript := filepath.Join(tmpDir, "openspec")
	if err := os.WriteFile(fakeOpenScript, []byte(`#!/bin/sh
if echo "$@" | grep -q "show.*--json.*--deltas-only.*my-change"; then
  echo '{"deltas":[{"operation":"ADDED","spec":"api","requirement":"GET /users"},{"operation":"ADDED","spec":"api","requirement":"POST /users"},{"operation":"MODIFIED","spec":"api","requirement":"GET /users/{id}"},{"operation":"REMOVED","spec":"api","requirement":"GET /legacy"}]}'
else
  echo '{"deltas":[]}'
fi
`), 0o755); err != nil {
		t.Fatal(err)
	}

	// Temporarily prepend fake openscript to PATH.
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+originalPath)
	defer os.Setenv("PATH", originalPath)

	g := NewGraph()
	g.AddNode(Node{
		Kind:  GraphKindSpec,
		ID:    graphNodeID(GraphKindSpec, "my-change"),
		Label: "My Change",
		Slug:  "my-change",
	})

	ctx := context.Background()

	if err := g.BuildDeltaAnnotations(ctx); err != nil {
		t.Fatalf("BuildDeltaAnnotations: %v", err)
	}

	// Verify the spec node label was updated with delta counts.
	n, ok := g.NodeByID(graphNodeID(GraphKindSpec, "my-change"))
	if !ok {
		t.Fatal("spec node not found")
	}
	// Label should be mutated to include delta counts: "My Change [+2 ~1 -1]"
	expectedLabel := "My Change [+2 ~1 -1]"
	if n.Label != expectedLabel {
		t.Errorf("label = %q, want %q", n.Label, expectedLabel)
	}
}

func TestGraph_BuildDeltaAnnotations_NoDeltas(t *testing.T) {
	// Create a fake openscript that returns empty deltas.
	tmpDir := t.TempDir()
	fakeOpenScript := filepath.Join(tmpDir, "openspec")
	if err := os.WriteFile(fakeOpenScript, []byte(`#!/bin/sh
echo '{"deltas":[]}'
`), 0o755); err != nil {
		t.Fatal(err)
	}

	// Temporarily prepend fake openscript to PATH.
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+originalPath)
	defer os.Setenv("PATH", originalPath)

	g := NewGraph()
	g.AddNode(Node{
		Kind:  GraphKindSpec,
		ID:    graphNodeID(GraphKindSpec, "my-change"),
		Label: "My Change",
		Slug:  "my-change",
	})

	ctx := context.Background()

	if err := g.BuildDeltaAnnotations(ctx); err != nil {
		t.Fatalf("BuildDeltaAnnotations: %v", err)
	}

	// Verify the spec node label was NOT updated (no deltas).
	n, ok := g.NodeByID(graphNodeID(GraphKindSpec, "my-change"))
	if !ok {
		t.Fatal("spec node not found")
	}
	if n.Label != "My Change" {
		t.Errorf("label = %q, want %q (no deltas)", n.Label, "My Change")
	}
}

func TestGraph_BuildDeltaAnnotations_MissingSlug(t *testing.T) {
	// Create a fake openscript that returns delta JSON.
	tmpDir := t.TempDir()
	fakeOpenScript := filepath.Join(tmpDir, "openspec")
	if err := os.WriteFile(fakeOpenScript, []byte(`#!/bin/sh
echo '{"deltas":[]}'
`), 0o755); err != nil {
		t.Fatal(err)
	}

	// Temporarily prepend fake openscript to PATH.
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+originalPath)
	defer os.Setenv("PATH", originalPath)

	g := NewGraph()
	// Add a spec node without a slug.
	g.AddNode(Node{
		Kind:  GraphKindSpec,
		ID:    graphNodeID(GraphKindSpec, "my-change"),
		Label: "My Change",
		Slug:  "", // No slug — should be skipped
	})

	ctx := context.Background()

	if err := g.BuildDeltaAnnotations(ctx); err != nil {
		t.Fatalf("BuildDeltaAnnotations: %v", err)
	}

	// Verify the spec node label was NOT updated (no slug).
	n, ok := g.NodeByID(graphNodeID(GraphKindSpec, "my-change"))
	if !ok {
		t.Fatal("spec node not found")
	}
	if n.Label != "My Change" {
		t.Errorf("label = %q, want %q (no slug)", n.Label, "My Change")
	}
}

func TestGraph_BuildDeltaAnnotations_ExecError(t *testing.T) {
	// Create a fake openscript that fails.
	tmpDir := t.TempDir()
	fakeOpenScript := filepath.Join(tmpDir, "openspec")
	if err := os.WriteFile(fakeOpenScript, []byte(`#!/bin/sh
exit 1
`), 0o755); err != nil {
		t.Fatal(err)
	}

	// Temporarily prepend fake openscript to PATH.
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+originalPath)
	defer os.Setenv("PATH", originalPath)

	g := NewGraph()
	g.AddNode(Node{
		Kind:  GraphKindSpec,
		ID:    graphNodeID(GraphKindSpec, "my-change"),
		Label: "My Change",
		Slug:  "my-change",
	})

	ctx := context.Background()

	// Should not error — openspec errors are logged to stderr, not returned.
	if err := g.BuildDeltaAnnotations(ctx); err != nil {
		t.Fatalf("BuildDeltaAnnotations should not error on openspec failures: %v", err)
	}

	// Graph should be unchanged (label not mutated).
	n, ok := g.NodeByID(graphNodeID(GraphKindSpec, "my-change"))
	if !ok {
		t.Fatal("spec node not found")
	}
	if n.Label != "My Change" {
		t.Errorf("label = %q, want %q (openspec error)", n.Label, "My Change")
	}
}

func TestCountDeltas(t *testing.T) {
	deltas := []OpenSpecDelta{
		{Operation: "ADDED", Spec: "api", Requirement: "GET /users"},
		{Operation: "ADDED", Spec: "api", Requirement: "POST /users"},
		{Operation: "MODIFIED", Spec: "api", Requirement: "GET /users/{id}"},
		{Operation: "REMOVED", Spec: "api", Requirement: "GET /legacy"},
	}

	added, modified, removed := countDeltas(deltas)
	if added != 2 {
		t.Errorf("expected 2 added, got %d", added)
	}
	if modified != 1 {
		t.Errorf("expected 1 modified, got %d", modified)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
}

func TestCountDeltas_Empty(t *testing.T) {
	added, modified, removed := countDeltas(nil)
	if added != 0 || modified != 0 || removed != 0 {
		t.Errorf("expected 0,0,0 for empty deltas, got %d,%d,%d", added, modified, removed)
	}
}

func TestCountDeltas_UnknownOperation(t *testing.T) {
	deltas := []OpenSpecDelta{
		{Operation: "UNKNOWN", Spec: "api", Requirement: "test"},
	}

	added, modified, removed := countDeltas(deltas)
	if added != 0 || modified != 0 || removed != 0 {
		t.Errorf("expected 0,0,0 for unknown operation, got %d,%d,%d", added, modified, removed)
	}
}
