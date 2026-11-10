package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildSpecsync compiles the CLI once per test into a temp dir.
func buildSpecsync(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "specsync")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// changeRepo makes a git repo on main with changes fix-login-race and other.
func changeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, slug := range []string{"fix-login-race", "other"} {
		dir := filepath.Join(repo, "openspec", "changes", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# "+slug+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	branchGit(t, repo, "init", "-q", "-b", "main")
	return repo
}

func branchGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func branchRun(dir, bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func stageFile(repo, slug string) string {
	return filepath.Join(repo, "openspec", "changes", slug, ".specsync", "metadata.json")
}

func TestSetStageInfersChangeFromBranch(t *testing.T) {
	bin := buildSpecsync(t)

	for _, branch := range []string{"feat/42-fix-login-race", "someuser/fix-login-race"} {
		t.Run(branch, func(t *testing.T) {
			repo := changeRepo(t)
			branchGit(t, repo, "checkout", "-q", "-b", branch)

			out, err := branchRun(repo, bin, "set-stage", "active")
			if err != nil {
				t.Fatalf("set-stage active: %v\n%s", err, out)
			}
			if !strings.Contains(out, "using change fix-login-race (from branch "+branch+")") {
				t.Errorf("pick not reported on stderr:\n%s", out)
			}
			meta, err := os.ReadFile(stageFile(repo, "fix-login-race"))
			if err != nil || !strings.Contains(string(meta), `"active"`) {
				t.Fatalf("fix-login-race not active: %v %s", err, meta)
			}
			if _, err := os.Stat(stageFile(repo, "other")); err == nil {
				t.Error("other change was touched")
			}
		})
	}
}

func TestSetStageOnDefaultBranchFails(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)

	out, err := branchRun(repo, bin, "set-stage", "active")
	if err == nil {
		t.Fatalf("set-stage on main succeeded:\n%s", out)
	}
	if !strings.Contains(out, `branch "main" names no active change`) {
		t.Errorf("unexpected error:\n%s", out)
	}
	for _, slug := range []string{"fix-login-race", "other"} {
		if _, err := os.Stat(stageFile(repo, slug)); err == nil {
			t.Errorf("%s metadata written on main", slug)
		}
	}
}

func TestExplicitChangeBeatsBranch(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	branchGit(t, repo, "checkout", "-q", "-b", "feat/42-fix-login-race")

	if out, err := branchRun(repo, bin, "set-stage", "other", "blocked"); err != nil {
		t.Fatalf("set-stage other blocked: %v\n%s", err, out)
	}
	if _, err := os.Stat(stageFile(repo, "fix-login-race")); err == nil {
		t.Error("branch change was touched despite an explicit change")
	}
	if meta, _ := os.ReadFile(stageFile(repo, "other")); !strings.Contains(string(meta), `"blocked"`) {
		t.Errorf("other not blocked: %s", meta)
	}
}

func TestNoteInfersChangeFromBranch(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	branchGit(t, repo, "checkout", "-q", "-b", "feat/42-fix-login-race")

	if out, err := branchRun(repo, bin, "note", "found an edge case"); err != nil {
		t.Fatalf("note: %v\n%s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(repo, "openspec", "changes", "fix-login-race", "discoveries.md"))
	if err != nil || !strings.Contains(string(got), "- found an edge case") {
		t.Fatalf("note not appended: %v %s", err, got)
	}
}

func TestArchiveWithoutChangeOnDefaultBranchFails(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)

	out, err := branchRun(repo, bin, "archive", "-dry-run")
	if err == nil {
		t.Fatalf("archive on main succeeded:\n%s", out)
	}
	if !strings.Contains(out, "pass -change <slug>") {
		t.Errorf("unexpected error:\n%s", out)
	}
}
