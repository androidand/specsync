package specsync

import (
	"context"
	"strings"
	"testing"
)

// TestReadDependencies verifies that ReadDependencies parses the GraphQL
// response into DependencyEdge values.
func TestReadDependencies(t *testing.T) {
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			return `{
				"data": {
					"repository": {
						"issue": {
							"issueDependenciesSummary": {
								"blockedBy": {
									"edges": [
										{
											"node": {
												"id": "I_kwDOA123",
												"databaseId": 5,
												"url": "https://github.com/owner/repo/issues/5"
											}
										}
									]
								},
								"blocking": {
									"edges": [
										{
											"node": {
												"id": "I_kwDOA456",
												"databaseId": 10,
												"url": "https://github.com/owner/repo/issues/10"
											}
										}
									]
								}
							}
						}
					}
				}
			}`, nil
		}
		return "", nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	edges, err := prov.ReadDependencies(context.Background(), Ref{
		Provider: "github:owner/repo",
		ID:       "42",
		URL:      "https://github.com/owner/repo/issues/42",
	})
	if err != nil {
		t.Fatalf("ReadDependencies: %v", err)
	}

	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}

	// BlockedBy edge.
	if edges[0].Ref.ID != "5" {
		t.Errorf("blockedBy edge ID = %q, want 5", edges[0].Ref.ID)
	}
	if edges[0].NodeID != "I_kwDOA123" {
		t.Errorf("blockedBy edge NodeID = %q, want I_kwDOA123", edges[0].NodeID)
	}
	if edges[0].IsBlocks {
		t.Error("blockedBy edge should not have IsBlocks set")
	}

	// Blocking edge (IsBlocks).
	if edges[1].Ref.ID != "10" {
		t.Errorf("blocking edge ID = %q, want 10", edges[1].Ref.ID)
	}
	if !edges[1].IsBlocks {
		t.Error("blocking edge should have IsBlocks set")
	}
}

// TestReadDependencies_Empty verifies that an issue with no dependencies
// returns an empty list.
func TestReadDependencies_Empty(t *testing.T) {
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			return `{
				"data": {
					"repository": {
						"issue": {
							"issueDependenciesSummary": {
								"blockedBy": {"edges": []},
								"blocking": {"edges": []}
							}
						}
					}
				}
			}`, nil
		}
		return "", nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	edges, err := prov.ReadDependencies(context.Background(), Ref{
		Provider: "github:owner/repo",
		ID:       "42",
		URL:      "https://github.com/owner/repo/issues/42",
	})
	if err != nil {
		t.Fatalf("ReadDependencies: %v", err)
	}

	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

// TestResolveNodeID verifies that ResolveNodeID parses the GraphQL response.
func TestResolveNodeID(t *testing.T) {
	fake := func(ctx context.Context, args ...string) (string, error) {
		return `{"data":{"repository":{"issue":{"id":"I_kwDOA999"}}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	nodeID, err := prov.ResolveNodeID(context.Background(), "https://github.com/owner/repo/issues/42")
	if err != nil {
		t.Fatalf("ResolveNodeID: %v", err)
	}
	if nodeID != "I_kwDOA999" {
		t.Errorf("ResolveNodeID = %q, want I_kwDOA999", nodeID)
	}
}

// TestAddBlockedBy verifies the mutation is called with correct arguments.
func TestAddBlockedBy(t *testing.T) {
	var capturedArgs []string
	fake := func(ctx context.Context, args ...string) (string, error) {
		capturedArgs = args
		return `{"data":{"addBlockedBy":{"clientMutationId":""}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	err := prov.AddBlockedBy(context.Background(), "42", "I_kwDOA123")
	if err != nil {
		t.Fatalf("AddBlockedBy: %v", err)
	}

	if !strings.Contains(strings.Join(capturedArgs, " "), "addBlockedBy") {
		t.Error("expected addBlockedBy mutation in args")
	}
	if !strings.Contains(strings.Join(capturedArgs, " "), "issueId=42") {
		t.Error("expected issueId=42 in args")
	}
	if !strings.Contains(strings.Join(capturedArgs, " "), "blockedById=I_kwDOA123") {
		t.Error("expected blockedById=I_kwDOA123 in args")
	}
}

// TestRemoveBlockedBy verifies the mutation is called with correct arguments.
func TestRemoveBlockedBy(t *testing.T) {
	var capturedArgs []string
	fake := func(ctx context.Context, args ...string) (string, error) {
		capturedArgs = args
		return `{"data":{"removeBlockedBy":{"clientMutationId":""}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	err := prov.RemoveBlockedBy(context.Background(), "42", "I_kwDOA123")
	if err != nil {
		t.Fatalf("RemoveBlockedBy: %v", err)
	}

	if !strings.Contains(strings.Join(capturedArgs, " "), "removeBlockedBy") {
		t.Error("expected removeBlockedBy mutation in args")
	}
}

// TestDepKey verifies the stable key generation for refs.
func TestDepKey(t *testing.T) {
	tests := []struct {
		ref    Ref
		want   string
	}{
		{
			ref:  Ref{Provider: "github:owner/repo", ID: "42", URL: "https://github.com/owner/repo/issues/42"},
			want: "owner/repo:42",
		},
		{
			ref:  Ref{Provider: "beads:board", ID: "abc123", URL: "https://beads.example/abc123"},
			want: "https://beads.example/abc123",
		},
	}

	for _, tt := range tests {
		got := depKey(tt.ref)
		if got != tt.want {
			t.Errorf("depKey(%v) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

// TestLoadSaveDepBaseline verifies the baseline cache round-trip.
func TestLoadSaveDepBaseline(t *testing.T) {
	dir := t.TempDir()
	provider := "github:owner/repo"

	// Load from non-existent file returns nil.
	bl, err := loadDepBaseline(dir, provider)
	if err != nil {
		t.Fatalf("loadDepBaseline (non-existent): %v", err)
	}
	if bl != nil {
		t.Error("expected nil baseline for missing file")
	}

	// Save a baseline.
	bl = &DependencyBaseline{
		BlockedBy: map[string]string{
			"owner/repo:5": "I_kwDOA123",
		},
		Blocks: map[string]string{
			"owner/repo:10": "I_kwDOA456",
		},
	}
	if err := saveDepBaseline(dir, provider, bl); err != nil {
		t.Fatalf("saveDepBaseline: %v", err)
	}

	// Load it back.
	loaded, err := loadDepBaseline(dir, provider)
	if err != nil {
		t.Fatalf("loadDepBaseline (existing): %v", err)
	}
	if len(loaded.BlockedBy) != 1 {
		t.Errorf("expected 1 blocked-by entry, got %d", len(loaded.BlockedBy))
	}
	if loaded.BlockedBy["owner/repo:5"] != "I_kwDOA123" {
		t.Errorf("blockedBy key = %q, want I_kwDOA123", loaded.BlockedBy["owner/repo:5"])
	}
	if len(loaded.Blocks) != 1 {
		t.Errorf("expected 1 blocks entry, got %d", len(loaded.Blocks))
	}
}

// TestDepSync_NoOp verifies that DepSync returns empty result when there's
// no GitHub provider (stdlib check).
func TestDepSync_NoOp(t *testing.T) {
	result, err := DepSync(context.Background(), DepSyncOptions{
		Ref: Ref{Provider: "beads:board", ID: "abc123"},
	})
	if err != nil {
		t.Fatalf("DepSync: %v", err)
	}
	if len(result.Added) != 0 || len(result.Removed) != 0 {
		t.Error("expected no-op for non-GitHub provider")
	}
}

// TestDepSync_AddBlockedBy verifies that DepSync adds edges that exist
// locally but not on GitHub.
func TestDepSync_AddBlockedBy(t *testing.T) {
	dir := t.TempDir()
	calledAdd := false
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			// No existing dependencies.
			return `{"data":{"repository":{"issue":{"issueDependenciesSummary":{"blockedBy":{"edges":[]},"blocking":{"edges":[]}}}}}}`, nil
		}
		if strings.Contains(strings.Join(args, " "), "addBlockedBy") {
			calledAdd = true
			return `{"data":{"addBlockedBy":{"clientMutationId":""}}}`, nil
		}
		return `{"data":{"repository":{"issue":{"id":"I_kwDOA999"}}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	result, err := DepSync(context.Background(), DepSyncOptions{
		ChangeDir: dir,
		Provider:  prov,
		Ref: Ref{
			Provider: "github:owner/repo",
			ID:       "42",
			URL:      "https://github.com/owner/repo/issues/42",
		},
		BlockedBy: []Ref{
			{
				Provider: "github:owner/repo",
				ID:       "5",
				URL:      "https://github.com/owner/repo/issues/5",
			},
		},
		Blocks: nil,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("DepSync: %v", err)
	}
	if !calledAdd {
		t.Error("expected addBlockedBy to be called")
	}
	if len(result.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(result.Added))
	}
}

// TestDepSync_AddBlocks verifies that DepSync adds inverse edges for Blocks.
func TestDepSync_AddBlocks(t *testing.T) {
	dir := t.TempDir()
	calledAdd := false
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			// No existing dependencies.
			return `{"data":{"repository":{"issue":{"issueDependenciesSummary":{"blockedBy":{"edges":[]},"blocking":{"edges":[]}}}}}}`, nil
		}
		if strings.Contains(strings.Join(args, " "), "addBlockedBy") {
			calledAdd = true
			return `{"data":{"addBlockedBy":{"clientMutationId":""}}}`, nil
		}
		return `{"data":{"repository":{"issue":{"id":"I_kwDOA999"}}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	result, err := DepSync(context.Background(), DepSyncOptions{
		ChangeDir: dir,
		Provider:  prov,
		Ref: Ref{
			Provider: "github:owner/repo",
			ID:       "42",
			URL:      "https://github.com/owner/repo/issues/42",
		},
		BlockedBy: nil,
		Blocks: []Ref{
			{
				Provider: "github:owner/repo",
				ID:       "10",
				URL:      "https://github.com/owner/repo/issues/10",
			},
		},
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("DepSync: %v", err)
	}
	if !calledAdd {
		t.Error("expected addBlockedBy to be called for inverse edge")
	}
	if len(result.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(result.Added))
	}
}

// TestDepSync_RemoveOnRemoval verifies that DepSync removes edges from GitHub
// when they are removed locally and were in the baseline.
func TestDepSync_RemoveOnRemoval(t *testing.T) {
	dir := t.TempDir()
	calledRemove := false
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			// The edge still exists on GitHub.
			return `{"data":{"repository":{"issue":{"issueDependenciesSummary":{"blockedBy":{"edges":[{"node":{"id":"I_kwDOA123","databaseId":5,"url":"https://github.com/owner/repo/issues/5"}}]},"blocking":{"edges":[]}}}}}}`, nil
		}
		if strings.Contains(strings.Join(args, " "), "removeBlockedBy") {
			calledRemove = true
			return `{"data":{"removeBlockedBy":{"clientMutationId":""}}}`, nil
		}
		return `{"data":{"repository":{"issue":{"id":"I_kwDOA999"}}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	// Save a baseline with the edge present.
	if err := saveDepBaseline(dir, "github:owner/repo", &DependencyBaseline{
		BlockedBy: map[string]string{
			"owner/repo:5": "I_kwDOA123",
		},
		Blocks: map[string]string{},
	}); err != nil {
		t.Fatalf("saveDepBaseline: %v", err)
	}

	result, err := DepSync(context.Background(), DepSyncOptions{
		ChangeDir: dir,
		Provider:  prov,
		Ref: Ref{
			Provider: "github:owner/repo",
			ID:       "42",
			URL:      "https://github.com/owner/repo/issues/42",
		},
		BlockedBy: nil, // Removed locally.
		Blocks:    nil,
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("DepSync: %v", err)
	}
	if !calledRemove {
		t.Error("expected removeBlockedBy to be called")
	}
	if len(result.Removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(result.Removed))
	}
}

// TestDepSync_DryRun verifies that DryRun prevents mutations.
func TestDepSync_DryRun(t *testing.T) {
	dir := t.TempDir()
	calledAdd := false
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			return `{"data":{"repository":{"issue":{"issueDependenciesSummary":{"blockedBy":{"edges":[]},"blocking":{"edges":[]}}}}}}`, nil
		}
		if strings.Contains(strings.Join(args, " "), "addBlockedBy") {
			calledAdd = true
			return `{"data":{"addBlockedBy":{"clientMutationId":""}}}`, nil
		}
		return `{"data":{"repository":{"issue":{"id":"I_kwDOA999"}}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	result, err := DepSync(context.Background(), DepSyncOptions{
		ChangeDir: dir,
		Provider:  prov,
		Ref: Ref{
			Provider: "github:owner/repo",
			ID:       "42",
			URL:      "https://github.com/owner/repo/issues/42",
		},
		BlockedBy: []Ref{
			{
				Provider: "github:owner/repo",
				ID:       "5",
				URL:      "https://github.com/owner/repo/issues/5",
			},
		},
		Blocks: nil,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("DepSync: %v", err)
	}
	if calledAdd {
		t.Error("expected no mutations in dry run")
	}
	if len(result.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(result.Added))
	}
}

// TestDepSync_CrossRepo verifies that DepSync can handle cross-repo
// dependencies.
func TestDepSync_CrossRepo(t *testing.T) {
	dir := t.TempDir()
	calledAdd := false
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			return `{"data":{"repository":{"issue":{"issueDependenciesSummary":{"blockedBy":{"edges":[]},"blocking":{"edges":[]}}}}}}`, nil
		}
		if strings.Contains(strings.Join(args, " "), "addBlockedBy") {
			calledAdd = true
			return `{"data":{"addBlockedBy":{"clientMutationId":""}}}`, nil
		}
		return `{"data":{"repository":{"issue":{"id":"I_kwDOA999"}}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	result, err := DepSync(context.Background(), DepSyncOptions{
		ChangeDir: dir,
		Provider:  prov,
		Ref: Ref{
			Provider: "github:owner/repo",
			ID:       "42",
			URL:      "https://github.com/owner/repo/issues/42",
		},
		BlockedBy: []Ref{
			{
				Provider: "github:other-org/other-repo",
				ID:       "7",
				URL:      "https://github.com/other-org/other-repo/issues/7",
			},
		},
		Blocks: nil,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("DepSync: %v", err)
	}
	if !calledAdd {
		t.Error("expected addBlockedBy to be called for cross-repo dependency")
	}
	if len(result.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(result.Added))
	}
}

// TestDepSync_UnmanagedEdgeGap verifies that DepSync doesn't remove edges
// that are present on GitHub but not in the local links.md file.
func TestDepSync_UnmanagedEdgeGap(t *testing.T) {
	dir := t.TempDir()
	calledRemove := false
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			return `{"data":{"repository":{"issue":{"issueDependenciesSummary":{"blockedBy":{"edges":[{"node":{"id":"I_kwDOA123","databaseId":5,"url":"https://github.com/owner/repo/issues/5"}}]},"blocking":{"edges":[]}}}}}}`, nil
		}
		if strings.Contains(strings.Join(args, " "), "removeBlockedBy") {
			calledRemove = true
			return `{"data":{"removeBlockedBy":{"clientMutationId":""}}}`, nil
		}
		return `{"data":{"repository":{"issue":{"id":"I_kwDOA999"}}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	result, err := DepSync(context.Background(), DepSyncOptions{
		ChangeDir: dir,
		Provider:  prov,
		Ref: Ref{
			Provider: "github:owner/repo",
			ID:       "42",
			URL:      "https://github.com/owner/repo/issues/42",
		},
		BlockedBy: nil,
		Blocks:    nil,
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("DepSync: %v", err)
	}
	if calledRemove {
		t.Error("expected no removeBlockedBy call for unmanaged edge")
	}
	if len(result.Removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(result.Removed))
	}
}

// TestDepSync_PullInRemote verifies that a dependency edge present on GitHub
// but not in the local links.md and not in the baseline is pulled into
// result.PulledIn.
func TestDepSync_PullInRemote(t *testing.T) {
	dir := t.TempDir()
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			return `{"data":{"repository":{"issue":{"issueDependenciesSummary":{"blockedBy":{"edges":[{"node":{"id":"I_kwDOA123","databaseId":99,"url":"https://github.com/owner/repo/issues/99"}}]},"blocking":{"edges":[]}}}}}}`, nil
		}
		return `{"data":{"repository":{"issue":{"id":"I_kwDOA999"}}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	result, err := DepSync(context.Background(), DepSyncOptions{
		ChangeDir: dir,
		Provider:  prov,
		Ref: Ref{
			Provider: "github:owner/repo",
			ID:       "42",
			URL:      "https://github.com/owner/repo/issues/42",
		},
		BlockedBy: nil,
		Blocks:    nil,
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("DepSync: %v", err)
	}
	if len(result.PulledIn) != 1 {
		t.Errorf("expected 1 pulled-in edge, got %d: %v", len(result.PulledIn), result.PulledIn)
	}
	if len(result.Added) != 0 {
		t.Errorf("expected 0 added, got %d", len(result.Added))
	}
	if len(result.Removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(result.Removed))
	}
}

// TestDepSync_CycleErrorSurfaced verifies that DepSync surfaces cycle
// errors from GitHub.
func TestDepSync_CycleErrorSurfaced(t *testing.T) {
	dir := t.TempDir()
	fake := func(ctx context.Context, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "issueDependenciesSummary") {
			return `{"data":{"repository":{"issue":{"issueDependenciesSummary":{"blockedBy":{"edges":[]},"blocking":{"edges":[]}}}}}}`, nil
		}
		if strings.Contains(strings.Join(args, " "), "addBlockedBy") {
			return `{"errors":[{"type":"INVALID_FIELD","path":["addBlockedBy"],"message":"Dependency cycle detected"}]}`, nil
		}
		return `{"data":{"repository":{"issue":{"id":"I_kwDOA999"}}}}`, nil
	}
	prov := NewGitHubProviderFuncWithRepo("owner/repo", fake)

	result, err := DepSync(context.Background(), DepSyncOptions{
		ChangeDir: dir,
		Provider:  prov,
		Ref: Ref{
			Provider: "github:owner/repo",
			ID:       "42",
			URL:      "https://github.com/owner/repo/issues/42",
		},
		BlockedBy: []Ref{
			{
				Provider: "github:owner/repo",
				ID:       "5",
				URL:      "https://github.com/owner/repo/issues/5",
			},
		},
		Blocks: nil,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("DepSync: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("expected cycle error to be in result.Errors")
	}
	foundCycle := false
	for _, e := range result.Errors {
		if strings.Contains(e.Error(), "Dependency cycle detected") {
			foundCycle = true
			break
		}
	}
	if !foundCycle {
		t.Errorf("expected cycle error in result.Errors, got: %v", result.Errors)
	}
}
