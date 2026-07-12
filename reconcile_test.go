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
