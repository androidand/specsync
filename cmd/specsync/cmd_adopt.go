package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/androidand/specsync"
)

// adoptCmd binds an existing local change to an existing tracker issue —
// the missing third verb: pull is issue-first, sync is change-first, adopt
// declares that an already-existing pair are the same work.
type adoptCmd struct {
	openspec  *string
	storeFlag *string
	issue     *string
	change    *string
	repo      *string
	dryRun    *bool
	force     *bool
}

func init() { Register(&adoptCmd{}) }

func (*adoptCmd) Spec() Spec {
	return Spec{
		Name:     "adopt",
		Group:    GroupPlan,
		Summary:  "Bind an existing local change to an existing tracker issue, when neither one knows about the other.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "plan", RelatedBefore: []string{"changes", "scan"}, RelatedAfter: []string{"sync"}},
		Required: []string{"issue"},
		Safety: []string{
			"Never hand-edit the marker into an issue body and hope the next sync finds it — marker discovery goes through search indexing, which lags writes by seconds to minutes, and a sync inside that window creates a duplicate issue instead. Use adopt: it writes the ref cache before touching the issue body, so the link holds immediately.",
			"Use -dry-run to preview before adopting",
			"A change already bound to a different issue, or an issue already marked for a different change, is refused without -force",
		},
		Examples: []string{
			"specsync adopt -issue 3691 -dry-run",
			"specsync adopt -issue 3691 -change my-change",
			"specsync adopt -issue 3691 -change my-change -force",
		},
	}
}

func (cm *adoptCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.issue = fs.String("issue", "", "issue number to bind the change to (required)")
	cm.change = fs.String("change", "", "change slug (default: derived from the current branch name, e.g. feat/42-change)")
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	cm.dryRun = fs.Bool("dry-run", false, "show what would be written without writing anything")
	cm.force = fs.Bool("force", false, "rebind despite an existing, conflicting binding on either side")
}

func (cm *adoptCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *adoptCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, issue, change, repo, dryRun, force := cm.openspec, cm.storeFlag, cm.issue, cm.change, cm.repo, cm.dryRun, cm.force
	if err := deprecatedSlugFlag(args); err != nil {
		fail(err)
	}
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	if *issue == "" {
		fail(fmt.Errorf("-issue is required"))
	}

	abs := resolveRoot(fs, openspec, storeFlag).Dir

	res, err := specsync.Adopt(context.Background(), specsync.AdoptOptions{
		OpenSpecDir: abs,
		Provider:    makeProvider(*repo, false, "github", ""),
		IssueID:     *issue,
		Slug:        *change,
		DryRun:      *dryRun,
		Force:       *force,
	})
	if err != nil {
		fail(err)
	}

	if *dryRun {
		fmt.Printf("DRY RUN — would bind change %q to issue %s (%s)\n", res.Slug, res.IssueID, res.IssueURL)
		fmt.Printf("  would write .specsync/refs.json in %s\n", res.Dir)
		if res.MarkerAdded {
			fmt.Printf("  would add marker to issue %s body\n", res.IssueID)
		} else {
			fmt.Printf("  issue %s already carries the marker (no body edit needed)\n", res.IssueID)
		}
		return
	}

	fmt.Printf("specsync: adopted issue %s -> %s\n", res.IssueURL, res.Slug)
	fmt.Println("  + .specsync/refs.json")
	if res.MarkerAdded {
		fmt.Println("  + marker added to issue body")
	} else {
		fmt.Println("  marker already present on issue body")
	}
}
