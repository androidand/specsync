package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/androidand/specsync"
)

// changesCmd lists OpenSpec changes with state and priority.
type changesCmd struct {
	openspec  *string
	storeFlag *string
	stages    *string
	sortBy    *string
	asJSON    *bool
	issues    *bool
	drift     *bool
}

// newIssueStateReader is the provider used by -issues; tests replace it.
var newIssueStateReader = func() specsync.IssueStateReader { return specsync.NewGitHubProvider() }

// issueInfo is what -issues knows about one change's issue.
type issueInfo struct {
	Ref   *specsync.Ref
	State specsync.IssueState
	Drift string
}

func (i issueInfo) number() string {
	if i.Ref == nil {
		return ""
	}
	return "#" + i.Ref.ID
}

func (i issueInfo) stateText() string {
	switch {
	case i.Ref == nil:
		return ""
	case i.State.State == "closed" && i.State.Reason != "":
		return "closed (" + i.State.Reason + ")"
	case i.State.State == "":
		return "unknown"
	}
	return i.State.State
}

// resolveIssueInfo reads each change's cached issue ref and asks the provider
// for all their states in one call. Changes without a cached ref are counted
// in unchecked and never drift. Archived changes are skipped.
func resolveIssueInfo(ctx context.Context, changes []specsync.Change, reader specsync.IssueStateReader) (map[string]issueInfo, int, error) {
	info := make(map[string]issueInfo, len(changes))
	var refs []specsync.Ref
	unchecked := 0
	for _, c := range changes {
		if c.Archived {
			continue
		}
		ref, ok := specsync.ChangeIssueRef(c.Dir)
		if !ok {
			unchecked++
			info[c.Slug] = issueInfo{}
			continue
		}
		info[c.Slug] = issueInfo{Ref: &ref, State: specsync.IssueState{State: "unknown"}}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return info, unchecked, nil
	}
	states, err := reader.IssueStates(ctx, refs)
	if err != nil {
		return nil, 0, err
	}
	for _, c := range changes {
		cur := info[c.Slug]
		if cur.Ref == nil {
			continue
		}
		if st, ok := states[specsync.IssueStateKey(*cur.Ref)]; ok {
			cur.State = st
		}
		cur.Drift = specsync.ChangeDrift(c, cur.State)
		info[c.Slug] = cur
	}
	return info, unchecked, nil
}

func init() { Register(&changesCmd{}) }

func (*changesCmd) Spec() Spec {
	return Spec{
		Name:     "changes",
		Group:    GroupOverview,
		Summary:  "List all OpenSpec changes with status.",
		Workflow: AgentCommandWorkflow{Position: "planning", RelatedAfter: []string{"pull", "scan", "link"}},
		Safety: []string{
			"Read-only operation; only -issues (and -drift) call the provider",
		},
		Examples: []string{
			"specsync changes",
			"specsync changes --stage backlog",
			"specsync changes -json",
			"specsync changes -issues",
			"specsync changes -drift",
		},
	}
}

func (cm *changesCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.stages = fs.String("stage", "", "filter by stages (comma-separated, e.g. backlog,blocked)")
	cm.sortBy = fs.String("sort", "stage", "sort order: stage (canonical), priority, or slug")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.issues = fs.Bool("issues", false, "show each change's issue and its state (one batched provider call) and flag drift: closed-unarchived, complete-open")
	cm.drift = fs.Bool("drift", false, "list only changes whose issue state disagrees with the change (implies -issues) and exit 1 when there are any")
}

func (cm *changesCmd) Run(ctx context.Context, args []string) error {
	cm.run(ctx, args)
	return nil
}

func (cm *changesCmd) run(ctx context.Context, args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, stages, sortBy, asJSON, wantIssues, driftOnly := cm.openspec, cm.storeFlag, cm.stages, cm.sortBy, cm.asJSON, cm.issues, cm.drift
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)

	if *sortBy != "stage" && *sortBy != "priority" && *sortBy != "slug" {
		fail(fmt.Errorf("invalid sort value %q: must be stage, priority, or slug", *sortBy))
	}

	changes, err := specsync.LoadChanges(resolveRoot(fs, openspec, storeFlag).Dir)
	if err != nil {
		fail(err)
	}

	// Filter by stage if specified
	var filtered []specsync.Change
	if *stages != "" {
		stageMap := make(map[string]bool)
		for _, s := range strings.Split(*stages, ",") {
			stageMap[strings.TrimSpace(s)] = true
		}
		for _, c := range changes {
			if stageMap[string(c.Stage)] {
				filtered = append(filtered, c)
			}
		}
	} else {
		filtered = changes
	}

	// Sort
	sortChanges(filtered, *sortBy)

	var issues map[string]issueInfo
	unchecked := 0
	if *wantIssues || *driftOnly {
		var err error
		issues, unchecked, err = resolveIssueInfo(ctx, filtered, newIssueStateReader())
		if err != nil {
			if *driftOnly {
				fmt.Fprintf(os.Stderr, "specsync: read issue states: %v\n", err)
				os.Exit(2)
			}
			fail(fmt.Errorf("read issue states: %w", err))
		}
	}
	drifting := 0
	for _, i := range issues {
		if i.Drift != "" {
			drifting++
		}
	}
	if *driftOnly {
		var only []specsync.Change
		for _, c := range filtered {
			if issues[c.Slug].Drift != "" {
				only = append(only, c)
			}
		}
		filtered = only
	}

	// Output
	if *asJSON {
		type changeJSON struct {
			Slug           string   `json:"slug"`
			Title          string   `json:"title"`
			Stage          string   `json:"stage"`
			CanonicalStage bool     `json:"canonicalStage"`
			StageSource    string   `json:"stageSource"`
			Progress       string   `json:"taskProgress"`
			Priority       *int     `json:"priority"`
			Archived       bool     `json:"archived"`
			CompletedTasks int      `json:"completedTasks"`
			TotalTasks     int      `json:"totalTasks"`
			Diagnostics    []string `json:"diagnostics"`

			Issue            *int   `json:"issue,omitempty"`
			IssueState       string `json:"issueState,omitempty"`
			IssueStateReason string `json:"issueStateReason,omitempty"`
			Drift            string `json:"drift,omitempty"`
		}

		var results []changeJSON
		if *driftOnly {
			results = []changeJSON{}
		}
		for _, c := range filtered {
			total, completed := specsync.CountCheckboxes(c.TasksMarkdown)
			diagnostics := collectDiagnostics(c)
			row := changeJSON{
				Slug:           c.Slug,
				Title:          c.Title,
				Stage:          string(c.Stage),
				CanonicalStage: specsync.IsCanonicalStage(c.Stage),
				StageSource:    string(c.StageSource),
				Progress:       string(c.Progress),
				Priority:       c.Priority,
				Archived:       c.Archived,
				CompletedTasks: completed,
				TotalTasks:     total,
				Diagnostics:    diagnostics,
			}
			if info := issues[c.Slug]; info.Ref != nil {
				if n, err := strconv.Atoi(info.Ref.ID); err == nil {
					row.Issue = &n
				}
				row.IssueState, row.IssueStateReason, row.Drift = info.State.State, info.State.Reason, info.Drift
			}
			results = append(results, row)
		}

		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			fail(fmt.Errorf("marshal JSON: %w", err))
		}
		fmt.Println(string(data))
	} else if !(*driftOnly && drifting == 0) {
		printChangeTable(filtered, issues)
	}

	if issues != nil && unchecked > 0 {
		msg := fmt.Sprintf("%d change(s) have no cached issue and were not checked\n", unchecked)
		if *driftOnly || *asJSON {
			fmt.Fprint(os.Stderr, msg)
		} else {
			fmt.Print("\n" + msg)
		}
	}
	if *driftOnly && drifting > 0 {
		os.Exit(1)
	}
}
