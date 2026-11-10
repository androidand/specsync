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
	root, state   *string
	apply, asJSON *bool
	sessions      sessionFlags
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
			"Never removes a dirty tree, unpushed commits, an open request, the current worktree, the main checkout, or a worktree with a live session; each candidate is checked again just before removal",
			"Configured cleanup hooks run only when the repo's config is trusted (specsync trust)",
			"Never deletes anything on the tracker; archiving a change closes nothing that is not already closed",
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
		Apply:         *cm.apply,
		Current:       cwd,
		ArchiveChange: archiveFinishedChange,
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

func archiveFinishedChange(ctx context.Context, row specsync.ChangeTopology) (string, error) {
	if row.Repo == "" {
		return "", fmt.Errorf("no repository known for the change")
	}
	res, err := specsync.Archive(ctx, specsync.ArchiveOptions{
		OpenSpecDir: filepath.Join(row.Store, "openspec"),
		Slug:        row.Slug,
		Provider:    specsync.NewGitHubProviderWithRepo(row.Repo),
	})
	if err != nil {
		return "", err
	}
	if res.UncheckedTasks > 0 {
		return "", fmt.Errorf("%d unchecked task(s); archive it yourself with specsync archive (-force or -reason not-planned)", res.UncheckedTasks)
	}
	return "archived change " + row.Slug, nil
}

func printCleanup(rep *specsync.CleanupReport) {
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
