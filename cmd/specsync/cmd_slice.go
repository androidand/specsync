package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/androidand/specsync"
)

// sliceCmd runs slice actions; today only `start`.
type sliceCmd struct {
	openspec    *string
	storeFlag   *string
	change      *string
	base        *string
	worktreeDir *string
	dryRun      *bool
}

func init() { Register(&sliceCmd{}) }

func (*sliceCmd) Spec() Spec {
	return Spec{
		Name:     "slice",
		Group:    GroupPlan,
		Summary:  "Start a slice: create its branch and worktree, stacked on the previous slice.",
		Intents:  []string{"start working on one slice of a change"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "implementation", RelatedBefore: []string{"slices"}, RelatedAfter: []string{"status"}},
		Args: []Arg{
			{Name: "start", Description: "the action; only `start` exists", Required: true},
			{Name: "n", Description: "the slice number", Required: true},
		},
		Safety: []string{
			"Use -dry-run to see the branch, worktree and base first",
			"Creates a branch and a worktree and records the binding under the change's .specsync/ folder; never creates a pull request",
			"A slice with `after` is based on the previous slice's branch; otherwise on -base (default origin/HEAD)",
			"Starting a slice that is already started changes nothing",
		},
		Examples: []string{
			"specsync slice start -change my-change -dry-run 1",
			"specsync slice start -change my-change 2",
		},
	}
}

func (cm *sliceCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.change = fs.String("change", "", "the change (default: the only change that declares slices)")
	cm.base = fs.String("base", "", "base for a slice with no `after` (default: origin/HEAD)")
	cm.worktreeDir = fs.String("worktree-dir", "", "folder for worktrees (default: $SPECSYNC_WORKTREE_DIR or ../worktrees)")
	cm.dryRun = fs.Bool("dry-run", false, "show the branch, worktree and base without creating anything")
}

func (cm *sliceCmd) Run(ctx context.Context, args []string) error {
	const usage = "usage: specsync slice start [-change <slug>] [-dry-run] <n>"
	fs := flagSetFor(cm)
	if len(args) == 0 || args[0] != "start" {
		_ = fs.Parse(args)
		fail(fmt.Errorf("%s", usage))
	}
	args = args[1:]
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 1)
	if fs.NArg() != 1 {
		fail(fmt.Errorf("%s", usage))
	}
	n, err := strconv.Atoi(fs.Arg(0))
	if err != nil || n < 0 {
		fail(fmt.Errorf("slice number %q is not a number", fs.Arg(0)))
	}
	root := resolveRoot(fs, cm.openspec, cm.storeFlag)
	c := findSliceChange(root.Dir, *cm.change)

	top, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		fail(fmt.Errorf("slice start needs a git repository: %w", err))
	}
	wtDir := *cm.worktreeDir
	if wtDir == "" {
		wtDir = os.Getenv("SPECSYNC_WORKTREE_DIR")
	}
	issue := ""
	if ref, ok := specsync.ChangeIssueRef(c.Dir); ok {
		issue = ref.ID
	}
	res, err := specsync.StartSlice(ctx, specsync.StartSliceOptions{
		RepoDir: strings.TrimSpace(string(top)), ChangeDir: c.Dir, Slug: c.Slug, IssueID: issue,
		N: n, WorktreeDir: wtDir, Base: *cm.base, DryRun: *cm.dryRun,
	})
	if err != nil {
		fail(err)
	}
	stack := ""
	if res.Stacked {
		stack = " (stacked on the previous slice)"
	}
	switch {
	case *cm.dryRun:
		fmt.Printf("DRY RUN — would create branch %s from %s%s\n  worktree: %s\n  would record the binding; no pull request is created\n", res.Branch, res.Base, stack, res.Worktree)
	case !res.Created:
		fmt.Printf("specsync: slice %d is already started\n  branch:   %s\n  worktree: %s\n", n, res.Branch, res.Worktree)
	default:
		fmt.Printf("specsync: started slice %d\n  branch:   %s (from %s%s)\n  worktree: %s\n", n, res.Branch, res.Base, stack, res.Worktree)
	}
	return nil
}

func sliceBranchExists(dir, branch string) bool {
	for _, ref := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch} {
		cmd := exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet", ref)
		if cmd.Run() == nil {
			return true
		}
	}
	return false
}
