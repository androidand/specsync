package main

import (
	"context"
	"flag"
	"os"

	"github.com/androidand/specsync"
)

// initCmd creates a scratch openspec/ here and keeps it out of git. It is the
// only way, besides `pull -init`, that specsync creates one.
type initCmd struct {
	dryRun *bool
}

func init() { Register(&initCmd{}) }

func (*initCmd) Spec() Spec {
	return Spec{
		Name:     "init",
		Group:    GroupPlan,
		Summary:  "Create a scratch openspec/ here and keep it out of git.",
		Intents:  []string{"start in a repo with no openspec", "scratch spec", "keep specs out of git"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "issue-first-start", RelatedBefore: []string{}, RelatedAfter: []string{"pull"}},
		Safety: []string{
			"Creates ./openspec/changes/ and adds /openspec/ to the local git exclude file (git rev-parse --git-path info/exclude); never edits a tracked .gitignore",
			"Idempotent; refuses where openspec/ is already tracked",
			"For repos that keep no specs (product repos): init, then `specsync pull -issue N`, work, `specsync -change <slug>`. Never commit the folder",
			"Nothing creates openspec/ unless you run init or pass pull -init",
		},
		Examples: []string{
			"specsync init -dry-run",
			"specsync init",
		},
	}
}

func (cm *initCmd) Flags(fs *flag.FlagSet) {
	cm.dryRun = fs.Bool("dry-run", false, "show what would be created without touching disk")
}

func (cm *initCmd) Run(_ context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := deprecatedSlugFlag(args); err != nil {
		fail(err)
	}
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	cwd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	res, err := specsync.InitScratch(cwd, *cm.dryRun)
	if err != nil {
		fail(err)
	}
	printInit(res, *cm.dryRun, true)
	return nil
}
