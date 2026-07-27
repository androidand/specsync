package specsync

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestChainLinker_FirstHitWins(t *testing.T) {
	counts := [2]int{}
	r1 := &stubResolver{
		resolve: func(ctx context.Context, changeDir string) (*Ref, error) {
			counts[0]++
			return &Ref{Provider: "p1", ID: "1", URL: "http://p1/1"}, nil
		},
	}
	r2 := &stubResolver{
		resolve: func(ctx context.Context, changeDir string) (*Ref, error) {
			counts[1]++
			return &Ref{Provider: "p2", ID: "2", URL: "http://p2/2"}, nil
		},
	}

	l := NewChainLinker(r1, r2)
	ref, err := l.Resolve(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Provider != "p1" {
		t.Errorf("expected p1, got %s", ref.Provider)
	}
	if counts[1] != 0 {
		t.Errorf("r2 was called; expected only r1 to run")
	}
}

func TestChainLinker_PassThrough(t *testing.T) {
	counts := [2]int{}
	r1 := &stubResolver{
		resolve: func(ctx context.Context, changeDir string) (*Ref, error) {
			counts[0]++
			return nil, nil
		},
	}
	r2 := &stubResolver{
		resolve: func(ctx context.Context, changeDir string) (*Ref, error) {
			counts[1]++
			return &Ref{Provider: "p2", ID: "2", URL: "http://p2/2"}, nil
		},
	}

	l := NewChainLinker(r1, r2)
	ref, err := l.Resolve(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Provider != "p2" {
		t.Errorf("expected p2, got %s", ref.Provider)
	}
	if counts[0] != 1 || counts[1] != 1 {
		t.Errorf("expected both resolvers called, got counts=%v", counts)
	}
}

func TestChainLinker_NoHit(t *testing.T) {
	r1 := &stubResolver{resolve: func(ctx context.Context, changeDir string) (*Ref, error) {
		return nil, nil
	}}
	r2 := &stubResolver{resolve: func(ctx context.Context, changeDir string) (*Ref, error) {
		return nil, nil
	}}

	l := NewChainLinker(r1, r2)
	ref, err := l.Resolve(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != nil {
		t.Errorf("expected nil ref, got %v", ref)
	}
}

func TestChainLinker_ErrorPropagates(t *testing.T) {
	r1 := &stubResolver{
		resolve: func(ctx context.Context, changeDir string) (*Ref, error) {
			return nil, os.ErrPermission
		},
	}

	l := NewChainLinker(r1)
	ref, err := l.Resolve(context.Background(), "x")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ref != nil {
		t.Errorf("expected nil ref, got %v", ref)
	}
}

func TestBranchResolver_ExtractsFromBranch(t *testing.T) {
	ctx := context.Background()
	branchResolver := NewBranchResolver("androidand/specsync")

	// This test depends on the actual git branch. Skip if not on a matching branch.
	ref, err := branchResolver.Resolve(ctx, "some-change")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// If we're on a feat/N- branch, we should get a ref.
	if ref != nil {
		if ref.Provider != "github:androidand/specsync" {
			t.Errorf("unexpected provider: %s", ref.Provider)
		}
	}
}

func TestBranchResolver_EmptyRepoReturnsNil(t *testing.T) {
	r := NewBranchResolver("")
	ref, err := r.Resolve(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != nil {
		t.Errorf("expected nil, got %v", ref)
	}
}

func TestBranchResolver_CustomPattern(t *testing.T) {
	repo := "androidand/specsync"
	pat := regexp.MustCompile(`^custom/(\d+)-.*`)
	r := NewBranchResolver(repo, pat)

	// Won't match feat/123-foo branch, so returns nil.
	ref, err := r.Resolve(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Will be nil unless we happen to be on a custom/N- branch.
	if ref != nil && ref.Provider != "github:"+repo {
		t.Errorf("unexpected provider: %s", ref.Provider)
	}
}

func TestMarkerResolver_FindsMarker(t *testing.T) {
	dir := t.TempDir()
	// Create a minimal change directory with a proposal.md
	changeDir := filepath.Join(dir, "openspec", "changes", "test-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test Change\n\nBody\n"), 0644); err != nil {
		t.Fatal(err)
	}

	expected := &Ref{Provider: "test", ID: "42", URL: "http://test/42"}
	provider := &markerStubProvider{
		findFunc: func(ctx context.Context, slug string) (*Ref, error) {
			if slug != "test-change" {
				t.Errorf("expected slug test-change, got %s", slug)
			}
			return expected, nil
		},
	}

	r := NewMarkerResolver(provider)
	ref, err := r.Resolve(context.Background(), changeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref == nil {
		t.Fatal("expected ref, got nil")
	}
	if ref.Provider != expected.Provider || ref.ID != expected.ID {
		t.Errorf("expected %+v, got %+v", expected, ref)
	}
}

func TestMarkerResolver_NoMarker(t *testing.T) {
	dir := t.TempDir()
	changeDir := filepath.Join(dir, "openspec", "changes", "test-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	provider := &markerStubProvider{
		findFunc: func(ctx context.Context, slug string) (*Ref, error) {
			return nil, nil
		},
	}

	r := NewMarkerResolver(provider)
	ref, err := r.Resolve(context.Background(), changeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != nil {
		t.Errorf("expected nil, got %v", ref)
	}
}

func TestCacheResolver_FindsProviderKey(t *testing.T) {
	dir := t.TempDir()
	refsPath := filepath.Join(dir, ".specsync", "refs.json")
	if err := os.MkdirAll(filepath.Dir(refsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refsPath, []byte(`{"github:androidand/specsync":{"Provider":"github:androidand/specsync","ID":"55","URL":"http://x/55"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewCacheResolver("github:androidand/specsync")
	ref, err := r.Resolve(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref == nil {
		t.Fatal("expected ref, got nil")
	}
	if ref.ID != "55" {
		t.Errorf("expected ID 55, got %s", ref.ID)
	}
}

func TestCacheResolver_LegacyBareGithubKey(t *testing.T) {
	dir := t.TempDir()
	refsPath := filepath.Join(dir, ".specsync", "refs.json")
	if err := os.MkdirAll(filepath.Dir(refsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refsPath, []byte(`{"github":{"Provider":"github","ID":"10","URL":"http://x/10"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewCacheResolver("github:androidand/specsync")
	ref, err := r.Resolve(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref == nil {
		t.Fatal("expected ref, got nil")
	}
	if ref.ID != "10" {
		t.Errorf("expected ID 10, got %s", ref.ID)
	}
}

func TestCacheResolver_NoMatch(t *testing.T) {
	dir := t.TempDir()
	refsPath := filepath.Join(dir, ".specsync", "refs.json")
	if err := os.MkdirAll(filepath.Dir(refsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refsPath, []byte(`{"other":{"Provider":"other","ID":"1","URL":"http://x/1"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewCacheResolver("github:androidand/specsync")
	ref, err := r.Resolve(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != nil {
		t.Errorf("expected nil, got %v", ref)
	}
}

func TestExternalResolver(t *testing.T) {
	expected := &Ref{Provider: "ext", ID: "99", URL: "http://ext/99"}
	r := NewExternalResolver(func(ctx context.Context, changeDir string) (*Ref, error) {
		return expected, nil
	})

	ref, err := r.Resolve(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Provider != "ext" || ref.ID != "99" {
		t.Errorf("expected %+v, got %+v", expected, ref)
	}
}

func TestResolveOpenSpecDir(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openspec/changes/foo", "openspec"},
		{"openspec/archive/foo", "openspec"},
		{"/abs/openspec/changes/foo", "/abs/openspec"},
		{"/abs/openspec/archive/foo", "/abs/openspec"},
		{"foo", ""},
	}
	for _, tc := range tests {
		got := resolveOpenSpecDir(tc.input)
		if got != tc.expected {
			t.Errorf("resolveOpenSpecDir(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// stubResolver implements Resolver for tests.
type stubResolver struct {
	resolve func(ctx context.Context, changeDir string) (*Ref, error)
}

func (s *stubResolver) Resolve(ctx context.Context, changeDir string) (*Ref, error) {
	return s.resolve(ctx, changeDir)
}

// markerStubProvider implements WorkProvider for MarkerResolver tests.
type markerStubProvider struct {
	findFunc func(ctx context.Context, slug string) (*Ref, error)
}

func (s *markerStubProvider) Name() string { return "test" }
func (s *markerStubProvider) Find(ctx context.Context, slug string) (*Ref, error) { return s.findFunc(ctx, slug) }
func (s *markerStubProvider) Push(ctx context.Context, item WorkItem, ref *Ref) (Ref, error) { return Ref{}, nil }
