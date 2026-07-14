package specsync

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestInstallInstructionsMatchCanonicalSources guards against README.md and
// site/index.html silently drifting from each other — or from the actual
// package manifests — since the npm package name and Go module path are each
// hardcoded independently in both docs rather than generated from one source.
func TestInstallInstructionsMatchCanonicalSources(t *testing.T) {
	goMod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindStringSubmatch(string(goMod))
	if m == nil {
		t.Fatal("could not find module path in go.mod")
	}
	modulePath := m[1]
	goInstallCmd := modulePath + "/cmd/specsync@latest"

	pkgJSON, err := os.ReadFile("npm/package.json")
	if err != nil {
		t.Fatal(err)
	}
	n := regexp.MustCompile(`"name":\s*"([^"]+)"`).FindStringSubmatch(string(pkgJSON))
	if n == nil {
		t.Fatal("could not find package name in npm/package.json")
	}
	npmPackage := n[1]

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	site, err := os.ReadFile("site/index.html")
	if err != nil {
		t.Fatal(err)
	}

	for _, doc := range []struct {
		name    string
		content string
	}{
		{"README.md", string(readme)},
		{"site/index.html", string(site)},
	} {
		if !strings.Contains(doc.content, npmPackage) {
			t.Errorf("%s doesn't mention npm package %q (npm/package.json) — install instructions have drifted", doc.name, npmPackage)
		}
		if !strings.Contains(doc.content, goInstallCmd) {
			t.Errorf("%s doesn't mention %q (go.mod module path) — install instructions have drifted", doc.name, goInstallCmd)
		}
	}
}
