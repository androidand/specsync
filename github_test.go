package specsync

import (
	"context"
	"testing"
)

func TestListOpenPRs(t *testing.T) {
	p := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		if args[0] == "pr" && args[1] == "list" {
			return `[{"number":123,"url":"https://github.com/o/r/pull/123","title":"feat: my feature","headRefName":"my-feature","body":"PR body"}]`, nil
		}
		return "", nil
	})

	prs, err := p.ListOpenPRs(context.Background())
	if err != nil {
		t.Fatalf("ListOpenPRs: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("want 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 123 {
		t.Errorf("number = %d, want 123", prs[0].Number)
	}
	if prs[0].URL != "https://github.com/o/r/pull/123" {
		t.Errorf("url = %q, want https://github.com/o/r/pull/123", prs[0].URL)
	}
	if prs[0].Merged {
		t.Errorf("open PR should not be merged")
	}
}

func TestListOpenPRsEmpty(t *testing.T) {
	p := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		return "[]", nil
	})

	prs, err := p.ListOpenPRs(context.Background())
	if err != nil {
		t.Fatalf("ListOpenPRs: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("want 0 PRs, got %d", len(prs))
	}
}

func TestListRecentMergedPRs(t *testing.T) {
	p := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		return `[{"number":100,"url":"https://github.com/o/r/pull/100","title":"fix: a bug","headRefName":"fix-bug","body":"PR body"}]`, nil
	})

	prs, err := p.ListRecentMergedPRs(context.Background())
	if err != nil {
		t.Fatalf("ListRecentMergedPRs: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("want 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 100 {
		t.Errorf("number = %d, want 100", prs[0].Number)
	}
	if !prs[0].Merged {
		t.Errorf("merged PR should have Merged = true")
	}
}

func TestListRecentMergedPRsEmpty(t *testing.T) {
	p := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		return "[]", nil
	})

	prs, err := p.ListRecentMergedPRs(context.Background())
	if err != nil {
		t.Fatalf("ListRecentMergedPRs: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("want 0 PRs, got %d", len(prs))
	}
}
