package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// firstUseEnv returns a fresh git repo with no openspec/, and an environment
// whose `gh` answers `issue view` with an issue that has no task list.
func firstUseEnv(t *testing.T) (repo string, env []string) {
	t.Helper()
	repo = t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	bin := t.TempDir()
	stub := `#!/bin/sh
case "$1 $2" in
  "issue view") echo '{"number":7,"url":"https://github.com/o/r/issues/7","title":"Fix the thing","body":"just prose","state":"open","labels":[]}' ;;
  *) echo '[]' ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "HOME="+home, "SPECSYNC_DEFAULT_STORE=")
	return repo, env
}

func runIn(t *testing.T, bin, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func gitStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestPullWithoutOpenspecNamesInitAndCreatesNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := firstUseEnv(t)
	out, err := runIn(t, bin, repo, env, "pull", "-issue", "7", "-repo", "o/r")
	if err == nil {
		t.Fatalf("pull must fail without openspec/ and without -init:\n%s", out)
	}
	for _, want := range []string{"specsync init", "-store <id>"} {
		if !strings.Contains(out, want) {
			t.Errorf("error should name %q:\n%s", want, out)
		}
	}
	if _, statErr := os.Stat(filepath.Join(repo, "openspec")); statErr == nil {
		t.Fatal("nothing may be created silently")
	}
}

func TestPullInitDryRunCreatesNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := firstUseEnv(t)
	out, err := runIn(t, bin, repo, env, "pull", "-init", "-dry-run", "-issue", "7", "-repo", "o/r")
	if err != nil {
		t.Fatalf("dry run failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "would create") {
		t.Errorf("dry run should say what it would create:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "openspec")); statErr == nil {
		t.Fatal("dry run created openspec/")
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".git", "info", "exclude")); statErr == nil {
		if data, _ := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude")); strings.Contains(string(data), "/openspec/") {
			t.Fatal("dry run edited the exclude file")
		}
	}
}

func TestPullInitCreatesScratchSpecAndStarterTasks(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := firstUseEnv(t)
	out, err := runIn(t, bin, repo, env, "pull", "-init", "-issue", "7", "-repo", "o/r")
	if err != nil {
		t.Fatalf("pull -init failed: %v\n%s", err, out)
	}
	change := filepath.Join(repo, "openspec", "changes", "fix-the-thing")
	for _, f := range []string{"proposal.md", "tasks.md"} {
		if _, statErr := os.Stat(filepath.Join(change, f)); statErr != nil {
			t.Errorf("%s missing after pull -init: %v\n%s", f, statErr, out)
		}
	}
	if got := gitStatus(t, repo); got != "" {
		t.Errorf("scratch openspec/ must stay out of git, status:\n%s", got)
	}
	if vout, verr := runIn(t, bin, repo, env, "validate"); verr != nil {
		t.Errorf("a freshly pulled change must validate: %v\n%s", verr, vout)
	}
}

func TestInitSubcommand(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := firstUseEnv(t)
	for i := 0; i < 2; i++ {
		if out, err := runIn(t, bin, repo, env, "init"); err != nil {
			t.Fatalf("init run %d: %v\n%s", i+1, err, out)
		}
	}
	if info, err := os.Stat(filepath.Join(repo, "openspec", "changes")); err != nil || !info.IsDir() {
		t.Fatalf("openspec/changes missing: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if n := strings.Count(string(data), "/openspec/"); n != 1 {
		t.Fatalf("exclude entry count = %d:\n%s", n, data)
	}
	if out, err := runIn(t, bin, repo, env, "init", "extra"); err == nil {
		t.Fatalf("init must reject stray arguments:\n%s", out)
	}
}
