package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/androidand/specsync"
)

// validateCmd checks all change folders for structural issues: required files,
// valid metadata, well-formed stages. Reports all issues in one pass.
type validateCmd struct {
	openspec  *string
	storeFlag *string
	asJSON    *bool
}

func init() { Register(&validateCmd{}) }

func (*validateCmd) Spec() Spec {
	return Spec{
		Name:     "validate",
		Group:    GroupReview,
		Summary:  "Validate OpenSpec change structure.",
		Workflow: AgentCommandWorkflow{Position: "validation"},
		Safety: []string{
			"Read-only operation",
			"Run before syncing",
		},
		Examples: []string{
			"specsync validate",
			"specsync validate -json",
		},
	}
}

func (cm *validateCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.asJSON = fs.Bool("json", false, "output as JSON")
}

func (cm *validateCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *validateCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, asJSON := cm.openspec, cm.storeFlag, cm.asJSON
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)

	abs := resolveRoot(fs, openspec, storeFlag).Dir

	result := specsync.ValidateChanges(abs)

	if *asJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fail(fmt.Errorf("marshal JSON: %w", err))
		}
		fmt.Println(string(data))
	} else {
		if len(result.Issues) == 0 {
			fmt.Println("specsync validate: all changes are structurally valid")
			return
		}
		fmt.Printf("specsync validate: %d issue(s) found\n\n", len(result.Issues))
		for _, issue := range result.Issues {
			prefix := issue.Field
			if issue.Slug != "" {
				prefix = fmt.Sprintf("%s/%s", issue.Slug, issue.Field)
			}
			fmt.Printf("  ❌ %s: %s\n", prefix, issue.Error)
		}
	}

	if len(result.Issues) > 0 {
		os.Exit(1)
	}
}
