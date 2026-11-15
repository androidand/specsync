package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

// syncCmd projects every OpenSpec change into the tracker (spec -> issue).
type syncCmd struct {
	openspec       *string
	store          *string
	change         *string
	all            *bool
	repo           *string
	providerNames  stringSlice
	mcpConfig      *string
	dryRun         *bool
	slices         *bool
	reconcile      *bool
	closeCompleted *bool
	labels         *bool
	force          *bool
	project        *string
	assignee       *string
	statusMap      *string
	specSource     *string
}

func init() { Register(&syncCmd{}) }

func (*syncCmd) Spec() Spec {
	return Spec{
		Name:     "sync",
		Group:    GroupSync,
		Summary:  "Synchronize an OpenSpec change with GitHub Issues.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "after-implementation", RelatedBefore: []string{"scan", "pull", "changes"}, RelatedAfter: []string{"changes", "verify", "release-plan"}},
		Safety: []string{
			"Always use -dry-run first to preview changes",
			"Pass -change for one change or -all for every change; without either a writing sync is refused (-dry-run previews all)",
			"Confirm git remote points to the right repo",
		},
		Confusions: map[string]string{
			"push": "specsync's sync also reconciles tracker state back into tasks.md, so it isn't a one-way push",
		},
		Examples: []string{
			"specsync -dry-run -change my-change",
			"specsync -change my-change",
			"specsync sync -dry-run -change my-change",
			"specsync -dry-run -change my-change -slices",
		},
	}
}

func (cm *syncCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.store = addRootFlags(fs)
	cm.change = fs.String("change", "", "sync only this change (a writing sync needs -change or -all)")
	cm.all = fs.Bool("all", false, "sync every change in the resolved root (a writing sync needs -change or -all)")
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	fs.Var(&cm.providerNames, "provider", "work provider: github, beads, mcp (repeatable; auto-detect when absent)")
	cm.mcpConfig = fs.String("mcp-config", ".specsync-mcp.json", "path to the MCP provider's config (used only with -provider mcp)")
	cm.dryRun = fs.Bool("dry-run", false, "print the provider commands and rendered body without executing")
	cm.slices = fs.Bool("slices", false, "project each declared slice as a child issue of the change's issue, blocked by the slices it comes after; list them in the issue body when the provider cannot nest issues")
	cm.reconcile = fs.Bool("reconcile", true, "merge external task state back into tasks.md before pushing")
	cm.closeCompleted = fs.Bool("close-completed", false, "close the tracker item once every task in a change is checked")
	cm.labels = fs.Bool("labels", false, "add specsync/stage:<stage> labels to synced issues (off by default: neither is read back by specsync; a sync without this flag also removes them from issues that already carry them)")
	cm.force = fs.Bool("force", false, "write to a closed issue instead of refusing (a closed issue is the strongest signal a cached ref is stale; an ambiguous marker match is never overridden here — use `specsync adopt` to disambiguate)")
	cm.project = fs.String("project", "", "target GitHub Projects board as owner/number (default: openspec/specsync.yml board; unset = no board)")
	cm.assignee = fs.String("assignee", "", "board assignee login (default: the acting viewer, \"me\")")
	cm.statusMap = fs.String("status-map", "", "stage→Status overrides as stage=Name pairs, e.g. \"active=In Progress,archived=Done\" (default: $SPECSYNC_STATUS_MAP)")
	cm.specSource = fs.String("spec", "openspec", "spec source: openspec (default) or beads")
}

func (cm *syncCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *syncCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, store, change, all, repo, mcpConfig, dryRun, reconcile, closeCompleted, labels, force, project, assignee, statusMap, specSource := cm.openspec, cm.store, cm.change, cm.all, cm.repo, cm.mcpConfig, cm.dryRun, cm.reconcile, cm.closeCompleted, cm.labels, cm.force, cm.project, cm.assignee, cm.statusMap, cm.specSource
	if err := deprecatedSlugFlag(args); err != nil {
		fail(err)
	}
	_ = fs.Parse(args)
	providerNames := cm.providerNames
	checkArgs(fs, args, 0)

	root := resolveRoot(fs, openspec, store)
	guardStoreScope(root, *change, *all)
	guardUnscopedSync(*change, *all, *dryRun)
	abs := root.Dir
	repoRoot := filepath.Dir(abs)
	if root.IsStore() {
		fmt.Printf("root: store %s (%s)\n", root.StoreID, abs)
	}

	// Resolve board: -project flag → openspec/specsync.yml → no board.
	resolvedBoard, err := specsync.ResolveBoard(*project, repoRoot)
	if err != nil {
		fail(err)
	}

	// Parse status mapping separately (still needed for BoardTarget).
	statusMapping, err := parseStatusMapping(*statusMap)
	if err != nil {
		fail(err)
	}

	target := specsync.BoardTarget{
		Owner:         resolvedBoard.Owner,
		Number:        resolvedBoard.Number,
		Assignee:      *assignee,
		StatusMapping: statusMapping,
	}

	// A scoped change may declare its target repos. Honour them ahead of
	// git-remote auto-detection, and fan out to one provider per target so a
	// single plan can hold its backend and frontend halves.
	targetRepos := scopedTargetRepos(abs, *change, *repo)
	if len(targetRepos) > 0 && *dryRun {
		fmt.Printf("targets: %s (declared by the change)\n", strings.Join(targetRepos, ", "))
	}

	// Build provider set. When -provider is absent, auto-detect a single one.
	var providers []specsync.WorkProvider
	if len(targetRepos) > 0 && len(providerNames) == 0 {
		if *dryRun {
			fmt.Printf("DRY RUN — nothing is written (issues are only read, to tell a create from an update)\n")
		}
		for _, tr := range targetRepos {
			providers = append(providers, makeProvider(tr, *dryRun, "github", *mcpConfig))
		}
		if *dryRun {
			fmt.Println()
		}
	} else if len(providerNames) == 0 {
		provider, providerReason := detectProvider("", repoRoot)
		prov := makeProvider(*repo, *dryRun, provider, *mcpConfig)
		providers = []specsync.WorkProvider{prov}
		if *dryRun {
			fmt.Printf("DRY RUN — no %s calls are made\n", provider)
			if providerReason != "" {
				fmt.Printf("provider: %s (auto-detected: %s)\n", provider, providerReason)
			}
			if provider == "github" {
				printTarget(prov, *repo)
			}
			fmt.Println()
		} else if providerReason != "" {
			// Auto-detecting away from the github default decides where every
			// item in this run lands; say so on real runs, not just dry-runs.
			fmt.Printf("provider: %s (auto-detected: %s)\n", provider, providerReason)
		}
	} else {
		for _, pn := range providerNames {
			prov := makeProvider(*repo, *dryRun, pn, *mcpConfig)
			providers = append(providers, prov)
		}
		if *dryRun {
			names := strings.Join(providerNames, ", ")
			fmt.Printf("DRY RUN — no %s calls are made\n", names)
			if len(providers) > 0 {
				printTarget(providers[0], *repo)
			}
			fmt.Println()
		}
	}
	for _, prov := range providers {
		if c, ok := prov.(io.Closer); ok {
			defer c.Close()
		}
	}

	// Board refusal: config-declared board on a different repo owner is refused.
	if target.Configured() && *repo == "" {
		// Need the resolved repo to check. Only the first GitHub provider matters.
		for _, prov := range providers {
			if gp, ok := prov.(*specsync.GitHubProvider); ok {
				resolvedRepo, _ := gp.Resolve(context.Background())
				if err := specsync.BoardRefusal(resolvedBoard, resolvedRepo); err != nil {
					fail(err)
				}
				break
			}
		}
	}

	// Board reporting.
	if target.Configured() {
		if *dryRun {
			ruleStr := resolvedBoard.Rule.String()
			fmt.Printf("board: %s/%d via %s (no GraphQL mutations on a dry run)\n\n", target.Owner, target.Number, ruleStr)
		} else {
			ruleStr := resolvedBoard.Rule.String()
			fmt.Printf("board: %s/%d via %s\n", target.Owner, target.Number, ruleStr)
		}
	}

	// Resolve spec source.
	specSrc, err := makeSpecSource(*specSource)
	if err != nil {
		fail(err)
	}

	// Pre-flight summary before any real (non-dry-run) write: a human
	// skimming this line should be able to catch a mismatched target the
	// way "target: acme/app — spec source: ~/dev/specsync/openspec
	// (81 changes)" would have caught the 2026-09-24 incident. Best-effort
	// and openspec-source only (no board/API calls beyond what's already
	// resolved above); a beads spec source or a preflight read failure
	// silently skips the line rather than blocking the sync.
	if !*dryRun && *specSource == "openspec" {
		if allChanges, err := specsync.LoadChanges(abs); err == nil {
			var toSync []specsync.Change
			for _, c := range allChanges {
				if c.Archived {
					continue
				}
				if *change != "" && c.Slug != *change {
					continue
				}
				toSync = append(toSync, c)
			}
			linked := 0
			for _, c := range toSync {
				if refs, err := specsync.LoadRefs(c.Dir); err == nil && len(refs) > 0 {
					linked++
				}
			}
			repoLabel := *repo
			if repoLabel == "" {
				for _, prov := range providers {
					if gp, ok := prov.(*specsync.GitHubProvider); ok {
						if resolved, err := gp.Resolve(context.Background()); err == nil && resolved.Repo != "" {
							repoLabel = resolved.Repo
						}
						break
					}
				}
			}
			if repoLabel == "" {
				repoLabel = "(unresolved)"
			}
			fmt.Printf("target: %s — spec source: %s (%d changes) — %d already linked, %d new\n\n",
				repoLabel, abs, len(toSync), linked, len(toSync)-linked)
		}
	}

	res, err := specsync.Sync(context.Background(), specsync.Options{
		OpenSpecDir:    abs,
		SpecSource:     specSrc,
		Providers:      providers,
		Slug:           *change,
		DryRun:         *dryRun,
		Slices:         *cm.slices,
		Reconcile:      *reconcile,
		CloseCompleted: *closeCompleted,
		Project:        target,
		Labels:         *labels,
		Force:          *force,
		Linker:         buildSyncLinker(*repo, providers),
	})
	if err != nil {
		fail(err)
	}
	if *dryRun && *reconcile {
		fmt.Println("(reconcile applies on a real sync — it merges task state from the issue)")
	}
	fmt.Println()
	sliceFailed := false
	var failed []string // "slug: reason", listed after the per-change output
	for _, it := range res.Items {
		if len(it.Providers) > 0 {
			for _, pr := range it.Providers {
				if pr.Error != nil {
					fmt.Printf("  %-8s %s  (%s) — ERROR: %v\n", "failed", pr.ProviderName, pr.Slug, pr.Error)
					failed = append(failed, fmt.Sprintf("%s [%s]: %v", pr.Slug, pr.ProviderName, pr.Error))
					continue
				}
				if pr.Created && *dryRun {
					fmt.Printf("  %-8s no issue exists yet  [%s] (%s)\n", "NEW", pr.ProviderName, pr.Slug)
				} else {
					verb := "updated"
					if pr.Created {
						verb = "created"
					}
					fmt.Printf("  %-8s %s  [%s] (%s)\n", verb, pr.URL, pr.ProviderName, pr.Slug)
				}
				printPriorArt(pr.PriorArt, pr.Slug, *dryRun)
				if pr.ClosedDeferred != "" {
					fmt.Printf("               • closed state left unchanged (%s)\n", pr.ClosedDeferred)
				}
			}
			// Flips are from the combined reconcile pass.
			for _, f := range it.Flips {
				state := "unchecked"
				if f.Checked {
					state = "checked"
				}
				fmt.Printf("           ↳ reconciled: %s → %s\n", f.Text, state)
			}
		} else {
			if it.Created && *dryRun {
				fmt.Printf("  %-8s no issue exists yet  (%s)\n", "NEW", it.Slug)
			} else {
				verb := "updated"
				if it.Created {
					verb = "created"
				}
				fmt.Printf("  %-8s %s  (%s)\n", verb, it.URL, it.Slug)
			}
			printPriorArt(it.PriorArt, it.Slug, *dryRun)
			for _, f := range it.Flips {
				state := "unchecked"
				if f.Checked {
					state = "checked"
				}
				fmt.Printf("           ↳ reconciled from issue: %s → %s\n", f.Text, state)
			}
			if it.ClosedDeferred != "" {
				fmt.Printf("               • closed state left unchanged (%s)\n", it.ClosedDeferred)
			}
		}
		if it.TitleSuggestion != "" {
			fmt.Printf("           ↳ title could be tighter: %q — edit the proposal.md H1 if you agree\n", it.TitleSuggestion)
		}
		for _, line := range it.Slices {
			fmt.Printf("           ↳ %s\n", line)
		}
		for _, line := range it.SliceFailures {
			fmt.Printf("           ✗ %s\n", line)
			sliceFailed = true
		}
		if it.BoardConfigured {
			printBoardPlan(it.Board, *dryRun)
		}
		if it.ClosedDeferred != "" {
			fmt.Printf("           ↳ closed state left unchanged (%s)\n", it.ClosedDeferred)
		}
	}
	summary := fmt.Sprintf("specsync: %d created, %d updated", res.Created, res.Updated)
	if *dryRun {
		summary = fmt.Sprintf("specsync: %d would be created as new issues, %d updated", res.Created, res.Updated)
	}
	if len(failed) > 0 {
		summary += fmt.Sprintf(", %d failed", len(failed))
	}
	fmt.Println(summary)
	if len(failed) > 0 {
		fmt.Fprintln(os.Stderr, "\nspecsync: these changes did not sync:")
		for _, f := range failed {
			fmt.Fprintf(os.Stderr, "  - %s\n", f)
		}
		os.Exit(1)
	}
	if sliceFailed {
		fmt.Fprintln(os.Stderr, "specsync: some slice steps failed; run again to retry them")
		os.Exit(1)
	}
}

// printPriorArt lists open issues a newly created (or about-to-be-created)
// issue may duplicate, with the adopt command that binds to one instead.
func printPriorArt(matches []specsync.PriorArtMatch, slug string, dryRun bool) {
	if len(matches) == 0 {
		return
	}
	lead := "may already cover this"
	if !dryRun {
		lead = "may have covered this (a new issue was opened)"
	}
	fmt.Printf("           %d open issue(s) %s:\n", len(matches), lead)
	for _, m := range matches {
		fmt.Printf("             #%d  %s  (shared: %s)\n", m.Number, m.Title, m.SharedList())
	}
	fmt.Printf("           → bind instead with:  specsync adopt -issue %d -change %s\n", matches[0].Number, slug)
}
