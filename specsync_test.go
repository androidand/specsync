package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadChanges(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "my-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# My Title\n\nthe body\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [ ] do the thing\n")
	mustWrite(t, filepath.Join(cdir, ".status"), "planned\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "priority"), "2\n")

	// An archived change projects as closed.
	adir := filepath.Join(root, "changes", "archive", "old-change")
	mustWrite(t, filepath.Join(adir, "proposal.md"), "# Old\n")

	changes, err := LoadChanges(root)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("want 2 changes, got %d", len(changes))
	}

	byslug := map[string]Change{}
	for _, c := range changes {
		byslug[c.Slug] = c
	}

	c := byslug["my-change"]
	if c.Title != "My Title" {
		t.Errorf("title = %q, want %q", c.Title, "My Title")
	}
	if c.Stage != "planned" {
		t.Errorf("stage = %q, want planned (from .status)", c.Stage)
	}
	if c.Priority != 2 {
		t.Errorf("priority = %d, want 2", c.Priority)
	}
	if c.Archived {
		t.Errorf("my-change should not be archived")
	}
	if !strings.Contains(c.TasksMarkdown, "do the thing") {
		t.Errorf("tasks not loaded: %q", c.TasksMarkdown)
	}

	old := byslug["old-change"]
	if !old.Archived || old.Stage != StageArchived {
		t.Errorf("old-change should be archived, got archived=%v stage=%q", old.Archived, old.Stage)
	}
}

func TestGitHubPushCreate(t *testing.T) {
	var calls [][]string
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "issue" && args[1] == "list":
			return "[]", nil // no existing issue
		case args[0] == "issue" && args[1] == "create":
			return "https://github.com/o/r/issues/7", nil
		default:
			return "", nil
		}
	}}

	ref, err := p.Push(context.Background(), WorkItem{
		Slug: "my-change", Title: "T", Body: "B", Stage: "planned", Priority: 2,
	}, nil)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if ref.ID != "7" {
		t.Errorf("ref.ID = %q, want 7", ref.ID)
	}

	create := findCall(calls, "issue", "create")
	if create == nil {
		t.Fatal("no issue create call")
	}
	body := flagValue(create, "--body")
	if !strings.Contains(body, "specsync:change=my-change") {
		t.Errorf("body missing identity marker: %q", body)
	}
	if !hasLabel(create, "stage:planned") || !hasLabel(create, "priority:2") {
		t.Errorf("create missing expected labels: %v", create)
	}
}

func TestGitHubPushUpdateReconcilesLabels(t *testing.T) {
	var editArgs []string
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		switch {
		case args[0] == "issue" && args[1] == "view":
			// current labels include a stale stage that must be removed.
			return `{"labels":[{"name":"specsync"},{"name":"stage:triaged"}]}`, nil
		case args[0] == "issue" && args[1] == "edit":
			editArgs = args
			return "", nil
		default:
			return "", nil
		}
	}}

	_, err := p.Push(context.Background(), WorkItem{
		Slug: "my-change", Title: "T", Body: "B", Stage: "planned",
	}, &Ref{Provider: "github", ID: "7"})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if !hasFlagValue(editArgs, "--add-label", "stage:planned") {
		t.Errorf("expected add stage:planned, got %v", editArgs)
	}
	if !hasFlagValue(editArgs, "--remove-label", "stage:triaged") {
		t.Errorf("expected remove stale stage:triaged, got %v", editArgs)
	}
}

func TestGitHubPushReopensManagedActiveIssue(t *testing.T) {
	var calls [][]string
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		if args[0] == "issue" && args[1] == "view" {
			return `{"state":"CLOSED","labels":[{"name":"stage:complete"}]}`, nil
		}
		return "", nil
	}}
	_, err := p.Push(context.Background(), WorkItem{
		Slug: "my-change", Title: "T", Stage: StageActive, ManageClosed: true,
	}, &Ref{Provider: "github", ID: "7"})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if findCall(calls, "issue", "reopen", "7") == nil {
		t.Fatalf("expected managed active issue to reopen; calls: %v", calls)
	}
}

func TestGitHubFindRequiresExactMarker(t *testing.T) {
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "list" {
			// First hit is a fuzzy body match, second is the exact marker.
			return `[
				{"number":1,"url":"https://github.com/o/r/issues/1","body":"mentions specsync:change=issue-first-intake in prose"},
				{"number":2,"url":"https://github.com/o/r/issues/2","body":"<!-- specsync:change=issue-first-intake -->\n\nreal marker"}
			]`, nil
		}
		return "", nil
	}}

	ref, err := p.Find(context.Background(), "issue-first-intake")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ref == nil {
		t.Fatal("Find returned nil ref, want exact marker match")
	}
	if ref.ID != "2" {
		t.Fatalf("ref.ID = %q, want 2", ref.ID)
	}
}

func TestGitHubFindReturnsNilWithoutExactMarker(t *testing.T) {
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "list" {
			return `[
				{"number":1,"url":"https://github.com/o/r/issues/1","body":"specsync:change=issue-first-intake appears only in text"}
			]`, nil
		}
		return "", nil
	}}

	ref, err := p.Find(context.Background(), "issue-first-intake")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ref != nil {
		t.Fatalf("Find returned ref %#v, want nil without exact marker", ref)
	}
}

// stubProvider returns a fixed ref and records nothing else.
type stubProvider struct{ ref Ref }

func (s stubProvider) Name() string { return "github" }
func (s stubProvider) Push(context.Context, WorkItem, *Ref) (Ref, error) {
	return s.ref, nil
}
func (s stubProvider) Find(context.Context, string) (*Ref, error) { return nil, nil }

func TestDryRunDoesNotWriteCache(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "c1")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# C1\n")
	prov := stubProvider{ref: Ref{Provider: "github", ID: "0"}}

	if _, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, DryRun: true}); err != nil {
		t.Fatalf("dry-run Sync: %v", err)
	}
	if _, err := os.Stat(refCachePath(cdir)); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not write the ref cache, but %s exists", refCachePath(cdir))
	}

	// A real run must write it.
	if _, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov}); err != nil {
		t.Fatalf("real Sync: %v", err)
	}
	if _, err := os.Stat(refCachePath(cdir)); err != nil {
		t.Fatalf("real run should write the ref cache: %v", err)
	}
}

// --- helpers ---

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findCall(calls [][]string, prefix ...string) []string {
	for _, c := range calls {
		if len(c) < len(prefix) {
			continue
		}
		match := true
		for i, p := range prefix {
			if c[i] != p {
				match = false
				break
			}
		}
		if match {
			return c
		}
	}
	return nil
}

func flagValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func hasFlagValue(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func hasLabel(args []string, label string) bool {
	return hasFlagValue(args, "--label", label)
}
