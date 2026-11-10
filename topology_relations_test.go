package specsync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func relationsFixture(t *testing.T) (TopologyOptions, string) {
	t.Helper()
	repo := t.TempDir()
	root := filepath.Join(repo, "openspec")
	cdir := filepath.Join(root, "changes", "scope-list")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Scope the list\n")
	mustWrite(t, filepath.Join(cdir, "links.md"), "## Blocked by\n- acme/web#2\n\n## Blocks\n- acme/docs#3\n")
	odd := filepath.Join(t.TempDir(), "elsewhere")
	porcelain := "worktree " + repo + "\nHEAD a\nbranch refs/heads/main\n\n" +
		"worktree " + odd + "\nHEAD b\nbranch refs/heads/spike-picker\n\n" +
		"worktree " + filepath.Join(t.TempDir(), "stray") + "\nHEAD c\nbranch refs/heads/experiment\n"
	return TopologyOptions{
		OpenSpecDir:      root,
		readCoordination: func(context.Context) (*Coordination, error) { return nil, nil },
		gitWorktreeList:  fakeGitWorktreeList(map[string]string{repo: porcelain}, nil),
		gitRemoteOrigin:  fakeGitRemoteOrigin(map[string]string{repo: "acme/api"}),
		loadRefs: func(string) (map[string]Ref, error) {
			return map[string]Ref{"github:acme/api": {ID: "12", URL: "https://github.com/acme/api/issues/12"}}, nil
		},
		run:        func(context.Context, ...string) (string, error) { return "open", nil },
		localState: func(context.Context, string, LocalStateOptions) (*LocalState, error) { return &LocalState{Upstream: "origin/x"}, nil },
		changeRequests: func(context.Context, string) (*ChangeRequestSet, error) {
			return &ChangeRequestSet{Requests: []ChangeRequest{
				{Number: 40, State: "open", Branch: "spike-picker", Closes: []string{"12"}},
				{Number: 41, State: "open", Branch: "remote-only"},
			}}, nil
		},
	}, odd
}

func TestBuildTopology_ResolvesBranchThroughTheClosingRequest(t *testing.T) {
	opts, odd := relationsFixture(t)
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	row := topo.Changes[0]
	if row.Slug != "scope-list" || row.Branch != "spike-picker" || row.Worktree != odd {
		t.Fatalf("a branch name unrelated to the change must resolve through the request that closes the issue: %+v", row)
	}
	if row.PR == nil || row.PR.Number != 40 {
		t.Fatalf("pr = %+v", row.PR)
	}
}

func TestBuildTopology_CarriesBlockedByAndBlocks(t *testing.T) {
	opts, _ := relationsFixture(t)
	topo, _ := BuildTopology(context.Background(), opts)
	row := topo.Changes[0]
	if len(row.BlockedBy) != 1 || row.BlockedBy[0] != "acme/web#2" || len(row.Blocks) != 1 || row.Blocks[0] != "acme/docs#3" {
		t.Fatalf("blockedBy=%v blocks=%v", row.BlockedBy, row.Blocks)
	}
}

func TestBuildTopology_UnlinkedRowsForWorktreesAndOpenRequests(t *testing.T) {
	opts, _ := relationsFixture(t)
	opts.IncludeUnlinked = true
	opts.NoCoordination = true
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	var branches []string
	for _, r := range topo.Changes {
		if r.NoChange {
			branches = append(branches, r.Branch)
		}
	}
	if len(branches) != 2 || branches[0] != "experiment" || branches[1] != "remote-only" {
		t.Fatalf("want the stray worktree and the request-only branch as unlinked rows, got %v", branches)
	}
	for _, r := range topo.Changes {
		if r.NoChange && r.Branch == "remote-only" && (r.PR == nil || r.Worktree != "") {
			t.Fatalf("request-only row must carry its request and no worktree: %+v", r)
		}
	}
}

func TestBuildTopology_StoreWithoutOpenspecStillListsUnlinkedWork(t *testing.T) {
	opts, _ := relationsFixture(t)
	opts.OpenSpecDir = filepath.Join(filepath.Dir(opts.OpenSpecDir), "missing", "openspec")
	opts.gitWorktreeList = func(context.Context, string) (string, error) {
		return "worktree " + filepath.Dir(opts.OpenSpecDir) + "\nHEAD a\nbranch refs/heads/feat/x\n", nil
	}
	opts.gitRemoteOrigin = func(context.Context, string) (string, error) { return "acme/api", nil }
	opts.IncludeUnlinked = true
	opts.NoCoordination = true
	opts.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) { return nil, errors.New("offline") }
	topo, err := BuildTopology(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range topo.Status {
		if len(s) > 0 && s[:5] == "could" {
			t.Fatalf("a store with no openspec/ must not warn: %v", topo.Status)
		}
	}
}
