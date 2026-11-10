package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	specsync "github.com/androidand/specsync"
)

type startCmd struct {
	openspec, storeFlag *string
	typ, base           *string
	launcher            *string
	agent, prompt       *string
	dryRun, asJSON      *bool
	sessions            sessionFlags
}

func init() { Register(&startCmd{}) }

func (*startCmd) Spec() Spec {
	return Spec{
		Name:     "start",
		Group:    GroupPlan,
		Summary:  "Start or resume work on a change in its own worktree, the same way every time.",
		Intents:  []string{"start work on an issue", "resume work", "make a worktree for a change"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "issue-first-start", RelatedBefore: []string{"pull", "new"}, RelatedAfter: []string{"next", "save"}},
		Args:     []Arg{{Name: "ref", Description: "a change slug, or the number or URL of an issue the change is linked to", Required: true}},
		Safety: []string{
			"Reuses the change's worktree when it exists and creates nothing; otherwise creates one from the worktrees: directory and branch pattern in openspec/specsync.yml",
			"Start hooks run only in a newly created worktree, and only after `specsync trust -yes`",
			"Refuses when the session provider reports a live session in the worktree, or cannot be asked while one is configured",
			"Records the branch-to-change link in the repository's git config; -dry-run changes nothing",
			"With the orca launcher (worktrees.launcher or -launcher), Orca creates the worktree linked to the change's issue; start fails if Orca put it on a different branch, and removes any upstream it set",
			"Orca is found via $ORCA_CLI_COMMAND, then orca-ide on Linux (a bare orca there is the screen reader), then orca",
		},
		Examples: []string{
			"specsync start add-retry -dry-run",
			"specsync start add-retry",
			"specsync start 42",
			"specsync start -launcher orca -agent claude add-retry",
		},
	}
}

func (cm *startCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.typ = fs.String("type", "feat", "branch type for the {type} placeholder")
	cm.base = fs.String("base", "", "start point for a new branch (default: the repository's default branch)")
	cm.launcher = fs.String("launcher", "", "git, orca or auto (Orca when installed) to create a new worktree (default: worktrees.launcher, else git)")
	cm.agent = fs.String("agent", "", "Orca only: agent to launch in the new workspace, e.g. claude or codex")
	cm.prompt = fs.String("prompt", "", "Orca only: initial prompt for that agent")
	cm.dryRun = fs.Bool("dry-run", false, "show the worktree and branch and create nothing")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.sessions = addSessionFlags(fs)
}

var issueRefForms = []*regexp.Regexp{
	regexp.MustCompile(`^#?(\d+)$`),
	regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+#(\d+)$`),
	regexp.MustCompile(`^https?://[^/\s]+/[^/\s]+/[^/\s]+/issues/(\d+)/?$`),
}

// issueNumberOf returns the issue number a ref names, or "" when it is not one
// of the issue forms: a bare number, #N, owner/repo#N or an /issues/N URL. A
// slug that merely ends in digits is not an issue.
func issueNumberOf(ref string) string {
	for _, re := range issueRefForms {
		if m := re.FindStringSubmatch(ref); m != nil {
			return m[1]
		}
	}
	return ""
}

func (cm *startCmd) Run(ctx context.Context, args []string) error {
	fs := flagSetFor(cm)
	args = reorderKnownFlags(fs, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkArgs(fs, args, -1)
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: specsync start <slug|issue number|issue URL>")
	}
	ref := strings.TrimSpace(fs.Arg(0))
	root := resolveRoot(fs, cm.openspec, cm.storeFlag)
	changes, err := specsync.LoadChanges(root.Dir)
	if err != nil {
		return err
	}
	change, issue := resolveStartRef(changes, ref)
	if change == nil {
		return fmt.Errorf("no change matches %q: pull the issue first (specsync pull -issue N) or create one (specsync new <title>)", ref)
	}
	cwd, _ := os.Getwd()
	repoRoot := configRoot(ctx, fs, cwd)
	ways := loadWaysOrFail(repoRoot)
	orca, err := startLauncher(*cm.launcher, ways.Worktrees.Launcher, os.Getenv, exec.LookPath, runtime.GOOS)
	if err != nil {
		return err
	}
	if orca == "" && (*cm.agent != "" || *cm.prompt != "") {
		return fmt.Errorf("-agent and -prompt need the orca launcher")
	}
	opts := specsync.StartOptions{RepoDir: cwd, ConfigRoot: repoRoot, Ways: ways, Slug: change.Slug, Issue: issue, Type: *cm.typ, Base: *cm.base, DryRun: *cm.dryRun,
		Orca: orca, Agent: *cm.agent, Prompt: *cm.prompt}
	if cmdline := cm.sessions.command(fs); cmdline != "" {
		opts.SessionsChecked = true
		opts.Sessions, opts.SessionsErr = specsync.LoadSessions(ctx, cmdline, *cm.sessions.timeout)
	}
	res, err := specsync.StartWork(ctx, opts)
	if err != nil {
		return err
	}
	if *cm.asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	switch {
	case res.Existing:
		fmt.Printf("specsync: %s already has a worktree: %s (branch %s); nothing created\n", change.Slug, res.Path, res.Branch)
	case *cm.dryRun && res.Launcher == "orca":
		fmt.Printf("DRY RUN — would run %s, then %d start hook(s) in the worktree Orca creates on branch %s\n", shellJoin(res.Command), len(ways.Worktrees.Hooks), res.Branch)
	case *cm.dryRun:
		fmt.Printf("DRY RUN — would create %s on branch %s, run %d start hook(s) there\n", res.Path, res.Branch, len(ways.Worktrees.Hooks))
	default:
		fmt.Printf("specsync: created %s on branch %s (%d start hook(s) run)\n", res.Path, res.Branch, res.Hooks)
	}
	if !*cm.dryRun {
		fmt.Printf("next: cd %s\n", shellQuote(res.Path))
	}
	return nil
}

// startLauncher returns the Orca CLI that should create a new worktree, or ""
// for git. The flag wins over the config; both default to git.
func startLauncher(flagValue, configValue string, getenv func(string) string, lookPath func(string) (string, error), goos string) (string, error) {
	launcher := flagValue
	if launcher == "" {
		launcher = configValue
	}
	switch launcher {
	case "", "git":
		return "", nil
	case "auto":
		return resolveOrca(getenv, lookPath, goos), nil
	case "orca":
		if orca := resolveOrca(getenv, lookPath, goos); orca != "" {
			return orca, nil
		}
		hint := "put orca on PATH"
		if goos == "linux" {
			hint = "put orca-ide on PATH (a bare orca on Linux is the screen reader)"
		}
		return "", fmt.Errorf("the orca launcher needs Orca: set ORCA_CLI_COMMAND or %s", hint)
	default:
		return "", fmt.Errorf("-launcher must be git, orca or auto; got %q", launcher)
	}
}

func resolveStartRef(changes []specsync.Change, ref string) (*specsync.Change, string) {
	issueOf := func(c specsync.Change) string {
		refs, err := specsync.LoadRefs(c.Dir)
		if err != nil {
			return ""
		}
		for _, r := range refs {
			if r.ID != "" {
				return r.ID
			}
		}
		return ""
	}
	for i := range changes {
		if !changes[i].Archived && changes[i].Slug == ref {
			return &changes[i], issueOf(changes[i])
		}
	}
	number := issueNumberOf(ref)
	if number == "" {
		return nil, ""
	}
	for i := range changes {
		if changes[i].Archived {
			continue
		}
		if refs, err := specsync.LoadRefs(changes[i].Dir); err == nil {
			for _, r := range refs {
				if r.ID == number {
					return &changes[i], number
				}
			}
		}
	}
	return nil, ""
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
