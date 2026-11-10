package main

import (
	"fmt"
	"os"

	specsync "github.com/androidand/specsync"
)

// ConfigDiagnostic is the result of validating openspec/specsync.yml.
type ConfigDiagnostic struct {
	Path     string                 `json:"path"`
	Present  bool                   `json:"present"`
	Flow     string                 `json:"flow"`
	Steps    []string               `json:"steps"`
	Trusted  bool                   `json:"trusted"`
	Commands []string               `json:"commands,omitempty"`
	Issues   []specsync.ConfigIssue `json:"issues,omitempty"`
}

func doctorConfig(asJSON bool) {
	d := configDiagnostic(".")
	if asJSON {
		emitJSON(d)
	} else {
		printConfigDiagnostic(d)
	}
	if len(d.Issues) > 0 {
		os.Exit(1)
	}
}

func configDiagnostic(root string) ConfigDiagnostic {
	ways, issues := specsync.LoadWays(root)
	_, statErr := os.Stat(root + "/" + specsync.SpecSyncConfigPath)
	d := ConfigDiagnostic{
		Path:     specsync.SpecSyncConfigPath,
		Present:  statErr == nil,
		Flow:     ways.Flow,
		Steps:    ways.Steps,
		Trusted:  specsync.IsTrusted(root),
		Commands: ways.GateCommands(true, nil),
		Issues:   issues,
	}
	if d.Flow == "" {
		d.Flow = "custom"
	}
	return d
}

func printConfigDiagnostic(d ConfigDiagnostic) {
	if !d.Present {
		fmt.Printf("%s: not present; defaults apply (flow %s)\n", d.Path, d.Flow)
		return
	}
	fmt.Printf("%s: flow %s (%v)\n", d.Path, d.Flow, d.Steps)
	if len(d.Commands) > 0 {
		state := "not trusted: run specsync trust before any command runs"
		if d.Trusted {
			state = "trusted"
		}
		fmt.Printf("  gate commands: %d, %s\n", len(d.Commands), state)
	}
	if len(d.Issues) == 0 {
		fmt.Println("  config is valid")
		return
	}
	for _, i := range d.Issues {
		fmt.Printf("  %s\n", i)
	}
}
