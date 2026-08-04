package specsync

import (
	"context"
	"path/filepath"
	"testing"
)

// TestCloseStateBasePersistsAcrossSyncs walks the full loop the merge depends on:
// sync closes a completed change, the base lands in the (gitignored) ref cache,
// new work appears, and the next sync reopens — because the close was specsync's
// own. This is the behavior the "Reversible completion state" requirement asks
// for, now expressed through the base rather than through unconditional reopen.
func TestCloseStateBasePersistsAcrossSyncs(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "done-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Done\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] only task\n")

	closed := false
	var calls [][]string
	run := func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "issue" && args[1] == "create":
			return "https://github.com/o/r/issues/7", nil
		case args[0] == "issue" && args[1] == "list":
			return "[]", nil
		case args[0] == "issue" && args[1] == "view":
			if closed {
				return `{"state":"CLOSED","labels":[{"name":"stage:complete"}]}`, nil
			}
			return `{"state":"OPEN","labels":[]}`, nil
		case args[0] == "issue" && args[1] == "close":
			closed = true
			return "", nil
		case args[0] == "issue" && args[1] == "reopen":
			closed = false
			return "", nil
		}
		return "", nil
	}

	// Sync 1: completed change closes; the base records specsync's assertion.
	if _, err := Sync(context.Background(), Options{
		OpenSpecDir:    root,
		Provider:       NewGitHubProviderFunc(run),
		Slug:           "done-change",
		CloseCompleted: true,
	}); err != nil {
		t.Fatalf("Sync 1: %v", err)
	}
	if !closed {
		t.Fatal("sync 1 should have closed the issue")
	}
	refs, err := loadRefs(cdir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	_, ref := firstRef(refs)
	if ref.BaseClosed == nil || !*ref.BaseClosed {
		t.Fatalf("sync 1 should persist base closed=true, got %v", ref.BaseClosed)
	}

	// New work appears locally.
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] only task\n- [ ] follow-up\n")

	// Sync 2: local wants open, remote is closed, base says specsync closed it →
	// reopen, and the base flips to open.
	calls = nil
	if _, err := Sync(context.Background(), Options{
		OpenSpecDir:    root,
		Provider:       NewGitHubProviderFunc(run),
		Slug:           "done-change",
		CloseCompleted: true,
	}); err != nil {
		t.Fatalf("Sync 2: %v", err)
	}
	if findCall(calls, "issue", "reopen", "7") == nil {
		t.Fatalf("sync 2 should reopen for new work; calls: %v", calls)
	}
	refs, _ = loadRefs(cdir)
	_, ref = firstRef(refs)
	if ref.BaseClosed == nil || *ref.BaseClosed {
		t.Fatalf("sync 2 should persist base closed=false, got %v", ref.BaseClosed)
	}
}

// TestSyncLeavesExternallyClosedIssueAlone is the reported scenario: the work
// shipped, a PR merge (or a human, or a reviewing agent) closed the issue, and
// the change's tasks are not all checked. A later spec push must not resurrect it.
func TestSyncLeavesExternallyClosedIssueAlone(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "shipped-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Shipped\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] done\n- [ ] still open\n")

	// A prior sync bound the ref while the issue was open — so the base says
	// specsync last left it OPEN. The close that followed came from outside.
	mustWrite(t, filepath.Join(cdir, ".specsync", "refs.json"),
		`{"github:o/r":{"provider":"github:o/r","id":"7","url":"https://github.com/o/r/issues/7","base_closed":false}}`)

	var calls [][]string
	run := func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		if args[0] == "issue" && args[1] == "view" {
			return `{"state":"CLOSED","labels":[{"name":"stage:active"}]}`, nil
		}
		return "", nil
	}

	if _, err := Sync(context.Background(), Options{
		OpenSpecDir:    root,
		Provider:       NewGitHubProviderFuncWithRepo("o/r", run),
		Slug:           "shipped-change",
		CloseCompleted: true,
	}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if call := findCall(calls, "issue", "reopen", "7"); call != nil {
		t.Errorf("an externally closed issue must not be reopened; calls: %v", calls)
	}
	// Content still syncs — only open/closed state is deferred.
	if findCall(calls, "issue", "edit", "7") == nil {
		t.Errorf("issue content should still be updated; calls: %v", calls)
	}
	// And the external close is not adopted as specsync's own base.
	refs, _ := loadRefs(cdir)
	_, ref := firstRef(refs)
	if ref.BaseClosed == nil || *ref.BaseClosed {
		t.Errorf("base should stay open=false, got %v", ref.BaseClosed)
	}
}

// TestDefaultSyncNeverTouchesClosedState: without -close-completed there is no
// open/closed management at all, so an externally closed issue is untouched
// regardless of base.
func TestDefaultSyncNeverTouchesClosedState(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "any-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Any\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] all done\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "refs.json"),
		`{"github:o/r":{"provider":"github:o/r","id":"7","url":"https://github.com/o/r/issues/7","base_closed":true}}`)

	var calls [][]string
	run := func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		if args[0] == "issue" && args[1] == "view" {
			return `{"state":"CLOSED","labels":[{"name":"stage:complete"}]}`, nil
		}
		return "", nil
	}

	if _, err := Sync(context.Background(), Options{
		OpenSpecDir: root,
		Provider:    NewGitHubProviderFuncWithRepo("o/r", run),
		Slug:        "any-change",
	}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	for _, verb := range []string{"close", "reopen"} {
		if call := findCall(calls, "issue", verb, "7"); call != nil {
			t.Errorf("default sync must not %s: %v", verb, call)
		}
	}
}

// TestPullPreservesCloseStateBase: a re-pull must not reset the open/closed merge
// base. Losing it silently revokes specsync's license to reopen an item it closed
// itself — and seeding it from the observed remote state would be worse, licensing
// the undo of a close specsync never made.
func TestPullPreservesCloseStateBase(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "pulled-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Pulled\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] done\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "refs.json"),
		`{"github:o/r":{"provider":"github:o/r","id":"7","url":"https://github.com/o/r/issues/7","base_closed":true}}`)

	prov := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "view" {
			return `{"number":7,"url":"https://github.com/o/r/issues/7","title":"Pulled","body":"# Pulled\n\nbody\n","state":"CLOSED","labels":[]}`, nil
		}
		return "", nil
	})

	if _, err := Pull(context.Background(), PullOptions{
		OpenSpecDir: root,
		Provider:    prov,
		IssueID:     "7",
		Slug:        "pulled-change",
	}); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	refs, err := loadRefs(cdir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	_, ref := firstRef(refs)
	if ref.BaseClosed == nil || !*ref.BaseClosed {
		t.Fatalf("re-pull must carry the base forward, got %v", ref.BaseClosed)
	}
}
