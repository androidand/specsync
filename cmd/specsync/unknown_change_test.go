package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
			cmd := exec.Command(buildSpecsyncBinary(t), args...)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Errorf("%v exited 0 for an unknown change:\n%s", args, out)
			}
			if !strings.Contains(string(out), "no-such-change") {
				t.Errorf("%v: output does not name the unknown slug:\n%s", args, out)
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
