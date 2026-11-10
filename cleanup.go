package specsync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	CleanupCandidate = "candidate"
	CleanupSkipped   = "skipped"
	CleanupAttention = "attention"
)

// CleanupItem is one worktree cleanup looked at and what it decided.
type CleanupItem struct {
	Status   string   `json:"status"` // candidate, skipped or attention
	Reason   string   `json:"reason"`
	Repo     string   `json:"repo,omitempty"`
	Slug     string   `json:"slug,omitempty"`
	Branch   string   `json:"branch"`
	Worktree string   `json:"worktree"`
	Steps    []string `json:"steps,omitempty"`
	Done     []string `json:"done,omitempty"`
	Error    string   `json:"error,omitempty"`

	row ChangeTopology
}

// CleanupOptions says what cleanup may look at and do. The zero value plans
// against the real clock and changes nothing.
type CleanupOptions struct {
	Overview OverviewOptions
	Apply    bool
	// Current is the directory cleanup runs from; its worktree is never touched.
	Current string
	Now     func() time.Time
	// MinAgeDays gives the repo's cleanup.min_age_days for a clone path.
	MinAgeDays func(store string) int
	// ArchiveChange archives a change whose issue is closed. nil skips the step.
	ArchiveChange func(ctx context.Context, row ChangeTopology) (string, error)
	// IncludeIgnored lets a worktree with ignored files (other than common
	// build output) be removed.
	IncludeIgnored bool
	// RunHook runs one configured hook in dir. nil uses the shell.
	RunHook func(ctx context.Context, dir, command string, env []string) error
}

// CleanupReport is the result of PlanCleanup plus, with Apply, its effects.
type CleanupReport struct {
	Apply  bool          `json:"apply"`
	Items  []CleanupItem `json:"items"`
	Status []string      `json:"status,omitempty"`
	// SessionsNote is set when live sessions could not be checked, so no
	// worktree was protected by them.
	SessionsNote string `json:"sessionsNote,omitempty"`
}

// PlanCleanup decides, for every row with a linked worktree, whether its work
// is provably in the base (a candidate), whether something blocks it (skipped),
// or whether it might be lost work (attention). It only reads.
func PlanCleanup(ctx context.Context, rows []ChangeTopology, opts CleanupOptions) []CleanupItem {
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	shared := sessionsByWorktree(rows)
	var items []CleanupItem
	for _, row := range rows {
		if row.Worktree != "" {
			row.Sessions = shared[resolved(absPath(row.Worktree))]
		}
		if it, ok := classifyCleanup(ctx, row, opts, now()); ok {
			items = append(items, it)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Status != items[j].Status {
			return items[i].Status < items[j].Status
		}
		return items[i].Worktree < items[j].Worktree
	})
	return items
}

func classifyCleanup(ctx context.Context, row ChangeTopology, opts CleanupOptions, now time.Time) (CleanupItem, bool) {
	it := CleanupItem{Repo: row.Repo, Slug: row.Slug, Branch: row.Branch, Worktree: row.Worktree, row: row}
	ls := row.Local
	if row.Worktree == "" || row.Branch == "" || ls == nil || ls.Head == "" {
		return it, false
	}
	if SameFolder(row.Worktree, row.Store) {
		return it, false
	}
	pr := row.PR
	if pr != nil && pr.State == "open" {
		return it, false
	}

	mergedHead := pr != nil && pr.State == "merged" && pr.HeadSHA != "" && ls.Head == pr.HeadSHA
	inBase := false
	if ls.Base != "" {
		if _, err := gitIn(ctx, row.Worktree, "merge-base", "--is-ancestor", ls.Head, ls.Base); err == nil {
			inBase = true
		}
	}
	ownCommits := inBase && branchHasCommits(ctx, row.Worktree, row.Branch)
	unpushed := unpushedCommits(ctx, row.Worktree, ls, mergedHead)

	closedUnmerged := pr != nil && pr.State == "closed"
	provable := (mergedHead || ownCommits) && !closedUnmerged

	if !provable {
		return notFinished(ctx, it, row, opts, now, unpushed, inBase, closedUnmerged)
	}

	it.Reason = "its work is in " + ls.Base
	if mergedHead {
		it.Reason = fmt.Sprintf("request #%d merged with this head", pr.Number)
	}
	if reason := blockingReason(ctx, row, opts); reason != "" {
		it.Status, it.Reason = CleanupSkipped, reason
		return it, true
	}
	it.Status = CleanupCandidate
	it.Steps = cleanupSteps(row)
	return it, true
}

func notFinished(ctx context.Context, it CleanupItem, row ChangeTopology, opts CleanupOptions, now time.Time, unpushed int, inBase, closedUnmerged bool) (CleanupItem, bool) {
	ls, pr := row.Local, row.PR
	switch {
	case closedUnmerged && inBase && unpushed == 0:
		it.Status, it.Reason = CleanupSkipped, fmt.Sprintf("request #%d was closed without merging; never removed automatically", pr.Number)
		return it, true
	case unpushed == 0:
		return it, false
	}
	detail := fmt.Sprintf("%d unpushed commit(s) not in %s", unpushed, ls.Base)
	if ls.UpstreamGone {
		detail += "; the upstream branch is gone"
	}
	if pr != nil && pr.State == "merged" && pr.HeadSHA != "" {
		if names := commitsAfter(ctx, row.Worktree, pr.HeadSHA); len(names) > 0 {
			it.Status = CleanupAttention
			it.Reason = "commits after the merge: " + strings.Join(names, "; ")
			return it, true
		}
	}
	if closedUnmerged {
		it.Status, it.Reason = CleanupAttention, fmt.Sprintf("request #%d closed without merging; %s", pr.Number, detail)
		return it, true
	}
	var minAge int
	if opts.MinAgeDays != nil {
		minAge = opts.MinAgeDays(row.Store)
	} else {
		cfg, _ := LoadWays(row.Store)
		minAge = cfg.Cleanup.MinAgeDays
	}
	if minAge > 0 {
		last, err := time.Parse(time.RFC3339, ls.LastCommit)
		if err != nil || now.Sub(last) < time.Duration(minAge)*24*time.Hour {
			return it, false
		}
	}
	it.Status, it.Reason = CleanupAttention, detail
	return it, true
}

// unpushedCommits counts commits not reachable from the base and not on the
// upstream. When the upstream is gone every commit outside the base counts,
// unless the head is the one the request merged with.
func unpushedCommits(ctx context.Context, dir string, ls *LocalState, mergedHead bool) int {
	if mergedHead {
		return 0
	}
	args := []string{"rev-list", "--count", ls.Head}
	if ls.Base != "" {
		args = append(args, "^"+ls.Base)
	}
	if ls.Upstream != "" && !ls.UpstreamGone {
		args = append(args, "^"+ls.Upstream)
	}
	out, err := gitIn(ctx, dir, args...)
	if err != nil {
		return 0
	}
	n := 0
	fmt.Sscanf(out, "%d", &n)
	return n
}

func commitsAfter(ctx context.Context, dir, mergedHead string) []string {
	out, err := gitIn(ctx, dir, "log", "--format=%h %s", "HEAD", "^"+mergedHead)
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func blockingReason(ctx context.Context, row ChangeTopology, opts CleanupOptions) string {
	ls := row.Local
	switch {
	case protectedBranch(ctx, row) != "":
		return protectedBranch(ctx, row)
	case ls.Modified+ls.Untracked > 0:
		return fmt.Sprintf("uncommitted changes: %d modified, %d untracked", ls.Modified, ls.Untracked)
	case opts.Current != "" && insideOrSame(opts.Current, row.Worktree):
		return "it is the current worktree"
	case len(row.Sessions) > 0:
		return "a live session is working in it: " + sessionName(row.Sessions[0])
	}
	if !opts.IncludeIgnored {
		if names := ignoredFiles(ctx, row.Worktree); len(names) > 0 {
			return "has ignored files: " + strings.Join(names, ", ") + " (they would be lost; -include-ignored overrides)"
		}
	}
	return ""
}

var protectedNames = []string{"main", "master", "develop", "trunk"}

// protectedBranch explains why a branch must never be removed: it is a
// well-known trunk name, the base or default branch, or the one checked out in
// the main checkout.
func protectedBranch(ctx context.Context, row ChangeTopology) string {
	b := row.Branch
	protected := append([]string(nil), protectedNames...)
	if base := row.Local.Base; base != "" {
		protected = append(protected, strings.TrimPrefix(base, "origin/"))
	}
	if out, err := gitIn(ctx, row.Store, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		protected = append(protected, strings.TrimPrefix(out, "origin/"))
	}
	for _, p := range protected {
		if p == b {
			return "protected branch " + b + ": it is a base or default branch"
		}
	}
	if out, err := gitIn(ctx, row.Store, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && out == b {
		return "protected branch " + b + ": it is checked out in the main checkout"
	}
	return ""
}

// defaultIgnored are build output folders whose loss costs only a rebuild.
var defaultIgnored = []string{"node_modules", "dist", "build", "target", ".next", "vendor", "__pycache__"}

// ignoredFiles lists ignored paths in the worktree other than common build
// output, a few at most.
func ignoredFiles(ctx context.Context, dir string) []string {
	out, err := gitIn(ctx, dir, "--no-optional-locks", "status", "--porcelain=v1", "--ignored")
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "!! ") {
			continue
		}
		p := strings.TrimSuffix(strings.TrimPrefix(line, "!! "), "/")
		skip := false
		for _, part := range strings.Split(p, "/") {
			if slices.Contains(defaultIgnored, part) {
				skip = true
			}
		}
		if !skip {
			names = append(names, SanitizeText(p, 60))
		}
	}
	if len(names) > 3 {
		names = append(names[:3], fmt.Sprintf("and %d more", len(names)-3))
	}
	return names
}

// sessionsByWorktree gathers the sessions of every row that shares a worktree,
// so each of those rows sees all of them.
func sessionsByWorktree(rows []ChangeTopology) map[string][]SessionEntry {
	out := map[string][]SessionEntry{}
	for _, r := range rows {
		if r.Worktree == "" || len(r.Sessions) == 0 {
			continue
		}
		key := resolved(absPath(r.Worktree))
		out[key] = append(out[key], r.Sessions...)
	}
	return out
}

// sessionsCheckNote says why live sessions were not checked, or "" when they were.
func sessionsCheckNote(o OverviewOptions, status []string) string {
	switch {
	case o.Topology.LocalOnly:
		return "live sessions not checked (-state off); a worktree in use by an agent is not protected by that check"
	case strings.TrimSpace(o.SessionsCmd) == "":
		return "live sessions not checked: no session provider configured (-sessions-cmd or SPECSYNC_SESSIONS_CMD); a worktree in use by an agent is not protected by that check"
	}
	for _, s := range status {
		if strings.Contains(s, "session provider") {
			return "live sessions not checked: " + s
		}
	}
	return ""
}

func sessionName(s SessionEntry) string {
	for _, v := range []string{s.Name, s.ID, s.CWD} {
		if v != "" {
			return SanitizeText(v, 60)
		}
	}
	return "session"
}

// branchHasCommits reports whether the branch's reflog records a commit made
// on it. A branch that points at the base tip because it was created, reset,
// pulled or rebased there is new work, not finished work.
func branchHasCommits(ctx context.Context, dir, branch string) bool {
	out, err := gitIn(ctx, dir, "reflog", "show", "--format=%gs", "refs/heads/"+branch)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "commit") {
			return true
		}
	}
	return false
}

// cleanupSteps names what applying would do for a candidate, from the repo's
// config. Hooks appear only as text here; whether they run is decided at apply
// time by trust.
func cleanupSteps(row ChangeTopology) []string {
	cfg, _ := LoadWays(row.Store)
	var steps []string
	for _, h := range cfg.Cleanup.Hooks {
		if IsTrusted(row.Store) {
			steps = append(steps, "run hook in the worktree: "+h)
		} else {
			steps = append(steps, "hook not run, config not trusted (specsync trust): "+h)
		}
	}
	if cfg.Cleanup.Archive && row.Slug != "" && row.Issue != nil && row.Issue.State == "closed" {
		steps = append(steps, "archive change "+row.Slug)
	}
	steps = append(steps, "remove worktree "+row.Worktree, "delete local branch "+row.Branch)
	if cfg.Cleanup.RemoteBranch == "delete" && !row.Local.UpstreamGone {
		steps = append(steps, "delete remote branch "+remoteOf(row)+"/"+row.Branch+" if its tip is still this head")
	}
	return steps
}

func remoteOf(row ChangeTopology) string {
	if up := row.Local.Upstream; strings.Contains(up, "/") {
		return strings.SplitN(up, "/", 2)[0]
	}
	return "origin"
}

// RunCleanup collects the work under the workspace roots, plans the cleanup
// and, with opts.Apply, carries out the candidates.
func RunCleanup(ctx context.Context, opts CleanupOptions) (*CleanupReport, error) {
	rows, err := CollectWorkRows(ctx, opts.Overview)
	if err != nil {
		return nil, err
	}
	items := PlanCleanup(ctx, rows.Rows, opts)
	if opts.Apply {
		items = ApplyCleanup(ctx, items, opts)
	}
	return &CleanupReport{Apply: opts.Apply, Items: items, Status: rows.Status, SessionsNote: sessionsCheckNote(opts.Overview, rows.Status)}, nil
}

// ApplyCleanup carries out the candidates in items and returns every item with
// what was done. Only candidates are touched, and each one is checked again
// right before it is removed: work added since planning stops the removal.
func ApplyCleanup(ctx context.Context, items []CleanupItem, opts CleanupOptions) []CleanupItem {
	out := make([]CleanupItem, len(items))
	for i, it := range items {
		if it.Status == CleanupCandidate {
			it = applyOne(ctx, it, opts)
		}
		out[i] = it
	}
	return out
}

// applyOne does one candidate in a fixed order, stopping at the first failure:
//  1. check it again from fresh facts (protected branch, ignored files, head,
//     status, sessions, proof that its work is in the base);
//  2. run the configured hooks in the worktree, only when the config is trusted;
//  3. archive the change when configured and its issue is closed;
//  4. remove the worktree and delete the local branch;
//  5. delete the remote branch when configured and its tip is still ours.
//
// A failure in 1 to 3 leaves the worktree and branch untouched.
func applyOne(ctx context.Context, it CleanupItem, opts CleanupOptions) CleanupItem {
	fail := func(format string, a ...any) CleanupItem {
		it.Error = fmt.Sprintf(format, a...)
		return it
	}
	row, reason := recheck(ctx, it.row, opts)
	if reason != "" {
		return fail("%s; nothing removed", reason)
	}
	cfg, _ := LoadWays(row.Store)

	var hookNote string
	if hooks := cfg.Cleanup.Hooks; len(hooks) > 0 {
		if err := RequireTrust(row.Store, hooks); err != nil {
			hookNote = "hooks not run: " + err.Error()
		} else {
			run := opts.RunHook
			if run == nil {
				run = runShellHook
			}
			env := []string{"SPECSYNC_WORKTREE=" + row.Worktree, "SPECSYNC_BRANCH=" + row.Branch, "SPECSYNC_CHANGE=" + row.Slug}
			for _, h := range hooks {
				if err := run(ctx, row.Worktree, h, env); err != nil {
					return fail("hook %q: %v; the worktree and branch were left in place", h, err)
				}
				it.Done = append(it.Done, "ran hook: "+h)
			}
		}
	}

	if cfg.Cleanup.Archive && row.Slug != "" && row.Issue != nil && row.Issue.State == "closed" && opts.ArchiveChange != nil {
		msg, err := opts.ArchiveChange(ctx, row)
		if err != nil {
			return fail("archive %s: %v; the worktree and branch were left in place", row.Slug, err)
		}
		it.Done = append(it.Done, msg)
	}

	if _, err := gitIn(ctx, row.Store, "worktree", "remove", row.Worktree); err != nil {
		return fail("%v", err)
	}
	it.Done = append(it.Done, "removed worktree "+row.Worktree)

	if _, err := gitIn(ctx, row.Store, "branch", "-d", row.Branch); err != nil {
		if _, err := gitIn(ctx, row.Store, "branch", "-D", row.Branch); err != nil {
			return fail("%v", err)
		}
	}
	it.Done = append(it.Done, "deleted local branch "+row.Branch)

	if cfg.Cleanup.RemoteBranch == "delete" {
		it.Done = append(it.Done, deleteRemoteBranch(ctx, row))
	}
	if hookNote != "" {
		it.Done = append(it.Done, hookNote)
	}
	return it
}

// recheck re-runs the classification from fresh facts and returns the row as
// it is now, or the reason it is no longer a candidate.
func recheck(ctx context.Context, row ChangeTopology, opts CleanupOptions) (ChangeTopology, string) {
	if row.Worktree == "" || row.Local == nil {
		return row, "no worktree to check"
	}
	planned := row.Local.Head
	ls, err := CollectLocalState(ctx, row.Worktree, LocalStateOptions{Base: row.Local.Base})
	if err != nil {
		return row, err.Error()
	}
	if ls.Head != planned {
		return row, "the branch moved since planning"
	}
	row.Local = ls
	if opts.Overview.SessionsCmd != "" && !opts.Overview.Topology.LocalOnly {
		if entries, err := LoadSessions(ctx, opts.Overview.SessionsCmd, opts.Overview.SessionsTimeout); err == nil {
			row.Sessions = nil
			for _, e := range entries {
				if filepath.IsAbs(e.CWD) && insideOrSame(e.CWD, row.Worktree) {
					row.Sessions = append(row.Sessions, e)
				}
			}
		}
	}
	it, ok := classifyCleanup(ctx, row, opts, time.Now())
	if !ok {
		return row, "it is no longer finished work"
	}
	if it.Status != CleanupCandidate {
		return row, it.Reason
	}
	return row, ""
}

// deleteRemoteBranch deletes the branch on its remote only when the upstream
// is the same remote and branch name and the remote tip, read now, is the head
// being removed or the head the request merged with.
func deleteRemoteBranch(ctx context.Context, row ChangeTopology) string {
	ls := row.Local
	keep := func(why string) string { return "kept remote branch: " + why }
	if ls.UpstreamGone {
		return "remote branch already gone"
	}
	remote := remoteOf(row)
	if ls.Upstream != remote+"/"+row.Branch {
		return keep("its upstream is " + orNone(ls.Upstream) + ", not a branch of the same name")
	}
	out, err := gitIn(ctx, row.Store, "ls-remote", remote, "refs/heads/"+row.Branch)
	if err != nil {
		return keep("could not read the remote tip: " + err.Error())
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "remote branch already gone"
	}
	tip := fields[0]
	if tip != ls.Head && (row.PR == nil || tip != row.PR.HeadSHA) {
		return keep("the remote tip " + shortSHA(tip) + " is not the head being removed")
	}
	if _, err := gitIn(ctx, row.Store, "push", remote, "--delete", row.Branch); err != nil {
		return keep(err.Error())
	}
	return "deleted remote branch " + remote + "/" + row.Branch
}

func orNone(s string) string {
	if s == "" {
		return "not set"
	}
	return s
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func runShellHook(ctx context.Context, dir, command string, env []string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
