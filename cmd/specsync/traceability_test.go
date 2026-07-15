package main

import (
	"os"
	"path/filepath"
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

func TestArchiveCompletedChanges(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	activeDir := filepath.Join(openspecDir, "changes", "done")
	if err := os.MkdirAll(activeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(activeDir, "proposal.md"), []byte("# Done\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	archived, err := archiveCompletedChanges(openspecDir, []string{"done"})
	if err != nil {
		t.Fatalf("archiveCompletedChanges error: %v", err)
	}
	if !reflect.DeepEqual(archived, []string{"done"}) {
		t.Fatalf("archived = %v, want [done]", archived)
	}
	if _, err := os.Stat(filepath.Join(openspecDir, "changes", "archive", "done", "proposal.md")); err != nil {
		t.Fatalf("expected archived proposal.md, got %v", err)
	}
}

func TestArchiveCompletedChangesDestinationExists(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	activeDir := filepath.Join(openspecDir, "changes", "done")
	archiveDir := filepath.Join(openspecDir, "changes", "archive", "done")
	if err := os.MkdirAll(activeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := archiveCompletedChanges(openspecDir, []string{"done"})
	if err == nil {
		t.Fatal("expected destination exists error")
	}
}
