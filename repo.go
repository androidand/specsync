package specsync

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// RepoRule names which branch of the resolution order produced the result.
type RepoRule string

const (
	RuleExplicit RepoRule = "explicit"     // -repo flag
	RuleDefault  RepoRule = "gh-set-default" // gh repo set-default
	RuleOrigin   RepoRule = "origin"       // git remote origin
)

// ResolvedRepo holds the resolved repository and which rule selected it.
type ResolvedRepo struct {
	Repo string   // "owner/name"
	Rule RepoRule // which branch produced the result
}

// RepoResolver resolves the target repository explicitly.
type RepoResolver struct {
	Explicit string // -repo flag, empty if not set
	run      func(ctx context.Context, args ...string) (string, error)
}

// NewRepoResolver returns a resolver that shells out to git/gh.
func NewRepoResolver(explicit string) *RepoResolver {
	return &RepoResolver{Explicit: explicit, run: runGH}
}

// NewRepoResolverFunc returns a resolver driven by the given runner for testing.
func NewRepoResolverFunc(explicit string, run func(ctx context.Context, args ...string) (string, error)) *RepoResolver {
	return &RepoResolver{Explicit: explicit, run: run}
}

// Resolve returns the target repo and which rule selected it.
// Order: explicit flag → gh-set-default → origin.
// Returns an error only if no resolution path succeeds.
func (r *RepoResolver) Resolve(ctx context.Context) (ResolvedRepo, error) {
	if r.Explicit != "" {
		return ResolvedRepo{Repo: r.Explicit, Rule: RuleExplicit}, nil
	}

	// Try gh repo set-default.
	if out, err := r.run(ctx, "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"); err == nil {
		repo := strings.TrimSpace(out)
		if repo != "" {
			return ResolvedRepo{Repo: repo, Rule: RuleDefault}, nil
		}
	}

	// Fall back to origin remote.
	repo, err := getOriginRepo()
	if err != nil {
		return ResolvedRepo{}, fmt.Errorf("resolve repo: no explicit -repo, no gh default, and cannot read git origin: %w", err)
	}
	if repo == "" {
		return ResolvedRepo{}, fmt.Errorf("resolve repo: no explicit -repo, no gh default, and origin remote is empty")
	}
	return ResolvedRepo{Repo: repo, Rule: RuleOrigin}, nil
}

// GetOriginRepo returns the "owner/name" from the git origin remote.
func getOriginRepo() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin: %w\n%s", err, out)
	}
	return parseGitRemoteURL(strings.TrimSpace(string(out))), nil
}

// GetUpstreamRepo returns the "owner/name" from the git upstream remote, or
// empty string if no upstream remote exists.
func GetUpstreamRepo() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "upstream").CombinedOutput()
	if err != nil {
		return "", nil // no upstream = no error, just absent
	}
	return parseGitRemoteURL(strings.TrimSpace(string(out))), nil
}

// IsForkDivergence returns true when origin and upstream exist and name
// different repositories. Always reads the actual origin remote, not the
// resolved repo (which may come from gh-set-default and be a different repo).
func IsForkDivergence(ctx context.Context, resolved ResolvedRepo) (bool, string, error) {
	if resolved.Rule == RuleExplicit {
		return false, "", nil // explicit override, no divergence concern
	}
	origin, err := getOriginRepo()
	if err != nil {
		return false, "", err
	}
	if origin == "" {
		return false, "", nil
	}
	upstream, err := GetUpstreamRepo()
	if err != nil {
		return false, "", err
	}
	if upstream == "" {
		return false, "", nil // no upstream = no divergence
	}
	if origin != upstream {
		return true, upstream, nil
	}
	return false, "", nil
}

// ForkRefusal returns (shouldRefuse, reason) when the resolved repo targets
// the fork parent (upstream) without explicit user consent. Returns false when
// the user explicitly named the repo via -repo, or when there's no fork.
func ForkRefusal(ctx context.Context, resolved ResolvedRepo) (bool, string, error) {
	if resolved.Rule == RuleExplicit {
		return false, "", nil
	}
	origin, err := getOriginRepo()
	if err != nil {
		return false, "", err
	}
	if origin == "" {
		return false, "", nil
	}
	upstream, err := GetUpstreamRepo()
	if err != nil {
		return false, "", err
	}
	if upstream == "" {
		return false, "", nil
	}
	// Fork exists. Refuse if targeting the parent without explicit override.
	if resolved.Repo == upstream && origin != upstream {
		return true, fmt.Sprintf(
			"fork divergence: origin=%s, upstream=%s — targeting upstream (parent) without -repo; override with -repo %s",
			origin, upstream, upstream), nil
	}
	return false, "", nil
}

// parseGitRemoteURL extracts "owner/name" from a git remote URL.
func parseGitRemoteURL(url string) string {
	if strings.HasPrefix(url, "git@") {
		if i := strings.Index(url, ":"); i >= 0 {
			return strings.TrimSuffix(url[i+1:], ".git")
		}
	}
	if strings.HasPrefix(url, "https://") {
		url = strings.TrimPrefix(url, "https://")
	} else if strings.HasPrefix(url, "http://") {
		url = strings.TrimPrefix(url, "http://")
	}
	url = strings.TrimSuffix(url, ".git")
	if i := strings.Index(url, "/"); i >= 0 {
		return url[i+1:]
	}
	return url
}
