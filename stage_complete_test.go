package specsync

import (
	"context"
	"path/filepath"
	"testing"
)

// TestStageDerivedFromCompletion covers the auto-derived "complete" stage: a
// change whose every task is checked becomes StageComplete, while archiving and
// an explicit .status still take precedence.
func TestStageDerivedFromCompletion(t *testing.T) {
	cases := []struct {
		name     string
		tasks    string
		status   string // .status contents, "" = none
		archived bool
		want     Stage
	}{
		{
			name:  "all tasks checked promotes to complete",
			tasks: "- [x] one\n- [x] two\n",
			want:  StageComplete,
		},
		{
			name:  "a single unchecked task stays active",
			tasks: "- [x] one\n- [ ] two\n",
			want:  StageActive,
		},
		{
			name:  "no task lines stays active",
			tasks: "Some prose with no checkboxes.\n",
			want:  StageActive,
		},
		{
			name:  "empty tasks file stays active",
			tasks: "",
			want:  StageActive,
		},
		{
			name:  "non-task checkbox markers do not count as complete",
			tasks: "- [~] in progress\n- [>] deferred\n",
			want:  StageActive,
		},
		{
			name:   "explicit .status overrides derived completion",
			tasks:  "- [x] one\n",
			status: "blocked",
			want:   Stage("blocked"),
		},
		{
			name:     "archived wins over completion",
			tasks:    "- [x] one\n",
			archived: true,
			want:     StageArchived,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mustWrite(t, filepath.Join(dir, "proposal.md"), "# Title\n\nbody\n")
			if tc.tasks != "" {
				mustWrite(t, filepath.Join(dir, "tasks.md"), tc.tasks)
			}
			if tc.status != "" {
				mustWrite(t, filepath.Join(dir, ".status"), tc.status+"\n")
			}

			c, err := LoadChange(dir, tc.archived, "")
			if err != nil {
				t.Fatalf("LoadChange: %v", err)
			}
			if c == nil {
				t.Fatal("LoadChange returned nil")
			}
			if c.Stage != tc.want {
				t.Errorf("stage = %q, want %q", c.Stage, tc.want)
			}
		})
	}
}

// TestWorkItemForCloseCompleted verifies that a completed change projects as
// closed only when closeCompleted is set, while archived always closes and an
// active change never does.
func TestWorkItemForCloseCompleted(t *testing.T) {
	cases := []struct {
		name           string
		stage          Stage
		archived       bool
		closeCompleted bool
		wantClosed     bool
	}{
		{name: "complete + flag closes", stage: StageComplete, closeCompleted: true, wantClosed: true},
		{name: "complete without flag stays open", stage: StageComplete, closeCompleted: false, wantClosed: false},
		{name: "active + flag stays open", stage: StageActive, closeCompleted: true, wantClosed: false},
		{name: "archived always closes", stage: StageArchived, archived: true, closeCompleted: false, wantClosed: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Change{Slug: "s", Title: "T", Body: "b", Stage: tc.stage, Archived: tc.archived}
			item := WorkItemFor(c, tc.closeCompleted)
			if item.Closed != tc.wantClosed {
				t.Errorf("Closed = %v, want %v", item.Closed, tc.wantClosed)
			}
		})
	}
}

// TestSyncClosesCompletedIssue is an end-to-end check that a real sync with
// CloseCompleted closes the issue for an all-checked change, and labels it
// stage:complete.
func TestSyncClosesCompletedIssue(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "done-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Done\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] only task\n")

	var calls [][]string
	run := func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		if len(args) >= 2 && args[0] == "issue" && args[1] == "create" {
			return "https://github.com/o/r/issues/7", nil
		}
		if len(args) >= 2 && args[0] == "issue" && args[1] == "list" {
			return "[]", nil // no existing issue
		}
		return "", nil
	}
	prov := NewGitHubProviderFunc(run)

	_, err := Sync(context.Background(), Options{
		OpenSpecDir:    root,
		Provider:       prov,
		Slug:           "done-change",
		CloseCompleted: true,
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if createCall := findCall(calls, "issue", "create"); createCall == nil {
		t.Fatal("expected an issue create call")
	} else if !hasLabel(createCall, "stage:complete") {
		t.Errorf("create call missing stage:complete label: %v", createCall)
	}
	if closeCall := findCall(calls, "issue", "close", "7"); closeCall == nil {
		t.Error("expected issue close call for the completed change")
	}
}
