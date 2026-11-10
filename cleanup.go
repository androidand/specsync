package specsync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	// RunHook runs one configured hook in dir. nil uses the shell.
	RunHook func(ctx context.Context, dir, command string, env []string) error
}

// CleanupReport is the result of PlanCleanup plus, with Apply, its effects.
type CleanupReport struct {
	Apply  bool          `json:"apply"`
	Items  []CleanupItem `json:"items"`
	Status []string      `json:"status,omitempty"`
}

// PlanCleanup decides, for every row with a linked worktree, whether its work
// is provably in the base (a candidate), whether something blocks it (skipped),
// or whether it might be lost work (attention). It only reads.
func PlanCleanup(ctx context.Context, rows []ChangeTopology, opts CleanupOptions) []CleanupItem {
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	var items []CleanupItem
	for _, row := range rows {
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
	baseTip := ""
	if ls.Base != "" {
		if _, err := gitIn(ctx, row.Worktree, "merge-base", "--is-ancestor", ls.Head, ls.Base); err == nil {
			inBase = true
		}
		baseTip, _ = gitIn(ctx, row.Worktree, "rev-parse", ls.Base)
	}
	ownCommits := inBase && (ls.Head != baseTip || branchHasHistory(ctx, row.Worktree, row.Branch))
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
	if reason := blockingReason(row, opts); reason != "" {
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

func blockingReason(row ChangeTopology, opts CleanupOptions) string {
	ls := row.Local
	switch {
	case ls.Modified+ls.Untracked > 0:
		return fmt.Sprintf("uncommitted changes: %d modified, %d untracked", ls.Modified, ls.Untracked)
	case opts.Current != "" && (SameFolder(opts.Current, row.Worktree) || within(opts.Current, row.Worktree)):
		return "it is the current worktree"
	case len(row.Sessions) > 0:
		return "a live session is working in it: " + sessionName(row.Sessions[0])
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

func within(child, parent string) bool {
	c, p := realPath(child), realPath(parent)
	rel, err := filepath.Rel(p, c)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func realPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	rest := ""
	for q := p; ; q = filepath.Dir(q) {
		if r, err := filepath.EvalSymlinks(q); err == nil {
			return filepath.Join(r, rest)
		}
		rest = filepath.Join(filepath.Base(q), rest)
		if filepath.Dir(q) == q {
			return p
		}
	}
}

// branchHasHistory reports whether the branch ever received a commit of its
// own. A branch that points at the base tip and was only ever created there
// is new work, not finished work.
func branchHasHistory(ctx context.Context, dir, branch string) bool {
	out, err := gitIn(ctx, dir, "reflog", "show", "--format=%gs", "refs/heads/"+branch)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if line != "" && !strings.HasPrefix(line, "branch: Created") {
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
	steps := []string{"remove worktree " + row.Worktree, "delete local branch " + row.Branch}
	if cfg.Cleanup.RemoteBranch == "delete" && !row.Local.UpstreamGone {
		steps = append(steps, "delete remote branch "+remoteOf(row)+"/"+row.Branch)
	}
	if cfg.Cleanup.Archive && row.Slug != "" && row.Issue != nil && row.Issue.State == "closed" {
		steps = append(steps, "archive change "+row.Slug)
	}
	for _, h := range cfg.Cleanup.Hooks {
		if IsTrusted(row.Store) {
			steps = append(steps, "run hook: "+h)
		} else {
			steps = append(steps, "hook not run, config not trusted (specsync trust): "+h)
		}
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
	return &CleanupReport{Apply: opts.Apply, Items: items, Status: rows.Status}, nil
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

func applyOne(ctx context.Context, it CleanupItem, opts CleanupOptions) CleanupItem {
	row := it.row
	fail := func(format string, a ...any) CleanupItem {
		it.Error = fmt.Sprintf(format, a...)
		return it
	}
	if SameFolder(row.Worktree, row.Store) {
		return fail("refusing to remove the main checkout")
	}
	if opts.Current != "" && (SameFolder(opts.Current, row.Worktree) || within(opts.Current, row.Worktree)) {
		return fail("refusing to remove the current worktree")
	}
	status, err := gitIn(ctx, row.Worktree, "--no-optional-locks", "status", "--porcelain=v1")
	if err != nil {
		return fail("%v", err)
	}
	if status != "" {
		return fail("uncommitted changes appeared since planning; nothing removed")
	}
	head, err := gitIn(ctx, row.Worktree, "rev-parse", "HEAD")
	if err != nil || head != row.Local.Head {
		return fail("the branch moved since planning; nothing removed")
	}
	cfg, _ := LoadWays(row.Store)

	if _, err := gitIn(ctx, row.Store, "worktree", "remove", row.Worktree); err != nil {
		return fail("%v", err)
	}
	it.Done = append(it.Done, "removed worktree "+row.Worktree)

	if _, err := gitIn(ctx, row.Store, "branch", "-D", row.Branch); err != nil {
		return fail("%v", err)
	}
	it.Done = append(it.Done, "deleted local branch "+row.Branch)

	if cfg.Cleanup.RemoteBranch == "delete" {
		switch {
		case row.Local.UpstreamGone:
			it.Done = append(it.Done, "remote branch already gone")
		default:
			remote := remoteOf(row)
			if _, err := gitIn(ctx, row.Store, "push", remote, "--delete", row.Branch); err != nil {
				return fail("%v", err)
			}
			it.Done = append(it.Done, "deleted remote branch "+remote+"/"+row.Branch)
		}
	}

	if cfg.Cleanup.Archive && row.Slug != "" && row.Issue != nil && row.Issue.State == "closed" && opts.ArchiveChange != nil {
		msg, err := opts.ArchiveChange(ctx, row)
		if err != nil {
			return fail("archive %s: %v", row.Slug, err)
		}
		it.Done = append(it.Done, msg)
	}

	if len(cfg.Cleanup.Hooks) > 0 {
		if !IsTrusted(row.Store) {
			for _, h := range cfg.Cleanup.Hooks {
				it.Done = append(it.Done, "hook not run, config not trusted (review it, then run specsync trust -yes): "+h)
			}
			return it
		}
		run := opts.RunHook
		if run == nil {
			run = runShellHook
		}
		env := []string{"SPECSYNC_WORKTREE=" + row.Worktree, "SPECSYNC_BRANCH=" + row.Branch, "SPECSYNC_CHANGE=" + row.Slug}
		for _, h := range cfg.Cleanup.Hooks {
			if err := run(ctx, row.Store, h, env); err != nil {
				return fail("hook %q: %v", h, err)
			}
			it.Done = append(it.Done, "ran hook: "+h)
		}
	}
	return it
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
