package specsync

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// LinkerResult holds a resolved Ref and the source that produced it
// (e.g., "branch", "cache"). Used by sync for dry-run visibility.
type LinkerResult struct {
	Ref    *Ref
	Source string // human-readable source label
}

// Linker resolves a change to its issue ref by consulting multiple sources
// in priority order. The first hit wins; the result is cached for the
// duration of the sync run.
type Linker interface {
	// Resolve returns the LinkerResult for the given change directory, or
	// (nil, nil) if no resolver found a match.
	Resolve(ctx context.Context, changeDir string) (*LinkerResult, error)
	// ResolveFromContext resolves without a change directory, using only
	// the context (e.g., current branch name). Returns (nil, nil) if not
	// supported by this linker.
	ResolveFromContext(ctx context.Context) (*LinkerResult, error)
}

// ChainLinker tries each resolver in order until one returns a non-nil Ref.
// The first hit wins; subsequent resolvers are not consulted.
type ChainLinker struct {
	resolvers []Resolver
}

// Resolver is a single link resolution strategy. Returns (nil, nil) if it
// cannot resolve (not a match), or (nil, err) if it encounters an error.
type Resolver interface {
	// Resolve returns the LinkerResult if this resolver can match, or
	// (nil, nil) to pass to the next resolver.
	Resolve(ctx context.Context, changeDir string) (*LinkerResult, error)
}

// NewChainLinker creates a ChainLinker with the given resolvers in priority
// order. The first resolver to return a non-nil Ref wins.
func NewChainLinker(resolvers ...Resolver) *ChainLinker {
	return &ChainLinker{resolvers: resolvers}
}

func (c *ChainLinker) Resolve(ctx context.Context, changeDir string) (*LinkerResult, error) {
	for _, r := range c.resolvers {
		result, err := r.Resolve(ctx, changeDir)
		if err != nil {
			return nil, fmt.Errorf("resolver: %w", err)
		}
		if result != nil && result.Ref != nil {
			return result, nil
		}
	}
	return nil, nil
}

func (c *ChainLinker) ResolveFromContext(ctx context.Context) (*LinkerResult, error) {
	for _, r := range c.resolvers {
		if cr, ok := r.(contextResolver); ok {
			result, err := cr.ResolveFromContext(ctx)
			if err != nil {
				return nil, fmt.Errorf("resolver: %w", err)
			}
			if result != nil && result.Ref != nil {
				return result, nil
			}
		}
	}
	return nil, nil
}

// contextResolver is a resolver that can resolve without a change directory.
type contextResolver interface {
	ResolveFromContext(ctx context.Context) (*LinkerResult, error)
}

// BranchResolver extracts an issue number from the current git branch name.
// The pattern is configurable; default: `feat/(\d+)-.*` or `fix/(\d+)-.*`.
// When given a changeDir, it verifies the slug matches the branch suffix
// to avoid resolving all changes to the same issue.
type BranchResolver struct {
	repo string // "owner/name" — required to build the URL
	pats []*regexp.Regexp
}

// NewBranchResolver creates a BranchResolver with the given repo and patterns.
// If repo is empty, the resolver cannot resolve (returns nil). Default patterns
// are `feat/(\d+)-.*` and `fix/(\d+)-.*`.
func NewBranchResolver(repo string, pats ...*regexp.Regexp) *BranchResolver {
	if len(pats) == 0 {
		pats = []*regexp.Regexp{
			regexp.MustCompile(`^(?:feat|fix)/(\d+)-(.+)$`),
		}
	}
	return &BranchResolver{repo: repo, pats: pats}
}

func (b *BranchResolver) Resolve(ctx context.Context, changeDir string) (*LinkerResult, error) {
	if b.repo == "" {
		return nil, nil
	}

	branch, err := currentBranch()
	if err != nil || branch == "" {
		return nil, nil
	}

	for _, pat := range b.pats {
		matches := pat.FindStringSubmatch(branch)
		if len(matches) < 3 {
			continue
		}
		num := matches[1]
		branchSuffix := matches[2]

		// When resolving for a specific change, verify the slug matches
		// the branch suffix. This prevents all changes from resolving to
		// the same issue when syncing multiple changes at once.
		if changeDir != "" {
			slug := filepath.Base(changeDir)
			if slug != branchSuffix {
				continue
			}
		}

		url := fmt.Sprintf("https://github.com/%s/issues/%s", b.repo, num)
		return &LinkerResult{
			Ref: &Ref{
				Provider: "github:" + b.repo,
				ID:       num,
				URL:      url,
			},
			Source: "branch",
		}, nil
	}

	return nil, nil
}

func (b *BranchResolver) ResolveFromContext(ctx context.Context) (*LinkerResult, error) {
	if b.repo == "" {
		return nil, nil
	}

	branch, err := currentBranch()
	if err != nil || branch == "" {
		return nil, nil
	}

	for _, pat := range b.pats {
		matches := pat.FindStringSubmatch(branch)
		if len(matches) < 2 {
			continue
		}
		num := matches[1]
		url := fmt.Sprintf("https://github.com/%s/issues/%s", b.repo, num)
		return &LinkerResult{
			Ref: &Ref{
				Provider: "github:" + b.repo,
				ID:       num,
				URL:      url,
			},
			Source: "branch",
		}, nil
	}

	return nil, nil
}

// CacheResolver reads the local .specsync/refs.json cache.
type CacheResolver struct {
	providerName string // e.g. "github:owner/repo" or "github"
}

// NewCacheResolver creates a CacheResolver for the given provider name.
func NewCacheResolver(providerName string) *CacheResolver {
	return &CacheResolver{providerName: providerName}
}

func (c *CacheResolver) Resolve(_ context.Context, changeDir string) (*LinkerResult, error) {
	refs, err := LoadRefs(changeDir)
	if err != nil {
		return nil, fmt.Errorf("cache read: %w", err)
	}

	// Try the exact provider key first.
	if ref, ok := refs[c.providerName]; ok {
		return &LinkerResult{Ref: &ref, Source: "cache"}, nil
	}

	// Try the legacy bare "github" key for github: prefixed providers.
	if strings.HasPrefix(c.providerName, "github:") {
		if ref, ok := refs["github"]; ok {
			return &LinkerResult{Ref: &ref, Source: "cache"}, nil
		}
	}

	return nil, nil
}

// currentBranchFn is the function that returns the current git branch name.
// It is a variable so it can be overridden in tests.
var currentBranchFn = func() (string, error) {
	out, err := runGit(context.Background(), "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// currentBranch returns the current git branch name, or "" if not on a branch.
func currentBranch() (string, error) {
	return currentBranchFn()
}
