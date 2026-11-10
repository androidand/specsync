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

// overviewCmd lists all work in all discovered repositories. Read-only.
type overviewCmd struct {
	root, epic, repo, stage, state      *string
	mine, attention, unattached, asJSON *bool
	budget                              *string
	exclude                             stringSlice
}

func init() { Register(&overviewCmd{}) }

func (*overviewCmd) Spec() Spec {
	return Spec{
		Name:     "overview",
		Group:    GroupOverview,
		Summary:  "List all work in all repositories, grouped by epic.",
		Intents:  []string{"what is everything I am working on", "all work across repos", "what needs attention"},
		Workflow: AgentCommandWorkflow{Position: "planning", RelatedBefore: []string{"status"}, RelatedAfter: []string{"status"}},
		Safety: []string{
			"Never modifies a clone, worktree, index, ref or the tracker; it may write a short-lived request cache outside the repo",
			"Looks only under the roots (-root, SPECSYNC_ROOTS, or the folder holding the current clone), two levels deep; nothing is remembered",
		},
		Examples: []string{
			"specsync overview",
			"specsync overview -attention",
			"specsync overview -repo api -attention",
			"specsync overview -epic acme/web#1 -json",
			"specsync overview -root ~/dev -unattached",
		},
	}
}

func (cm *overviewCmd) Flags(fs *flag.FlagSet) {
	cm.root = fs.String("root", "", "folders to look in, separated by the path list separator (default: $SPECSYNC_ROOTS, else the folder holding the current clone)")
	cm.epic = fs.String("epic", "", "only work under this epic: an issue URL, owner/repo#N or #N")
	cm.repo = fs.String("repo", "", "only work in this repository: owner/name or name")
	cm.stage = fs.String("stage", "", "only changes at this stage")
	cm.state = fs.String("state", "", "on (default), fresh to bypass the request cache, or off for local facts only")
	cm.mine = fs.Bool("mine", false, "only work whose request I opened, and local work with no request")
	cm.attention = fs.Bool("attention", false, "only work that needs attention")
	cm.unattached = fs.Bool("unattached", false, "only work under no epic")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.budget = fs.String("budget", "", "advisory size budget for the size flags, e.g. files=15,lines=400,commits=30 (default: each change's budget line, then $SPECSYNC_BUDGET, then built-in limits)")
	fs.Var(&cm.exclude, "exclude", "glob of paths left out of the size counts (repeatable)")
}

func (cm *overviewCmd) Run(_ context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkArgs(fs, args, 0)
	if !validState(*cm.state) {
		return fmt.Errorf("-state must be on, fresh or off, got %q", *cm.state)
	}
	if _, err := specsync.ResolveBudget(*cm.budget, "", os.Getenv(specsync.BudgetEnv)); err != nil {
		return fmt.Errorf("size budget: %w", err)
	}
	var roots []string
	for _, r := range filepath.SplitList(*cm.root) {
		if r = strings.TrimSpace(r); r != "" {
			roots = append(roots, r)
		}
	}
	ov, err := specsync.BuildOverview(context.Background(), specsync.OverviewOptions{
		Workspace: specsync.WorkspaceOptions{Roots: roots},
		Topology: specsync.TopologyOptions{
			LocalOnly:       *cm.state == "off",
			Cache:           requestCache(),
			RefreshRequests: *cm.state == "fresh",
			Budget:          *cm.budget,
			Exclude:         cm.exclude,
		},
		Scope: specsync.OverviewScope{
			Epic: *cm.epic, Repo: *cm.repo, Stage: *cm.stage,
			Mine: *cm.mine, Attention: *cm.attention, Unattached: *cm.unattached,
		},
	})
	if err != nil {
		return err
	}
	if *cm.asJSON {
		emitJSON(ov)
		return nil
	}
	printOverview(ov)
	return nil
}

func printOverview(ov *specsync.Overview) {
	scope := "everything"
	if len(ov.Scope) > 0 {
		scope = strings.Join(ov.Scope, ", ")
	}
	fmt.Printf("scope: %s\nroots: %s\n", scope, strings.Join(ov.Roots, ", "))
	if len(ov.Epics) == 0 && len(ov.NoEpic) == 0 && len(ov.Unlinked) == 0 {
		fmt.Println("\nNo work found.")
	}
	for _, g := range ov.Epics {
		fmt.Printf("\nepic %s  %d of %d known done\n", g.Epic, g.Done, g.Total)
		for _, r := range g.Items {
			printWorkRow(r, "  ", "")
		}
	}
	if len(ov.NoEpic) > 0 {
		fmt.Println("\nunder no epic")
		for _, r := range ov.NoEpic {
			printWorkRow(r, "  ", "")
		}
	}
	if len(ov.Unlinked) > 0 {
		fmt.Println("\nunlinked: a branch with no change")
		for _, r := range ov.Unlinked {
			printWorkRow(r, "  ", "")
		}
	}
	if ov.Incomplete {
		fmt.Println("\nincomplete: some sources were unavailable, so this may not show everything:")
	}
	for _, s := range ov.Status {
		fmt.Println("status:", s)
	}
}

// printWorkRow prints one item: its key, change and stage, then the request,
// local state and attention that are known. mark, when set, follows the key.
func printWorkRow(r specsync.ChangeTopology, indent, mark string) {
	head := specsync.RowKey(r)
	switch {
	case r.NoChange:
		head += "  (no change)"
	default:
		head += "  " + r.Slug + "  " + r.Stage
		if r.Tasks != nil {
			head += fmt.Sprintf("  %d/%d tasks", r.Tasks.Done, r.Tasks.Total)
		}
		if r.Issue != nil && r.Issue.State != "" {
			head += "  issue " + r.Issue.State
		}
	}
	if mark != "" {
		head += "  " + mark
	}
	fmt.Println(indent + head)
	sub := indent + "    "
	if r.PR != nil {
		fmt.Println(sub + requestLine(r.PR))
	}
	if l := r.Local; l != nil {
		line := fmt.Sprintf("local: %d modified, %d untracked; %d ahead", l.Modified, l.Untracked, l.Ahead)
		if l.Base != "" {
			line += fmt.Sprintf("; %d behind %s", l.BehindBase, l.Base)
		}
		if l.VolatilePath {
			line += "; in a temporary folder"
		}
		fmt.Println(sub + line)
		if len(l.Conflicts) > 0 {
			fmt.Println(sub + "would conflict in: " + strings.Join(l.Conflicts, ", "))
		}
	} else if r.Branch != "" && r.Worktree == "" {
		fmt.Println(sub + "branch " + r.Branch + " (no local worktree)")
	}
	if len(r.Attention) > 0 {
		parts := make([]string, len(r.Attention))
		for i, f := range r.Attention {
			parts[i] = attentionDetail(f, r)
		}
		fmt.Println(sub + "attention: " + strings.Join(parts, ", "))
	}
}
