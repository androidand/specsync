package main

import (
	"os"
	"path/filepath"
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

func TestSetStage_CanonicalStages(t *testing.T) {
	tmpDir := t.TempDir()
	changesDir := filepath.Join(tmpDir, "openspec", "changes")
	changeDir := filepath.Join(changesDir, "test-change")
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set stage to backlog
	backlog := specsync.StageBacklog
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Stage: &backlog}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	meta, err := specsync.LoadChangeMetadata(changeDir)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if meta == nil || meta.Stage == nil || *meta.Stage != specsync.StageBacklog {
		t.Fatalf("expected stage backlog, got %v", meta)
	}

	// Set stage to active
	active := specsync.StageActive
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Stage: &active}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	meta, err = specsync.LoadChangeMetadata(changeDir)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if *meta.Stage != specsync.StageActive {
		t.Fatalf("expected stage active, got %s", *meta.Stage)
	}
}

func TestSetStage_Auto(t *testing.T) {
	tmpDir := t.TempDir()
	changesDir := filepath.Join(tmpDir, "openspec", "changes")
	changeDir := filepath.Join(changesDir, "test-change")
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set stage to backlog first
	backlog := specsync.StageBacklog
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Stage: &backlog}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	// Set stage to auto (should remove stage)
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	meta, err := specsync.LoadChangeMetadata(changeDir)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if meta != nil && meta.Stage != nil {
		t.Fatalf("expected nil stage, got %s", *meta.Stage)
	}
}

func TestSetStage_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	changesDir := filepath.Join(tmpDir, "openspec", "changes")
	changeDir := filepath.Join(changesDir, "test-change")
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Invalid stage should fail
	invalid := specsync.Stage("INVALID")
	err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Stage: &invalid})
	if err == nil {
		t.Fatal("expected error for invalid stage")
	}
}

func TestSetPriority_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	changesDir := filepath.Join(tmpDir, "openspec", "changes")
	changeDir := filepath.Join(changesDir, "test-change")
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set priority
	p := 75
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Priority: &p}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	meta, err := specsync.LoadChangeMetadata(changeDir)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if meta == nil || meta.Priority == nil || *meta.Priority != 75 {
		t.Fatalf("expected priority 75, got %v", meta)
	}

	// Unset priority
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	meta, err = specsync.LoadChangeMetadata(changeDir)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if meta != nil && meta.Priority != nil {
		t.Fatalf("expected nil priority, got %d", *meta.Priority)
	}
}

func TestSetPriority_OutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	changesDir := filepath.Join(tmpDir, "openspec", "changes")
	changeDir := filepath.Join(changesDir, "test-change")
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Priority 0 should fail
	p0 := 0
	err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Priority: &p0})
	if err == nil {
		t.Fatal("expected error for priority 0")
	}

	// Priority 101 should fail
	p101 := 101
	err = specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Priority: &p101})
	if err == nil {
		t.Fatal("expected error for priority 101")
	}

	// Boundary: 1 should succeed
	p1 := 1
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Priority: &p1}); err != nil {
		t.Fatalf("set priority 1: %v", err)
	}

	// Boundary: 100 should succeed
	p100 := 100
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Priority: &p100}); err != nil {
		t.Fatalf("set priority 100: %v", err)
	}
}

func TestSetStage_ArchivedReject(t *testing.T) {
	tmpDir := t.TempDir()
	changesDir := filepath.Join(tmpDir, "openspec", "changes")
	archiveDir := filepath.Join(changesDir, "archive", "test-change")
	if err := os.MkdirAll(filepath.Join(archiveDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archiveDir, "proposal.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// LoadChangeBySlug should find the archived change
	c, err := specsync.LoadChangeBySlug(filepath.Join(tmpDir, "openspec"), "test-change")
	if err != nil {
		t.Fatalf("load change: %v", err)
	}
	if !c.Archived {
		t.Fatal("expected archived change")
	}
}

func TestEmptyMetadataCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	changesDir := filepath.Join(tmpDir, "openspec", "changes")
	changeDir := filepath.Join(changesDir, "test-change")
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set stage + priority
	backlog := specsync.StageBacklog
	p := 50
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Stage: &backlog, Priority: &p}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	// Remove stage, keep priority
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Priority: &p}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	meta, err := specsync.LoadChangeMetadata(changeDir)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if meta.Stage != nil {
		t.Fatal("expected nil stage")
	}
	if *meta.Priority != 50 {
		t.Fatalf("expected priority 50, got %d", *meta.Priority)
	}

	// Remove both — file should be deleted
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	meta, err = specsync.LoadChangeMetadata(changeDir)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if meta != nil {
		t.Fatal("expected nil metadata after cleanup")
	}
}

func TestSaveChangeMetadata_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	changesDir := filepath.Join(tmpDir, "openspec", "changes")
	changeDir := filepath.Join(changesDir, "test-change")
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write metadata
	active := specsync.StageActive
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Stage: &active}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	// Verify no temp files remain
	tmps, _ := filepath.Glob(filepath.Join(changeDir, ".specsync", "*.tmp"))
	if len(tmps) > 0 {
		t.Fatalf("expected no temp files, got %v", tmps)
	}

	// Overwrite
	backlog := specsync.StageBacklog
	if err := specsync.SaveChangeMetadata(changeDir, specsync.ChangeMetadata{Version: 1, Stage: &backlog}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	meta, err := specsync.LoadChangeMetadata(changeDir)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if *meta.Stage != specsync.StageBacklog {
		t.Fatalf("expected backlog, got %s", *meta.Stage)
	}

	// No temp files after overwrite
	tmps, _ = filepath.Glob(filepath.Join(changeDir, ".specsync", "*.tmp"))
	if len(tmps) > 0 {
		t.Fatalf("expected no temp files after overwrite, got %v", tmps)
	}
}
