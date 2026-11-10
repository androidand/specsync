package specsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func commitAs(t *testing.T, dir, email, name, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutT(t, dir, "add", name)
	gitOutT(t, dir, "-c", "user.email="+email, "commit", "-q", "-m", msg)
}

func TestCleanupResetBranchIsNotProvenByItsOldCommits(t *testing.T) {
	f := newCleanupFixture(t)
	before := f.head()
	gitOutT(t, f.wt, "reset", "-q", "--hard", "main")
	row := f.row(nil)
	it := planOne(t, row, CleanupOptions{})
	if it.Status == CleanupCandidate || !strings.Contains(it.Reason, "branch was reset") || !strings.Contains(it.Reason, before[:8]) {
		t.Fatalf("discarded commits must not make a branch look finished: %+v", it)
	}
	merged := f.row(&ChangeRequest{Number: 7, State: "merged", Base: "main", HeadSHA: gitOutT(t, f.wt, "rev-parse", "HEAD")})
	if it := planOne(t, merged, CleanupOptions{}); it.Status != CleanupCandidate {
		t.Fatalf("a merged request still proves it: %+v", it)
	}
}

func TestApplyCleanupRecordsHowToRestoreTheBranchAndDropsItsConfig(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.withRemote()
	row := finishedRow(t, f)
	head := row.Local.Head
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	it := only(t, ApplyCleanup(context.Background(), items, CleanupOptions{AssumeNoSessions: true}))
	if it.Error != "" {
		t.Fatal(it.Error)
	}
	want := "git branch feat/42-thing " + head
	if !strings.Contains(it.Recovery, want) || !strings.Contains(strings.Join(it.Done, "\n"), want) {
		t.Fatalf("recovery = %q, done = %v", it.Recovery, it.Done)
	}
	cmd := exec.Command("git", "config", "--get-regexp", `^branch\.feat/42-thing\.`)
	cmd.Dir = f.main
	if out, _ := cmd.Output(); len(out) != 0 {
		t.Fatalf("a later branch of the same name must not inherit the upstream: %s", out)
	}
}

func TestShellHookRunsInTheGivenDirectoryUnderATimeoutAndKillsTheProcessGroup(t *testing.T) {
	old := hookTimeout
	hookTimeout = 300 * time.Millisecond
	defer func() { hookTimeout = old }()
	start := time.Now()
	err := runShellHook(context.Background(), t.TempDir(), "sleep 30 & sleep 30", nil)
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("the hook must be stopped at the timeout: err=%v after %s", err, time.Since(start))
	}
}

func TestApplyCleanupRunsHooksInTheMainCheckout(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.writeConfig("cleanup:\n  hooks:\n    - make stop\n")
	if _, err := Trust(f.main); err != nil {
		t.Fatal(err)
	}
	row := finishedRow(t, f)
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	var dir string
	opts := CleanupOptions{AssumeNoSessions: true, RunHook: func(_ context.Context, d, _ string, _ []string) error { dir = d; return nil }}
	if it := only(t, ApplyCleanup(context.Background(), items, opts)); it.Error != "" {
		t.Fatal(it.Error)
	}
	if !SameFolder(dir, f.main) {
		t.Fatalf("branch-controlled code must not run with the worktree as its directory: %s", dir)
	}
}

func TestCleanupConfigIssuesFailClosed(t *testing.T) {
	for name, cfg := range map[string]string{
		"unknown key": "cleanup:\n  bogus: 1\n",
		"bad glob":    "cleanup:\n  protect:\n    - '[x'\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := newCleanupFixture(t)
			f.writeConfig(cfg)
			it := planOne(t, finishedRow(t, f), CleanupOptions{})
			if it.Status != CleanupSkipped || !strings.Contains(it.Reason, "fix openspec/specsync.yml") {
				t.Fatalf("a config with problems must never fall back to defaults: %+v", it)
			}
		})
	}
}

func TestCleanupProtectGlobsCrossSlashes(t *testing.T) {
	f := newCleanupFixture(t)
	row := finishedRow(t, f)
	row.Branch = "release/1.0/rc1"
	if it := planOne(t, row, CleanupOptions{}); it.Status != CleanupSkipped || !strings.Contains(it.Reason, "protected") {
		t.Fatalf("release/* must protect nested branches: %+v", it)
	}
}

func TestCleanupSharedBranchIsDetectedWhenTheHeadIsAlreadyInTheBase(t *testing.T) {
	f := newCleanupFixture(t)
	commitAs(t, f.wt, "second@example.com", "b.txt", "from someone else")
	gitOutT(t, f.main, "merge", "-q", "--ff-only", "feat/42-thing")
	it := planOne(t, f.row(nil), CleanupOptions{})
	if it.Status != CleanupSkipped || !strings.Contains(it.Reason, "several authors") {
		t.Fatalf("got %+v", it)
	}
}

func TestApplyCleanupKeepsTheRemoteBranchWhileAnOpenRequestTargetsIt(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	bare := remoteFor(t, f)
	row := finishedRow(t, f)
	stacked := ChangeTopology{Branch: "feat/stacked", Repo: row.Repo, Store: row.Store, PR: &ChangeRequest{Number: 9, State: "open", Base: "feat/42-thing"}}
	items := PlanCleanup(context.Background(), []ChangeTopology{row, stacked}, CleanupOptions{})
	var cand []CleanupItem
	for _, it := range items {
		if it.Status == CleanupCandidate {
			cand = append(cand, it)
		}
	}
	it := only(t, ApplyCleanup(context.Background(), cand, CleanupOptions{AssumeNoSessions: true}))
	if it.Error != "" {
		t.Fatal(it.Error)
	}
	if gitOutT(t, f.main, "ls-remote", bare, "refs/heads/feat/42-thing") == "" {
		t.Fatal("an open request targets this branch, so the remote branch must stay")
	}
	if !strings.Contains(strings.Join(it.Done, "\n"), "open request #9") {
		t.Fatalf("the reason must be reported: %v", it.Done)
	}
}

func TestCleanupIgnoredBuildDirAllowlistIsTopLevelOnly(t *testing.T) {
	f := newCleanupFixture(t)
	writeCommit(t, f.wt, ".gitignore", "build/\nout.o\n")
	if err := os.MkdirAll(filepath.Join(f.wt, "src", "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.wt, "src", "build", "out.o"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := ignoredFiles(context.Background(), f.wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || !strings.Contains(names[0], "src/build") {
		t.Fatalf("a nested build directory is real data: %v", names)
	}
}

func TestCleanupProtectsTheBaseWhateverTheRemoteIsCalled(t *testing.T) {
	f := newCleanupFixture(t)
	gitOutT(t, f.main, "remote", "add", "upstream", "https://github.com/acme/upstream.git")
	row := finishedRow(t, f)
	row.Branch = "integration"
	row.Local.Base = "upstream/integration"
	if got := protectedBranch(context.Background(), row); !strings.Contains(got, "protected") {
		t.Fatalf("a base on another remote is still the base: %q", got)
	}
}

type countingCache struct{ puts int }

func (c *countingCache) Get(string) (ChangeRequestSet, bool) { return ChangeRequestSet{}, false }
func (c *countingCache) Put(string, ChangeRequestSet)        { c.puts++ }

func objectCount(t *testing.T, dir string) string {
	t.Helper()
	return gitOutT(t, dir, "count-objects", "-v")
}

func TestCleanupDryRunWritesNothingNotEvenObjectsOrTheRequestCache(t *testing.T) {
	trustFile(t)
	root := t.TempDir()
	f := &cleanupFixture{t: t, main: filepath.Join(root, "app"), wt: filepath.Join(root, "app-wt")}
	gitRepo(t, f.main, "https://github.com/acme/app.git")
	gitOutT(t, f.main, "worktree", "add", "-q", "-b", "feat/42-thing", f.wt)
	writeCommit(t, f.wt, "a.txt", "work")
	writeCommit(t, f.main, "base.txt", "base moved on")
	gitOutT(t, f.main, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	gitOutT(t, f.main, "update-ref", "refs/remotes/origin/main", "main")
	cache := &countingCache{}
	opts := CleanupOptions{Overview: OverviewOptions{
		Workspace: WorkspaceOptions{Roots: []string{root}},
		Topology: TopologyOptions{
			Cache: cache,
			run:   func(context.Context, ...string) (string, error) { return "", os.ErrNotExist },
			changeRequests: func(context.Context, string) (*ChangeRequestSet, error) {
				return &ChangeRequestSet{Requests: []ChangeRequest{{Number: 7, State: "open", Branch: "feat/42-thing", Base: "main"}}}, nil
			},
		},
	}}
	before := objectCount(t, f.main)
	if _, err := RunCleanup(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if after := objectCount(t, f.main); after != before {
		t.Fatalf("a dry run must not write git objects:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if cache.puts != 0 {
		t.Fatalf("a dry run must not write the request cache (%d writes)", cache.puts)
	}
}
