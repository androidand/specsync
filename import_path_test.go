package specsync

import (
	"go/build"
	"strings"
	"testing"
)

// TestImportPath guards the documented import path: the package must live at
// the module root so that the path in doc.go (github.com/androidand/specsync)
// always resolves. If the package is ever relocated, this test fails until
// the documentation is updated to match.
func TestImportPath(t *testing.T) {
	// Import the module by its canonical path; this resolves to the actual
	// directory regardless of where the test is run from.
	pkg, err := build.Import("github.com/androidand/specsync", ".", build.ImportMode(0))
	if err != nil {
		t.Fatalf("import specsync: %v", err)
	}
	if !strings.HasSuffix(pkg.Dir, "specsync") {
		t.Errorf("package directory %q does not match module root", pkg.Dir)
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
