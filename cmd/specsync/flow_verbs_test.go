package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commitAll(t *testing.T, repo, msg string) {
	t.Helper()
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", msg)
}

func TestGatesDryRunListsAndRunsNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	marker := filepath.Join(t.TempDir(), "ran")
	repo, env := waysRepo(t, "gates:\n  - {name: t, run: \"touch "+marker+"\"}\n")
	out, err := runIn(t, bin, repo, env, "gates", "-dry-run")
	if err != nil || !strings.Contains(out, "t: touch") || !strings.Contains(out, "not trusted") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("a dry run must run nothing")
	}
}

func TestGatesRefuseUntrustedThenRunAndStampAfterTrust(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	marker := filepath.Join(t.TempDir(), "ran")
	repo, env := waysRepo(t, "gates:\n  - {name: t, run: \"touch "+marker+"\"}\n")
	commitAll(t, repo, "config")
	out, err := runIn(t, bin, repo, env, "gates")
	if err == nil || !strings.Contains(out, "specsync trust") {
		t.Fatalf("untrusted must refuse: err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("nothing may run before trust")
	}
	if out, err := runIn(t, bin, repo, env, "trust", "-yes"); err != nil {
		t.Fatalf("trust: %v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "gates")
	if err != nil || !strings.Contains(out, "stamped") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Fatal("the gate must have run")
	}
}

func TestGatesFailureExitsNonZero(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates:\n  - {name: bad, run: \"exit 2\"}\n")
	commitAll(t, repo, "config")
	runIn(t, bin, repo, env, "trust", "-yes")
	out, err := runIn(t, bin, repo, env, "gates")
	if err == nil || !strings.Contains(out, "gates failed") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestTrustListsWorktreeHooksWithTheGates(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates:\n  - {name: t, run: make test}\nworktrees:\n  hooks: [make setup]\n")
	out, err := runIn(t, bin, repo, env, "trust")
	if err != nil || !strings.Contains(out, "$ make test") || !strings.Contains(out, "$ make setup") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}
