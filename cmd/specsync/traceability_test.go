package main

import (
	"reflect"
	"testing"

	"github.com/androidand/specsync"
)

func TestCompletedShipped(t *testing.T) {
	shipped := []specsync.TraceNode{
		{ID: "change:done"},
		{ID: "change:partial"},
		{ID: "change:missing"},
	}
	status := map[string]specsync.OpenSpecChange{
		"done":    {Name: "done", TotalTasks: 3, CompletedTasks: 3},
		"partial": {Name: "partial", TotalTasks: 3, CompletedTasks: 2},
		"missing": {Name: "missing", TotalTasks: 0, CompletedTasks: 0},
	}

	got := completedShipped(shipped, status)
	want := []string{"done"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completedShipped() = %v, want %v", got, want)
	}
}

func TestArchiveHygieneError(t *testing.T) {
	tests := []struct {
		name    string
		cands   []string
		failOn  bool
		wantErr bool
	}{
		{name: "disabled no candidates", cands: nil, failOn: false, wantErr: false},
		{name: "disabled with candidates", cands: []string{"one"}, failOn: false, wantErr: false},
		{name: "enabled no candidates", cands: nil, failOn: true, wantErr: false},
		{name: "enabled with candidates", cands: []string{"one", "two"}, failOn: true, wantErr: true},
	}

	for _, tc := range tests {
		err := archiveHygieneError(tc.cands, tc.failOn)
		if (err != nil) != tc.wantErr {
			t.Fatalf("%s: archiveHygieneError(%v, %v) err=%v wantErr=%v", tc.name, tc.cands, tc.failOn, err, tc.wantErr)
		}
	}
}
