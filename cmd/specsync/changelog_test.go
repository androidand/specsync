package main

import (
	"strings"
	"testing"

	"github.com/androidand/specsync"
)

// TestUnlinkedCommitsError pins the changelog-commit-linking gate: it must
// only fire on entries that reached the raw fallback (Hash set, Slug empty —
// exactly what looseEntry produces for a conventional commit with no
// recognized issue reference), must be a no-op when the flag isn't set, and
// must not fire on properly-linked change entries (Slug set, no Hash).
func TestUnlinkedCommitsError(t *testing.T) {
	linked := specsync.ChangelogEntry{Text: "add widget support", Slug: "add-widget-support"}
	unlinked := specsync.ChangelogEntry{Text: "add gadget support", Hash: "abc1234"}

	t.Run("flag off is always a no-op", func(t *testing.T) {
		cl := specsync.Changelog{Entries: []specsync.ChangelogEntry{unlinked}}
		if err := unlinkedCommitsError(cl, false); err != nil {
			t.Fatalf("expected nil with the flag off, got %v", err)
		}
	})

	t.Run("only linked entries: no error", func(t *testing.T) {
		cl := specsync.Changelog{Entries: []specsync.ChangelogEntry{linked}}
		if err := unlinkedCommitsError(cl, true); err != nil {
			t.Fatalf("expected nil for a fully-linked changelog, got %v", err)
		}
	})

	t.Run("an unlinked commit fails and names the commit", func(t *testing.T) {
		cl := specsync.Changelog{Entries: []specsync.ChangelogEntry{linked, unlinked}}
		err := unlinkedCommitsError(cl, true)
		if err == nil {
			t.Fatal("expected an error when an unlinked commit is present")
		}
		if !strings.Contains(err.Error(), "add gadget support") || !strings.Contains(err.Error(), "abc1234") {
			t.Fatalf("error %q should name the offending commit's description and hash", err)
		}
		if strings.Contains(err.Error(), "add widget support") {
			t.Fatalf("error %q should not mention the properly-linked entry", err)
		}
	})

	t.Run("empty changelog: no error", func(t *testing.T) {
		if err := unlinkedCommitsError(specsync.Changelog{}, true); err != nil {
			t.Fatalf("expected nil for an empty changelog, got %v", err)
		}
	})
}

// TestUnlinkedCommitsErrorIgnoresSilentlyOmittedCommits: a non-conventional
// loose commit never reaches cl.Entries at all (BuildChangelog only counts
// it in OmittedCommits), so the gate has nothing to flag for it — this only
// catches commits that would otherwise render as a raw fallback line, not
// the ones already silently omitted.
func TestUnlinkedCommitsErrorIgnoresSilentlyOmittedCommits(t *testing.T) {
	in := specsync.TraceInput{
		Commits: []specsync.Commit{
			specsync.ParseCommit("h1", "a", "d", "random non conventional message"),
		},
	}
	cl := specsync.BuildChangelog(in, nil)
	if len(cl.Entries) != 0 {
		t.Fatalf("non-conventional commit should not become a changelog entry, got %+v", cl.Entries)
	}
	if err := unlinkedCommitsError(cl, true); err != nil {
		t.Fatalf("expected nil — nothing for the gate to flag, got %v", err)
	}
}
