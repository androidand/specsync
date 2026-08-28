package specsync

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadChangeBySlugFindsDatedArchiveFolder: `openspec archive` renames a
// change's folder with a date prefix; LoadChangeBySlug must still find it by
// the original (pre-archive) slug.
func TestLoadChangeBySlugFindsDatedArchiveFolder(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "changes", "archive", "2026-08-10-fix-thing")
	mustWrite(t, filepath.Join(dir, "proposal.md"), "# Fix thing\n")

	c, err := LoadChangeBySlug(root, "fix-thing")
	if err != nil {
		t.Fatalf("LoadChangeBySlug: %v", err)
	}
	if c.Slug != "fix-thing" {
		t.Errorf("Slug = %q, want fix-thing", c.Slug)
	}
	if !c.Archived {
		t.Errorf("expected Archived = true")
	}
	if c.Dir != dir {
		t.Errorf("Dir = %q, want %q (real, date-prefixed path)", c.Dir, dir)
	}
}

// TestArchivedChangeSlugStripsDatePrefix: an archived change's Slug is the
// original slug, not the date-prefixed folder name — LoadChanges must
// enumerate it under the identity it had while active.
func TestArchivedChangeSlugStripsDatePrefix(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "changes", "archive", "2026-08-10-fix-thing")
	mustWrite(t, filepath.Join(dir, "proposal.md"), "# Fix thing\n")

	changes, err := LoadChanges(root)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("want 1 change, got %d", len(changes))
	}
	if changes[0].Slug != "fix-thing" {
		t.Errorf("Slug = %q, want fix-thing (date prefix stripped)", changes[0].Slug)
	}
}

// TestArchivedChangeSlugNoPrefixUnaffected: a change archived without a date
// prefix (this repo's older, pre-openspec-archive convention) keeps its slug
// unchanged.
func TestArchivedChangeSlugNoPrefixUnaffected(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "changes", "archive", "fix-thing")
	mustWrite(t, filepath.Join(dir, "proposal.md"), "# Fix thing\n")

	c, err := LoadChange(dir, true, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}
	if c.Slug != "fix-thing" {
		t.Errorf("Slug = %q, want fix-thing", c.Slug)
	}
}

// TestLoadChangeBySlugAmbiguousArchiveMatch: two archived folders both
// ending in "-<slug>" must error naming the candidates, not silently pick
// one.
func TestLoadChangeBySlugAmbiguousArchiveMatch(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "archive", "2026-08-10-fix-thing", "proposal.md"), "# A\n")
	mustWrite(t, filepath.Join(root, "changes", "archive", "2026-09-01-fix-thing", "proposal.md"), "# B\n")

	_, err := LoadChangeBySlug(root, "fix-thing")
	if err == nil {
		t.Fatal("expected an ambiguous-match error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error = %v, want it to mention ambiguity", err)
	}
	if !strings.Contains(err.Error(), "2026-08-10-fix-thing") || !strings.Contains(err.Error(), "2026-09-01-fix-thing") {
		t.Errorf("error should name both candidates: %v", err)
	}
}

// TestLoadChangeBySlugNotFound: a slug matching nothing anywhere still
// reports the same "no change found" error as before.
func TestLoadChangeBySlugNotFound(t *testing.T) {
	root := t.TempDir()
	_, err := LoadChangeBySlug(root, "does-not-exist")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "no change found") {
		t.Errorf("error = %v, want \"no change found\"", err)
	}
}

// TestLinksMDResolvesAfterSiblingArchived: a links.md slug entry naming a
// sibling change must still resolve once that sibling is archived under a
// date-prefixed folder.
func TestLinksMDResolvesAfterSiblingArchived(t *testing.T) {
	root := t.TempDir()
	siblingDir := filepath.Join(root, "changes", "archive", "2026-08-10-sibling-change")
	mustWrite(t, filepath.Join(siblingDir, "proposal.md"), "# Sibling\n")
	mustWrite(t, filepath.Join(siblingDir, ".specsync", "refs.json"),
		`{"github:o/r":{"provider":"github:o/r","id":"5","url":"https://github.com/o/r/issues/5"}}`)

	changeDir := filepath.Join(root, "changes", "this-change")
	mustWrite(t, filepath.Join(changeDir, "proposal.md"), "# This change\n")
	mustWrite(t, filepath.Join(changeDir, "links.md"), "## Related\n\n- sibling-change\n")

	links, _, _ := parseLinksMD(changeDir, root)
	if len(links) != 1 {
		t.Fatalf("want 1 resolved link, got %d: %v", len(links), links)
	}
	if links[0].ID != "5" {
		t.Errorf("link ID = %q, want 5", links[0].ID)
	}
}

// TestFindResolvesArchivedChangeByPreArchiveMarker is the regression test for
// the sync-design-notes incident (#139/#141): with no local ref cache,
// Find must still resolve an archived change's issue by the marker written
// under its pre-archive slug, not the date-prefixed folder name.
func TestFindResolvesArchivedChangeByPreArchiveMarker(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "changes", "archive", "2026-08-27-sync-design-notes")
	mustWrite(t, filepath.Join(dir, "proposal.md"), "# Sync design.md into the issue body\n")

	changes, err := LoadChanges(root)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("want 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.Slug != "sync-design-notes" {
		t.Fatalf("Slug = %q, want sync-design-notes (date prefix stripped)", c.Slug)
	}

	// The issue's marker was written pre-archive, under the original slug.
	issueBody := marker(c.Slug) + "\n\n# Sync design.md into the issue body\n"
	p := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "list" {
			return `[{"number":139,"url":"https://github.com/o/r/issues/139","body":` +
				quoteJSON(issueBody) + `}]`, nil
		}
		return "", nil
	})

	ref, err := p.Find(context.Background(), c.Slug)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ref == nil {
		t.Fatal("Find returned nil; archived change's issue was not resolved by its pre-archive marker")
	}
	if ref.ID != "139" {
		t.Errorf("ref.ID = %q, want 139", ref.ID)
	}
}

// TestResolveLiveRefsFindsArchivedChangeWithNoCache pins the same regression
// at the ResolveLiveRefs level (what changelog -resolve-refs and a
// cache-less sync.yml run both use), confirming a fresh checkout with no
// .specsync/ anywhere still attributes the archived change to its issue
// instead of leaving it unresolved (which is what caused the empty
// changelog and the duplicate issue in the incident this pins).
func TestResolveLiveRefsFindsArchivedChangeWithNoCache(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "changes", "archive", "2026-08-27-sync-design-notes")
	mustWrite(t, filepath.Join(dir, "proposal.md"), "# Sync design.md into the issue body\n")

	in, err := GatherTrace(context.Background(), root, nil, Scope{})
	if err != nil {
		t.Fatalf("GatherTrace: %v", err)
	}
	if len(in.Changes) != 1 || len(in.Changes[0].IssueIDs) != 0 {
		t.Fatalf("expected 1 change with no cached issue ids yet, got %+v", in.Changes)
	}

	issueBody := marker("sync-design-notes") + "\n\n# Sync design.md into the issue body\n"
	p := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "list" {
			return `[{"number":139,"url":"https://github.com/o/r/issues/139","body":` +
				quoteJSON(issueBody) + `}]`, nil
		}
		return "", nil
	})

	if err := ResolveLiveRefs(context.Background(), &in, p); err != nil {
		t.Fatalf("ResolveLiveRefs: %v", err)
	}
	if len(in.Changes[0].IssueIDs) != 1 || in.Changes[0].IssueIDs[0] != "139" {
		t.Fatalf("IssueIDs = %v, want [139]", in.Changes[0].IssueIDs)
	}
}

func quoteJSON(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
