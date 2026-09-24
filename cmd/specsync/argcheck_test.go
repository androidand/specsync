package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestShadowedFlags pins the exact 2026-09-24 incident shape: a flag placed
// after a positional-looking token is never seen by fs.Parse (Go's flag
// package stops parsing at the first non-flag argument) and must be reported
// as shadowed instead of silently keeping its zero value.
func TestShadowedFlags(t *testing.T) {
	newFS := func() *flag.FlagSet {
		fs := flag.NewFlagSet("sync", flag.ContinueOnError)
		fs.String("repo", "", "")
		fs.String("change", "", "")
		fs.Bool("dry-run", false, "")
		return fs
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "incident shape: url before -dry-run",
			args: []string{"-repo", "ExopenGitHub/portal", "https://github.com/ExopenGitHub/portal/issues/4304", "-dry-run"},
			want: []string{"-dry-run"},
		},
		{
			name: "no positional: nothing shadowed",
			args: []string{"-repo", "owner/repo", "-dry-run"},
			want: nil,
		},
		{
			name: "flags before positional: nothing shadowed",
			args: []string{"-dry-run", "-repo", "owner/repo", "some-positional"},
			want: nil,
		},
		{
			name: "value flag after positional is also shadowed",
			args: []string{"some-positional", "-repo", "owner/repo"},
			want: []string{"-repo"},
		},
		{
			name: "inline = value flag after positional",
			args: []string{"some-positional", "-repo=owner/repo"},
			want: []string{"-repo"},
		},
		{
			name: "unregistered flag after positional is not reported",
			args: []string{"some-positional", "-not-a-real-flag"},
			want: nil,
		},
		{
			name: "value flag immediately followed by another shadowed flag: both reported",
			args: []string{"some-positional", "-repo", "-dry-run"},
			want: []string{"-repo", "-dry-run"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFS()
			got := shadowedFlags(fs, tt.args)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("shadowedFlags(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// TestCheckArgs verifies checkArgs reports both an unconsumed-positional
// problem and a shadowed-flag problem in the same call when both are
// present — the exact combination that turned the 2026-09-24 incident's
// mistyped invocation into a full live sync with zero feedback.
func TestCheckArgs(t *testing.T) {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.String("repo", "", "")
	fs.Bool("dry-run", false, "")

	rawArgs := []string{"-repo", "ExopenGitHub/portal", "https://github.com/ExopenGitHub/portal/issues/4304", "-dry-run"}
	_ = fs.Parse(rawArgs)
	if fs.NArg() != 2 {
		t.Fatalf("expected 2 leftover args, got %d: %v", fs.NArg(), fs.Args())
	}

	err := checkArgsErr(fs, rawArgs, 0)
	if err == nil {
		t.Fatal("expected checkArgs to report a problem, got nil")
	}
	if !strings.Contains(err.Error(), "-dry-run") {
		t.Errorf("error %q does not name the shadowed flag -dry-run", err.Error())
	}
	if !strings.Contains(err.Error(), "https://github.com/ExopenGitHub/portal/issues/4304") {
		t.Errorf("error %q does not name the unconsumed URL argument", err.Error())
	}
	if strings.Count(err.Error(), "-dry-run") != 1 {
		t.Errorf("error %q names -dry-run more than once — it should be reported only as shadowed, not also as a separate unexpected argument", err.Error())
	}
}

var buildOnce sync.Once
var builtBinary string
var buildErr error

// buildSpecsyncBinary compiles the specsync CLI once per test run and
// returns the path to the resulting binary.
func buildSpecsyncBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir := t.TempDir()
		builtBinary = filepath.Join(dir, "specsync-test-bin")
		cmd := exec.Command("go", "build", "-o", builtBinary, ".")
		cmd.Dir = "."
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = err
			t.Logf("go build output: %s", out)
		}
	})
	if buildErr != nil {
		t.Fatalf("build specsync binary: %v", buildErr)
	}
	return builtBinary
}

// TestIncidentRegression reproduces the exact 2026-09-24 incident's argument
// shape end to end against the compiled binary: `specsync -repo <repo> <url>
// -dry-run` run against a directory whose openspec/changes/ holds changes
// unrelated to <repo> must fail with a clear error, make zero provider
// calls, and exit non-zero. A stub `gh` on PATH records every invocation so
// a live sync (the actual incident) would be unmistakable.
func TestIncidentRegression(t *testing.T) {
	bin := buildSpecsyncBinary(t)

	fixture := t.TempDir()
	changeDir := filepath.Join(fixture, "openspec", "changes", "unrelated-change")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Unrelated change\n\n## Why\n\nUnrelated.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stub `gh` that records every call it receives; the fix must produce
	// zero calls to it.
	binDir := t.TempDir()
	callLog := filepath.Join(binDir, "gh-calls.log")
	ghStub := "#!/bin/sh\necho \"$@\" >> " + callLog + "\necho '[]'\n"
	ghPath := filepath.Join(binDir, "gh")
	if err := os.WriteFile(ghPath, []byte(ghStub), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "-repo", "ExopenGitHub/portal",
		"https://github.com/ExopenGitHub/portal/issues/4304", "-dry-run")
	cmd.Dir = fixture
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("expected non-zero exit, got success; output:\n%s", out)
	}
	if !strings.Contains(string(out), "unexpected argument") && !strings.Contains(string(out), "not applied") {
		t.Errorf("expected a clear unexpected-argument/shadowed-flag error, got:\n%s", out)
	}
	if _, statErr := os.Stat(callLog); statErr == nil {
		logged, _ := os.ReadFile(callLog)
		t.Fatalf("expected zero gh calls, got:\n%s", logged)
	}
}
