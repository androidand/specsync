package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/androidand/specsync"
)

// setStageCmd sets or unsets a change's explicit workflow stage. Only the
// stage field is touched: an explicit priority survives set-stage auto.
type setStageCmd struct {
	openspec  *string
	storeFlag *string
}

func init() { Register(&setStageCmd{}) }

func (*setStageCmd) Spec() Spec {
	return Spec{
		Name:     "set-stage",
		Group:    GroupPlan,
		Summary:  "Set the workflow stage of a change.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "workflow", RelatedBefore: []string{"changes"}, RelatedAfter: []string{"sync"}},
		Args: []Arg{
			{Name: "change", Description: "Change slug", Required: true},
			{Name: "stage", Description: "New stage (backlog, active, blocked, in-review, complete, archived, auto)", Required: true},
		},
		Safety: []string{
			"Use 'auto' to let specsync derive stage from tasks",
		},
		Examples: []string{
			"specsync set-stage my-change active",
			"specsync set-stage my-change blocked",
			"specsync set-stage my-change complete",
		},
	}
}

func (cm *setStageCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
}

func (cm *setStageCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *setStageCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag := cm.openspec, cm.storeFlag
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, -1)
	if fs.NArg() < 2 {
		fail(fmt.Errorf("usage: specsync set-stage <change> <stage|auto>"))
	}
	changeName, stage := fs.Arg(0), fs.Arg(1)

	change := mutableChange(resolveRoot(fs, openspec, storeFlag).Dir, changeName, false)
	meta := changeMetadata(change)

	if stage == "auto" {
		meta.Stage = nil // back to derived state; explicit priority survives
	} else {
		s := specsync.Stage(stage)
		if err := specsync.ValidateStage(s); err != nil {
			fail(err)
		}
		meta.Stage = &s
	}

	if err := specsync.SaveChangeMetadata(change.Dir, meta); err != nil {
		fail(err)
	}
	// Migrate: delete legacy .status file once metadata.json exists.
	statusPath := filepath.Join(change.Dir, ".status")
	_ = os.Remove(statusPath) // best-effort; absence is fine
	fmt.Printf("set-stage: %s → %s\n", changeName, stage)
}
