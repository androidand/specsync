package main

import (
	"testing"

	"github.com/androidand/specsync"
)

func TestSortChangesBySlug_Sorted(t *testing.T) {
	changes := []specsync.Change{
		{Slug: "alpha"},
		{Slug: "beta"},
		{Slug: "gamma"},
	}
	sortChangesBySlug(changes)
	if changes[0].Slug != "alpha" || changes[1].Slug != "beta" || changes[2].Slug != "gamma" {
		t.Errorf("expected alpha, beta, gamma; got %s, %s, %s", changes[0].Slug, changes[1].Slug, changes[2].Slug)
	}
}

func TestSortChangesBySlug_Unsorted(t *testing.T) {
	changes := []specsync.Change{
		{Slug: "gamma"},
		{Slug: "alpha"},
		{Slug: "beta"},
	}
	sortChangesBySlug(changes)
	if changes[0].Slug != "alpha" || changes[1].Slug != "beta" || changes[2].Slug != "gamma" {
		t.Errorf("expected alpha, beta, gamma; got %s, %s, %s", changes[0].Slug, changes[1].Slug, changes[2].Slug)
	}
}

func TestSortChangesBySlug_Descending(t *testing.T) {
	changes := []specsync.Change{
		{Slug: "zulu"},
		{Slug: "yankee"},
		{Slug: "xray"},
		{Slug: "whiskey"},
		{Slug: "victor"},
		{Slug: "tango"},
		{Slug: "sierra"},
		{Slug: "romeo"},
		{Slug: "papa"},
		{Slug: "oscar"},
		{Slug: "november"},
		{Slug: "mike"},
		{Slug: "lima"},
		{Slug: "india"},
		{Slug: "hotel"},
		{Slug: "golf"},
		{Slug: "foxtrot"},
		{Slug: "echo"},
		{Slug: "delta"},
		{Slug: "charlie"},
		{Slug: "bravo"},
		{Slug: "alpha"},
	}
	sortChangesBySlug(changes)
	expected := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "lima", "mike", "november", "oscar", "papa", "romeo", "sierra", "tango", "victor", "whiskey", "xray", "yankee", "zulu"}
	for i, slug := range expected {
		if changes[i].Slug != slug {
			t.Errorf("changes[%d]: expected %q, got %q", i, slug, changes[i].Slug)
		}
	}
}

func TestSortChangesBySlug_Single(t *testing.T) {
	changes := []specsync.Change{{Slug: "only"}}
	sortChangesBySlug(changes)
	if changes[0].Slug != "only" {
		t.Errorf("expected only; got %s", changes[0].Slug)
	}
}

func TestSortChangesBySlug_Empty(t *testing.T) {
	changes := []specsync.Change{}
	sortChangesBySlug(changes)
	if len(changes) != 0 {
		t.Errorf("expected empty; got %d items", len(changes))
	}
}

func TestSortChangesBySlug_Duplicates(t *testing.T) {
	changes := []specsync.Change{
		{Slug: "beta"},
		{Slug: "alpha"},
		{Slug: "beta"},
		{Slug: "alpha"},
	}
	sortChangesBySlug(changes)
	if changes[0].Slug != "alpha" || changes[1].Slug != "alpha" || changes[2].Slug != "beta" || changes[3].Slug != "beta" {
		t.Errorf("expected alpha, alpha, beta, beta; got %s, %s, %s, %s", changes[0].Slug, changes[1].Slug, changes[2].Slug, changes[3].Slug)
	}
}

func TestExtractIssueRefs_BareNumber(t *testing.T) {
	body := "This fixes #42 and closes #99"
	refs := extractIssueRefs(body)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d: %v", len(refs), refs)
	}
	if refs[0] != "#42" || refs[1] != "#99" {
		t.Errorf("expected #42, #99; got %s, %s", refs[0], refs[1])
	}
}

func TestExtractIssueRefs_CrossRepo(t *testing.T) {
	body := "Related to owner/repo#123 and other-org/repo#456"
	refs := extractIssueRefs(body)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d: %v", len(refs), refs)
	}
	if refs[0] != "owner/repo#123" || refs[1] != "other-org/repo#456" {
		t.Errorf("expected owner/repo#123, other-org/repo#456; got %s, %s", refs[0], refs[1])
	}
}

func TestExtractIssueRefs_Mixed(t *testing.T) {
	body := "Fixes #42 and related to owner/repo#123. Also #999"
	refs := extractIssueRefs(body)
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d: %v", len(refs), refs)
	}
	expected := []string{"#42", "owner/repo#123", "#999"}
	for i, want := range expected {
		if refs[i] != want {
			t.Errorf("refs[%d]: expected %q, got %q", i, want, refs[i])
		}
	}
}

func TestExtractIssueRefs_NoRefs(t *testing.T) {
	body := "This is just prose with no references"
	refs := extractIssueRefs(body)
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d: %v", len(refs), refs)
	}
}

func TestExtractIssueRefs_Dedup(t *testing.T) {
	body := "Fixes #42 and also #42 again"
	refs := extractIssueRefs(body)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref (deduped), got %d: %v", len(refs), refs)
	}
	if refs[0] != "#42" {
		t.Errorf("expected #42; got %s", refs[0])
	}
}

func TestExtractIssueRefs_NestedPath(t *testing.T) {
	// owner/repo/sub#42 should match as owner/repo/sub#42 (the parser is permissive).
	body := "Related to org/team/repo#42"
	refs := extractIssueRefs(body)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d: %v", len(refs), refs)
	}
	// The parser matches from the first '/' backward, so it captures "org/team/repo#42".
	if refs[0] != "org/team/repo#42" {
		t.Errorf("expected org/team/repo#42; got %s", refs[0])
	}
}

func TestExtractIssueRefs_IssueNumberInProse(t *testing.T) {
	// "#1" in prose should still be matched — the parser can't distinguish
	// intentional refs from incidental numbers without more context.
	body := "See issue #1 for details"
	refs := extractIssueRefs(body)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d: %v", len(refs), refs)
	}
	if refs[0] != "#1" {
		t.Errorf("expected #1; got %s", refs[0])
	}
}

func TestExtractSlugFromBranch_Feat(t *testing.T) {
	if got := extractSlugFromBranch("feat/42-my-change"); got != "my-change" {
		t.Errorf("expected my-change; got %s", got)
	}
}

func TestExtractSlugFromBranch_Fix(t *testing.T) {
	if got := extractSlugFromBranch("fix/99-add-auth"); got != "add-auth" {
		t.Errorf("expected add-auth; got %s", got)
	}
}

func TestExtractSlugFromBranch_Bugfix(t *testing.T) {
	if got := extractSlugFromBranch("bugfix/101-fix-crash"); got != "fix-crash" {
		t.Errorf("expected fix-crash; got %s", got)
	}
}

func TestExtractSlugFromBranch_Feature(t *testing.T) {
	if got := extractSlugFromBranch("feature/102-new-ui"); got != "new-ui" {
		t.Errorf("expected new-ui; got %s", got)
	}
}

func TestExtractSlugFromBranch_Hotfix(t *testing.T) {
	if got := extractSlugFromBranch("hotfix/103-security-patch"); got != "security-patch" {
		t.Errorf("expected security-patch; got %s", got)
	}
}

func TestExtractSlugFromBranch_NoPrefix(t *testing.T) {
	if got := extractSlugFromBranch("my-change"); got != "my-change" {
		t.Errorf("expected my-change; got %s", got)
	}
}

func TestExtractSlugFromBranch_JustNumber(t *testing.T) {
	if got := extractSlugFromBranch("123"); got != "123" {
		t.Errorf("expected 123; got %s", got)
	}
}

func TestExtractSlugFromBranch_NumberWithHyphen(t *testing.T) {
	if got := extractSlugFromBranch("123-my-slug"); got != "my-slug" {
		t.Errorf("expected my-slug; got %s", got)
	}
}
