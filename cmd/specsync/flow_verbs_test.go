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

func withIdentity(env []string) []string {
	return append(env, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
}

func TestSaveDryRunListsFilesAndChangesNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "flow: minimal\n")
	env = withIdentity(env)
	commitAll(t, repo, "config")
	os.WriteFile(filepath.Join(repo, "openspec", "specsync.yml"), []byte("flow: minimal\n# edit\n"), 0o644)
	out, err := runIn(t, bin, repo, env, "save", "-m", "edit config", "-dry-run")
	if err != nil || !strings.Contains(out, "openspec/specsync.yml") || !strings.Contains(out, "DRY RUN") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if st := gitStatus(t, repo); !strings.Contains(st, "M openspec/specsync.yml") {
		t.Fatalf("dry run must leave the change uncommitted: %q", st)
	}
}

func TestSaveRefusesASecretAndCommitsOtherwiseWithoutPushing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "flow: minimal\n")
	env = withIdentity(env)
	commitAll(t, repo, "config")
	os.WriteFile(filepath.Join(repo, ".env.local"), []byte("A=1\n"), 0o644)
	out, err := runIn(t, bin, repo, env, "save", "-m", "x", "-add", ".env.local")
	if err == nil || !strings.Contains(out, ".env.local") || !strings.Contains(out, "refusing") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(repo, "ok.txt"), []byte("ok\n"), 0o644)
	out, err = runIn(t, bin, repo, env, "save", "-m", "add ok", "-add", "ok.txt")
	if err != nil || !strings.Contains(out, "not pushed") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if st := gitStatus(t, repo); !strings.Contains(st, "?? .env.local") || strings.Contains(st, "ok.txt") {
		t.Fatalf("status = %q", st)
	}
}

func TestParkSetsTheStageAddsANoteAndCommitsItWhenTracked(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "flow: minimal\n")
	env = withIdentity(env)
	change := filepath.Join(repo, "openspec", "changes", "add-x")
	os.MkdirAll(change, 0o755)
	os.WriteFile(filepath.Join(change, "proposal.md"), []byte("# Add x\n"), 0o644)
	os.WriteFile(filepath.Join(change, "tasks.md"), []byte("- [ ] 1.1 a\n"), 0o644)
	gitIn(t, repo, "checkout", "-q", "-b", "feat/add-x")
	commitAll(t, repo, "spec")
	out, err := runIn(t, bin, repo, env, "park", "-m", "waiting on the API", "-stage", "blocked", "-dry-run")
	if err != nil || !strings.Contains(out, "DRY RUN") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if st := gitStatus(t, repo); st != "" {
		t.Fatalf("a dry run must not touch anything: %q", st)
	}
	out, err = runIn(t, bin, repo, env, "park", "-m", "waiting on the API", "-stage", "blocked")
	if err != nil {
		t.Fatalf("err=%v\n%s", err, out)
	}
	note, _ := os.ReadFile(filepath.Join(change, "discoveries.md"))
	if !strings.Contains(string(note), "- parked: waiting on the API") {
		t.Fatalf("note = %q", note)
	}
	if st := gitStatus(t, repo); st != "" {
		t.Fatalf("park must commit the stage and note it wrote: %q", st)
	}
}

func TestParkNeedsAMessageAndAKnownChange(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "flow: minimal\n")
	env = withIdentity(env)
	if out, err := runIn(t, bin, repo, env, "park"); err == nil || !strings.Contains(out, "-m") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if out, err := runIn(t, bin, repo, env, "park", "-m", "x"); err == nil || !strings.Contains(out, "which change") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestSaveSyncNeedsAKnownChangeAndCommitsNothingWithoutOne(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "flow: minimal\n")
	env = withIdentity(env)
	commitAll(t, repo, "config")
	os.WriteFile(filepath.Join(repo, "openspec", "specsync.yml"), []byte("flow: minimal\n# edit\n"), 0o644)
	out, err := runIn(t, bin, repo, env, "save", "-m", "edit", "-sync")
	if err == nil || !strings.Contains(out, "which change") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if st := gitStatus(t, repo); !strings.Contains(st, "M openspec/specsync.yml") {
		t.Fatalf("nothing may be committed when -sync cannot run: %q", st)
	}
}
