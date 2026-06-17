package specsync

import (
	"go/build"
	"strings"
	"testing"
)

// TestStdlibOnly enforces the invariant documented in doc.go: the specsync
// package depends only on the Go standard library. Standard-library import
// paths never contain a dot ("os", "go/build", "encoding/json"); any external
// dependency would appear as a dotted path ("github.com/..."). Keeping this
// green is what lets specsync stay a small, embeddable, single-binary tool.
func TestStdlibOnly(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("import dir: %v", err)
	}
	for _, imp := range pkg.Imports {
		if strings.Contains(imp, ".") {
			t.Errorf("specsync must depend only on the standard library, found %q", imp)
		}
	}
}
