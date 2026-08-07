package specsync

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
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
	refs, err := LoadRefs(cdir)
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
	refs, _ = LoadRefs(cdir)
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
	refs, _ := LoadRefs(cdir)
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

	refs, err := LoadRefs(cdir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	_, ref := firstRef(refs)
	if ref.BaseClosed == nil || !*ref.BaseClosed {
		t.Fatalf("re-pull must carry the base forward, got %v", ref.BaseClosed)
	}
}

// TestPullRecordsTaskBase covers the task merge base across pull. Pull overwrites
// tasks.md from the issue, so local and remote end up identical — that is a real
// sync point and becomes the new base. Without it the next sync falls back to a
// monotonic union, where a task unchecked on the issue can never propagate.
func TestPullRecordsTaskBase(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "pulled-tasks")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Pulled\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [ ] stale local\n")

	body := "# Pulled\n\nbody\n\n## Tasks\n\n- [x] first\n- [ ] second\n"
	prov := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "view" {
			b, _ := json.Marshal(map[string]any{
				"number": 7, "url": "https://github.com/o/r/issues/7",
				"title": "Pulled", "body": body, "state": "OPEN", "labels": []any{},
			})
			return string(b), nil
		}
		return "", nil
	})

	if _, err := Pull(context.Background(), PullOptions{
		OpenSpecDir: root, Provider: prov, IssueID: "7", Slug: "pulled-tasks",
	}); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	refs, err := LoadRefs(cdir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	_, ref := firstRef(refs)
	if ref.Base == "" {
		t.Fatal("pull must record a task merge base, got empty")
	}
	// The base must describe what pull actually wrote to tasks.md.
	onDisk := readFileStr(t, filepath.Join(cdir, "tasks.md"))
	if ref.Base != onDisk {
		t.Errorf("base does not match tasks.md:\nbase   %q\non disk %q", ref.Base, onDisk)
	}
	if ref.BaseSHA != taskSHA(onDisk) {
		t.Errorf("BaseSHA does not match its content")
	}
}

// TestPullWithoutTasksKeepsPriorTaskBase: no "## Tasks" section means tasks.md was
// left alone, so the prior base still describes it and must survive.
func TestPullWithoutTasksKeepsPriorTaskBase(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "no-tasks")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# NoTasks\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] local work\n")
	priorBase := "- [ ] local work\n"
	mustWrite(t, filepath.Join(cdir, ".specsync", "refs.json"),
		`{"github:o/r":{"provider":"github:o/r","id":"7","url":"https://github.com/o/r/issues/7","base":"- [ ] local work\n","base_sha":"deadbeef"}}`)

	prov := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "view" {
			b, _ := json.Marshal(map[string]any{
				"number": 7, "url": "https://github.com/o/r/issues/7",
				"title": "NoTasks", "body": "# NoTasks\n\nbody\n", "state": "OPEN", "labels": []any{},
			})
			return string(b), nil
		}
		return "", nil
	})

	if _, err := Pull(context.Background(), PullOptions{
		OpenSpecDir: root, Provider: prov, IssueID: "7", Slug: "no-tasks",
	}); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	refs, _ := LoadRefs(cdir)
	_, ref := firstRef(refs)
	if ref.Base != priorBase {
		t.Errorf("prior task base must survive a task-less pull:\nwant %q\ngot  %q", priorBase, ref.Base)
	}
	if ref.BaseSHA != "deadbeef" {
		t.Errorf("prior BaseSHA must survive, got %q", ref.BaseSHA)
	}
}

// TestUncheckPropagatesAfterPull is the user-facing payoff of recording a base at
// pull time: with one, the next sync runs a real three-way merge and an uncheck on
// the issue reaches tasks.md. With no base, reconcile degrades to a monotonic
// union where "checked wins" and the uncheck is silently dropped.
func TestUncheckPropagatesAfterPull(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "uncheck-flow")

	issueBody := "# Uncheck flow\n\nbody\n\n## Tasks\n\n- [x] first\n- [ ] second\n"
	viewJSON := func(body string) string {
		b, _ := json.Marshal(map[string]any{
			"number": 7, "url": "https://github.com/o/r/issues/7",
			"title": "Uncheck flow", "body": body, "state": "OPEN", "labels": []any{},
		})
		return string(b)
	}

	// Pull the issue: tasks.md becomes "- [x] first", and that is the base.
	pullProv := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "view" {
			return viewJSON(issueBody), nil
		}
		return "", nil
	})
	if _, err := Pull(context.Background(), PullOptions{
		OpenSpecDir: root, Provider: pullProv, IssueID: "7", Slug: "uncheck-flow",
	}); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if got := readFileStr(t, filepath.Join(cdir, "tasks.md")); !strings.Contains(got, "- [x] first") {
		t.Fatalf("pull should have written the checked task, got %q", got)
	}

	// The task is now unchecked on the issue. Sync with reconcile must honor it.
	unchecked := "# Uncheck flow\n\nbody\n\n## Tasks\n\n- [ ] first\n- [ ] second\n"
	syncProv := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "view" {
			return viewJSON(unchecked), nil
		}
		return "", nil
	})
	if _, err := Sync(context.Background(), Options{
		OpenSpecDir: root, Provider: syncProv, Slug: "uncheck-flow", Reconcile: true,
	}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got := readFileStr(t, filepath.Join(cdir, "tasks.md"))
	if strings.Contains(got, "- [x] first") {
		t.Errorf("uncheck did not propagate; tasks.md still has it checked:\n%s", got)
	}
}
