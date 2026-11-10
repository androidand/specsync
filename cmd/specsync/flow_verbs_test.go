package main

import (
	"os"
	"os/exec"
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

func updateRepo(t *testing.T) (clone string, env []string, advance func()) {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	gitIn(t, root, "init", "-q", "--bare", "-b", "main", origin)
	clone = filepath.Join(root, "clone")
	gitIn(t, root, "clone", "-q", origin, clone)
	_, env = firstUseEnv(t)
	env = withIdentity(env)
	gitIn(t, clone, "checkout", "-q", "-b", "main")
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("a\n"), 0o644)
	commitAll(t, clone, "a")
	gitIn(t, clone, "push", "-q", "-u", "origin", "main")
	gitIn(t, clone, "remote", "set-head", "origin", "main")
	gitIn(t, clone, "checkout", "-q", "-b", "feature")
	os.WriteFile(filepath.Join(clone, "mine.txt"), []byte("mine\n"), 0o644)
	commitAll(t, clone, "mine")
	advance = func() {
		other := filepath.Join(root, "other")
		gitIn(t, root, "clone", "-q", origin, other)
		os.WriteFile(filepath.Join(other, "m.txt"), []byte("m\n"), 0o644)
		commitAll(t, other, "m")
		gitIn(t, other, "push", "-q", "origin", "main")
	}
	return clone, env, advance
}

func TestUpdatePlansWithoutApplyAndAppliesWithIt(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	clone, env, advance := updateRepo(t)
	advance()
	head := func() string {
		out, _ := runIn(t, bin, clone, env, "version")
		_ = out
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = clone
		b, _ := cmd.Output()
		return string(b)
	}
	before := head()
	out, err := runIn(t, bin, clone, env, "update", "-fetch")
	if err != nil || !strings.Contains(out, "nothing changed") || !strings.Contains(out, "git rebase") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if head() != before {
		t.Fatal("a plan must not move HEAD")
	}
	out, err = runIn(t, bin, clone, env, "update", "-apply")
	if err != nil || !strings.Contains(out, "not pushed") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if head() == before {
		t.Fatal("-apply must rebase")
	}
	if out, err := runIn(t, bin, clone, env, "update", "-push"); err == nil || !strings.Contains(out, "-push needs -apply") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

// prRepo is a feature branch with a spec, pushed, with gates trusted and
// stamped, and a gh that records `pr create` calls.
func prRepo(t *testing.T, cfg string) (repo string, env []string, ghLog string) {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	gitIn(t, root, "init", "-q", "--bare", "-b", "main", origin)
	repo = filepath.Join(root, "clone")
	gitIn(t, root, "clone", "-q", origin, repo)
	_, env = firstUseEnv(t)
	env = withIdentity(env)
	env = append(env, "SPECSYNC_TRUST_FILE="+filepath.Join(t.TempDir(), "trust.json"))
	gitIn(t, repo, "checkout", "-q", "-b", "main")
	os.MkdirAll(filepath.Join(repo, "openspec", "changes", "add-x"), 0o755)
	os.WriteFile(filepath.Join(repo, "openspec", "specsync.yml"), []byte(cfg), 0o644)
	os.WriteFile(filepath.Join(repo, "openspec", "changes", "add-x", "proposal.md"), []byte("# Add x\n\n## Why\n\nBecause.\n"), 0o644)
	os.WriteFile(filepath.Join(repo, "openspec", "changes", "add-x", "tasks.md"), []byte("- [x] 1.1 a\n"), 0o644)
	commitAll(t, repo, "base")
	gitIn(t, repo, "push", "-q", "-u", "origin", "main")
	gitIn(t, repo, "remote", "set-head", "origin", "main")
	gitIn(t, repo, "checkout", "-q", "-b", "feat/add-x")
	os.WriteFile(filepath.Join(repo, "x.txt"), []byte("x\n"), 0o644)
	commitAll(t, repo, "work")
	gitIn(t, repo, "push", "-q", "-u", "origin", "feat/add-x")

	ghLog = filepath.Join(t.TempDir(), "gh.log")
	bin := t.TempDir()
	stub := "#!/bin/sh\necho \"$@\" >> " + ghLog + "\ncase \"$1 $2\" in\n  \"pr create\") cat >/dev/null; echo https://github.com/acme/app/pull/9 ;;\n  \"pr list\") echo '[]' ;;\n  *) echo '[]' ;;\nesac\n"
	os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o755)
	env = append(env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return repo, env, ghLog
}

func TestPrPrepFailsNamingTheGatesStep(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, _ := prRepo(t, "gates:\n  - {name: t, run: \"true\"}\n")
	out, err := runIn(t, bin, repo, env, "pr", "prep")
	if err == nil || !strings.Contains(out, "gates") || !strings.Contains(out, "specsync gates") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	runIn(t, bin, repo, env, "trust", "-yes")
	if out, err := runIn(t, bin, repo, env, "gates"); err != nil {
		t.Fatalf("gates: %v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "pr", "prep")
	if err != nil || !strings.Contains(out, "ready") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestPrOpenRefusesMissingConfirmationsAndListsThem(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, ghLog := prRepo(t, "pr:\n  confirm: [tests-run, manual-test]\n")
	out, err := runIn(t, bin, repo, env, "pr", "open", "-confirm", "tests-run")
	if err == nil || !strings.Contains(out, "manual-test") || strings.Contains(out, "missing confirmation: tests-run") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if b, _ := os.ReadFile(ghLog); strings.Contains(string(b), "pr create") {
		t.Fatal("nothing may be created without every confirmation")
	}
}

func TestPrOpenDryRunShowsTheBodyAndCreatesNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, ghLog := prRepo(t, "flow: minimal\n")
	out, err := runIn(t, bin, repo, env, "pr", "open", "-dry-run")
	if err != nil || !strings.Contains(out, "DRY RUN") || !strings.Contains(out, "Add x") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if b, _ := os.ReadFile(ghLog); strings.Contains(string(b), "pr create") {
		t.Fatal("a dry run must not create anything")
	}
}

func TestPrOpenCreatesOnceEverythingHoldsAndRefusesWhenPrepFails(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, ghLog := prRepo(t, "pr:\n  confirm: [tests-run]\n")
	out, err := runIn(t, bin, repo, env, "pr", "open", "-confirm", "tests-run")
	if err != nil || !strings.Contains(out, "pull/9") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	b, _ := os.ReadFile(ghLog)
	if !strings.Contains(string(b), "pr create") || !strings.Contains(string(b), "--head feat/add-x") || !strings.Contains(string(b), "--base main") {
		t.Fatalf("gh calls: %s", b)
	}
	os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("d\n"), 0o644)
	out, err = runIn(t, bin, repo, env, "pr", "open", "-confirm", "tests-run")
	if err == nil || !strings.Contains(out, "uncommitted") {
		t.Fatalf("a failing prep must stop open: err=%v\n%s", err, out)
	}
}

func TestPrOpenRefusesAnUnpushedBranch(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, ghLog := prRepo(t, "flow: minimal\n")
	os.WriteFile(filepath.Join(repo, "y.txt"), []byte("y\n"), 0o644)
	commitAll(t, repo, "more")
	out, err := runIn(t, bin, repo, env, "pr", "open")
	if err == nil || !strings.Contains(out, "push") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if b, _ := os.ReadFile(ghLog); strings.Contains(string(b), "pr create") {
		t.Fatal("an unpushed branch must not be opened")
	}
}

func TestPrNeedsAKnownVerb(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, _ := prRepo(t, "flow: minimal\n")
	if out, err := runIn(t, bin, repo, env, "pr", "merge"); err == nil || !strings.Contains(out, "prep or open") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func startRepoCLI(t *testing.T, cfg string) (repo string, env []string) {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	gitIn(t, root, "init", "-q", "--bare", "-b", "main", origin)
	repo = filepath.Join(root, "clone")
	gitIn(t, root, "clone", "-q", origin, repo)
	_, env = firstUseEnv(t)
	env = withIdentity(env)
	env = append(env, "SPECSYNC_TRUST_FILE="+filepath.Join(t.TempDir(), "trust.json"))
	gitIn(t, repo, "checkout", "-q", "-b", "main")
	os.MkdirAll(filepath.Join(repo, "openspec", "changes", "add-x"), 0o755)
	os.WriteFile(filepath.Join(repo, "openspec", "specsync.yml"), []byte(cfg), 0o644)
	os.WriteFile(filepath.Join(repo, "openspec", "changes", "add-x", "proposal.md"), []byte("# Add x\n"), 0o644)
	os.WriteFile(filepath.Join(repo, "openspec", "changes", "add-x", "tasks.md"), []byte("- [ ] 1.1 a\n"), 0o644)
	commitAll(t, repo, "base")
	gitIn(t, repo, "push", "-q", "-u", "origin", "main")
	gitIn(t, repo, "remote", "set-head", "origin", "main")
	return repo, env
}

func TestStartCreatesThenReusesAndDryRunCreatesNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := startRepoCLI(t, "worktrees:\n  dir: ../wt\n")
	out, err := runIn(t, bin, repo, env, "start", "add-x", "-dry-run")
	if err != nil || !strings.Contains(out, "DRY RUN") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(repo), "wt")); statErr == nil {
		t.Fatal("a dry run must create nothing")
	}
	out, err = runIn(t, bin, repo, env, "start", "add-x")
	if err != nil || !strings.Contains(out, "created") || !strings.Contains(out, "feat/add-x") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "start", "add-x")
	if err != nil || !strings.Contains(out, "already has a worktree") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if out, err := runIn(t, bin, repo, env, "start", "nope"); err == nil || !strings.Contains(out, "no change matches") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestStartRefusesALiveSessionFromTheProvider(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := startRepoCLI(t, "worktrees:\n  dir: ../wt\n")
	if out, err := runIn(t, bin, repo, env, "start", "add-x"); err != nil {
		t.Fatalf("start: %v\n%s", err, out)
	}
	wt := filepath.Join(filepath.Dir(repo), "wt", "clone-add-x")
	env = append(env, `SPECSYNC_SESSIONS_CMD=printf '[{"cwd":"`+wt+`","id":"s-9","name":"mid-task"}]'`)
	out, err := runIn(t, bin, repo, env, "start", "add-x")
	if err == nil || !strings.Contains(out, "s-9") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func newEnv(t *testing.T, issues string) (repo string, env []string) {
	t.Helper()
	repo, env = waysRepo(t, "flow: minimal\n")
	bin := t.TempDir()
	stub := "#!/bin/sh\ncase \"$1 $2\" in\n  \"issue list\") echo '" + issues + "' ;;\n  *) echo '[]' ;;\nesac\n"
	os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o755)
	return repo, append(env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestNewShowsDuplicatesFirstAndCreatesNothingUnlessAnyway(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := newEnv(t, `[{"number":12,"title":"Retry the sync of issues","url":"https://github.com/acme/app/issues/12"}]`)
	out, err := runIn(t, bin, repo, env, "new", "Add a retry to the sync of issues")
	if err == nil || !strings.Contains(out, "#12") || !strings.Contains(out, "nothing created") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "openspec", "changes", "add-a-retry-to-the-sync-of-issues")); statErr == nil {
		t.Fatal("nothing may be created when duplicates are shown")
	}
	out, err = runIn(t, bin, repo, env, "new", "Add a retry to the sync of issues", "-anyway")
	if err != nil || !strings.Contains(out, "issue was not created") || !strings.Contains(out, "## Why") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "openspec", "changes", "add-a-retry-to-the-sync-of-issues", "proposal.md")); statErr != nil {
		t.Fatal("the change must exist locally")
	}
}

func TestNewDryRunAndNoOpenspecCreateNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := newEnv(t, `[]`)
	out, err := runIn(t, bin, repo, env, "new", "Brand new thing", "-dry-run")
	if err != nil || !strings.Contains(out, "DRY RUN") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "openspec", "changes", "brand-new-thing")); statErr == nil {
		t.Fatal("dry run created a change")
	}
	bare, benv := firstUseEnv(t)
	out, err = runIn(t, bin, bare, benv, "new", "Brand new thing")
	if err == nil || !strings.Contains(out, "specsync init") {
		t.Fatalf("a repo with no openspec/ must be pointed at init: err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(bare, "openspec")); statErr == nil {
		t.Fatal("new must not create openspec/")
	}
}

func parkRepo(t *testing.T) (repo string, env []string, change string) {
	t.Helper()
	repo, env = waysRepo(t, "flow: minimal\n")
	env = withIdentity(env)
	change = filepath.Join(repo, "openspec", "changes", "add-x")
	os.MkdirAll(change, 0o755)
	os.WriteFile(filepath.Join(change, "proposal.md"), []byte("# Add x\n"), 0o644)
	os.WriteFile(filepath.Join(change, "tasks.md"), []byte("- [ ] 1.1 a\n"), 0o644)
	gitIn(t, repo, "checkout", "-q", "-b", "feat/add-x")
	commitAll(t, repo, "spec")
	return repo, env, change
}

func TestParkNeverCommitsStrayUntrackedFilesInTheChangeFolder(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, change := parkRepo(t)
	os.WriteFile(filepath.Join(change, "notes.env"), []byte("TOKEN=abc\n"), 0o644)
	out, err := runIn(t, bin, repo, env, "park", "-m", "pausing")
	if err != nil {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if st := gitStatus(t, repo); st != "?? openspec/changes/add-x/notes.env" {
		t.Fatalf("only the stray file may remain, untracked: %q", st)
	}
	show := exec.Command("git", "show", "--name-only", "--format=", "HEAD")
	show.Dir = repo
	b, _ := show.Output()
	if strings.Contains(string(b), "notes.env") || !strings.Contains(string(b), "discoveries.md") {
		t.Fatalf("park may commit only what it wrote:\n%s", b)
	}
}

func TestParkRefusalChangesNothingOnDisk(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, change := parkRepo(t)
	os.WriteFile(filepath.Join(repo, ".env.local"), []byte("A=1\n"), 0o644)
	out, err := runIn(t, bin, repo, env, "park", "-m", "pausing", "-add", ".env.local")
	if err == nil || !strings.Contains(out, ".env.local") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(change, "discoveries.md")); statErr == nil {
		t.Fatal("a refused park must not write the note")
	}
	if _, statErr := os.Stat(filepath.Join(change, ".specsync", "metadata.json")); statErr == nil {
		t.Fatal("a refused park must not write the stage")
	}
}

func TestPrOpenOversizedIsWaivedOnlyForARealOverBudgetResult(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, ghLog := prRepo(t, "flow: minimal\n")
	os.WriteFile(filepath.Join(repo, "y.txt"), []byte("y\n"), 0o644)
	commitAll(t, repo, "more")
	gitIn(t, repo, "push", "-q")
	out, err := runIn(t, bin, repo, env, "pr", "open", "-budget", "commits=1")
	if err == nil || !strings.Contains(out, "over budget") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "pr", "open", "-budget", "commits=abc", "-confirm", "oversized")
	if err == nil || !strings.Contains(out, "size") {
		t.Fatalf("a budget error must not be waived: err=%v\n%s", err, out)
	}
	if b, _ := os.ReadFile(ghLog); strings.Contains(string(b), "pr create") {
		t.Fatal("nothing may be created yet")
	}
	out, err = runIn(t, bin, repo, env, "pr", "open", "-budget", "commits=1", "-confirm", "oversized")
	if err != nil || !strings.Contains(out, "pull/9") {
		t.Fatalf("a real oversized result is waived by -confirm oversized: err=%v\n%s", err, out)
	}
}

func TestPrFlagsWorkInAnyPosition(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env, ghLog := prRepo(t, "pr:\n  confirm: [tests-run]\n")
	out, err := runIn(t, bin, repo, env, "pr", "-dry-run", "-confirm", "tests-run", "open")
	if err != nil || !strings.Contains(out, "DRY RUN") {
		t.Fatalf("flags before the verb: err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "pr", "open", "-confirm", "tests-run", "-dry-run")
	if err != nil || !strings.Contains(out, "DRY RUN") {
		t.Fatalf("flags after the verb: err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "pr", "prep", "-json")
	if err != nil || !strings.Contains(out, `"passed"`) {
		t.Fatalf("prep -json: err=%v\n%s", err, out)
	}
	if b, _ := os.ReadFile(ghLog); strings.Contains(string(b), "pr create") {
		t.Fatal("dry runs create nothing")
	}
	if out, err := runIn(t, bin, repo, env, "pr", "open", "prep"); err == nil || !strings.Contains(out, "prep or open") {
		t.Fatalf("two verbs are an error: err=%v\n%s", err, out)
	}
}
