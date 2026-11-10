package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeleteBranchAtDeletesOnlyTheVerifiedHead(t *testing.T) {
	f := newCleanupFixture(t)
	head := f.head()
	gitOutT(t, f.main, "worktree", "remove", "--force", f.wt)
	writeCommit(t, f.main, "x.txt", "unrelated")
	moved := gitOutT(t, f.main, "rev-parse", "HEAD")
	gitOutT(t, f.main, "update-ref", "refs/heads/feat/42-thing", moved)
	if err := deleteBranchAt(context.Background(), f.main, "feat/42-thing", head); err == nil {
		t.Fatal("a branch whose tip moved must not be deleted")
	}
	if !f.branchExists(f.main, "feat/42-thing") {
		t.Fatal("the branch must still exist")
	}
	if err := deleteBranchAt(context.Background(), f.main, "feat/42-thing", moved); err != nil {
		t.Fatal(err)
	}
	if f.branchExists(f.main, "feat/42-thing") {
		t.Fatal("the branch at the verified head must be deleted")
	}
}

func TestCollectLocalStateReportsAFailedStatusInsteadOfZeroCounts(t *testing.T) {
	f := newCleanupFixture(t)
	idx := gitOutT(t, f.wt, "rev-parse", "--git-path", "index")
	if !filepath.IsAbs(idx) {
		idx = filepath.Join(f.wt, idx)
	}
	if err := os.WriteFile(idx, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	ls, err := CollectLocalState(context.Background(), f.wt, LocalStateOptions{Base: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !ls.StatusUnavailable {
		t.Fatalf("a failed status must be reported: %+v", ls)
	}
}

func TestCollectLocalStateCountsUntrackedFilesWhateverTheUserConfigSays(t *testing.T) {
	f := newCleanupFixture(t)
	gitOutT(t, f.main, "config", "status.showUntrackedFiles", "no")
	if err := os.WriteFile(filepath.Join(f.wt, "hidden.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ls, err := CollectLocalState(context.Background(), f.wt, LocalStateOptions{Base: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if ls.Untracked != 1 {
		t.Fatalf("an untracked file must not be hidden by config: %+v", ls)
	}
}

func TestCleanupFailsClosedWhenCleanlinessCannotBeRead(t *testing.T) {
	f := newCleanupFixture(t)
	row := finishedRow(t, f)
	row.Local.StatusUnavailable = true
	it := planOne(t, row, CleanupOptions{})
	if it.Status == CleanupCandidate || !strings.Contains(it.Reason, "git status") {
		t.Fatalf("got %+v", it)
	}
	if _, err := ignoredFiles(context.Background(), filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("a failing ignored-files check must return an error")
	}
}

func TestCleanupMergedIntoAnotherFeatureBranchIsNotInTrunk(t *testing.T) {
	f := newCleanupFixture(t)
	f.squashIntoMain()
	row := f.row(&ChangeRequest{Number: 7, State: "merged", Base: "feat/parent", HeadSHA: f.head()})
	for _, it := range PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{}) {
		if it.Status == CleanupCandidate {
			t.Fatalf("a stacked request merged into another branch proves nothing about trunk: %+v", it)
		}
	}
	row.PR.Base = ""
	for _, it := range PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{}) {
		if it.Status == CleanupCandidate {
			t.Fatalf("an unknown request base needs the ancestor proof: %+v", it)
		}
	}
}

func TestApplyCleanupRefusesWhenSessionsCouldNotBeVerified(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	row := finishedRow(t, f)
	items := PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{})
	it := only(t, ApplyCleanup(context.Background(), items, CleanupOptions{SessionsUnverified: true}))
	if it.Error == "" || !strings.Contains(it.Error, "live sessions not verified") {
		t.Fatalf("got %+v", it)
	}
	if _, err := os.Stat(f.wt); err != nil {
		t.Fatal("the worktree must stay")
	}
	failing := CleanupOptions{Overview: OverviewOptions{SessionsCmd: "exit 3"}}
	it = only(t, ApplyCleanup(context.Background(), items, failing))
	if it.Error == "" || !strings.Contains(it.Error, "live sessions") {
		t.Fatalf("a provider that fails at recheck refuses the removal: %+v", it)
	}
	ok := CleanupOptions{SessionsUnverified: true, AssumeNoSessions: true}
	if it := only(t, ApplyCleanup(context.Background(), items, ok)); it.Error != "" {
		t.Fatalf("-assume-no-sessions overrides: %+v", it)
	}
}

func TestMarkSessionsUnverifiedListsCandidatesAsSkipped(t *testing.T) {
	f := newCleanupFixture(t)
	row := finishedRow(t, f)
	items := markSessionsUnverified(PlanCleanup(context.Background(), []ChangeTopology{row}, CleanupOptions{}))
	if it := only(t, items); it.Status != CleanupSkipped || !strings.Contains(it.Reason, "live sessions not verified") {
		t.Fatalf("got %+v", it)
	}
}

func TestCleanupProtectsMoreBranchesAndHonoursTheConfiguredGlobs(t *testing.T) {
	for _, branch := range []string{"release/1.2", "hotfix/urgent", "staging", "production", "prod", "gh-pages", "keep/this"} {
		t.Run(branch, func(t *testing.T) {
			f := newCleanupFixture(t)
			f.writeConfig("cleanup:\n  protect:\n    - keep/*\n")
			row := finishedRow(t, f)
			row.Branch = branch
			it := planOne(t, row, CleanupOptions{})
			if it.Status != CleanupSkipped || !strings.Contains(it.Reason, "protected") {
				t.Fatalf("got %+v", it)
			}
		})
	}
}

func TestCleanupNeverRemovesASharedBranch(t *testing.T) {
	f := newCleanupFixture(t)
	row := finishedRow(t, f)
	row.Local.SharedBranch = true
	if it := planOne(t, row, CleanupOptions{}); it.Status != CleanupSkipped || !strings.Contains(it.Reason, "several authors") {
		t.Fatalf("got %+v", it)
	}
}

func TestCleanupCurrentWorktreeGuardSurvivesCaseInsensitivePaths(t *testing.T) {
	f := newCleanupFixture(t)
	upper := strings.ToUpper(f.wt)
	if _, err := os.Stat(upper); err != nil {
		t.Skip("this filesystem is case sensitive")
	}
	row := finishedRow(t, f)
	if it := planOne(t, row, CleanupOptions{Current: upper}); it.Status != CleanupSkipped || !strings.Contains(it.Reason, "current") {
		t.Fatalf("got %+v", it)
	}
}

func TestCleanupTwoRowsForOneWorktreeAreOneItem(t *testing.T) {
	trustFile(t)
	f := newCleanupFixture(t)
	row := finishedRow(t, f)
	other := row
	other.Slug, other.NoChange = "", true
	items := PlanCleanup(context.Background(), []ChangeTopology{other, row}, CleanupOptions{})
	it := only(t, items)
	if it.Slug != "thing" {
		t.Fatalf("the row with a change is kept: %+v", it)
	}
	dup := append(items, items[0])
	rep := ApplyCleanup(context.Background(), dup, CleanupOptions{})
	for _, r := range rep {
		if r.Error != "" {
			t.Fatalf("a duplicate item must not be an error: %+v", r)
		}
	}
}

func TestCleanupSanitisesCommitSubjects(t *testing.T) {
	f := newCleanupFixture(t)
	merged := f.head()
	f.squashIntoMain()
	writeCommit(t, f.wt, "b.txt", "late \x1b[31mred\x1b[0m fix")
	it := only(t, plan(t, []ChangeTopology{f.row(&ChangeRequest{Number: 7, State: "merged", Base: "main", HeadSHA: merged})}, nil))
	if strings.ContainsRune(it.Reason, 0x1b) {
		t.Fatalf("escape sequences must not reach the terminal: %q", it.Reason)
	}
}

func TestShellHookOutputIsSanitisedAndTruncated(t *testing.T) {
	err := runShellHook(context.Background(), t.TempDir(), `printf '\033[31m%s' "$(head -c 5000 /dev/zero | tr '\0' x)"; exit 1`, nil)
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.ContainsRune(err.Error(), 0x1b) || len(err.Error()) > 400 {
		t.Fatalf("hook output must be sanitised and truncated: %d bytes", len(err.Error()))
	}
}
