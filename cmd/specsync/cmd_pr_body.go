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
	template  *bool
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
			"specsync pr-body -change my-change -template",
		},
	}
}

func (cm *prBodyCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.change = fs.String("change", "", "change slug (required)")
	cm.bodyFile = fs.String("body-file", "", "file whose contents are merged after the reference line")
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	cm.template = fs.Bool("template", false, "render a short body (Summary, Changes, Review focus, Size, Verification, Closes) from the change and the repo's pull request template")
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
	if *cm.template && *bodyFile != "" {
		fail(fmt.Errorf("pr-body: -template renders the whole body; it cannot be combined with -body-file"))
	}

	resolved := resolveRoot(fs, openspec, storeFlag)
	abs := resolved.Dir

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
	var refLine string
	if tp, ok := prov.(specsync.TraceabilityProvider); ok {
		refLine = tp.ReferenceLine(*ref, allComplete)
		if refLine == "" {
			fail(fmt.Errorf("pr-body: %s has no tracker id — cannot emit a reference line", *change))
		}
	} else if ref.ID != "" {
		keyword := "Part of"
		if allComplete {
			keyword = "Closes"
		}
		refLine = fmt.Sprintf("%s #%s", keyword, ref.ID)
	}

	if *cm.template {
		printTemplateBody(ctx, &resolved, c, refLine)
		return
	}
	if refLine != "" {
		fmt.Println(refLine)
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

func printTemplateBody(ctx context.Context, resolved *specsync.ResolvedRoot, c *specsync.Change, refLine string) {
	repoRoot := textRepoRoot(resolved)
	ways, issues := specsync.LoadWays(repoRoot)
	if len(issues) > 0 {
		fmt.Fprintf(os.Stderr, "specsync: %s has problems (%s); run specsync doctor config\n", specsync.SpecSyncConfigPath, issues[0])
	}
	_, tpl, err := specsync.FindPRTemplate(repoRoot, ways.Text)
	if err != nil {
		fail(err)
	}
	cwd, _ := os.Getwd()
	fmt.Print(specsync.RenderPRBody(specsync.PRBodyInput{
		Change: *c, Template: tpl, Reference: refLine, Size: branchSizeLine(ctx, repoTop(cwd)), Text: ways.Text,
	}))
}

// branchSizeLine summarises the checked-out branch against its base, or
// returns "" when there is no base to measure against.
func branchSizeLine(ctx context.Context, dir string) string {
	base, err := specsync.DefaultBase(ctx, dir)
	if err != nil {
		return ""
	}
	size, _, err := specsync.AnalyzeBranch(ctx, dir, base, "HEAD", nil)
	if err != nil || size == nil || size.Files == 0 {
		return ""
	}
	return fmt.Sprintf("%d files, +%d -%d, %d commits", size.Files, size.Additions, size.Deletions, size.Commits)
}
