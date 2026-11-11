package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGHOnPath puts a `gh` first on PATH that records every call and fails,
// and returns the PATH value and the log file.
func fakeGHOnPath(t *testing.T) (pathEnv, logFile string) {
	t.Helper()
	dir := t.TempDir()
	logFile = filepath.Join(dir, "gh.log")
	script := "#!/bin/sh\necho \"$*\" >> '" + logFile + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir + string(os.PathListSeparator) + os.Getenv("PATH"), logFile
}

func runWithPath(dir, bin, pathEnv string, args ...string) (string, int) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+pathEnv)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return string(out), code
}

func ghCalls(t *testing.T, logFile string) string {
	t.Helper()
	b, err := os.ReadFile(logFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

// TestEmptyInvocationPrintsUsageAndNeverReachesTheTracker pins the incident:
// an agent ran bare `specsync` to see its commands and it synced every change.
func TestEmptyInvocationPrintsUsageAndNeverReachesTheTracker(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	pathEnv, logFile := fakeGHOnPath(t)

	for _, args := range [][]string{nil, {"-h"}, {"--help"}, {"help"}} {
		out, code := runWithPath(repo, bin, pathEnv, args...)
		if code != 0 {
			t.Fatalf("specsync %v exited %d:\n%s", args, code, out)
		}
		for _, want := range []string{"Usage: specsync <command>", "pull", "set-stage", "-all"} {
			if !strings.Contains(out, want) {
				t.Fatalf("specsync %v output lacks %q:\n%s", args, want, out)
			}
		}
		if calls := ghCalls(t, logFile); calls != "" {
			t.Fatalf("specsync %v called the tracker:\n%s", args, calls)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "openspec", "changes", "fix-login-race", ".specsync")); err == nil {
		t.Fatal("the empty invocation wrote a ref cache")
	}
}

func TestUnscopedSyncIsRefusedBeforeTheTracker(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	pathEnv, logFile := fakeGHOnPath(t)

	for _, args := range [][]string{{"-repo", "o/r"}, {"sync", "-repo", "o/r"}} {
		out, code := runWithPath(repo, bin, pathEnv, args...)
		if code == 0 || !strings.Contains(out, "-change <slug>") || !strings.Contains(out, "-all") || !strings.Contains(out, "-dry-run") {
			t.Fatalf("specsync %v: exit %d, want a refusal naming -change, -all and -dry-run:\n%s", args, code, out)
		}
		if calls := ghCalls(t, logFile); calls != "" {
			t.Fatalf("specsync %v called the tracker before refusing:\n%s", args, calls)
		}
	}
}

func TestScopedOrPreviewSyncIsNotRefused(t *testing.T) {
	for _, tc := range []struct {
		name, slug string
		all, dry   bool
	}{
		{"one change", "fix-login-race", false, false},
		{"every change", "", true, false},
		{"preview", "", false, true},
	} {
		if err := unscopedSyncErr(tc.slug, tc.all, tc.dry); err != nil {
			t.Fatalf("%s: refused: %v", tc.name, err)
		}
	}
	if err := unscopedSyncErr("", false, false); err == nil {
		t.Fatal("an unscoped writing sync must be refused")
	}
}

func TestUnscopedDryRunStillPreviews(t *testing.T) {
	bin := buildSpecsync(t)
	repo := changeRepo(t)
	pathEnv, _ := fakeGHOnPath(t)

	out, code := runWithPath(repo, bin, pathEnv, "-dry-run", "-repo", "o/r")
	if code != 0 || !strings.Contains(out, "DRY RUN") || !strings.Contains(out, "fix-login-race") {
		t.Fatalf("unscoped dry run: exit %d\n%s", code, out)
	}
}
