package spec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/androidand/specsync"
)

func TestFileSpecSource_Name(t *testing.T) {
	s := FileSpecSource{}
	if s.Name() != "openspec" {
		t.Errorf("Name() = %q, want %q", s.Name(), "openspec")
	}
}

func TestFileSpecSource_LoadChanges(t *testing.T) {
	s := FileSpecSource{}

	// Create a temporary openspec directory with a change.
	tmp := t.TempDir()
	changesDir := filepath.Join(tmp, "changes")
	changeDir := filepath.Join(changesDir, "test-change")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test Change\n\nBody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changes, err := s.LoadChanges(tmp)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("LoadChanges: got %d changes, want 1", len(changes))
	}

	if changes[0].Slug != "test-change" {
		t.Errorf("Slug = %q, want %q", changes[0].Slug, "test-change")
	}
	if changes[0].Title != "Test Change" {
		t.Errorf("Title = %q, want %q", changes[0].Title, "Test Change")
	}
}

func TestFileSpecSource_LoadChanges_Empty(t *testing.T) {
	s := FileSpecSource{}

	tmp := t.TempDir()
	changesDir := filepath.Join(tmp, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	changes, err := s.LoadChanges(tmp)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("LoadChanges: got %d changes, want 0", len(changes))
	}
}

func TestFileSpecSource_LoadChanges_MissingDir(t *testing.T) {
	s := FileSpecSource{}

	// Non-existent directory should return empty slice, not error.
	changes, err := s.LoadChanges("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("LoadChanges: unexpected error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("LoadChanges: got %d changes, want 0", len(changes))
	}
}

func TestFileSpecSource_SaveChange(t *testing.T) {
	s := FileSpecSource{}
	// SaveChange is a no-op; should not error.
	err := s.SaveChange(specsync.Change{})
	if err != nil {
		t.Errorf("SaveChange: unexpected error: %v", err)
	}
}

func TestBeadsSource_Name(t *testing.T) {
	s := BeadsSource{}
	if s.Name() != "beads" {
		t.Errorf("Name() = %q, want %q", s.Name(), "beads")
	}
}

func TestBeadsSource_NotImplemented(t *testing.T) {
	s := BeadsSource{}

	_, err := s.LoadChanges("/tmp")
	if err == nil {
		t.Fatal("LoadChanges: expected error, got nil")
	}
	if err.Error() != "beads support not yet implemented" {
		t.Errorf("LoadChanges error = %q, want %q", err.Error(), "beads support not yet implemented")
	}

	err = s.SaveChange(specsync.Change{})
	if err == nil {
		t.Fatal("SaveChange: expected error, got nil")
	}
	if err.Error() != "beads support not yet implemented" {
		t.Errorf("SaveChange error = %q, want %q", err.Error(), "beads support not yet implemented")
	}
}

func TestInterfaceCompliance(t *testing.T) {
	// Verify both implementations satisfy specsync.SpecSource.
	var _ specsync.SpecSource = FileSpecSource{}
	var _ specsync.SpecSource = BeadsSource{}
}

func TestNewFactoryFunctions(t *testing.T) {
	fs := NewFileSpecSource()
	if fs.Name() != "openspec" {
		t.Errorf("NewFileSpecSource().Name() = %q, want %q", fs.Name(), "openspec")
	}

	bs := NewBeadsSource()
	if bs.Name() != "beads" {
		t.Errorf("NewBeadsSource().Name() = %q, want %q", bs.Name(), "beads")
	}

	// Verify factory results implement SpecSource.
	var _ specsync.SpecSource = fs
	var _ specsync.SpecSource = bs
}
