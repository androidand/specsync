package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	specsync "github.com/androidand/specsync"
)

// runTopology prints the change → repo → issue → branch → worktree join
// (see openspec/changes/agent-topology-command). Read-only; every source is
// optional, so a missing openspec/gh/git-worktree source narrows a row
// instead of dropping it. Exits non-zero only when nothing could be resolved
// at all — a command an agent checks before starting work must not fail
// closed just because one source degraded.
func runTopology(args []string) {
	fs := flag.NewFlagSet("topology", flag.ExitOnError)
	openspec, storeFlag := addRootFlags(fs)
	change := fs.String("change", "", "scope to one change (default: every change)")
	all := fs.Bool("all", false, "include archived changes")
	asJSON := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}

	root := resolveRoot(fs, openspec, storeFlag)
	ctx := context.Background()

	topo, err := specsync.BuildTopology(ctx, specsync.TopologyOptions{
		OpenSpecDir: root.Dir,
		Slug:        *change,
		All:         *all,
	})
	if err != nil {
		fail(err)
	}

	if *asJSON {
		emitJSON(topo)
	} else {
		printTopologyTable(*topo)
	}

	if len(topo.Changes) == 0 {
		os.Exit(1)
	}
}

func printTopologyTable(topo specsync.Topology) {
	if len(topo.Changes) == 0 {
		fmt.Println("No changes resolved.")
	} else {
		fmt.Println("SLUG                            STAGE       REPO                       ISSUE       BRANCH                                    WORKTREE")
		fmt.Println("──────────────────────────────  ──────────  ─────────────────────────  ──────────  ────────────────────────────────────────  ────────────────────────────────────────")
		for _, c := range topo.Changes {
			issue := "-"
			if c.Issue != nil {
				issue = c.Issue.ID
				if c.Issue.State != "" {
					issue += " (" + c.Issue.State + ")"
				}
			}
			branch := c.Branch
			if branch == "" {
				branch = "-"
			}
			worktree := c.Worktree
			if worktree == "" {
				worktree = "-"
			}
			repo := c.Repo
			if repo == "" {
				repo = "-"
			}
			fmt.Printf("%-32s %-11s %-27s %-11s %-41s %s\n", c.Slug, c.Stage, repo, issue, branch, worktree)
			if c.Epic != "" {
				fmt.Printf("    ↳ epic: %s\n", c.Epic)
			}
			if len(c.Linked) > 0 {
				fmt.Printf("    ↳ linked: %s\n", joinCommaOr(c.Linked, "-"))
			}
		}
	}
	for _, s := range topo.Status {
		fmt.Println("status:", s)
	}
}

func joinCommaOr(vals []string, empty string) string {
	if len(vals) == 0 {
		return empty
	}
	out := vals[0]
	for _, v := range vals[1:] {
		out += ", " + v
	}
	return out
}
