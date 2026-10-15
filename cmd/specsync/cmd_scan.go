package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

// scanCmd answers "what already exists here?" for an area before planning.
type scanCmd struct {
	openspec   *string
	storeFlag  *string
	openspecs  *string
	asJSON     *bool
	references *bool
}

func init() { Register(&scanCmd{}) }

func (*scanCmd) Spec() Spec {
	return Spec{
		Name:     "scan",
		Group:    GroupOverview,
		Summary:  "Scan for existing work in a code area or topic.",
		Workflow: AgentCommandWorkflow{Position: "plan", RelatedBefore: []string{"changes"}, RelatedAfter: []string{"pull", "link"}},
		Safety: []string{
			"At least one path or topic word is required",
			"Flags must come before positional arguments",
		},
		Examples: []string{
			"specsync scan cmd/specsync/ 'label creation'",
			"specsync scan cmd/specsync/ -json",
			"specsync scan openspec/changes/ reconcile",
		},
	}
}

func (cm *scanCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.openspecs = fs.String("openspecs", "", "comma-separated list of openspec directories for cross-repo scanning")
	cm.asJSON = fs.Bool("json", false, "emit JSON for a planning agent")
	cm.references = fs.Bool("references", false, "also show OpenSpec references and worksets")
}

func (cm *scanCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *scanCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, openspecs, asJSON, references := cm.openspec, cm.storeFlag, cm.openspecs, cm.asJSON, cm.references
	_ = fs.Parse(args)
	checkArgs(fs, args, -1) // paths + topic are free-form positionals

	paths, topic := splitArea(fs.Args())
	if len(paths) == 0 && topic == "" {
		fail(fmt.Errorf("scan: give an area — one or more paths and/or a topic\nusage: specsync scan <path...> [topic]"))
	}

	defaultRoot := resolveRoot(fs, openspec, storeFlag).Dir

	// Determine if this is a cross-repo scan
	var abs []string
	if *openspecs != "" {
		// Cross-repo scan mode
		dirs := strings.Split(*openspecs, ",")
		for _, dir := range dirs {
			d, err := filepath.Abs(strings.TrimSpace(dir))
			if err != nil {
				fail(err)
			}
			abs = append(abs, d)
		}
	} else {
		abs = append(abs, defaultRoot)
	}

	ctx := context.Background()
	scope := specsync.Scope{Paths: paths, Topic: topic}

	var in specsync.TraceInput
	if len(abs) == 1 {
		// Single-repo scan (original behavior)
		var err error
		in, err = specsync.GatherTrace(ctx, abs[0], specsync.NewGitCommitSource(), scope)
		if err != nil {
			fail(err)
		}
	} else {
		// Cross-repo scan (new behavior)
		var err error
		in, err = specsync.GatherTraceMulti(ctx, abs, specsync.NewGitCommitSource(), scope)
		if err != nil {
			fail(err)
		}
	}

	tr := specsync.ResolveTrace(in, scope)
	looseIssues, issuesNote := openIssuesInArea(ctx, in, topic)

	// Compute cross-repo relationships for multi-repo scans
	var crossRepo map[string][]specsync.ChangeRelationship
	if len(abs) > 1 {
		crossRepo = specsync.CrossRepoCorrelation(in.Changes, scope)
	}

	// Read OpenSpec coordination data if requested.
	var coordination *specsync.Coordination
	var worksets *specsync.Workset
	if *references {
		coordination, _ = specsync.ReadCoordination(ctx) // degrade on error
		worksets, _ = specsync.ReadWorksets(ctx)         // degrade on error
	}

	if *asJSON {
		out := map[string]any{"trace": tr, "openIssuesNoChange": looseIssues, "crossRepo": crossRepo}
		if coordination != nil {
			out["coordination"] = coordination
		}
		if worksets != nil {
			out["worksets"] = worksets
		}
		emitJSON(out)
		return
	}
	label := strings.Join(append(append([]string{}, paths...), quoteIf(topic)...), "  +  ")
	fmt.Printf("Scan  %s\n\n", strings.TrimSpace(label))
	changes := nodesOfKind(tr, specsync.NodeChange)
	if len(changes) == 0 && len(looseIssues) == 0 && len(crossRepo) == 0 {
		fmt.Println("Nothing exists here yet.")
		return
	}
	if len(changes) > 0 {
		fmt.Println("Related changes")
		for _, n := range changes {
			fmt.Printf("  %-32s %s\n", strings.TrimPrefix(n.ID, "change:"), n.Label)
		}
	}

	// Print cross-repo relationships
	if len(crossRepo) > 0 {
		fmt.Println("\nCross-repo relationships")
		for slug, rels := range crossRepo {
			for _, rel := range rels {
				fmt.Printf("  %-32s --[%s]--> %s (%s)\n", slug, rel.Provenance, rel.RelatedChange.Slug, rel.RelatedChange.Title)
			}
		}
	}

	fmt.Println("\nOpen issues here (no linked change)")
	if issuesNote != "" {
		fmt.Printf("  (%s)\n", issuesNote)
	} else if len(looseIssues) == 0 {
		fmt.Println("  (none)")
	}
	for _, it := range looseIssues {
		fmt.Printf("  #%-6s %s\n", it.ID, it.Title)
	}
	if commits := nodesOfKind(tr, specsync.NodeCommit); len(commits) > 0 {
		fmt.Println("\nRecent commits here")
		for _, n := range commits {
			fmt.Printf("  %s\n", n.Label)
		}
	}

	// Print OpenSpec coordination data if requested.
	if coordination != nil {
		siblings := coordination.ReferencedSiblings()
		if len(siblings) > 0 {
			fmt.Println("\nReferenced sibling repos")
			for _, s := range siblings {
				fmt.Printf("  %-12s %s  (source: %s)\n", s.Role, s.Path, s.Source)
			}
		}
	}
	if worksets != nil && len(worksets.Worksets) > 0 {
		fmt.Println("\nWorkset folders")
		for _, w := range worksets.Worksets {
			fmt.Printf("  %-12s %s\n", w.Name, w.Path)
		}
	}
}
