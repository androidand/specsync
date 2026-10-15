package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/androidand/specsync"
)

// pullCmd reads an existing issue and materializes it as a local change
// (issue -> spec). A dry run reads the issue but writes nothing to disk.
type pullCmd struct {
	openspec    *string
	storeFlag   *string
	issue       *string
	change      *string
	repo        *string
	dryRun      *bool
	project     *string
	assignee    *string
	statusMap   *string
	worktree    *bool
	worktreeDir *string
}

func init() { Register(&pullCmd{}) }

func (*pullCmd) Spec() Spec {
	return Spec{
		Name:     "pull",
		Group:    GroupPlan,
		Summary:  "Pull an issue into a local OpenSpec change.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "issue-first-start", RelatedBefore: []string{"changes", "scan"}, RelatedAfter: []string{"sync", "link"}},
		Required: []string{"issue"},
		Safety: []string{
			"Use -dry-run to preview before pulling",
			"Verify the issue number is correct",
			"Work each issue in its own branch (feat/<n>-<slug>) and, ideally, its own worktree — pass -worktree to have pull set both up automatically, rather than accumulating unrelated changes on whatever branch happens to be checked out",
		},
		Examples: []string{
			"specsync pull -issue 42 -dry-run",
			"specsync pull -issue 42",
			"specsync pull -issue 42 -json",
		},
	}
}

func (cm *pullCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.issue = fs.String("issue", "", "issue number to pull into a local change (auto-resolved from branch name like feat/42-change when omitted)")
	cm.change = fs.String("change", "", "change name (default: derived from the issue title)")
	cm.repo = fs.String("repo", "", "source repo as owner/name (default: auto-detect from git remote)")
	cm.dryRun = fs.Bool("dry-run", false, "show what would be written without touching disk")
	cm.project = fs.String("project", "", "target GitHub Projects board as owner/number (default: openspec/specsync.yml board; unset = no board)")
	cm.assignee = fs.String("assignee", "", "board assignee login (default: the acting viewer, \"me\")")
	cm.statusMap = fs.String("status-map", "", "stage→Status overrides as stage=Name pairs, e.g. \"active=In Progress,archived=Done\" (default: $SPECSYNC_STATUS_MAP)")
	cm.worktree = fs.Bool("worktree", false, "create or reuse a worktree and run pull inside it")
	cm.worktreeDir = fs.String("worktree-dir", "", "worktree base directory (default: $SPECSYNC_WORKTREE_DIR or ../worktrees)")
}

func (cm *pullCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *pullCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, issue, change, repo, dryRun, project, assignee, statusMap, worktree, worktreeDir := cm.openspec, cm.storeFlag, cm.issue, cm.change, cm.repo, cm.dryRun, cm.project, cm.assignee, cm.statusMap, cm.worktree, cm.worktreeDir
	if err := deprecatedSlugFlag(args); err != nil {
		fail(err)
	}
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	abs := resolveRoot(fs, openspec, storeFlag).Dir
	repoRoot := filepath.Dir(abs)

	resolvedBoard, err := specsync.ResolveBoard(*project, repoRoot)
	if err != nil {
		fail(err)
	}

	statusMapping, err := parseStatusMapping(*statusMap)
	if err != nil {
		fail(err)
	}

	target := specsync.BoardTarget{
		Owner:         resolvedBoard.Owner,
		Number:        resolvedBoard.Number,
		Assignee:      *assignee,
		StatusMapping: statusMapping,
	}

	if *worktree {
		runPullWithWorktree(*issue, *change, *repo, *dryRun, target, *worktreeDir)
		return
	}

	res, err := specsync.Pull(context.Background(), specsync.PullOptions{
		OpenSpecDir: abs,
		Provider:    makeProvider(*repo, false, "github", ""),
		IssueID:     *issue,
		Slug:        *change,
		DryRun:      *dryRun,
		Project:     target,
	})
	if err != nil {
		fail(err)
	}

	dest := filepath.Join("openspec", "changes", res.Slug)
	if *dryRun {
		fmt.Printf("DRY RUN — would write %s from issue %s\n\n", dest, res.IssueID)
		printPreview("proposal.md", res.Proposal)
		if res.Tasks != "" {
			printPreview("tasks.md", res.Tasks)
		}
		if res.MarkerPresent {
			fmt.Printf("\nissue %s already carries the marker %s (no edit needed)\n", res.IssueID, res.Marker)
		} else {
			fmt.Printf("\nwould add marker to issue %s body: %s\n", res.IssueID, res.Marker)
		}
		if res.TitleSuggestion != "" {
			fmt.Printf("\ntitle could be tighter: %q — edit the proposal.md H1 if you agree\n", res.TitleSuggestion)
		}
		if res.BoardConfigured {
			printBoardPlan(res.Board, true)
		}
		return
	}
	// Record where the issue came from, so a change living in a store still
	// knows its target repo when the working directory no longer implies one.
	if prov := makeProvider(*repo, false, "github", ""); prov != nil {
		if gp, ok := prov.(*specsync.GitHubProvider); ok {
			if key := gp.Name(); specsync.RepoFromProviderKey(key) != "" {
				if err := specsync.WriteChangeTargets(filepath.Join(abs, "changes", res.Slug), []string{key}); err != nil {
					fmt.Fprintf(os.Stderr, "specsync: could not record target: %v\n", err)
				}
			}
		}
	}

	fmt.Printf("specsync: pulled issue %s -> %s\n", res.IssueID, dest)
	fmt.Println("  + proposal.md")
	fmt.Println("  + specsync.yml (target)")
	if res.Tasks != "" {
		fmt.Println("  + tasks.md")
	}
	if res.TitleSuggestion != "" {
		fmt.Printf("  title could be tighter: %q — edit the proposal.md H1 if you agree\n", res.TitleSuggestion)
	}
	if res.BoardConfigured {
		printBoardPlan(res.Board, false)
	}
}
