package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeTaskStateUnion(t *testing.T) {
	local := strings.Join([]string{
		"- [ ] first task",
		"- [x] already done locally",
		"- [ ] only local task",
		"  - [ ] indented subtask",
		"- [~] dropped: superseded", // living-plan marker: must be untouched
		"- [>] moved: other-slug",   // living-plan marker: must be untouched
		"not a task line",
	}, "\n")

	issue := map[string]bool{
		"first task":             true,  // ticked on the issue -> should flip local
		"already done locally":   false, // issue lags; union must NOT revert local
		"indented subtask":       true,  // matches across indentation
		"dropped: superseded":    true,  // ignored: not a [ ]/[x] line
		"task only on the issue": true,  // no local match -> ignored
	}

	merged, flips := mergeTaskState(local, issue)

	if !strings.Contains(merged, "- [x] first task") {
		t.Errorf("first task should be checked from issue:\n%s", merged)
	}
	if !strings.Contains(merged, "- [x] already done locally") {
		t.Errorf("local progress must not be reverted:\n%s", merged)
	}
	if !strings.Contains(merged, "  - [x] indented subtask") {
		t.Errorf("indented subtask should flip and keep indentation:\n%s", merged)
	}
	if !strings.Contains(merged, "- [ ] only local task") {
		t.Errorf("unmatched local task should stay unchecked:\n%s", merged)
	}
	if !strings.Contains(merged, "- [~] dropped: superseded") || !strings.Contains(merged, "- [>] moved: other-slug") {
		t.Errorf("living-plan markers must be left untouched:\n%s", merged)
	}

	// Two flips: "first task" and "indented subtask". The already-done local task
	// is unchanged (union == its existing state), so it is not a flip.
	if len(flips) != 2 {
		t.Fatalf("want 2 flips, got %d: %+v", len(flips), flips)
	}
	for _, f := range flips {
		if !f.Checked {
			t.Errorf("v1 union only ever flips toward checked, got %+v", f)
		}
	}
}

func TestParseTaskLine(t *testing.T) {
	cases := []struct {
		line        string
		wantText    string
		wantChecked bool
		wantOK      bool
	}{
		{"- [ ] do a thing", "do a thing", false, true},
		{"- [x] done", "done", true, true},
		{"- [X] done caps", "done caps", true, true},
		{"  - [ ]  spaced  out ", "spaced out", false, true},
		{"- [~] dropped: reason", "", false, false},
		{"- [>] moved: slug", "", false, false},
		{"- [link](url) not a task", "", false, false},
		{"plain text", "", false, false},
	}
	for _, c := range cases {
		text, checked, ok := parseTaskLine(c.line)
		if ok != c.wantOK || text != c.wantText || checked != c.wantChecked {
			t.Errorf("parseTaskLine(%q) = (%q,%v,%v), want (%q,%v,%v)",
				c.line, text, checked, ok, c.wantText, c.wantChecked, c.wantOK)
		}
	}
}

// fakeIssueProvider implements WorkProvider + IssueReader, returning a fixed
// issue body and recording the last pushed item.
type fakeIssueProvider struct {
	body   string
	ref    Ref
	pushed WorkItem
	gets   int
}

func (f *fakeIssueProvider) Name() string { return "github" }
func (f *fakeIssueProvider) Push(_ context.Context, item WorkItem, _ *Ref) (Ref, error) {
	f.pushed = item
	return f.ref, nil
}
func (f *fakeIssueProvider) Find(context.Context, string) (*Ref, error) { return &f.ref, nil }
func (f *fakeIssueProvider) Get(_ context.Context, _ string) (FetchedItem, error) {
	f.gets++
	return FetchedItem{ID: f.ref.ID, URL: f.ref.URL, Body: f.body}, nil
}

func seedChange(t *testing.T, root, slug, tasks string) string {
	t.Helper()
	cdir := filepath.Join(root, "changes", slug)
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# "+slug+"\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), tasks)
	mustWrite(t, refCachePath(cdir),
		`{"github":{"provider":"github","id":"7","url":"https://github.com/o/r/issues/7"}}`)
	return cdir
}

func TestSyncReconcilesIssueChecks(t *testing.T) {
	root := t.TempDir()
	cdir := seedChange(t, root, "c1", "- [ ] first task\n- [ ] second task\n")

	prov := &fakeIssueProvider{
		ref:  Ref{Provider: "github", ID: "7", URL: "https://github.com/o/r/issues/7"},
		body: "<!-- specsync:change=c1 -->\n\n# c1\n\nbody\n\n## Tasks\n\n- [x] first task\n- [ ] second task\n",
	}

	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, Reconcile: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(cdir, "tasks.md"))
	if !strings.Contains(string(got), "- [x] first task") {
		t.Errorf("first task not reconciled to checked on disk:\n%s", got)
	}
	if !strings.Contains(string(got), "- [ ] second task") {
		t.Errorf("second task should remain unchecked:\n%s", got)
	}
	if !strings.Contains(prov.pushed.Body, "- [x] first task") {
		t.Errorf("pushed body should reflect the merged check:\n%s", prov.pushed.Body)
	}
	if len(res.Items) != 1 || len(res.Items[0].Flips) != 1 {
		t.Fatalf("want exactly 1 flip reported, got %+v", res.Items)
	}
}

func TestSyncReconcileDoesNotRevertLocalProgress(t *testing.T) {
	root := t.TempDir()
	cdir := seedChange(t, root, "c1", "- [x] done locally\n")

	// Issue still shows it unchecked (it lags an un-pushed local edit).
	prov := &fakeIssueProvider{
		ref:  Ref{Provider: "github", ID: "7", URL: "https://github.com/o/r/issues/7"},
		body: "<!-- specsync:change=c1 -->\n\n## Tasks\n\n- [ ] done locally\n",
	}

	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, Reconcile: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(cdir, "tasks.md"))
	if !strings.Contains(string(got), "- [x] done locally") {
		t.Errorf("local completion must survive (issue must not revert it):\n%s", got)
	}
	if len(res.Items[0].Flips) != 0 {
		t.Errorf("no flip expected when union equals local state, got %+v", res.Items[0].Flips)
	}
}

func TestSyncDryRunSkipsReconcile(t *testing.T) {
	root := t.TempDir()
	cdir := seedChange(t, root, "c1", "- [ ] first task\n")

	prov := &fakeIssueProvider{
		ref:  Ref{Provider: "github", ID: "7", URL: "https://github.com/o/r/issues/7"},
		body: "<!-- specsync:change=c1 -->\n\n## Tasks\n\n- [x] first task\n",
	}

	if _, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, Reconcile: true, DryRun: true}); err != nil {
		t.Fatalf("dry-run Sync: %v", err)
	}
	if prov.gets != 0 {
		t.Errorf("dry-run must make no issue reads, got %d Get calls", prov.gets)
	}
	got, _ := os.ReadFile(filepath.Join(cdir, "tasks.md"))
	if !strings.Contains(string(got), "- [ ] first task") {
		t.Errorf("dry-run must not modify tasks.md:\n%s", got)
	}
}

func TestThreeWayMergePropagatesUncheck(t *testing.T) {
	// Base: task was checked. Issue unchecked it. Local still checked it.
	// 3-way merge should propagate the uncheck from issue.
	base := "- [x] task one\n- [ ] task two\n"
	local := "- [x] task one\n- [ ] task two\n" // local hasn't changed
	issue := map[string]bool{
		"task one": false, // unchecked on the issue
		"task two": false, // still unchecked
	}

	merged, flips, used3way, err := reconcileThreeWay(context.Background(), &Change{
		TasksMarkdown: local,
	}, issue, &Ref{Base: base})
	if err != nil {
		t.Fatalf("reconcileThreeWay: %v", err)
	}
	if !used3way {
		t.Fatal("expected 3-way merge to be used")
	}
	if !strings.Contains(merged, "- [ ] task one") {
		t.Errorf("task one should be unchecked (issue uncheck relative to base):\n%s", merged)
	}
	if len(flips) != 1 || flips[0].Text != "task one" || flips[0].Checked {
		t.Errorf("expected one uncheck flip for task one, got %+v", flips)
	}
}

func TestThreeWayMergeNoUncheckWhenLocalProgress(t *testing.T) {
	// Base: task was checked. Issue unchecked it. Local already unchecked it independently.
	// 3-way merge should NOT re-check (issue uncheck relative to base, but local already unchecked).
	base := "- [x] task one\n"
	local := "- [ ] task one\n" // local already unchecked
	issue := map[string]bool{
		"task one": false, // unchecked on the issue
	}

	merged, flips, used3way, err := reconcileThreeWay(context.Background(), &Change{
		TasksMarkdown: local,
	}, issue, &Ref{Base: base})
	if err != nil {
		t.Fatalf("reconcileThreeWay: %v", err)
	}
	if !used3way {
		t.Fatal("expected 3-way merge to be used")
	}
	if !strings.Contains(merged, "- [ ] task one") {
		t.Errorf("task one should remain unchecked:\n%s", merged)
	}
	if len(flips) != 0 {
		t.Errorf("expected no flips (already unchecked locally), got %+v", flips)
	}
}

func TestThreeWayMergeDoesNotRevertLocalChecks(t *testing.T) {
	// Base: task was unchecked. Issue still unchecked. Local checked it.
	// 3-way merge should NOT revert local progress.
	base := "- [ ] task one\n"
	local := "- [x] task one\n" // local checked it
	issue := map[string]bool{
		"task one": false, // still unchecked on the issue
	}

	merged, flips, used3way, err := reconcileThreeWay(context.Background(), &Change{
		TasksMarkdown: local,
	}, issue, &Ref{Base: base})
	if err != nil {
		t.Fatalf("reconcileThreeWay: %v", err)
	}
	if !used3way {
		t.Fatal("expected 3-way merge to be used")
	}
	if !strings.Contains(merged, "- [x] task one") {
		t.Errorf("local check must not be reverted:\n%s", merged)
	}
	if len(flips) != 0 {
		t.Errorf("expected no flips, got %+v", flips)
	}
}

func TestThreeWayMergeSkipsTasksNotInBase(t *testing.T) {
	// Task was added locally after base was saved. Issue doesn't know about it.
	// 3-way merge should NOT touch it.
	base := "- [ ] task one\n"
	local := "- [ ] task one\n- [ ] new task\n"
	issue := map[string]bool{
		"task one": true, // checked on the issue
	}

	merged, flips, used3way, err := reconcileThreeWay(context.Background(), &Change{
		TasksMarkdown: local,
	}, issue, &Ref{Base: base})
	if err != nil {
		t.Fatalf("reconcileThreeWay: %v", err)
	}
	if !used3way {
		t.Fatal("expected 3-way merge to be used")
	}
	if !strings.Contains(merged, "- [x] task one") {
		t.Errorf("task one should be checked from issue:\n%s", merged)
	}
	if !strings.Contains(merged, "- [ ] new task") {
		t.Errorf("new task should remain unchecked:\n%s", merged)
	}
	if len(flips) != 1 || flips[0].Text != "task one" {
		t.Errorf("expected one flip for task one, got %+v", flips)
	}
}

func TestThreeWayMergeFallsBackNoBase(t *testing.T) {
	// No base state — should fall back to 2-way union.
	_, _, used3way, err := reconcileThreeWay(context.Background(), &Change{
		TasksMarkdown: "- [x] task one\n",
	}, map[string]bool{
		"task one": false, // unchecked on the issue
	}, &Ref{})
	if err != nil {
		t.Fatalf("reconcileThreeWay: %v", err)
	}
	if used3way {
		t.Fatal("expected 3-way merge to NOT be used (no base)")
	}
}

func TestBaseStatePreservedAcrossSyncs(t *testing.T) {
	root := t.TempDir()
	tasks := "- [ ] task one\n- [ ] task two\n"
	cdir := filepath.Join(root, "changes", "c1")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# c1\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), tasks)
	mustWrite(t, refCachePath(cdir),
		`{"github":{"provider":"github","id":"7","url":"https://github.com/o/r/issues/7"}}`)

	prov := &fakeIssueProvider{
		ref:  Ref{Provider: "github", ID: "7", URL: "https://github.com/o/r/issues/7"},
		body: "<!-- specsync:change=c1 -->\n\n## Tasks\n\n- [ ] task one\n- [ ] task two\n",
	}

	// First sync — establishes base state.
	_, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, Reconcile: true})
	if err != nil {
		t.Fatalf("first Sync: %v", err)
	}

	// Verify base state is saved in ref cache.
	refs, err := loadRefs(cdir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	if refs["github"].Base == "" {
		t.Error("base state should be saved after first sync")
	}

	// Second sync — base state should still be there.
	refs2, err := loadRefs(cdir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	if refs2["github"].Base == "" {
		t.Error("base state should persist across syncs")
	}
}

func TestStableTaskIDWordingChange(t *testing.T) {
	// Base has "task one". Local rewrote it to "task one revised".
	// Issue checked "task one". 3-way merge should match by base text.
	base := "- [ ] task one\n- [ ] task two\n"
	local := "- [ ] task one revised\n- [ ] task two\n"
	issue := map[string]bool{
		"task one": true, // checked on the issue (original text)
		"task two": false,
	}

	merged, flips, used3way, err := reconcileThreeWay(context.Background(), &Change{
		TasksMarkdown: local,
	}, issue, &Ref{Base: base})
	if err != nil {
		t.Fatalf("reconcileThreeWay: %v", err)
	}
	if !used3way {
		t.Fatal("expected 3-way merge to be used")
	}
	if !strings.Contains(merged, "- [x] task one revised") {
		t.Errorf("rewritten task should be checked (matched by base text):\n%s", merged)
	}
	if len(flips) != 1 || flips[0].Text != "task one revised" {
		t.Errorf("expected one flip for rewritten task, got %+v", flips)
	}
}

func TestStableTaskIDWordingChangeUncheck(t *testing.T) {
	// Base has "task one" (checked). Local rewrote to "task one revised" (checked).
	// Issue unchecked "task one". 3-way merge should propagate uncheck.
	base := "- [x] task one\n- [ ] task two\n"
	local := "- [x] task one revised\n- [ ] task two\n"
	issue := map[string]bool{
		"task one": false, // unchecked on the issue
		"task two": false,
	}

	merged, flips, used3way, err := reconcileThreeWay(context.Background(), &Change{
		TasksMarkdown: local,
	}, issue, &Ref{Base: base})
	if err != nil {
		t.Fatalf("reconcileThreeWay: %v", err)
	}
	if !used3way {
		t.Fatal("expected 3-way merge to be used")
	}
	if !strings.Contains(merged, "- [ ] task one revised") {
		t.Errorf("rewritten task should be unchecked (matched by base text):\n%s", merged)
	}
	if len(flips) != 1 || flips[0].Text != "task one revised" || flips[0].Checked {
		t.Errorf("expected one uncheck flip for rewritten task, got %+v", flips)
	}
}

func TestBuildBaseToCurrentMapping(t *testing.T) {
	base := "- [ ] task one\n- [ ] task two\n- [ ] task three\n"
	current := "- [ ] task one revised\n- [ ] task two\n- [ ] task three\n"

	mapping := buildBaseToCurrentMapping(base, current)

	if baseText, ok := mapping["task one revised"]; !ok || baseText != "task one" {
		t.Errorf("expected task one revised -> task one, got %q", mapping["task one revised"])
	}
	if _, ok := mapping["task two"]; ok {
		t.Error("task two unchanged, should not be in mapping")
	}
	if _, ok := mapping["task three"]; ok {
		t.Error("task three unchanged, should not be in mapping")
	}
}

func TestParseTaskState_Todo(t *testing.T) {
	_, state, ok := parseTaskState("- [ ] todo task")
	if !ok || state != TaskStateTodo {
		t.Errorf("state = %v, ok = %v, want TaskStateTodo, true", state, ok)
	}
}

func TestParseTaskState_Done(t *testing.T) {
	_, state, ok := parseTaskState("- [x] done task")
	if !ok || state != TaskStateDone {
		t.Errorf("state = %v, ok = %v, want TaskStateDone, true", state, ok)
	}
}

func TestParseTaskState_Dropped(t *testing.T) {
	_, state, ok := parseTaskState("- [~] dropped task")
	if !ok || state != TaskStateDropped {
		t.Errorf("state = %v, ok = %v, want TaskStateDropped, true", state, ok)
	}
}

func TestParseTaskState_Moved(t *testing.T) {
	_, state, ok := parseTaskState("- [>] moved task")
	if !ok || state != TaskStateMoved {
		t.Errorf("state = %v, ok = %v, want TaskStateMoved, true", state, ok)
	}
}

func TestParseTaskState_NotTask(t *testing.T) {
	_, _, ok := parseTaskState("not a task")
	if ok {
		t.Error("expected ok=false for non-task line")
	}
}

func TestCountTaskStates_Mixed(t *testing.T) {
	md := "- [ ] todo\n- [x] done\n- [~] dropped\n- [>] moved\n- [ ] another todo\n"
	c := countTaskStates(md)
	if c.Todo != 2 {
		t.Errorf("todo = %d, want 2", c.Todo)
	}
	if c.Done != 1 {
		t.Errorf("done = %d, want 1", c.Done)
	}
	if c.Dropped != 1 {
		t.Errorf("dropped = %d, want 1", c.Dropped)
	}
	if c.Moved != 1 {
		t.Errorf("moved = %d, want 1", c.Moved)
	}
	if c.LiveTotal() != 3 {
		t.Errorf("liveTotal = %d, want 3", c.LiveTotal())
	}
	if c.Total() != 5 {
		t.Errorf("total = %d, want 5", c.Total())
	}
}

func TestCountTaskStates_IsComplete(t *testing.T) {
	md := "- [x] done\n- [x] done\n"
	if !countTaskStates(md).IsComplete() {
		t.Error("expected complete")
	}
}

func TestCountTaskStates_IsCompleteWithDropped(t *testing.T) {
	md := "- [x] done\n- [~] dropped\n"
	if !countTaskStates(md).IsComplete() {
		t.Error("expected complete (dropped tasks don't count)")
	}
}

func TestCountTaskStates_NotCompleteWithTodo(t *testing.T) {
	md := "- [x] done\n- [ ] todo\n- [~] dropped\n"
	if countTaskStates(md).IsComplete() {
		t.Error("expected not complete")
	}
}

func TestCountCheckboxes_ExcludesDroppedMoved(t *testing.T) {
	md := "- [ ] todo\n- [x] done\n- [~] dropped\n- [>] moved\n"
	total, completed := CountCheckboxes(md)
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if completed != 1 {
		t.Errorf("completed = %d, want 1", completed)
	}
}

func TestTasksComplete_ExcludesDroppedMoved(t *testing.T) {
	md := "- [x] done\n- [~] dropped\n- [>] moved\n"
	if !tasksComplete(md) {
		t.Error("expected complete (only live tasks matter)")
	}
}
