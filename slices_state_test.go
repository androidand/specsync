package specsync

import (
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
