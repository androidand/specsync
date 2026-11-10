package specsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldChangeWritesTheShortTemplateAndRefusesAnExistingOne(t *testing.T) {
	root := t.TempDir()
	slug, err := ScaffoldChange(root, "Show a clean summary in status")
	if err != nil || slug != "show-a-clean-summary-in-status" {
		t.Fatalf("%q %v", slug, err)
	}
	proposal, _ := os.ReadFile(filepath.Join(root, "changes", slug, "proposal.md"))
	for _, want := range []string{"# Show a clean summary in status", "## Why", "## What", "## Out of scope"} {
		if !strings.Contains(string(proposal), want) {
			t.Errorf("proposal lacks %q:\n%s", want, proposal)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "changes", slug, "tasks.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := ScaffoldChange(root, "Show a clean summary in status"); err == nil {
		t.Fatal("an existing change must not be overwritten")
	}
	if _, err := ScaffoldChange(root, "   "); err == nil {
		t.Fatal("an empty title must be refused")
	}
}

func TestKeyWordsDropFillerAndDuplicatesAreFoundByOverlap(t *testing.T) {
	got := KeyWords("Add a retry to the sync of issues")
	want := []string{"add", "retry", "sync", "issues"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v", got)
	}
	cands := []DuplicateCandidate{
		{Kind: "change", Ref: "retry-sync", Title: "Retry the sync of issues on failure"},
		{Kind: "change", Ref: "other", Title: "Rename the topology command"},
		{Kind: "issue", Ref: "#12", Title: "Sync issues with retry"},
	}
	dups := FindDuplicates("Add a retry to the sync of issues", cands)
	if len(dups) != 2 || dups[0].Ref != "retry-sync" && dups[0].Ref != "#12" {
		t.Fatalf("got %+v", dups)
	}
	for _, d := range dups {
		if d.Ref == "other" {
			t.Fatal("an unrelated title is not a duplicate")
		}
	}
	if got := FindDuplicates("x", cands); len(got) != 0 {
		t.Fatalf("one key word is not a match: %+v", got)
	}
}

func TestScaffoldPlaceholdersAreDetectedUntilEdited(t *testing.T) {
	root := t.TempDir()
	slug, _ := ScaffoldChange(root, "Add a retry")
	path := filepath.Join(root, "changes", slug, "proposal.md")
	text, _ := os.ReadFile(path)
	if !HasScaffoldPlaceholders(string(text)) {
		t.Fatal("a fresh scaffold has placeholders")
	}
	edited := strings.NewReplacer("(Say what is wrong today, in two sentences.)", "Syncs fail on one flaky call.").Replace(string(text))
	if !HasScaffoldPlaceholders(edited) {
		t.Fatal("one edited placeholder is not enough: every one must go")
	}
	edited = strings.NewReplacer("(The change, one bullet per behaviour.)", "Retry once.", "(What this does not do.)", "No backoff.").Replace(edited)
	if HasScaffoldPlaceholders(edited) {
		t.Fatalf("an edited proposal has none:\n%s", edited)
	}
}
