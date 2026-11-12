package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestHelperRunMain re-executes the test binary as the CLI: when
// SPECSYNC_RUN_MAIN is set it runs main() with the args after "--".
func TestHelperRunMain(t *testing.T) {
	if os.Getenv("SPECSYNC_RUN_MAIN") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			os.Args = append([]string{"specsync"}, args[i+1:]...)
			break
		}
	}
	main()
	os.Exit(0)
}

// TestMutatingCommandsRefuseUnknownChange pins the invariant behind #207: a
// command that mutates a change by slug must fail, and write nothing, when
// the slug names no change. A typo must never leave a directory behind.
func TestMutatingCommandsRefuseUnknownChange(t *testing.T) {
	for _, args := range [][]string{
		{"set-stage", "no-such-change", "active"},
		{"set-priority", "no-such-change", "5"},
		{"note", "no-such-change", "hello"},
	} {
		t.Run(args[0], func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "openspec", "changes", "real"), 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestHelperRunMain", "--"}, args...)...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "SPECSYNC_RUN_MAIN=1")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Errorf("%v exited 0 for an unknown change:\n%s", args, out)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "openspec", "changes", "no-such-change")); statErr == nil {
				t.Errorf("%v created openspec/changes/no-such-change", args)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "openspec", "openspec")); statErr == nil {
				t.Errorf("%v created openspec/openspec", args)
			}
		})
	}
}
