package main

import (
	"testing"

	"github.com/androidand/specsync"
)

func TestSortChanges_StageOrder(t *testing.T) {
	changes := []specsync.Change{
		{Slug: "c", Stage: specsync.StageActive},
		{Slug: "a", Stage: specsync.StageBacklog},
		{Slug: "b", Stage: specsync.StageArchived},
	}
	sortChanges(changes, "stage")

	if changes[0].Slug != "a" || changes[0].Stage != specsync.StageBacklog {
		t.Errorf("first: got %s/%s, want a/backlog", changes[0].Slug, changes[0].Stage)
	}
	if changes[1].Slug != "c" || changes[1].Stage != specsync.StageActive {
		t.Errorf("second: got %s/%s, want c/active", changes[1].Slug, changes[1].Stage)
	}
	if changes[2].Slug != "b" || changes[2].Stage != specsync.StageArchived {
		t.Errorf("third: got %s/%s, want b/archived", changes[2].Slug, changes[2].Stage)
	}
}

func TestSortChanges_Priority(t *testing.T) {
	p1, p2, p3 := 10, 90, 50
	changes := []specsync.Change{
		{Slug: "c", Priority: &p1},
		{Slug: "a", Priority: &p3},
		{Slug: "b", Priority: &p2},
	}
	sortChanges(changes, "priority")

	if changes[0].Slug != "b" || *changes[0].Priority != 90 {
		t.Errorf("first: got %s/%d, want b/90", changes[0].Slug, *changes[0].Priority)
	}
	if changes[2].Slug != "c" || *changes[2].Priority != 10 {
		t.Errorf("third: got %s/%d, want c/10", changes[2].Slug, *changes[2].Priority)
	}
}

func TestSortChanges_Slug(t *testing.T) {
	changes := []specsync.Change{
		{Slug: "c"},
		{Slug: "a"},
		{Slug: "b"},
	}
	sortChanges(changes, "slug")

	if changes[0].Slug != "a" || changes[1].Slug != "b" || changes[2].Slug != "c" {
		t.Errorf("got %s, %s, %s; want a, b, c", changes[0].Slug, changes[1].Slug, changes[2].Slug)
	}
}

func TestSortChanges_NilPriority(t *testing.T) {
	p1 := 90
	changes := []specsync.Change{
		{Slug: "c"},
		{Slug: "a", Priority: &p1},
		{Slug: "b"},
	}
	sortChanges(changes, "priority")

	if changes[0].Slug != "a" || changes[0].Priority == nil {
		t.Errorf("first should be a with priority 90")
	}
	if changes[2].Priority != nil {
		t.Errorf("last should have nil priority")
	}
}

func TestTruncate(t *testing.T) {
	result := truncate("hello", 5)
	if result != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}

	result = truncate("hello world", 5)
	if result != "hell…" {
		t.Errorf("got %q, want %q", result, "hell…")
	}
}

func TestCollectDiagnostics(t *testing.T) {
	c := specsync.Change{
		Slug:        "test",
		Stage:       specsync.StageActive,
		StageSource: specsync.StageSourceDefault,
		Priority:    nil,
	}
	diagnostics := collectDiagnostics(c)
	if len(diagnostics) != 0 {
		t.Errorf("expected no diagnostics, got %v", diagnostics)
	}

	// Non-canonical stage
	c.Stage = "custom-stage"
	c.StageSource = specsync.StageSourceDefault
	diagnostics = collectDiagnostics(c)
	if len(diagnostics) != 1 {
		t.Errorf("expected 1 diagnostic, got %d: %v", len(diagnostics), diagnostics)
	}

	// Out-of-range priority
	c.Stage = specsync.StageActive
	c.Priority = intPtr(101)
	diagnostics = collectDiagnostics(c)
	if len(diagnostics) != 1 {
		t.Errorf("expected 1 diagnostic, got %d: %v", len(diagnostics), diagnostics)
	}
}

func TestCountCheckboxes(t *testing.T) {
	tests := []struct {
		name        string
		md          string
		wantTotal   int
		wantCompleted int
	}{
		{
			name:        "empty",
			md:          "",
			wantTotal:   0,
			wantCompleted: 0,
		},
		{
			name:        "no checkboxes",
			md:          "# Tasks\n\nSome text here.",
			wantTotal:   0,
			wantCompleted: 0,
		},
		{
			name:        "all unchecked",
			md:          "- [ ] task 1\n- [ ] task 2",
			wantTotal:   2,
			wantCompleted: 0,
		},
		{
			name:        "all checked",
			md:          "- [x] task 1\n- [x] task 2",
			wantTotal:   2,
			wantCompleted: 2,
		},
		{
			name:        "mixed",
			md:          "- [x] task 1\n- [ ] task 2\n- [x] task 3",
			wantTotal:   3,
			wantCompleted: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, completed := specsync.CountCheckboxes(tt.md)
			if total != tt.wantTotal {
				t.Errorf("total: got %d, want %d", total, tt.wantTotal)
			}
			if completed != tt.wantCompleted {
				t.Errorf("completed: got %d, want %d", completed, tt.wantCompleted)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
