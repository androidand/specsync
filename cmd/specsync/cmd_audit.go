package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/androidand/specsync"
)

// auditCmd cross-references archived changes against PR state and classifies
// each as unmerged, shipped, or orphaned.
type auditCmd struct {
	openspec       *string
	storeFlag      *string
	repo           *string
	asJSON         *bool
	failOnUnmerged *bool
	markShipped    *bool
}

func init() { Register(&auditCmd{}) }

func (*auditCmd) Spec() Spec {
	return Spec{
		Name:     "audit",
		Group:    GroupOverview,
		Summary:  "Audit OpenSpec changes for structural issues.",
		Workflow: AgentCommandWorkflow{Position: "validation"},
		Safety: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync audit",
			"specsync audit -json",
		},
	}
}

func (cm *auditCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.failOnUnmerged = fs.Bool("fail-on-unmerged", false, "exit non-zero when unmerged changes exist")
	cm.markShipped = fs.Bool("mark-shipped", false, "write shipped stage to .specsync/metadata.json for confirmed merges")
}

func (cm *auditCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *auditCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, repo, asJSON, failOnUnmerged, markShipped := cm.openspec, cm.storeFlag, cm.repo, cm.asJSON, cm.failOnUnmerged, cm.markShipped
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)

	abs := resolveRoot(fs, openspec, storeFlag).Dir

	changes, err := specsync.LoadChanges(abs)
	if err != nil {
		fail(err)
	}

	provider := specsync.NewGitHubProvider()
	if *repo != "" {
		provider = specsync.NewGitHubProviderWithRepo(*repo)
	}

	result := specsync.Audit(context.Background(), provider, changes)

	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "specsync: audit warning: %v\n", e)
	}

	if *markShipped {
		for _, f := range result.Findings {
			if f.Status == "shipped" && f.PR != nil {
				// Find the change dir
				for _, c := range changes {
					if c.Slug == f.Slug && c.Archived {
						stg := specsync.StageShipped
						if err := specsync.SaveChangeMetadata(c.Dir, specsync.ChangeMetadata{
							Version: 1,
							Stage:   &stg,
						}); err != nil {
							fmt.Fprintf(os.Stderr, "specsync: failed to mark %s as shipped: %v\n", f.Slug, err)
						} else {
							fmt.Fprintf(os.Stderr, "specsync: marked %s as shipped\n", f.Slug)
						}
						break
					}
				}
			}
		}
	}

	if *asJSON {
		type prJSON struct {
			Number      int    `json:"number"`
			URL         string `json:"url"`
			Title       string `json:"title"`
			HeadRefName string `json:"headRefName"`
		}
		type findingJSON struct {
			Slug   string  `json:"slug"`
			Status string  `json:"status"`
			PR     *prJSON `json:"pr,omitempty"`
		}
		type resultJSON struct {
			Findings []findingJSON `json:"findings"`
		}
		var out resultJSON
		for _, f := range result.Findings {
			fj := findingJSON{Slug: f.Slug, Status: f.Status}
			if f.PR != nil {
				fj.PR = &prJSON{
					Number:      f.PR.Number,
					URL:         f.PR.URL,
					Title:       f.PR.Title,
					HeadRefName: f.PR.HeadRefName,
				}
			}
			out.Findings = append(out.Findings, fj)
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fail(fmt.Errorf("marshal JSON: %w", err))
		}
		fmt.Println(string(data))
	} else {
		fmt.Println("SLUG                          STATUS    PR")
		fmt.Println("────────────────────────────  ─────────  ──────────────────────────────────────")
		for _, f := range result.Findings {
			pr := "-"
			if f.PR != nil {
				pr = fmt.Sprintf("#%d (%s)", f.PR.Number, f.PR.URL)
			}
			fmt.Printf("%-30s %-10s %s\n", f.Slug, f.Status, pr)
		}
		fmt.Printf("\nspecsync audit: %d unmerged, %d shipped, %d orphaned\n",
			countStatus(result.Findings, "unmerged"),
			countStatus(result.Findings, "shipped"),
			countStatus(result.Findings, "orphaned"),
		)
	}

	if *failOnUnmerged && result.HasUnmerged() {
		fmt.Fprintln(os.Stderr, "specsync: unmerged archived changes detected")
		os.Exit(1)
	}
}
