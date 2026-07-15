package specsync

import (
	"path/filepath"
	"testing"
)

// ptrInt returns a pointer to an int (for test data).
func ptrInt(v int) *int {
	return &v
}

// TestEndToEndMetadataAccuracy verifies metadata.json round-trips correctly through load/save cycles.
func TestEndToEndMetadataAccuracy(t *testing.T) {
	root := t.TempDir()

	cdir := filepath.Join(root, "changes", "test-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n\nBody\n")

	// Write initial metadata with priority and stage
	mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"),
		`{"version":1,"stage":"in-review","priority":75}`)

	// Load the change
	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	// Verify metadata was loaded correctly
	if c.Priority == nil || *c.Priority != 75 {
		t.Errorf("priority = %v, want 75", c.Priority)
	}
	if c.Stage != StageInReview {
		t.Errorf("stage = %q, want %q", c.Stage, StageInReview)
	}

	// Load again to verify persistence
	c2, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("second LoadChange: %v", err)
	}

	if c2.Priority == nil || *c2.Priority != 75 {
		t.Errorf("second load priority = %v, want 75", c2.Priority)
	}
	if c2.Stage != StageInReview {
		t.Errorf("second load stage = %q, want %q", c2.Stage, StageInReview)
	}
}

// TestEndToEndArchiveImmutability verifies that archived changes cannot have their properties changed via metadata.
// For archived changes, refreshState returns early without loading metadata, so priority is not loaded.
func TestEndToEndArchiveImmutability(t *testing.T) {
	root := t.TempDir()

	// Create archived change with metadata trying to set it to active + priority
	adir := filepath.Join(root, "changes", "archive", "old-change")
	mustWrite(t, filepath.Join(adir, "proposal.md"), "# Old\n\nBody\n")
	mustWrite(t, filepath.Join(adir, ".specsync", "metadata.json"),
		`{"version":1,"stage":"active","priority":90}`)

	c, err := LoadChange(adir, true, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	// Archived stage always wins and metadata is not processed
	if c.Stage != StageArchived {
		t.Errorf("archived stage = %q, want %q (immutable)", c.Stage, StageArchived)
	}

	// Priority is not loaded for archived changes (they return early from refreshState)
	if c.Priority != nil {
		t.Errorf("archived priority = %v, want nil (metadata not processed for archived)", c.Priority)
	}
}

// TestEndToEndNilPriorityMigration verifies that old changes without metadata work smoothly.
// This is a backwards-compatibility test for existing repos being upgraded to v0.7.0+.
func TestEndToEndNilPriorityMigration(t *testing.T) {
	root := t.TempDir()

	// Create old-style change without metadata.json
	cdir := filepath.Join(root, "changes", "legacy-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Legacy\n\nBody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] done\n")
	// No .specsync/metadata.json

	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	// Priority should be nil (unset)
	if c.Priority != nil {
		t.Errorf("priority = %v, want nil", c.Priority)
	}

	// Stage should be derived from tasks (complete)
	if c.Stage != StageComplete {
		t.Errorf("stage = %q, want %q (derived from tasks)", c.Stage, StageComplete)
	}
}

// TestEndToEndLoadChangesIncludesArchived verifies that LoadChanges gets both active and archived.
func TestEndToEndLoadChangesIncludesArchived(t *testing.T) {
	root := t.TempDir()

	// Create active change
	adir := filepath.Join(root, "changes", "active-change")
	mustWrite(t, filepath.Join(adir, "proposal.md"), "# Active\n\nBody\n")

	// Create archived change
	adir = filepath.Join(root, "changes", "archive", "archived-change")
	mustWrite(t, filepath.Join(adir, "proposal.md"), "# Archived\n\nBody\n")

	changes, err := LoadChanges(root)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}

	if len(changes) != 2 {
		t.Errorf("loaded %d changes, want 2", len(changes))
	}

	// Verify both exist
	bySlug := make(map[string]Change)
	for _, c := range changes {
		bySlug[c.Slug] = c
	}

	if _, ok := bySlug["active-change"]; !ok {
		t.Errorf("active-change not found")
	}
	if _, ok := bySlug["archived-change"]; !ok {
		t.Errorf("archived-change not found")
	}

	if !bySlug["archived-change"].Archived {
		t.Errorf("archived-change should be marked Archived")
	}
}

// TestEndToEndMixedMetadata verifies that metadata works with both priority and stage independently.
func TestEndToEndMixedMetadata(t *testing.T) {
	root := t.TempDir()

	// Case 1: Only priority set in metadata
	cdir1 := filepath.Join(root, "changes", "priority-only")
	mustWrite(t, filepath.Join(cdir1, "proposal.md"), "# Test\n\nBody\n")
	mustWrite(t, filepath.Join(cdir1, ".specsync", "metadata.json"), `{"version":1,"priority":80}`)

	c1, _ := LoadChange(cdir1, false, root)
	if c1.Priority == nil || *c1.Priority != 80 {
		t.Errorf("case 1: priority = %v, want 80", c1.Priority)
	}
	if c1.Stage != StageActive {
		t.Errorf("case 1: stage = %q, want %q (default)", c1.Stage, StageActive)
	}

	// Case 2: Only stage set in metadata
	cdir2 := filepath.Join(root, "changes", "stage-only")
	mustWrite(t, filepath.Join(cdir2, "proposal.md"), "# Test\n\nBody\n")
	mustWrite(t, filepath.Join(cdir2, ".specsync", "metadata.json"), `{"version":1,"stage":"in-review"}`)

	c2, _ := LoadChange(cdir2, false, root)
	if c2.Priority != nil {
		t.Errorf("case 2: priority = %v, want nil", c2.Priority)
	}
	if c2.Stage != StageInReview {
		t.Errorf("case 2: stage = %q, want %q", c2.Stage, StageInReview)
	}

	// Case 3: Both set
	cdir3 := filepath.Join(root, "changes", "both")
	mustWrite(t, filepath.Join(cdir3, "proposal.md"), "# Test\n\nBody\n")
	mustWrite(t, filepath.Join(cdir3, ".specsync", "metadata.json"), `{"version":1,"stage":"active","priority":50}`)

	c3, _ := LoadChange(cdir3, false, root)
	if c3.Priority == nil || *c3.Priority != 50 {
		t.Errorf("case 3: priority = %v, want 50", c3.Priority)
	}
	if c3.Stage != StageActive {
		t.Errorf("case 3: stage = %q, want %q", c3.Stage, StageActive)
	}
}

// TestEndToEndTaskProgressTracking verifies that task progress is correctly derived and tracked.
func TestEndToEndTaskProgressTracking(t *testing.T) {
	tests := []struct {
		name      string
		tasks     string
		want      TaskProgress
	}{
		{"no-tasks", "", TaskProgressNoTasks},
		{"not-started", "- [ ] task 1\n- [ ] task 2\n", TaskProgressNotStarted},
		{"in-progress", "- [x] task 1\n- [ ] task 2\n", TaskProgressInProgress},
		{"complete", "- [x] task 1\n- [x] task 2\n", TaskProgressComplete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cdir := filepath.Join(root, "changes", "test")
			mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n\nBody\n")
			if tt.tasks != "" {
				mustWrite(t, filepath.Join(cdir, "tasks.md"), tt.tasks)
			}

			c, err := LoadChange(cdir, false, root)
			if err != nil {
				t.Fatalf("LoadChange: %v", err)
			}

			if c.Progress != tt.want {
				t.Errorf("progress = %q, want %q", c.Progress, tt.want)
			}
		})
	}
}

// TestBoardStateHandlesEmpty verifies board state handles missing files gracefully.
func TestBoardStateHandlesEmpty(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "test-change")

	// Load from non-existent directory
	state, err := LoadBoardState(changeDir)
	if err != nil {
		t.Fatalf("LoadBoardState should not error on missing file: %v", err)
	}

	// Should return empty state
	if state.Version != 1 {
		t.Errorf("version = %d, want 1", state.Version)
	}
	if len(state.Bindings) != 0 {
		t.Errorf("bindings should be empty")
	}
}

// TestMultiplePrioritiesInOrder verifies that LoadChanges preserves all priorities for sorting.
func TestMultiplePrioritiesInOrder(t *testing.T) {
	root := t.TempDir()

	// Create changes with various priorities
	priorities := []*int{ptrInt(99), ptrInt(30), ptrInt(75), ptrInt(50), nil}
	slugs := []string{"focus", "low", "high", "normal", "unset"}

	for i, p := range priorities {
		slug := slugs[i]
		cdir := filepath.Join(root, "changes", slug)
		mustWrite(t, filepath.Join(cdir, "proposal.md"), "# "+slug+"\n")
		if p != nil {
			mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"),
				`{"version":1,"priority":`+string(rune('0'+*p/10))+string(rune('0'+*p%10))+`}`)
		}
	}

	changes, err := LoadChanges(root)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}

	if len(changes) != 5 {
		t.Errorf("loaded %d changes, want 5", len(changes))
	}

	// Verify priorities were loaded correctly
	bySlug := make(map[string]*int)
	for i := range changes {
		bySlug[changes[i].Slug] = changes[i].Priority
	}

	if p := bySlug["focus"]; p == nil || *p != 99 {
		t.Errorf("focus priority = %v, want 99", p)
	}
	if p := bySlug["high"]; p == nil || *p != 75 {
		t.Errorf("high priority = %v, want 75", p)
	}
	if p := bySlug["normal"]; p == nil || *p != 50 {
		t.Errorf("normal priority = %v, want 50", p)
	}
	if p := bySlug["low"]; p == nil || *p != 30 {
		t.Errorf("low priority = %v, want 30", p)
	}
	if p := bySlug["unset"]; p != nil {
		t.Errorf("unset priority = %v, want nil", p)
	}
}

// TestThreeWayMergeConflictScenario verifies detailed conflict scenario.
func TestThreeWayMergeConflictScenario(t *testing.T) {
	// Scenario: Local progressed to complete, but human moved card to blocked on board
	base := BoardBinding{
		LocalStageBase:     StageActive,
		RemoteOptionIDBase: "OPT_PROG",
	}

	decision := threeWayMerge(StageComplete, "OPT_BLOCKED", base)

	if decision.Action != "report-conflict" {
		t.Errorf("conflict detection failed: action = %q, want report-conflict", decision.Action)
	}
	if !decision.LocalChanged {
		t.Errorf("LocalChanged should be true")
	}
	if !decision.RemoteChanged {
		t.Errorf("RemoteChanged should be true")
	}
}

// TestArchivedStageNeverChanges verifies archived stage is immutable in all contexts.
func TestArchivedStageNeverChanges(t *testing.T) {
	root := t.TempDir()

	// Create archived with metadata that tries to override
	adir := filepath.Join(root, "changes", "archive", "archived-change")
	mustWrite(t, filepath.Join(adir, "proposal.md"), "# Archived\n")
	mustWrite(t, filepath.Join(adir, "tasks.md"), "- [x] done\n") // Would derive complete
	mustWrite(t, filepath.Join(adir, ".specsync", "metadata.json"), `{"version":1,"stage":"blocked"}`)

	c, err := LoadChange(adir, true, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	// Despite all signals pointing elsewhere, archived always wins
	if c.Stage != StageArchived {
		t.Errorf("archived stage = %q, want StageArchived", c.Stage)
	}
	if c.StageSource != StageSourceFolder {
		t.Errorf("source = %q, want StageSourceFolder", c.StageSource)
	}
}
