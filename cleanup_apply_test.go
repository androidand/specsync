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
	row := f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: f.head()})
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

func TestApplyCleanup_ArchivesTheChangeOnlyWhenItsIssueIsClosed(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.writeConfig("cleanup:\n  archive: true\n")
	f.squashIntoMain()
	var archived []string
	opts := CleanupOptions{ArchiveChange: func(_ context.Context, row ChangeTopology) (string, error) {
		archived = append(archived, row.Slug)
		return "archived", nil
	}}
	open := f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: f.head()})
	open.Issue = &TopologyIssue{State: "open"}
	items := PlanCleanup(context.Background(), []ChangeTopology{open}, opts)
	if it := only(t, ApplyCleanup(context.Background(), items, opts)); it.Error != "" {
		t.Fatal(it.Error)
	}
	if len(archived) != 0 {
		t.Fatalf("an open issue must not be archived: %v", archived)
	}
}

func TestApplyCleanup_ArchivesWhenTheIssueIsClosed(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.writeConfig("cleanup:\n  archive: true\n")
	f.squashIntoMain()
	var archived []string
	opts := CleanupOptions{ArchiveChange: func(_ context.Context, row ChangeTopology) (string, error) {
		archived = append(archived, row.Slug)
		return "archived change", nil
	}}
	row := f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: f.head()})
	row.Issue = &TopologyIssue{State: "closed"}
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, opts)
	it := only(t, ApplyCleanup(context.Background(), items, opts))
	if !reflect.DeepEqual(archived, []string{"thing"}) {
		t.Fatalf("archived = %v (%+v)", archived, it)
	}
	if last := it.Done[len(it.Done)-1]; last != "archived change" {
		t.Fatalf("archive comes after the branch steps: %v", it.Done)
	}
}

func hookOpts(ran *[]string) CleanupOptions {
	return CleanupOptions{RunHook: func(_ context.Context, dir, command string, env []string) error {
		*ran = append(*ran, command+"@"+filepath.Base(dir))
		return nil
	}}
}

func TestApplyCleanup_RunsHooksOnlyWhenTheConfigIsTrusted(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.writeConfig("cleanup:\n  hooks:\n    - make stop\n")
	var ran []string
	opts := hookOpts(&ran)
	items := mergedCandidate(t, f)

	untrusted := ApplyCleanup(context.Background(), items, opts)
	if len(ran) != 0 {
		t.Fatalf("an untrusted hook must not run: %v", ran)
	}
	got := strings.Join(only(t, untrusted).Done, "\n")
	if !strings.Contains(got, "not run") || !strings.Contains(got, "make stop") || !strings.Contains(got, "specsync trust") {
		t.Fatalf("the hook must be printed with how to trust it: %q", got)
	}
	if _, err := os.Stat(f.wt); err == nil {
		t.Fatal("the other steps still apply when the config is untrusted")
	}
}

func TestApplyCleanup_RunsTheHookAfterTheOtherStepsWhenTrusted(t *testing.T) {
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
	if !strings.HasPrefix(it.Done[len(it.Done)-1], "ran hook") {
		t.Fatalf("hooks run last: %v", it.Done)
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
	ApplyCleanup(context.Background(), items, opts)
	if len(ran) != 1 || ran[0] != "make stop@app" {
		t.Fatalf("only the trusted repo's hook runs: %v", ran)
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
	w, issues := ParseWays([]byte("cleanup:\n  remote_branch: delete\n  archive: false\n  min_age_days: 14\n  hooks:\n    - make stop\n    - rm notes\n"))
	if len(issues) != 0 {
		t.Fatalf("issues: %+v", issues)
	}
	want := CleanupConfig{RemoteBranch: "delete", Archive: false, MinAgeDays: 14, Hooks: []string{"make stop", "rm notes"}}
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

func TestApplyCleanup_DoesNotArchiveUnlessConfigured(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	f.squashIntoMain()
	called := false
	opts := CleanupOptions{ArchiveChange: func(context.Context, ChangeTopology) (string, error) { called = true; return "", nil }}
	row := f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: f.head()})
	row.Issue = &TopologyIssue{State: "closed"}
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, opts)
	ApplyCleanup(context.Background(), items, opts)
	if called {
		t.Fatal("archive defaults to off")
	}
}
