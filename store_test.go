package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTargetsBlockAndInline(t *testing.T) {
	block := "targets:\n  - github:acme/backend\n  - github:acme/frontend\n"
	got := parseTargets(block)
	if len(got) != 2 || got[0] != "github:acme/backend" || got[1] != "github:acme/frontend" {
		t.Fatalf("block form: got %v", got)
	}
	inline := parseTargets(`targets: [github:acme/backend, "github:acme/frontend"]`)
	if len(inline) != 2 || inline[1] != "github:acme/frontend" {
		t.Fatalf("inline form: got %v", inline)
	}
	if got := parseTargets("board: acme/6\n"); len(got) != 0 {
		t.Fatalf("no targets key should yield none, got %v", got)
	}
}

func TestWriteChangeTargetsPreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ChangeConfigPath)
	if err := os.WriteFile(path, []byte("board: acme/6\ntargets:\n  - github:acme/old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteChangeTargets(dir, []string{"github:acme/new"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "board: acme/6\ntargets:\n  - github:acme/new\n" {
		t.Fatalf("unexpected rewrite:\n%q", got)
	}
	if got := ChangeTargets(dir); len(got) != 1 || got[0] != "github:acme/new" {
		t.Fatalf("round-trip: got %v", got)
	}
}

func TestGitHubTargetReposSkipsOtherProviders(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ChangeConfigPath),
		[]byte("targets:\n  - github:acme/backend\n  - beads\n  - mcp:linear\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := GitHubTargetRepos(dir)
	if len(got) != 1 || got[0] != "acme/backend" {
		t.Fatalf("got %v", got)
	}
}

func TestResolveSpecRootPrefersFlagOverDeclaredStore(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "openspec", "config.yaml"), []byte("store: some-store\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(cwd, "elsewhere")
	root, err := ResolveSpecRoot(explicit, true, "", cwd)
	if err != nil {
		t.Fatal(err)
	}
	if root.Rule != RootRuleFlag || root.Dir != explicit {
		t.Fatalf("got %+v", root)
	}
}

func TestResolveSpecRootFallsBackToRepoLocal(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "openspec", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := ResolveSpecRoot("openspec", false, "", cwd)
	if err != nil {
		t.Fatal(err)
	}
	if root.Rule != RootRuleLocal || root.IsStore() {
		t.Fatalf("got %+v", root)
	}
}

func TestResolvedRootValidateReportsMissingChanges(t *testing.T) {
	dir := t.TempDir()
	if err := ResolveRootFor(dir).Validate(); err == nil {
		t.Fatal("expected an error for a root with no changes/ directory")
	}
}

// ResolveRootFor is a test helper mirroring a repo-local resolution.
func ResolveRootFor(dir string) ResolvedRoot {
	return ResolvedRoot{Dir: dir, Rule: RootRuleLocal}
}

func TestStoreIDAtDetectsMarker(t *testing.T) {
	storeRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(storeRoot, ".openspec-store"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeRoot, StoreMarkerPath), []byte("version: 1\nid: team-plans\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := storeIDAt(filepath.Join(storeRoot, "openspec")); got != "team-plans" {
		t.Fatalf("got %q", got)
	}
	if got := storeIDAt(filepath.Join(t.TempDir(), "openspec")); got != "" {
		t.Fatalf("non-store should yield empty, got %q", got)
	}
}

func TestParseStoreListEnvelopes(t *testing.T) {
	wrapped := []byte(`{"stores":[{"id":"a","root":"/tmp/a"}]}`)
	got, err := parseStoreList(wrapped)
	if err != nil || len(got) != 1 || got[0].location() != "/tmp/a" {
		t.Fatalf("wrapped: %v %v", got, err)
	}
	bare := []byte(`[{"id":"b","location":"/tmp/b"}]`)
	got, err = parseStoreList(bare)
	if err != nil || len(got) != 1 || got[0].location() != "/tmp/b" {
		t.Fatalf("bare: %v %v", got, err)
	}
}
