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

type cleanupFixture struct {
	t    *testing.T
	main string
	wt   string
}

func gitOutT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeCommit(t *testing.T, dir, name, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutT(t, dir, "add", name)
	gitOutT(t, dir, "commit", "-q", "-m", msg)
}

func newCleanupFixture(t *testing.T) *cleanupFixture {
	t.Helper()
	root := t.TempDir()
	f := &cleanupFixture{t: t, main: filepath.Join(root, "app"), wt: filepath.Join(root, "wt")}
	gitRepo(t, f.main, "https://github.com/acme/app.git")
	gitOutT(t, f.main, "worktree", "add", "-q", "-b", "feat/42-thing", f.wt)
	writeCommit(t, f.wt, "a.txt", "work")
	return f
}

func (f *cleanupFixture) head() string { return gitOutT(f.t, f.wt, "rev-parse", "HEAD") }

func (f *cleanupFixture) squashIntoMain() {
	gitOutT(f.t, f.main, "merge", "-q", "--squash", "feat/42-thing")
	gitOutT(f.t, f.main, "commit", "-q", "-m", "squash of thing")
}

func (f *cleanupFixture) row(pr *ChangeRequest) ChangeTopology {
	ls, err := CollectLocalState(context.Background(), f.wt, LocalStateOptions{Base: "main"})
	if err != nil {
		f.t.Fatal(err)
	}
	return ChangeTopology{Slug: "thing", Store: f.main, Repo: "acme/app", Branch: "feat/42-thing", Worktree: f.wt, Local: ls, PR: pr}
}

func plan(t *testing.T, rows []ChangeTopology, mod func(*CleanupOptions)) []CleanupItem {
	t.Helper()
	opts := CleanupOptions{Now: func() time.Time { return time.Now() }}
	if mod != nil {
		mod(&opts)
	}
	return PlanCleanup(context.Background(), rows, opts)
}

func only(t *testing.T, items []CleanupItem) CleanupItem {
	t.Helper()
	if len(items) != 1 {
		t.Fatalf("want one item, got %+v", items)
	}
	return items[0]
}

func TestCleanupCandidate_MergedWithSameHeadAndClean(t *testing.T) {
	f := newCleanupFixture(t)
	f.squashIntoMain()
	it := only(t, plan(t, []ChangeTopology{f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: f.head()})}, nil))
	if it.Status != CleanupCandidate || !strings.Contains(it.Reason, "#7") {
		t.Fatalf("got %+v", it)
	}
}

func TestCleanupCandidate_AncestorOfBaseWithoutARequest(t *testing.T) {
	f := newCleanupFixture(t)
	gitOutT(t, f.main, "merge", "-q", "--ff-only", "feat/42-thing")
	it := only(t, plan(t, []ChangeTopology{f.row(nil)}, nil))
	if it.Status != CleanupCandidate || !strings.Contains(it.Reason, "main") {
		t.Fatalf("got %+v", it)
	}
}

func TestCleanupNotCandidate_BranchWithNoCommitsOfItsOwn(t *testing.T) {
	root := t.TempDir()
	f := &cleanupFixture{t: t, main: filepath.Join(root, "app"), wt: filepath.Join(root, "wt")}
	gitRepo(t, f.main, "https://github.com/acme/app.git")
	gitOutT(t, f.main, "worktree", "add", "-q", "-b", "feat/42-thing", f.wt)
	if items := plan(t, []ChangeTopology{f.row(nil)}, nil); len(items) != 0 {
		t.Fatalf("a fresh worktree is not finished work: %+v", items)
	}
}

func TestCleanupNotCandidate_ClosedWithoutMerging(t *testing.T) {
	f := newCleanupFixture(t)
	gitOutT(t, f.main, "merge", "-q", "--ff-only", "feat/42-thing")
	it := only(t, plan(t, []ChangeTopology{f.row(&ChangeRequest{Number: 7, State: "closed", HeadSHA: f.head()})}, nil))
	if it.Status == CleanupCandidate {
		t.Fatalf("a request closed unmerged is never a candidate: %+v", it)
	}
}

func TestCleanupNotCandidate_ClosedUnmergedWithUnpushedCommitsNeedsAttention(t *testing.T) {
	f := newCleanupFixture(t)
	it := only(t, plan(t, []ChangeTopology{f.row(&ChangeRequest{Number: 7, State: "closed", HeadSHA: f.head()})}, nil))
	if it.Status != CleanupAttention || !strings.Contains(it.Reason, "unpushed") {
		t.Fatalf("got %+v", it)
	}
}

func TestCleanupNotCandidate_CommitAfterTheMergeIsNamed(t *testing.T) {
	f := newCleanupFixture(t)
	merged := f.head()
	f.squashIntoMain()
	writeCommit(t, f.wt, "b.txt", "late fix")
	it := only(t, plan(t, []ChangeTopology{f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: merged})}, nil))
	if it.Status == CleanupCandidate || !strings.Contains(it.Reason, "late fix") {
		t.Fatalf("the extra commit must be named: %+v", it)
	}
}

func TestCleanupNotCandidate_UpstreamGoneAndHeadNotMergedNorInBase(t *testing.T) {
	f := newCleanupFixture(t)
	row := f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: "0000000000000000000000000000000000000000"})
	row.Local.Upstream, row.Local.UpstreamGone = "origin/feat/42-thing", true
	it := only(t, plan(t, []ChangeTopology{row}, nil))
	if it.Status != CleanupAttention || !strings.Contains(it.Reason, "unpushed") {
		t.Fatalf("with the upstream gone every commit outside the base is unpushed: %+v", it)
	}
}

func TestCleanupUpstreamGone_MergedHeadIsStillACandidate(t *testing.T) {
	f := newCleanupFixture(t)
	f.squashIntoMain()
	row := f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: f.head()})
	row.Local.Upstream, row.Local.UpstreamGone = "origin/feat/42-thing", true
	if it := only(t, plan(t, []ChangeTopology{row}, nil)); it.Status != CleanupCandidate {
		t.Fatalf("got %+v", it)
	}
}

func TestCleanupSkips_ShownWithTheReason(t *testing.T) {
	cases := []struct {
		name string
		mod  func(f *cleanupFixture, row *ChangeTopology, o *CleanupOptions)
		want string
	}{
		{"dirty", func(f *cleanupFixture, row *ChangeTopology, _ *CleanupOptions) {
			if err := os.WriteFile(filepath.Join(f.wt, "dirty.txt"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			ls, _ := CollectLocalState(context.Background(), f.wt, LocalStateOptions{Base: "main"})
			row.Local = ls
		}, "uncommitted"},
		{"current", func(f *cleanupFixture, _ *ChangeTopology, o *CleanupOptions) { o.Current = filepath.Join(f.wt, "sub") }, "current"},
		{"live session", func(_ *cleanupFixture, row *ChangeTopology, _ *CleanupOptions) {
			row.Sessions = []SessionEntry{{CWD: row.Worktree, Name: "build"}}
		}, "session"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newCleanupFixture(t)
			f.squashIntoMain()
			row := f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: f.head()})
			opts := CleanupOptions{}
			c.mod(f, &row, &opts)
			it := only(t, PlanCleanup(context.Background(), []ChangeTopology{row}, opts))
			if it.Status != CleanupSkipped || !strings.Contains(it.Reason, c.want) {
				t.Fatalf("got %+v", it)
			}
		})
	}
}

func TestCleanupNeverListsTheMainCheckout(t *testing.T) {
	f := newCleanupFixture(t)
	f.squashIntoMain()
	row := f.row(&ChangeRequest{Number: 7, State: "merged", HeadSHA: f.head()})
	row.Worktree = f.main
	if items := plan(t, []ChangeTopology{row}, nil); len(items) != 0 {
		t.Fatalf("the main checkout is never a cleanup target: %+v", items)
	}
}

func TestCleanupOpenRequestIsNeverACandidate(t *testing.T) {
	f := newCleanupFixture(t)
	gitOutT(t, f.main, "merge", "-q", "--ff-only", "feat/42-thing")
	for _, it := range plan(t, []ChangeTopology{f.row(&ChangeRequest{Number: 7, State: "open"})}, nil) {
		if it.Status == CleanupCandidate {
			t.Fatalf("got %+v", it)
		}
	}
}

func TestCleanupAttention_OldUnmergedWorkHonoursTheMinimumAge(t *testing.T) {
	f := newCleanupFixture(t)
	row := f.row(nil)
	row.Local.LastCommit = time.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	age := func(days int) func(*CleanupOptions) {
		return func(o *CleanupOptions) { o.MinAgeDays = func(string) int { return days } }
	}
	if items := plan(t, []ChangeTopology{row}, age(30)); len(items) != 0 {
		t.Fatalf("younger than the minimum age: %+v", items)
	}
	it := only(t, plan(t, []ChangeTopology{row}, age(7)))
	if it.Status != CleanupAttention {
		t.Fatalf("got %+v", it)
	}
}
