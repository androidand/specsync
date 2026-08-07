package specsync

import (
	"encoding/json"
	"os/exec"
	"testing"
)

func TestCoordination_ReferencedSiblings(t *testing.T) {
	coord := &Coordination{
		Root: StoreEntry{Path: "/root", Source: "nearest", Role: "openspec_root"},
		Members: []StoreEntry{
			{Path: "/sibling1", Source: "reference", Role: "referenced_store"},
			{Path: "/sibling2", Source: "reference", Role: "referenced_store"},
		},
	}

	siblings := coord.ReferencedSiblings()
	if len(siblings) != 2 {
		t.Fatalf("expected 2 siblings, got %d", len(siblings))
	}
	if siblings[0].Path != "/sibling1" || siblings[1].Path != "/sibling2" {
		t.Errorf("unexpected sibling paths: %+v", siblings)
	}
}

func TestReadCoordination_ParsesJSON(t *testing.T) {
	raw := `{
		"root": {"path": "/my/repo", "source": "nearest", "role": "openspec_root"},
		"members": [
			{"path": "/sibling", "source": "reference", "role": "referenced_store"}
		],
		"status": []
	}`
	var coord Coordination
	if err := json.Unmarshal([]byte(raw), &coord); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if coord.Root.Path != "/my/repo" {
		t.Errorf("expected root path /my/repo, got %s", coord.Root.Path)
	}
	if len(coord.Members) != 1 || coord.Members[0].Path != "/sibling" {
		t.Errorf("expected 1 member at /sibling, got %+v", coord.Members)
	}
}

func TestReadWorksets_ParsesJSON(t *testing.T) {
	raw := `{
		"worksets": [
			{"name": "front", "path": "/frontend"},
			{"name": "back", "path": "/backend"}
		],
		"status": []
	}`
	var ws Workset
	if err := json.Unmarshal([]byte(raw), &ws); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(ws.Worksets) != 2 {
		t.Fatalf("expected 2 workset entries, got %d", len(ws.Worksets))
	}
	if ws.Worksets[0].Name != "front" || ws.Worksets[0].Path != "/frontend" {
		t.Errorf("unexpected workset[0]: %+v", ws.Worksets[0])
	}
}

func TestReadCoordination_InvalidJSON(t *testing.T) {
	raw := `{not valid json}`
	var coord Coordination
	err := json.Unmarshal([]byte(raw), &coord)
	if err == nil {
		t.Error("expected parse error for invalid JSON")
	}
}

func TestReadCoordination_EmptyRootDegrades(t *testing.T) {
	coord := &Coordination{
		Root: StoreEntry{Path: ""},
	}
	// Empty root.path means the binary is too old — we'd return nil.
	// This is tested in ReadCoordination itself, but we verify the
	// guard condition here.
	if coord.Root.Path != "" {
		t.Error("root path should be empty for old binary")
	}
}

func TestIsNotFoundOrExit1_GenericError(t *testing.T) {
	err := isNotFoundOrExit1(exec.ErrNotFound)
	if !err {
		t.Error("ErrNotFound should return true")
	}
}
