package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpinoffFromFreeText(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openspec := filepath.Join(dir, "openspec")
	parentDir := filepath.Join(openspec, "changes", "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(parentDir, "proposal.md"), []byte("# Parent\n"), 0o644)
	os.WriteFile(filepath.Join(parentDir, "tasks.md"), []byte("- [x] task one\n- [ ] task two\n"), 0o644)

	res, err := Spinoff(context.Background(), SpinoffOptions{
		OpenSpecDir: openspec,
		Parent:      "parent",
		Text:        "rate limiter drops bursts",
		DryRun:      false,
	})
	if err != nil {
		t.Fatal(err)
	}

	if res.ChildSlug != "rate-limiter-drops-bursts" {
		t.Errorf("slug = %q; want rate-limiter-drops-bursts", res.ChildSlug)
	}

	proposal, err := os.ReadFile(filepath.Join(res.ChildDir, "proposal.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proposal), "rate limiter drops bursts") {
		t.Error("proposal missing discovery text")
	}
	if !strings.Contains(string(proposal), "Spun off from `parent`") {
		t.Error("proposal missing provenance")
	}

	tasks, err := os.ReadFile(filepath.Join(res.ChildDir, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(tasks) != "- [ ] TODO\n" {
		t.Errorf("tasks = %q; want - [ ] TODO\\n", string(tasks))
	}

	// Parent tasks should be unchanged (no task index given).
	parentTasks, err := os.ReadFile(filepath.Join(parentDir, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(parentTasks), "- [x] task one") {
		t.Error("parent task one should be unchanged")
	}
}

func TestSpinoffFromTaskIndex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openspec := filepath.Join(dir, "openspec")
	parentDir := filepath.Join(openspec, "changes", "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(parentDir, "proposal.md"), []byte("# Parent\n"), 0o644)
	os.WriteFile(filepath.Join(parentDir, "tasks.md"), []byte("- [x] task one\n- [ ] task two\n- [ ] task three\n"), 0o644)

	res, err := Spinoff(context.Background(), SpinoffOptions{
		OpenSpecDir: openspec,
		Parent:      "parent",
		TaskIndex:   2,
		DryRun:      false,
	})
	if err != nil {
		t.Fatal(err)
	}

	if res.ChildSlug != "task-two" {
		t.Errorf("slug = %q; want task-two", res.ChildSlug)
	}
	if res.ParentTaskN != 2 {
		t.Errorf("ParentTaskN = %d; want 2", res.ParentTaskN)
	}

	// Parent task 2 should be marked as moved.
	parentTasks, err := os.ReadFile(filepath.Join(parentDir, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(parentTasks), "- [>] moved: task-two") {
		t.Errorf("parent task not marked as moved: %s", parentTasks)
	}
	if !strings.Contains(string(parentTasks), "- [x] task one") {
		t.Error("parent task one should be unchanged")
	}
}

func TestSpinoffTaskIndexOutOfRange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openspec := filepath.Join(dir, "openspec")
	parentDir := filepath.Join(openspec, "changes", "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(parentDir, "proposal.md"), []byte("# Parent\n"), 0o644)
	os.WriteFile(filepath.Join(parentDir, "tasks.md"), []byte("- [ ] task one\n"), 0o644)

	_, err := Spinoff(context.Background(), SpinoffOptions{
		OpenSpecDir: openspec,
		Parent:      "parent",
		TaskIndex:   5,
	})
	if err == nil {
		t.Fatal("expected error for out-of-range task index")
	}
}

func TestSpinoffChildAlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openspec := filepath.Join(dir, "openspec")
	parentDir := filepath.Join(openspec, "changes", "parent")
	childDir := filepath.Join(openspec, "changes", "existing")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(parentDir, "proposal.md"), []byte("# Parent\n"), 0o644)
	os.WriteFile(filepath.Join(parentDir, "tasks.md"), []byte("- [ ] task one\n"), 0o644)

	_, err := Spinoff(context.Background(), SpinoffOptions{
		OpenSpecDir: openspec,
		Parent:      "parent",
		Text:        "existing",
		Slug:        "existing",
	})
	if err == nil {
		t.Fatal("expected error for existing child change")
	}
}

func TestSpinoffDryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openspec := filepath.Join(dir, "openspec")
	parentDir := filepath.Join(openspec, "changes", "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(parentDir, "proposal.md"), []byte("# Parent\n"), 0o644)
	os.WriteFile(filepath.Join(parentDir, "tasks.md"), []byte("- [ ] task one\n"), 0o644)

	res, err := Spinoff(context.Background(), SpinoffOptions{
		OpenSpecDir: openspec,
		Parent:      "parent",
		Text:        "new feature idea",
		DryRun:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Child dir should NOT exist on dry run.
	if _, err := os.Stat(res.ChildDir); !os.IsNotExist(err) {
		t.Error("child dir should not exist on dry run")
	}

	// Parent tasks should be unchanged.
	parentTasks, err := os.ReadFile(filepath.Join(parentDir, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(parentTasks), "- [ ] task one") {
		t.Error("parent task should be unchanged on dry run")
	}
}

func TestSpinoffWithParentURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openspec := filepath.Join(dir, "openspec")
	parentDir := filepath.Join(openspec, "changes", "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(parentDir, "proposal.md"), []byte("# Parent\n"), 0o644)
	os.WriteFile(filepath.Join(parentDir, "tasks.md"), []byte("- [ ] task one\n"), 0o644)

	// Set up a ref for the parent.
	refDir := filepath.Join(parentDir, ".specsync")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatal(err)
	}
	refData := `{"github":{"provider":"github","id":"42","url":"https://github.com/test/repo/issues/42"}}`
	os.WriteFile(filepath.Join(refDir, "refs.json"), []byte(refData), 0o644)

	res, err := Spinoff(context.Background(), SpinoffOptions{
		OpenSpecDir: openspec,
		Parent:      "parent",
		Text:        "spun off bug",
		DryRun:      false,
	})
	if err != nil {
		t.Fatal(err)
	}

	proposal, err := os.ReadFile(filepath.Join(res.ChildDir, "proposal.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proposal), "Spun off from [parent](https://github.com/test/repo/issues/42)") {
		t.Errorf("proposal missing provenance link: %s", proposal)
	}

	// Child should have a link to the parent.
	links, err := os.ReadFile(filepath.Join(res.ChildDir, "links.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(links), "test/repo#42") {
		t.Errorf("child links missing parent ref: %s", links)
	}
}

func TestKindLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind string
		want string
	}{
		{"bug", "kind:bug"},
		{"followup", "kind:followup"},
		{"task", "kind:task"},
		{"Bug", "kind:bug"},
		{"", ""},
		{"unknown", ""},
	}

	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			if got := kindLabel(tc.kind); got != tc.want {
				t.Errorf("kindLabel(%q) = %q; want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestSpinoffNoParent(t *testing.T) {
	t.Parallel()

	_, err := Spinoff(context.Background(), SpinoffOptions{
		OpenSpecDir: "/tmp",
		Parent:      "",
		Text:        "some text",
	})
	if err == nil {
		t.Fatal("expected error for empty parent")
	}
}

func TestSpinoffNoText(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openspec := filepath.Join(dir, "openspec")
	parentDir := filepath.Join(openspec, "changes", "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(parentDir, "proposal.md"), []byte("# Parent\n"), 0o644)
	os.WriteFile(filepath.Join(parentDir, "tasks.md"), []byte("- [ ] task one\n"), 0o644)

	_, err := Spinoff(context.Background(), SpinoffOptions{
		OpenSpecDir: openspec,
		Parent:      "parent",
	})
	if err == nil {
		t.Fatal("expected error for no text")
	}
}
