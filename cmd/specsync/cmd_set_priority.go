package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"

	"github.com/androidand/specsync"
)

// setPriorityCmd sets a change's priority.
type setPriorityCmd struct {
	openspec  *string
	storeFlag *string
}

func init() { Register(&setPriorityCmd{}) }

func (*setPriorityCmd) Spec() Spec {
	return Spec{
		Name:     "set-priority",
		Group:    GroupPlan,
		Summary:  "Set or unset a change's numeric priority override.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "plan", RelatedBefore: []string{"changes"}, RelatedAfter: []string{"changes"}},
		Safety: []string{
			"Takes positional args: <change> <1-100|unset>, not flags",
		},
		Examples: []string{
			"specsync set-priority my-change 80",
			"specsync set-priority my-change unset",
		},
	}
}

func (cm *setPriorityCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
}

func (cm *setPriorityCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *setPriorityCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag := cm.openspec, cm.storeFlag
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, -1)
	if fs.NArg() < 2 {
		fail(fmt.Errorf("usage: specsync set-priority <change> <1-100|unset>"))
	}
	changeName, priorityArg := fs.Arg(0), fs.Arg(1)

	change := mutableChange(resolveRoot(fs, openspec, storeFlag).Dir, changeName, true)
	meta := changeMetadata(change)

	if priorityArg == "unset" {
		meta.Priority = nil // an explicit stage survives
	} else {
		priority, err := strconv.Atoi(priorityArg)
		if err != nil || priority < 1 || priority > 100 {
			fail(fmt.Errorf("priority must be between 1 and 100; got %s", priorityArg))
		}
		meta.Priority = &priority
	}

	// Only fields already explicit in metadata.json are preserved; a stage
	// derived from tasks or legacy .status is never frozen into an override.
	if err := specsync.SaveChangeMetadata(change.Dir, meta); err != nil {
		fail(err)
	}
	fmt.Printf("set-priority: %s → %s\n", changeName, priorityArg)
}
