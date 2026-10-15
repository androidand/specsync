package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

// releasePlanCmd prints the read-only follow-up report for a revision range.
type releasePlanCmd struct {
	openspec                *string
	storeFlag               *string
	since                   *string
	until                   *string
	asJSON                  *bool
	failOnArchiveCandidates *bool
	archiveCompleted        *bool
	apply                   *bool
}

func init() { Register(&releasePlanCmd{}) }

func (*releasePlanCmd) Spec() Spec {
	return Spec{
		Name:     "release-plan",
		Group:    GroupReview,
		Summary:  "Inspect release impact and generate release notes.",
		Workflow: AgentCommandWorkflow{Position: "release", RelatedBefore: []string{"changelog"}},
		Safety: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync release-plan",
			"specsync release-plan --since v0.9.0",
			"specsync release-plan -json",
		},
	}
}

func (cm *releasePlanCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.since = fs.String("since", "", "range start (default: latest tag)")
	cm.until = fs.String("until", "", "range end (default: HEAD)")
	cm.asJSON = fs.Bool("json", false, "emit JSON")
	cm.failOnArchiveCandidates = fs.Bool("fail-on-archive-candidates", false, "exit non-zero when shipped completed changes remain unarchived")
	cm.archiveCompleted = fs.Bool("archive-completed", false, "move shipped completed changes from openspec/changes/ to openspec/changes/archive/")
	cm.apply = fs.Bool("apply", false, "perform suggested spec actions (archive completed changes)")
}

func (cm *releasePlanCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *releasePlanCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, since, until, asJSON, failOnArchiveCandidates, archiveCompleted, apply := cm.openspec, cm.storeFlag, cm.since, cm.until, cm.asJSON, cm.failOnArchiveCandidates, cm.archiveCompleted, cm.apply
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	abs := resolveRoot(fs, openspec, storeFlag).Dir
	ctx := context.Background()
	scope := specsync.Scope{Since: *since, Until: *until}

	src := specsync.NewGitCommitSource()
	in, err := specsync.GatherTrace(ctx, abs, src, scope)
	if err != nil {
		fail(err)
	}
	tr := specsync.ResolveTrace(in, scope)

	// Advisory bump: commit signals × OpenSpec requirement deltas.
	os2 := specsync.NewOpenSpecCLI()
	hasBaseline, _ := os2.HasBaseline(ctx)
	var deltas []specsync.OpenSpecDelta
	shipped := nodesOfKind(tr, specsync.NodeChange)
	statusBySlug := openSpecStatus(ctx, os2)
	for _, n := range shipped {
		slug := strings.TrimPrefix(n.ID, "change:")
		if d, err := os2.Deltas(ctx, slug); err == nil {
			deltas = append(deltas, d...)
		}
	}
	impact := specsync.InferImpact(in.Commits, deltas, hasBaseline, nil)
	archiveCandidates := completedShipped(shipped, statusBySlug)
	if err := archiveHygieneError(archiveCandidates, *failOnArchiveCandidates); err != nil {
		fail(err)
	}

	tool := specsync.DetectReleaseTool(filepath.Dir(abs))
	rng := rangeLabel(*since, *until)

	if *asJSON {
		emitJSON(map[string]any{
			"range": rng, "trace": tr, "bump": impact.Impact.String(),
			"reasons": impact.Reasons, "releaseTool": tool,
			"archiveCandidates": archiveCandidates,
		})
		return
	}

	fmt.Printf("Follow-up  (%s)\n\n", rng)
	fmt.Println("Shipped")
	for _, n := range shipped {
		fmt.Printf("  %-32s %s\n", strings.TrimPrefix(n.ID, "change:"), n.Label)
	}
	if len(shipped) == 0 {
		fmt.Println("  (no linked changes in range)")
	}

	if len(tr.Gaps) > 0 {
		fmt.Println("\nLoose ends")
		for _, g := range tr.Gaps {
			fmt.Printf("  %s  %s (%s)\n", g.Kind, g.Subject, g.Reason)
		}
	}

	if len(archiveCandidates) > 0 {
		fmt.Println("\nArchive candidates  (all tasks done)")
		for _, slug := range archiveCandidates {
			fmt.Printf("  %s\n", slug)
		}
	}

	fmt.Printf("\nAdvisory bump   %s%s\n", impact.Impact.String(), nextVersionSuffix(impact))
	if len(impact.Reasons) > 0 {
		fmt.Println("Why")
		for _, r := range impact.Reasons {
			fmt.Printf("  %s\n", r)
		}
	}

	fmt.Println("\nRelease path (detected)")
	fmt.Printf("  tool: %s", tool.Name)
	if len(tool.Owns) > 0 {
		fmt.Printf("  → owns %s", strings.Join(tool.Owns, ", "))
	}
	fmt.Println("\n  specsync defers to it; the bump above is advisory only")

	if *archiveCompleted {
		archived, err := archiveCompletedChanges(abs, archiveCandidates)
		if err != nil {
			fail(err)
		}
		if len(archived) == 0 {
			fmt.Println("\narchive-completed: no changes archived")
		} else {
			fmt.Println("\nArchived changes")
			for _, slug := range archived {
				fmt.Printf("  %s\n", slug)
			}
		}
	}

	if *apply && !*archiveCompleted {
		fmt.Println("\n--apply preview: archive candidates (execution requires -archive-completed)")
		for _, slug := range archiveCandidates {
			fmt.Printf("  openspec archive %s\n", slug)
		}
	}
}
