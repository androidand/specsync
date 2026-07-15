package specsync

import (
	"path/filepath"
	"testing"
)

// TestValidateStageEmpty rejects empty stage string.
func TestValidateStageEmpty(t *testing.T) {
	if err := ValidateStage(""); err == nil {
		t.Errorf("ValidateStage(\"\") should error")
	}
}

// TestValidateStageDashPrefix rejects stages starting with dash.
func TestValidateStageDashPrefix(t *testing.T) {
	if err := ValidateStage("-invalid"); err == nil {
		t.Errorf("ValidateStage(\"-invalid\") should error")
	}
}

// TestValidateStageUppercase rejects uppercase stages.
func TestValidateStageUppercase(t *testing.T) {
	if err := ValidateStage("Invalid-Case"); err == nil {
		t.Errorf("ValidateStage(\"Invalid-Case\") should error")
	}
	if err := ValidateStage("UPPERCASE"); err == nil {
		t.Errorf("ValidateStage(\"UPPERCASE\") should error")
	}
}

// TestValidateStageSpaces rejects stages with spaces.
func TestValidateStageSpaces(t *testing.T) {
	if err := ValidateStage("with space"); err == nil {
		t.Errorf("ValidateStage(\"with space\") should error")
	}
}

// TestValidateStageSlash rejects stages with slashes.
func TestValidateStageSlash(t *testing.T) {
	if err := ValidateStage("with/slash"); err == nil {
		t.Errorf("ValidateStage(\"with/slash\") should error")
	}
}

// TestValidateStageDots rejects stages with double dots.
func TestValidateStageDots(t *testing.T) {
	if err := ValidateStage("with..dots"); err == nil {
		t.Errorf("ValidateStage(\"with..dots\") should error")
	}
}

// TestValidateStageSpecialChars rejects special characters.
func TestValidateStageSpecialChars(t *testing.T) {
	invalid := []string{
		"with@char",
		"with#char",
		"with$char",
		"with%char",
	}

	for _, stage := range invalid {
		if err := ValidateStage(Stage(stage)); err == nil {
			t.Errorf("ValidateStage(%q) should error", stage)
		}
	}
}

// TestValidateStageTooLong rejects stages longer than 64 characters.
func TestValidateStageTooLong(t *testing.T) {
	longStage := "this-is-a-very-long-stage-name-that-exceeds-the-maximum-allowed-length-of-64-chars"
	if err := ValidateStage(Stage(longStage)); err == nil {
		t.Errorf("ValidateStage (>64 chars) should error")
	}
}

// TestTaskProgressTransitions verifies all task progress states are distinct and valid.
// This replaces 4 noisy constant-value tests with a single property-based test
// that ensures stage transitions work correctly based on task progress.
func TestTaskProgressTransitions(t *testing.T) {
	tests := []struct {
		tasks    string
		wantProg TaskProgress
		wantStage Stage
	}{
		{"", TaskProgressNoTasks, StageActive},                                   // no tasks → active (default)
		{"- [ ] task 1\n- [ ] task 2\n", TaskProgressNotStarted, StageActive},  // 0/N → active
		{"- [x] task 1\n- [ ] task 2\n", TaskProgressInProgress, StageActive},  // 0 < X < N → active
		{"- [x] task 1\n- [x] task 2\n", TaskProgressComplete, StageComplete},  // N/N → complete
	}

	for _, tt := range tests {
		root := t.TempDir()
		cdir := filepath.Join(root, "changes", "test")
		mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n")
		if tt.tasks != "" {
			mustWrite(t, filepath.Join(cdir, "tasks.md"), tt.tasks)
		}

		c, err := LoadChange(cdir, false, root)
		if err != nil {
			t.Fatalf("LoadChange: %v", err)
		}

		if c.Progress != tt.wantProg {
			t.Errorf("tasks %q: progress = %q, want %q", tt.tasks[:20], c.Progress, tt.wantProg)
		}
		if c.Stage != tt.wantStage {
			t.Errorf("tasks %q: stage = %q, want %q", tt.tasks[:20], c.Stage, tt.wantStage)
		}
	}
}

// TestStageSourceConstants verifies all stage source constants are defined.
func TestStageSourceConstants(t *testing.T) {
	sources := []StageSource{
		StageSourceDefault,
		StageSourceTasks,
		StageSourceMetadata,
		StageSourceLegacyStatus,
		StageSourceFolder,
	}

	if len(sources) != 5 {
		t.Errorf("expected 5 stage sources, got %d", len(sources))
	}

	// Verify they're all different
	seen := make(map[StageSource]bool)
	for _, s := range sources {
		if seen[s] {
			t.Errorf("duplicate source: %s", s)
		}
		seen[s] = true
	}
}

// TestArchivedChangeMetadata verifies that archived changes are marked correctly.
func TestArchivedChangeMetadata(t *testing.T) {
	root := t.TempDir()

	// Create archived change
	adir := filepath.Join(root, "changes", "archive", "old-change")
	mustWrite(t, filepath.Join(adir, "proposal.md"), "# Old\n")

	c, err := LoadChange(adir, true, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if !c.Archived {
		t.Errorf("Archived = %v, want true", c.Archived)
	}
	if c.Stage != StageArchived {
		t.Errorf("Stage = %q, want %q", c.Stage, StageArchived)
	}
}

// TestEmptyProposal returns nil for changes without proposal.md.
func TestEmptyProposal(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "missing-proposal")

	// Create directory but no proposal.md
	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c != nil {
		t.Errorf("LoadChange should return nil for missing proposal.md, got %+v", c)
	}
}
