package specsync

import (
	"os"
	"strings"
	"testing"
)

// TestNoRedundantSiteDeployWorkflow guards against reintroducing a GitHub
// Actions deploy workflow that would silently no-op (or race) alongside
// Cloudflare Pages' own git integration, which is the actual deploy path —
// see site/README.md.
func TestNoRedundantSiteDeployWorkflow(t *testing.T) {
	if _, err := os.Stat(".github/workflows/deploy-site.yml"); err == nil {
		t.Fatal("deploy-site.yml exists again — Cloudflare Pages' git integration is the real deploy path (site/README.md); a second Actions-based deploy will confuse or race it")
	}
}

func TestSiteDeclaresCanonicalDomain(t *testing.T) {
	page, err := os.ReadFile("site/index.html")
	if err != nil {
		t.Fatal(err)
	}
	content := string(page)
	for _, required := range []string{
		`<link rel="canonical" href="https://specsync.se/">`,
		`<meta property="og:url" content="https://specsync.se/">`,
	} {
		if !strings.Contains(content, required) {
			t.Errorf("site metadata is missing %q", required)
		}
	}
}
