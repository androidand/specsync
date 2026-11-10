package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/androidand/specsync"
)

// slicesCmd lists a change's declared slices with progress, order, budget and
// derived state.
type slicesCmd struct {
	openspec  *string
	storeFlag *string
	change    *string
	repo      *string
	state     *string
	asJSON    *bool
}

func init() { Register(&slicesCmd{}) }

func (*slicesCmd) Spec() Spec {
	return Spec{
		Name:     "slices",
		Group:    GroupPlan,
		Summary:  "List a change's slices with progress, order, budget and state.",
		Intents:  []string{"break a change into slices", "see each slice's branch and pull request"},
		Workflow: AgentCommandWorkflow{Position: "planning", RelatedBefore: []string{"changes"}, RelatedAfter: []string{"slice", "status"}},
		Safety: []string{
			"Read-only: never writes to disk, git or the tracker",
			"Slices exist only where tasks.md declares a `## Slice <n>: <title>` heading; specsync never proposes or infers them",
		},
		Examples: []string{
			"specsync slices -change my-change",
			"specsync slices -change my-change -json",
		},
	}
}

func (cm *slicesCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.change = fs.String("change", "", "the change (default: the only change that declares slices)")
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	cm.state = fs.String("state", "on", "on asks the tracker for each slice's pull request; off uses local git only")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
}

func (cm *slicesCmd) Run(ctx context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)
	root := resolveRoot(fs, cm.openspec, cm.storeFlag)

	c := findSliceChange(root.Dir, *cm.change)
	plan := specsync.ParseSlices(c.TasksMarkdown)

	type out struct {
		Change string                 `json:"change"`
		Single bool                   `json:"single"`
		Slices []specsync.SliceStatus `json:"slices"`
		Issues []string               `json:"issues,omitempty"`
	}
	res := out{Change: c.Slug, Single: len(plan.Slices) == 0, Issues: plan.Validate()}
	if !res.Single {
		cwd, _ := os.Getwd()
		bindings, err := specsync.LoadSliceBindings(c.Dir)
		if err != nil {
			fail(err)
		}
		var set *specsync.ChangeRequestSet
		if *cm.state != "off" {
			set = sliceRequests(ctx, *cm.repo)
		}
		res.Slices = specsync.SliceStatuses(plan, bindings, func(b string) bool { return specsync.BranchRef(ctx, cwd, b) != "" }, set)
	}

	if *cm.asJSON {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			fail(err)
		}
		fmt.Println(string(data))
		return nil
	}
	if res.Single {
		fmt.Printf("%s is a single slice: tasks.md declares no `## Slice <n>: <title>` headings\n", c.Slug)
		return nil
	}
	fmt.Printf("%s: %d slices\n\n", c.Slug, len(res.Slices))
	for _, s := range res.Slices {
		after := ""
		if len(s.After) > 0 {
			after = fmt.Sprintf("  after %s", specsync.JoinSliceNumbers(s.After))
		}
		budget := ""
		if s.Budget != "" {
			budget = "  budget " + s.Budget
		}
		fmt.Printf("%d  %-13s %d/%d tasks%s%s  %s\n", s.N, s.State, s.Done, s.Tasks, after, budget, s.Title)
		if s.Contract != "" {
			fmt.Printf("     contract: %s\n", s.Contract)
		}
		if s.Branch != "" {
			fmt.Printf("     branch:   %s\n", s.Branch)
		}
		if s.Binding != nil && s.Binding.Worktree != "" {
			fmt.Printf("     worktree: %s\n", s.Binding.Worktree)
		}
		if s.PR != nil {
			fmt.Printf("     pr:       #%d %s%s\n", s.PR.Number, s.PR.State, prDetail(s.PR))
		}
	}
	for _, msg := range res.Issues {
		fmt.Printf("\nproblem: %s\n", msg)
	}
	return nil
}

func prDetail(pr *specsync.ChangeRequest) string {
	var parts []string
	if pr.ReviewDecision != "" {
		parts = append(parts, "review "+strings.ReplaceAll(pr.ReviewDecision, "_", " "))
	}
	if pr.Checks.Fail > 0 {
		parts = append(parts, fmt.Sprintf("%d checks failing", pr.Checks.Fail))
	} else if pr.Checks.Pending > 0 {
		parts = append(parts, fmt.Sprintf("%d checks pending", pr.Checks.Pending))
	} else if pr.Checks.Pass > 0 {
		parts = append(parts, "checks pass")
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// findSliceChange resolves the change a slice command works on: the named one,
// or the only active change that declares slices.
func findSliceChange(openspecDir, slug string) specsync.Change {
	changes, err := specsync.LoadChanges(openspecDir)
	if err != nil {
		fail(err)
	}
	var withSlices []specsync.Change
	for _, c := range changes {
		if c.Archived {
			continue
		}
		if slug != "" && c.Slug == slug {
			return c
		}
		if len(specsync.ParseSlices(c.TasksMarkdown).Slices) > 0 {
			withSlices = append(withSlices, c)
		}
	}
	if slug != "" {
		fail(fmt.Errorf("no active change %q", slug))
	}
	switch len(withSlices) {
	case 1:
		return withSlices[0]
	case 0:
		fail(fmt.Errorf("no change declares slices; pass -change <slug>"))
	}
	names := make([]string, len(withSlices))
	for i, c := range withSlices {
		names[i] = c.Slug
	}
	fail(fmt.Errorf("several changes declare slices (%s); pass -change <slug>", strings.Join(names, ", ")))
	return specsync.Change{}
}

func sliceRequests(ctx context.Context, repo string) *specsync.ChangeRequestSet {
	src, ok := makeProvider(repo, false, "github", "").(specsync.ChangeRequestSource)
	if !ok {
		return nil
	}
	set, err := src.ChangeRequests(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "specsync: pull request state unavailable: %v\n", err)
		return nil
	}
	return &set
}
