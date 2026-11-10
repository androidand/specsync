package main

import (
	"context"
	"flag"

	"github.com/androidand/specsync"
)

// traceCmd prints the raw resolved trace graph for a scope (debugging/scripting).
type traceCmd struct {
	openspec  *string
	storeFlag *string
	change    *string
	since     *string
	until     *string
	asJSON    *bool
}

func init() { Register(&traceCmd{}) }

func (*traceCmd) Spec() Spec {
	return Spec{
		Name:     "trace",
		Group:    GroupOverview,
		Summary:  "Trace dependencies and relationships of a change.",
		Workflow: AgentCommandWorkflow{Position: "debugging", RelatedBefore: []string{"changes"}},
		Safety: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync trace",
			"specsync trace -change my-change",
			"specsync trace -json",
		},
	}
}

func (cm *traceCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.change = fs.String("change", "", "scope to a single change slug")
	cm.since = fs.String("since", "", "range start (default: latest tag)")
	cm.until = fs.String("until", "", "range end (default: HEAD)")
	cm.asJSON = fs.Bool("json", false, "emit JSON")
}

func (cm *traceCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *traceCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, change, since, until, asJSON := cm.openspec, cm.storeFlag, cm.change, cm.since, cm.until, cm.asJSON
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	abs := resolveRoot(fs, openspec, storeFlag).Dir
	scope := specsync.Scope{Change: *change, Since: *since, Until: *until}
	tr := resolve(abs, scope)

	if *asJSON {
		emitJSON(tr)
		return
	}
	printTrace(tr)
}
