package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	specsync "github.com/androidand/specsync"
)

// statusCmd prints the change → repo → issue → branch → worktree join
// (see openspec/changes/agent-topology-command). Read-only; every source is
// optional, so a missing openspec/gh/git-worktree source narrows a row
// instead of dropping it. Exits non-zero only when nothing could be resolved
// at all — a command an agent checks before starting work must not fail
// closed just because one source degraded.
type statusCmd struct {
	openspec, change *string
	all, asJSON      *bool
}

func init() { Register(&statusCmd{}) }

func (*statusCmd) Spec() Spec {
	return Spec{
		Name:     "status",
		Group:    GroupOverview,
		Summary:  "Show each change with its issue, branch, worktree and local git state.",
		Workflow: AgentCommandWorkflow{Position: "planning", RelatedAfter: []string{"changes", "relate"}},
		Safety:   []string{"Read-only: never writes to disk or the tracker"},
		Examples: []string{
			"specsync status",
			"specsync status -change my-change -json",
		},
		Confusions: map[string]string{"topology": "it was renamed to status"},
	}
}

func (cm *statusCmd) Flags(fs *flag.FlagSet) {
	cm.openspec = fs.String("openspec", "openspec", "path to the openspec/ directory")
	cm.change = fs.String("change", "", "scope to one change (default: every change)")
	cm.all = fs.Bool("all", false, "include archived changes")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
}

func (cm *statusCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *statusCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, change, all, asJSON := cm.openspec, cm.change, cm.all, cm.asJSON
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)

	ctx := context.Background()

	topo, err := specsync.BuildTopology(ctx, specsync.TopologyOptions{
		OpenSpecDir: *openspec,
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
			if len(c.Attention) > 0 {
				fmt.Printf("    ↳ attention: %s\n", strings.Join(c.Attention, ", "))
			}
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
