package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// doctorCmd handles the doctor command.
type doctorCmd struct {
	jsonFlag   *bool
	skipUpdate *bool
}

func init() { Register(&doctorCmd{}) }

func (*doctorCmd) Spec() Spec {
	return Spec{
		Name:     "doctor",
		Group:    GroupHousekeeping,
		Summary:  "Diagnose skill installation, environment, and context health.",
		Workflow: AgentCommandWorkflow{Position: "anytime", RelatedAfter: []string{"install-skill"}},
		Safety: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync doctor",
			"specsync doctor install -json",
			"specsync doctor context -json",
		},
	}
}

func (cm *doctorCmd) Flags(fs *flag.FlagSet) {
	cm.jsonFlag = fs.Bool("json", false, "emit machine-readable JSON output")
	cm.skipUpdate = fs.Bool("skip-skill-update", false, "skip auto-updating skill files")
}

func (cm *doctorCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *doctorCmd) run(args []string) {
	fs := flagSetFor(cm)
	jsonFlag, skipUpdate := cm.jsonFlag, cm.skipUpdate
	if err := fs.Parse(reorderFlagsFirst(args)); err != nil {
		fail(err)
	}

	// Auto-update skills unless --skip-skill-update is set
	if !*skipUpdate && version != "" && version != "dev" {
		if updateSkillsIfNeeded(version) {
			fmt.Fprintf(os.Stderr, "\n")
		}
	}

	remaining := fs.Args()
	subcommand := "default"
	if len(remaining) > 0 {
		subcommand = remaining[0]
	}

	switch subcommand {
	case "default", "claude":
		doctorClaude(*jsonFlag)
	case "install":
		doctorInstall(*jsonFlag)
	case "context":
		doctorContext(*jsonFlag)
	case "skill":
		doctorSkill(*jsonFlag)
	case "store":
		doctorStore(*jsonFlag)
	default:
		fmt.Fprintf(os.Stderr, "specsync doctor: unknown subcommand %q\n", subcommand)
		fmt.Fprintf(os.Stderr, "Available: claude, install, context, skill, store\n")
		os.Exit(2)
	}
}
