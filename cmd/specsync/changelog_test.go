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

// TestUnlinkedCommitsErrorNamesEveryAvailableRemedy pins the gate's message
// against the failure it caused: the message used to recommend only forms that
// require editing the commit message, which is impossible for a squash-merge.
// A reader hitting that case had no stated remedy and, because the rejected
// commit still printed a "(#N)", no way to tell the gate was right.
//
// Each assertion below fails if its sentence is removed, which is the point:
// the message is the only documentation an author has at the moment they are
// blocked by this gate.
func TestUnlinkedCommitsErrorNamesEveryAvailableRemedy(t *testing.T) {
	cl := specsync.Changelog{Entries: []specsync.ChangelogEntry{
		{Text: "unify agent skill integration", Hash: "57c7e49"},
	}}
	err := unlinkedCommitsError(cl, true)
	if err == nil {
		t.Fatal("expected an error for an unlinked commit")
	}
	msg := err.Error()

	t.Run("names the proposal remedy for commits you cannot edit", func(t *testing.T) {
		if !strings.Contains(msg, "## Release note") {
			t.Fatalf("message must name the only remedy available to a squash-merge author; got %q", msg)
		}
		if !strings.Contains(msg, "squash-merge") {
			t.Fatalf("message must say when the proposal remedy is the applicable one; got %q", msg)
		}
	})

	t.Run("explains why a printed ref can still be unlinked", func(t *testing.T) {
		if !strings.Contains(msg, "PR number") {
			t.Fatalf("message must distinguish a PR ref from an issue ref; got %q", msg)
		}
	})

	// "(#42)" is retained deliberately. It looks like the form to distrust,
	// because extractRefs files a header "(#N)" under PRRefs — but BuildChangelog
	// consults PRRefs too, so "(#42)" binds whenever 42 is the change's issue.
	// See TestBindingDistinguishesHeaderRefFromPRRef. Dropping it from the
	// examples would remove correct advice, so pin its presence.
	t.Run("keeps the header form that does bind", func(t *testing.T) {
		if !strings.Contains(msg, "(#42)") {
			t.Fatalf(`message must keep "(#42)": it binds when 42 is the change's issue; got %q`, msg)
		}
	})
}
