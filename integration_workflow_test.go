package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

// SaveChangeMetadata / LoadChangeMetadata round-trip, and the unset contract:
// clearing one field must never drop the other (regression: `set-stage auto`
// used to delete the whole metadata.json, nuking an explicit priority).
func TestSaveChangeMetadataUnsetKeepsOtherField(t *testing.T) {
	dir := t.TempDir()
	stage := StageInReview

	if err := SaveChangeMetadata(dir, ChangeMetadata{Version: 1, Stage: &stage, Priority: ptr(75)}); err != nil {
		t.Fatalf("SaveChangeMetadata: %v", err)
	}

	// Unset the stage; the explicit priority must survive.
	meta, err := LoadChangeMetadata(dir)
	if err != nil || meta == nil {
		t.Fatalf("LoadChangeMetadata: meta=%v err=%v", meta, err)
	}
	meta.Stage = nil
	if err := SaveChangeMetadata(dir, *meta); err != nil {
		t.Fatalf("SaveChangeMetadata(unset stage): %v", err)
	}

	meta, err = LoadChangeMetadata(dir)
	if err != nil || meta == nil {
		t.Fatalf("reload: meta=%v err=%v", meta, err)
	}
	if meta.Stage != nil {
		t.Errorf("stage = %q, want unset", *meta.Stage)
	}
	if meta.Priority == nil || *meta.Priority != 75 {
		t.Errorf("priority = %v, want 75 (must survive unsetting stage)", meta.Priority)
	}

	// Unsetting the last field removes the file entirely.
	meta.Priority = nil
	if err := SaveChangeMetadata(dir, *meta); err != nil {
		t.Fatalf("SaveChangeMetadata(unset all): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".specsync", "metadata.json")); !os.IsNotExist(err) {
		t.Errorf("metadata.json should be removed when no overrides remain (stat err=%v)", err)
	}
	if meta, err := LoadChangeMetadata(dir); err != nil || meta != nil {
		t.Errorf("LoadChangeMetadata after full unset: meta=%v err=%v, want nil,nil", meta, err)
	}
}

// Archived changes ignore metadata entirely: the folder wins and stays final.
func TestEndToEndArchiveImmutability(t *testing.T) {
	root := t.TempDir()

	adir := filepath.Join(root, "changes", "archive", "old-change")
	mustWrite(t, filepath.Join(adir, "proposal.md"), "# Old\n\nBody\n")
	mustWrite(t, filepath.Join(adir, ".specsync", "metadata.json"),
		`{"version":1,"stage":"active","priority":90}`)

	c, err := LoadChange(adir, true, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c.Stage != StageArchived {
		t.Errorf("archived stage = %q, want %q (immutable)", c.Stage, StageArchived)
	}
	// refreshState returns early for archived changes, so metadata is not read.
	if c.Priority != nil {
		t.Errorf("archived priority = %v, want nil (metadata not processed for archived)", c.Priority)
	}
}

// Old-style changes without any .specsync/ metadata keep working after upgrade.
func TestEndToEndNilPriorityMigration(t *testing.T) {
	root := t.TempDir()

	cdir := filepath.Join(root, "changes", "legacy-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Legacy\n\nBody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] done\n")

	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c.Priority != nil {
		t.Errorf("priority = %v, want nil", c.Priority)
	}
	if c.Stage != StageComplete {
		t.Errorf("stage = %q, want %q (derived from tasks)", c.Stage, StageComplete)
	}
}

// LoadChanges returns both active and archived changes, flagged correctly.
func TestEndToEndLoadChangesIncludesArchived(t *testing.T) {
	root := t.TempDir()

	mustWrite(t, filepath.Join(root, "changes", "active-change", "proposal.md"), "# Active\n")
	mustWrite(t, filepath.Join(root, "changes", "archive", "archived-change", "proposal.md"), "# Archived\n")

	changes, err := LoadChanges(root)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("loaded %d changes, want 2", len(changes))
	}

	bySlug := make(map[string]Change)
	for _, c := range changes {
		bySlug[c.Slug] = c
	}
	if _, ok := bySlug["active-change"]; !ok {
		t.Errorf("active-change not found")
	}
	if a, ok := bySlug["archived-change"]; !ok || !a.Archived {
		t.Errorf("archived-change missing or not flagged archived")
	}
}

// Priority and stage are independent metadata fields: either, both, or neither.
func TestEndToEndMixedMetadata(t *testing.T) {
	root := t.TempDir()

	cases := []struct {
		slug     string
		json     string
		priority *int
		stage    Stage
	}{
		{"priority-only", `{"version":1,"priority":80}`, ptr(80), StageActive},
		{"stage-only", `{"version":1,"stage":"in-review"}`, nil, StageInReview},
		{"both", `{"version":1,"stage":"blocked","priority":50}`, ptr(50), StageBlocked},
	}

	for _, tc := range cases {
		cdir := filepath.Join(root, "changes", tc.slug)
		mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n")
		mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"), tc.json)

		c, err := LoadChange(cdir, false, root)
		if err != nil {
			t.Fatalf("%s: LoadChange: %v", tc.slug, err)
		}
		switch {
		case tc.priority == nil && c.Priority != nil:
			t.Errorf("%s: priority = %d, want nil", tc.slug, *c.Priority)
		case tc.priority != nil && (c.Priority == nil || *c.Priority != *tc.priority):
			t.Errorf("%s: priority = %v, want %d", tc.slug, c.Priority, *tc.priority)
		}
		if c.Stage != tc.stage {
			t.Errorf("%s: stage = %q, want %q", tc.slug, c.Stage, tc.stage)
		}
	}
}

// Task progress derives from the checkbox state of tasks.md.
func TestEndToEndTaskProgressTracking(t *testing.T) {
	tests := []struct {
		name  string
		tasks string
		want  TaskProgress
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
			mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n")
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
