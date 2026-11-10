package specsync

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func finishedRow(t *testing.T, f *cleanupFixture) ChangeTopology {
	t.Helper()
	f.squashIntoMain()
	row := f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: f.head()})
	row.Issue = &TopologyIssue{State: "closed"}
	return row
}

func planOne(t *testing.T, row ChangeTopology, opts CleanupOptions) CleanupItem {
	t.Helper()
	return only(t, PlanCleanup(context.Background(), []ChangeTopology{row}, opts))
}

func TestCleanupNeverRemovesAProtectedBranch(t *testing.T) {
	for _, name := range []string{"main", "develop"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			f := &cleanupFixture{t: t, main: filepath.Join(root, "app"), wt: filepath.Join(root, "wt")}
			gitRepo(t, f.main, "https://github.com/acme/app.git")
			if name == "main" {
				gitOutT(t, f.main, "checkout", "-q", "-b", "scratch")
				gitOutT(t, f.main, "worktree", "add", "-q", f.wt, "main")
			} else {
				gitOutT(t, f.main, "worktree", "add", "-q", "-b", "develop", f.wt)
				writeCommit(t, f.wt, "d.txt", "develop work")
				gitOutT(t, f.main, "merge", "-q", "--ff-only", "develop")
			}
			row := ChangeTopology{Slug: "x", Store: f.main, Branch: name, Worktree: f.wt}
			row.Local, _ = CollectLocalState(context.Background(), f.wt, LocalStateOptions{Base: "main"})
			it := planOne(t, row, CleanupOptions{})
			if it.Status == CleanupCandidate || !strings.Contains(it.Reason, "protected") {
				t.Fatalf("a protected branch is never a candidate: %+v", it)
			}
			it.Status = CleanupCandidate
			it.row = row
			if got := ApplyCleanup(context.Background(), []CleanupItem{it}, CleanupOptions{})[0]; got.Error == "" {
				t.Fatalf("apply must refuse a protected branch even for a forged candidate: %+v", got)
			}
			if !f.branchExists(f.main, name) {
				t.Fatal("the branch must still exist")
			}
		})
	}
}

func TestCleanupIgnoredFilesAreRealData(t *testing.T) {
	f := newCleanupFixture(t)
	writeCommit(t, f.wt, ".gitignore", "*.log\nnode_modules/\ndist/\n")
	if err := os.WriteFile(filepath.Join(f.wt, "debug.log"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	row := finishedRow(t, f)
	it := planOne(t, row, CleanupOptions{})
	if it.Status != CleanupSkipped || !strings.Contains(it.Reason, "has ignored files: debug.log") {
		t.Fatalf("got %+v", it)
	}
	if it := planOne(t, row, CleanupOptions{IncludeIgnored: true}); it.Status != CleanupCandidate {
		t.Fatalf("-include-ignored overrides: %+v", it)
	}
	if err := os.Remove(filepath.Join(f.wt, "debug.log")); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"node_modules", "dist"} {
		if err := os.MkdirAll(filepath.Join(f.wt, d, "x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.wt, d, "x", "o.js"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if it := planOne(t, row, CleanupOptions{}); it.Status != CleanupCandidate {
		t.Fatalf("build output directories alone do not block: %+v", it)
	}
}

func TestApplyCleanup_RechecksIgnoredFilesBeforeRemoving(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	writeCommit(t, f.wt, ".gitignore", "*.log\n")
	row := finishedRow(t, f)
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	if err := os.WriteFile(filepath.Join(f.wt, "late.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	it := only(t, ApplyCleanup(context.Background(), items, CleanupOptions{}))
	if it.Error == "" || !strings.Contains(it.Error, "ignored") {
		t.Fatalf("got %+v", it)
	}
	if _, err := os.Stat(f.wt); err != nil {
		t.Fatal("the worktree must stay")
	}
}

func remoteFor(t *testing.T, f *cleanupFixture) string {
	t.Helper()
	f.writeConfig("cleanup:\n  remote_branch: delete\n")
	return f.withRemote()
}

func TestApplyCleanup_RemoteBranchIsDeletedOnlyWhenItsTipIsTheHeadBeingRemoved(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	bare := remoteFor(t, f)
	row := finishedRow(t, f)
	clone := filepath.Join(filepath.Dir(f.main), "other")
	gitOutT(t, filepath.Dir(f.main), "clone", "-q", bare, clone)
	gitOutT(t, clone, "checkout", "-q", "feat/42-thing")
	writeCommit(t, clone, "extra.txt", "pushed by someone else")
	gitOutT(t, clone, "push", "-q", "origin", "feat/42-thing")
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	it := only(t, ApplyCleanup(context.Background(), items, CleanupOptions{}))
	if it.Error != "" {
		t.Fatal(it.Error)
	}
	if gitOutT(t, f.main, "ls-remote", bare, "refs/heads/feat/42-thing") == "" {
		t.Fatal("a remote branch that moved on must be kept")
	}
	if !strings.Contains(strings.Join(it.Done, "\n"), "kept remote branch") {
		t.Fatalf("the reason must be reported: %v", it.Done)
	}
}

func TestApplyCleanup_RemoteBranchIsKeptWhenTheUpstreamHasAnotherName(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.writeConfig("cleanup:\n  remote_branch: delete\n")
	bare := f.withRemote()
	gitOutT(t, f.wt, "push", "-q", "origin", "HEAD:refs/heads/other-name")
	gitOutT(t, f.wt, "branch", "-q", "--set-upstream-to=origin/other-name")
	row := finishedRow(t, f)
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	it := only(t, ApplyCleanup(context.Background(), items, CleanupOptions{}))
	if it.Error != "" {
		t.Fatal(it.Error)
	}
	if gitOutT(t, f.main, "ls-remote", bare, "refs/heads/other-name") == "" || gitOutT(t, f.main, "ls-remote", bare, "refs/heads/feat/42-thing") == "" {
		t.Fatal("neither remote branch may be deleted on the strength of the local name")
	}
}

func TestWaysTrustedCommandsIncludeGatesAndCleanupHooks(t *testing.T) {
	w, issues := ParseWays([]byte("gates:\n  - name: test\n    run: make test\ncleanup:\n  hooks:\n    - make stop\n"))
	if len(issues) != 0 {
		t.Fatalf("%+v", issues)
	}
	if got := w.TrustedCommands(); !reflect.DeepEqual(got, []string{"make test", "make stop"}) {
		t.Fatalf("got %v", got)
	}
}

func TestApplyCleanup_UntrustedHooksAreReportedWithTheirCommands(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.writeConfig("cleanup:\n  hooks:\n    - make stop\n")
	row := finishedRow(t, f)
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	var ran []string
	it := only(t, ApplyCleanup(context.Background(), items, hookOpts(&ran)))
	if len(ran) != 0 {
		t.Fatalf("ran %v", ran)
	}
	if got := strings.Join(it.Done, "\n"); !strings.Contains(got, "$ make stop") || !strings.Contains(got, "specsync trust -yes") {
		t.Fatalf("the RequireTrust message must show the commands: %q", got)
	}
}

func TestCleanupSessionsAreSharedByEveryRowOfAWorktree(t *testing.T) {
	f := newCleanupFixture(t)
	row := finishedRow(t, f)
	other := row
	other.Slug, other.NoChange = "", true
	other.Sessions = []SessionEntry{{CWD: f.wt, Name: "agent"}}
	for _, rows := range [][]ChangeTopology{{row, other}, {other, row}} {
		items := PlanCleanup(context.Background(), rows, CleanupOptions{})
		for _, it := range items {
			if it.Status == CleanupCandidate || !strings.Contains(it.Reason, "session") {
				t.Fatalf("a session on any row of the worktree protects it: %+v", it)
			}
		}
	}
}

func TestApplyCleanup_RechecksLiveSessionsBeforeRemoving(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	row := finishedRow(t, f)
	opts := CleanupOptions{Overview: OverviewOptions{SessionsCmd: `printf '[{"cwd":"` + f.wt + `","name":"late agent"}]'`}}
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	it := only(t, ApplyCleanup(context.Background(), items, opts))
	if it.Error == "" || !strings.Contains(it.Error, "late agent") {
		t.Fatalf("a session that started after planning must stop the removal: %+v", it)
	}
	if _, err := os.Stat(f.wt); err != nil {
		t.Fatal("the worktree must stay")
	}
}

func TestSessionsCheckNoteSaysWhenSessionsWereNotChecked(t *testing.T) {
	cases := []struct {
		name   string
		opts   OverviewOptions
		status []string
		want   string
	}{
		{"none configured", OverviewOptions{}, nil, "no session provider"},
		{"offline", OverviewOptions{SessionsCmd: "x", Topology: TopologyOptions{LocalOnly: true}}, nil, "-state off"},
		{"failed", OverviewOptions{SessionsCmd: "x"}, []string{"session provider timed out after 5s"}, "timed out"},
		{"checked", OverviewOptions{SessionsCmd: "x"}, nil, ""},
	}
	for _, c := range cases {
		note := sessionsCheckNote(c.opts, c.status)
		if (c.want == "") != (note == "") || !strings.Contains(note, c.want) {
			t.Errorf("%s: %q", c.name, note)
		}
	}
}

func TestApplyCleanup_HooksRunInTheWorktreeBeforeItIsRemoved(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.writeConfig("cleanup:\n  hooks:\n    - make stop\n")
	if _, err := Trust(f.main); err != nil {
		t.Fatal(err)
	}
	row := finishedRow(t, f)
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	var existed bool
	var gotEnv []string
	opts := CleanupOptions{RunHook: func(_ context.Context, dir, _ string, env []string) error {
		_, err := os.Stat(dir)
		existed = err == nil && SameFolder(dir, f.wt)
		gotEnv = env
		return nil
	}}
	it := only(t, ApplyCleanup(context.Background(), items, opts))
	if it.Error != "" {
		t.Fatal(it.Error)
	}
	if !existed {
		t.Fatal("the hook must run with the worktree as its directory while it still exists")
	}
	if !strings.Contains(strings.Join(gotEnv, " "), "SPECSYNC_WORKTREE="+f.wt) {
		t.Fatalf("env: %v", gotEnv)
	}
}

func TestApplyCleanup_AFailingHookOrArchiveLeavesTheWorkInPlace(t *testing.T) {
	trustFile(t)
	for _, name := range []string{"hook", "archive"} {
		t.Run(name, func(t *testing.T) {
			f := newCleanupFixture(t)
			f.writeConfig("cleanup:\n  archive: true\n  hooks:\n    - make stop\n")
			if _, err := Trust(f.main); err != nil {
				t.Fatal(err)
			}
			row := finishedRow(t, f)
			opts := CleanupOptions{
				RunHook: func(context.Context, string, string, []string) error {
					if name == "hook" {
						return os.ErrPermission
					}
					return nil
				},
				ArchiveChange: func(context.Context, ChangeTopology) (string, error) {
					if name == "archive" {
						return "", os.ErrInvalid
					}
					return "archived", nil
				},
			}
			items := PlanCleanup(context.Background(), []ChangeTopology{row}, opts)
			it := only(t, ApplyCleanup(context.Background(), items, opts))
			if it.Error == "" {
				t.Fatalf("the failure must be reported: %+v", it)
			}
			if _, err := os.Stat(f.wt); err != nil || !f.branchExists(f.main, "feat/42-thing") {
				t.Fatal("a failed hook or archive leaves the worktree and branch untouched")
			}
		})
	}
}

func TestCleanupEmptyBranchIsNotFinishedWork(t *testing.T) {
	for _, how := range []string{"reset", "merge", "fresh"} {
		t.Run(how, func(t *testing.T) {
			root := t.TempDir()
			f := &cleanupFixture{t: t, main: filepath.Join(root, "app"), wt: filepath.Join(root, "wt")}
			gitRepo(t, f.main, "https://github.com/acme/app.git")
			gitOutT(t, f.main, "worktree", "add", "-q", "-b", "feat/42-thing", f.wt)
			writeCommit(t, f.main, "base.txt", "base moved")
			switch how {
			case "reset":
				gitOutT(t, f.wt, "reset", "-q", "--hard", "main")
			case "merge":
				gitOutT(t, f.wt, "merge", "-q", "--ff-only", "main")
			}
			if items := plan(t, []ChangeTopology{f.row(nil)}, nil); len(items) != 0 {
				t.Fatalf("an empty branch brought up to the base is not finished work: %+v", items)
			}
		})
	}
}

func TestCleanupBranchWithACommitThatIsInTheBaseIsFinished(t *testing.T) {
	f := newCleanupFixture(t)
	gitOutT(t, f.main, "merge", "-q", "--ff-only", "feat/42-thing")
	if it := only(t, plan(t, []ChangeTopology{f.row(nil)}, nil)); it.Status != CleanupCandidate {
		t.Fatalf("got %+v", it)
	}
}

var _ = time.Now
