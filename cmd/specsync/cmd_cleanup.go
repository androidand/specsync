package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	specsync "github.com/androidand/specsync"
)

// cleanupCmd lists finished work and, with -apply, removes it.
type cleanupCmd struct {
	root, state                    *string
	apply, asJSON, ignored, assume *bool
	sessions                       sessionFlags
}

func init() { Register(&cleanupCmd{}) }

func (*cleanupCmd) Spec() Spec {
	return Spec{
		Name:     "cleanup",
		Group:    GroupHousekeeping,
		Summary:  "List finished worktrees and, with -apply, remove them and their branches.",
		Intents:  []string{"clean up merged work", "remove finished worktrees", "which worktrees can go"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "cleanup", RelatedBefore: []string{"overview", "status"}},
		Safety: []string{
			"Dry run by default: nothing is removed unless -apply is given",
			"A worktree is a candidate only when its work is provably in the base (its request merged with this head, or the head is in the base) and its tree is clean; a request closed without merging is never a candidate",
			"Never removes a dirty tree, a worktree with ignored files other than common build output (unless -include-ignored), unpushed commits, an open request, a protected branch (main, master, develop, trunk, staging, production, prod, gh-pages, release/*, hotfix/*, cleanup.protect globs, the base or default branch, or the one checked out in the main checkout) or a branch with commits from several authors, the current worktree or the main checkout",
			"Live sessions protect a worktree only when a session provider is configured and answers; otherwise the dry run warns and -apply refuses unless -assume-no-sessions is given",
			"Each candidate is checked again from fresh facts just before it is touched, in this order: hooks (in the worktree; an item whose hooks cannot run because the config is not trusted is skipped), remove the worktree, delete the local branch only if its tip is still the verified head, delete the remote branch; a failed hook leaves the worktree and branch in place",
			"The remote branch is deleted only when its upstream is the same remote and branch name and its tip, read now, is the head being removed",
			"Never archives a change or touches the tracker: when cleanup.archive is true and the issue is closed it prints the specsync archive command as a follow-up",
		},
		Examples: []string{
			"specsync cleanup",
			"specsync cleanup -root ~/dev",
			"specsync cleanup -apply",
			"specsync cleanup -json",
		},
	}
}

func (cm *cleanupCmd) Flags(fs *flag.FlagSet) {
	cm.root = fs.String("root", "", "folders to look in, separated by the path list separator (default: $SPECSYNC_ROOTS, else the folder holding the current clone)")
	cm.state = fs.String("state", "", "on (default), fresh to bypass the request cache, or off for local facts only (then only work whose head is in the base is recognised)")
	cm.ignored = fs.Bool("include-ignored", false, "also remove worktrees whose only extra files are ignored ones (build output such as node_modules, dist, build, target, .next, vendor and __pycache__ never blocks)")
	cm.assume = fs.Bool("assume-no-sessions", false, "with -apply, proceed although live sessions could not be verified (no session provider, -state off, or it failed)")
	cm.apply = fs.Bool("apply", false, "remove the candidates; without it nothing changes")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.sessions = addSessionFlags(fs)
}

func (cm *cleanupCmd) Run(_ context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkArgs(fs, args, 0)
	if !validState(*cm.state) {
		return fmt.Errorf("-state must be on, fresh or off, got %q", *cm.state)
	}
	var roots []string
	for _, r := range filepath.SplitList(*cm.root) {
		if r = strings.TrimSpace(r); r != "" {
			roots = append(roots, r)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	rep, err := specsync.RunCleanup(context.Background(), specsync.CleanupOptions{
		Overview: specsync.OverviewOptions{
			Workspace: specsync.WorkspaceOptions{Roots: roots},
			Topology: specsync.TopologyOptions{
				LocalOnly:       *cm.state == "off",
				Cache:           requestCache(),
				RefreshRequests: *cm.state == "fresh",
			},
			SessionsCmd:     cm.sessions.command(fs),
			SessionsTimeout: *cm.sessions.timeout,
		},
		Apply:            *cm.apply,
		IncludeIgnored:   *cm.ignored,
		Current:          cwd,
		AssumeNoSessions: *cm.assume,
	})
	if err != nil {
		return err
	}
	if *cm.asJSON {
		emitJSON(rep)
	} else {
		printCleanup(rep)
	}
	for _, it := range rep.Items {
		if it.Error != "" {
			os.Exit(1)
		}
	}
	return nil
}

func printCleanup(rep *specsync.CleanupReport) {
	if rep.SessionsNote != "" {
		fmt.Println("warning:", rep.SessionsNote)
		fmt.Println()
	}
	groups := []struct{ status, title string }{
		{specsync.CleanupCandidate, "finished"},
		{specsync.CleanupSkipped, "skipped"},
		{specsync.CleanupAttention, "needs attention (never removed)"},
	}
	any := false
	for _, g := range groups {
		var items []specsync.CleanupItem
		for _, it := range rep.Items {
			if it.Status == g.status {
				items = append(items, it)
			}
		}
		if len(items) == 0 {
			continue
		}
		any = true
		fmt.Printf("%s\n", g.title)
		for _, it := range items {
			label := it.Branch
			if it.Repo != "" {
				label = it.Repo + " " + it.Branch
			}
			fmt.Printf("  %s\n    %s\n    why: %s\n", label, it.Worktree, it.Reason)
			for _, f := range it.FollowUps {
				fmt.Println("    next:", f)
			}
			if it.Status == specsync.CleanupCandidate {
				if rep.Apply {
					for _, d := range it.Done {
						fmt.Println("    done:", d)
					}
					if it.Error != "" {
						fmt.Println("    stopped:", it.Error)
					}
				} else {
					for _, s := range it.Steps {
						fmt.Println("    would:", s)
					}
				}
			}
		}
		fmt.Println()
	}
	if !any {
		fmt.Println("Nothing to clean up.")
	}
	if !rep.Apply {
		for _, it := range rep.Items {
			if it.Status == specsync.CleanupCandidate {
				fmt.Println("dry run: nothing was changed; add -apply to do the steps above")
				break
			}
		}
	}
	for _, s := range rep.Status {
		fmt.Println("status:", s)
	}
}
