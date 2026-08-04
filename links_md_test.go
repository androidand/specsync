package specsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// authoredLinksMD is the shape that made the old truncating write a data-loss
// bug: prose and dependency order a human wrote, mixed with link entries.
const authoredLinksMD = `# Links

Sequencing: the migration must land before the API cutover, otherwise the
dashboard reads a schema that isn't there yet.

## Blocked by

- owner/repo#10

## Related

- owner/repo#11
`

// TestSaveLinksToMD_PreservesAuthoredContent is the regression: recording a new
// ref must append, never rewrite. The old implementation replaced the whole file
// with a bare list, destroying the prose, the dependency sections, and the
// sequencing notes.
func TestSaveLinksToMD_PreservesAuthoredContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "links.md")
	if err := os.WriteFile(path, []byte(authoredLinksMD), 0o644); err != nil {
		t.Fatal(err)
	}

	newRef := Ref{Provider: "github:owner/repo", ID: "42", URL: "https://github.com/owner/repo/issues/42"}
	if err := saveLinksToMD(dir, "", []Ref{newRef}); err != nil {
		t.Fatalf("saveLinksToMD: %v", err)
	}

	got := readFileStr(t, path)
	if !strings.HasPrefix(got, authoredLinksMD) {
		t.Fatalf("authored content not preserved verbatim:\n%s", got)
	}
	for _, want := range []string{
		"Sequencing: the migration must land",
		"## Blocked by",
		"- owner/repo#10",
		"## Related",
		"- owner/repo#11",
		"- owner/repo#42",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// TestSaveLinksToMD_AlreadyRecordedWritesNothing pins idempotence: a ref already
// present — in any accepted spelling — leaves the file byte-for-byte unchanged,
// so repeat `link` runs produce no diff to review.
func TestSaveLinksToMD_AlreadyRecordedWritesNothing(t *testing.T) {
	ref := Ref{Provider: "github:owner/repo", ID: "42", URL: "https://github.com/owner/repo/issues/42"}

	for _, tc := range []struct {
		name     string
		existing string
	}{
		{"shorthand", "- owner/repo#42\n"},
		{"full URL", "- https://github.com/owner/repo/issues/42\n"},
		{"inside a section", "## Related\n\n- owner/repo#42\n"},
		{"alongside prose", "Notes about ordering.\n\n- owner/repo#42\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "links.md")
			if err := os.WriteFile(path, []byte(tc.existing), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := saveLinksToMD(dir, "", []Ref{ref}); err != nil {
				t.Fatalf("saveLinksToMD: %v", err)
			}
			if got := readFileStr(t, path); got != tc.existing {
				t.Errorf("file changed:\nwant %q\ngot  %q", tc.existing, got)
			}
		})
	}
}

// TestSaveLinksToMD_DedupsAgainstSiblingSlug covers the resolved-entry compare:
// a sibling recorded by slug already means that issue is linked, so writing its
// shorthand too would render the same issue twice in the Related section.
func TestSaveLinksToMD_DedupsAgainstSiblingSlug(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	sibling := filepath.Join(openspecDir, "changes", "other-change")
	if err := os.MkdirAll(filepath.Join(sibling, ".specsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	refsJSON := `{"github:owner/repo":{"provider":"github:owner/repo","id":"42","url":"https://github.com/owner/repo/issues/42"}}`
	if err := os.WriteFile(filepath.Join(sibling, ".specsync", "refs.json"), []byte(refsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	mine := filepath.Join(openspecDir, "changes", "my-change")
	if err := os.MkdirAll(mine, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "- other-change\n"
	path := filepath.Join(mine, "links.md")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	ref := Ref{Provider: "github:owner/repo", ID: "42", URL: "https://github.com/owner/repo/issues/42"}
	if err := saveLinksToMD(mine, openspecDir, []Ref{ref}); err != nil {
		t.Fatalf("saveLinksToMD: %v", err)
	}
	if got := readFileStr(t, path); got != existing {
		t.Errorf("slug entry should already count as recorded:\nwant %q\ngot  %q", existing, got)
	}
}

// TestSaveLinksToMD_NewFile keeps the from-scratch output a plain list — the
// append-only rule must not impose a section header on changes that have none.
func TestSaveLinksToMD_NewFile(t *testing.T) {
	dir := t.TempDir()
	refs := []Ref{
		{Provider: "github:owner/repo", ID: "42", URL: "https://github.com/owner/repo/issues/42"},
		{Provider: "github:owner/other", ID: "7", URL: "https://github.com/owner/other/issues/7"},
	}
	if err := saveLinksToMD(dir, "", refs); err != nil {
		t.Fatalf("saveLinksToMD: %v", err)
	}
	want := "- owner/repo#42\n- owner/other#7\n"
	if got := readFileStr(t, filepath.Join(dir, "links.md")); got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

// TestSaveLinksToMD_NoNewRefsLeavesFileAbsent: nothing to record means nothing
// to write, so a change with no links keeps a clean working tree.
func TestSaveLinksToMD_NoNewRefsLeavesFileAbsent(t *testing.T) {
	dir := t.TempDir()
	if err := saveLinksToMD(dir, "", nil); err != nil {
		t.Fatalf("saveLinksToMD: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "links.md")); !os.IsNotExist(err) {
		t.Error("no refs should write no links.md")
	}
}

// TestSaveLinksToMD_AppendsAfterUnterminatedLine guards the join: a file whose
// last line lacks a newline must not have the new entry glued onto it.
func TestSaveLinksToMD_AppendsAfterUnterminatedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "links.md")
	if err := os.WriteFile(path, []byte("- owner/repo#11"), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := Ref{Provider: "github:owner/repo", ID: "42", URL: "https://github.com/owner/repo/issues/42"}
	if err := saveLinksToMD(dir, "", []Ref{ref}); err != nil {
		t.Fatalf("saveLinksToMD: %v", err)
	}
	want := "- owner/repo#11\n- owner/repo#42\n"
	if got := readFileStr(t, path); got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func readFileStr(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
