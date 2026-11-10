package specsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

func statusOf(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestInitScratchPlainRepo(t *testing.T) {
	dir := newRepo(t)
	res, err := InitScratch(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.CreatedDir || !res.Excluded {
		t.Fatalf("want created+excluded, got %+v", res)
	}
	if info, err := os.Stat(filepath.Join(dir, "openspec", "changes")); err != nil || !info.IsDir() {
		t.Fatalf("openspec/changes not created: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openspec", "changes", "x.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, dir); got != "" {
		t.Fatalf("openspec/ should be invisible to git, status:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err == nil {
		t.Fatal("init must not create a .gitignore")
	}
}

func TestInitScratchDryRunWritesNothing(t *testing.T) {
	dir := newRepo(t)
	res, err := InitScratch(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.CreatedDir || !res.Excluded {
		t.Fatalf("dry run should report the plan, got %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "openspec")); err == nil {
		t.Fatal("dry run created openspec/")
	}
}

func TestInitScratchTwiceIsIdempotent(t *testing.T) {
	dir := newRepo(t)
	if _, err := InitScratch(dir, false); err != nil {
		t.Fatal(err)
	}
	res, err := InitScratch(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.CreatedDir || res.Excluded || !res.AlreadyIgnored {
		t.Fatalf("second run should change nothing, got %+v", res)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "/openspec/"); n != 1 {
		t.Fatalf("exclude line appears %d times:\n%s", n, data)
	}
}

func TestInitScratchLinkedWorktree(t *testing.T) {
	main := newRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	gitRun(t, main, "worktree", "add", "-q", "-b", "feat", wt)
	if _, err := InitScratch(wt, false); err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, wt); got != "" {
		t.Fatalf("openspec/ should be ignored in the linked worktree, status:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(wt, ".git")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(filepath.Join(wt, ".git")); info.IsDir() {
		t.Fatal("expected .git to be a file in a linked worktree")
	}
	data, err := os.ReadFile(filepath.Join(main, ".git", "info", "exclude"))
	if err != nil || !strings.Contains(string(data), "/openspec/") {
		t.Fatalf("exclude should land in the common git dir: %v\n%s", err, data)
	}
}

func TestInitScratchRefusesTrackedOpenspec(t *testing.T) {
	dir := newRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "openspec", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openspec", "config.yaml"), []byte("x: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "openspec")
	gitRun(t, dir, "commit", "-q", "-m", "spec")
	if _, err := InitScratch(dir, false); err == nil || !strings.Contains(err.Error(), "tracked") {
		t.Fatalf("want tracked refusal, got %v", err)
	}
}

func TestInitScratchRespectsExistingGitignore(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("openspec/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := InitScratch(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.AlreadyIgnored || res.Excluded {
		t.Fatalf("an existing ignore should be reused, got %+v", res)
	}
}

func TestInitScratchOutsideGit(t *testing.T) {
	if _, err := InitScratch(t.TempDir(), false); err == nil {
		t.Fatal("want an error outside a git repo")
	}
	if _, err := os.Stat(filepath.Join(t.TempDir(), "openspec")); err == nil {
		t.Fatal("unexpected")
	}
}
