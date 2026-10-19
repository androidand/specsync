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

func TestChangeTopology_LocalAndAttentionOmittedWhenAbsent(t *testing.T) {
	b, err := json.Marshal(ChangeTopology{Slug: "x", Title: "X", Stage: "active"})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"local"`, `"attention"`} {
		if strings.Contains(string(b), field) {
			t.Errorf("expected %s to be omitted, got %s", field, b)
		}
	}
}

func localStateFixture(t *testing.T) (TopologyOptions, string) {
	t.Helper()
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "add-field-forms")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Add field forms\n")
	worktreePath := filepath.Join(root, "..", "worktrees", "add-field-forms")
	porcelain := "worktree " + filepath.Dir(root) + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + worktreePath + "\nHEAD def\nbranch refs/heads/feat/4231-add-field-forms\n"
	return TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{filepath.Dir(root): porcelain}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{filepath.Dir(root): "acme/portal"}),
		run:              func(context.Context, ...string) (string, error) { return "", errors.New("gh offline") },
	}, worktreePath
}

func TestBuildTopology_AttachesLocalStateAndAttention(t *testing.T) {
	opts, worktreePath := localStateFixture(t)
	var asked []string
	opts.localState = func(_ context.Context, dir string, _ LocalStateOptions) (*LocalState, error) {
		asked = append(asked, dir)
		return &LocalState{Upstream: "origin/x", Ahead: 2, Modified: 1}, nil
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	row := topo.Changes[0]
	if row.Local == nil || row.Local.Ahead != 2 {
		t.Fatalf("local = %+v", row.Local)
	}
	if want := []string{"unpushed", "uncommitted"}; !reflect.DeepEqual(row.Attention, want) {
		t.Fatalf("attention = %v, want %v", row.Attention, want)
	}
	if !reflect.DeepEqual(asked, []string{worktreePath}) {
		t.Fatalf("collector asked for %v, want only the change's worktree", asked)
	}
}

func TestBuildTopology_LocalStateFailureNarrowsTheRow(t *testing.T) {
	opts, _ := localStateFixture(t)
	opts.localState = func(context.Context, string, LocalStateOptions) (*LocalState, error) {
		return nil, errors.New("not a repo")
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	row := topo.Changes[0]
	if row.Local != nil || row.Attention != nil {
		t.Fatalf("a failed collector must leave local and attention empty: %+v", row)
	}
	if row.Worktree == "" {
		t.Fatal("the row must keep its worktree")
	}
	found := false
	for _, s := range topo.Status {
		found = found || strings.Contains(s, "local state unavailable")
	}
	if !found {
		t.Fatalf("expected a status line about local state, got %v", topo.Status)
	}
}

func TestBuildTopology_NoWorktreeMeansNoLocalState(t *testing.T) {
	opts, _ := localStateFixture(t)
	opts.gitWorktreeList = fakeGitWorktreeList(map[string]string{filepath.Dir(opts.OpenSpecDir): ""}, nil)
	opts.localState = func(context.Context, string, LocalStateOptions) (*LocalState, error) {
		t.Fatal("collector must not run without a worktree")
		return nil, nil
	}
	if _, err := BuildTopology(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
}

func TestBuildTopology_AttachesSyncSuggestionFromLocalState(t *testing.T) {
	opts, _ := localStateFixture(t)
	opts.localState = func(context.Context, string, LocalStateOptions) (*LocalState, error) {
		return &LocalState{Upstream: "origin/x", Base: "origin/main", BehindBase: 2}, nil
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	got := topo.Changes[0].Sync
	if got == nil || got.Action != "rebase-onto-base" || !got.ForcePush {
		t.Fatalf("sync = %+v", got)
	}

	opts.Sync = SyncConfig{Strategy: "merge"}
	topo, _ = BuildTopology(context.Background(), opts)
	if got := topo.Changes[0].Sync; got == nil || got.Action != "merge-base" {
		t.Fatalf("configured strategy not used: %+v", got)
	}
}

func TestBuildTopology_UpToDateRowHasNoSyncSuggestion(t *testing.T) {
	opts, _ := localStateFixture(t)
	opts.localState = func(context.Context, string, LocalStateOptions) (*LocalState, error) {
		return &LocalState{Upstream: "origin/x", Base: "origin/main"}, nil
	}
	topo, _ := BuildTopology(context.Background(), opts)
	if topo.Changes[0].Sync != nil {
		t.Fatalf("expected no suggestion, got %+v", topo.Changes[0].Sync)
	}
}

func requestFixture(t *testing.T, calls *[]string) TopologyOptions {
	t.Helper()
	opts, _ := localStateFixture(t)
	opts.localState = func(context.Context, string, LocalStateOptions) (*LocalState, error) {
		return &LocalState{Upstream: "origin/x", Base: "origin/main", BehindBase: 1}, nil
	}
	opts.changeRequests = func(_ context.Context, repo string) (*ChangeRequestSet, error) {
		*calls = append(*calls, repo)
		return &ChangeRequestSet{Requests: []ChangeRequest{{
			Number: 7, State: "open", Branch: "feat/4231-add-field-forms",
			ReviewDecision: "approved", Mergeable: "mergeable", ReviewStarted: true,
			Checks: ChangeRequestChecks{Pass: 4},
		}}}, nil
	}
	return opts
}

func TestBuildTopology_AttachesRequestByBranch(t *testing.T) {
	var asked []string
	topo, err := BuildTopology(context.Background(), requestFixture(t, &asked))
	if err != nil {
		t.Fatal(err)
	}
	row := topo.Changes[0]
	if row.PR == nil || row.PR.Number != 7 || row.PR.Checks.Pass != 4 {
		t.Fatalf("pr = %+v", row.PR)
	}
	if !reflect.DeepEqual(asked, []string{"acme/portal"}) {
		t.Fatalf("expected one listing for the row's repo, got %v", asked)
	}
	if want := []string{"behind-base", "ready-to-merge"}; !reflect.DeepEqual(row.Attention, want) {
		t.Fatalf("attention = %v, want %v", row.Attention, want)
	}
	// review has started, so advice merges the base instead of rebasing
	if row.Sync == nil || row.Sync.Action != "merge-base" || row.Sync.ForcePush {
		t.Fatalf("sync = %+v", row.Sync)
	}
}

func TestBuildTopology_NoRequestForBranchOmitsPR(t *testing.T) {
	var asked []string
	opts := requestFixture(t, &asked)
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) {
		return &ChangeRequestSet{}, nil
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if topo.Changes[0].PR != nil {
		t.Fatalf("expected no pr, got %+v", topo.Changes[0].PR)
	}
	for _, s := range topo.Status {
		if strings.Contains(s, "request") {
			t.Fatalf("no PR is not an error, got status %q", s)
		}
	}
}

func TestBuildTopology_ProviderWithoutCapabilityIsSilent(t *testing.T) {
	var asked []string
	opts := requestFixture(t, &asked)
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) { return nil, nil }
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if topo.Changes[0].PR != nil || topo.Changes[0].Local == nil {
		t.Fatalf("rows carry local facts only: %+v", topo.Changes[0])
	}
	for _, s := range topo.Status {
		if strings.Contains(s, "request") {
			t.Fatalf("unexpected status %q", s)
		}
	}
}

func TestBuildTopology_RequestListingFailureNarrowsRows(t *testing.T) {
	var asked []string
	opts := requestFixture(t, &asked)
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) {
		return nil, errors.New("gh: not logged in")
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatalf("a failed provider must not fail the command: %v", err)
	}
	row := topo.Changes[0]
	if row.PR != nil || row.Local == nil || row.Worktree == "" {
		t.Fatalf("row should keep local facts: %+v", row)
	}
	found := false
	for _, s := range topo.Status {
		found = found || (strings.Contains(s, "request state omitted") && strings.Contains(s, "not logged in"))
	}
	if !found {
		t.Fatalf("expected a status line saying why, got %v", topo.Status)
	}
}

func TestBuildTopology_TruncatedListingIsReported(t *testing.T) {
	var asked []string
	opts := requestFixture(t, &asked)
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) {
		return &ChangeRequestSet{Truncated: true}, nil
	}
	topo, _ := BuildTopology(context.Background(), opts)
	found := false
	for _, s := range topo.Status {
		found = found || strings.Contains(s, "truncated")
	}
	if !found {
		t.Fatalf("expected a truncation status line, got %v", topo.Status)
	}
}

func TestBuildTopology_LocalOnlyStartsNoGHAndNoListing(t *testing.T) {
	var asked []string
	opts := requestFixture(t, &asked)
	opts.LocalOnly = true
	ghCalls := 0
	opts.run = func(context.Context, ...string) (string, error) { ghCalls++; return "", nil }
	cdir := filepath.Join(opts.OpenSpecDir, "changes", "add-field-forms")
	mustWrite(t, filepath.Join(cdir, ".specsync", "refs.json"),
		`{"github:acme/portal":{"provider":"github:acme/portal","id":"4231","url":"https://github.com/acme/portal/issues/4231"}}`)
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 0 || ghCalls != 0 {
		t.Fatalf("-state=off must not call the provider or gh: listings=%v gh=%d", asked, ghCalls)
	}
	row := topo.Changes[0]
	if row.Local == nil || row.PR != nil || len(row.Attention) == 0 {
		t.Fatalf("local facts and flags must remain: %+v", row)
	}
	if row.Issue == nil || row.Issue.State != "" {
		t.Fatalf("the issue is kept but its live state is not fetched: %+v", row.Issue)
	}
}

func TestBuildTopology_OneListingPerRepo(t *testing.T) {
	var asked []string
	opts := requestFixture(t, &asked)
	root := opts.OpenSpecDir
	porcelain := "worktree " + filepath.Dir(root) + "\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree " + filepath.Join(root, "..", "worktrees", "add-field-forms") + "\nHEAD d\nbranch refs/heads/feat/4231-add-field-forms\n"
	for i, slug := range []string{"second-change", "third-change"} {
		mustWrite(t, filepath.Join(root, "changes", slug, "proposal.md"), "# "+slug+"\n")
		porcelain += "\nworktree " + filepath.Join(root, "..", "worktrees", slug) + "\nHEAD e\nbranch refs/heads/feat/" + string(rune('5'+i)) + "-" + slug + "\n"
	}
	opts.gitWorktreeList = fakeGitWorktreeList(map[string]string{filepath.Dir(root): porcelain}, nil)
	if _, err := BuildTopology(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 {
		t.Fatalf("three changes with worktrees in one repo must share one listing, got %v", asked)
	}
}

func TestBuildTopology_RequestFailureStatusShowsTheReasonNotTheCommand(t *testing.T) {
	var asked []string
	opts := requestFixture(t, &asked)
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) {
		return nil, errors.New("gh pr list --repo acme/portal --state all --json a,b,c: exit status 1\nGraphQL: rate limited\n")
	}
	topo, _ := BuildTopology(context.Background(), opts)
	joined := strings.Join(topo.Status, "\n")
	if !strings.Contains(joined, "GraphQL: rate limited") || strings.Contains(joined, "--json") {
		t.Fatalf("status should carry the reason only, got %q", joined)
	}
}

func TestBuildTopology_PullRequestBaseBecomesTheBase(t *testing.T) {
	var asked []string
	opts := requestFixture(t, &asked)
	var gotBase string
	opts.localState = func(_ context.Context, _ string, lo LocalStateOptions) (*LocalState, error) {
		gotBase = lo.Base
		return &LocalState{Upstream: "origin/x", Base: lo.Base}, nil
	}
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) {
		return &ChangeRequestSet{Requests: []ChangeRequest{{
			Number: 7, State: "open", Branch: "feat/4231-add-field-forms", Base: "release/2",
		}}}, nil
	}
	if _, err := BuildTopology(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if gotBase != "origin/release/2" {
		t.Fatalf("local state base = %q, want the PR's base as a remote-tracking ref", gotBase)
	}
}

func TestBuildTopology_NoPullRequestLeavesBaseToTheCollector(t *testing.T) {
	var asked []string
	opts := requestFixture(t, &asked)
	var gotBase = "unset"
	opts.localState = func(_ context.Context, _ string, lo LocalStateOptions) (*LocalState, error) {
		gotBase = lo.Base
		return &LocalState{}, nil
	}
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) { return &ChangeRequestSet{}, nil }
	if _, err := BuildTopology(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if gotBase != "" {
		t.Fatalf("base = %q, want empty so origin/HEAD is used", gotBase)
	}
}

// scopedFixture has two changes with worktrees. add-field-forms has none of the
// tracker refs; other-change has one, so any gh call proves it was not skipped.
func scopedFixture(t *testing.T) (opts TopologyOptions, mine, theirs string, ghCalls *int) {
	t.Helper()
	opts, mine = localStateFixture(t)
	root := opts.OpenSpecDir
	mustWrite(t, filepath.Join(root, "changes", "add-field-forms", "tasks.md"), "- [x] one\n- [x] two\n- [ ] three\n")
	mustWrite(t, filepath.Join(root, "changes", "other-change", "proposal.md"), "# Other\n")
	mustWrite(t, filepath.Join(root, "changes", "other-change", ".specsync", "refs.json"),
		`{"github:acme/portal":{"provider":"github:acme/portal","id":"9","url":"https://github.com/acme/portal/issues/9"}}`)
	theirs = filepath.Join(root, "..", "worktrees", "other-change")
	porcelain := "worktree " + filepath.Dir(root) + "\nHEAD a\nbranch refs/heads/main\n\n" +
		"worktree " + mine + "\nHEAD b\nbranch refs/heads/feat/4231-add-field-forms\n\n" +
		"worktree " + theirs + "\nHEAD c\nbranch refs/heads/feat/9-other-change\n"
	opts.gitWorktreeList = fakeGitWorktreeList(map[string]string{filepath.Dir(root): porcelain}, nil)
	n := 0
	ghCalls = &n
	opts.run = func(context.Context, ...string) (string, error) { n++; return "", errors.New("gh offline") }
	opts.localState = func(context.Context, string, LocalStateOptions) (*LocalState, error) {
		return &LocalState{Upstream: "origin/x"}, nil
	}
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) { return &ChangeRequestSet{}, nil }
	return
}

func TestBuildTopology_DirScopesToTheCurrentWorktreeBeforeAnyLookup(t *testing.T) {
	opts, mine, _, ghCalls := scopedFixture(t)
	opts.Dir = filepath.Join(mine, "pkg")
	opts.gitCurrent = func(context.Context, string) (string, string, error) {
		return mine, "feat/4231-add-field-forms", nil
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(topo.Changes) != 1 || topo.Changes[0].Slug != "add-field-forms" || topo.Changes[0].NoChange {
		t.Fatalf("expected only the current change, got %+v", topo.Changes)
	}
	if *ghCalls != 0 {
		t.Fatalf("other changes must be skipped before their gh lookups, saw %d calls", *ghCalls)
	}
	if got := topo.Changes[0].Tasks; got == nil || got.Done != 2 || got.Total != 3 {
		t.Fatalf("tasks = %+v, want 2 of 3", got)
	}
}

func TestBuildTopology_BranchWithoutAChangeGetsAnOrphanRow(t *testing.T) {
	opts, _, _, _ := scopedFixture(t)
	elsewhere := t.TempDir()
	opts.Dir = elsewhere
	opts.gitCurrent = func(context.Context, string) (string, string, error) { return elsewhere, "scratch/idea", nil }
	opts.gitRemoteOrigin = fakeGitRemoteOrigin(map[string]string{elsewhere: "acme/portal", filepath.Dir(opts.OpenSpecDir): "acme/portal"})
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) {
		return &ChangeRequestSet{Requests: []ChangeRequest{{Number: 3, Branch: "scratch/idea", State: "open"}}}, nil
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(topo.Changes) != 1 {
		t.Fatalf("expected one row, got %+v", topo.Changes)
	}
	row := topo.Changes[0]
	if !row.NoChange || row.Slug != "" || row.Branch != "scratch/idea" || row.Worktree != elsewhere || row.Repo != "acme/portal" {
		t.Fatalf("orphan row = %+v", row)
	}
	if row.Local == nil || row.PR == nil || row.PR.Number != 3 {
		t.Fatalf("an orphan row still gets local state and its request: %+v", row)
	}
}

func TestBuildTopology_DirOutsideGitFallsBackToEverythingWithAStatusLine(t *testing.T) {
	opts, _, _, _ := scopedFixture(t)
	opts.Dir = t.TempDir()
	opts.gitCurrent = func(context.Context, string) (string, string, error) {
		return "", "", errors.New("not a git repository")
	}
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(topo.Changes) != 2 {
		t.Fatalf("expected every change, got %d", len(topo.Changes))
	}
	found := false
	for _, s := range topo.Status {
		found = found || strings.Contains(s, "not inside a git worktree")
	}
	if !found {
		t.Fatalf("expected a status line, got %v", topo.Status)
	}
}

func TestBuildTopology_ExplicitSlugBeatsDir(t *testing.T) {
	opts, mine, _, _ := scopedFixture(t)
	opts.Slug = "other-change"
	opts.Dir = mine
	opts.gitCurrent = func(context.Context, string) (string, string, error) { return mine, "x", nil }
	topo, _ := BuildTopology(context.Background(), opts)
	if len(topo.Changes) != 1 || topo.Changes[0].Slug != "other-change" {
		t.Fatalf("got %+v", topo.Changes)
	}
}

func TestBuildTopology_IdenticalFactsGiveByteIdenticalOutput(t *testing.T) {
	render := func() string {
		opts, _, _, _ := scopedFixture(t)
		// same fixture shape each time; paths differ per TempDir, so blank them
		topo, err := BuildTopology(context.Background(), opts)
		if err != nil {
			t.Fatal(err)
		}
		for i := range topo.Changes {
			topo.Changes[i].Store, topo.Changes[i].Worktree = "", ""
		}
		b, _ := json.Marshal(topo)
		return string(b)
	}
	first := render()
	for i := 0; i < 5; i++ {
		if got := render(); got != first {
			t.Fatalf("run %d differs:\n%s\n%s", i, first, got)
		}
	}
}

// TestChangeTopology_JSONShapeGolden pins the wire shape of a fully populated
// row so later changes to -json are additive: fields may be added, never
// renamed or removed. Regenerate with UPDATE_GOLDEN=1 after a deliberate change.
func TestChangeTopology_JSONShapeGolden(t *testing.T) {
	two := 2
	row := ChangeTopology{
		Slug: "add-thing", Title: "Add thing", Stage: "active", Store: "/repo", Repo: "acme/portal",
		Issue:  &TopologyIssue{Provider: "github:acme/portal", ID: "42", URL: "https://github.com/acme/portal/issues/42", State: "open"},
		Branch: "feat/42-add-thing", Worktree: "/work/add-thing", Epic: "https://github.com/acme/planning/issues/1", Linked: []string{"other"},
		Tasks: &TaskCount{Done: 2, Total: 5},
		Local: &LocalState{
			Head: "abc123", Modified: 1, Untracked: 2, Upstream: "origin/feat/42-add-thing", Ahead: 1, UpstreamBehind: 0,
			Base: "origin/main", BehindBase: 3, Size: &BranchSize{Files: 4, Additions: 50, Deletions: 6, Commits: 2},
			LastCommit: "2026-01-02T03:04:05Z", VolatilePath: true, BaseMovedAt: "2026-01-02T03:00:00Z",
			BaseTouchesYourFiles: []string{"a.go"}, Conflicts: []string{"a.go"}, ConflictCheckUnavailable: true, SharedBranch: true,
		},
		Attention: []string{"unpushed", "conflicts-with-base"},
		Sync:      &SyncAdvice{Action: "merge-base", Summary: "merge origin/main", Commands: []string{"git merge origin/main"}, ForcePush: false},
		PR: &ChangeRequest{
			Number: 7, URL: "https://github.com/acme/portal/pull/7", State: "open", Draft: true, Branch: "feat/42-add-thing", Base: "main",
			ReviewDecision: "approved", Mergeable: "mergeable",
			Checks:        ChangeRequestChecks{Pass: 3, Fail: 1, Pending: 0, Failing: []string{"unit"}},
			Size:          ChangeRequestSize{Files: 4, Additions: 50, Deletions: 6, Commits: 2},
			ReviewStarted: true, Closes: []string{"42"}, UnresolvedThreads: &two, HumanReviews: &two, HeadSHA: "abc123", Fork: true,
		},
	}
	got, err := json.MarshalIndent(Topology{Changes: []ChangeTopology{row}, Status: []string{"a status line"}, RequestState: "ok"}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "status-row.golden.json")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run with UPDATE_GOLDEN=1 to create it)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("status -json shape changed; additive changes only. If deliberate: UPDATE_GOLDEN=1 go test . -run JSONShapeGolden\n%s", got)
	}
}

func TestBuildTopology_RequestStateSaysWhetherRequestsWereLookedUp(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(*TopologyOptions)
		want  string
	}{
		{"looked up", func(o *TopologyOptions) {}, "ok"},
		{"provider failed", func(o *TopologyOptions) {
			o.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) { return nil, errors.New("down") }
		}, "omitted"},
		{"-state=off", func(o *TopologyOptions) { o.LocalOnly = true }, "off"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var asked []string
			opts := requestFixture(t, &asked)
			c.setup(&opts)
			topo, err := BuildTopology(context.Background(), opts)
			if err != nil {
				t.Fatal(err)
			}
			if topo.RequestState != c.want {
				t.Fatalf("requestState = %q, want %q", topo.RequestState, c.want)
			}
		})
	}
}
