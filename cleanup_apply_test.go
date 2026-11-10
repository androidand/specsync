package specsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func (f *cleanupFixture) writeConfig(yaml string) {
	f.t.Helper()
	p := filepath.Join(f.main, SpecSyncConfigPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *cleanupFixture) withRemote() string {
	f.t.Helper()
	bare := filepath.Join(filepath.Dir(f.main), "origin.git")
	gitOutT(f.t, filepath.Dir(f.main), "init", "-q", "--bare", "-b", "main", bare)
	gitOutT(f.t, f.main, "remote", "set-url", "origin", bare)
	gitOutT(f.t, f.main, "push", "-q", "origin", "main")
	gitOutT(f.t, f.wt, "push", "-q", "-u", "origin", "feat/42-thing")
	return bare
}

func (f *cleanupFixture) branchExists(dir, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "-q", "refs/heads/"+branch)
	cmd.Dir = dir
	return cmd.Run() == nil
}

func trustFile(t *testing.T) {
	t.Helper()
	t.Setenv(TrustFileEnv, filepath.Join(t.TempDir(), "trust.json"))
}

func mergedCandidate(t *testing.T, f *cleanupFixture) []CleanupItem {
	t.Helper()
	f.squashIntoMain()
	row := f.row(&ChangeRequest{Number: 7, State: "merged", Base: "main", HeadSHA: f.head()})
	row.Issue = &TopologyIssue{State: "closed"}
	return PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
}

func TestApplyCleanup_RemovesWorktreeThenBranchAndKeepsTheRemoteByDefault(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	bare := f.withRemote()
	items := mergedCandidate(t, f)
	rep := ApplyCleanup(context.Background(), items, CleanupOptions{})
	it := only(t, rep)
	if it.Error != "" {
		t.Fatal(it.Error)
	}
	if _, err := os.Stat(f.wt); err == nil {
		t.Fatal("the worktree must be gone")
	}
	if f.branchExists(f.main, "feat/42-thing") {
		t.Fatal("the local branch must be deleted")
	}
	if gitOutT(t, f.main, "ls-remote", bare, "refs/heads/feat/42-thing") == "" {
		t.Fatal("remote_branch defaults to keep, so the remote branch must stay")
	}
	if len(it.Done) < 2 || !strings.HasPrefix(it.Done[0], "removed worktree") || !strings.HasPrefix(it.Done[1], "deleted local branch") {
		t.Fatalf("steps out of order: %v", it.Done)
	}
}

func TestApplyCleanup_DeletesTheRemoteBranchWhenConfigured(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	bare := f.withRemote()
	f.writeConfig("cleanup:\n  remote_branch: delete\n")
	items := mergedCandidate(t, f)
	if it := only(t, ApplyCleanup(context.Background(), items, CleanupOptions{})); it.Error != "" {
		t.Fatal(it.Error)
	}
	if got := gitOutT(t, f.main, "ls-remote", bare, "refs/heads/feat/42-thing"); got != "" {
		t.Fatalf("the remote branch must be deleted, got %q", got)
	}
}

func hookOpts(ran *[]string) CleanupOptions {
	return CleanupOptions{RunHook: func(_ context.Context, dir, command string, env []string) error {
		*ran = append(*ran, command+"@"+filepath.Base(dir))
		return nil
	}}
}

func TestCleanupSkipsAnItemWhoseHookCannotRunBecauseTheConfigIsNotTrusted(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.writeConfig("cleanup:\n  hooks:\n    - make stop\n")
	var ran []string
	opts := hookOpts(&ran)
	items := mergedCandidate(t, f)
	it := only(t, items)
	if it.Status != CleanupSkipped || !strings.Contains(it.Reason, "not trusted") || !strings.Contains(it.Reason, "make stop") {
		t.Fatalf("an item whose hooks cannot run is skipped with the commands shown: %+v", it)
	}
	forged := it
	forged.Status = CleanupCandidate
	got := only(t, ApplyCleanup(context.Background(), []CleanupItem{forged}, opts))
	if got.Error == "" || len(ran) != 0 {
		t.Fatalf("apply must refuse too: %+v ran=%v", got, ran)
	}
	if _, err := os.Stat(f.wt); err != nil {
		t.Fatal("the worktree must stay")
	}
}

func TestApplyCleanup_RunsTheHookBeforeTheOtherStepsWhenTrusted(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.writeConfig("cleanup:\n  hooks:\n    - make stop\n")
	if _, err := Trust(f.main); err != nil {
		t.Fatal(err)
	}
	var ran []string
	opts := hookOpts(&ran)
	items := mergedCandidate(t, f)
	it := only(t, ApplyCleanup(context.Background(), items, opts))
	if !reflect.DeepEqual(ran, []string{"make stop@app"}) {
		t.Fatalf("ran = %v", ran)
	}
	if !strings.HasPrefix(it.Done[0], "ran hook") {
		t.Fatalf("hooks run first, while the worktree still exists: %v", it.Done)
	}
}

func TestApplyCleanup_EachRepoDecidesTrustForItsOwnHooks(t *testing.T) {
	trustFile(t)
	a, b := newCleanupFixture(t), newCleanupFixture(t)
	for _, f := range []*cleanupFixture{a, b} {
		f.writeConfig("cleanup:\n  hooks:\n    - make stop\n")
	}
	if _, err := Trust(a.main); err != nil {
		t.Fatal(err)
	}
	var ran []string
	opts := hookOpts(&ran)
	items := append(mergedCandidate(t, a), mergedCandidate(t, b)...)
	rep := ApplyCleanup(context.Background(), items, opts)
	if len(ran) != 1 || ran[0] != "make stop@app" {
		t.Fatalf("only the trusted repo's hook runs: %v", ran)
	}
	skipped := 0
	for _, it := range rep {
		if it.Status == CleanupSkipped {
			skipped++
		}
	}
	if skipped != 1 {
		t.Fatalf("the untrusted repo's item is skipped: %+v", rep)
	}
}

func TestApplyCleanup_RechecksSafetyBeforeRemoving(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	items := mergedCandidate(t, f)
	if err := os.WriteFile(filepath.Join(f.wt, "late.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	it := only(t, ApplyCleanup(context.Background(), items, CleanupOptions{}))
	if it.Error == "" || !strings.Contains(it.Error, "uncommitted") {
		t.Fatalf("work added after planning must stop the removal: %+v", it)
	}
	if _, err := os.Stat(f.wt); err != nil {
		t.Fatal("the worktree must still exist")
	}
	writeCommit(t, f.wt, "c.txt", "later")
	if err := os.Remove(filepath.Join(f.wt, "late.txt")); err != nil {
		t.Fatal(err)
	}
	it = only(t, ApplyCleanup(context.Background(), items, CleanupOptions{}))
	if it.Error == "" || !f.branchExists(f.main, "feat/42-thing") {
		t.Fatalf("a new commit after planning must stop the removal: %+v", it)
	}
}

func TestApplyCleanup_NeverActsOnSkippedOrAttentionItems(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	row := f.row(nil)
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	rep := ApplyCleanup(context.Background(), items, CleanupOptions{})
	if len(rep) != 1 || len(rep[0].Done) != 0 {
		t.Fatalf("got %+v", rep)
	}
	if _, err := os.Stat(f.wt); err != nil {
		t.Fatal("an attention item must stay in place")
	}
}

func TestParseWaysReadsTheCleanupSection(t *testing.T) {
	w, issues := ParseWays([]byte("cleanup:\n  remote_branch: delete\n  archive: false\n  min_age_days: 14\n  hooks:\n    - make stop\n    - rm notes\n  protect:\n    - keep/*\n"))
	if len(issues) != 0 {
		t.Fatalf("issues: %+v", issues)
	}
	want := CleanupConfig{RemoteBranch: "delete", Archive: false, MinAgeDays: 14, Hooks: []string{"make stop", "rm notes"}, Protect: []string{"keep/*"}}
	if !reflect.DeepEqual(w.Cleanup, want) {
		t.Fatalf("got %+v", w.Cleanup)
	}
	d, _ := ParseWays(nil)
	if d.Cleanup.RemoteBranch != "keep" || d.Cleanup.Archive || d.Cleanup.MinAgeDays != 0 || len(d.Cleanup.Hooks) != 0 {
		t.Fatalf("defaults: %+v", d.Cleanup)
	}
	for _, bad := range []string{"cleanup:\n  min_age_days: soon\n", "cleanup:\n  hooks: make\n", "cleanup:\n  remote_branch: maybe\n"} {
		if _, issues := ParseWays([]byte(bad)); len(issues) == 0 {
			t.Errorf("%q must be reported", bad)
		}
	}
}

func TestCleanupArchiveIsAFollowUpLineNeverARunAction(t *testing.T) {
	trustFile(t)
	for _, c := range []struct {
		name, cfg, issue string
		want             bool
	}{
		{"configured and closed", "cleanup:\n  archive: true\n", "closed", true},
		{"configured but open", "cleanup:\n  archive: true\n", "open", false},
		{"not configured", "", "closed", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newCleanupFixture(t)
			if c.cfg != "" {
				f.writeConfig(c.cfg)
			}
			f.squashIntoMain()
			row := f.row(&ChangeRequest{Number: 7, State: "merged", Base: "main", HeadSHA: f.head()})
			row.Issue = &TopologyIssue{State: c.issue}
			items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
			it := only(t, ApplyCleanup(context.Background(), items, CleanupOptions{}))
			if it.Error != "" {
				t.Fatal(it.Error)
			}
			got := strings.Join(it.FollowUps, "\n")
			if c.want != strings.Contains(got, "archive pending: specsync archive -change thing") {
				t.Fatalf("follow-ups = %q", got)
			}
			if _, err := os.Stat(filepath.Join(f.main, "openspec")); c.cfg == "" && err == nil {
				t.Fatal("cleanup must not touch the main checkout's specs")
			}
		})
	}
}
