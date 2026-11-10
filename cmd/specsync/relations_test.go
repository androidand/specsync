package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	specsync "github.com/androidand/specsync"
)

func relGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func relFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func relClone(t *testing.T, dir, origin string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	relGit(t, dir, "init", "-q", "-b", "main")
	relGit(t, dir, "remote", "add", "origin", origin)
}

func relChange(t *testing.T, clone, slug, repo, id, links string) {
	t.Helper()
	dir := filepath.Join(clone, "openspec", "changes", slug)
	relFile(t, filepath.Join(dir, "proposal.md"), "# "+slug+"\n")
	relFile(t, filepath.Join(dir, ".specsync", "refs.json"),
		`{"github:`+repo+`":{"provider":"github:`+repo+`","id":"`+id+`","url":"https://github.com/`+repo+`/issues/`+id+`"}}`)
	if links != "" {
		relFile(t, filepath.Join(dir, "links.md"), links)
	}
}

type relWorkspace struct {
	root, worktree string
	env            []string
}

func newRelWorkspace(t *testing.T) relWorkspace {
	t.Helper()
	root := t.TempDir()
	api, web := filepath.Join(root, "api"), filepath.Join(root, "web")
	relClone(t, api, "https://github.com/acme/api.git")
	relClone(t, web, "https://github.com/acme/web.git")
	relChange(t, api, "scope-list", "acme/api", "12", "## Blocked by\n- acme/web#2\n")
	relChange(t, web, "company-scope", "acme/web", "2", "")
	for _, c := range []string{api, web} {
		relGit(t, c, "add", "openspec")
		relGit(t, c, "commit", "-q", "-m", "init")
	}
	wt := filepath.Join(t.TempDir(), "scope-list")
	relGit(t, api, "worktree", "add", "-q", "-b", "feat/12-scope-list", wt)
	relGit(t, web, "worktree", "add", "-q", "-b", "experiment", filepath.Join(t.TempDir(), "exp"))
	home := t.TempDir()
	return relWorkspace{root: root, worktree: wt, env: append(os.Environ(), "HOME="+home, "XDG_CACHE_HOME="+home, "SPECSYNC_ROOTS=")}
}

func snapshotTree(t *testing.T, roots ...string) string {
	t.Helper()
	var lines []string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == "objects" && filepath.Base(filepath.Dir(path)) == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			b, _ := os.ReadFile(path)
			sum := sha256.Sum256(b)
			lines = append(lines, path+" "+hex.EncodeToString(sum[:]))
			return nil
		})
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func TestOverviewListsAllWorkAcrossClones(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	w := newRelWorkspace(t)
	out, err := runIn(t, bin, w.root, w.env, "overview", "-root", w.root, "-state=off")
	if err != nil {
		t.Fatalf("overview: %v\n%s", err, out)
	}
	for _, want := range []string{"scope: everything", "acme/api#12  scope-list", "acme/web#2  company-scope", "under no epic", "unlinked", "acme/web:experiment"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	scoped, err := runIn(t, bin, w.root, w.env, "overview", "-root", w.root, "-state=off", "-repo", "api")
	if err != nil {
		t.Fatalf("overview -repo: %v\n%s", err, scoped)
	}
	if !strings.Contains(scoped, "scope: repo api") || strings.Contains(scoped, "company-scope") {
		t.Errorf("scope not applied:\n%s", scoped)
	}
	if out, err := runIn(t, bin, w.root, w.env, "overview", "-root", w.root, "-state=off", "-json"); err != nil || !strings.Contains(out, `"epics": []`) || !strings.Contains(out, `"noEpic"`) {
		t.Errorf("json: %v\n%s", err, out)
	}
}

func TestStatusShowsBlockersAcrossRepos(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	w := newRelWorkspace(t)
	out, err := runIn(t, bin, w.worktree, w.env, "status", "-state=off")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	if !strings.Contains(out, "blocked by:") || !strings.Contains(out, "acme/web#2  company-scope") {
		t.Errorf("related work missing:\n%s", out)
	}
	alone, err := runIn(t, bin, filepath.Join(w.root, "web"), w.env, "status", "-state=off")
	if err != nil {
		t.Fatalf("status in a clone with no relations: %v\n%s", err, alone)
	}
	if strings.Contains(alone, "blocked by:") || strings.Contains(alone, "epic ") {
		t.Errorf("work with no relations is shown alone:\n%s", alone)
	}
}

func TestOverviewAndStatusAreReadOnly(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	w := newRelWorkspace(t)
	roots := []string{w.root, filepath.Dir(w.worktree)}
	if _, err := runIn(t, bin, w.worktree, w.env, "status", "-state=off"); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, roots...)
	var first []string
	for round := 0; round < 2; round++ {
		a, err := runIn(t, bin, w.root, w.env, "overview", "-root", w.root, "-state=off")
		if err != nil {
			t.Fatal(err)
		}
		b, err := runIn(t, bin, w.worktree, w.env, "status", "-state=off")
		if err != nil {
			t.Fatal(err)
		}
		if round == 0 {
			first = []string{a, b}
		} else if a != first[0] || b != first[1] {
			t.Fatalf("running twice with unchanged inputs must print the same output:\n--first overview--\n%s\n--second--\n%s\n--first status--\n%s\n--second--\n%s", first[0], a, first[1], b)
		}
	}
	if after := snapshotTree(t, roots...); after != before {
		t.Fatalf("overview and status changed files in a clone or worktree:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestStatusOutsideAWorktreeListsTheRepoChanges(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	w := newRelWorkspace(t)
	dir := t.TempDir()
	relFile(t, filepath.Join(dir, "openspec", "changes", "plain-change", "proposal.md"), "# plain-change\n")
	out, err := runIn(t, bin, dir, w.env, "status", "-state=off")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	if !strings.Contains(out, "plain-change") || !strings.Contains(out, "not inside a git worktree") {
		t.Errorf("outside a worktree status lists the repo's changes and says why:\n%s", out)
	}
}

func TestPrintRelatedDoesNotRepeatTheSyncSuggestion(t *testing.T) {
	rel := &specsync.RelatedWork{Current: "a/b:x", Blocks: []specsync.RelatedItem{{Key: "a/b#2"}}}
	out := captureStdout(t, func() { printRelated(rel) })
	if strings.Contains(out, "next:") {
		t.Fatalf("printCurrentStatus already prints suggest:, so related must not repeat it:\n%s", out)
	}
}

func TestStatusRelatedMatchesTheCurrentRowFromARelativeOpenspec(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	w := newRelWorkspace(t)
	relFile(t, filepath.Join(w.worktree, "openspec", "changes", "scope-list", "links.md"), "## Related\n- acme/web#2\n")
	out, err := runIn(t, bin, w.worktree, w.env, "status", "-state=off", "-json")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"linked": [`) || !strings.Contains(out, `"key": "acme/web#2"`) {
		t.Fatalf("a link to a change in another clone must show under linked:\n%s", out)
	}
	if n := strings.Count(out, `"slug": "scope-list"`); n != 1 {
		t.Fatalf("the current row must appear once (%d):\n%s", n, out)
	}
}
