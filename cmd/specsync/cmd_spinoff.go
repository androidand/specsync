package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/androidand/specsync"
)

// spinoffCmd spawns a new linked change from a discovery or task in an
// existing change.
type spinoffCmd struct {
	openspec  *string
	storeFlag *string
	from      *string
	task      *string
	repo      *string
	kind      *string
	change    *string
	text      *string
	dryRun    *bool
}

func init() { Register(&spinoffCmd{}) }

func (*spinoffCmd) Spec() Spec {
	return Spec{
		Name:     "spinoff",
		Group:    GroupPlan,
		Summary:  "Spin off emergent work from a discovery.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "implementation", RelatedBefore: []string{"changes"}, RelatedAfter: []string{"sync", "link"}},
		Required: []string{"from"},
		Safety: []string{
			"Either -task or -text required",
			"Creates a new change locally",
			"Link the changes with specsync link after",
		},
		Examples: []string{
			"specsync spinoff -from my-change -task 3",
			"specsync spinoff -from my-change -text 'Found edge case in X'",
		},
	}
}

func (cm *spinoffCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.from = fs.String("from", "", "parent change slug (required)")
	cm.task = fs.String("task", "", "task index to spin off (1-based); omit for free text mode")
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: same as parent)")
	cm.kind = fs.String("kind", "", "issue kind: bug, followup, or task (sets label)")
	cm.change = fs.String("change", "", "child change slug (default: derived from text)")
	cm.text = fs.String("text", "", "discovery text (required when -task is not set)")
	cm.dryRun = fs.Bool("dry-run", false, "show what would be written without modifying files")
}

func (cm *spinoffCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *spinoffCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, from, task, repo, kind, change, text, dryRun := cm.openspec, cm.storeFlag, cm.from, cm.task, cm.repo, cm.kind, cm.change, cm.text, cm.dryRun
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)

	if strings.TrimSpace(*from) == "" {
		fail(fmt.Errorf("spinoff: -from <slug> is required"))
	}

	taskIndex := 0
	if *task != "" {
		n, err := strconv.Atoi(*task)
		if err != nil {
			fail(fmt.Errorf("spinoff: invalid task index %q", *task))
		}
		if n < 1 {
			fail(fmt.Errorf("spinoff: task index must be >= 1"))
		}
		taskIndex = n
	}

	// Either -task or -text is required, but not both.
	if taskIndex > 0 && *text != "" {
		fail(fmt.Errorf("spinoff: cannot use both -task and -text"))
	}
	if taskIndex == 0 && *text == "" {
		fail(fmt.Errorf("spinoff: either -task <n> or -text <text> is required"))
	}

	abs := resolveRoot(fs, openspec, storeFlag).Dir

	res, err := specsync.Spinoff(context.Background(), specsync.SpinoffOptions{
		OpenSpecDir: abs,
		Parent:      *from,
		TaskIndex:   taskIndex,
		Text:        *text,
		Slug:        *change,
		Repo:        *repo,
		Kind:        *kind,
		DryRun:      *dryRun,
	})
	if err != nil {
		fail(err)
	}

	if *dryRun {
		fmt.Println("DRY RUN — no files or GitHub calls will be modified")
		fmt.Println()
		fmt.Printf("  new change: %s\n", res.ChildDir)
		fmt.Println()
		fmt.Println("  ┌─ proposal.md ──────────────")
		for _, line := range strings.Split(res.Proposal, "\n") {
			fmt.Println("  │ " + line)
		}
		fmt.Println("  └───────────────────────────")
		if res.ParentTaskN > 0 {
			fmt.Printf("\n  would mark parent task %d as: - [>] moved: %s\n", res.ParentTaskN, res.ChildSlug)
		}
		if res.Label != "" {
			fmt.Printf("\n  child issue label: %s\n", res.Label)
		}
		if res.Linked {
			fmt.Printf("\n  would link %s ↔ %s\n", res.ParentSlug, res.ChildSlug)
		}
		return
	}

	fmt.Printf("specsync spinoff: %s -> %s\n", *from, res.ChildSlug)
	fmt.Println("  + proposal.md")
	fmt.Println("  + tasks.md")
	if res.ParentTaskN > 0 {
		fmt.Printf("  parent task %d marked as moved\n", res.ParentTaskN)
	}
	if res.Linked {
		fmt.Printf("  linked %s ↔ %s\n", res.ParentSlug, res.ChildSlug)
	}
	if res.Label != "" {
		fmt.Printf("  label: %s\n", res.Label)
	}
}
