package specsync

import (
	"context"
	"strings"
	"testing"
)

// fakeGit returns a runner that records calls and answers `describe`/`log` with
// canned output, so commit-source behavior is exercised without a real repo.
func fakeGit(tag, log string, calls *[][]string) func(context.Context, ...string) (string, error) {
	return func(_ context.Context, args ...string) (string, error) {
		*calls = append(*calls, args)
		switch {
		case len(args) >= 1 && args[0] == "describe":
			return tag, nil
		case len(args) >= 1 && args[0] == "log":
			return log, nil
		default:
			return "", nil
		}
	}
}

func gitLogFixture() string {
	rec := func(h, a, d, msg string) string {
		return strings.Join([]string{h, a, d, msg}, gitFieldSep) + gitRecordSep
	}
	return rec("h1", "Dev", "2026-06-01T10:00:00Z", "feat(ui): split modal (#51)") +
		"\n" + rec("h2", "Dev", "2026-06-02T10:00:00Z", "fix: slug off-by-one\n\nCloses #42")
}

func TestGitCommitsParsesLog(t *testing.T) {
	var calls [][]string
	src := NewGitCommitSourceFunc(fakeGit("v0.2.0", gitLogFixture(), &calls))

	commits, err := src.Commits(context.Background(), "", "", nil)
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	if commits[0].Type != "feat" || len(commits[0].PRRefs) != 1 || commits[0].PRRefs[0] != "#51" {
		t.Fatalf("commit 0 parsed wrong: %+v", commits[0])
	}
	if commits[1].Type != "fix" || len(commits[1].IssueRefs) != 1 || commits[1].IssueRefs[0] != "#42" {
		t.Fatalf("commit 1 parsed wrong: %+v", commits[1])
	}
}

func TestGitCommitsDefaultRangeUsesTag(t *testing.T) {
	var calls [][]string
	src := NewGitCommitSourceFunc(fakeGit("v0.2.0", gitLogFixture(), &calls))
	if _, err := src.Commits(context.Background(), "", "", nil); err != nil {
		t.Fatalf("Commits: %v", err)
	}
	// The log call must scope to v0.2.0..HEAD when since/until are empty.
	var sawRange bool
	for _, c := range calls {
		for _, a := range c {
			if a == "v0.2.0..HEAD" {
				sawRange = true
			}
		}
	}
	if !sawRange {
		t.Fatalf("expected a v0.2.0..HEAD range in git calls, got %v", calls)
	}
}

func TestGitCommitsNoTagFallsBackToFullHistory(t *testing.T) {
	var calls [][]string
	src := NewGitCommitSourceFunc(fakeGit("", gitLogFixture(), &calls))
	if _, err := src.Commits(context.Background(), "", "HEAD", nil); err != nil {
		t.Fatalf("Commits: %v", err)
	}
	// With no tag, the range is just HEAD (full history), never "..HEAD".
	for _, c := range calls {
		for _, a := range c {
			if strings.Contains(a, "..") {
				t.Fatalf("did not expect a range with no tag, got %q", a)
			}
		}
	}
}
