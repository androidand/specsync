// Command specsync projects OpenSpec changes into a work tracker (GitHub today).
// It is a standalone, single-binary tool that works in any OpenSpec project,
// regardless of the project's own language.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/androidand/specsync"
)

// version is the binary version, stamped at release time via -ldflags "-X main.version=...".
var version = "dev"

// resolveSubcommand decides which subcommand os.Args[1:] selects and returns
// its remaining arguments. A missing first argument, or one starting with
// "-", both select "sync" (bare invocation with flags only) — that keeps
// `specsync -change foo` working. Any other bare word that isn't in
// a registered command is an error: Go's flag package stops parsing at the first
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
	if _, ok := lookupCommand(first); ok {
		return first, args[1:], nil
	}
	if strings.HasPrefix(first, "-") {
		return "sync", args, nil
	}
	if suggestion, note, ok := confusionFor(first); ok {
		return "", nil, fmt.Errorf("unknown subcommand %q — did you mean %q? %s", first, suggestion, note)
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
		fmt.Fprintf(os.Stderr, "specsync: %v\n\nRun with no subcommand (optionally with flags) to sync, or use one of: %s\n", err, strings.Join(commandNames(), ", "))
		os.Exit(2)
	}

	if c, ok := lookupCommand(cmd); ok {
		if err := c.Run(context.Background(), rest); err != nil {
			fail(err)
		}
		return
	}

	if cmd == "version" {
		fmt.Println("specsync " + versionString())
		return
	}
	fail(fmt.Errorf("unknown subcommand %q", cmd))
}

// isVersionArg reports whether the first CLI arg requests the binary version.
func isVersionArg(arg string) bool {
	return arg == "version" || arg == "-version" || arg == "--version"
}

// containsFlag checks if args contain a specific flag (with or without value).
// Checks both -flag and --flag forms, and -flag=value.
func containsFlag(args []string, name string) bool {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		// Remove leading dashes
		flagName := strings.TrimLeft(arg, "-")
		// Handle -flag=value by splitting on =
		if idx := strings.Index(flagName, "="); idx != -1 {
			flagName = flagName[:idx]
		}
		if flagName == name {
			return true
		}
	}
	return false
}

// reorderFlagsFirst moves every flag token (anything starting with "-") ahead
// of positional args, preserving relative order within each group. Go's flag
// package stops parsing at the first positional token, so a flag written
// after a subcommand name (e.g. "agent-help sync -json") is otherwise left
// unparsed. Only safe for all-boolean flag sets, where no flag consumes a
// separate value token.
func reorderFlagsFirst(args []string) []string {
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}

// checkArgs fails loudly, naming the exact problem, when args left over
// after fs.Parse indicate a mistake rather than legitimate input: either a
// registered flag shadowed by an earlier positional token (see
// shadowedFlags), or unconsumed positional arguments beyond maxPositional.
// Pass -1 for maxPositional to skip the positional-count check for
// subcommands whose positional arguments are free-form and validated
// elsewhere (e.g. link, scan, set-stage, set-priority, note, epic).
//
// This is the fix for the 2026-09-24 incident: `specsync -repo
// ExopenGitHub/portal <issue-url> -dry-run` silently discarded the URL
// (leaving -change unset, meaning "sync everything") and never even parsed
// -dry-run (shadowed by the URL), turning a mistyped single-issue command
// into a full live sync with zero warning.
func checkArgs(fs *flag.FlagSet, rawArgs []string, maxPositional int) {
	if err := checkArgsErr(fs, rawArgs, maxPositional); err != nil {
		fail(err)
	}
}

// checkArgsErr is checkArgs's testable core: it returns the problem instead
// of exiting, so tests can assert on it in-process.
func checkArgsErr(fs *flag.FlagSet, rawArgs []string, maxPositional int) error {
	shadowed := shadowedFlags(fs, rawArgs)
	shadowedName := map[string]bool{}
	for _, s := range shadowed {
		shadowedName[strings.TrimPrefix(strings.TrimPrefix(s, "-"), "-")] = true
	}

	var problems []string
	if len(shadowed) > 0 {
		problems = append(problems, fmt.Sprintf(
			"flag(s) %s appeared after a positional argument and were not applied — Go's flag parser stops parsing at the first non-flag token; put flags before positional arguments",
			strings.Join(shadowed, ", ")))
	}
	if maxPositional >= 0 && fs.NArg() > maxPositional {
		// Exclude tokens already named above as shadowed flags — fs.Args()
		// past the positional boundary contains every leftover token
		// (Parse never touched any of them), so a shadowed flag would
		// otherwise be named twice as two separate-looking problems.
		var extra []string
		for _, a := range fs.Args()[maxPositional:] {
			name := strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
			if idx := strings.Index(name, "="); idx != -1 {
				name = name[:idx]
			}
			if shadowedName[name] {
				continue
			}
			extra = append(extra, a)
		}
		if len(extra) > 0 {
			problems = append(problems, fmt.Sprintf("unexpected argument(s): %s", strings.Join(extra, " ")))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s", fs.Name(), strings.Join(problems, "; "))
}

// shadowedFlags scans rawArgs — the args exactly as given, before fs.Parse —
// for any token naming a flag registered on fs that appears after the first
// non-flag ("positional") token. fs.Parse stops parsing entirely at that
// first positional token (a well-known stdlib `flag` footgun), so such a
// flag is never even looked at and silently keeps its zero value instead of
// erroring or taking effect. Matching is exact-name only — no typo-fuzzing
// (e.g. -dryrun for -dry-run is not caught; that's a separate, fuzzier
// problem).
func shadowedFlags(fs *flag.FlagSet, rawArgs []string) []string {
	registered := map[string]bool{}
	valueFlag := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		registered[f.Name] = true
		if bv, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !bv.IsBoolFlag() {
			valueFlag[f.Name] = true
		}
	})

	var shadowed []string
	seen := map[string]bool{}
	seenPositional := false
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if !strings.HasPrefix(arg, "-") {
			seenPositional = true
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
		hasInlineValue := false
		if idx := strings.Index(name, "="); idx != -1 {
			name = name[:idx]
			hasInlineValue = true
		}
		if !registered[name] {
			continue
		}
		if seenPositional && !seen[name] {
			shadowed = append(shadowed, "-"+name)
			seen[name] = true
		}
		// Only skip a value-flag's value token before the positional
		// boundary — that's the one region where a value token is real
		// (Go's flag.Parse actually consumes it there). Once past the
		// boundary, Parse never looks at any of these tokens at all, so a
		// registered flag name immediately following another one is
		// independently shadowed, not "consumed" as the first flag's value.
		if !seenPositional && valueFlag[name] && !hasInlineValue && i+1 < len(rawArgs) {
			i++
		}
	}
	return shadowed
}

// stringSlice implements flag.Value for repeatable -provider flags.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
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
		Provider:    makeProvider(repo, false, "github", ""),
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

// detectProvider returns ("beads", reason) when Beads should be auto-selected,
// or ("github", "") otherwise. An explicit -provider always wins.
//
// The signal for Beads is a Beads database in *this project* — a `.beads/`
// directory at the repo root or in the working directory. `bd` merely being
// installed is not a signal: a globally installed `bd` used to redirect every
// repo's sync away from GitHub, so a plain `specsync sync` in a GitHub project
// would create phantom beads instead of updating the issues it had already
// created. `bd` on PATH is still necessary — a project carrying `.beads/` on a
// machine without the binary falls back to github rather than failing on the
// first shell-out.
func detectProvider(provider, repoRoot string) (string, string) {
	if provider != "" {
		return provider, ""
	}
	if _, err := exec.LookPath("bd"); err != nil {
		return "github", ""
	}
	for _, dir := range []string{repoRoot, "."} {
		if dir == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(dir, ".beads")); err == nil && fi.IsDir() {
			return "beads", ".beads/ found at " + dir
		}
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
// graph and ignores repo; mcp delegates to an external MCP server configured
// via mcpConfigPath (only consulted when provider is "mcp").
func makeProvider(repo string, dryRun bool, provider string, mcpConfigPath string) specsync.WorkProvider {
	switch provider {
	case "beads":
		if dryRun {
			return specsync.NewBeadsProviderFunc(beadsDryRunner)
		}
		return specsync.NewBeadsProvider()
	case "mcp":
		cfg, err := specsync.LoadMCPConfig(mcpConfigPath)
		if err != nil {
			fail(fmt.Errorf("-provider mcp: %w", err))
		}
		if dryRun {
			return specsync.NewMCPProviderDryRun(cfg)
		}
		return specsync.NewMCPProvider(cfg)
	case "", "github":
		if dryRun {
			return specsync.NewGitHubProviderFuncWithRepo(repo, dryRunner)
		}
		if repo != "" {
			return specsync.NewGitHubProviderWithRepo(repo)
		}
		return specsync.NewGitHubProvider()
	default:
		fail(fmt.Errorf("unknown provider %q: use github, beads, or mcp", provider))
		return nil // unreachable: fail exits
	}
}

// makeSpecSource returns a SpecSource for the given name.
// Supported values: "openspec" (default), "beads" (placeholder).
func makeSpecSource(name string) (specsync.SpecSource, error) {
	switch name {
	case "openspec":
		return specsync.FileSpecSource{}, nil
	case "beads":
		return specsync.BeadsSource{}, nil
	default:
		return nil, fmt.Errorf("unknown spec source %q: use openspec or beads", name)
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
	if plan.HumanMovedToDone != "" {
		fmt.Printf("               • ⚠ %s — specsync won't drag it back\n", plan.HumanMovedToDone)
	}
	if plan.HumanMovedToActive != "" {
		fmt.Printf("               • ⚠ %s\n", plan.HumanMovedToActive)
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
		return fmt.Errorf("invalid slug %q: must be a plain directory name (no slashes or ..); use ^[a-z0-9][a-z0-9_-]*$", slug)
	}
	if len(slug) == 0 {
		return fmt.Errorf("invalid slug: cannot be empty; use ^[a-z0-9][a-z0-9_-]*$ (lowercase letters, digits, hyphens, underscores)")
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

// deriveIdeaTitle returns a title from the idea text: first line or first
// sentence, truncated to 70 chars.
func deriveIdeaTitle(text string) string {
	// Try first line.
	lines := strings.SplitN(text, "\n", 2)
	title := strings.TrimSpace(lines[0])

	// If the first line is very long, try to find a sentence boundary.
	if len(title) > 70 {
		// Look for a sentence ending between 40 and 70 chars.
		for i := 40; i < 70 && i < len(title); i++ {
			if title[i] == '.' || title[i] == '!' || title[i] == '?' {
				title = strings.TrimSpace(title[:i+1])
				break
			}
		}
	}

	// Truncate to 70 chars if still too long.
	if len(title) > 70 {
		title = strings.TrimSpace(title[:70])
	}

	return title
}
