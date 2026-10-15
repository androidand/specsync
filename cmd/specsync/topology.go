package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	specsync "github.com/androidand/specsync"
)

// statusCmd prints the change → repo → issue → branch → worktree join
// (see openspec/changes/agent-topology-command). Read-only; every source is
// optional, so a missing openspec/gh/git-worktree source narrows a row
// instead of dropping it. Exits non-zero only when nothing could be resolved
// at all — a command an agent checks before starting work must not fail
// closed just because one source degraded.
type statusCmd struct {
	openspec, change *string
	all, asJSON      *bool
	state            *string
	everything       *bool
}

func init() { Register(&statusCmd{}) }

func (*statusCmd) Spec() Spec {
	return Spec{
		Name:     "status",
		Group:    GroupOverview,
		Summary:  "Show where your current work stands, or every change with -everything.",
		Workflow: AgentCommandWorkflow{Position: "planning", RelatedAfter: []string{"changes", "relate"}},
		Safety:   []string{"Read-only: never writes to disk or the tracker"},
		Examples: []string{
			"specsync status",
			"specsync status -everything",
			"specsync status -change my-change -json",
			"specsync status -state=off",
			"specsync status -state=fresh",
		},
	}
}

func (cm *statusCmd) Flags(fs *flag.FlagSet) {
	cm.openspec = fs.String("openspec", "openspec", "path to the openspec/ directory")
	cm.change = fs.String("change", "", "scope to one change (default: every change)")
	cm.all = fs.Bool("all", false, "include archived changes")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.everything = fs.Bool("everything", false, "list every change instead of the current work")
	cm.state = fs.String("state", "on", "on (default) reuses pull request data for up to a minute; fresh refetches it; off skips the tracker and pull request lookups: local git facts only, offline")
}

func (cm *statusCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *statusCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, change, all, asJSON, state, everything := cm.openspec, cm.change, cm.all, cm.asJSON, cm.state, cm.everything
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)
	if !validState(*state) {
		fail(fmt.Errorf("-state must be on, fresh or off, got %q", *state))
	}

	ctx := context.Background()

	// With no -change, status is the current work. Outside a git worktree it
	// falls back to every change, with a status line saying so. -all (include
	// archived) and -everything list changes rather than the current work.
	dir := ""
	current := *change == "" && !*everything && !*all
	if current {
		dir, _ = os.Getwd()
	}

	topo, err := specsync.BuildTopology(ctx, specsync.TopologyOptions{
		OpenSpecDir:     *openspec,
		Slug:            *change,
		All:             *all,
		Dir:             dir,
		LocalOnly:       *state == "off",
		Cache:           requestCache(),
		RefreshRequests: *state == "fresh",
	})
	if err != nil {
		fail(err)
	}

	switch {
	case *asJSON:
		emitJSON(topo)
	case current && len(topo.Changes) == 1:
		printCurrentStatus(topo.Changes[0], *state != "off")
		for _, s := range topo.Status {
			fmt.Println("status:", s)
		}
	default:
		printTopologyTable(*topo)
	}

	if len(topo.Changes) == 0 {
		os.Exit(1)
	}
}

// requestLine summarises a change request on one line.
func requestLine(pr *specsync.ChangeRequest) string {
	line := fmt.Sprintf("pr #%d %s", pr.Number, pr.State)
	if pr.Draft {
		return line + " (draft)"
	}
	line += fmt.Sprintf(": checks %d pass, %d fail, %d pending", pr.Checks.Pass, pr.Checks.Fail, pr.Checks.Pending)
	if pr.ReviewDecision != "" {
		line += "; review " + pr.ReviewDecision
	}
	if pr.Mergeable != "" && pr.Mergeable != "unknown" {
		line += "; " + pr.Mergeable
	}
	if n := pr.UnresolvedThreads; n != nil && *n > 0 {
		line += fmt.Sprintf("; %d unresolved threads", *n)
	}
	return line
}

// validState reports whether v is an accepted -state value.
func validState(v string) bool { return v == "" || v == "on" || v == "off" || v == "fresh" }

func printTopologyTable(topo specsync.Topology) {
	if len(topo.Changes) == 0 {
		fmt.Println("No changes resolved.")
	} else {
		fmt.Println("SLUG                            STAGE       REPO                       ISSUE       BRANCH                                    WORKTREE")
		fmt.Println("──────────────────────────────  ──────────  ─────────────────────────  ──────────  ────────────────────────────────────────  ────────────────────────────────────────")
		for _, c := range topo.Changes {
			issue := "-"
			if c.Issue != nil {
				issue = c.Issue.ID
				if c.Issue.State != "" {
					issue += " (" + c.Issue.State + ")"
				}
			}
			branch := c.Branch
			if branch == "" {
				branch = "-"
			}
			worktree := c.Worktree
			if worktree == "" {
				worktree = "-"
			}
			repo := c.Repo
			if repo == "" {
				repo = "-"
			}
			fmt.Printf("%-32s %-11s %-27s %-11s %-41s %s\n", c.Slug, c.Stage, repo, issue, branch, worktree)
			if len(c.Attention) > 0 {
				fmt.Printf("    ↳ attention: %s\n", strings.Join(c.Attention, ", "))
			}
			if pr := c.PR; pr != nil {
				fmt.Printf("    ↳ %s\n", requestLine(pr))
				if len(pr.Checks.Failing) > 0 {
					fmt.Printf("        failing checks: %s\n", strings.Join(pr.Checks.Failing, ", "))
				}
			}
			if c.Sync != nil {
				fmt.Printf("    ↳ suggest: %s\n", c.Sync.Summary)
				for _, cmd := range c.Sync.Commands {
					fmt.Printf("        $ %s\n", cmd)
				}
			}
			if l := c.Local; l != nil {
				if len(l.BaseTouchesYourFiles) > 0 {
					fmt.Printf("    ↳ base touches your files: %s\n", strings.Join(l.BaseTouchesYourFiles, ", "))
				}
				if len(l.Conflicts) > 0 {
					fmt.Printf("    ↳ would conflict in: %s\n", strings.Join(l.Conflicts, ", "))
				}
				if l.ConflictCheckUnavailable {
					fmt.Println("    ↳ conflict check unavailable (needs git 2.38 or newer)")
				}
			}
			if c.Epic != "" {
				fmt.Printf("    ↳ epic: %s\n", c.Epic)
			}
			if len(c.Linked) > 0 {
				fmt.Printf("    ↳ linked: %s\n", joinCommaOr(c.Linked, "-"))
			}
		}
	}
	for _, s := range topo.Status {
		fmt.Println("status:", s)
	}
}

func joinCommaOr(vals []string, empty string) string {
	if len(vals) == 0 {
		return empty
	}
	out := vals[0]
	for _, v := range vals[1:] {
		out += ", " + v
	}
	return out
}

// requestCache returns the on-disk request cache, or nil (no caching) when the
// user has no cache directory. A nil *FileRequestCache must not become a
// non-nil interface.
func requestCache() specsync.RequestCache {
	if c := specsync.DefaultRequestCache(); c != nil {
		return c
	}
	return nil
}

// printCurrentStatus prints the detail view of one row: attention first, then
// the change, issue, branch, request and local facts. requestsKnown is false
// when request lookups were skipped, so an absent request is unknown, not none.
func printCurrentStatus(row specsync.ChangeTopology, requestsKnown bool) {
	if row.NoChange {
		fmt.Printf("no change for branch %s\n", orDash(row.Branch))
	} else {
		head := fmt.Sprintf("%s  %s", row.Slug, row.Stage)
		if row.Tasks != nil {
			head += fmt.Sprintf("  %d/%d tasks", row.Tasks.Done, row.Tasks.Total)
		}
		fmt.Println(head)
	}
	if len(row.Attention) > 0 {
		parts := make([]string, len(row.Attention))
		for i, f := range row.Attention {
			parts[i] = attentionDetail(f, row)
		}
		fmt.Printf("attention: %s\n", strings.Join(parts, ", "))
	}
	if row.Issue != nil {
		issue := "#" + row.Issue.ID
		if row.Issue.State != "" {
			issue += " (" + row.Issue.State + ")"
		}
		fmt.Printf("issue:   %s  %s\n", issue, row.Issue.URL)
	}
	branch := orDash(row.Branch)
	if l := row.Local; l != nil {
		if l.Upstream == "" {
			branch += " (not pushed)"
		} else {
			branch += " -> " + l.Upstream
		}
	}
	fmt.Printf("branch:  %s\n", branch)
	if row.Worktree != "" {
		fmt.Printf("path:    %s\n", row.Worktree)
	}
	switch {
	case row.PR != nil:
		fmt.Printf("%s\n", requestLine(row.PR))
		if len(row.PR.Checks.Failing) > 0 {
			fmt.Printf("         failing checks: %s\n", strings.Join(row.PR.Checks.Failing, ", "))
		}
	case requestsKnown:
		fmt.Println("pr: none")
	}
	if l := row.Local; l != nil {
		fmt.Printf("local:   %d modified, %d untracked; %d ahead, %d behind %s", l.Modified, l.Untracked, l.Ahead, l.UpstreamBehind, orDash(l.Upstream))
		if l.Base != "" {
			fmt.Printf("; %d behind %s", l.BehindBase, l.Base)
		}
		fmt.Println()
		if s := l.Size; s != nil {
			fmt.Printf("size:    %d files, +%d -%d, %d commits\n", s.Files, s.Additions, s.Deletions, s.Commits)
		}
		for _, f := range l.BaseTouchesYourFiles {
			fmt.Printf("         base also changed: %s\n", f)
		}
		for _, f := range l.Conflicts {
			fmt.Printf("         would conflict in: %s\n", f)
		}
		if l.ConflictCheckUnavailable {
			fmt.Println("         conflict check unavailable (needs git 2.38 or newer)")
		}
	}
	if row.Sync != nil {
		fmt.Printf("suggest: %s\n", row.Sync.Summary)
		for _, c := range row.Sync.Commands {
			fmt.Printf("           $ %s\n", c)
		}
	}
}

// attentionDetail adds the count a flag stands for, when it has one.
func attentionDetail(flag string, row specsync.ChangeTopology) string {
	plural := func(n int, one string) string {
		if n == 1 {
			return fmt.Sprintf("%d %s", n, one)
		}
		return fmt.Sprintf("%d %ss", n, one)
	}
	l, pr := row.Local, row.PR
	switch {
	case flag == "unpushed" && l != nil:
		return fmt.Sprintf("%s (%s)", flag, plural(l.Ahead, "commit"))
	case flag == "uncommitted" && l != nil:
		return fmt.Sprintf("%s (%s)", flag, plural(l.Modified+l.Untracked, "file"))
	case flag == "behind-upstream" && l != nil:
		return fmt.Sprintf("%s (%s)", flag, plural(l.UpstreamBehind, "commit"))
	case flag == "behind-base" && l != nil:
		return fmt.Sprintf("%s (%s)", flag, plural(l.BehindBase, "commit"))
	case flag == "ci-failing" && pr != nil:
		return fmt.Sprintf("%s (%d failing)", flag, pr.Checks.Fail)
	case flag == "unresolved-comments" && pr != nil && pr.UnresolvedThreads != nil:
		return fmt.Sprintf("%s (%s)", flag, plural(*pr.UnresolvedThreads, "thread"))
	}
	return flag
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
