package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DependencyBaseline is the last-synced set of dependency edges for one
// change on one provider. It is stored in .specsync/deps.json so the
// reconciliation is two-way: local changes and remote-only changes are both
// detected relative to this merge base.
type DependencyBaseline struct {
	// BlockedByIDKey maps "owner/repo:N" to the issue's GitHub node id
	// (e.g. "I_kwDOA...") that was in the BlockedBy edge at last sync.
	BlockedBy map[string]string `json:"blockedBy,omitempty"`

	// BlockByIDKey maps "owner/repo:N" to the issue's GitHub node id
	// that was in the Blocks edge at last sync.
	Blocks map[string]string `json:"blocks,omitempty"`
}

// depKey returns a stable key for a Ref: "owner/repo:N" for GitHub refs,
// or the full URL for others.
func depKey(r Ref) string {
	if strings.HasPrefix(r.Provider, "github:") {
		repo := strings.TrimPrefix(r.Provider, "github:")
		return repo + ":" + r.ID
	}
	return r.URL
}

// depCacheDir is the .specsync directory under a change folder.
func depCacheDir(changeDir string) string {
	return filepath.Join(changeDir, ".specsync")
}

// depBaselinePath is the file path for the dependency baseline cache.
func depBaselinePath(changeDir string, provider string) string {
	return filepath.Join(depCacheDir(changeDir), "deps", provider+".json")
}

// loadDepBaseline reads the last-synced dependency baseline for a provider.
// Returns nil when the file is absent (first sync).
func loadDepBaseline(changeDir, provider string) (*DependencyBaseline, error) {
	data, err := os.ReadFile(depBaselinePath(changeDir, provider))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read deps baseline: %w", err)
	}
	var bl DependencyBaseline
	if err := json.Unmarshal(data, &bl); err != nil {
		return nil, fmt.Errorf("parse deps baseline: %w", err)
	}
	return &bl, nil
}

// saveDepBaseline writes the dependency baseline for a provider.
func saveDepBaseline(changeDir, provider string, bl *DependencyBaseline) error {
	if bl == nil {
		return nil
	}
	dir := filepath.Dir(depBaselinePath(changeDir, provider))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create deps dir: %w", err)
	}
	data, err := json.MarshalIndent(bl, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal deps baseline: %w", err)
	}
	tmp := depBaselinePath(changeDir, provider) + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write deps baseline: %w", err)
	}
	if err := os.Rename(tmp, depBaselinePath(changeDir, provider)); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename deps baseline: %w", err)
	}
	return nil
}

// DependencyEdge is one dependency edge returned by ReadDependencies.
type DependencyEdge struct {
	Ref      Ref
	NodeID   string // GitHub node id (e.g. "I_kwDOA...")
	IsBlocks bool   // true when this is a "blocks" edge (our issue blocks the target)
}

// depEdge represents one dependency edge as seen on GitHub.
type depEdge struct {
	Key     string // "owner/repo:N"
	NodeID  string // GitHub node id (e.g. "I_kwDOA...")
	IsLocal bool   // true when this ref is in the local change's links.md
}

// DepSyncPlan describes what dependency changes need to happen.
type DepSyncPlan struct {
	// AddBlockedBy lists edges that exist locally but not on GitHub.
	AddBlockedBy []depEdge

	// RemoveBlockedBy lists edges that exist on GitHub but not locally
	// and were present in the last baseline (meaning we removed them locally).
	RemoveBlockedBy []depEdge

	// AddBlocks lists edges in the Blocks section that need to be set as
	// blockedBy on the target issue (inverse edge).
	AddBlocks []depEdge

	// RemoveBlocks lists inverse edges to remove from target issues.
	RemoveBlocks []depEdge

	// RemoteBlockedBy lists edges that exist only on GitHub and were not
	// in the last baseline — someone set them on GitHub. They should be
	// pulled into links.md.
	RemoteBlockedBy []depEdge

	// RemoteBlocks lists edges that exist only on GitHub via inverse edges
	// (our issue appears in another issue's blockedBy). They should be
	// pulled into the Blocks section.
	RemoteBlocks []depEdge
}

// DepSyncOptions configures dependency synchronization.
type DepSyncOptions struct {
	ChangeDir  string
	Provider   *GitHubProvider
	Ref        Ref         // the ref for this change's own issue
	BlockedBy  []Ref       // from links.md ## Blocked by
	Blocks     []Ref       // from links.md ## Blocks
	DryRun     bool
}

// DepSyncResult reports what a dependency sync did.
type DepSyncResult struct {
	Added    []string
	Removed  []string
	PulledIn []string // remote-only edges pulled into links.md
	Errors   []error
}

// DepSync reconciles local dependency edges (from links.md) with GitHub's
// dependency edges. It uses the baseline in .specsync/deps/ to detect remote
// changes that happened outside the local tool, and local changes that need
// to be pushed. Returns the result and the updated baseline.
func DepSync(ctx context.Context, opts DepSyncOptions) (*DepSyncResult, error) {
	// Only GitHub supports dependency edges.
	if !strings.HasPrefix(opts.Ref.Provider, "github:") {
		return &DepSyncResult{}, nil
	}

	// Read current GitHub dependencies for this issue.
	remoteBlockedBy, err := opts.Provider.ReadDependencies(ctx, opts.Ref)
	if err != nil {
		return nil, fmt.Errorf("read dependencies: %w", err)
	}

	// Load baseline.
	baseline, err := loadDepBaseline(opts.ChangeDir, opts.Ref.Provider)
	if err != nil {
		return nil, err
	}
	if baseline == nil {
		baseline = &DependencyBaseline{
			BlockedBy: map[string]string{},
			Blocks:    map[string]string{},
		}
	}

	// Build local sets.
	localBlockedBy := make(map[string]Ref)
	for _, r := range opts.BlockedBy {
		k := depKey(r)
		localBlockedBy[k] = r
	}
	localBlocks := make(map[string]Ref)
	for _, r := range opts.Blocks {
		k := depKey(r)
		localBlocks[k] = r
	}

	// Build remote sets (keyed by depKey).
	remoteBlockedByMap := make(map[string]string)
	for _, e := range remoteBlockedBy {
		k := depKey(e.Ref)
		remoteBlockedByMap[k] = e.NodeID
	}
	remoteBlocksMap := make(map[string]string)
	for _, e := range remoteBlockedBy {
		if e.IsBlocks {
			k := depKey(e.Ref)
			remoteBlocksMap[k] = e.NodeID
		}
	}

	result := &DepSyncResult{}

	// --- BlockedBy reconciliation ---

	// Local edges not on GitHub → add.
	for k, r := range localBlockedBy {
		if _, onRemote := remoteBlockedByMap[k]; !onRemote {
			nodeID, err := opts.Provider.ResolveNodeID(ctx, r.URL)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("resolve node id for %s: %w", k, err))
				continue
			}
			result.Added = append(result.Added, k)
			if !opts.DryRun {
				if err := opts.Provider.AddBlockedBy(ctx, opts.Ref.ID, nodeID); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("addBlockedBy %s: %w", k, err))
				}
			}
		}
	}

	// Remote edges not locally → check baseline.
	for k, nodeID := range remoteBlockedByMap {
		if _, local := localBlockedBy[k]; !local {
			if _, inBaseline := baseline.BlockedBy[k]; inBaseline {
				// Was in baseline but removed locally → remove from GitHub.
				result.Removed = append(result.Removed, k)
				if !opts.DryRun {
					if err := opts.Provider.RemoveBlockedBy(ctx, opts.Ref.ID, nodeID); err != nil {
						result.Errors = append(result.Errors, fmt.Errorf("removeBlockedBy %s: %w", k, err))
					}
				}
			} else {
				// Not in baseline → added on GitHub, pull into links.md.
				result.PulledIn = append(result.PulledIn, k)
			}
		}
	}

	// --- Blocks reconciliation (inverse edge) ---
	// "## Blocks" means: the named issue is blocked by us.
	// We check: does the named issue have our issue in its blockedBy?
	for k, r := range localBlocks {
		// Resolve the target issue's dependencies.
		targetDeps, err := opts.Provider.ReadDependenciesForRef(ctx, r)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("read deps for %s: %w", k, err))
			continue
		}
		// Check if our issue appears in the target's blockedBy.
		ourKey := depKey(opts.Ref)
		hasEdge := false
		for _, e := range targetDeps {
			if depKey(e.Ref) == ourKey {
				hasEdge = true
				break
			}
		}
		if !hasEdge {
			// Need to set the inverse edge: add our issue as blockedBy on target.
			ourNodeID, err := opts.Provider.ResolveNodeID(ctx, opts.Ref.URL)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("resolve our node id for Blocks: %w", err))
				continue
			}
			// We need the target issue's issue number.
			result.Added = append(result.Added, "(blocks)"+k)
			if !opts.DryRun {
				if err := opts.Provider.AddBlockedBy(ctx, r.ID, ourNodeID); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("addBlockedBy (inverse) %s: %w", k, err))
				}
			}
		}
	}

	// Update baseline to the current state (what exists locally now, which is
	// the converged set after add/remove). Store NodeID from remote map so
	// future syncs can use it for mutations without re-querying.
	newBaseline := &DependencyBaseline{
		BlockedBy: make(map[string]string),
		Blocks:    make(map[string]string),
	}
	for k := range localBlockedBy {
		newBaseline.BlockedBy[k] = remoteBlockedByMap[k]
	}
	for k := range localBlocks {
		newBaseline.Blocks[k] = remoteBlocksMap[k]
	}
	if !opts.DryRun {
		if err := saveDepBaseline(opts.ChangeDir, opts.Ref.Provider, newBaseline); err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	return result, nil
}
