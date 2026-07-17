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
	"changes": true, "set-stage": true, "set-priority": true,
	"sync": true,
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
// `specsync -slug foo` working. Any other bare word that isn't in
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
	// Check for deprecated -slug flag used as a subcommand.
	if first == "-slug" || first == "--slug" {
		return "", nil, fmt.Errorf("unknown subcommand %q — did you mean -change? (specsync defaults to syncing all changes; use -change <name> to sync a single change)", first)
	}
	return "", nil, fmt.Errorf("unknown subcommand %q", first)
}

func main() {
	cmd, rest, err := resolveSubcommand(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "specsync: %v\n\nRun with no subcommand (optionally with flags) to sync, or use one of: pull, link, scan, trace, release-plan, changelog, install-skill, changes, set-stage, set-priority\n", err)
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
	case "sync":
		runSync(rest)
	}
}

// isVersionArg reports whether the first CLI arg requests the binary version.
func isVersionArg(arg string) bool {
	return arg == "version" || arg == "-version" || arg == "--version"
}

// runSync projects every OpenSpec change into the tracker (spec -> issue).
func runSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	change := fs.String("change", "", "sync only this change (default: all changes)")
	repo := fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	providerName := fs.String("provider", "github", "work provider: github (default, human-facing) or beads (agent-facing)")
	dryRun := fs.Bool("dry-run", false, "print the provider commands and rendered body without executing")
	reconcile := fs.Bool("reconcile", true, "merge external task state back into tasks.md before pushing")
	closeCompleted := fs.Bool("close-completed", false, "close the tracker item once every task in a change is checked")
	project := fs.String("project", "", "target GitHub Projects board as owner/number (default: $SPECSYNC_PROJECT; unset = no board)")
	assignee := fs.String("assignee", "", "board assignee login (default: the acting viewer, \"me\")")
	statusMap := fs.String("status-map", "", "stage→Status overrides as stage=Name pairs, e.g. \"active=In Progress,archived=Done\" (default: $SPECSYNC_STATUS_MAP)")
	// Handle deprecated -slug flag with helpful error.
	for _, arg := range args {
		if arg == "-slug" || arg == "--slug" {
			fail(fmt.Errorf("unknown flag %s — did you mean -change?", arg))
		}
	}
	_ = fs.Parse(args)

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	target, err := boardTarget(*project, *assignee, *statusMap)
	if err != nil {
		fail(err)
	}

	provider := makeProvider(*repo, *dryRun, *providerName)
	if *dryRun {
		fmt.Printf("DRY RUN — no %s calls are made\n", *providerName)
		if *providerName == "github" {
			if *repo != "" {
				fmt.Printf("target: %s\n", *repo)
			} else {
				fmt.Println("target: auto-detected from the current repo's git remote")
			}
		}
		fmt.Println()
	}

	if *dryRun && target.Configured() {
		fmt.Printf("board: %s/%d (no GraphQL mutations on a dry run)\n\n", target.Owner, target.Number)
	}

	res, err := specsync.Sync(context.Background(), specsync.Options{
		OpenSpecDir:    abs,
		Provider:       provider,
		Slug:           *change,
		DryRun:         *dryRun,
		Reconcile:      *reconcile,
		CloseCompleted: *closeCompleted,
		Project:        target,
	})
	if err != nil {
		fail(err)
	}
	if *dryRun && *reconcile {
		fmt.Println("(reconcile applies on a real sync — dry-run makes no issue reads)")
	}
	fmt.Println()
	for _, it := range res.Items {
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
	issue := fs.String("issue", "", "issue number to pull into a local change (required)")
	change := fs.String("change", "", "change name (default: derived from the issue title)")
	repo := fs.String("repo", "", "source repo as owner/name (default: auto-detect from git remote)")
	dryRun := fs.Bool("dry-run", false, "show what would be written without touching disk")
	project := fs.String("project", "", "target GitHub Projects board as owner/number (default: $SPECSYNC_PROJECT; unset = no board)")
	assignee := fs.String("assignee", "", "board assignee login (default: the acting viewer, \"me\")")
	statusMap := fs.String("status-map", "", "stage→Status overrides as stage=Name pairs, e.g. \"active=In Progress,archived=Done\" (default: $SPECSYNC_STATUS_MAP)")
	// Handle deprecated -slug flag with helpful error.
	for _, arg := range args {
		if arg == "-slug" || arg == "--slug" {
			fail(fmt.Errorf("unknown flag %s — did you mean -change?", arg))
		}
	}
	_ = fs.Parse(args)

	if strings.TrimSpace(*issue) == "" {
		fail(fmt.Errorf("pull: -issue is required"))
	}
	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}
	target, err := boardTarget(*project, *assignee, *statusMap)
	if err != nil {
		fail(err)
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
		fmt.Printf("DRY RUN — would write %s from issue %s\n\n", dest, *issue)
		printPreview("proposal.md", res.Proposal)
		if res.Tasks != "" {
			printPreview("tasks.md", res.Tasks)
		}
		if res.TitleCleaned {
			fmt.Printf("\n  title cleaned: %q -> %q\n", res.TitleBefore, res.TitleAfter)
		}
		if res.MarkerPresent {
			fmt.Printf("\nissue %s already carries the marker %s (no edit needed)\n", *issue, res.Marker)
		} else {
			fmt.Printf("\nwould add marker to issue %s body: %s\n", *issue, res.Marker)
		}
		if res.BoardConfigured {
			printBoardPlan(res.Board, true)
		}
		return
	}
	fmt.Printf("specsync: pulled issue %s -> %s\n", *issue, dest)
	fmt.Println("  + proposal.md")
	if res.Tasks != "" {
		fmt.Println("  + tasks.md")
	}
	if res.TitleCleaned {
		fmt.Printf("  title cleaned: %q -> %q\n", res.TitleBefore, res.TitleAfter)
	}
	if res.BoardConfigured {
		printBoardPlan(res.Board, false)
	}
}

// runLink writes links.md for each change (recording the other's issue URL) and
// then syncs each spec so the "## Related" section appears in both GitHub issues.
//
// Usage: specsync link [flags] <change1> <change2> [<change3>...]
func runLink(args []string) {
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	dryRun := fs.Bool("dry-run", false, "show what would change without writing files or calling GitHub")
	_ = fs.Parse(args)

	changes := fs.Args()
	if len(changes) < 2 {
		fail(fmt.Errorf("link: at least 2 changes required\nusage: specsync link <change1> <change2> [<change3>...]"))
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	pairs, err := specsync.Link(specsync.LinkOptions{
		OpenSpecDir: abs,
		Slugs:       changes,
		DryRun:      *dryRun,
	})
	if err != nil {
		fail(err)
	}

	if *dryRun {
		fmt.Println("DRY RUN — no files or GitHub calls will be modified")
		fmt.Println()
		for i, p := range pairs {
			fmt.Printf("  %s/links.md would contain:\n", p.Slug)
			for j, other := range pairs {
				if j != i {
					fmt.Printf("    - %s\n", other.Ref.URL)
				}
			}
			// Render the Related section preview by loading the change and
			// injecting the would-be links directly, bypassing disk.
			c, err := specsync.LoadChange(p.Dir, false, abs)
			if err == nil && c != nil {
				c.Links = nil
				for j, other := range pairs {
					if j != i {
						c.Links = append(c.Links, other.Ref)
					}
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
		fmt.Printf("specsync link: would cross-link %d specs\n", len(pairs))
		return
	}

	// Real run: sync each spec with the provider matching its repo.
	for _, p := range pairs {
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
	fmt.Printf("specsync link: %d specs cross-linked\n", len(pairs))
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

// boardTarget parses the -project flag (falling back to $SPECSYNC_PROJECT so the
// board need not be retyped) into a BoardTarget. An empty value yields the zero
// target, which disables all board behavior. statusMap (falling back to
// $SPECSYNC_STATUS_MAP) overrides the default stage→Status-name mapping; its
// syntax is validated even without a project so a typo never fails silently.
func boardTarget(project, assignee, statusMap string) (specsync.BoardTarget, error) {
	mapping, err := parseStatusMapping(statusMap)
	if err != nil {
		return specsync.BoardTarget{}, err
	}
	if strings.TrimSpace(project) == "" {
		project = os.Getenv("SPECSYNC_PROJECT")
	}
	project = strings.TrimSpace(project)
	if project == "" {
		return specsync.BoardTarget{}, nil
	}
	parts := strings.Split(project, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return specsync.BoardTarget{}, fmt.Errorf("-project must be owner/number, got %q", project)
	}
	number, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return specsync.BoardTarget{}, fmt.Errorf("-project number is invalid in %q: %w", project, err)
	}
	return specsync.BoardTarget{
		Owner:         strings.TrimSpace(parts[0]),
		Number:        number,
		Assignee:      strings.TrimSpace(assignee),
		StatusMapping: mapping,
	}, nil
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
			return nil, fmt.Errorf("-status-map stage %q is unknown; valid stages: active, complete, archived", k)
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
	asJSON := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		fail(err)
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

	// Output
	if *asJSON {
		// Marshal as proper JSON
		type changeJSON struct {
			Slug           string `json:"slug"`
			Title          string `json:"title"`
			Stage          string `json:"stage"`
			CanonicalStage bool   `json:"canonicalStage"`
			StageSource    string `json:"stageSource"`
			Progress       string `json:"taskProgress"`
			Priority       *int   `json:"priority"`
			Archived       bool   `json:"archived"`
		}

		var results []changeJSON
		for _, c := range filtered {
			results = append(results, changeJSON{
				Slug:           c.Slug,
				Title:          c.Title,
				Stage:          string(c.Stage),
				CanonicalStage: specsync.IsCanonicalStage(c.Stage),
				StageSource:    string(c.StageSource),
				Progress:       string(c.Progress),
				Priority:       c.Priority,
				Archived:       c.Archived,
			})
		}

		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			fail(fmt.Errorf("marshal JSON: %w", err))
		}
		fmt.Println(string(data))
	} else {
		// Table output
		fmt.Println("SLUG                          STAGE          PROGRESS        PRIORITY")
		fmt.Println("────────────────────────────  ─────────────  ──────────────  ────────")
		for _, c := range filtered {
			priority := "-"
			if c.Priority != nil {
				priority = fmt.Sprintf("%d", *c.Priority)
			}
			fmt.Printf("%-30s %-14s %-15s %s\n", c.Slug, c.Stage, c.Progress, priority)
		}
	}
}

// mutableChange validates the slug, loads the change, and rejects archived
// changes — the shared guard path for every metadata-mutating subcommand, so
// the two commands can never drift on validation again.
func mutableChange(openspecDir, slug string) *specsync.Change {
	if strings.ContainsAny(slug, `/\`) || strings.Contains(slug, "..") {
		fail(fmt.Errorf("invalid slug %q: must be a plain change directory name", slug))
	}
	change, err := specsync.LoadChangeBySlug(openspecDir, slug)
	if err != nil {
		fail(fmt.Errorf("change not found: %s", slug))
	}
	if change.Archived {
		fail(fmt.Errorf("cannot mutate archived change %s", slug))
	}
	return change
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

	change := mutableChange(*openspec, changeName)
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

	change := mutableChange(*openspec, changeName)
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

func fail(err error) {
	fmt.Fprintln(os.Stderr, "specsync:", err)
	os.Exit(1)
}
