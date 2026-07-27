package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestBranchResolver_ExtractsFromBranch(t *testing.T) {
	br := NewBranchResolver("androidand/specsync")

	result, err := br.Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil && result.Ref != nil {
		if result.Ref.Provider != "github:androidand/specsync" {
			t.Errorf("unexpected provider: %s", result.Ref.Provider)
		}
	}
}

func TestBranchResolver_EmptyRepoReturnsNil(t *testing.T) {
	r := NewBranchResolver("")
	ref, err := r.Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != nil {
		t.Errorf("expected nil, got %v", ref)
	}
}

func TestBranchResolver_CustomPattern(t *testing.T) {
	repo := "androidand/specsync"
	pat := regexp.MustCompile(`^custom/(\d+)-(.+)$`)
	r := NewBranchResolver(repo, pat)

	result, err := r.Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil && result.Ref != nil && result.Ref.Provider != "github:"+repo {
		t.Errorf("unexpected provider: %s", result.Ref.Provider)
	}
}

func TestBranchResolver_SlugAware_Match(t *testing.T) {
	// Simulate being on feat/42-my-change branch; resolving for "my-change" should match.
	prev := currentBranchFn
	currentBranchFn = func() (string, error) { return "feat/42-my-change", nil }
	defer func() { currentBranchFn = prev }()

	br := NewBranchResolver("owner/repo")
	result, err := br.Resolve(context.Background(), "openspec/changes/my-change")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Ref == nil {
		t.Fatal("expected ref, got nil")
	}
	if result.Ref.ID != "42" {
		t.Errorf("expected ID 42, got %s", result.Ref.ID)
	}
	if result.Ref.Provider != "github:owner/repo" {
		t.Errorf("unexpected provider: %s", result.Ref.Provider)
	}
	if result.Ref.URL != "https://github.com/owner/repo/issues/42" {
		t.Errorf("unexpected URL: %s", result.Ref.URL)
	}
	if result.Source != "branch" {
		t.Errorf("expected source branch, got %s", result.Source)
	}
}

func TestBranchResolver_SlugAware_NoMatch(t *testing.T) {
	// On feat/42-other branch, resolving for "my-change" should NOT match.
	prev := currentBranchFn
	currentBranchFn = func() (string, error) { return "feat/42-other", nil }
	defer func() { currentBranchFn = prev }()

	br := NewBranchResolver("owner/repo")
	ref, err := br.Resolve(context.Background(), "openspec/changes/my-change")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != nil {
		t.Errorf("expected nil, got %+v", ref)
	}
}

func TestBranchResolver_SlugAware_EmptyChangeDir(t *testing.T) {
	// With empty changeDir, any matching branch should resolve (no slug check).
	prev := currentBranchFn
	currentBranchFn = func() (string, error) { return "feat/42-something", nil }
	defer func() { currentBranchFn = prev }()

	br := NewBranchResolver("owner/repo")
	result, err := br.Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Ref == nil {
		t.Fatal("expected ref, got nil")
	}
	if result.Ref.ID != "42" {
		t.Errorf("expected ID 42, got %s", result.Ref.ID)
	}
}

func TestBranchResolver_FixBranch(t *testing.T) {
	prev := currentBranchFn
	currentBranchFn = func() (string, error) { return "fix/99-bugfix", nil }
	defer func() { currentBranchFn = prev }()

	br := NewBranchResolver("owner/repo")
	result, err := br.Resolve(context.Background(), "openspec/changes/bugfix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Ref == nil {
		t.Fatal("expected ref, got nil")
	}
	if result.Ref.ID != "99" {
		t.Errorf("expected ID 99, got %s", result.Ref.ID)
	}
}

func TestBranchResolver_NotOnBranch(t *testing.T) {
	prev := currentBranchFn
	currentBranchFn = func() (string, error) { return "HEAD", nil } // detached HEAD
	defer func() { currentBranchFn = prev }()

	br := NewBranchResolver("owner/repo")
	ref, err := br.Resolve(context.Background(), "openspec/changes/something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != nil {
		t.Errorf("expected nil on detached HEAD, got %+v", ref)
	}
}

func TestBranchResolver_ResolveFromContext(t *testing.T) {
	prev := currentBranchFn
	currentBranchFn = func() (string, error) { return "feat/55-feature", nil }
	defer func() { currentBranchFn = prev }()

	br := NewBranchResolver("androidand/specsync")
	result, err := br.ResolveFromContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Ref == nil {
		t.Fatal("expected ref, got nil")
	}
	if result.Ref.ID != "55" {
		t.Errorf("expected ID 55, got %s", result.Ref.ID)
	}
	if result.Ref.Provider != "github:androidand/specsync" {
		t.Errorf("unexpected provider: %s", result.Ref.Provider)
	}
	if result.Source != "branch" {
		t.Errorf("expected source branch, got %s", result.Source)
	}
}

func TestChainLinker_FirstHitWins(t *testing.T) {
	hit := 0
	r1 := &mockResolver{resultFn: func() (*LinkerResult, error) {
		hit++
		return &LinkerResult{Ref: &Ref{ID: "42"}, Source: "test"}, nil
	}}
	r2 := &mockResolver{resultFn: func() (*LinkerResult, error) {
		t.Fatal("r2 should not be called")
		return nil, nil
	}}

	chain := NewChainLinker(r1, r2)
	result, err := chain.Resolve(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Ref == nil || result.Ref.ID != "42" {
		t.Errorf("expected ref 42, got %v", result)
	}
	if hit != 1 {
		t.Errorf("expected 1 hit, got %d", hit)
	}
}

func TestChainLinker_SkipsNil(t *testing.T) {
	r1 := &mockResolver{resultFn: func() (*LinkerResult, error) {
		return nil, nil
	}}
	r2 := &mockResolver{resultFn: func() (*LinkerResult, error) {
		return &LinkerResult{Ref: &Ref{ID: "99"}, Source: "test"}, nil
	}}

	chain := NewChainLinker(r1, r2)
	result, err := chain.Resolve(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Ref == nil || result.Ref.ID != "99" {
		t.Errorf("expected ref 99, got %v", result)
	}
}

func TestChainLinker_AllNil(t *testing.T) {
	r1 := &mockResolver{resultFn: func() (*LinkerResult, error) {
		return nil, nil
	}}
	r2 := &mockResolver{resultFn: func() (*LinkerResult, error) {
		return nil, nil
	}}

	chain := NewChainLinker(r1, r2)
	result, err := chain.Resolve(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestChainLinker_ErrorPropagates(t *testing.T) {
	r1 := &mockResolver{resultFn: func() (*LinkerResult, error) {
		return nil, nil
	}}
	r2 := &mockResolver{resultFn: func() (*LinkerResult, error) {
		return nil, errBoom
	}}

	chain := NewChainLinker(r1, r2)
	_, err := chain.Resolve(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestChainLinker_ResolveFromContext(t *testing.T) {
	r1 := &mockContextResolver{resultFn: func() (*LinkerResult, error) {
		return nil, nil
	}}
	r2 := &mockContextResolver{resultFn: func() (*LinkerResult, error) {
		return &LinkerResult{Ref: &Ref{ID: "77"}, Source: "ctx"}, nil
	}}

	chain := NewChainLinker(r1, r2)
	result, err := chain.ResolveFromContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Ref == nil || result.Ref.ID != "77" {
		t.Errorf("expected ref 77, got %v", result)
	}
}

func TestCacheResolver_FindsRef(t *testing.T) {
	dir := t.TempDir()
	refsPath := filepath.Join(dir, ".specsync", "refs.json")
	os.MkdirAll(filepath.Dir(refsPath), 0755)
	os.WriteFile(refsPath, []byte(`{"github:owner/repo":{"id":"42","url":"https://github.com/owner/repo/issues/42"}}`), 0644)

	cr := NewCacheResolver("github:owner/repo")
	result, err := cr.Resolve(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Ref == nil {
		t.Fatal("expected ref, got nil")
	}
	if result.Ref.ID != "42" {
		t.Errorf("expected ID 42, got %s", result.Ref.ID)
	}
	if result.Source != "cache" {
		t.Errorf("expected source cache, got %s", result.Source)
	}
}

func TestCacheResolver_LegacyKey(t *testing.T) {
	dir := t.TempDir()
	refsPath := filepath.Join(dir, ".specsync", "refs.json")
	os.MkdirAll(filepath.Dir(refsPath), 0755)
	os.WriteFile(refsPath, []byte(`{"github":{"id":"42","url":"https://github.com/owner/repo/issues/42"}}`), 0644)

	cr := NewCacheResolver("github:owner/repo")
	result, err := cr.Resolve(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Ref == nil {
		t.Fatal("expected ref, got nil")
	}
	if result.Ref.ID != "42" {
		t.Errorf("expected ID 42, got %s", result.Ref.ID)
	}
}

func TestCacheResolver_NoRef(t *testing.T) {
	dir := t.TempDir()
	cr := NewCacheResolver("github:owner/repo")
	result, err := cr.Resolve(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// mockResolver implements Resolver.
type mockResolver struct {
	resultFn func() (*LinkerResult, error)
}

func (m *mockResolver) Resolve(ctx context.Context, changeDir string) (*LinkerResult, error) {
	return m.resultFn()
}

// mockContextResolver implements Resolver + contextResolver.
type mockContextResolver struct {
	resultFn func() (*LinkerResult, error)
}

func (m *mockContextResolver) Resolve(ctx context.Context, changeDir string) (*LinkerResult, error) {
	return m.resultFn()
}

func (m *mockContextResolver) ResolveFromContext(ctx context.Context) (*LinkerResult, error) {
	return m.resultFn()
}

var errBoom = fmt.Errorf("boom")
