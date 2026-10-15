package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

// TestAgentHelpGolden pins the agent-help output of every command so the
// migration to one Command type per command cannot silently lose or change
// a description. Change the golden file only when the difference is deliberate.
func TestAgentHelpGolden(t *testing.T) {
	cmds := availableCommands()
	all := map[string]AgentCommandHelp{}
	for _, c := range cmds {
		all[c] = commandMetadata[c]
	}
	got, err := json.MarshalIndent(map[string]any{"commands": cmds, "help": all}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	path := filepath.Join("testdata", "agent-help.golden.json")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create it)", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("agent-help output changed; if deliberate, run: go test ./cmd/specsync -run TestAgentHelpGolden -update")
	}
}
