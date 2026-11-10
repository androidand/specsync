package specsync

import (
	"os"
	"path/filepath"
	"strings"
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

// fakeOpenspecStores puts an `openspec` shim on PATH whose `store list --json`
// reports the given id at the given location, and isolates the user config.
func fakeOpenspecStores(t *testing.T, id, loc string) (home string) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\necho '[{\"id\":\"" + id + "\",\"root\":\"" + loc + "\"}]'\n"
	if err := os.WriteFile(filepath.Join(bin, "openspec"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(DefaultStoreEnv, "")
	return home
}

func TestDefaultStoreFromEnvAndConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv(DefaultStoreEnv, "")
	if got := DefaultStore(); got != "" {
		t.Fatalf("nothing configured, got %q", got)
	}
	cfg := filepath.Join(home, UserConfigPath)
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("default-store: from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DefaultStore(); got != "from-file" {
		t.Fatalf("config file: got %q", got)
	}
	t.Setenv(DefaultStoreEnv, "from-env")
	if got := DefaultStore(); got != "from-env" {
		t.Fatalf("env should beat the file: got %q", got)
	}
}

func TestResolveSpecRootUsesDefaultStoreWhenNoLocalOpenspec(t *testing.T) {
	storeRoot := t.TempDir()
	fakeOpenspecStores(t, "team", storeRoot)
	t.Setenv(DefaultStoreEnv, "team")
	root, err := ResolveSpecRoot("openspec", false, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if root.Rule != RootRuleDefault || root.StoreID != "team" || root.Dir != filepath.Join(storeRoot, "openspec") {
		t.Fatalf("got %+v", root)
	}
}

func TestResolveSpecRootLocalOpenspecBeatsDefaultStore(t *testing.T) {
	fakeOpenspecStores(t, "team", t.TempDir())
	t.Setenv(DefaultStoreEnv, "team")
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "openspec", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := ResolveSpecRoot("openspec", false, "", cwd)
	if err != nil {
		t.Fatal(err)
	}
	if root.Rule != RootRuleLocal {
		t.Fatalf("local openspec must win, got %+v", root)
	}
}

func TestResolveSpecRootFlagsAndDeclaredStoreBeatDefaultStore(t *testing.T) {
	storeRoot := t.TempDir()
	fakeOpenspecStores(t, "team", storeRoot)
	t.Setenv(DefaultStoreEnv, "team")
	cwd := t.TempDir()

	root, err := ResolveSpecRoot(filepath.Join(cwd, "x"), true, "", cwd)
	if err != nil || root.Rule != RootRuleFlag {
		t.Fatalf("-openspec: %+v %v", root, err)
	}
	root, err = ResolveSpecRoot("openspec", false, "team", cwd)
	if err != nil || root.Rule != RootRuleStore {
		t.Fatalf("-store: %+v %v", root, err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "openspec", "config.yaml"), []byte("store: team\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err = ResolveSpecRoot("openspec", false, "", cwd)
	if err != nil || root.Rule != RootRuleDeclare {
		t.Fatalf("config store: %+v %v", root, err)
	}
}

func TestResolveSpecRootWithoutDefaultKeepsMissingLocalError(t *testing.T) {
	fakeOpenspecStores(t, "team", t.TempDir())
	cwd := t.TempDir()
	t.Chdir(cwd)
	root, err := ResolveSpecRoot("openspec", false, "", cwd)
	if err != nil || root.Rule != RootRuleLocal {
		t.Fatalf("got %+v %v", root, err)
	}
	err = root.Validate()
	if err == nil {
		t.Fatal("want an error for a missing openspec/")
	}
	for _, want := range []string{"specsync init", "-store <id>", "normal"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should name %q, got:\n%v", want, err)
		}
	}
}

func TestDefaultStoreIgnoresInlineCommentAndHonorsXDG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(DefaultStoreEnv, "")
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	cfg := filepath.Join(xdg, "specsync", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("default-store: team  # shared specs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DefaultStore(); got != "team" {
		t.Fatalf("got %q, want team", got)
	}
}

func TestResolveSpecRootFromSubdirectoryDoesNotUseDefaultStore(t *testing.T) {
	fakeOpenspecStores(t, "team", t.TempDir())
	t.Setenv(DefaultStoreEnv, "team")
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "openspec", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "pkg", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := ResolveSpecRoot("openspec", false, "", sub)
	if err != nil {
		t.Fatal(err)
	}
	if root.Rule != RootRuleLocal {
		t.Fatalf("a parent openspec/ must keep the default store out, got %+v", root)
	}
}
