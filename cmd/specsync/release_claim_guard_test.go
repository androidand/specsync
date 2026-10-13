package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildSpecsync compiles the specsync binary once and caches it. The test needs
// a real process because the fix is enforced through fail() -> os.Exit(1), which
// cannot be caught inside the test binary without terminating it.
func buildSpecsync(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "specsync")
	// Build from the module root so `./cmd/specsync` and the go module resolve.
	// This test file lives at cmd/specsync/, so the root is two parents up.
	root := mustModuleRoot(t)
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/specsync")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build specsync: %v\n%s", err, out)
	}
	return bin
}

// mustModuleRoot returns the directory containing go.mod, by walking up from
// this test file's location. It is stable regardless of where t.TempDir() sits.
func mustModuleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := file
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from test file")
		}
		dir = parent
	}
}

// fixtureOpenspecRepo creates a minimal openspec/ with one real change and
// returns its path. The change is a valid change; the test targets a slug that
// does not exist.
func fixtureOpenspecRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	openspec := filepath.Join(root, "openspec")
	change := filepath.Join(openspec, "changes", "feat-login")
	if err := os.MkdirAll(change, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(change, "proposal.md"), []byte("# feat-login\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return openspec
}

// runClaimRelease exercises runClaim/runRelease through a real process so the
// fail() -> os.Exit(1) path and the on-disk outcome are both observed.
func runClaimRelease(t *testing.T, bin, openspec, slug string) (exitCode int, stderr string) {
	t.Helper()
	cmd := exec.Command(bin, "claim", "-openspec", openspec, "-change", slug)
	if slug == "__release__" {
		cmd = exec.Command(bin, "release", "-openspec", openspec, "-change", slug[:len(slug)-len("__release__")])
	}
	out, err := cmd.CombinedOutput()
	stderr = string(out)
	exitCode = 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run failed to exit cleanly: %v", err)
		}
		exitCode = exitErr.ExitCode()
	}
	return exitCode, stderr
}

// metadataFile points at the path a wrong slug would materialise.
func metadataFile(openspec, slug string) string {
	return filepath.Join(openspec, "changes", slug, ".specsync", "metadata.json")
}

// TestReleaseClaim_RefusesTypoSlugIsLoadBuilding is the regression for the
// estate-wide openspec/openspec/ artifact: a typo'd change slug must exit
// non-zero and create no directory. It fails on the pre-fix code, where
// SaveChangeMetadata's MkdirAll creates changes/<typo>/.specsync/metadata.json
// and the command exits 0.
func TestReleaseClaim_RefusesTypoSlugIsLoadBuilding(t *testing.T) {
	bin := buildSpecsync(t)
	openspec := fixtureOpenspecRepo(t)
	typo := "feat-loginn"

	exit, stderr := runClaimRelease(t, bin, openspec, typo)
	if exit == 0 {
		t.Fatalf("claim with typo slug exited 0 (silent create); want non-zero. stderr=%q", stderr)
	}
	if _, err := os.Stat(metadataFile(openspec, typo)); !os.IsNotExist(err) {
		t.Fatalf("typo slug created a metadata file: err=%v", err)
	}

	// The release path is the same guard; assert it too.
	exit, _ = runClaimRelease(t, bin, openspec, "__release__"+typo)
	if exit == 0 {
		t.Fatalf("release with typo slug exited 0; want non-zero")
	}
	if _, err := os.Stat(metadataFile(openspec, typo)); !os.IsNotExist(err) {
		t.Fatalf("release typo slug created a metadata file: err=%v", err)
	}
}

// TestReleaseClaim_AcceptsRealChange asserts the fix does not over-refuse: a
// valid slug loads (claim then fails at the network-bound sync step, but it must
// NOT fail on a missing change, and must NOT create a phantom dir for the real
// slug either). This bounds the fix so it refuses only what it should.
func TestReleaseClaim_AcceptsRealChange(t *testing.T) {
	bin := buildSpecsync(t)
	openspec := fixtureOpenspecRepo(t)

	// A valid slug reaches SaveChangeMetadata (which writes metadata.json into
	// the real change dir) before the sync step; it must not error on loading.
	_, stderr := runClaimRelease(t, bin, openspec, "feat-login")
	// The command may fail later (sync needs a tracker) but must not fail with
	// "change not found", and must not create a phantom directory.
	if strings.Contains(stderr, "change not found") {
		t.Fatalf("real change wrongly refused: %q", stderr)
	}
	// No phantom changes/<anything-other-than-feat-login> dir should exist.
	changesDir := filepath.Join(openspec, "changes")
	entries, _ := os.ReadDir(changesDir)
	for _, e := range entries {
		if e.Name() != "feat-login" {
			t.Fatalf("unexpected change directory created: %s", e.Name())
		}
	}
}

// TestReleaseClaim_RefusesMissingChangesDir asserts task 2: an -openspec with no
// changes/ child is refused, so the default `-openspec openspec` run from inside
// openspec/ (which resolves to openspec/openspec/) cannot be written into.
func TestReleaseClaim_RefusesMissingChangesDir(t *testing.T) {
	bin := buildSpecsync(t)
	// An openspec path that exists but has no changes/ child.
	root := t.TempDir()
	openspec := filepath.Join(root, "openspec")
	if err := os.MkdirAll(openspec, 0o755); err != nil {
		t.Fatal(err)
	}

	exit, stderr := runClaimRelease(t, bin, openspec, "feat-login")
	if exit == 0 {
		t.Fatalf("claim against openspec without changes/ exited 0; want non-zero. stderr=%q", stderr)
	}
}
