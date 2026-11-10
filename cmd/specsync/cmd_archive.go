package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/androidand/specsync"
)

// archiveCmd archives a completed change: final push, close + spec:archived
// label, then retention (move or prune). Refuses without -force when tasks
// are unchecked.
type archiveCmd struct {
	openspec  *string
	storeFlag *string
	change    *string
	repo      *string
	retain    *string
	force     *bool
	dryRun    *bool
}

func init() { Register(&archiveCmd{}) }

func (*archiveCmd) Spec() Spec {
	return Spec{
		Name:     "archive",
		Group:    GroupReview,
		Summary:  "Archive a completed change: final push, close, then retention.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "after-complete", RelatedBefore: []string{"changes", "sync"}, RelatedAfter: []string{"verify"}},
		Required: []string{"change"},
		Safety: []string{
			"Use -dry-run to preview before archiving",
			"Refuses unchecked-task changes unless -force is set",
		},
		Examples: []string{
			"specsync archive -change my-change -dry-run",
			"specsync archive -change my-change -retain move",
		},
	}
}

func (cm *archiveCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.change = fs.String("change", "", "change to archive (required)")
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: auto-detect)")
	cm.retain = fs.String("retain", "", "retention policy: move (keep) or prune (delete)")
	cm.force = fs.Bool("force", false, "archive even when tasks are unchecked")
	cm.dryRun = fs.Bool("dry-run", false, "print the plan without making changes")
}

func (cm *archiveCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *archiveCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, change, repo, retain, force, dryRun := cm.openspec, cm.storeFlag, cm.change, cm.repo, cm.retain, cm.force, cm.dryRun
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)

	if *change == "" {
		fail(fmt.Errorf("archive: -change <slug> is required"))
	}

	abs := resolveRoot(fs, openspec, storeFlag).Dir

	// Build provider.
	var provider specsync.WorkProvider
	if *dryRun {
		provider = specsync.NewGitHubProviderFunc(dryRunner)
	} else {
		p := specsync.NewGitHubProvider()
		if *repo != "" {
			p = specsync.NewGitHubProviderWithRepo(*repo)
		}
		provider = p
	}

	retainPolicy := specsync.RetentionPolicy(*retain)

	result, err := specsync.Archive(context.Background(), specsync.ArchiveOptions{
		OpenSpecDir: abs,
		Slug:        *change,
		Provider:    provider,
		Retain:      retainPolicy,
		Force:       *force,
		DryRun:      *dryRun,
	})
	if err != nil {
		fail(err)
	}

	for _, line := range result.Plan {
		fmt.Println(line)
	}

	if result.UncheckedTasks > 0 && !*force {
		fmt.Fprintln(os.Stderr, "specsync: archive refused — use -force to override")
		os.Exit(1)
	}

	if *dryRun {
		fmt.Println("\nDRY RUN — no files or GitHub calls were modified")
	}
}
