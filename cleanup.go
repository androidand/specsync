package specsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"syscall"
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
	// FollowUps are things left for the user, such as archiving the change.
	FollowUps []string `json:"followUps,omitempty"`
	// Recovery says how to get the deleted branch back.
	Recovery string   `json:"recovery,omitempty"`
	Done     []string `json:"done,omitempty"`
	Error    string   `json:"error,omitempty"`

	row ChangeTopology
	// openBases are the branches that open requests in this repo target.
	openBases map[string]int
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
	// AssumeNoSessions lets a removal proceed although live sessions could not
	// be verified. SessionsUnverified is set by RunCleanup when they could not.
	AssumeNoSessions   bool
	SessionsUnverified bool
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
	openBases := map[string]map[string]int{}
	for _, r := range rows {
		if r.PR != nil && r.PR.State == "open" && r.PR.Base != "" {
			if openBases[r.Repo] == nil {
				openBases[r.Repo] = map[string]int{}
			}
			openBases[r.Repo][r.PR.Base] = r.PR.Number
		}
	}
	var items []CleanupItem
	for _, row := range onePerWorktree(rows) {
		if row.Worktree != "" {
			row.Sessions = shared[resolved(absPath(row.Worktree))]
		}
		if it, ok := classifyCleanup(ctx, row, opts, now()); ok {
			it.openBases = openBases[row.Repo]
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

// onePerWorktree keeps one row per worktree, preferring the one that names a
// change, so a worktree is decided and cleaned once.
func onePerWorktree(rows []ChangeTopology) []ChangeTopology {
	var out []ChangeTopology
	at := map[string]int{}
	for _, r := range rows {
		if r.Worktree == "" {
			out = append(out, r)
			continue
		}
		key := resolved(absPath(r.Worktree))
		i, seen := at[key]
		switch {
		case !seen:
			at[key] = len(out)
			out = append(out, r)
		case out[i].Slug == "" && r.Slug != "":
			out[i] = r
		}
	}
	return out
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

	remotes := remoteNames(ctx, row.Store)
	mergedHead := pr != nil && pr.State == "merged" && pr.HeadSHA != "" && ls.Head == pr.HeadSHA && baseIsRequestBase(ls.Base, pr.Base, remotes)
	inBase := false
	if ls.Base != "" {
		if _, err := gitIn(ctx, row.Worktree, "merge-base", "--is-ancestor", ls.Head, ls.Base); err == nil {
			inBase = true
		}
	}
	hasCommit, resetFrom := branchReflog(ctx, row.Worktree, row.Branch)
	ownCommits := inBase && hasCommit && resetFrom == ""
	unpushed := unpushedCommits(ctx, row.Worktree, ls, mergedHead)

	closedUnmerged := pr != nil && pr.State == "closed"
	provable := (mergedHead || ownCommits) && !closedUnmerged

	if inBase && !mergedHead && hasCommit && resetFrom != "" && !closedUnmerged {
		it.Status = CleanupSkipped
		it.Reason = "branch was reset: " + shortSHA(resetFrom) + " before; commits may have been discarded, and only a merged request proves the work is in " + ls.Base
		return it, true
	}
	if !provable {
		return notFinished(ctx, it, row, opts, now, unpushed, inBase, closedUnmerged)
	}

	it.Reason = "its work is in " + ls.Base
	if mergedHead {
		it.Reason = fmt.Sprintf("request #%d merged with this head", pr.Number)
	}
	if _, issues := LoadWays(row.Store); len(issues) > 0 {
		it.Status = CleanupSkipped
		it.Reason = "fix " + SpecSyncConfigPath + ": " + issues[0].String()
		return it, true
	}
	if reason := blockingReason(ctx, row, opts); reason != "" {
		it.Status, it.Reason = CleanupSkipped, reason
		return it, true
	}
	if reason := hooksCannotRun(row); reason != "" {
		it.Status, it.Reason = CleanupSkipped, reason
		return it, true
	}
	it.Status = CleanupCandidate
	it.Steps = cleanupSteps(row)
	it.FollowUps = followUps(row)
	return it, true
}

// hooksCannotRun explains why an item with configured hooks must wait: the
// hooks only run from a trusted config, and an item is not removed with its
// hooks left undone.
func hooksCannotRun(row ChangeTopology) string {
	cfg, _ := LoadWays(row.Store)
	if len(cfg.Cleanup.Hooks) == 0 || IsTrusted(row.Store) {
		return ""
	}
	return fmt.Sprintf("its cleanup hooks cannot run because %s is not trusted (review it, then specsync trust -yes): %s",
		SpecSyncConfigPath, strings.Join(cfg.Cleanup.Hooks, "; "))
}

// followUps are the steps cleanup leaves to the user. Archiving mutates the
// main checkout and the tracker, so it is only suggested.
func followUps(row ChangeTopology) []string {
	cfg, _ := LoadWays(row.Store)
	if cfg.Cleanup.Archive && row.Slug != "" && row.Issue != nil && row.Issue.State == "closed" {
		return []string{"archive pending: specsync archive -change " + row.Slug}
	}
	return nil
}

// baseIsRequestBase reports whether the branch the work is measured against is
// the branch the request merged into. A request merged into another feature
// branch (a stacked change) proves nothing about the base.
func baseIsRequestBase(base, requestBase string, remotes []string) bool {
	if base == "" || requestBase == "" {
		return false
	}
	return stripRemote(base, remotes) == stripRemote(requestBase, remotes)
}

func remoteNames(ctx context.Context, dir string) []string {
	out, err := gitIn(ctx, dir, "remote")
	if err != nil {
		return []string{"origin"}
	}
	return strings.Fields(out)
}

// stripRemote removes a leading refs/remotes/ and a known remote name.
func stripRemote(ref string, remotes []string) string {
	ref = strings.TrimPrefix(ref, "refs/remotes/")
	for _, r := range remotes {
		if strings.HasPrefix(ref, r+"/") {
			return strings.TrimPrefix(ref, r+"/")
		}
	}
	return ref
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
	lines := strings.Split(out, "\n")
	for i := range lines {
		lines[i] = SanitizeText(lines[i], 80)
	}
	return lines
}

func blockingReason(ctx context.Context, row ChangeTopology, opts CleanupOptions) string {
	ls := row.Local
	switch {
	case ls.StatusUnavailable:
		return "git status could not be read, so the tree may hold uncommitted work"
	case protectedBranch(ctx, row) != "":
		return protectedBranch(ctx, row)
	case sharedBranch(ctx, row):
		return "shared branch: it has commits from several authors (or its authors cannot be told), so it is never removed automatically"
	case ls.Modified+ls.Untracked > 0:
		return fmt.Sprintf("uncommitted changes: %d modified, %d untracked", ls.Modified, ls.Untracked)
	case opts.Current != "" && insideOrSame(opts.Current, row.Worktree):
		return "it is the current worktree"
	case len(row.Sessions) > 0:
		return "a live session is working in it: " + sessionName(row.Sessions[0])
	}
	if !opts.IncludeIgnored {
		names, err := ignoredFiles(ctx, row.Worktree)
		if err != nil {
			return "ignored files could not be checked: " + SanitizeText(err.Error(), 100)
		}
		if len(names) > 0 {
			return "has ignored files: " + strings.Join(names, ", ") + " (they would be lost; -include-ignored overrides)"
		}
	}
	return ""
}

// sharedBranch reports whether the branch's own commits have more than one
// author. A branch whose head is already in the base has no base..head range,
// so its range starts where its reflog says it was created; when that cannot
// be found the branch is treated as shared.
func sharedBranch(ctx context.Context, row ChangeTopology) bool {
	ls := row.Local
	if ls.SharedBranch {
		return true
	}
	if _, err := gitIn(ctx, row.Worktree, "merge-base", "--is-ancestor", ls.Head, ls.Base); err != nil {
		return false
	}
	out, err := gitIn(ctx, row.Worktree, "reflog", "show", "--format=%H %gs", "refs/heads/"+row.Branch)
	if err != nil {
		return true
	}
	lines := strings.Split(out, "\n")
	start, subject, _ := strings.Cut(lines[len(lines)-1], " ")
	if !strings.HasPrefix(subject, "branch: Created") || start == "" {
		return true
	}
	authors, err := gitIn(ctx, row.Worktree, "log", "--format=%ae", start+".."+ls.Head)
	if err != nil {
		return true
	}
	seen := map[string]bool{}
	for _, a := range strings.Fields(authors) {
		seen[a] = true
	}
	return len(seen) > 1
}

var protectedNames = []string{"main", "master", "develop", "trunk", "staging", "production", "prod", "gh-pages"}

var protectedGlobs = []string{"release/*", "hotfix/*"}

// protectedBranch explains why a branch must never be removed: it is a
// well-known trunk name, the base or default branch, matches a protect glob,
// or is the one checked out in the main checkout.
func protectedBranch(ctx context.Context, row ChangeTopology) string {
	b := row.Branch
	remotes := remoteNames(ctx, row.Store)
	protected := append([]string(nil), protectedNames...)
	globs := append([]string(nil), protectedGlobs...)
	cfg, _ := LoadWays(row.Store)
	globs = append(globs, cfg.Cleanup.Protect...)
	if base := row.Local.Base; base != "" {
		protected = append(protected, stripRemote(base, remotes))
	}
	for _, r := range remotes {
		if out, err := gitIn(ctx, row.Store, "symbolic-ref", "--short", "refs/remotes/"+r+"/HEAD"); err == nil {
			protected = append(protected, stripRemote(out, remotes))
		}
	}
	for _, p := range protected {
		if p == b {
			return "protected branch " + b + ": it is a base or default branch"
		}
	}
	for _, g := range globs {
		if matchBranchGlob(g, b) {
			return "protected branch " + b + ": it matches " + g
		}
	}
	if out, err := gitIn(ctx, row.Store, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && out == b {
		return "protected branch " + b + ": it is checked out in the main checkout"
	}
	return ""
}

// matchBranchGlob matches a branch against a glob. A pattern ending in /* or
// /** covers everything below that prefix, however deep; a malformed pattern
// matches everything, so a typo protects instead of exposing.
func matchBranchGlob(glob, branch string) bool {
	for _, suffix := range []string{"/**", "/*"} {
		if prefix, ok := strings.CutSuffix(glob, suffix); ok {
			if _, err := path.Match(prefix, "x"); err != nil {
				return true
			}
			ok, _ := path.Match(prefix+"/*", firstSegments(branch, strings.Count(prefix, "/")+2))
			return ok
		}
	}
	ok, err := path.Match(glob, branch)
	return ok || err != nil
}

func firstSegments(s string, n int) string {
	parts := strings.SplitN(s, "/", n+1)
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, "/")
}

// defaultIgnored are build output folders whose loss costs only a rebuild.
var defaultIgnored = []string{"node_modules", "dist", "build", "target", ".next", "vendor", "__pycache__"}

// ignoredFiles lists ignored paths in the worktree other than common build
// output, a few at most.
func ignoredFiles(ctx context.Context, dir string) ([]string, error) {
	out, err := gitIn(ctx, dir, "--no-optional-locks", "status", "--porcelain=v1", "--ignored=matching", "--untracked-files=normal")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "!! ") {
			continue
		}
		p := strings.TrimSuffix(strings.TrimPrefix(line, "!! "), "/")
		if !strings.Contains(p, "/") && slices.Contains(defaultIgnored, p) {
			continue
		}
		{
			names = append(names, SanitizeText(p, 60))
		}
	}
	if len(names) > 3 {
		names = append(names[:3], fmt.Sprintf("and %d more", len(names)-3))
	}
	return names, nil
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

// branchReflog reads the branch's reflog: whether a commit was ever made on
// it, and, when it was reset, the tip it had before the most recent reset.
// A reset can discard commits and leave their reflog entries behind, so a
// reset branch is never proven finished by its history.
func branchReflog(ctx context.Context, dir, branch string) (hasCommit bool, resetFrom string) {
	out, err := gitIn(ctx, dir, "reflog", "show", "--format=%H %gs", "refs/heads/"+branch)
	if err != nil {
		return false, ""
	}
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		_, subject, _ := strings.Cut(line, " ")
		switch {
		case strings.HasPrefix(subject, "commit"):
			hasCommit = true
		case strings.HasPrefix(subject, "reset") && resetFrom == "":
			resetFrom = "unknown"
			if i+1 < len(lines) {
				resetFrom, _, _ = strings.Cut(lines[i+1], " ")
			}
		}
	}
	return hasCommit, resetFrom
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
// and, with opts.Apply, carries out the candidates. When live sessions could
// not be verified, applying is refused unless opts.AssumeNoSessions is set.
func RunCleanup(ctx context.Context, opts CleanupOptions) (*CleanupReport, error) {
	opts.Overview.Topology.NoConflictCheck = true
	if opts.Overview.Topology.Cache != nil {
		opts.Overview.Topology.Cache = readOnlyCache{opts.Overview.Topology.Cache}
	}
	rows, err := CollectWorkRows(ctx, opts.Overview)
	if err != nil {
		return nil, err
	}
	note := sessionsCheckNote(opts.Overview, rows.Status)
	opts.SessionsUnverified = note != ""
	items := PlanCleanup(ctx, rows.Rows, opts)
	if opts.Apply {
		if opts.SessionsUnverified && !opts.AssumeNoSessions {
			items = markSessionsUnverified(items)
		}
		items = ApplyCleanup(ctx, items, opts)
	}
	return &CleanupReport{Apply: opts.Apply, Items: items, Status: rows.Status, SessionsNote: note}, nil
}

// readOnlyCache serves cached request listings but never writes one, so a dry
// run changes nothing outside memory.
type readOnlyCache struct{ RequestCache }

func (readOnlyCache) Put(string, ChangeRequestSet) {}

// markSessionsUnverified turns every candidate into a skipped item, because
// nothing says no agent is working in it.
func markSessionsUnverified(items []CleanupItem) []CleanupItem {
	out := make([]CleanupItem, len(items))
	for i, it := range items {
		if it.Status == CleanupCandidate {
			it.Status = CleanupSkipped
			it.Reason = "live sessions not verified; pass -assume-no-sessions to remove it anyway (" + it.Reason + ")"
		}
		out[i] = it
	}
	return out
}

// ApplyCleanup carries out the candidates in items and returns every item with
// what was done. Only candidates are touched, and each one is checked again
// right before it is removed: work added since planning stops the removal.
func ApplyCleanup(ctx context.Context, items []CleanupItem, opts CleanupOptions) []CleanupItem {
	out := make([]CleanupItem, len(items))
	handled := map[string]bool{}
	for i, it := range items {
		if it.Status == CleanupCandidate {
			key := resolved(absPath(it.Worktree))
			if handled[key] {
				it.Status, it.Reason = CleanupSkipped, "already handled as another row of the same worktree"
			} else {
				handled[key] = true
				it = applyOne(ctx, it, opts)
			}
		}
		out[i] = it
	}
	return out
}

// applyOne does one candidate in a fixed order, stopping at the first failure:
//  1. check it again from fresh facts (protected branch, ignored files, head,
//     status, sessions, trust for its hooks, proof that its work is in the base);
//  2. run the configured hooks in the worktree;
//  3. remove the worktree and delete the local branch, only if its tip is
//     still the verified head;
//  4. delete the remote branch when configured and its tip is still ours.
//
// Archiving is never done here: it is reported as a follow-up. A failure in 1
// or 2 leaves the worktree and branch untouched.
func applyOne(ctx context.Context, it CleanupItem, opts CleanupOptions) CleanupItem {
	fail := func(format string, a ...any) CleanupItem {
		it.Error = fmt.Sprintf(format, a...)
		return it
	}
	if opts.SessionsUnverified && !opts.AssumeNoSessions {
		return fail("live sessions not verified; nothing removed (pass -assume-no-sessions to proceed)")
	}
	row, reason := recheck(ctx, it.row, opts)
	if reason != "" {
		return fail("%s; nothing removed", reason)
	}
	cfg, _ := LoadWays(row.Store)

	if hooks := cfg.Cleanup.Hooks; len(hooks) > 0 {
		run := opts.RunHook
		if run == nil {
			run = runShellHook
		}
		env := []string{"SPECSYNC_WORKTREE=" + row.Worktree, "SPECSYNC_BRANCH=" + row.Branch, "SPECSYNC_CHANGE=" + row.Slug}
		for _, h := range hooks {
			if err := run(ctx, row.Store, h, env); err != nil {
				return fail("hook %q: %v; the worktree and branch were left in place", h, err)
			}
			it.Done = append(it.Done, "ran hook: "+h)
		}
	}

	if _, err := gitIn(ctx, row.Store, "worktree", "remove", row.Worktree); err != nil {
		return fail("%v", err)
	}
	it.Done = append(it.Done, "removed worktree "+row.Worktree)

	it.Recovery = fmt.Sprintf("removed branch %s at %s; restore with: git branch %s %s", row.Branch, row.Local.Head, row.Branch, row.Local.Head)
	if err := deleteBranchAt(ctx, row.Store, row.Branch, row.Local.Head); err != nil {
		it.Done = append(it.Done, it.Recovery)
		return fail("%v", err)
	}
	it.Done = append(it.Done, "deleted local branch "+row.Branch, it.Recovery)

	if cfg.Cleanup.RemoteBranch == "delete" {
		it.Done = append(it.Done, deleteRemoteBranch(ctx, row, it.openBases))
	}
	it.FollowUps = followUps(row)
	return it
}

// deleteBranchAt deletes the branch only while it still points at head, so a
// commit made after the last check is never thrown away.
func deleteBranchAt(ctx context.Context, store, branch, head string) error {
	if _, err := gitIn(ctx, store, "update-ref", "-d", "refs/heads/"+branch, head); err != nil {
		return fmt.Errorf("the branch %s moved or could not be deleted, so it was left: %w", branch, err)
	}
	if _, err := gitIn(ctx, store, "config", "--get-regexp", `^branch\.`+regexp.QuoteMeta(branch)+`\.`); err == nil {
		_, _ = gitIn(ctx, store, "config", "--remove-section", "branch."+branch)
	}
	return nil
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
		entries, err := LoadSessions(ctx, opts.Overview.SessionsCmd, opts.Overview.SessionsTimeout)
		switch {
		case err != nil && !opts.AssumeNoSessions:
			return row, "live sessions could not be verified: " + SanitizeText(err.Error(), 100)
		case err == nil:
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
func deleteRemoteBranch(ctx context.Context, row ChangeTopology, openBases map[string]int) string {
	ls := row.Local
	keep := func(why string) string { return "kept remote branch: " + why }
	if ls.UpstreamGone {
		return "remote branch already gone"
	}
	remote := remoteOf(row)
	if n, ok := openBases[row.Branch]; ok {
		return keep(fmt.Sprintf("open request #%d targets it", n))
	}
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
	if tip != ls.Head {
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

// hookTimeout bounds one hook; the whole process group is killed at the limit.
var hookTimeout = 60 * time.Second

const maxHookOutput = 64 << 10

type cappedBuffer struct {
	buf bytes.Buffer
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := maxHookOutput - c.buf.Len(); room > 0 {
		if len(p) > room {
			c.buf.Write(p[:room])
		} else {
			c.buf.Write(p)
		}
	}
	return len(p), nil
}

// runShellHook runs one configured hook in dir. Hooks are repository-defined
// commands, so they run in the main checkout, not in the branch's worktree.
func runShellHook(ctx context.Context, dir, command string, env []string) error {
	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 500 * time.Millisecond
	var out cappedBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("timed out after %s and was stopped", hookTimeout)
	}
	if err != nil {
		return fmt.Errorf("%v: %s", err, SanitizeText(strings.TrimSpace(out.buf.String()), 200))
	}
	return nil
}
