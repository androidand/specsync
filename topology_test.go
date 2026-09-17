package specsync

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestChangeTopology_JSONRoundTripsWithEveryOptionalFieldAbsent(t *testing.T) {
	row := ChangeTopology{Slug: "add-thing", Title: "Add thing", Stage: "active"}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ChangeTopology
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, row) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, row)
	}
	// Every optional field must be omitted from the wire form, not emitted as
	// a zero value (null/"") — that's the whole point of omitempty here.
	for _, field := range []string{`"store"`, `"repo"`, `"issue"`, `"branch"`, `"worktree"`, `"epic"`, `"linked"`} {
		if strings.Contains(string(b), field) {
			t.Errorf("expected %s to be omitted from JSON, got %s", field, b)
		}
	}
}

func fakeGitWorktreeList(porcelain map[string]string, calls *[]string) func(context.Context, string) (string, error) {
	return func(_ context.Context, dir string) (string, error) {
		if calls != nil {
			*calls = append(*calls, dir)
		}
		out, ok := porcelain[dir]
		if !ok {
			return "", errors.New("no such repo: " + dir)
		}
		return out, nil
	}
}

func fakeGitRemoteOrigin(repos map[string]string) func(context.Context, string) (string, error) {
	return func(_ context.Context, dir string) (string, error) {
		repo, ok := repos[dir]
		if !ok {
			return "", errors.New("no remote for " + dir)
		}
		return repo, nil
	}
}

func TestBuildTopology_FullyResolvedSingleStore(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "add-field-forms")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Add field forms\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "refs.json"),
		`{"github:acme/portal":{"provider":"github:acme/portal","id":"4231","url":"https://github.com/acme/portal/issues/4231"}}`)

	worktreePath := filepath.Join(root, "..", "worktrees", "add-field-forms")
	porcelain := "worktree " + filepath.Dir(root) + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + worktreePath + "\nHEAD def\nbranch refs/heads/feat/4231-add-field-forms\n"

	opts := TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{filepath.Dir(root): porcelain}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{filepath.Dir(root): "acme/portal"}),
		run: func(_ context.Context, args ...string) (string, error) {
			return "open", nil // -q .state
		},
	}

	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildTopology: %v", err)
	}
	if len(topo.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(topo.Changes), topo.Changes)
	}
	row := topo.Changes[0]
	if row.Slug != "add-field-forms" {
		t.Errorf("Slug = %q", row.Slug)
	}
	if row.Repo != "acme/portal" {
		t.Errorf("Repo = %q, want acme/portal", row.Repo)
	}
	if row.Issue == nil || row.Issue.URL != "https://github.com/acme/portal/issues/4231" {
		t.Fatalf("Issue = %+v", row.Issue)
	}
	if row.Issue.State != "open" {
		t.Errorf("Issue.State = %q, want open", row.Issue.State)
	}
	if row.Branch != "feat/4231-add-field-forms" {
		t.Errorf("Branch = %q", row.Branch)
	}
	if row.Worktree != worktreePath {
		t.Errorf("Worktree = %q, want %q", row.Worktree, worktreePath)
	}
}

func TestBuildTopology_ChangeWithNoWorktreeStillReportedExitZero(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "c1", "proposal.md"), "# C1\n")
	mustWrite(t, filepath.Join(root, "changes", "c1", ".specsync", "refs.json"),
		`{"github:acme/portal":{"provider":"github:acme/portal","id":"1","url":"https://github.com/acme/portal/issues/1"}}`)

	porcelain := "worktree " + filepath.Dir(root) + "\nHEAD abc\nbranch refs/heads/main\n"
	opts := TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{root: porcelain}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{root: "acme/portal"}),
		run:              func(context.Context, ...string) (string, error) { return "open", nil },
	}

	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildTopology must not error: %v", err)
	}
	if len(topo.Changes) != 1 {
		t.Fatalf("expected 1 row even with no worktree, got %d", len(topo.Changes))
	}
	if topo.Changes[0].Worktree != "" {
		t.Errorf("expected null/empty worktree, got %q", topo.Changes[0].Worktree)
	}
}

func TestBuildTopology_GHUnauthenticatedOmitsStateNotesReason(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "c1", "proposal.md"), "# C1\n")
	mustWrite(t, filepath.Join(root, "changes", "c1", ".specsync", "refs.json"),
		`{"github:acme/portal":{"provider":"github:acme/portal","id":"1","url":"https://github.com/acme/portal/issues/1"}}`)

	opts := TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{root: ""}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{root: "acme/portal"}),
		run:              func(context.Context, ...string) (string, error) { return "", errors.New("gh: not authenticated") },
	}

	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildTopology: %v", err)
	}
	if len(topo.Changes) != 1 {
		t.Fatalf("expected 1 row despite gh being unavailable, got %d", len(topo.Changes))
	}
	row := topo.Changes[0]
	if row.Issue == nil || row.Issue.URL == "" {
		t.Fatalf("expected the issue id/URL from the local ref cache regardless of gh, got %+v", row.Issue)
	}
	if row.Issue.State != "" {
		t.Errorf("expected State empty when gh is unavailable, got %q", row.Issue.State)
	}
	found := false
	for _, s := range topo.Status {
		if s == "gh unavailable: issue state omitted" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a status line explaining the omission, got %v", topo.Status)
	}
}

func TestBuildTopology_NothingResolvedIsAnEmptyResultNotAnError(t *testing.T) {
	root := t.TempDir() // no changes/ dir at all
	opts := TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildTopology: %v", err)
	}
	if len(topo.Changes) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(topo.Changes))
	}
	// The CLI, not BuildTopology, decides the exit code for "nothing resolved"
	// (see cmd/specsync/topology.go) — BuildTopology itself just reports an
	// empty, error-free result.
}

func TestBuildTopology_ReadOnly_NoCacheOrGitStateWritten(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "c1")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# C1\n")

	opts := TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{root: ""}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{}),
	}
	if _, err := BuildTopology(context.Background(), opts); err != nil {
		t.Fatalf("BuildTopology: %v", err)
	}
	if _, err := LoadRefs(cdir); err != nil {
		t.Fatalf("LoadRefs after BuildTopology: %v", err)
	}
	// No .specsync dir should have been created as a side effect.
	entries, _ := os.ReadDir(cdir)
	for _, e := range entries {
		if e.Name() == ".specsync" {
			t.Errorf("BuildTopology must not write a ref cache as a side effect")
		}
	}
}

func TestBuildTopology_ScopesToOneChange(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "a", "proposal.md"), "# A\n")
	mustWrite(t, filepath.Join(root, "changes", "b", "proposal.md"), "# B\n")

	opts := TopologyOptions{
		OpenSpecDir:      root,
		Slug:             "a",
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{root: ""}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{}),
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildTopology: %v", err)
	}
	if len(topo.Changes) != 1 || topo.Changes[0].Slug != "a" {
		t.Fatalf("expected only change 'a', got %+v", topo.Changes)
	}
}

func TestBuildTopology_ArchivedExcludedByDefaultIncludedWithAll(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "active-one", "proposal.md"), "# Active\n")
	mustWrite(t, filepath.Join(root, "changes", "archive", "2026-01-01-old-one", "proposal.md"), "# Old\n")

	base := TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{root: ""}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{}),
	}

	topo, err := BuildTopology(context.Background(), base)
	if err != nil {
		t.Fatalf("BuildTopology: %v", err)
	}
	if len(topo.Changes) != 1 {
		t.Fatalf("expected archived change excluded by default, got %+v", topo.Changes)
	}

	withAll := base
	withAll.All = true
	topo, err = BuildTopology(context.Background(), withAll)
	if err != nil {
		t.Fatalf("BuildTopology -all: %v", err)
	}
	if len(topo.Changes) != 2 {
		t.Fatalf("expected both changes with -all, got %+v", topo.Changes)
	}
}

func TestBuildTopology_MultiRepoJoinAcrossCoordinatedStores(t *testing.T) {
	rootStore := t.TempDir()
	siblingRepo := t.TempDir()
	mustWrite(t, filepath.Join(rootStore, "changes", "backend-part", "proposal.md"), "# Backend part\n")
	mustWrite(t, filepath.Join(siblingRepo, "openspec", "changes", "frontend-part", "proposal.md"), "# Frontend part\n")

	opts := TopologyOptions{
		OpenSpecDir: rootStore,
		readCoordination: func(context.Context) (*Coordination, error) {
			return &Coordination{
				Root:    StoreEntry{Path: filepath.Dir(rootStore)},
				Members: []StoreEntry{{Path: siblingRepo}},
			}, nil
		},
		gitWorktreeList: fakeGitWorktreeList(map[string]string{
			filepath.Dir(rootStore): "",
			siblingRepo:             "",
		}, nil),
		gitRemoteOrigin: fakeGitRemoteOrigin(map[string]string{
			filepath.Dir(rootStore): "acme/backend",
			siblingRepo:             "acme/frontend",
		}),
	}

	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildTopology: %v", err)
	}
	if len(topo.Changes) != 2 {
		t.Fatalf("expected 2 changes across both stores, got %+v", topo.Changes)
	}
	byRepo := map[string]string{}
	for _, c := range topo.Changes {
		byRepo[c.Slug] = c.Repo
	}
	if byRepo["backend-part"] != "acme/backend" || byRepo["frontend-part"] != "acme/frontend" {
		t.Errorf("expected each change to carry its own store's repo, got %+v", byRepo)
	}
}

func TestBuildTopology_LinksResolveToLocalSiblingSlugs(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "api-part", "proposal.md"), "# API part\n")
	mustWrite(t, filepath.Join(root, "changes", "api-part", ".specsync", "refs.json"),
		`{"github:acme/portal":{"provider":"github:acme/portal","id":"10","url":"https://github.com/acme/portal/issues/10"}}`)
	mustWrite(t, filepath.Join(root, "changes", "ui-part", "proposal.md"), "# UI part\n")
	mustWrite(t, filepath.Join(root, "changes", "ui-part", "links.md"), "- https://github.com/acme/portal/issues/10\n")

	opts := TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{root: ""}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{root: "acme/portal"}),
	}

	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildTopology: %v", err)
	}
	var ui *ChangeTopology
	for i := range topo.Changes {
		if topo.Changes[i].Slug == "ui-part" {
			ui = &topo.Changes[i]
		}
	}
	if ui == nil {
		t.Fatalf("ui-part not found in %+v", topo.Changes)
	}
	if len(ui.Linked) != 1 || ui.Linked[0] != "api-part" {
		t.Errorf("Linked = %v, want [api-part]", ui.Linked)
	}
}

func TestBuildTopology_EpicResolvedViaLabelOnUnresolvedRelatedLink(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "child-part", "proposal.md"), "# Child part\n")
	mustWrite(t, filepath.Join(root, "changes", "child-part", "links.md"), "- https://github.com/acme/planning/issues/88\n")

	opts := TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{root: ""}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{root: "acme/portal"}),
		run: func(_ context.Context, args ...string) (string, error) {
			return "type:epic\nspecsync", nil // -q .labels[].name
		},
	}

	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatalf("BuildTopology: %v", err)
	}
	if len(topo.Changes) != 1 || topo.Changes[0].Epic != "https://github.com/acme/planning/issues/88" {
		t.Fatalf("expected the related link to resolve as Epic, got %+v", topo.Changes)
	}
}
