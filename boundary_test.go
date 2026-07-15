package specsync

import (
	"go/build"
	"strings"
	"testing"
)

// TestStdlibOnly enforces the invariant documented in doc.go: the specsync
// package depends only on the Go standard library and internal sub-packages.
// External dependencies would come from third-party modules like "github.com/...".
// Keeping this green is what lets specsync stay a small, embeddable, single-binary tool.
func TestStdlibOnly(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("import dir: %v", err)
	}
	const thisModule = "github.com/androidand/specsync"
	for _, imp := range pkg.Imports {
		// Allow stdlib (no dots) and internal sub-packages (start with this module)
		if strings.Contains(imp, ".") && !strings.HasPrefix(imp, thisModule) {
			t.Errorf("specsync must depend only on the standard library, found %q", imp)
		}
	}
}
