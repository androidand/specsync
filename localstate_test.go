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

// gitFixture runs git in dir with a fixed identity and no user config.
func gitFixture(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, dir, "add", name)
	gitFixture(t, dir, "commit", "-q", "-m", "add "+name)
}

// repoWithOrigin returns a clone whose origin/main exists and is checked out
// on a feature branch with upstream origin/feature.
func repoWithOrigin(t *testing.T) (clone, origin string) {
	t.Helper()
	root := t.TempDir()
	origin = filepath.Join(root, "origin.git")
	gitFixture(t, root, "init", "-q", "--bare", "-b", "main", origin)
	clone = filepath.Join(root, "clone")
	gitFixture(t, root, "clone", "-q", origin, clone)
	gitFixture(t, clone, "checkout", "-q", "-b", "main")
	commitFile(t, clone, "a.txt", "a\n")
	gitFixture(t, clone, "push", "-q", "-u", "origin", "main")
	gitFixture(t, clone, "remote", "set-head", "origin", "main")
	gitFixture(t, clone, "checkout", "-q", "-b", "feature")
	gitFixture(t, clone, "push", "-q", "-u", "origin", "feature")
	return clone, origin
}

func collect(t *testing.T, dir string) LocalState {
	t.Helper()
	ls, err := CollectLocalState(context.Background(), dir, LocalStateOptions{VolatileDirs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	return *ls
}

func TestLocalState_Clean(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	ls := collect(t, clone)
	if ls.Modified != 0 || ls.Untracked != 0 || ls.Ahead != 0 || ls.Behind != 0 {
		t.Fatalf("clean worktree reported changes: %+v", ls)
	}
	if ls.Upstream != "origin/feature" || ls.Base != "origin/main" {
		t.Fatalf("upstream/base = %q/%q", ls.Upstream, ls.Base)
	}
	if ls.LastCommit == "" {
		t.Fatal("last commit time missing")
	}
	if got := LocalAttention(ls); len(got) != 0 {
		t.Fatalf("clean worktree has attention %v", got)
	}
}

func TestLocalState_DirtyAndUntracked(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	if err := os.WriteFile(filepath.Join(clone, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "new.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ls := collect(t, clone)
	if ls.Modified != 1 || ls.Untracked != 1 {
		t.Fatalf("modified/untracked = %d/%d, want 1/1", ls.Modified, ls.Untracked)
	}
	if !reflect.DeepEqual(LocalAttention(ls), []string{"uncommitted"}) {
		t.Fatalf("attention = %v", LocalAttention(ls))
	}
}

func TestLocalState_AheadOfUpstream(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFile(t, clone, "b.txt", "b\n")
	commitFile(t, clone, "c.txt", "c\n")
	ls := collect(t, clone)
	if ls.Ahead != 2 || ls.Behind != 0 {
		t.Fatalf("ahead/behind = %d/%d, want 2/0", ls.Ahead, ls.Behind)
	}
	if !reflect.DeepEqual(LocalAttention(ls), []string{"unpushed"}) {
		t.Fatalf("attention = %v", LocalAttention(ls))
	}
	if ls.Size == nil || ls.Size.Commits != 2 || ls.Size.Files != 2 || ls.Size.Additions != 2 || ls.Size.Deletions != 0 {
		t.Fatalf("size = %+v, want 2 commits, 2 files, +2 -0", ls.Size)
	}
}

func TestLocalState_BehindUpstreamAndDiverged(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	other := filepath.Join(t.TempDir(), "other")
	gitFixture(t, filepath.Dir(other), "clone", "-q", "-b", "feature", origin, other)
	commitFile(t, other, "t.txt", "teammate\n")
	gitFixture(t, other, "push", "-q", "origin", "feature")
	gitFixture(t, clone, "fetch", "-q")

	ls := collect(t, clone)
	if ls.Behind != 1 || ls.Ahead != 0 {
		t.Fatalf("ahead/behind = %d/%d, want 0/1", ls.Ahead, ls.Behind)
	}
	if !reflect.DeepEqual(LocalAttention(ls), []string{"behind-upstream"}) {
		t.Fatalf("attention = %v", LocalAttention(ls))
	}

	commitFile(t, clone, "mine.txt", "mine\n")
	ls = collect(t, clone)
	if ls.Ahead != 1 || ls.Behind != 1 {
		t.Fatalf("ahead/behind = %d/%d, want 1/1", ls.Ahead, ls.Behind)
	}
	if !reflect.DeepEqual(LocalAttention(ls), []string{"unpushed", "diverged"}) {
		t.Fatalf("attention = %v", LocalAttention(ls))
	}
}

func TestLocalState_NoUpstream(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	gitFixture(t, clone, "checkout", "-q", "-b", "local-only")
	commitFile(t, clone, "x.txt", "x\n")
	ls := collect(t, clone)
	if ls.Upstream != "" || ls.Ahead != 0 {
		t.Fatalf("upstream/ahead = %q/%d, want empty/0", ls.Upstream, ls.Ahead)
	}
	if !reflect.DeepEqual(LocalAttention(ls), []string{"no-upstream"}) {
		t.Fatalf("attention = %v", LocalAttention(ls))
	}
	if ls.Size == nil || ls.Size.Commits != 1 {
		t.Fatalf("size against base should still be reported: %+v", ls.Size)
	}
}

func TestLocalState_BehindBase(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	gitFixture(t, clone, "checkout", "-q", "main")
	commitFile(t, clone, "m1.txt", "1\n")
	commitFile(t, clone, "m2.txt", "2\n")
	gitFixture(t, clone, "push", "-q", "origin", "main")
	gitFixture(t, clone, "checkout", "-q", "feature")
	ls := collect(t, clone)
	if ls.BehindBase != 2 {
		t.Fatalf("behindBase = %d, want 2", ls.BehindBase)
	}
	if !reflect.DeepEqual(LocalAttention(ls), []string{"behind-base"}) {
		t.Fatalf("attention = %v", LocalAttention(ls))
	}
}

func TestLocalState_NoBase(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	gitFixture(t, clone, "remote", "set-head", "origin", "-d")
	ls := collect(t, clone)
	if ls.Base != "" || ls.Size != nil {
		t.Fatalf("without a base, base and size are omitted: %+v", ls)
	}
}

func TestLocalState_NotARepo(t *testing.T) {
	if _, err := CollectLocalState(context.Background(), t.TempDir(), LocalStateOptions{}); err == nil {
		t.Fatal("expected an error outside a git repo")
	}
}

func TestIsVolatilePath(t *testing.T) {
	dirs := []string{"/tmp", "/var/tmp"}
	for path, want := range map[string]bool{
		"/tmp/work/x":         true,
		"/var/tmp/x":          true,
		"/tmpfoo/x":           false,
		"/home/u/worktrees/x": false,
		"/home/u/tmp/x":       false,
	} {
		if got := isVolatilePath(path, dirs); got != want {
			t.Errorf("isVolatilePath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestLocalAttention_Volatile(t *testing.T) {
	got := LocalAttention(LocalState{Upstream: "origin/x", VolatilePath: true})
	if !reflect.DeepEqual(got, []string{"volatile-path"}) {
		t.Fatalf("attention = %v", got)
	}
}

// TestCollectLocalState_ReadOnly proves collecting state does not change the
// worktree, index or refs.
func TestCollectLocalState_ReadOnly(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	if err := os.WriteFile(filepath.Join(clone, "new.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := gitFixture(t, clone, "status", "--porcelain=v2", "--branch") + gitFixture(t, clone, "for-each-ref")
	collect(t, clone)
	after := gitFixture(t, clone, "status", "--porcelain=v2", "--branch") + gitFixture(t, clone, "for-each-ref")
	if before != after {
		t.Fatalf("collecting state changed the repo:\n%s\n---\n%s", before, after)
	}
}
