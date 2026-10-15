package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/androidand/specsync"
)

// auditTasksCmd scans all changes for unchecked tasks and flags mismatches
// where code exists but tasks remain unchecked — the dogfooding failure mode.
type auditTasksCmd struct {
	openspec       *string
	storeFlag      *string
	asJSON         *bool
	failOnMismatch *bool
}

func init() { Register(&auditTasksCmd{}) }

func (*auditTasksCmd) Spec() Spec {
	return Spec{
		Name:     "audit-tasks",
		Group:    GroupOverview,
		Summary:  "Audit that task status matches code changes (dogfooding).",
		Workflow: AgentCommandWorkflow{Position: "validation"},
		Safety: []string{
			"Enforced by CI",
			"Keeps this repo's dogfooding honest",
		},
		Examples: []string{
			"specsync audit-tasks",
			"specsync audit-tasks -json",
		},
	}
}

func (cm *auditTasksCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.failOnMismatch = fs.Bool("fail-on-mismatch", false, "exit non-zero when mismatches exist")
}

func (cm *auditTasksCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *auditTasksCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, asJSON, failOnMismatch := cm.openspec, cm.storeFlag, cm.asJSON, cm.failOnMismatch
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)

	abs := resolveRoot(fs, openspec, storeFlag).Dir

	changes, err := specsync.LoadChanges(abs)
	if err != nil {
		fail(err)
	}

	result := specsync.AuditTasks(changes)

	if *asJSON {
		type findingJSON struct {
			Slug      string `json:"slug"`
			Unchecked int    `json:"unchecked"`
			Total     int    `json:"total"`
			HasCode   bool   `json:"hasCode"`
			CodeRefs  int    `json:"codeRefs"`
			Progress  string `json:"progress"`
			Stage     string `json:"stage"`
		}
		type resultJSON struct {
			Findings   []findingJSON `json:"findings"`
			Mismatches []findingJSON `json:"mismatches"`
		}
		var out resultJSON
		for _, f := range result.Findings {
			out.Findings = append(out.Findings, findingJSON{
				Slug: f.Slug, Unchecked: f.Unchecked, Total: f.Total,
				HasCode: f.HasCode, CodeRefs: f.CodeRefs,
				Progress: f.Progress, Stage: f.Stage,
			})
		}
		for _, f := range result.Mismatches {
			out.Mismatches = append(out.Mismatches, findingJSON{
				Slug: f.Slug, Unchecked: f.Unchecked, Total: f.Total,
				HasCode: f.HasCode, CodeRefs: f.CodeRefs,
				Progress: f.Progress, Stage: f.Stage,
			})
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fail(fmt.Errorf("marshal JSON: %w", err))
		}
		fmt.Println(string(data))
	} else {
		if len(result.Findings) == 0 {
			fmt.Println("No changes with tasks.md found.")
			return
		}
		fmt.Println("SLUG                          UNCHECKED/TOTAL  CODE  PROGRESS        STAGE")
		fmt.Println("────────────────────────────  ───────────────  ────  ──────────────  ─────────────")
		for _, f := range result.Findings {
			code := "-"
			if f.HasCode {
				code = fmt.Sprintf("%d", f.CodeRefs)
			}
			fmt.Printf("%-30s %-18s %-5s %-15s %s\n", f.Slug, fmt.Sprintf("%d/%d", f.Unchecked, f.Total), code, f.Progress, f.Stage)
		}
		if len(result.Mismatches) > 0 {
			fmt.Printf("\n⚠ MISMATCH: %d change(s) have unchecked tasks but code references:\n", len(result.Mismatches))
			for _, f := range result.Mismatches {
				fmt.Printf("  %s: %d/%d unchecked, %d code refs\n", f.Slug, f.Unchecked, f.Total, f.CodeRefs)
			}
		}
		fmt.Printf("\nspecsync audit-tasks: %d changes with tasks, %d mismatches\n",
			len(result.Findings), len(result.Mismatches))
	}

	if *failOnMismatch && result.HasMismatches() {
		fmt.Fprintln(os.Stderr, "specsync: unchecked tasks with code references detected")
		os.Exit(1)
	}
}
