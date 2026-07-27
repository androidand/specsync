package specsync

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Linker resolves a change to its issue ref by consulting multiple sources
// in priority order. The first hit wins; the result is cached for the
// duration of the sync run.
type Linker interface {
	// Resolve returns the Ref for the given change directory. Returns nil if
	// no resolver found a match.
	Resolve(ctx context.Context, changeDir string) (*Ref, error)
}

// ChainLinker tries each resolver in order until one returns a non-nil Ref.
// The first hit wins; subsequent resolvers are not consulted.
type ChainLinker struct {
	resolvers []Resolver
}

// Resolver is a single link resolution strategy. Returns (nil, nil) if it
// cannot resolve (not a match), or (nil, err) if it encounters an error.
type Resolver interface {
	// Resolve returns the Ref if this resolver can match, or (nil, nil) to
	// pass to the next resolver.
	Resolve(ctx context.Context, changeDir string) (*Ref, error)
}

// NewChainLinker creates a ChainLinker with the given resolvers in priority
// order. The first resolver to return a non-nil Ref wins.
func NewChainLinker(resolvers ...Resolver) *ChainLinker {
	return &ChainLinker{resolvers: resolvers}
}

func (c *ChainLinker) Resolve(ctx context.Context, changeDir string) (*Ref, error) {
	for _, r := range c.resolvers {
		ref, err := r.Resolve(ctx, changeDir)
		if err != nil {
			return nil, fmt.Errorf("resolver: %w", err)
		}
		if ref != nil {
			return ref, nil
		}
	}
	return nil, nil
}

// BranchResolver extracts an issue number from the current git branch name.
// The pattern is configurable; default: `feat/(\d+)-.*` or `fix/(\d+)-.*`.
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
			regexp.MustCompile(`^(?:feat|fix)/(\d+)-.*`),
		}
	}
	return &BranchResolver{repo: repo, pats: pats}
}

func (b *BranchResolver) Resolve(_ context.Context, changeDir string) (*Ref, error) {
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
		return &Ref{
			Provider: "github:" + b.repo,
			ID:       num,
			URL:      url,
		}, nil
	}

	return nil, nil
}

// MarkerResolver resolves via the <!-- specsync:change=<slug> --> marker in
// an existing issue body. It uses the provider's Find method.
type MarkerResolver struct {
	provider WorkProvider
}

// NewMarkerResolver creates a MarkerResolver that uses the given provider's
// Find method to search for the specsync marker.
func NewMarkerResolver(provider WorkProvider) *MarkerResolver {
	return &MarkerResolver{provider: provider}
}

func (m *MarkerResolver) Resolve(ctx context.Context, changeDir string) (*Ref, error) {
	openspecDir := resolveOpenSpecDir(changeDir)
	if openspecDir == "" {
		return nil, nil
	}
	c, err := LoadChangeBySlug(openspecDir, filepath.Base(changeDir))
	if err != nil {
		return nil, nil
	}
	ref, err := m.provider.Find(ctx, c.Slug)
	if err != nil {
		return nil, fmt.Errorf("marker find: %w", err)
	}
	return ref, nil
}

// CacheResolver reads the local .specsync/refs.json cache.
type CacheResolver struct {
	providerName string // e.g. "github:owner/repo" or "github"
}

// NewCacheResolver creates a CacheResolver for the given provider name.
func NewCacheResolver(providerName string) *CacheResolver {
	return &CacheResolver{providerName: providerName}
}

func (c *CacheResolver) Resolve(_ context.Context, changeDir string) (*Ref, error) {
	refs, err := loadRefs(changeDir)
	if err != nil {
		return nil, fmt.Errorf("cache read: %w", err)
	}

	// Try the exact provider key first.
	if ref, ok := refs[c.providerName]; ok {
		return &ref, nil
	}

	// Try the legacy bare "github" key for github: prefixed providers.
	if strings.HasPrefix(c.providerName, "github:") {
		if ref, ok := refs["github"]; ok {
			return &ref, nil
		}
	}

	return nil, nil
}

// ExternalResolver is a configurable hook for external relation sources
// (e.g. MCP, database, or custom logic).
type ExternalResolver struct {
	fn func(ctx context.Context, changeDir string) (*Ref, error)
}

// NewExternalResolver creates an ExternalResolver with the given function.
func NewExternalResolver(fn func(ctx context.Context, changeDir string) (*Ref, error)) *ExternalResolver {
	return &ExternalResolver{fn: fn}
}

func (e *ExternalResolver) Resolve(ctx context.Context, changeDir string) (*Ref, error) {
	return e.fn(ctx, changeDir)
}

// currentBranch returns the current git branch name, or "" if not on a branch.
func currentBranch() (string, error) {
	out, err := runGit(context.Background(), "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// resolveOpenSpecDir returns the openspec/ directory that contains the given
// change directory. It walks up from changeDir looking for a parent named
// "changes" or "archive", then returns that parent.
func resolveOpenSpecDir(changeDir string) string {
	dir := changeDir
	for {
		parent := filepath.Dir(dir)
		base := filepath.Base(dir)
		if base == "changes" || base == "archive" {
			return parent
		}
		dir = parent
		if dir == "/" || dir == "" || dir == "." {
			break
		}
	}
	return ""
}
