package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateChanges_AllValid(t *testing.T) {
	tmpDir := t.TempDir()
	openspecDir := filepath.Join(tmpDir, "openspec")
	changesDir := filepath.Join(openspecDir, "changes")
	if err := os.MkdirAll(filepath.Join(changesDir, "good-change"), 0755); err != nil {
		t.Fatal(err)
	}
	// Create required files.
	if err := os.WriteFile(filepath.Join(changesDir, "good-change", "proposal.md"), []byte("# Good"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "good-change", "tasks.md"), []byte("# Tasks\n\n- [ ] task"), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateChanges(openspecDir)
	if len(result.Issues) > 0 {
		t.Fatalf("expected no issues, got %d: %v", len(result.Issues), result.Issues)
	}
}

func TestValidateChanges_MissingProposal(t *testing.T) {
	tmpDir := t.TempDir()
	openspecDir := filepath.Join(tmpDir, "openspec")
	changesDir := filepath.Join(openspecDir, "changes")
	if err := os.MkdirAll(filepath.Join(changesDir, "no-proposal"), 0755); err != nil {
		t.Fatal(err)
	}
	// Only tasks.md, no proposal.md.
	if err := os.WriteFile(filepath.Join(changesDir, "no-proposal", "tasks.md"), []byte("# Tasks"), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateChanges(openspecDir)
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Field != "proposal.md" {
		t.Fatalf("expected proposal.md issue, got %s", result.Issues[0].Field)
	}
}

func TestValidateChanges_MissingTasks(t *testing.T) {
	tmpDir := t.TempDir()
	openspecDir := filepath.Join(tmpDir, "openspec")
	changesDir := filepath.Join(openspecDir, "changes")
	if err := os.MkdirAll(filepath.Join(changesDir, "no-tasks"), 0755); err != nil {
		t.Fatal(err)
	}
	// Only proposal.md, no tasks.md.
	if err := os.WriteFile(filepath.Join(changesDir, "no-tasks", "proposal.md"), []byte("# No Tasks"), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateChanges(openspecDir)
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Field != "tasks.md" {
		t.Fatalf("expected tasks.md issue, got %s", result.Issues[0].Field)
	}
}

func TestValidateChanges_InvalidMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	openspecDir := filepath.Join(tmpDir, "openspec")
	changesDir := filepath.Join(openspecDir, "changes")
	changeDir := filepath.Join(changesDir, "bad-meta")
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Bad Meta"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("# Tasks"), 0644); err != nil {
		t.Fatal(err)
	}
	// Write invalid JSON.
	if err := os.WriteFile(filepath.Join(changeDir, ".specsync", "metadata.json"), []byte("{bad"), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateChanges(openspecDir)
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %v", len(result.Issues), result.Issues)
	}
	if result.Issues[0].Field != ".specsync/metadata.json" {
		t.Fatalf("expected metadata.json issue, got %s", result.Issues[0].Field)
	}
}

func TestValidateChanges_InvalidStage(t *testing.T) {
	tmpDir := t.TempDir()
	openspecDir := filepath.Join(tmpDir, "openspec")
	changesDir := filepath.Join(openspecDir, "changes")
	changeDir := filepath.Join(changesDir, "bad-stage")
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Bad Stage"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("# Tasks"), 0644); err != nil {
		t.Fatal(err)
	}
	// Write metadata with invalid stage.
	if err := os.WriteFile(filepath.Join(changeDir, ".specsync", "metadata.json"), []byte(`{"stage": "INVALID_STAGE"}`), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateChanges(openspecDir)
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %v", len(result.Issues), result.Issues)
	}
	if result.Issues[0].Field != ".specsync/metadata.json.stage" {
		t.Fatalf("expected metadata.json.stage issue, got %s", result.Issues[0].Field)
	}
}

func TestValidateChanges_ArchivedChanges(t *testing.T) {
	tmpDir := t.TempDir()
	openspecDir := filepath.Join(tmpDir, "openspec")
	changesDir := filepath.Join(openspecDir, "changes")
	archiveDir := filepath.Join(changesDir, "archive")
	if err := os.MkdirAll(filepath.Join(archiveDir, "archived-change"), 0755); err != nil {
		t.Fatal(err)
	}
	// Archived change without tasks.md.
	if err := os.WriteFile(filepath.Join(archiveDir, "archived-change", "proposal.md"), []byte("# Archived"), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateChanges(openspecDir)
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Slug != "archived-change" {
		t.Fatalf("expected archived-change, got %s", result.Issues[0].Slug)
	}
}

func TestValidateChanges_MultipleIssues(t *testing.T) {
	tmpDir := t.TempDir()
	openspecDir := filepath.Join(tmpDir, "openspec")
	changesDir := filepath.Join(openspecDir, "changes")

	// Change 1: missing both files.
	change1 := filepath.Join(changesDir, "empty-change")
	if err := os.MkdirAll(change1, 0755); err != nil {
		t.Fatal(err)
	}

	// Change 2: missing tasks.md.
	change2 := filepath.Join(changesDir, "no-tasks-change")
	if err := os.MkdirAll(change2, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(change2, "proposal.md"), []byte("# No Tasks"), 0644); err != nil {
		t.Fatal(err)
	}

	result := ValidateChanges(openspecDir)
	if len(result.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d: %v", len(result.Issues), result.Issues)
	}
}
