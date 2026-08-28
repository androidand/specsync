package specsync

import (
	"go/build"
	"os"
	"testing"
)

// TestImportPath guards the documented import path: the package must live at
// the module root so that the path in doc.go (github.com/androidand/specsync)
// always resolves, not nested under pkg/ or similar. If the package is ever
// relocated, this test fails until the documentation is updated to match.
//
// The check compares against the test binary's own working directory (which
// `go test` always sets to the package's source directory) rather than a
// literal "specsync" folder-name check: a checkout or git worktree can be
// named anything (this repo's own worktree convention names them after the
// branch, e.g. "specsync-<change>"), so asserting on the basename would fail
// outside the one clone happening to be named "specsync" — checking that the
// canonical import path resolves back to wherever the package's own files
// actually are is the real invariant, and it holds regardless of the
// directory's name.
func TestImportPath(t *testing.T) {
	// Import the module by its canonical path; this resolves to the actual
	// directory regardless of where the test is run from.
	pkg, err := build.Import("github.com/androidand/specsync", ".", build.ImportMode(0))
	if err != nil {
		t.Fatalf("import specsync: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if pkg.Dir != wd {
		t.Errorf("package directory %q does not match the module root %q", pkg.Dir, wd)
	}
}

// TestExportedSymbols ensures the surface that downstream consumers embed
// does not silently disappear. These are compile-time checks: if any symbol
// is renamed or removed the test will not compile.
func TestExportedSymbols(t *testing.T) {
	// Functions
	var _ = Sync
	var _ = Pull
	var _ = Link
	var _ = Spinoff
	var _ = LoadChange
	var _ = WorkItemFor

	// Types (interfaces and structs that consumers rely on)
	var _ WorkProvider

	// Verify LoadChange is the right signature (not LoadChanges)
	_ = func(dir string, archived bool, openspecDir string) (*Change, error) {
		return LoadChange(dir, archived, openspecDir)
	}
}
