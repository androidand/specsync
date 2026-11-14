package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSaveChangeMetadataRefusesMissingDir: the writer is the backstop for #207.
// Callers validate the change first, but a caller that does not must get an
// error, not a freshly created change directory.
func TestSaveChangeMetadataRefusesMissingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "openspec", "changes", "no-such-change")
	stage := StageActive
	if err := SaveChangeMetadata(missing, ChangeMetadata{Stage: &stage}); err == nil {
		t.Fatal("SaveChangeMetadata succeeded for a directory that does not exist")
	}
	if _, err := os.Stat(missing); err == nil {
		t.Errorf("SaveChangeMetadata created %s", missing)
	}
}

func TestSaveChangeMetadataWritesIntoExistingDir(t *testing.T) {
	dir := t.TempDir()
	stage := StageActive
	if err := SaveChangeMetadata(dir, ChangeMetadata{Stage: &stage}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".specsync", "metadata.json")); err != nil {
		t.Errorf("metadata not written: %v", err)
	}
}
