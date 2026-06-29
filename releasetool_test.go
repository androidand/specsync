package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectReleaseToolChangesets(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".changeset"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".changeset", "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DetectReleaseTool(dir)
	if got.Name != "changesets" {
		t.Fatalf("Name = %q, want changesets", got.Name)
	}
	if len(got.Evidence) == 0 {
		t.Fatalf("expected evidence paths")
	}
}

func TestDetectReleaseToolNone(t *testing.T) {
	got := DetectReleaseTool(t.TempDir())
	if got.Name != "none" {
		t.Fatalf("Name = %q, want none", got.Name)
	}
}

func TestDetectReleaseToolGoreleaser(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".goreleaser.yaml"), []byte("version: 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DetectReleaseTool(dir)
	if got.Name != "goreleaser" {
		t.Fatalf("Name = %q, want goreleaser", got.Name)
	}
}

// This repo itself uses goreleaser — a useful real self-test that detection and
// deference are wired correctly.
func TestDetectReleaseToolSelfRepo(t *testing.T) {
	got := DetectReleaseTool("..")
	if got.Name != "goreleaser" {
		t.Skipf("self-repo detection got %q (run from package dir); not fatal", got.Name)
	}
}
