package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sizeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	clone := filepath.Join(root, "clone")
	env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@example.com")
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(root, "init", "-q", "--bare", "-b", "main", origin)
	git(root, "clone", "-q", origin, clone)
	git(clone, "checkout", "-q", "-b", "main")
	write := func(name, body, msg string) {
		if err := os.WriteFile(filepath.Join(clone, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		git(clone, "add", name)
		git(clone, "commit", "-q", "-m", msg)
	}
	write("a.txt", "a\n", "init")
	git(clone, "push", "-q", "-u", "origin", "main")
	git(clone, "remote", "set-head", "origin", "main")
	git(clone, "checkout", "-q", "-b", "feat/x")
	write("b.txt", strings.Repeat("l\n", 40), "feat(billing): b")
	write("c.txt", "c\n", "feat(onboarding): c")
	return clone
}

func TestVerifySizeIsAdvisoryByDefault(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo := sizeRepo(t)
	_, env := firstUseEnv(t)
	out, err := runIn(t, bin, repo, env, "verify", "-size", "-budget", "lines=10")
	if err != nil {
		t.Fatalf("advisory must exit 0: %v\n%s", err, out)
	}
	if !strings.Contains(out, "oversized: lines 41 > 10") || !strings.Contains(out, "mixed-scopes: billing, onboarding") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestVerifySizeFailOnOversizedExitsNonZero(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo := sizeRepo(t)
	_, env := firstUseEnv(t)
	out, err := runIn(t, bin, repo, env, "verify", "-size", "-fail-on-oversized", "-budget", "lines=10")
	if err == nil || !strings.Contains(out, "oversized: lines 41 > 10") {
		t.Fatalf("want a non-zero exit naming the limit, got err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "verify", "-size", "-fail-on-oversized", "-exclude", "b.txt", "-budget", "lines=10")
	if err != nil || !strings.Contains(out, "within budget") && !strings.Contains(out, "mixed-scopes") {
		t.Fatalf("an excluded path must not count: err=%v\n%s", err, out)
	}
}

func TestVerifySizeEnvBudgetAndWithinBudget(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo := sizeRepo(t)
	_, env := firstUseEnv(t)
	out, err := runIn(t, bin, repo, append(env, "SPECSYNC_BUDGET=commits=1"), "verify", "-size", "-fail-on-oversized")
	if err == nil || !strings.Contains(out, "commits 2 > 1") {
		t.Fatalf("SPECSYNC_BUDGET must apply: err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, append(env, "SPECSYNC_BUDGET=commits=1"), "verify", "-size", "-fail-on-oversized", "-budget", "commits=5")
	if err != nil {
		t.Fatalf("the flag beats the environment: %v\n%s", err, out)
	}
}

func TestVerifySizeFlagsNeedSize(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo := sizeRepo(t)
	_, env := firstUseEnv(t)
	out, err := runIn(t, bin, repo, env, "verify", "-fail-on-oversized")
	if err == nil || !strings.Contains(out, "need -size") {
		t.Fatalf("got err=%v\n%s", err, out)
	}
}
