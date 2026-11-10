package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sliceCLIRepo(t *testing.T) (repo string, env []string) {
	t.Helper()
	root := t.TempDir()
	repo = filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(repo, "openspec", "changes", "big-thing"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(repo, "openspec", "changes", "big-thing", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("proposal.md", "# Big thing\n")
	write("tasks.md", "# Tasks\n\n## Slice 1: Parser\ncontract: parse\nbudget: files<=5\n- [x] a\n- [ ] b\n\n## Slice 2: Commands\nafter: 1\ncontract: list\n- [ ] c\n")
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo, append(os.Environ(), "HOME="+t.TempDir(), "SPECSYNC_DEFAULT_STORE=")
}

func TestSlicesListsDeclaredSlicesAndChangesNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := sliceCLIRepo(t)
	before := gitStatus(t, repo)
	out, err := runIn(t, bin, repo, env, "slices", "-state", "off", "-change", "big-thing")
	if err != nil {
		t.Fatalf("slices: %v\n%s", err, out)
	}
	for _, want := range []string{"2 slices", "not started", "1/2 tasks", "budget files<=5", "contract: parse", "after 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if gitStatus(t, repo) != before {
		t.Fatal("slices must not change the worktree")
	}
	js, err := runIn(t, bin, repo, env, "slices", "-state", "off", "-json")
	if err != nil {
		t.Fatalf("slices -json: %v\n%s", err, js)
	}
	var parsed struct {
		Single bool
		Slices []struct{ N int }
	}
	if err := json.Unmarshal([]byte(js), &parsed); err != nil || parsed.Single || len(parsed.Slices) != 2 {
		t.Fatalf("json = %s (%v)", js, err)
	}
}

func TestSlicesSingleSliceWhenNoneDeclared(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := sliceCLIRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "openspec", "changes", "big-thing", "tasks.md"), []byte("# Tasks\n- [ ] a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runIn(t, bin, repo, env, "slices", "-state", "off", "-change", "big-thing")
	if err != nil || !strings.Contains(out, "single slice") || strings.Contains(out, "not started") {
		t.Fatalf("got err=%v out=%s", err, out)
	}
}

func TestSliceStartDryRunThenStackedStart(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := sliceCLIRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "wts")
	out, err := runIn(t, bin, repo, env, "slice", "start", "-change", "big-thing", "-base", "main", "-worktree-dir", wt, "-dry-run", "1")
	if err != nil || !strings.Contains(out, "DRY RUN") || !strings.Contains(out, "feat/big-thing-slice-1") {
		t.Fatalf("dry run: err=%v out=%s", err, out)
	}
	if _, statErr := os.Stat(wt); statErr == nil {
		t.Fatal("dry run must not create anything")
	}
	if out, err := runIn(t, bin, repo, env, "slice", "start", "-change", "big-thing", "-base", "main", "-worktree-dir", wt, "2"); err == nil || !strings.Contains(out, "slice 1") {
		t.Fatalf("slice 2 before slice 1 must be refused naming slice 1: err=%v out=%s", err, out)
	}
	for _, n := range []string{"1", "2"} {
		if out, err := runIn(t, bin, repo, env, "slice", "start", "-change", "big-thing", "-base", "main", "-worktree-dir", wt, n); err != nil {
			t.Fatalf("start %s: %v\n%s", n, err, out)
		}
	}
	out, err = runIn(t, bin, repo, env, "slices", "-state", "off", "-change", "big-thing")
	if err != nil || strings.Count(out, "branch exists") != 2 || !strings.Contains(out, "feat/big-thing-slice-2") {
		t.Fatalf("slices after start: err=%v out=%s", err, out)
	}
}
