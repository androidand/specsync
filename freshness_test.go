package specsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const nineLines = "1\n2\n3\n4\n5\n6\n7\n8\n9\n"

// editLine rewrites line n (1-based) of file in dir and commits it.
func editLine(t *testing.T, dir, file string, n int, text string) {
	t.Helper()
	p := filepath.Join(dir, file)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	lines[n-1] = text
	commitFile(t, dir, file, strings.Join(lines, "\n"))
}

// movedBase returns a clone whose feature branch and origin/main start from a
// nine-line a.txt. Callers then edit main and feature as the scenario needs.
func movedBase(t *testing.T) string {
	t.Helper()
	clone, _ := repoWithOrigin(t)
	gitFixture(t, clone, "checkout", "-q", "main")
	commitFile(t, clone, "a.txt", nineLines)
	gitFixture(t, clone, "push", "-q", "origin", "main")
	gitFixture(t, clone, "checkout", "-q", "feature")
	gitFixture(t, clone, "reset", "-q", "--hard", "main")
	gitFixture(t, clone, "push", "-q", "-f", "origin", "feature")
	return clone
}

// advanceMain commits on main, pushes it, and returns to feature.
func advanceMain(t *testing.T, clone string, edit func()) {
	t.Helper()
	gitFixture(t, clone, "checkout", "-q", "main")
	edit()
	gitFixture(t, clone, "push", "-q", "origin", "main")
	gitFixture(t, clone, "checkout", "-q", "feature")
}

func pushFeature(t *testing.T, clone string) {
	t.Helper()
	gitFixture(t, clone, "push", "-q", "origin", "feature")
}

func TestFreshness_BaseMovedWithoutOverlap(t *testing.T) {
	clone := movedBase(t)
	advanceMain(t, clone, func() { commitFile(t, clone, "m.txt", "m\n") })
	commitFile(t, clone, "b.txt", "b\n")
	pushFeature(t, clone)

	ls := collect(t, clone)
	if ls.BehindBase != 1 || ls.BaseMovedAt == "" {
		t.Fatalf("behindBase/baseMovedAt = %d/%q", ls.BehindBase, ls.BaseMovedAt)
	}
	if len(ls.BaseTouchesYourFiles) != 0 || len(ls.Conflicts) != 0 || ls.ConflictCheckUnavailable {
		t.Fatalf("no overlap expected: %+v", ls)
	}
	if !reflect.DeepEqual(LocalAttention(ls), []string{"behind-base"}) {
		t.Fatalf("attention = %v", LocalAttention(ls))
	}
}

func TestFreshness_OverlapWithoutConflict(t *testing.T) {
	clone := movedBase(t)
	advanceMain(t, clone, func() { editLine(t, clone, "a.txt", 1, "main edit") })
	editLine(t, clone, "a.txt", 9, "feature edit")
	pushFeature(t, clone)

	ls := collect(t, clone)
	if !reflect.DeepEqual(ls.BaseTouchesYourFiles, []string{"a.txt"}) {
		t.Fatalf("baseTouchesYourFiles = %v", ls.BaseTouchesYourFiles)
	}
	if len(ls.Conflicts) != 0 {
		t.Fatalf("different lines must not conflict: %v", ls.Conflicts)
	}
	if !reflect.DeepEqual(LocalAttention(ls), []string{"behind-base", "base-touches-your-files"}) {
		t.Fatalf("attention = %v", LocalAttention(ls))
	}
}

func TestFreshness_PredictsConflictWithoutTouchingTheWorktree(t *testing.T) {
	clone := movedBase(t)
	advanceMain(t, clone, func() { editLine(t, clone, "a.txt", 5, "main edit") })
	editLine(t, clone, "a.txt", 5, "feature edit")
	pushFeature(t, clone)

	before := gitFixture(t, clone, "status", "--porcelain=v2", "--branch") + gitFixture(t, clone, "for-each-ref")
	head := gitFixture(t, clone, "rev-parse", "HEAD")
	ls := collect(t, clone)
	if !reflect.DeepEqual(ls.Conflicts, []string{"a.txt"}) {
		t.Fatalf("conflicts = %v, want [a.txt]", ls.Conflicts)
	}
	if !reflect.DeepEqual(LocalAttention(ls), []string{"behind-base", "base-touches-your-files", "conflicts-with-base"}) {
		t.Fatalf("attention = %v", LocalAttention(ls))
	}
	after := gitFixture(t, clone, "status", "--porcelain=v2", "--branch") + gitFixture(t, clone, "for-each-ref")
	if before != after || head != gitFixture(t, clone, "rev-parse", "HEAD") {
		t.Fatal("conflict prediction changed the repo")
	}
	if _, err := os.Stat(filepath.Join(clone, ".git", "MERGE_HEAD")); err == nil {
		t.Fatal("a merge was started")
	}
}

func TestFreshness_SharedBranchNeedsMoreThanOneAuthor(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFile(t, clone, "mine.txt", "m\n")
	if ls := collect(t, clone); ls.SharedBranch {
		t.Fatal("one author is not shared")
	}
	cmd := exec.Command("git", "-C", clone, "commit", "-q", "--allow-empty", "-m", "teammate")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=Other", "GIT_AUTHOR_EMAIL=other@example.com",
		"GIT_COMMITTER_NAME=Other", "GIT_COMMITTER_EMAIL=other@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if ls := collect(t, clone); !ls.SharedBranch {
		t.Fatal("two authors on the branch must be reported as shared")
	}
}

func TestFreshness_NoBaseMeansNoFreshnessFacts(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	gitFixture(t, clone, "remote", "set-head", "origin", "-d")
	ls := collect(t, clone)
	if ls.BaseMovedAt != "" || ls.BaseTouchesYourFiles != nil || ls.Conflicts != nil || ls.ConflictCheckUnavailable {
		t.Fatalf("no base, no facts: %+v", ls)
	}
}

func TestClassifyMergeTree(t *testing.T) {
	cases := []struct {
		name      string
		exit      int
		stdout    string
		wantPaths []string
		wantOK    bool
	}{
		{"clean", 0, "abc123\n", nil, true},
		{"conflict", 1, "abc123\nsrc/a.go\nb.txt\n\nAuto-merging src/a.go\nCONFLICT (content)\n", []string{"b.txt", "src/a.go"}, true},
		{"old git or bad option", 129, "", nil, false},
		{"other failure", 128, "", nil, false},
	}
	for _, c := range cases {
		paths, ok := classifyMergeTree(c.exit, c.stdout)
		if ok != c.wantOK || !reflect.DeepEqual(paths, c.wantPaths) {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", c.name, paths, ok, c.wantPaths, c.wantOK)
		}
	}
}
