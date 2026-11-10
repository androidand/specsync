package specsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// workspaceFixture builds openspec/changes/fix-login-race (committed-looking,
// no ref cache) and returns the openspec dir.
func workspaceFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "openspec")
	dir := filepath.Join(root, "changes", "fix-login-race")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# Fix the login race\n\nLocal proposal.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func proposalOf(t *testing.T, root, slug string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "changes", slug, "proposal.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWorkspaceChangeFromBranch(t *testing.T) {
	root := workspaceFixture(t)
	res, err := WorkspaceChange(context.Background(), WorkspaceChangeOptions{
		OpenSpecDir: root, Branch: "someuser/fix-login-race", DefaultBranch: "main", LinkedIssue: "99",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Slug != "fix-login-race" || res.Source != "branch" {
		t.Fatalf("got %+v", res)
	}
}

func TestWorkspaceChangeFromRefCache(t *testing.T) {
	root := workspaceFixture(t)
	dir := filepath.Join(root, "changes", "fix-login-race")
	if err := saveRef(dir, "github:o/r", Ref{Provider: "github", ID: "42", URL: "https://github.com/o/r/issues/42"}); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	res, err := WorkspaceChange(context.Background(), WorkspaceChangeOptions{
		OpenSpecDir: root, Branch: "someuser/login-work", DefaultBranch: "main", LinkedIssue: "42",
		Provider: NewGitHubProviderFunc(ghRunner(fakeIssue{}, &calls)), PullMissing: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Slug != "fix-login-race" || res.Source != "ref-cache" {
		t.Fatalf("got %+v", res)
	}
	if len(calls) != 0 {
		t.Fatalf("read the tracker although the cache answered: %v", calls)
	}
}

// A fresh worktree has the committed change but no ref cache: the issue's
// marker must find it, bind the cache, and leave the local proposal alone.
func TestWorkspaceChangeFromIssueMarker(t *testing.T) {
	root := workspaceFixture(t)
	issue := fakeIssue{Number: 42, URL: "https://github.com/o/r/issues/42", Title: "Fix the login race", State: "open",
		Body: "<!-- specsync:change=fix-login-race -->\n\nRendered issue body.\n"}
	var calls [][]string
	res, err := WorkspaceChange(context.Background(), WorkspaceChangeOptions{
		OpenSpecDir: root, Branch: "someuser/login-work", DefaultBranch: "main", LinkedIssue: "42",
		Provider: NewGitHubProviderFunc(ghRunner(issue, &calls)), PullMissing: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Slug != "fix-login-race" || res.Source != "issue-marker" {
		t.Fatalf("got %+v", res)
	}
	if got := proposalOf(t, root, "fix-login-race"); got != "# Fix the login race\n\nLocal proposal.\n" {
		t.Fatalf("local proposal overwritten:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "changes", "fix-login-race-2")); err == nil {
		t.Fatal("created a suffixed copy of the change")
	}
	refs, _ := LoadRefs(filepath.Join(root, "changes", "fix-login-race"))
	bound := false
	for _, r := range refs {
		bound = bound || r.ID == "42"
	}
	if !bound {
		t.Fatalf("ref cache not bound to issue 42: %+v", refs)
	}
}

func TestWorkspaceChangePullsIssueWithoutChange(t *testing.T) {
	root := workspaceFixture(t)
	issue := fakeIssue{Number: 57, URL: "https://github.com/o/r/issues/57", Title: "Add CSV export", State: "open",
		Body: "Users want their data.\n"}
	var calls [][]string
	res, err := WorkspaceChange(context.Background(), WorkspaceChangeOptions{
		OpenSpecDir: root, Branch: "someuser/csv", DefaultBranch: "main", LinkedIssue: "57",
		Provider: NewGitHubProviderFunc(ghRunner(issue, &calls)), PullMissing: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "pulled" || res.Slug == "" {
		t.Fatalf("got %+v", res)
	}
	if _, err := os.Stat(filepath.Join(root, "changes", res.Slug, "proposal.md")); err != nil {
		t.Fatalf("pulled change missing: %v", err)
	}
}

func TestWorkspaceChangeDoesNotPullWhenNotAsked(t *testing.T) {
	root := workspaceFixture(t)
	issue := fakeIssue{Number: 57, URL: "https://github.com/o/r/issues/57", Title: "Add CSV export", State: "open", Body: "x\n"}
	var calls [][]string
	for _, opts := range []WorkspaceChangeOptions{
		{OpenSpecDir: root, Branch: "someuser/csv", LinkedIssue: "57", Provider: NewGitHubProviderFunc(ghRunner(issue, &calls))},
		{OpenSpecDir: root, Branch: "someuser/csv", LinkedIssue: "57", Provider: NewGitHubProviderFunc(ghRunner(issue, &calls)), PullMissing: true, DryRun: true},
	} {
		res, err := WorkspaceChange(context.Background(), opts)
		if err != nil {
			t.Fatal(err)
		}
		if res.Slug != "" {
			t.Fatalf("got %+v", res)
		}
		if opts.DryRun != res.WouldPull {
			t.Fatalf("WouldPull = %v on dry run %v", res.WouldPull, opts.DryRun)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(root, "changes"))
	if len(entries) != 1 {
		t.Fatalf("wrote a change: %v", entries)
	}
}

func TestWorkspaceChangeUnrelatedWorkspace(t *testing.T) {
	root := workspaceFixture(t)
	res, err := WorkspaceChange(context.Background(), WorkspaceChangeOptions{
		OpenSpecDir: root, Branch: "someuser/spike", DefaultBranch: "main",
	})
	if err != nil || res.Slug != "" {
		t.Fatalf("got %+v, %v", res, err)
	}
}
