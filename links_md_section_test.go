package specsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Entries belong to the "## " heading above them, and saveLinksToMD appends at
// the end of the file. So a related link appended to a file whose last section
// is "## Blocked by" was read back as a dependency (#198).

var link5 = Ref{Provider: "github:owner/repo", ID: "5", URL: "https://github.com/owner/repo/issues/5"}

func saveAndParse(t *testing.T, existing string, refs ...Ref) (string, []Ref, []Ref, []Ref) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "links.md")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := saveLinksToMD(dir, "", refs); err != nil {
		t.Fatalf("saveLinksToMD: %v", err)
	}
	related, blockedBy, blocks := parseLinksMD(dir, "")
	return readFileStr(t, path), related, blockedBy, blocks
}

func hasID(refs []Ref, id string) bool {
	for _, r := range refs {
		if r.ID == id {
			return true
		}
	}
	return false
}

func TestSaveLinksToMD_AppendAfterBlockedByStaysRelated(t *testing.T) {
	existing := "## Blocked by\n\n- owner/repo#2\n"
	got, related, blockedBy, _ := saveAndParse(t, existing, link5)
	if !hasID(related, "5") {
		t.Errorf("appended #5 should be related, got related=%v", related)
	}
	if len(blockedBy) != 1 || !hasID(blockedBy, "2") {
		t.Errorf("Blocked by must still list only #2, got %v", blockedBy)
	}
	want := existing + "\n## Related\n\n- owner/repo#5\n"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestSaveLinksToMD_AppendAfterBlocksStaysRelated(t *testing.T) {
	_, related, _, blocks := saveAndParse(t, "## Blocks\n- owner/repo#3\n", link5)
	if !hasID(related, "5") || hasID(blocks, "5") {
		t.Errorf("appended #5 should be related only: related=%v blocks=%v", related, blocks)
	}
}

// A heading specsync does not own (an author's notes, or another tool's
// section) must not swallow related links either.
func TestSaveLinksToMD_AppendAfterForeignHeadingAddsRelated(t *testing.T) {
	existing := "## Notes\n\n- see the design doc\n"
	got, related, _, _ := saveAndParse(t, existing, link5)
	if !strings.HasPrefix(got, existing) {
		t.Fatalf("existing bytes must be unchanged, got %q", got)
	}
	if !strings.Contains(got[len(existing):], "## Related") {
		t.Errorf("expected a Related heading before the appended entry, got %q", got)
	}
	if !hasID(related, "5") {
		t.Errorf("appended #5 should be related, got %v", related)
	}
}

func TestSaveLinksToMD_LastSectionRelatedGetsNoSecondHeading(t *testing.T) {
	existing := "## Blocked by\n- owner/repo#2\n\n## Related\n- owner/repo#11\n"
	got, related, blockedBy, _ := saveAndParse(t, existing, link5)
	if n := strings.Count(got, "## Related"); n != 1 {
		t.Errorf("want one Related heading, got %d in %q", n, got)
	}
	if got != existing+"- owner/repo#5\n" {
		t.Errorf("want plain append, got %q", got)
	}
	if !hasID(related, "5") || hasID(blockedBy, "5") {
		t.Errorf("related=%v blockedBy=%v", related, blockedBy)
	}
}

func TestSaveLinksToMD_SectionAppendIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "links.md")
	if err := os.WriteFile(path, []byte("## Blocked by\n- owner/repo#2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := saveLinksToMD(dir, "", []Ref{link5}); err != nil {
			t.Fatalf("saveLinksToMD: %v", err)
		}
	}
	got := readFileStr(t, path)
	if n := strings.Count(got, "owner/repo#5"); n != 1 {
		t.Errorf("second save must write nothing, #5 appears %d times in %q", n, got)
	}
	if n := strings.Count(got, "## Related"); n != 1 {
		t.Errorf("second save must not add another heading, got %d", n)
	}
}
