package specsync

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func startRepo(t *testing.T, cfg string) (clone string) {
	t.Helper()
	clone, _ = repoWithOrigin(t)
	gitFixture(t, clone, "checkout", "-q", "main")
	os.MkdirAll(filepath.Join(clone, "openspec"), 0o755)
	os.WriteFile(filepath.Join(clone, SpecSyncConfigPath), []byte(cfg), 0o644)
	gitFixture(t, clone, "add", "openspec")
	gitFixture(t, clone, "commit", "-q", "-m", "config")
	t.Setenv(TrustFileEnv, filepath.Join(t.TempDir(), "trust.json"))
	return clone
}

func startIn(t *testing.T, clone string, o StartOptions) (StartResult, error) {
	t.Helper()
	w, _ := LoadWays(clone)
	o.RepoDir, o.Ways = clone, w
	if o.Slug == "" {
		o.Slug = "add-x"
	}
	return StartWork(context.Background(), o)
}

func TestStartCreatesTheWorktreeFromConfigRunsHooksAndRecordsTheLink(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "hook.log")
	clone := startRepo(t, "worktrees:\n  dir: ../wt\n  branch: \"{type}/{issue}-{slug}\"\n  hooks: [\"echo $SPECSYNC_BRANCH in $PWD >> "+marker+"\"]\n")
	if _, err := Trust(clone); err != nil {
		t.Fatal(err)
	}
	res, err := startIn(t, clone, StartOptions{Issue: "42"})
	if err != nil || res.Existing {
		t.Fatalf("%v %+v", err, res)
	}
	want := filepath.Join(filepath.Dir(clone), "wt", filepath.Base(clone)+"-add-x")
	if res.Path != want || res.Branch != "feat/42-add-x" {
		t.Fatalf("path=%q branch=%q", res.Path, res.Branch)
	}
	if b := gitFixture(t, res.Path, "rev-parse", "--abbrev-ref", "HEAD"); b != "feat/42-add-x" {
		t.Fatalf("branch = %q", b)
	}
	log, _ := os.ReadFile(marker)
	if !strings.Contains(string(log), "feat/42-add-x in ") || !strings.Contains(string(log), "-add-x") {
		t.Fatalf("the hook must run in the new worktree: %q", log)
	}
	if got := gitFixture(t, clone, "config", "--get", "branch.feat/42-add-x.specsyncChange"); got != "add-x" {
		t.Fatalf("link = %q", got)
	}
}

func TestStartRefusesUntrustedHooksAndCreatesNothing(t *testing.T) {
	clone := startRepo(t, "worktrees:\n  hooks: [make setup]\n")
	_, err := startIn(t, clone, StartOptions{})
	if err == nil || !strings.Contains(err.Error(), "specsync trust") {
		t.Fatalf("got %v", err)
	}
	if out := gitFixture(t, clone, "worktree", "list"); strings.Count(out, "\n") != 0 {
		t.Fatalf("no worktree may be created: %q", out)
	}
}

func TestStartReusesAnExistingWorktreeAndRunsNoHooks(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "hook.log")
	clone := startRepo(t, "worktrees:\n  hooks: [\"echo ran >> "+marker+"\"]\n")
	Trust(clone)
	first, err := startIn(t, clone, StartOptions{})
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(marker)
	second, err := startIn(t, clone, StartOptions{})
	if err != nil || !second.Existing || resolved(absPath(second.Path)) != resolved(absPath(first.Path)) {
		t.Fatalf("%v %+v", err, second)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("an existing worktree must not run the start hooks again")
	}
}

func TestStartRefusesALiveSessionAndAProviderThatCannotBeAsked(t *testing.T) {
	clone := startRepo(t, "flow: minimal\n")
	first, err := startIn(t, clone, StartOptions{})
	if err != nil {
		t.Fatal(err)
	}
	live := []SessionEntry{{CWD: filepath.Join(first.Path, "sub"), ID: "s-1", Name: "fixing x"}}
	_, err = startIn(t, clone, StartOptions{SessionsChecked: true, Sessions: live})
	if err == nil || !strings.Contains(err.Error(), "s-1") || !strings.Contains(err.Error(), "fixing x") {
		t.Fatalf("a live session must be named: %v", err)
	}
	ended := []SessionEntry{{CWD: first.Path, ID: "s-2", Status: "ended"}}
	if _, err := startIn(t, clone, StartOptions{SessionsChecked: true, Sessions: ended}); err != nil {
		t.Fatalf("an ended session is not live: %v", err)
	}
	_, err = startIn(t, clone, StartOptions{SessionsChecked: true, SessionsErr: errors.New("timed out")})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("an unverifiable session check must refuse reuse: %v", err)
	}
}

func TestStartBranchPatternIsPerRepoConfigAndDryRunCreatesNothing(t *testing.T) {
	a := startRepo(t, "worktrees: {branch: \"work/{slug}\"}\n")
	res, err := startIn(t, a, StartOptions{Issue: "7", DryRun: true})
	if err != nil || res.Branch != "work/add-x" || res.Created {
		t.Fatalf("%v %+v", err, res)
	}
	if _, statErr := os.Stat(res.Path); statErr == nil {
		t.Fatal("a dry run must create nothing")
	}
	b := startRepo(t, "flow: minimal\n")
	res, err = startIn(t, b, StartOptions{})
	if err != nil || res.Branch != "feat/add-x" {
		t.Fatalf("with no issue the default pattern drops it: %v %+v", err, res)
	}
}

func TestStartRejectsAnInvalidBranchPattern(t *testing.T) {
	clone := startRepo(t, "worktrees: {branch: \"bad name/{slug}\"}\n")
	if _, err := startIn(t, clone, StartOptions{}); err == nil || !strings.Contains(err.Error(), "branch") {
		t.Fatalf("got %v", err)
	}
}

func TestStartChecksTrustAgainstTheConfigActuallyRead(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "evil.log")
	clone := startRepo(t, "flow: minimal\n")
	if _, err := Trust(clone); err != nil {
		t.Fatal(err)
	}
	local := t.TempDir()
	os.MkdirAll(filepath.Join(local, "openspec"), 0o755)
	os.WriteFile(filepath.Join(local, SpecSyncConfigPath), []byte("worktrees:\n  hooks: [\"touch "+marker+"\"]\n"), 0o644)
	w, _ := LoadWays(local)
	_, err := StartWork(context.Background(), StartOptions{RepoDir: clone, ConfigRoot: local, Ways: w, Slug: "add-x"})
	if err == nil || !strings.Contains(err.Error(), "specsync trust") {
		t.Fatalf("hooks from an edited local config must be refused until that config is trusted: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("the hook ran without trust")
	}
	if out := gitFixture(t, clone, "worktree", "list"); strings.Count(out, "\n") != 0 {
		t.Fatalf("no worktree may be created: %q", out)
	}
	if _, err := Trust(local); err != nil {
		t.Fatal(err)
	}
	if _, err := StartWork(context.Background(), StartOptions{RepoDir: clone, ConfigRoot: local, Ways: w, Slug: "add-x"}); err != nil {
		t.Fatalf("trusted: %v", err)
	}
}

func TestStartBranchHasNoUpstreamUntilTheFirstPush(t *testing.T) {
	clone := startRepo(t, "flow: minimal\n")
	res, err := startIn(t, clone, StartOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", res.Path, "rev-parse", "--abbrev-ref", "@{u}")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("a branch made from origin/main must not track it, got upstream %q", out)
	}
}
