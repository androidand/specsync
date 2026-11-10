package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	specsync "github.com/androidand/specsync"
)

type flowFlags struct {
	openspec *string
	asJSON   *bool
	state    *string
}

func (ff *flowFlags) define(fs *flag.FlagSet) {
	ff.openspec = fs.String("openspec", "openspec", "path to the openspec/ directory")
	ff.asJSON = fs.Bool("json", false, "output as JSON")
	ff.state = fs.String("state", "on", "on (default) reuses pull request data for up to a minute; fresh refetches it; off skips the tracker and pull request lookups: local facts only, offline")
}

// currentPosition reads the current work's facts and applies the repo's flow.
// It runs nothing: the only side effect is status's short-lived request cache.
func (ff *flowFlags) currentPosition(args []string, fs *flag.FlagSet) specsync.FlowPosition {
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)
	if !validState(*ff.state) {
		fail(fmt.Errorf("-state must be on, fresh or off, got %q", *ff.state))
	}
	abs, err := filepath.Abs(*ff.openspec)
	if err != nil {
		fail(err)
	}
	root := filepath.Dir(abs)
	ways, issues := specsync.LoadWays(root)
	if len(issues) > 0 {
		fmt.Fprintf(os.Stderr, "specsync: %s has problems (%s); run specsync doctor config\n", specsync.SpecSyncConfigPath, issues[0])
	}

	ctx := context.Background()
	cwd, _ := os.Getwd()
	topo, err := specsync.BuildTopology(ctx, specsync.TopologyOptions{
		OpenSpecDir:     *ff.openspec,
		Dir:             cwd,
		LocalOnly:       *ff.state == "off",
		Cache:           requestCache(),
		RefreshRequests: *ff.state == "fresh",
	})
	if err != nil {
		fail(err)
	}
	var row *specsync.ChangeTopology
	if len(topo.Changes) == 1 {
		row = &topo.Changes[0]
	}
	facts := specsync.FactsFromRow(row)
	facts.Trusted = specsync.IsTrusted(root)
	dir := cwd
	if row != nil && row.Worktree != "" {
		dir = row.Worktree
	}
	facts.GatesPassed = specsync.GatesPassedAt(ctx, dir, facts.Head)
	return ways.Position(facts)
}

type nextCmd struct{ flowFlags }

func init() { Register(&nextCmd{}) }

func (*nextCmd) Spec() Spec {
	return Spec{
		Name:     "next",
		Group:    GroupOverview,
		Summary:  "Say the next step for your current work, and the command for it.",
		Intents:  []string{"what do I do next", "next step"},
		Workflow: AgentCommandWorkflow{Position: "any", RelatedBefore: []string{"status"}, RelatedAfter: []string{"flow"}},
		Safety:   []string{"Suggests only: never runs the step, creates a file, branch or tracker item, or starts a process; it may write status's short-lived request cache outside the repo"},
		Examples: []string{
			"specsync next",
			"specsync next -json",
			"specsync next -state=off",
		},
	}
}

func (cm *nextCmd) Flags(fs *flag.FlagSet) { cm.define(fs) }

func (cm *nextCmd) Run(_ context.Context, args []string) error {
	pos := cm.currentPosition(args, flagSetFor(cm))
	if *cm.asJSON {
		out := struct {
			Flow     string `json:"flow"`
			Step     string `json:"step,omitempty"`
			Command  string `json:"command,omitempty"`
			Reason   string `json:"reason,omitempty"`
			Complete bool   `json:"complete"`
		}{Flow: pos.Flow, Complete: pos.Complete}
		if pos.Current != nil {
			out.Step, out.Command, out.Reason = pos.Current.Step, pos.Current.Command, pos.Current.Reason
		}
		emitJSON(out)
		return nil
	}
	if pos.Current == nil {
		fmt.Println("Every step of the flow is done.")
		return nil
	}
	fmt.Println(pos.Current.Reason + ".")
	fmt.Println("Next:", pos.Current.Step)
	if pos.Current.Command != "" {
		fmt.Println("  $", pos.Current.Command)
	}
	return nil
}

type flowCmd struct{ flowFlags }

func init() { Register(&flowCmd{}) }

func (*flowCmd) Spec() Spec {
	return Spec{
		Name:     "flow",
		Group:    GroupOverview,
		Summary:  "Show this repo's flow and mark where your current work is in it.",
		Intents:  []string{"show the workflow", "where am I in the flow"},
		Workflow: AgentCommandWorkflow{Position: "any", RelatedBefore: []string{"next"}},
		Safety:   []string{"Read-only: reports the configured steps and the current one; it may write status's short-lived request cache outside the repo"},
		Examples: []string{
			"specsync flow",
			"specsync flow -json",
		},
	}
}

func (cm *flowCmd) Flags(fs *flag.FlagSet) { cm.define(fs) }

func (cm *flowCmd) Run(_ context.Context, args []string) error {
	pos := cm.currentPosition(args, flagSetFor(cm))
	if *cm.asJSON {
		emitJSON(pos)
		return nil
	}
	fmt.Println("flow:", pos.Flow)
	for _, s := range pos.Steps {
		mark := "[ ]"
		suffix := ""
		switch {
		case s.Done:
			mark = "[x]"
		case pos.Current != nil && s.Step == pos.Current.Step:
			mark, suffix = "[>]", "  <- you are here"
		}
		fmt.Printf("  %s %s%s\n", mark, s.Step, suffix)
	}
	return nil
}
