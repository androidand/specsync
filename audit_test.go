package specsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchPRToChange(t *testing.T) {
	tests := []struct {
		name string
		pr   PRState
		slug string
		want bool
	}{
		{
			name: "exact branch name",
			pr:   PRState{HeadRefName: "my-feature"},
			slug: "my-feature",
			want: true,
		},
		{
			name: "branch with prefix",
			pr:   PRState{HeadRefName: "skein/my-feature"},
			slug: "my-feature",
			want: true,
		},
		{
			name: "branch with different prefix",
			pr:   PRState{HeadRefName: "feature/my-feature"},
			slug: "my-feature",
			want: true,
		},
		{
			name: "specsync marker in body",
			pr:   PRState{Body: "some text <!-- specsync:change=my-feature --> more text"},
			slug: "my-feature",
			want: true,
		},
		{
			name: "PR title contains slug",
			pr:   PRState{Title: "feat: add my-feature capability"},
			slug: "my-feature",
			want: true,
		},
		{
			name: "no match",
			pr:   PRState{HeadRefName: "other-feature", Title: "fix: something else", Body: "no marker here"},
			slug: "my-feature",
			want: false,
		},
		{
			name: "branch name is prefix of slug (no match)",
			pr:   PRState{HeadRefName: "my-feat"},
			slug: "my-feature",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchPRToChange(tc.pr, tc.slug)
			if got != tc.want {
				t.Errorf("matchPRToChange(%v, %q) = %v, want %v", tc.pr, tc.slug, got, tc.want)
			}
		})
	}
}

func TestAudit(t *testing.T) {
	p := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		if args[0] == "pr" && args[1] == "list" {
			for i, a := range args {
				if a == "--state" && i+1 < len(args) {
					if args[i+1] == "open" {
						return `[{"number":123,"url":"https://github.com/o/r/pull/123","title":"feat: add my-feature","headRefName":"my-feature","body":"PR body"}]`, nil
					}
					if args[i+1] == "merged" {
						return `[{"number":100,"url":"https://github.com/o/r/pull/100","title":"fix: a bug","headRefName":"fix-bug","body":"PR body"}]`, nil
					}
				}
			}
		}
		return "", nil
	})

	changes := []Change{
		{Slug: "my-feature", Archived: true, Stage: StageArchived},
		{Slug: "fix-bug", Archived: true, Stage: StageArchived},
		{Slug: "orphaned-change", Archived: true, Stage: StageArchived},
		{Slug: "active-change", Archived: false, Stage: StageActive},
	}

	result := Audit(context.Background(), p, changes)

	if len(result.Findings) != 3 {
		t.Fatalf("want 3 findings, got %d", len(result.Findings))
	}

	byslug := map[string]AuditFinding{}
	for _, f := range result.Findings {
		byslug[f.Slug] = f
	}

	if byslug["my-feature"].Status != "unmerged" {
		t.Errorf("my-feature status = %q, want unmerged", byslug["my-feature"].Status)
	}
	if byslug["fix-bug"].Status != "shipped" {
		t.Errorf("fix-bug status = %q, want shipped", byslug["fix-bug"].Status)
	}
	if byslug["orphaned-change"].Status != "orphaned" {
		t.Errorf("orphaned-change status = %q, want orphaned", byslug["orphaned-change"].Status)
	}

	if !result.HasUnmerged() {
		t.Errorf("HasUnmerged() = false, want true")
	}
}

func TestAuditResultHasUnmerged(t *testing.T) {
	result := AuditResult{
		Findings: []AuditFinding{
			{Slug: "a", Status: "shipped"},
			{Slug: "b", Status: "orphaned"},
		},
	}
	if result.HasUnmerged() {
		t.Errorf("HasUnmerged() = true, want false")
	}

	result.Findings = append(result.Findings, AuditFinding{Slug: "c", Status: "unmerged"})
	if !result.HasUnmerged() {
		t.Errorf("HasUnmerged() = false, want true")
	}
}

func TestShippedStageIsCanonical(t *testing.T) {
	if !IsCanonicalStage(StageShipped) {
		t.Errorf("StageShipped should be canonical")
	}
}

func TestShippedStageValidates(t *testing.T) {
	if err := ValidateStage(StageShipped); err != nil {
		t.Errorf("ValidateStage(StageShipped) = %v, want nil", err)
	}
}

func TestShippedStageAfterArchived(t *testing.T) {
	order := CanonicalStageOrder()
	archivedIdx := -1
	shippedIdx := -1
	for i, s := range order {
		if s == StageArchived {
			archivedIdx = i
		}
		if s == StageShipped {
			shippedIdx = i
		}
	}
	if archivedIdx == -1 || shippedIdx == -1 {
		t.Fatalf("StageArchived or StageShipped not found in CanonicalStageOrder")
	}
	if shippedIdx <= archivedIdx {
		t.Errorf("StageShipped should come after StageArchived: archived=%d, shipped=%d", archivedIdx, shippedIdx)
	}
}

func TestMarkShippedMetadata(t *testing.T) {
	root := t.TempDir()
	changesDir := filepath.Join(root, "archive", "my-feature")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "proposal.md"), []byte("# My Feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mark as shipped
	stg := StageShipped
	if err := SaveChangeMetadata(changesDir, ChangeMetadata{
		Version: 1,
		Stage:   &stg,
	}); err != nil {
		t.Fatalf("SaveChangeMetadata: %v", err)
	}

	// Verify
	meta, err := LoadChangeMetadata(changesDir)
	if err != nil {
		t.Fatalf("LoadChangeMetadata: %v", err)
	}
	if meta == nil {
		t.Fatal("metadata is nil")
	}
	if meta.Stage == nil || *meta.Stage != StageShipped {
		t.Errorf("stage = %v, want shipped", meta.Stage)
	}
}
