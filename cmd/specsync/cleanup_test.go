package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func cleanupWorkspace(t *testing.T) (root, wt string) {
	t.Helper()
	root = t.TempDir()
	repo := filepath.Join(root, "app")
	wt = filepath.Join(root, "app-feature")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "init", "-q", "-b", "main")
	gitRun(t, repo, "remote", "add", "origin", "https://github.com/acme/app.git")
	gitRun(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	gitRun(t, repo, "worktree", "add", "-q", "-b", "feat/42-thing", wt)
	if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", "a.txt")
	gitRun(t, wt, "commit", "-q", "-m", "work")
	gitRun(t, repo, "merge", "-q", "--ff-only", "feat/42-thing")
	gitRun(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	gitRun(t, repo, "update-ref", "refs/remotes/origin/main", "main")
	return root, wt
}

func TestCleanupCommandIsADryRunUnlessApplied(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	root, wt := cleanupWorkspace(t)
	env := append(os.Environ(), "SPECSYNC_TRUST_FILE="+filepath.Join(t.TempDir(), "trust.json"), "SPECSYNC_ROOTS=")

	out, err := runIn(t, bin, t.TempDir(), env, "cleanup", "-root", root, "-state", "off")
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "finished") || !strings.Contains(out, "would: remove worktree") || !strings.Contains(out, "nothing was changed") {
		t.Fatalf("the dry run must list the candidate and say nothing changed:\n%s", out)
	}
	if _, err := os.Stat(wt); err != nil {
		t.Fatal("a dry run must not remove the worktree")
	}

	out, err = runIn(t, bin, t.TempDir(), env, "cleanup", "-root", root, "-state", "off", "-apply")
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	if !strings.Contains(out, "done: removed worktree") || !strings.Contains(out, "done: deleted local branch") {
		t.Fatalf("apply must report what it did:\n%s", out)
	}
	if _, err := os.Stat(wt); err == nil {
		t.Fatal("-apply must remove the worktree")
	}
}

func TestCleanupCommandNeverRemovesTheCurrentWorktree(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	root, wt := cleanupWorkspace(t)
	env := append(os.Environ(), "SPECSYNC_TRUST_FILE="+filepath.Join(t.TempDir(), "trust.json"))
	out, err := runIn(t, bin, wt, env, "cleanup", "-root", root, "-state", "off", "-apply")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "current worktree") {
		t.Fatalf("the current worktree must be skipped with the reason:\n%s", out)
	}
	if _, err := os.Stat(wt); err != nil {
		t.Fatal("the current worktree must stay")
	}
}
