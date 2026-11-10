package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	specsync "github.com/androidand/specsync"
)

type startCmd struct {
	openspec, storeFlag *string
	typ, base           *string
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
		},
		Examples: []string{
			"specsync start add-retry -dry-run",
			"specsync start add-retry",
			"specsync start 42",
		},
	}
}

func (cm *startCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.typ = fs.String("type", "feat", "branch type for the {type} placeholder")
	cm.base = fs.String("base", "", "start point for a new branch (default: the repository's default branch)")
	cm.dryRun = fs.Bool("dry-run", false, "show the worktree and branch and create nothing")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.sessions = addSessionFlags(fs)
}

var issueRefNumber = regexp.MustCompile(`(?:#|/issues/)?(\d+)$`)

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
	repoRoot := textRepoRoot(&root)
	ways := loadWaysOrFail(repoRoot)
	cwd, _ := os.Getwd()
	opts := specsync.StartOptions{RepoDir: cwd, ConfigRoot: repoRoot, Ways: ways, Slug: change.Slug, Issue: issue, Type: *cm.typ, Base: *cm.base, DryRun: *cm.dryRun}
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
	m := issueRefNumber.FindStringSubmatch(ref)
	if m == nil {
		return nil, ""
	}
	for i := range changes {
		if changes[i].Archived {
			continue
		}
		if refs, err := specsync.LoadRefs(changes[i].Dir); err == nil {
			for _, r := range refs {
				if r.ID == m[1] {
					return &changes[i], m[1]
				}
			}
		}
	}
	return nil, ""
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
