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
			{Name: "change", Description: "Change slug; omit it inside the change's worktree to use the one the branch names"},
			{Name: "stage", Description: "New stage (backlog, active, blocked, in-review, complete, archived, auto)", Required: true},
		},
		Safety: []string{
			"Use 'auto' to let specsync derive stage from tasks",
			"Omitting the change uses the one the current branch names (feat/<issue>-<slug> or <prefix>/<slug>); on the default branch nothing resolves",
		},
		Examples: []string{
			"specsync set-stage my-change active",
			"specsync set-stage my-change blocked",
			"specsync set-stage my-change complete",
			"specsync set-stage active",
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
	if fs.NArg() < 1 || fs.NArg() > 2 {
		fail(fmt.Errorf("usage: specsync set-stage [<change>] <stage|auto>  (change defaults to the one the current branch names)"))
	}
	root := resolveRoot(fs, openspec, storeFlag).Dir
	changeName, rest, err := splitChangeArgs("set-stage", root, fs.Args(), 1)
	if err != nil {
		fail(err)
	}
	stage := rest[0]

	change := mutableChange(root, changeName, false)
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
