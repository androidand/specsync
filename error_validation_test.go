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

// TestTaskProgressNoTasks verifies the no-tasks constant.
func TestTaskProgressNoTasks(t *testing.T) {
	if TaskProgressNoTasks != "no-tasks" {
		t.Errorf("TaskProgressNoTasks = %q, want %q", TaskProgressNoTasks, "no-tasks")
	}
}

// TestTaskProgressNotStarted verifies the not-started constant.
func TestTaskProgressNotStarted(t *testing.T) {
	if TaskProgressNotStarted != "not-started" {
		t.Errorf("TaskProgressNotStarted = %q, want %q", TaskProgressNotStarted, "not-started")
	}
}

// TestTaskProgressInProgress verifies the in-progress constant.
func TestTaskProgressInProgress(t *testing.T) {
	if TaskProgressInProgress != "in-progress" {
		t.Errorf("TaskProgressInProgress = %q, want %q", TaskProgressInProgress, "in-progress")
	}
}

// TestTaskProgressComplete verifies the complete constant.
func TestTaskProgressComplete(t *testing.T) {
	if TaskProgressComplete != "complete" {
		t.Errorf("TaskProgressComplete = %q, want %q", TaskProgressComplete, "complete")
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
