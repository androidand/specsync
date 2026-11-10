package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOrcaLinkedIssue(t *testing.T) {
	tests := []struct {
		name, json, want string
	}{
		// Shape from Orca's source: printResult prints the whole RPC envelope,
		// and Worktree.linkedIssue is number | null.
		{"linked", `{"id":"r1","ok":true,"result":{"worktree":{"branch":"u/x","linkedIssue":42,"linkedPR":null}},"_meta":{"runtimeId":"rt"}}`, "42"},
		{"null", `{"id":"r1","ok":true,"result":{"worktree":{"linkedIssue":null}}}`, ""},
		{"not ok", `{"id":"r1","ok":false,"error":{"code":"not_found"}}`, ""},
		{"garbage", `Orca is not running`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseOrcaLinkedIssue([]byte(tt.json)); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// fakeOrca writes an executable that answers `worktree current --json` with
// linkedIssue, or "null", and records that it ran.
func fakeOrca(t *testing.T, linkedIssue string) (path, ranMarker string) {
	t.Helper()
	dir := t.TempDir()
	ranMarker = filepath.Join(dir, "ran")
	path = filepath.Join(dir, "orca")
	script := "#!/bin/sh\ntouch '" + ranMarker + "'\n" +
		`echo '{"id":"r","ok":true,"result":{"worktree":{"linkedIssue":` + linkedIssue + `}}}'` + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, ranMarker
}

func runWithOrca(dir, bin, orca string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ORCA_CLI_COMMAND="+orca)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestWorkspaceSetupAndArchiveFromBranch(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	branchGit(t, repo, "checkout", "-q", "-b", "someuser/fix-login-race")
	orca, ran := fakeOrca(t, "null")

	out, err := runWithOrca(repo, bin, orca, "workspace", "setup")
	if err != nil {
		t.Fatalf("setup: %v\n%s", err, out)
	}
	if b, _ := os.ReadFile(stageFile(repo, "fix-login-race")); !strings.Contains(string(b), `"active"`) {
		t.Fatalf("stage not active: %s\n%s", b, out)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Fatal("queried Orca although the branch named the change")
	}

	out, err = runWithOrca(repo, bin, orca, "workspace", "archive")
	if err != nil {
		t.Fatalf("archive: %v\n%s", err, out)
	}
	if b, _ := os.ReadFile(stageFile(repo, "fix-login-race")); strings.Contains(string(b), "stage") {
		t.Fatalf("archive left an explicit stage: %s", b)
	}
}

func TestWorkspaceSetupFromOrcaLinkedIssue(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	branchGit(t, repo, "checkout", "-q", "-b", "someuser/login-work")
	refs := filepath.Join(repo, "openspec", "changes", "fix-login-race", ".specsync")
	if err := os.MkdirAll(refs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refs, "refs.json"), []byte(`{"github:o/r":{"provider":"github","id":"42","url":"https://github.com/o/r/issues/42"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	orca, _ := fakeOrca(t, "42")

	out, err := runWithOrca(repo, bin, orca, "workspace", "setup")
	if err != nil {
		t.Fatalf("setup: %v\n%s", err, out)
	}
	if !strings.Contains(out, "linked to issue #42") {
		t.Fatalf("output: %s", out)
	}
	if b, _ := os.ReadFile(stageFile(repo, "fix-login-race")); !strings.Contains(string(b), `"active"`) {
		t.Fatalf("stage not active: %s", b)
	}
	if _, err := os.Stat(stageFile(repo, "other")); err == nil {
		t.Fatal("touched an unrelated change")
	}
}

func TestWorkspaceSetupUnrelatedWorkspaceIsNoOp(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	branchGit(t, repo, "checkout", "-q", "-b", "someuser/spike")
	orca, _ := fakeOrca(t, "null")

	for _, action := range []string{"setup", "archive"} {
		out, err := runWithOrca(repo, bin, orca, "workspace", action)
		if err != nil {
			t.Fatalf("%s should exit 0 for an unrelated workspace: %v\n%s", action, err, out)
		}
		if !strings.Contains(out, "names no change") {
			t.Fatalf("%s output: %s", action, out)
		}
	}
	for _, slug := range []string{"fix-login-race", "other"} {
		if _, err := os.Stat(stageFile(repo, slug)); err == nil {
			t.Fatalf("wrote metadata for %s", slug)
		}
	}
}

func TestWorkspaceDryRunWritesNothing(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	branchGit(t, repo, "checkout", "-q", "-b", "someuser/fix-login-race")
	orca, _ := fakeOrca(t, "null")

	out, err := runWithOrca(repo, bin, orca, "workspace", "setup", "-dry-run")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "would set stage active") {
		t.Fatalf("output: %s", out)
	}
	if _, err := os.Stat(stageFile(repo, "fix-login-race")); err == nil {
		t.Fatal("dry run wrote metadata")
	}
}

func TestWorkspaceRejectsUnknownAction(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	if out, err := branchRun(repo, bin, "workspace", "open"); err == nil {
		t.Fatalf("accepted an unknown action:\n%s", out)
	}
}
