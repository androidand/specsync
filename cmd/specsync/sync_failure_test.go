package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSyncExitsNonZeroWhenAChangeFails pins #130: one change that cannot be
// synced must fail the run, show a failure count, and be named in a trailing
// block, while the other changes still sync.
func TestSyncExitsNonZeroWhenAChangeFails(t *testing.T) {
	dir := t.TempDir()
	write := func(slug, proposal string) {
		d := filepath.Join(dir, "openspec", "changes", slug)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "proposal.md"), []byte(proposal), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "tasks.md"), []byte("- [ ] 1. a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("big-change", "# Big\n\n## Why\n"+strings.Repeat("x", 70000)+"\n")
	write("small-change", "# Small\n\n## Why\nfine\n")

	cmd := exec.Command(buildSpecsyncBinary(t), "sync", "-dry-run", "-repo", "example/demo")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("exit 0 although big-change cannot sync:\n%s", out)
	}
	s := string(out)
	if !strings.Contains(s, "1 failed") {
		t.Errorf("summary has no failure count:\n%s", s)
	}
	idx := strings.LastIndex(s, "big-change")
	if idx < 0 || idx < strings.LastIndex(s, "small-change") {
		t.Errorf("failed slug not listed after the per-change output:\n%s", s)
	}
}
