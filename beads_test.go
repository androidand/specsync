package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeBD records the bd commands issued and returns canned output: a fixed JSON
// list for `list`/`show`, distinct ids for successive `create`s (first is the
// epic), and empty for `close`.
type fakeBD struct {
	list    string
	calls   [][]string
	created int
}

func (f *fakeBD) run(_ context.Context, args ...string) (string, error) {
	f.calls = append(f.calls, args)
	switch args[0] {
	case "list", "show":
		if f.list == "" {
			return "[]", nil
		}
		return f.list, nil
	case "create":
		f.created++
		if f.created == 1 {
			return "bd-epic", nil
		}
		return fmt.Sprintf("bd-child-%d", f.created-1), nil
	default: // close, etc.
		return "", nil
	}
}

func hasFlagVal(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}

func countCreates(calls [][]string, pred func([]string) bool) int {
	n := 0
	for _, c := range calls {
		if c[0] == "create" && pred(c) {
			n++
		}
	}
	return n
}

const beadsFamilyJSON = `[
  {"id":"bd-epic","title":"Change One","description":"# Change One\n\nbody\n\n<!-- specsync:change=c1 -->","status":"open","issue_type":"epic"},
  {"id":"bd-1","title":"first task","description":"<!-- specsync:change=c1 -->","status":"closed","issue_type":"task"},
  {"id":"bd-2","title":"second task","description":"<!-- specsync:change=c1 -->","status":"open","issue_type":"task"}
]`

func beadsWorkItem() WorkItem {
	return WorkItem{
		Slug:  "c1",
		Title: "Change One",
		Body:  "<!-- specsync:change=c1 -->\n\n# Change One\n\nbody\n\n## Tasks\n\n- [ ] first task\n- [ ] second task\n",
	}
}

func TestBeadsPushCreatesEpicAndChildren(t *testing.T) {
	f := &fakeBD{} // empty family -> everything is created
	p := NewBeadsProviderFunc(f.run)

	ref, err := p.Push(context.Background(), beadsWorkItem(), nil)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if ref.ID != "bd-epic" || ref.URL != "bd://bd-epic" {
		t.Fatalf("epic ref = %+v, want id bd-epic / url bd://bd-epic", ref)
	}

	epics := countCreates(f.calls, func(c []string) bool { return hasFlagVal(c, "--type", "epic") })
	if epics != 1 {
		t.Errorf("want 1 epic create, got %d:\n%v", epics, f.calls)
	}
	children := countCreates(f.calls, func(c []string) bool { return hasFlagVal(c, "--parent", "bd-epic") })
	if children != 2 {
		t.Errorf("want 2 child creates parented to the epic, got %d:\n%v", children, f.calls)
	}
}

func TestBeadsPushIsCreateOnlyWhenFamilyExists(t *testing.T) {
	f := &fakeBD{list: beadsFamilyJSON} // epic + both children already present
	p := NewBeadsProviderFunc(f.run)

	ref, err := p.Push(context.Background(), beadsWorkItem(), nil)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if ref.ID != "bd-epic" {
		t.Fatalf("should reuse existing epic, got %+v", ref)
	}
	if f.created != 0 {
		t.Errorf("re-push must not create anything, got %d creates:\n%v", f.created, f.calls)
	}
}

func TestBeadsTaskStatesExcludesEpic(t *testing.T) {
	f := &fakeBD{list: beadsFamilyJSON}
	p := NewBeadsProviderFunc(f.run)

	states, err := p.TaskStates(context.Background(), "c1", nil)
	if err != nil {
		t.Fatalf("TaskStates: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("want 2 task states (epic excluded), got %d: %+v", len(states), states)
	}
	if !states["first task"] {
		t.Errorf("closed child should be done=true: %+v", states)
	}
	if states["second task"] {
		t.Errorf("open child should be done=false: %+v", states)
	}
	if _, ok := states["Change One"]; ok {
		t.Errorf("epic title must not appear as a task state: %+v", states)
	}
}

func TestBeadsFindReturnsEpic(t *testing.T) {
	f := &fakeBD{list: beadsFamilyJSON}
	p := NewBeadsProviderFunc(f.run)

	ref, err := p.Find(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ref == nil || ref.ID != "bd-epic" {
		t.Fatalf("Find should return the epic ref, got %+v", ref)
	}
}

// TestSyncReconcilesFromBeads is the keystone: a closed child bead must flip the
// matching tasks.md checkbox via the SAME mergeTaskState the GitHub path uses,
// driven by the TaskStateReader capability — no reconcile logic duplicated.
func TestSyncReconcilesFromBeads(t *testing.T) {
	root := t.TempDir()
	cdir := seedChange(t, root, "c1", "- [ ] first task\n- [ ] second task\n")

	f := &fakeBD{list: beadsFamilyJSON} // "first task" is closed in Beads
	p := NewBeadsProviderFunc(f.run)

	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: p, Reconcile: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(cdir, "tasks.md"))
	if !strings.Contains(string(got), "- [x] first task") {
		t.Errorf("closed bead should flip its task to checked on disk:\n%s", got)
	}
	if !strings.Contains(string(got), "- [ ] second task") {
		t.Errorf("open bead's task should stay unchecked:\n%s", got)
	}
	if len(res.Items) != 1 || len(res.Items[0].Flips) != 1 {
		t.Fatalf("want exactly 1 reconciled flip, got %+v", res.Items)
	}
}
