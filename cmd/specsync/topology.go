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
		Safety:   []string{"Never modifies your worktree, index, refs or the tracker; it may write a short-lived request cache outside the repo, and git merge-tree may add unreferenced objects"},
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

	var related *specsync.RelatedWork
	if current && len(topo.Changes) == 1 && hasRelations(topo.Changes[0]) {
		related = relatedWork(ctx, topo.Changes[0], *state)
	}

	switch {
	case *asJSON:
		emitJSON(struct {
			*specsync.Topology
			Related *specsync.RelatedWork `json:"related,omitempty"`
		}{topo, related})
	case current && len(topo.Changes) == 1:
		printCurrentStatus(topo.Changes[0], topo.RequestState == "ok")
		printRelated(related)
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

// printRequest prints a request's summary line and, when checks fail, their
// names. lead starts the first line; indent starts the continuation.
func printRequest(pr *specsync.ChangeRequest, lead, indent string) {
	if pr == nil {
		return
	}
	fmt.Printf("%s%s\n", lead, requestLine(pr))
	if len(pr.Checks.Failing) > 0 {
		fmt.Printf("%sfailing checks: %s\n", indent, strings.Join(pr.Checks.Failing, ", "))
	}
}

// printSync prints the sync suggestion and its commands as text.
func printSync(s *specsync.SyncAdvice, lead, indent string) {
	if s == nil {
		return
	}
	fmt.Printf("%ssuggest: %s\n", lead, s.Summary)
	for _, c := range s.Commands {
		fmt.Printf("%s$ %s\n", indent, c)
	}
}

// printFreshness prints which files the base also changed and where a merge
// would conflict.
func printFreshness(l *specsync.LocalState, lead string) {
	if l == nil {
		return
	}
	if len(l.BaseTouchesYourFiles) > 0 {
		fmt.Printf("%sbase touches your files: %s\n", lead, strings.Join(l.BaseTouchesYourFiles, ", "))
	}
	if len(l.Conflicts) > 0 {
		fmt.Printf("%swould conflict in: %s\n", lead, strings.Join(l.Conflicts, ", "))
	}
	if l.ConflictCheckUnavailable {
		fmt.Printf("%sconflict check unavailable (needs git 2.38 or newer)\n", lead)
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
			printRequest(c.PR, "    ↳ ", "        ")
			printSync(c.Sync, "    ↳ ", "        ")
			printFreshness(c.Local, "    ↳ ")
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
	if row.PR != nil {
		printRequest(row.PR, "", "         ")
	} else if requestsKnown {
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
		printFreshness(l, "         ")
	}
	printSync(row.Sync, "", "           ")
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

// hasRelations reports whether the row asserts anything worth a workspace walk.
func hasRelations(row specsync.ChangeTopology) bool {
	return row.Epic != "" || len(row.BlockedBy) > 0 || len(row.Blocks) > 0 || len(row.Linked) > 0 || len(row.LinkedRefs) > 0
}

// relatedWork finds what relates to the current work across the workspace.
// Sources that fail are carried as reasons, never hidden.
func relatedWork(ctx context.Context, current specsync.ChangeTopology, state string) *specsync.RelatedWork {
	work, err := specsync.CollectWorkRows(ctx, specsync.OverviewOptions{
		Topology: specsync.TopologyOptions{
			LocalOnly:       state == "off",
			Cache:           requestCache(),
			RefreshRequests: state == "fresh",
		},
	})
	if err != nil {
		return &specsync.RelatedWork{Current: specsync.RowKey(current), Incomplete: []string{err.Error()}}
	}
	rows := make([]specsync.ChangeTopology, 0, len(work.Rows)+1)
	key, found := specsync.RowKey(current), false
	for _, r := range work.Rows {
		if specsync.RowKey(r) == key && specsync.SameFolder(r.Store, current.Store) {
			r, found = current, true
		}
		rows = append(rows, r)
	}
	if !found {
		rows = append(rows, current)
	}
	rel := specsync.RelatedTo(current, rows, specsync.IncompleteReasons(work.Status))
	if reasons := specsync.IncompleteReasons(work.Status); rel == nil && len(reasons) > 0 {
		rel = &specsync.RelatedWork{Current: key, Incomplete: reasons}
	}
	return rel
}

// printRelated prints the epic chain, siblings and blockers around the current
// work and marks it. The next step is the suggestion printed with the work itself.
func printRelated(rel *specsync.RelatedWork) {
	if rel == nil {
		return
	}
	fmt.Println()
	if e := rel.Epic; e != nil {
		fmt.Printf("epic %s  %d of %d known done\n", e.Key, e.Done, e.Total)
		for _, up := range e.Above {
			fmt.Printf("  above: %s\n", up)
		}
		for i, it := range e.Items {
			branch := "├─ "
			if i == len(e.Items)-1 {
				branch = "└─ "
			}
			printRelatedItem(it, branch, rel.Current)
		}
	}
	for _, group := range []struct {
		title string
		items []specsync.RelatedItem
	}{{"blocked by", rel.BlockedBy}, {"blocks", rel.Blocks}, {"linked", rel.Linked}} {
		if len(group.items) == 0 {
			continue
		}
		fmt.Println(group.title + ":")
		for _, it := range group.items {
			printRelatedItem(it, "  ", rel.Current)
		}
	}
	for _, reason := range rel.Incomplete {
		fmt.Println("incomplete:", reason)
	}
}

func printRelatedItem(it specsync.RelatedItem, branch, current string) {
	if it.Row == nil {
		fmt.Printf("%s%s  (not found in the workspace)\n", branch, it.Key)
		return
	}
	mark := ""
	if it.Key == current {
		mark = "◄ you are here"
	}
	printWorkRow(*it.Row, branch, mark)
}
