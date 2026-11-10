package specsync

import (
	"context"
	"path/filepath"
	"testing"
)

func TestListWorktrees_ParsesPorcelain(t *testing.T) {
	porcelain := "worktree /repo\n" +
		"HEAD aaaa\n" +
		"branch refs/heads/main\n" +
		"\n" +
		"worktree /worktrees/add-thing\n" +
		"HEAD bbbb\n" +
		"branch refs/heads/feat/42-add-thing\n" +
		"\n" +
		"worktree /worktrees/detached\n" +
		"HEAD cccc\n" +
		"detached\n" +
		"\n" +
		"worktree /repo.git\n" +
		"bare\n"

	run := func(_ context.Context, dir string) (string, error) { return porcelain, nil }
	entries, err := listWorktrees(context.Background(), run, "/repo")
	if err != nil {
		t.Fatalf("listWorktrees: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d: %+v", len(entries), entries)
	}
	if entries[1].path != "/worktrees/add-thing" || entries[1].branch != "feat/42-add-thing" {
		t.Errorf("entry[1] = %+v", entries[1])
	}
	if entries[2].branch != "" {
		t.Errorf("detached-HEAD entry should have empty branch, got %q", entries[2].branch)
	}
	if entries[3].branch != "" {
		t.Errorf("bare-repo entry should have empty branch, got %q", entries[3].branch)
	}
}

func TestMatchWorktree(t *testing.T) {
	entries := []worktreeEntry{
		{path: "/repo", branch: "main"},
		{path: "/worktrees/add-thing", branch: "feat/42-add-thing"},
		{path: "/worktrees/detached", branch: ""},
		{path: "/some/path/basename-only-match", branch: ""},
	}

	t.Run("matched by branch pattern", func(t *testing.T) {
		branch, path, ok := matchWorktree(entries, "add-thing")
		if !ok || branch != "feat/42-add-thing" || path != "/worktrees/add-thing" {
			t.Errorf("got branch=%q path=%q ok=%v", branch, path, ok)
		}
	})

	t.Run("matched by directory basename fallback", func(t *testing.T) {
		branch, path, ok := matchWorktree(entries, "basename-only-match")
		if !ok || path != "/some/path/basename-only-match" || branch != "" {
			t.Errorf("got branch=%q path=%q ok=%v", branch, path, ok)
		}
	})

	t.Run("unmatched", func(t *testing.T) {
		_, _, ok := matchWorktree(entries, "no-such-change")
		if ok {
			t.Error("expected no match")
		}
	})

	t.Run("detached HEAD with no branch falls through to basename match", func(t *testing.T) {
		// entries[2].branch == "" (detached), but its directory basename
		// happens to equal the slug — the documented basename fallback still
		// applies; a detached worktree isn't unmatchable, just unmatchable
		// by branch.
		branch, path, ok := matchWorktree(entries, "detached")
		if !ok || branch != "" || path != "/worktrees/detached" {
			t.Errorf("got branch=%q path=%q ok=%v", branch, path, ok)
		}
	})

	t.Run("bare repo (no entries at all) yields no match, not an error", func(t *testing.T) {
		_, _, ok := matchWorktree(nil, "anything")
		if ok {
			t.Error("expected no match against an empty entry list")
		}
	})
}

func TestMatchWorktreeByIssue(t *testing.T) {
	entries := []worktreeEntry{
		{path: "/w/a", branch: "feat/186-status-local-state"},
		{path: "/w/b", branch: "fix/18-other"},
		{path: "/w/c", branch: "1860-not-it"},
		{path: "/w/d", branch: "main"},
	}
	cases := []struct {
		issue, wantPath string
	}{
		{"186", "/w/a"},
		{"18", "/w/b"},
		{"1860", "/w/c"},
		{"19", ""},
		{"", ""},
	}
	for _, c := range cases {
		_, path, ok := matchWorktreeByIssue(entries, c.issue)
		if path != c.wantPath || ok != (c.wantPath != "") {
			t.Errorf("issue %q: got (%q, %v), want %q", c.issue, path, ok, c.wantPath)
		}
	}
}

func TestBuildTopology_ResolvesTheWorktreeThroughTheIssueNumber(t *testing.T) {
	opts, _ := localStateFixture(t)
	root := opts.OpenSpecDir
	// The branch carries the issue number but not the slug.
	mustWrite(t, filepath.Join(root, "changes", "add-field-forms", ".specsync", "refs.json"),
		`{"github:acme/portal":{"provider":"github:acme/portal","id":"4231","url":"https://github.com/acme/portal/issues/4231"}}`)
	porcelain := "worktree " + filepath.Dir(root) + "\nHEAD a\nbranch refs/heads/main\n\n" +
		"worktree /elsewhere/wt\nHEAD b\nbranch refs/heads/feat/4231-something-else\n"
	opts.gitWorktreeList = fakeGitWorktreeList(map[string]string{filepath.Dir(root): porcelain}, nil)
	opts.localState = func(context.Context, string, LocalStateOptions) (*LocalState, error) { return &LocalState{}, nil }
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if row := topo.Changes[0]; row.Branch != "feat/4231-something-else" || row.Worktree != "/elsewhere/wt" {
		t.Fatalf("row = %+v", row)
	}
}
