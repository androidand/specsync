// Command specsync projects OpenSpec changes into a work tracker (GitHub today).
// It is a standalone, single-binary tool that works in any OpenSpec project,
// regardless of the project's own language.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/androidand/specsync"
)

// version is the binary version, stamped at release time via -ldflags "-X main.version=...".
var version = "dev"

// knownSubcommands lists every recognized subcommand name. Anything else
// that doesn't start with "-" is an unrecognized bare word, not a flag set
// for the default sync action.
var knownSubcommands = map[string]bool{
	"pull": true, "link": true, "scan": true, "trace": true,
	"release-plan": true, "changelog": true, "install-skill": true,
	"changes": true, "set-stage": true, "set-priority": true, "note": true,
	"sync": true, "audit": true, "audit-tasks": true, "validate": true,
	"spinoff": true,
}

// knownConfusions maps a word someone might reach for by habit (e.g. git's
// "push") to the actual subcommand it's confused with, purely to make the
// unknown-subcommand error more helpful. It is NOT an alias: the word still
// fails to dispatch. Deliberately not an alias — specsync's default action
// reconciles tracker state back into tasks.md before writing out (-reconcile
// defaults to true), so it is not a one-way push the way git's is, and
// "sync" is the more honest name; teach that instead of encoding the
// git-habit word permanently into the tool.
var knownConfusions = map[string]string{
	"push": "sync",
}

// resolveSubcommand decides which subcommand os.Args[1:] selects and returns
// its remaining arguments. A missing first argument, or one starting with
// "-", both select "sync" (bare invocation with flags only) — that keeps
// `specsync -change foo` working. Any other bare word that isn't in
// knownSubcommands is an error: Go's flag package stops parsing at the first
// non-flag argument, so letting an unrecognized word like a typo'd
// subcommand name reach runSync's flag.Parse would silently discard every
// flag after it (including -dry-run) instead of failing loud.
func resolveSubcommand(args []string) (cmd string, rest []string, err error) {
	if len(args) == 0 {
		return "sync", args, nil
	}
	first := args[0]
	if isVersionArg(first) {
		return "version", args[1:], nil
	}
	if knownSubcommands[first] {
		return first, args[1:], nil
	}
	if strings.HasPrefix(first, "-") {
		return "sync", args, nil
	}
	if suggestion, ok := knownConfusions[first]; ok {
		return "", nil, fmt.Errorf("unknown subcommand %q — did you mean %q? specsync's sync also reconciles tracker state back into tasks.md, so it isn't a one-way push", first, suggestion)
	}
	return "", nil, fmt.Errorf("unknown subcommand %q", first)
}

// deprecatedSlugFlag reports an error pointing at -change when the removed
// -slug flag appears in args, in either "-slug value" or "-slug=value" form.
func deprecatedSlugFlag(args []string) error {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
		if name == "slug" || strings.HasPrefix(name, "slug=") {
			return fmt.Errorf("unknown flag %s — did you mean -change?", arg)
		}
	}
	return nil
}

func main() {
	cmd, rest, err := resolveSubcommand(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "specsync: %v\n\nRun with no subcommand (optionally with flags) to sync, or use one of: pull, link, scan, trace, release-plan, changelog, install-skill, changes, set-stage, set-priority, note, audit, audit-tasks, validate, spinoff\n", err)
		os.Exit(2)
	}
	switch cmd {
	case "version":
		fmt.Println("specsync " + version)
	case "pull":
		runPull(rest)
	case "link":
		runLink(rest)
	case "scan":
		runScan(rest)
	case "trace":
		runTrace(rest)
	case "release-plan":
		runReleasePlan(rest)
	case "changelog":
		runChangelog(rest)
	case "install-skill":
		runInstallSkill(rest)
	case "changes":
		runChanges(rest)
	case "set-stage":
		runSetStage(rest)
	case "set-priority":
		runSetPriority(rest)
	case "note":
		runNote(rest)
	case "sync":
		runSync(rest)
	case "audit":
		runAudit(rest)
	case "audit-tasks":
		runAuditTasks(rest)
	case "validate":
		runValidate(rest)
	case "spinoff":
		runSpinoff(rest)
	default:
		runSync(rest)
	}
}

// isVersionArg reports whether the first CLI arg requests the binary version.
func isVersionArg(arg string) bool {
	return arg == "version" || arg == "-version" || arg == "--version"
}

// stringSlice implements flag.Value for repeatable -provider flags.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// runSync projects every OpenSpec change into the tracker (spec -> issue).
func runSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	change := fs.String("change", "", "sync only this change (default: all changes)")
	repo := fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	var providerNames stringSlice
	fs.Var(&providerNames, "provider", "work provider: github, beads (repeatable; auto-detect when absent)")
	dryRun := fs.Bool("dry-run", false, "print the provider commands and rendered body without executing")
	reconcile := fs.Bool("reconcile", true, "merge external task state back into tasks.md before pushing")
	closeCompleted := fs.Bool("close-completed", false, "close the tracker item once every task in a change is checked")
	project := fs.String("project", "", "target GitHub Projects board as owner/number (default: openspec/specsync.yml board; unset = no board)")
	assignee := fs.String("assignee", "", "board assignee login (default: the acting viewer, \"me\")")
	statusMap := fs.String("status-map", "", "stage→Status overrides as stage=Name pairs, e.g. \"active=In Progress,archived=Done\" (default: $SPECSYNC_STATUS_MAP)")
	if err := deprecatedSlugFlag(args); err != nil {
		fail(err)
	}
	_ = fs.Parse(args)

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}
	repoRoot := filepath.Dir(abs)

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

	// Build provider set. When -provider is absent, auto-detect a single one.
	var providers []specsync.WorkProvider
	if len(providerNames) == 0 {
		provider, providerReason := detectProvider("")
		prov := makeProvider(*repo, *dryRun, provider)
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
		}
	} else {
		for _, pn := range providerNames {
			prov := makeProvider(*repo, *dryRun, pn)
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

	res, err := specsync.Sync(context.Background(), specsync.Options{
		OpenSpecDir:    abs,
		Providers:      providers,
		Slug:           *change,
		DryRun:         *dryRun,
		Reconcile:      *reconcile,
		CloseCompleted: *closeCompleted,
		Project:        target,
		Linker:         buildSyncLinker(*repo, providers),
	})
	if err != nil {
		fail(err)
	}
	if *dryRun && *reconcile {
		fmt.Println("(reconcile applies on a real sync — dry-run makes no issue reads)")
	}
	fmt.Println()
	for _, it := range res.Items {
		if len(it.Providers) > 0 {
			for _, pr := range it.Providers {
				if pr.Error != nil {
					fmt.Printf("  %-8s %s  (%s) — ERROR: %v\n", "skip", pr.ProviderName, pr.Slug, pr.Error)
					continue
				}
				verb := "updated"
				if pr.Created {
					verb = "created"
				}
				fmt.Printf("  %-8s %s  [%s] (%s)\n", verb, pr.URL, pr.ProviderName, pr.Slug)
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
			verb := "updated"
			if it.Created {
				verb = "created"
			}
			fmt.Printf("  %-8s %s  (%s)\n", verb, it.URL, it.Slug)
			for _, f := range it.Flips {
				state := "unchecked"
				if f.Checked {
					state = "checked"
				}
				fmt.Printf("           ↳ reconciled from issue: %s → %s\n", f.Text, state)
			}
		}
		if it.TitleSuggestion != "" {
			fmt.Printf("           ↳ title could be tighter: %q — edit the proposal.md H1 if you agree\n", it.TitleSuggestion)
		}
		if it.BoardConfigured {
			printBoardPlan(it.Board, *dryRun)
		}
	}
	fmt.Printf("specsync: %d created, %d updated\n", res.Created, res.Updated)
}

// runPull reads an existing issue and materializes it as a local change
// (issue -> spec). A dry run reads the issue but writes nothing to disk.
func runPull(args []string) {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	issue := fs.String("issue", "", "issue number to pull into a local change (auto-resolved from branch name like feat/42-change when omitted)")
	change := fs.String("change", "", "change name (default: derived from the issue title)")
	repo := fs.String("repo", "", "source repo as owner/name (default: auto-detect from git remote)")
	dryRun := fs.Bool("dry-run", false, "show what would be written without touching disk")
	project := fs.String("project", "", "target GitHub Projects board as owner/number (default: openspec/specsync.yml board; unset = no board)")
	assignee := fs.String("assignee", "", "board assignee login (default: the acting viewer, \"me\")")
	statusMap := fs.String("status-map", "", "stage→Status overrides as stage=Name pairs, e.g. \"active=In Progress,archived=Done\" (default: $SPECSYNC_STATUS_MAP)")
	worktree := fs.Bool("worktree", false, "create or reuse a worktree and run pull inside it")
	worktreeDir := fs.String("worktree-dir", "", "worktree base directory (default: $SPECSYNC_WORKTREE_DIR or ../worktrees)")
	if err := deprecatedSlugFlag(args); err != nil {
		fail(err)
	}
	_ = fs.Parse(args)

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}
	repoRoot := filepath.Dir(abs)

	resolvedBoard, err := specsync.ResolveBoard(*project, repoRoot)
	if err != nil {
		fail(err)
	}

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

	if *worktree {
		runPullWithWorktree(*issue, *change, *repo, *dryRun, target, *worktreeDir)
		return
	}

	res, err := specsync.Pull(context.Background(), specsync.PullOptions{
		OpenSpecDir: abs,
		Provider:    makeProvider(*repo, false, "github"),
		IssueID:     *issue,
		Slug:        *change,
		DryRun:      *dryRun,
		Project:     target,
	})
	if err != nil {
		fail(err)
	}

	dest := filepath.Join("openspec", "changes", res.Slug)
	if *dryRun {
		fmt.Printf("DRY RUN — would write %s from issue %s\n\n", dest, res.IssueID)
		printPreview("proposal.md", res.Proposal)
		if res.Tasks != "" {
			printPreview("tasks.md", res.Tasks)
		}
		if res.MarkerPresent {
			fmt.Printf("\nissue %s already carries the marker %s (no edit needed)\n", res.IssueID, res.Marker)
		} else {
			fmt.Printf("\nwould add marker to issue %s body: %s\n", res.IssueID, res.Marker)
		}
		if res.TitleSuggestion != "" {
			fmt.Printf("\ntitle could be tighter: %q — edit the proposal.md H1 if you agree\n", res.TitleSuggestion)
		}
		if res.BoardConfigured {
			printBoardPlan(res.Board, true)
		}
		return
	}
	fmt.Printf("specsync: pulled issue %s -> %s\n", res.IssueID, dest)
	fmt.Println("  + proposal.md")
	if res.Tasks != "" {
		fmt.Println("  + tasks.md")
	}
	if res.TitleSuggestion != "" {
		fmt.Printf("  title could be tighter: %q — edit the proposal.md H1 if you agree\n", res.TitleSuggestion)
	}
	if res.BoardConfigured {
		printBoardPlan(res.Board, false)
	}
}

// runPullWithWorktree creates or reuses a git worktree, checks out a feature
// branch, and runs the pull operation inside it.
func runPullWithWorktree(issue, change, repo string, dryRun bool, project specsync.BoardTarget, worktreeDir string) {
	ctx := context.Background()

	if worktreeDir == "" {
		worktreeDir = os.Getenv("SPECSYNC_WORKTREE_DIR")
	}
	if worktreeDir == "" {
		worktreeDir = "../worktrees"
	}

	repoName, err := getRepoName(ctx, repo)
	if err != nil {
		fail(fmt.Errorf("worktree: %w", err))
	}

	worktreeName := repoName + "-" + issue
	branchName := "feat/" + issue + "-" + change
	if change == "" {
		branchName = "feat/" + issue
	}
	worktreePath := filepath.Join(worktreeDir, worktreeName)

	if dryRun {
		fmt.Printf("DRY RUN — worktree setup:\n")
		fmt.Printf("  worktree dir: %s\n", worktreePath)
		fmt.Printf("  branch: %s\n", branchName)
		fmt.Printf("  would run: specsync pull -issue %s -change %s\n", issue, change)
		return
	}

	if err := ensureWorktree(ctx, worktreePath, branchName); err != nil {
		fail(fmt.Errorf("worktree: %w", err))
	}

	fmt.Printf("specsync: using worktree %s (branch %s)\n", worktreePath, branchName)

	cwd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	if err := os.Chdir(worktreePath); err != nil {
		fail(err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()

	abs, err := filepath.Abs("openspec")
	if err != nil {
		fail(err)
	}

	res, err := specsync.Pull(ctx, specsync.PullOptions{
		OpenSpecDir: abs,
		Provider:    makeProvider(repo, false, "github"),
		IssueID:     issue,
		Slug:        change,
		DryRun:      dryRun,
		Project:     project,
	})
	if err != nil {
		fail(err)
	}

	dest := filepath.Join("openspec", "changes", res.Slug)
	fmt.Printf("specsync: pulled issue %s -> %s\n", issue, dest)
	fmt.Println("  + proposal.md")
	if res.Tasks != "" {
		fmt.Println("  + tasks.md")
	}
	if res.TitleSuggestion != "" {
		fmt.Printf("  title could be tighter: %q — edit the proposal.md H1 if you agree\n", res.TitleSuggestion)
	}
	if res.BoardConfigured {
		printBoardPlan(res.Board, false)
	}
}

// getRepoName returns the repo name as "owner/name". If repo is provided,
// it's used directly. Otherwise, it's auto-detected from the git remote.
func getRepoName(ctx context.Context, repo string) (string, error) {
	if repo != "" {
		return repo, nil
	}
	out, err := exec.CommandContext(ctx, "git", "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not detect repo from git remote: %w\n%s", err, out)
	}
	url := strings.TrimSpace(string(out))
	url = strings.TrimPrefix(url, "git@")
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "github.com:") {
		url = strings.TrimPrefix(url, "github.com:")
	} else if strings.HasPrefix(url, "https://github.com/") {
		url = strings.TrimPrefix(url, "https://github.com/")
	} else if strings.HasPrefix(url, "ssh://git@github.com/") {
		url = strings.TrimPrefix(url, "ssh://git@github.com/")
	}
	return url, nil
}

// ensureWorktree creates a worktree if it doesn't exist, or reuses an existing one.
func ensureWorktree(ctx context.Context, worktreePath, branchName string) error {
	if _, err := os.Stat(worktreePath); err == nil {
		out, err := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain").CombinedOutput()
		if err != nil {
			return fmt.Errorf("list worktrees: %w\n%s", err, out)
		}
		if strings.Contains(string(out), worktreePath) {
			return nil
		}
	}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		if err := os.MkdirAll(worktreePath, 0755); err != nil {
			return fmt.Errorf("create worktree dir: %w", err)
		}
	}

	out, err := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, worktreePath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create worktree: %w\n%s", err, out)
	}
	return nil
}

// runLink writes links.md for each change (recording the other's issue URL) and
// then syncs each spec so the "## Related" section appears in both GitHub issues.
// Issue references (#N, owner/repo#N, URL) are also accepted — they are linked
// directly without requiring a local spec.
//
// Usage: specsync link [flags] <change1> <change2> [<change3>...]
func runLink(args []string) {
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	dryRun := fs.Bool("dry-run", false, "show what would change without writing files or calling GitHub")
	repo := fs.String("repo", "", "repo (owner/name) for bare issue refs")
	_ = fs.Parse(args)

	args = fs.Args()
	if len(args) < 2 {
		fail(fmt.Errorf("link: at least 2 arguments required\nusage: specsync link <change1> <change2> [<change3>...]"))
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	result, err := specsync.Link(context.Background(), specsync.LinkOptions{
		OpenSpecDir: abs,
		Args:        args,
		Repo:        *repo,
		DryRun:      *dryRun,
	})
	if err != nil {
		fail(err)
	}

	if *dryRun {
		fmt.Println("DRY RUN — no files or GitHub calls will be modified")
		fmt.Println()
		for i, p := range result.Pairs {
			fmt.Printf("  %s/links.md would contain:\n", p.Slug)
			for j, other := range result.Pairs {
				if j != i {
					fmt.Printf("    - %s\n", other.Ref.URL)
				}
			}
			for _, lr := range result.Refs {
				fmt.Printf("    - %s\n", lr.Ref.URL)
			}
			// Render the Related section preview by loading the change and
			// injecting the would-be links directly, bypassing disk.
			c, err := specsync.LoadChange(p.Dir, false, abs)
			if err == nil && c != nil {
				c.Links = nil
				for j, other := range result.Pairs {
					if j != i {
						c.Links = append(c.Links, other.Ref)
					}
				}
				for _, lr := range result.Refs {
					c.Links = append(c.Links, lr.Ref)
				}
				item := specsync.WorkItemFor(*c, false)
				if idx := strings.Index(item.Body, "\n\n## Related\n\n"); idx >= 0 {
					fmt.Printf("\n  Related section in %s issue:\n", p.Slug)
					for _, line := range strings.Split(item.Body[idx+2:], "\n") {
						fmt.Println("    " + line)
					}
				}
			}
			fmt.Println()
		}
		// Reference path: show what each issue would be edited to contain.
		for _, lr := range result.Refs {
			var others []specsync.Ref
			for _, p := range result.Pairs {
				others = append(others, p.Ref)
			}
			for _, other := range result.Refs {
				if other.ID != lr.ID || other.Repo != lr.Repo {
					others = append(others, other.Ref)
				}
			}
			relatedBody := specsync.UpsertRelatedSection("(existing issue body)", others)
			fmt.Printf("  %s would be edited:\n", lr.Ref.URL)
			fmt.Printf("    ## Related section:\n")
			for _, line := range strings.Split(relatedBody, "\n") {
				if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "## Related") {
					fmt.Println("      " + line)
				}
			}
			fmt.Println()
		}

		fmt.Printf("specsync link: would cross-link %d specs and %d issue refs\n", len(result.Pairs), len(result.Refs))
		return
	}

	// Real run: sync each spec with the provider matching its repo.
	for _, p := range result.Pairs {
		provider := makeProvider(p.Repo, false, "github")
		_, err := specsync.Sync(context.Background(), specsync.Options{
			OpenSpecDir: abs,
			Provider:    provider,
			Slug:        p.Slug,
		})
		if err != nil {
			fail(fmt.Errorf("sync %s after link: %w", p.Slug, err))
		}
		fmt.Printf("  linked  %s  <->  %s\n", p.Slug, p.Ref.URL)
	}

	// Reference path: fetch each issue, upsert "## Related", push edited body.
	for _, lr := range result.Refs {
		// Collect "other" refs (all refs except this one).
		var others []specsync.Ref
		for _, p := range result.Pairs {
			others = append(others, p.Ref)
		}
		for _, other := range result.Refs {
			if other.ID != lr.ID || other.Repo != lr.Repo {
				others = append(others, other.Ref)
			}
		}

		provider := makeProvider(lr.Repo, false, "github")
		// Fetch the issue to get existing title, body, and labels.
		reader, ok := provider.(specsync.IssueReader)
		if !ok {
			fail(fmt.Errorf("provider %T does not support reading issues", provider))
		}
		item, err := reader.Get(context.Background(), lr.ID)
		if err != nil {
			fail(fmt.Errorf("fetch issue %s: %w", lr.Ref.URL, err))
		}

		// Upsert the Related section in the body.
		edited := specsync.UpsertRelatedSection(item.Body, others)

		// Push the edited body back, preserving title and labels.
		workItem := specsync.WorkItem{
			Slug:         "",
			Title:        item.Title,
			Body:         edited,
			Labels:       item.Labels,
			ManageClosed: false,
		}
		_, err = provider.Push(context.Background(), workItem, &lr.Ref)
		if err != nil {
			fail(fmt.Errorf("push edited body for %s: %w", lr.Ref.URL, err))
		}
		fmt.Printf("  linked  %s\n", lr.Ref.URL)
	}

	fmt.Printf("specsync link: %d specs and %d refs cross-linked\n", len(result.Pairs), len(result.Refs))
}

// detectProvider returns ("beads", reason) when Beads should be auto-selected,
// or ("github", "") otherwise. Checks (in order): explicit provider flag,
// `bd` on PATH, `.beads/` in working directory.
func detectProvider(provider string) (string, string) {
	if provider != "" {
		return provider, ""
	}
	if _, err := exec.LookPath("bd"); err == nil {
		return "beads", "`bd` found on PATH"
	}
	if _, err := os.Stat(".beads"); err == nil {
		return "beads", ".beads/ found in working directory"
	}
	return "github", ""
}

// buildSyncLinker builds a ChainLinker for sync with discovery resolvers
// in priority order: branch only. The cache is NOT a resolver here — the
// provider loop already reads the cache for each provider. Including the
// cache in the Linker would return a ref from one provider and use it as
// a fallback for another, causing cross-provider ref pollution.
func buildSyncLinker(repo string, providers []specsync.WorkProvider) specsync.Linker {
	var resolvers []specsync.Resolver

	// Branch resolver — slug-aware to prevent resolving all changes to the
	// same issue when syncing multiple changes at once.
	if repo != "" {
		resolvers = append(resolvers, specsync.NewBranchResolver(repo))
	}

	return specsync.NewChainLinker(resolvers...)
}

// makeProvider builds the selected work provider, substituting a dry-runner that
// prints commands instead of executing them when dryRun is set. github
// (default) targets repo (auto-detect when empty); beads drives the local `bd`
// graph and ignores repo.
func makeProvider(repo string, dryRun bool, provider string) specsync.WorkProvider {
	switch provider {
	case "beads":
		if dryRun {
			return specsync.NewBeadsProviderFunc(beadsDryRunner)
		}
		return specsync.NewBeadsProvider()
	default: // github
		if dryRun {
			return specsync.NewGitHubProviderFuncWithRepo(repo, dryRunner)
		}
		if repo != "" {
			return specsync.NewGitHubProviderWithRepo(repo)
		}
		return specsync.NewGitHubProvider()
	}
}

// parseStatusMapping parses "-status-map" (falling back to $SPECSYNC_STATUS_MAP)
// into per-stage Status-name overrides. The format is comma-separated
// stage=Name pairs where stage is active, complete, or archived; Status names
// may contain spaces ("active=In Progress,archived=Done"). Empty yields nil
// (the built-in defaults).
func parseStatusMapping(s string) (map[specsync.Stage]string, error) {
	if strings.TrimSpace(s) == "" {
		s = os.Getenv("SPECSYNC_STATUS_MAP")
	}
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	stages := map[string]specsync.Stage{
		"active":   specsync.StageActive,
		"complete": specsync.StageComplete,
		"archived": specsync.StageArchived,
		"shipped":  specsync.StageShipped,
	}
	mapping := map[specsync.Stage]string{}
	for _, pair := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(pair, "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || k == "" || v == "" {
			return nil, fmt.Errorf("-status-map entry %q must be stage=Name (e.g. \"active=In Progress\")", strings.TrimSpace(pair))
		}
		stage, known := stages[strings.ToLower(k)]
		if !known {
			return nil, fmt.Errorf("-status-map stage %q is unknown; valid stages: active, complete, archived, shipped", k)
		}
		if _, dup := mapping[stage]; dup {
			return nil, fmt.Errorf("-status-map maps stage %q twice", k)
		}
		mapping[stage] = v
	}
	return mapping, nil
}

// printBoardPlan renders the board projection for one change: what happened on a
// real run, or what would happen on a dry run.
func printBoardPlan(plan specsync.BoardPlan, dryRun bool) {
	if dryRun {
		fmt.Println("           ↳ board (dry run):")
		fmt.Println("               • would ensure the issue is on the board")
		if plan.StatusName != "" {
			fmt.Printf("               • would set Status → %s\n", plan.StatusName)
		}
		if plan.AssigneeLogin != "" {
			fmt.Printf("               • would assign → %s\n", plan.AssigneeLogin)
		}
		return
	}
	if plan.AddedToBoard {
		fmt.Println("           ↳ board: added to the board")
	} else if plan.AlreadyOnBoard {
		fmt.Println("           ↳ board: already on the board")
	}
	if plan.StatusName != "" {
		fmt.Printf("               • Status → %s\n", plan.StatusName)
	} else if plan.StatusSkipped != "" {
		fmt.Printf("               • Status left unchanged (%s)\n", plan.StatusSkipped)
	}
	if plan.AssigneeLogin != "" {
		fmt.Printf("               • assigned → %s\n", plan.AssigneeLogin)
	} else if plan.AssignSkipped != "" {
		fmt.Printf("               • assignee left unchanged (%s)\n", plan.AssignSkipped)
	}
}

// printTarget prints the resolved repo name and the rule that selected it,
// replacing the old "auto-detected" message with the concrete value.
func printTarget(prov specsync.WorkProvider, explicitRepo string) {
	gp, ok := prov.(*specsync.GitHubProvider)
	if !ok {
		return
	}
	resolved, _ := gp.Resolve(context.Background())
	if resolved.Repo == "" {
		fmt.Println("target: could not resolve repo (no -repo, no gh default, no origin)")
		return
	}
	rule := resolved.Rule
	if rule == "" {
		rule = "default"
	}
	fmt.Printf("target: %s (resolved via %s)\n", resolved.Repo, rule)
	if explicitRepo != "" {
		fmt.Printf("       (override with -repo flag)\n")
	}
	// Fork divergence warning
	divergent, upstreamRepo, _ := gp.CheckForkDivergence(context.Background())
	if divergent {
		fmt.Printf("       fork divergence: origin=%s vs upstream=%s — targeting origin (override with -repo %s)\n",
			resolved.Repo, upstreamRepo, upstreamRepo)
	}
	// Fork refusal warning (targeting upstream parent without explicit consent)
	if refuse, reason, _ := specsync.ForkRefusal(context.Background(), resolved); refuse {
		fmt.Printf("       FORK REFUSAL: %s\n", reason)
	}
}

func printPreview(name, content string) {
	fmt.Println("  " + name)
	fmt.Println("    ┌───────────────────────────")
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		fmt.Println("    │ " + line)
	}
	fmt.Println("    └───────────────────────────")
}

// dryRunner prints what would run instead of calling gh, returning canned output
// so the orchestration proceeds through the create path.
func dryRunner(ctx context.Context, args ...string) (string, error) {
	// Repo auto-detection must stay live even on a dry run: it is read-only, and
	// canned output would key the ref cache as the bare "github", previewing
	// "created" for changes a real run would resolve and update. A failure (no
	// gh, offline) degrades to the bare key, same as a real run.
	if len(args) >= 2 && args[0] == "repo" && args[1] == "view" {
		out, err := exec.CommandContext(ctx, "gh", args...).Output()
		if err != nil {
			return "", nil
		}
		return strings.TrimSpace(string(out)), nil
	}

	var inline []string
	var body string
	for i := 0; i < len(args); i++ {
		if args[i] == "--body" && i+1 < len(args) {
			body = args[i+1]
			inline = append(inline, "--body", "«see below»")
			i++
			continue
		}
		inline = append(inline, args[i])
	}
	fmt.Println("  $ gh " + shellJoin(inline))
	if body != "" {
		fmt.Println("    ┌─ issue body ──────────────")
		for _, line := range strings.Split(body, "\n") {
			fmt.Println("    │ " + line)
		}
		fmt.Println("    └───────────────────────────")
	}

	switch {
	case len(args) >= 2 && args[0] == "issue" && args[1] == "list":
		return "[]", nil // pretend no existing issue
	case len(args) >= 2 && args[0] == "issue" && args[1] == "create":
		return "https://github.com/<owner>/<repo>/issues/0", nil
	case len(args) >= 2 && args[0] == "issue" && args[1] == "view":
		return `{"labels":[]}`, nil
	default:
		return "", nil
	}
}

// beadsDryRunner prints the bd commands that would run instead of executing
// them, returning canned output so Push proceeds through its create path: an
// empty list (no existing family) and a placeholder id for creates.
func beadsDryRunner(_ context.Context, args ...string) (string, error) {
	var inline []string
	var desc string
	for i := 0; i < len(args); i++ {
		if (args[i] == "-d" || args[i] == "--description") && i+1 < len(args) {
			desc = args[i+1]
			inline = append(inline, args[i], "«see below»")
			i++
			continue
		}
		inline = append(inline, args[i])
	}
	fmt.Println("  $ bd " + shellJoin(inline))
	if desc != "" {
		fmt.Println("    ┌─ description ─────────────")
		for _, line := range strings.Split(desc, "\n") {
			fmt.Println("    │ " + line)
		}
		fmt.Println("    └───────────────────────────")
	}

	switch {
	case len(args) >= 1 && args[0] == "list":
		return "[]", nil // pretend no existing beads
	case len(args) >= 1 && args[0] == "create":
		return "bd-dryrun", nil
	case len(args) >= 1 && args[0] == "show":
		return "[]", nil
	default:
		return "", nil
	}
}

func shellJoin(args []string) string {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		if a == "" || strings.ContainsAny(a, " \t\n\"'") {
			b.WriteString(fmt.Sprintf("%q", a))
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}

// runChanges lists OpenSpec changes with state and priority.
func runChanges(args []string) {
	fs := flag.NewFlagSet("changes", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	stages := fs.String("stage", "", "filter by stages (comma-separated, e.g. backlog,blocked)")
	sortBy := fs.String("sort", "stage", "sort order: stage (canonical), priority, or slug")
	asJSON := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}

	if *sortBy != "stage" && *sortBy != "priority" && *sortBy != "slug" {
		fail(fmt.Errorf("invalid sort value %q: must be stage, priority, or slug", *sortBy))
	}

	changes, err := specsync.LoadChanges(*openspec)
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

func sortChanges(changes []specsync.Change, sortBy string) {
	stageOrder := make(map[string]int)
	for i, s := range specsync.CanonicalStageOrder() {
		stageOrder[string(s)] = i
	}

	switch sortBy {
	case "stage":
		sort.SliceStable(changes, func(i, j int) bool {
			si, oki := stageOrder[string(changes[i].Stage)]
			sj, okj := stageOrder[string(changes[j].Stage)]
			if !oki {
				si = len(stageOrder)
			}
			if !okj {
				sj = len(stageOrder)
			}
			if si != sj {
				return si < sj
			}
			return changes[i].Slug < changes[j].Slug
		})
	case "priority":
		sort.SliceStable(changes, func(i, j int) bool {
			pi := priorityVal(changes[i].Priority)
			pj := priorityVal(changes[j].Priority)
			if pi != pj {
				return pi > pj // higher priority first
			}
			return changes[i].Slug < changes[j].Slug
		})
	case "slug":
		sort.SliceStable(changes, func(i, j int) bool {
			return changes[i].Slug < changes[j].Slug
		})
	}
}

func priorityVal(p *int) int {
	if p == nil {
		return -1
	}
	return *p
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func printChangeTable(changes []specsync.Change) {
	fmt.Println("STAGE          PRIORITY  TASKS       SLUG                          PROGRESS        TITLE")
	fmt.Println("─────────────  ────────  ─────────  ────────────────────────────  ──────────────  ────────────────────────────────────────────────────────────")

	prevStage := ""
	for _, c := range changes {
		if string(c.Stage) != prevStage {
			if prevStage != "" {
				fmt.Println()
			}
			prevStage = string(c.Stage)
		}

		priority := "-"
		if c.Priority != nil {
			priority = fmt.Sprintf("%d", *c.Priority)
		}

		total, completed := specsync.CountCheckboxes(c.TasksMarkdown)
		tasks := "-"
		if total > 0 {
			tasks = fmt.Sprintf("%d/%d", completed, total)
		}

		title := truncate(c.Title, 60)
		fmt.Printf("%-13s %-9s %-11s %-30s %-15s %s\n", c.Stage, priority, tasks, c.Slug, c.Progress, title)
	}
}

func collectDiagnostics(c specsync.Change) []string {
	var diagnostics []string
	if !specsync.IsCanonicalStage(c.Stage) && c.StageSource != specsync.StageSourceMetadata {
		diagnostics = append(diagnostics, fmt.Sprintf("non-canonical stage %q with source %q", c.Stage, c.StageSource))
	}
	if c.Priority != nil && (*c.Priority < 1 || *c.Priority > 100) {
		diagnostics = append(diagnostics, fmt.Sprintf("priority out of range: %d", *c.Priority))
	}
	return diagnostics
}

// mutableChange validates the slug, loads the change, and rejects archived
// changes — the shared guard path for every metadata-mutating subcommand, so
// the two commands can never drift on validation again.
func mutableChange(openspecDir, slug string, allowArchived bool) *specsync.Change {
	if err := validateSlug(slug); err != nil {
		fail(err)
	}
	change, err := specsync.LoadChangeBySlug(openspecDir, slug)
	if err != nil {
		fail(fmt.Errorf("change not found: %s", slug))
	}
	if change.Archived && !allowArchived {
		fail(fmt.Errorf("cannot mutate archived change %s", slug))
	}
	return change
}

// validateSlug ensures the slug is a safe directory name: lowercase letters,
// digits, hyphens, and underscores only; must start with a letter or digit.
func validateSlug(slug string) error {
	if strings.ContainsAny(slug, `/\`) || strings.Contains(slug, "..") {
		return fmt.Errorf("invalid slug %q: must be a plain change directory name (no slashes or path traversal)", slug)
	}
	if len(slug) == 0 {
		return fmt.Errorf("invalid slug: cannot be empty")
	}
	// Match ^[a-z0-9][a-z0-9_-]*$
	valid := func(s string) bool {
		for i, r := range s {
			if i == 0 {
				if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
					return false
				}
			} else {
				if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
					return false
				}
			}
		}
		return true
	}
	if !valid(slug) {
		return fmt.Errorf("invalid slug %q: must match ^[a-z0-9][a-z0-9_-]*$ (lowercase letters, digits, hyphens, underscores only)", slug)
	}
	return nil
}

// changeMetadata reads the change's current metadata, returning an empty
// (version-1) value when no file exists, so callers can read-modify-write.
func changeMetadata(change *specsync.Change) specsync.ChangeMetadata {
	meta, err := specsync.LoadChangeMetadata(change.Dir)
	if err != nil {
		fail(err)
	}
	if meta == nil {
		return specsync.ChangeMetadata{Version: 1}
	}
	return *meta
}

// runSetStage sets or unsets a change's explicit workflow stage. Only the
// stage field is touched: an explicit priority survives set-stage auto.
func runSetStage(args []string) {
	fs := flag.NewFlagSet("set-stage", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	if fs.NArg() < 2 {
		fail(fmt.Errorf("usage: specsync set-stage <change> <stage|auto>"))
	}
	changeName, stage := fs.Arg(0), fs.Arg(1)

	change := mutableChange(*openspec, changeName, false)
	meta := changeMetadata(change)

	if stage == "auto" {
		meta.Stage = nil // back to derived state; explicit priority survives
	} else {
		s := specsync.Stage(stage)
		if err := specsync.ValidateStage(s); err != nil {
			fail(err)
		}
		meta.Stage = &s
	}

	if err := specsync.SaveChangeMetadata(change.Dir, meta); err != nil {
		fail(err)
	}
	fmt.Printf("set-stage: %s → %s\n", changeName, stage)
}

// runSetPriority sets a change's priority.
func runSetPriority(args []string) {
	fs := flag.NewFlagSet("set-priority", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	if fs.NArg() < 2 {
		fail(fmt.Errorf("usage: specsync set-priority <change> <1-100|unset>"))
	}
	changeName, priorityArg := fs.Arg(0), fs.Arg(1)

	change := mutableChange(*openspec, changeName, true)
	meta := changeMetadata(change)

	if priorityArg == "unset" {
		meta.Priority = nil // an explicit stage survives
	} else {
		priority, err := strconv.Atoi(priorityArg)
		if err != nil || priority < 1 || priority > 100 {
			fail(fmt.Errorf("priority must be between 1 and 100; got %s", priorityArg))
		}
		meta.Priority = &priority
	}

	// Only fields already explicit in metadata.json are preserved; a stage
	// derived from tasks or legacy .status is never frozen into an override.
	if err := specsync.SaveChangeMetadata(change.Dir, meta); err != nil {
		fail(err)
	}
	fmt.Printf("set-priority: %s → %s\n", changeName, priorityArg)
}

// runNote appends a discovery line to the ## Discoveries section of a change.
func runNote(args []string) {
	fs := flag.NewFlagSet("note", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	dryRun := fs.Bool("dry-run", false, "show what would be written without modifying files")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	if fs.NArg() < 2 {
		fail(fmt.Errorf("usage: specsync note <change> <text>"))
	}
	changeName, text := fs.Arg(0), fs.Arg(1)

	c, err := specsync.LoadChangeBySlug(*openspec, changeName)
	if err != nil {
		fail(err)
	}

	discPath := filepath.Join(c.Dir, "discoveries.md")
	existingBytes, _ := os.ReadFile(discPath)
	existing := strings.TrimSpace(string(existingBytes))
	if existing != "" {
		existing += "\n"
	}
	newContent := existing + "- " + text + "\n"

	if *dryRun {
		fmt.Printf("[dry-run] would append to %s:\n%s\n", discPath, newContent)
		return
	}

	if err := os.WriteFile(discPath, []byte(newContent), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("note: appended to %s/discoveries.md\n", changeName)
}

// runSpinoff spawns a new linked change from a discovery or task in an
// existing change.
func runSpinoff(args []string) {
	fs := flag.NewFlagSet("spinoff", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	from := fs.String("from", "", "parent change slug (required)")
	task := fs.String("task", "", "task index to spin off (1-based); omit for free text mode")
	repo := fs.String("repo", "", "target repo as owner/name (default: same as parent)")
	kind := fs.String("kind", "", "issue kind: bug, followup, or task (sets label)")
	change := fs.String("change", "", "child change slug (default: derived from text)")
	text := fs.String("text", "", "discovery text (required when -task is not set)")
	dryRun := fs.Bool("dry-run", false, "show what would be written without modifying files")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}

	if strings.TrimSpace(*from) == "" {
		fail(fmt.Errorf("spinoff: -from <slug> is required"))
	}

	taskIndex := 0
	if *task != "" {
		n, err := strconv.Atoi(*task)
		if err != nil {
			fail(fmt.Errorf("spinoff: invalid task index %q", *task))
		}
		if n < 1 {
			fail(fmt.Errorf("spinoff: task index must be >= 1"))
		}
		taskIndex = n
	}

	// Either -task or -text is required, but not both.
	if taskIndex > 0 && *text != "" {
		fail(fmt.Errorf("spinoff: cannot use both -task and -text"))
	}
	if taskIndex == 0 && *text == "" {
		fail(fmt.Errorf("spinoff: either -task <n> or -text <text> is required"))
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	res, err := specsync.Spinoff(context.Background(), specsync.SpinoffOptions{
		OpenSpecDir: abs,
		Parent:      *from,
		TaskIndex:   taskIndex,
		Text:        *text,
		Slug:        *change,
		Repo:        *repo,
		Kind:        *kind,
		DryRun:      *dryRun,
	})
	if err != nil {
		fail(err)
	}

	if *dryRun {
		fmt.Println("DRY RUN — no files or GitHub calls will be modified")
		fmt.Println()
		fmt.Printf("  new change: %s\n", res.ChildDir)
		fmt.Println()
		fmt.Println("  ┌─ proposal.md ──────────────")
		for _, line := range strings.Split(res.Proposal, "\n") {
			fmt.Println("  │ " + line)
		}
		fmt.Println("  └───────────────────────────")
		if res.ParentTaskN > 0 {
			fmt.Printf("\n  would mark parent task %d as: - [>] moved: %s\n", res.ParentTaskN, res.ChildSlug)
		}
		if res.Label != "" {
			fmt.Printf("\n  child issue label: %s\n", res.Label)
		}
		if res.Linked {
			fmt.Printf("\n  would link %s ↔ %s\n", res.ParentSlug, res.ChildSlug)
		}
		return
	}

	fmt.Printf("specsync spinoff: %s -> %s\n", *from, res.ChildSlug)
	fmt.Println("  + proposal.md")
	fmt.Println("  + tasks.md")
	if res.ParentTaskN > 0 {
		fmt.Printf("  parent task %d marked as moved\n", res.ParentTaskN)
	}
	if res.Linked {
		fmt.Printf("  linked %s ↔ %s\n", res.ParentSlug, res.ChildSlug)
	}
	if res.Label != "" {
		fmt.Printf("  label: %s\n", res.Label)
	}
}

// runAudit cross-references archived changes against PR state and classifies
// each as unmerged, shipped, or orphaned.
func runAudit(args []string) {
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	repo := fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	asJSON := fs.Bool("json", false, "output as JSON")
	failOnUnmerged := fs.Bool("fail-on-unmerged", false, "exit non-zero when unmerged changes exist")
	markShipped := fs.Bool("mark-shipped", false, "write shipped stage to .specsync/metadata.json for confirmed merges")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

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

func countStatus(findings []specsync.AuditFinding, status string) int {
	n := 0
	for _, f := range findings {
		if f.Status == status {
			n++
		}
	}
	return n
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "specsync:", err)
	os.Exit(1)
}

// runAuditTasks scans all changes for unchecked tasks and flags mismatches
// where code exists but tasks remain unchecked — the dogfooding failure mode.
func runAuditTasks(args []string) {
	fs := flag.NewFlagSet("audit-tasks", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	asJSON := fs.Bool("json", false, "output as JSON")
	failOnMismatch := fs.Bool("fail-on-mismatch", false, "exit non-zero when mismatches exist")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

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

// runValidate checks all change folders for structural issues: required files,
// valid metadata, well-formed stages. Reports all issues in one pass.
func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	asJSON := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	result := specsync.ValidateChanges(abs)

	if *asJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fail(fmt.Errorf("marshal JSON: %w", err))
		}
		fmt.Println(string(data))
	} else {
		if len(result.Issues) == 0 {
			fmt.Println("specsync validate: all changes are structurally valid")
			return
		}
		fmt.Printf("specsync validate: %d issue(s) found\n\n", len(result.Issues))
		for _, issue := range result.Issues {
			prefix := issue.Field
			if issue.Slug != "" {
				prefix = fmt.Sprintf("%s/%s", issue.Slug, issue.Field)
			}
			fmt.Printf("  ❌ %s: %s\n", prefix, issue.Error)
		}
	}

	if len(result.Issues) > 0 {
		os.Exit(1)
	}
}
