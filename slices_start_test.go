package specsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sliceRepo(t *testing.T) (repo, changeDir string) {
	t.Helper()
	root := t.TempDir()
	repo = filepath.Join(root, "app")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		runGitS(t, repo, args...)
	}
	changeDir = filepath.Join(repo, "openspec", "changes", "big-thing")
	mustWrite(t, filepath.Join(changeDir, "proposal.md"), "# Big thing\n")
	mustWrite(t, filepath.Join(changeDir, "tasks.md"),
		"# Tasks\n\n## Slice 1: Parser\ncontract: parse\n- [ ] a\n\n## Slice 2: Commands\nafter: 1\ncontract: list\n- [ ] b\n\n## Slice 3: Docs\ncontract: docs\n- [ ] c\n")
	return repo, changeDir
}

func runGitS(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestStartSlice_StackedOnThePreviousSliceAndBindingRecorded(t *testing.T) {
	repo, changeDir := sliceRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "worktrees")
	opts := func(n int) StartSliceOptions {
		return StartSliceOptions{RepoDir: repo, ChangeDir: changeDir, Slug: "big-thing", IssueID: "42", N: n, WorktreeDir: wt, Base: "main"}
	}
	r1, err := StartSlice(context.Background(), opts(1))
	if err != nil {
		t.Fatal(err)
	}
	if r1.Branch != "feat/42-big-thing-slice-1" || r1.Base != "main" || r1.Stacked || !r1.Created {
		t.Fatalf("slice 1 = %+v", r1)
	}
	runGitS(t, r1.Worktree, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "slice one work")

	r2, err := StartSlice(context.Background(), opts(2))
	if err != nil {
		t.Fatal(err)
	}
	if r2.Base != r1.Branch || !r2.Stacked || r2.Branch != "feat/42-big-thing-slice-2" {
		t.Fatalf("slice 2 must be based on slice 1's branch: %+v", r2)
	}
	if subj := runGitS(t, r2.Worktree, "log", "-1", "--format=%s"); subj != "slice one work" {
		t.Fatalf("slice 2 does not contain slice 1's commit, head is %q", subj)
	}
	bind, err := LoadSliceBindings(changeDir)
	if err != nil || bind[2].Branch != r2.Branch || bind[2].Base != r1.Branch || bind[1].Worktree != r1.Worktree {
		t.Fatalf("bindings = %+v, %v", bind, err)
	}
}

func TestStartSlice_UnstackedUsesTheBaseAndIsIdempotent(t *testing.T) {
	repo, changeDir := sliceRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "worktrees")
	o := StartSliceOptions{RepoDir: repo, ChangeDir: changeDir, Slug: "big-thing", N: 3, WorktreeDir: wt, Base: "main"}
	r, err := StartSlice(context.Background(), o)
	if err != nil || r.Stacked || r.Base != "main" || r.Branch != "feat/big-thing-slice-3" {
		t.Fatalf("unstacked: %+v %v", r, err)
	}
	again, err := StartSlice(context.Background(), o)
	if err != nil || again.Created || again.Worktree != r.Worktree {
		t.Fatalf("a second start must change nothing: %+v %v", again, err)
	}
}

func TestStartSlice_RefusesBeforeThePreviousSliceHasABranch(t *testing.T) {
	repo, changeDir := sliceRepo(t)
	_, err := StartSlice(context.Background(), StartSliceOptions{RepoDir: repo, ChangeDir: changeDir, Slug: "big-thing", N: 2, Base: "main", WorktreeDir: filepath.Join(filepath.Dir(repo), "worktrees")})
	if err == nil || !strings.Contains(err.Error(), "slice 1") {
		t.Fatalf("want an error naming slice 1, got %v", err)
	}
}

func TestStartSlice_DryRunAndUnknownSliceChangeNothing(t *testing.T) {
	repo, changeDir := sliceRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "worktrees")
	before := runGitS(t, repo, "branch", "--list")
	r, err := StartSlice(context.Background(), StartSliceOptions{RepoDir: repo, ChangeDir: changeDir, Slug: "big-thing", N: 1, Base: "main", WorktreeDir: wt, DryRun: true})
	if err != nil || r.Branch == "" || r.Created {
		t.Fatalf("dry run: %+v %v", r, err)
	}
	if _, statErr := os.Stat(wt); statErr == nil {
		t.Fatal("dry run must not create the worktree folder")
	}
	if runGitS(t, repo, "branch", "--list") != before {
		t.Fatal("dry run must not create a branch")
	}
	if _, err := os.Stat(filepath.Join(changeDir, ".specsync", "slices.json")); err == nil {
		t.Fatal("dry run must not record a binding")
	}
	if _, err := StartSlice(context.Background(), StartSliceOptions{RepoDir: repo, ChangeDir: changeDir, Slug: "big-thing", N: 9, Base: "main"}); err == nil {
		t.Fatal("an undeclared slice cannot be started")
	}
}
