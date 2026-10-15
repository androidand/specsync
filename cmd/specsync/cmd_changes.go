package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
}

func init() { Register(&changesCmd{}) }

func (*changesCmd) Spec() Spec {
	return Spec{
		Name:     "changes",
		Group:    GroupOverview,
		Summary:  "List all OpenSpec changes with status.",
		Workflow: AgentCommandWorkflow{Position: "planning", RelatedAfter: []string{"pull", "scan", "link"}},
		Safety: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync changes",
			"specsync changes --stage backlog",
			"specsync changes -json",
		},
	}
}

func (cm *changesCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.stages = fs.String("stage", "", "filter by stages (comma-separated, e.g. backlog,blocked)")
	cm.sortBy = fs.String("sort", "stage", "sort order: stage (canonical), priority, or slug")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
}

func (cm *changesCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *changesCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, stages, sortBy, asJSON := cm.openspec, cm.storeFlag, cm.stages, cm.sortBy, cm.asJSON
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
		}

		var results []changeJSON
		for _, c := range filtered {
			total, completed := specsync.CountCheckboxes(c.TasksMarkdown)
			diagnostics := collectDiagnostics(c)
			results = append(results, changeJSON{
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
			})
		}

		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			fail(fmt.Errorf("marshal JSON: %w", err))
		}
		fmt.Println(string(data))
	} else {
		// Table output with stage grouping
		printChangeTable(filtered)
	}
}
