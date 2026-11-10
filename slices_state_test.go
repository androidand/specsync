package specsync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSliceBindingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if got, err := LoadSliceBindings(dir); err != nil || len(got) != 0 {
		t.Fatalf("no file: %v %v", got, err)
	}
	if err := SaveSliceBinding(dir, 2, SliceBinding{Branch: "feat/7-x-slice-2", Worktree: "/w/x-2", Base: "feat/7-x-slice-1"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveSliceBinding(dir, 1, SliceBinding{Branch: "feat/7-x-slice-1"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSliceBindings(dir)
	if err != nil || got[2].Branch != "feat/7-x-slice-2" || got[2].Base != "feat/7-x-slice-1" || got[1].Branch != "feat/7-x-slice-1" {
		t.Fatalf("bindings = %+v, %v", got, err)
	}
}

func TestSliceStatuses_StateComesFromAssertedFacts(t *testing.T) {
	plan := ParseSlices("## Slice 1: A\ncontract: x\n- [x] a\n## Slice 2: B\ncontract: y\nafter: 1\n- [ ] b\n## Slice 3: C\ncontract: z\nafter: 2\n- [ ] c\n## Slice 4: D\ncontract: w\n- [ ] d\n")
	bindings := map[int]SliceBinding{
		1: {Branch: "feat/x-slice-1"},
		2: {Branch: "feat/x-slice-2"},
		3: {Branch: "feat/x-slice-3"},
	}
	exists := func(b string) bool { return b != "feat/x-slice-3" }
	set := &ChangeRequestSet{Requests: []ChangeRequest{
		{Number: 10, State: "merged", Branch: "feat/x-slice-1"},
		{Number: 11, State: "open", Branch: "feat/x-slice-2", ReviewDecision: "approved"},
	}}
	got := SliceStatuses(plan, bindings, exists, set)
	var states []string
	for _, s := range got {
		states = append(states, s.State)
	}
	if want := []string{"merged", "open", "not started", "not started"}; !reflect.DeepEqual(states, want) {
		t.Fatalf("states = %v, want %v (a binding whose branch is gone is not started; a branch name alone infers nothing)", states, want)
	}
	if got[1].PR == nil || got[1].PR.Number != 11 {
		t.Fatalf("open slice must carry its request: %+v", got[1])
	}
	noPR := SliceStatuses(plan, bindings, func(string) bool { return true }, nil)
	if noPR[0].State != "branch exists" {
		t.Fatalf("a branch without a known request is just a branch: %q", noPR[0].State)
	}
}

func TestSliceStatuses_MergedSliceReadsMergedEvenWhenItsBranchIsGone(t *testing.T) {
	plan := ParseSlices("## Slice 1: A\ncontract: x\n- [x] a\n")
	bindings := map[int]SliceBinding{1: {Branch: "feat/x-slice-1"}}
	set := &ChangeRequestSet{Requests: []ChangeRequest{{Number: 10, State: "merged", Branch: "feat/x-slice-1"}}}
	got := SliceStatuses(plan, bindings, func(string) bool { return false }, set)
	if got[0].State != "merged" || got[0].PR == nil {
		t.Fatalf("a merged slice whose branch was deleted must read merged: %+v", got[0])
	}
	none := SliceStatuses(plan, bindings, func(string) bool { return false }, &ChangeRequestSet{})
	if none[0].State != "not started" {
		t.Fatalf("a stale binding with no request and no branch is not started: %q", none[0].State)
	}
}

func TestSliceBindings_AreSharedByEveryWorktreeOfTheChange(t *testing.T) {
	repo, changeDir := sliceRepo(t)
	linked := filepath.Join(filepath.Dir(repo), "app-wip")
	runGitS(t, repo, "worktree", "add", "-q", "-b", "wip", linked, "main")
	linkedChange := filepath.Join(linked, "openspec", "changes", "big-thing")
	if err := os.MkdirAll(linkedChange, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SaveSliceBinding(linkedChange, 1, SliceBinding{Branch: "feat/b1"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSliceBindings(changeDir)
	if err != nil || got[1].Branch != "feat/b1" {
		t.Fatalf("the main checkout must see a binding written from a linked worktree: %+v %v", got, err)
	}
	if _, statErr := os.Stat(filepath.Join(linkedChange, ".specsync", "slices.json")); statErr == nil {
		t.Fatal("bindings must not be stored inside one worktree")
	}
}
