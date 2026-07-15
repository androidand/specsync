package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/androidand/specsync"
)

func TestMigrateAutoPrioritize(t *testing.T) {
	root := t.TempDir()

	// Create test changes with different task counts.
	changes := []specsync.Change{
		{
			Slug: "feature-a",
			Dir:  filepath.Join(root, "changes", "feature-a"),
			// Will have 5 tasks
		},
		{
			Slug: "feature-b",
			Dir:  filepath.Join(root, "changes", "feature-b"),
			// Will have 10 tasks (higher = higher priority)
		},
		{
			Slug: "feature-c",
			Dir:  filepath.Join(root, "changes", "feature-c"),
			// Will have 2 tasks
		},
	}

	// Create directories and metadata files (mocked).
	for _, c := range changes {
		if err := os.MkdirAll(filepath.Join(c.Dir, ".specsync"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	// Run migration (dry-run).
	err := migrateAutoPrioritize(changes, true)
	if err != nil {
		t.Fatalf("migrateAutoPrioritize: %v", err)
	}

	// Verify no files written in dry-run.
	for _, c := range changes {
		metaPath := filepath.Join(c.Dir, ".specsync", "metadata.json")
		if _, err := os.Stat(metaPath); err == nil {
			t.Errorf("dry-run wrote file: %s", metaPath)
		}
	}
}

func TestMigrateClear(t *testing.T) {
	root := t.TempDir()

	changes := []specsync.Change{
		{
			Slug: "feature-a",
			Dir:  filepath.Join(root, "changes", "feature-a"),
		},
		{
			Slug: "feature-b",
			Dir:  filepath.Join(root, "changes", "feature-b"),
		},
	}

	for _, c := range changes {
		if err := os.MkdirAll(filepath.Join(c.Dir, ".specsync"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	// Run migration (dry-run).
	err := migrateClear(changes, true)
	if err != nil {
		t.Fatalf("migrateClear: %v", err)
	}

	// Verify no files written.
	for _, c := range changes {
		metaPath := filepath.Join(c.Dir, ".specsync", "metadata.json")
		if _, err := os.Stat(metaPath); err == nil {
			t.Errorf("dry-run wrote file: %s", metaPath)
		}
	}
}

func TestEstimatePriority(t *testing.T) {
	tests := []struct {
		position int
		total    int
		wantMin  int
		wantMax  int
	}{
		{0, 100, 80, 99},       // Top 10%
		{5, 100, 80, 99},       // Top 10%
		{15, 100, 50, 79},      // Next 30%
		{50, 100, 30, 49},      // Next 30%
		{75, 100, 1, 29},       // Last 30%
		{99, 100, 1, 29},       // Last 30%
	}

	for _, tt := range tests {
		got := estimatePriority(tt.position, tt.total)
		if got < tt.wantMin || got > tt.wantMax {
			t.Errorf("estimatePriority(%d, %d) = %d, want [%d, %d]",
				tt.position, tt.total, got, tt.wantMin, tt.wantMax)
		}
	}
}
