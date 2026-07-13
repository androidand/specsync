package specsync

import (
	"os"
	"strings"
	"testing"
)

func TestSiteDeploymentContract(t *testing.T) {
	workflow, err := os.ReadFile(".github/workflows/deploy-site.yml")
	if err != nil {
		t.Fatal(err)
	}
	content := string(workflow)
	for _, required := range []string{
		"branches: [main]",
		"workflow_dispatch:",
		"CLOUDFLARE_PAGES_ENABLED",
		"node build.sh",
		"cloudflare/wrangler-action@v3",
		"CLOUDFLARE_API_TOKEN",
		"CLOUDFLARE_ACCOUNT_ID",
		"pages deploy site --project-name=specsync --branch=main",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("deployment workflow is missing %q", required)
		}
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
