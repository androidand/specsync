package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/androidand/specsync"
)

// prBodyCmd emits a PR-body fragment for a change: the provider-specific
// reference line (Part of #N / Closes #N), optionally merged with a user-supplied
// body file. Idempotent — re-running does not stack duplicate lines.
type prBodyCmd struct {
	openspec  *string
	storeFlag *string
	change    *string
	bodyFile  *string
	repo      *string
}

func init() { Register(&prBodyCmd{}) }

func (*prBodyCmd) Spec() Spec {
	return Spec{
		Name:     "pr-body",
		Group:    GroupReview,
		Summary:  "Generate the correct PR body fragment for a change.",
		Workflow: AgentCommandWorkflow{Position: "implementation", RelatedBefore: []string{"changes"}},
		Required: []string{"change"},
		Safety: []string{
			"Always include the output in your PR body",
			"Part of or Closes depends on task completion",
		},
		Examples: []string{
			"specsync pr-body -change my-change",
		},
	}
}

func (cm *prBodyCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.change = fs.String("change", "", "change slug (required)")
	cm.bodyFile = fs.String("body-file", "", "file whose contents are merged after the reference line")
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
}

func (cm *prBodyCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *prBodyCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, change, bodyFile, repo := cm.openspec, cm.storeFlag, cm.change, cm.bodyFile, cm.repo
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	if *change == "" {
		fail(fmt.Errorf("pr-body: -change <slug> is required"))
	}

	abs := resolveRoot(fs, openspec, storeFlag).Dir

	c, err := specsync.LoadChangeBySlug(abs, *change)
	if err != nil {
		fail(err)
	}

	// Build a GitHub provider to resolve the repo and read the ref.
	var prov specsync.WorkProvider = specsync.NewGitHubProvider()
	if *repo != "" {
		prov = specsync.NewGitHubProviderWithRepo(*repo)
	}

	// Resolve the synced ref for this change.
	ctx := context.Background()
	ref, err := resolvePRBodyRef(ctx, prov, c)
	if err != nil {
		fail(err)
	}
	if ref == nil {
		fail(fmt.Errorf("pr-body: change %s has no synced tracker item — run `specsync sync -change %s` first", *change, *change))
	}

	// Determine whether all tasks are complete (same predicate as -close-completed).
	allComplete := specsync.TasksComplete(c.TasksMarkdown)

	// Get the reference line from the provider.
	if tp, ok := prov.(specsync.TraceabilityProvider); ok {
		line := tp.ReferenceLine(*ref, allComplete)
		if line == "" {
			fail(fmt.Errorf("pr-body: %s has no tracker id — cannot emit a reference line", *change))
		}
		fmt.Println(line)
	} else {
		// Fallback: GitHub-style default.
		if ref.ID != "" {
			keyword := "Part of"
			if allComplete {
				keyword = "Closes"
			}

			fmt.Printf("%s #%s\n", keyword, ref.ID)
		}
	}

	// Merge with user body if provided.
	if *bodyFile != "" {
		bodyBytes, err := os.ReadFile(*bodyFile)
		if err != nil {
			fail(err)
		}
		userBody := strings.TrimSpace(string(bodyBytes))
		if userBody != "" {
			fmt.Println()
			fmt.Println(ensureNoDuplicateReference(userBody, allComplete, ref.ID))
		}
	}
}
