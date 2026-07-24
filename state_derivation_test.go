package specsync

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestStagePrecedenceArchived verifies that archived stage always takes highest precedence.
// Archived changes are immutable and cannot be overridden by metadata or any other source.
func TestStagePrecedenceArchived(t *testing.T) {
	root := t.TempDir()

	// Create archived change with metadata trying to set it to backlog
	adir := filepath.Join(root, "changes", "archive", "old-change")
	mustWrite(t, filepath.Join(adir, "proposal.md"), "# Old Title\n\nBody\n")
	mustWrite(t, filepath.Join(adir, ".specsync", "metadata.json"), `{"version":1,"stage":"backlog"}`)

	c, err := LoadChange(adir, true, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c.Stage != StageArchived {
		t.Errorf("archived change stage = %q, want %q (archived always wins)", c.Stage, StageArchived)
	}
	if c.StageSource != StageSourceFolder {
		t.Errorf("archived change source = %q, want %q", c.StageSource, StageSourceFolder)
	}
}

// TestStagePrecedenceMetadata verifies that explicit metadata.stage overrides derived.
func TestStagePrecedenceMetadata(t *testing.T) {
	root := t.TempDir()

	// Create change with all tasks complete (would derive to complete)
	// but metadata says in-review (should win)
	cdir := filepath.Join(root, "changes", "test-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n\nBody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] task 1\n- [x] task 2\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"), `{"version":1,"stage":"in-review"}`)

	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c.Stage != StageInReview {
		t.Errorf("metadata stage = %q, want %q (metadata should override tasks)", c.Stage, StageInReview)
	}
	if c.StageSource != StageSourceMetadata {
		t.Errorf("source = %q, want %q", c.StageSource, StageSourceMetadata)
	}
}

// TestStagePrecedenceDerived verifies that task completion is used when no metadata/legacy.
func TestStagePrecedenceDerived(t *testing.T) {
	root := t.TempDir()

	cdir := filepath.Join(root, "changes", "test-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n\nBody\n")
	// All tasks complete, no metadata or legacy
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] task 1\n- [x] task 2\n")

	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c.Stage != StageComplete {
		t.Errorf("derived stage from tasks = %q, want %q", c.Stage, StageComplete)
	}
	if c.StageSource != StageSourceTasks {
		t.Errorf("source = %q, want %q", c.StageSource, StageSourceTasks)
	}
}

// TestStagePrecedenceInProgress verifies that partial tasks don't override default.
// Only 100% complete tasks derive a stage; partial progress uses default (active).
func TestStagePrecedenceInProgress(t *testing.T) {
	root := t.TempDir()

	cdir := filepath.Join(root, "changes", "test-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n\nBody\n")
	// Some tasks done, some not - should not override default
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] task 1\n- [ ] task 2\n")

	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c.Stage != StageActive {
		t.Errorf("stage with partial tasks = %q, want %q", c.Stage, StageActive)
	}
	if c.StageSource != StageSourceDefault {
		t.Errorf("source = %q, want %q (partial tasks don't derive, only 100%% complete does)", c.StageSource, StageSourceDefault)
	}
	if c.Progress != TaskProgressInProgress {
		t.Errorf("progress = %q, want %q", c.Progress, TaskProgressInProgress)
	}
}

// TestStagePrecedenceDefault verifies active as default when no other source.
func TestStagePrecedenceDefault(t *testing.T) {
	root := t.TempDir()

	cdir := filepath.Join(root, "changes", "test-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n\nBody\n")
	// No tasks, no metadata, no legacy

	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c.Stage != StageActive {
		t.Errorf("default stage = %q, want %q", c.Stage, StageActive)
	}
	if c.StageSource != StageSourceDefault {
		t.Errorf("source = %q, want %q", c.StageSource, StageSourceDefault)
	}
}

// TestPriorityMetadataLoading verifies that priority from metadata.json is loaded correctly.
// TestEmptyProposal: a change folder without proposal.md is skipped, not an error.
func TestEmptyProposal(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "missing-proposal")

	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}
	if c != nil {
		t.Errorf("LoadChange should return nil for missing proposal.md, got %+v", c)
	}
}

// TestValidateStageCanonical verifies all canonical stages are accepted.
func TestValidateStageCanonical(t *testing.T) {
	canonical := []Stage{
		StageBacklog, StageBlocked, StageActive, StageInReview, StageComplete, StageArchived,
	}

	for _, stage := range canonical {
		if err := ValidateStage(stage); err != nil {
			t.Errorf("ValidateStage(%q) = %v; want no error", stage, err)
		}
	}
}

// TestValidateStageCustomValid verifies custom stages with valid format are accepted.
func TestValidateStageCustomValid(t *testing.T) {
	valid := []Stage{
		"custom-stage",
		"awaiting-review",
		"in-testing",
		"a",
		"a1b2c3",
		"ready-to-ship",
	}

	for _, stage := range valid {
		if err := ValidateStage(stage); err != nil {
			t.Errorf("ValidateStage(%q) = %v; want no error", stage, err)
		}
	}
}

// TestValidateStageCustomInvalid verifies invalid stage formats are rejected.
func TestValidateStageCustomInvalid(t *testing.T) {
	invalid := []Stage{
		"",
		"-invalid",
		"Invalid-Case",
		"UPPERCASE",
		"with space",
		"with/slash",
		"with..dots",
		"with@char",
		Stage("way-too-long-" + strings.Repeat("x", 64)),
	}

	for _, stage := range invalid {
		if err := ValidateStage(stage); err == nil {
			t.Errorf("ValidateStage(%q) = nil; want error", stage)
		}
	}
}

// TestIsCanonicalStage verifies the canonical check function.
func TestIsCanonicalStage(t *testing.T) {
	tests := []struct {
		stage    Stage
		wantTrue bool
	}{
		{StageBacklog, true},
		{StageBlocked, true},
		{StageActive, true},
		{StageInReview, true},
		{StageComplete, true},
		{StageArchived, true},
		{"custom-stage", false},
		{"awaiting-review", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsCanonicalStage(tt.stage)
		if got != tt.wantTrue {
			t.Errorf("IsCanonicalStage(%q) = %v, want %v", tt.stage, got, tt.wantTrue)
		}
	}
}

// TestCanonicalStageOrder verifies the stage ordering.
func TestCanonicalStageOrder(t *testing.T) {
	order := CanonicalStageOrder()
	expected := []Stage{
		StageBacklog, StageBlocked, StageActive, StageInReview, StageComplete, StageArchived, StageShipped,
	}

	if len(order) != len(expected) {
		t.Errorf("CanonicalStageOrder() returned %d stages, want %d", len(order), len(expected))
	}

	for i, stage := range order {
		if i < len(expected) && stage != expected[i] {
			t.Errorf("CanonicalStageOrder()[%d] = %q, want %q", i, stage, expected[i])
		}
	}
}
