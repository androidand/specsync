package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

// noteCmd appends a discovery line to the ## Discoveries section of a change.
type noteCmd struct {
	openspec  *string
	storeFlag *string
	dryRun    *bool
}

func init() { Register(&noteCmd{}) }

func (*noteCmd) Spec() Spec {
	return Spec{
		Name:     "note",
		Group:    GroupPlan,
		Summary:  "Append a discovery line to a change's discoveries.md.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "implementation", RelatedAfter: []string{"spinoff"}},
		Safety: []string{
			"Takes positional args: <change> <text>, not a -text flag",
		},
		Examples: []string{
			"specsync note my-change 'Found edge case in X' -dry-run",
			"specsync note my-change 'Found edge case in X'",
		},
	}
}

func (cm *noteCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.dryRun = fs.Bool("dry-run", false, "show what would be written without modifying files")
}

func (cm *noteCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *noteCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, dryRun := cm.openspec, cm.storeFlag, cm.dryRun
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, -1)
	if fs.NArg() < 2 {
		fail(fmt.Errorf("usage: specsync note <change> <text>"))
	}
	changeName, text := fs.Arg(0), fs.Arg(1)

	c, err := specsync.LoadChangeBySlug(resolveRoot(fs, openspec, storeFlag).Dir, changeName)
	if err != nil {
		fail(err)
	}

	discPath := filepath.Join(c.Dir, "discoveries.md")
	existingBytes, _ := os.ReadFile(discPath)
	existing := strings.TrimSpace(string(existingBytes))
	if existing != "" {
		existing += "\n"
	}
	newContent := existing + "- " + text + "\n"

	if *dryRun {
		fmt.Printf("[dry-run] would append to %s:\n%s\n", discPath, newContent)
		return
	}

	if err := os.WriteFile(discPath, []byte(newContent), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("note: appended to %s/discoveries.md\n", changeName)
}
