package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func archiveRepo(t *testing.T) (repo string, env []string) {
	t.Helper()
	repo, env = firstUseEnv(t)
	dir := filepath.Join(repo, "openspec", "changes", "idea")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# Idea\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte("- [ ] todo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, env
}

func TestArchiveReasonFlagsAreValidatedBeforeAnythingHappens(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	cases := map[string][]string{
		"-duplicate-of": {"-reason", "duplicate"},
		"unknown":       {"-reason", "wontfix"},
	}
	for want, extra := range cases {
		repo, env := archiveRepo(t)
		out, err := runIn(t, bin, repo, env, append([]string{"archive", "-change", "idea"}, extra...)...)
		if err == nil || !strings.Contains(out, want) {
			t.Fatalf("%v: want a refusal mentioning %q, got err=%v out=%s", extra, want, err, out)
		}
		if _, statErr := os.Stat(filepath.Join(repo, "openspec", "changes", "idea")); statErr != nil {
			t.Fatalf("%v: the change must stay in place: %v", extra, statErr)
		}
	}
}

func TestArchiveNotPlannedDryRunNeedsNoForce(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := archiveRepo(t)
	out, err := runIn(t, bin, repo, env, "archive", "-change", "idea", "-reason", "not-planned", "-dry-run")
	if err != nil {
		t.Fatalf("archive -reason not-planned -dry-run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "close as not planned") || strings.Contains(out, "use -force") {
		t.Fatalf("unexpected plan:\n%s", out)
	}
}
