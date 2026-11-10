package specsync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Topology is the read-only join across coordinated stores: for every change,
// its stage, owning store and repo, tracker issue, git branch, worktree path,
// epic parent, and linked sibling changes. Every field beyond
// Slug/Title/Stage is independently optional — a source that's unavailable
// narrows a row instead of dropping it or failing the whole command. See
// openspec/changes/agent-topology-command.
type Topology struct {
	Changes []ChangeTopology `json:"changes"`
	Status  []string         `json:"status,omitempty"`
	// RequestState says whether request lookups happened: "ok", "omitted"
	// (a provider failed) or "off" (-state=off). Empty when none was needed.
	// An absent pr block only means "no request" when this is "ok".
	RequestState string `json:"requestState,omitempty"`
}

// ChangeTopology is one row of the join.
type ChangeTopology struct {
	Slug     string         `json:"slug"`
	Title    string         `json:"title"`
	Stage    string         `json:"stage"`
	Store    string         `json:"store,omitempty"`
	Repo     string         `json:"repo,omitempty"`
	Issue    *TopologyIssue `json:"issue,omitempty"`
	Branch   string         `json:"branch,omitempty"`
	Worktree string         `json:"worktree,omitempty"`
	Epic     string         `json:"epic,omitempty"`
	Linked   []string       `json:"linked,omitempty"`
	// BlockedBy and Blocks are issue keys (owner/repo#N) from the change's
	// links.md; the relations graph also reads Blocks as the inverse.
	BlockedBy []string `json:"blockedBy,omitempty"`
	Blocks    []string `json:"blocks,omitempty"`
	// LinkedRefs are related issues (owner/repo#N) that are not changes in this
	// store and not the epic; the graph resolves them across clones.
	LinkedRefs []string `json:"linkedRefs,omitempty"`
	// Local and Attention come from git alone, for the row's worktree.
	Local     *LocalState `json:"local,omitempty"`
	Attention []string    `json:"attention,omitempty"`
	// Sync is advice for bringing the branch up to date. It is never executed.
	Sync *SyncAdvice `json:"sync,omitempty"`
	// NoChange marks the row for a branch that belongs to no change (Slug is empty).
	NoChange bool `json:"noChange,omitempty"`
	// Tasks counts the change's checklist items.
	Tasks *TaskCount `json:"tasks,omitempty"`
	// dir is the change folder, used to read its slice declarations.
	dir string
	// PR is the change request on the row's branch, from the work provider.
	PR *ChangeRequest `json:"pr,omitempty"`
}

// TaskCount is how many checklist items are done of the total.
type TaskCount struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// TopologyIssue is the tracker reference for one change. State is set only
// when a live tracker query (gh) succeeded; its absence is not an error.
type TopologyIssue struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	URL      string `json:"url"`
	State    string `json:"state,omitempty"`
}

// TopologyOptions configures BuildTopology. OpenSpecDir/Slug/All are the
// public inputs; the rest are dependency seams overridable in tests (nil
// means "use the real implementation" — openspec/git/gh via exec.Command).
type TopologyOptions struct {
	OpenSpecDir string // the root store's openspec dir
	Slug        string // scope to one change; "" = every change
	All         bool   // include archived changes

	readCoordination func(ctx context.Context) (*Coordination, error)
	loadChanges      func(openspecDir string) ([]Change, error)
	loadRefs         func(changeDir string) (map[string]Ref, error)
	gitWorktreeList  func(ctx context.Context, repoDir string) (string, error)
	gitRemoteOrigin  func(ctx context.Context, repoDir string) (string, error)
	run              func(ctx context.Context, args ...string) (string, error) // gh
	localState       func(ctx context.Context, worktree string, lo LocalStateOptions) (*LocalState, error)

	// Dir, when set and Slug is empty, scopes the result to the worktree that
	// contains it: the current work. A branch that belongs to no change gets a
	// row with NoChange set. Outside a git worktree everything is returned.
	Dir        string
	gitCurrent func(ctx context.Context, dir string) (toplevel, branch string, err error)

	// LocalOnly (-state=off) skips every gh call and the provider: local git
	// facts only, instant and offline.
	LocalOnly bool

	// Cache, when set, serves a repo's request listing for a short time.
	// RefreshRequests skips reading it (the fresh listing is still stored).
	Cache           RequestCache
	RefreshRequests bool

	// changeRequests lists one repo's requests. nil set means the provider
	// cannot list them, which is silent. The default asks the GitHub provider.
	changeRequests func(ctx context.Context, repo string) (*ChangeRequestSet, error)

	// Sync is the `sync:` config for suggestions; zero means the defaults.
	Sync SyncConfig

	// Budget is the -budget flag value and Exclude the -exclude globs for the
	// advisory size flags; the slice budget line, SPECSYNC_BUDGET and the
	// defaults fill in what they leave unset.
	Budget  string
	Exclude []string

	// NoCoordination skips the coordinated sibling stores: the caller walks
	// the workspace itself and asks one store at a time.
	NoCoordination bool
	// IncludeUnlinked adds rows for worktrees and open requests that belong to
	// no change, and tolerates a store that has no openspec/ directory.
	IncludeUnlinked bool
}

// storeEntry is one coordinated store to scan: its repo root (for git
// commands) and its openspec directory (for LoadChanges).
type storeEntry struct {
	path        string
	openspecDir string
}

// BuildTopology joins every change across the root store and any coordinated
// sibling stores with its tracker issue, git branch, and worktree. It never
// returns an error for a degraded source — those narrow individual rows and
// are recorded in Topology.Status instead. The CLI decides what a fully
// empty result means (see cmd/specsync topology.go).
func BuildTopology(ctx context.Context, opts TopologyOptions) (*Topology, error) {
	if _, err := ResolveBudget(opts.Budget, "", os.Getenv(BudgetEnv)); err != nil {
		return nil, fmt.Errorf("size budget: %w", err)
	}
	if opts.readCoordination == nil {
		opts.readCoordination = ReadCoordination
	}
	if opts.loadChanges == nil {
		opts.loadChanges = LoadChanges
	}
	if opts.loadRefs == nil {
		opts.loadRefs = LoadRefs
	}
	if opts.gitWorktreeList == nil {
		opts.gitWorktreeList = gitWorktreeListReal
	}
	if opts.gitRemoteOrigin == nil {
		opts.gitRemoteOrigin = gitRemoteOriginReal
	}
	if opts.run == nil {
		opts.run = runGH
	}
	if opts.changeRequests == nil {
		run := opts.run
		opts.changeRequests = func(ctx context.Context, repo string) (*ChangeRequestSet, error) {
			var src ChangeRequestSource = NewGitHubProviderFuncWithRepo(repo, run)
			set, err := src.ChangeRequests(ctx)
			if err != nil {
				return nil, err
			}
			return &set, nil
		}
	}
	if opts.localState == nil {
		opts.localState = CollectLocalState
	}
	if opts.gitCurrent == nil {
		opts.gitCurrent = gitCurrentReal
	}

	var status []string
	addStatus := func(format string, args ...any) {
		status = append(status, fmt.Sprintf(format, args...))
	}

	repoRoot := filepath.Dir(opts.OpenSpecDir)
	stores := []storeEntry{{path: repoRoot, openspecDir: opts.OpenSpecDir}}

	var coord *Coordination
	var cerr error
	if !opts.NoCoordination {
		coord, cerr = opts.readCoordination(ctx)
	}
	if cerr != nil {
		addStatus("openspec context unavailable: %v", cerr)
	} else if coord != nil {
		for _, m := range coord.Members {
			stores = append(stores, storeEntry{path: m.Path, openspecDir: filepath.Join(m.Path, "openspec")})
		}
	}

	type loadedChange struct {
		change Change
		store  storeEntry
		refs   map[string]Ref
	}

	var all []loadedChange
	urlToSlug := map[string]string{}
	for _, st := range stores {
		if opts.IncludeUnlinked {
			if info, err := os.Stat(st.openspecDir); err != nil || !info.IsDir() {
				continue
			}
		}
		changes, err := opts.loadChanges(st.openspecDir)
		if err != nil {
			addStatus("could not load changes from %s: %v", st.openspecDir, err)
			continue
		}
		for _, c := range changes {
			if !opts.All && c.Archived {
				continue
			}
			refs, rerr := opts.loadRefs(c.Dir)
			if rerr != nil {
				addStatus("could not read ref cache for %s: %v", c.Slug, rerr)
				refs = map[string]Ref{}
			}
			if len(refs) > 0 {
				_, ref := firstRef(refs)
				if ref.URL != "" {
					urlToSlug[ref.URL] = c.Slug
				}
			}
			all = append(all, loadedChange{change: c, store: st, refs: refs})
		}
	}

	worktrees := map[string][]worktreeEntry{}
	worktreeFailed := map[string]bool{}
	repos := map[string]string{}
	repoFailed := map[string]bool{}
	epicLabel := map[string]bool{}
	ghDown := opts.LocalOnly
	requestSets := map[string]*ChangeRequestSet{}
	requestState := ""
	if opts.LocalOnly {
		requestState = "off"
	}
	requestsFailed := map[string]bool{}

	worktreesOf := func(storePath string) []worktreeEntry {
		if entries, have := worktrees[storePath]; have {
			return entries
		}
		if worktreeFailed[storePath] {
			return nil
		}
		es, err := listWorktrees(ctx, opts.gitWorktreeList, storePath)
		if err != nil {
			worktreeFailed[storePath] = true
			addStatus("git worktree list failed in %s: %v", storePath, err)
			return nil
		}
		worktrees[storePath] = es
		return es
	}

	repoOf := func(storePath string) string {
		if repo, ok := repos[storePath]; ok {
			return repo
		}
		if repoFailed[storePath] {
			return ""
		}
		repo, err := opts.gitRemoteOrigin(ctx, storePath)
		if err != nil || repo == "" {
			repoFailed[storePath] = true
			addStatus("no git remote in store %s", storePath)
			return ""
		}
		repos[storePath] = repo
		return repo
	}

	// resolveWorktree finds a change's worktree: by slug, then by the issue
	// number in the branch name (feat/<issue>-<name>).
	resolveWorktree := func(storePath, slug, issueID string) (branch, path string, ok bool) {
		entries := worktreesOf(storePath)
		if entries == nil {
			return "", "", false
		}
		if b, p, m := matchWorktree(entries, slug); m {
			return b, p, true
		}
		return matchWorktreeByIssue(entries, issueID)
	}

	// requestsFor returns a repo's request listing, once per run. nil means it
	// is unavailable, which narrows rows and never fails the run.
	requestsFor := func(repo string) *ChangeRequestSet {
		if opts.LocalOnly || repo == "" || requestsFailed[repo] {
			return nil
		}
		if set, seen := requestSets[repo]; seen {
			return set
		}
		var got *ChangeRequestSet
		var err error
		if opts.Cache != nil && !opts.RefreshRequests {
			if cached, ok := opts.Cache.Get(repo); ok {
				got = &cached
			}
		}
		if got == nil {
			got, err = opts.changeRequests(ctx, repo)
			if err == nil && got != nil && opts.Cache != nil {
				opts.Cache.Put(repo, *got)
			}
		}
		if err != nil {
			requestsFailed[repo] = true
			requestState = "omitted"
			addStatus("request state omitted for %s: %s", repo, errReason(err))
			return nil
		}
		if requestState == "" {
			requestState = "ok"
		}
		if got != nil && got.Repo == "" {
			got.Repo = repo
		}
		requestSets[repo] = got
		if got != nil && got.Truncated {
			addStatus("request listing for %s may be truncated; older requests can be missing", repo)
		}
		return got
	}

	// enrich adds everything that needs the row's branch and worktree: the
	// request, local git state, attention flags and the sync suggestion.
	warnedBudget := false
	enrich := func(row *ChangeTopology) {
		var cr *ChangeRequest
		if row.Branch != "" {
			if set := requestsFor(row.Repo); set != nil {
				cr = set.ForBranch(row.Branch)
				row.PR = cr
			}
		}

		// An open request's base is the branch this work is measured against.
		base := ""
		if cr != nil && cr.State == "open" && cr.Base != "" {
			base = "origin/" + cr.Base
		}
		var ls *LocalState
		if row.Worktree != "" {
			budget, err := ResolveBudget(opts.Budget, SliceBudgetLine(row.dir), os.Getenv(BudgetEnv))
			if err != nil {
				if !warnedBudget {
					warnedBudget = true
					addStatus("budget line of change %s ignored: %v", row.Slug, err)
				}
				budget, _ = ResolveBudget(opts.Budget, "", os.Getenv(BudgetEnv))
			}
			if got, err := opts.localState(ctx, row.Worktree, LocalStateOptions{Base: base, Budget: budget, Exclude: opts.Exclude}); err != nil {
				addStatus("local state unavailable for %s: %v", rowLabel(*row), err)
			} else {
				ls = got
				row.Local = ls
				ls.NoSlicePlan = row.dir != "" && !HasSlices(row.dir)
			}
		}

		row.Attention = RowAttention(ls, cr)
		if ls != nil {
			row.Sync = SuggestSync(*ls, SyncInput{UnderReview: cr != nil && cr.State == "open" && !cr.Draft}, opts.Sync)
		}
	}

	// Scope to the current worktree: decided up front so other changes are
	// skipped before any per-change lookup.
	scoped, curTop, curBranch := false, "", ""
	if opts.Dir != "" && opts.Slug == "" {
		top, branch, err := opts.gitCurrent(ctx, opts.Dir)
		if err != nil {
			addStatus("not inside a git worktree (%s): showing every change", errReason(err))
		} else {
			scoped, curTop, curBranch = true, top, branch
		}
	}

	var rows []ChangeTopology
	for _, l := range all {
		c := l.change
		if opts.Slug != "" && c.Slug != opts.Slug {
			continue
		}
		if scoped {
			if _, path, ok := resolveWorktree(l.store.path, c.Slug, refIssueID(l.refs)); !ok || resolved(path) != resolved(curTop) {
				continue
			}
		}

		row := ChangeTopology{
			Slug:  c.Slug,
			Title: c.Title,
			Stage: string(c.Stage),
			Store: l.store.path,
			dir:   c.Dir,
		}
		if total, done := CountCheckboxes(c.TasksMarkdown); total > 0 {
			row.Tasks = &TaskCount{Done: done, Total: total}
		}

		row.Repo = repoOf(l.store.path)

		if len(l.refs) > 0 {
			key, ref := firstRef(l.refs)
			row.Issue = &TopologyIssue{Provider: key, ID: ref.ID, URL: ref.URL}
			issueRepo := repoFromKey(key)
			if !ghDown && ref.ID != "" && issueRepo != "" {
				state, err := fetchIssueState(ctx, opts.run, issueRepo, ref.ID)
				if err != nil {
					ghDown = true
					addStatus("gh unavailable: issue state omitted")
				} else {
					row.Issue.State = state
				}
			}
		}

		for _, r := range c.BlockedBy {
			row.BlockedBy = append(row.BlockedBy, refKey(r))
		}
		for _, r := range c.Blocks {
			row.Blocks = append(row.Blocks, refKey(r))
		}

		if branch, path, matched := resolveWorktree(l.store.path, c.Slug, refIssueID(l.refs)); matched {
			row.Branch = branch
			row.Worktree = path
		} else if row.Issue != nil {
			if set := requestsFor(row.Repo); set != nil {
				if cr := set.ForIssue(repoFromKey(row.Issue.Provider), row.Issue.ID); cr != nil {
					row.Branch = cr.Branch
					for _, e := range worktreesOf(l.store.path) {
						if e.branch == cr.Branch {
							row.Worktree = e.path
							break
						}
					}
				}
			}
		}
		if row.Branch != "" {
			enrich(&row)
		}

		for _, link := range c.Links {
			if slug, known := urlToSlug[link.URL]; known {
				row.Linked = append(row.Linked, slug)
				continue
			}
			ref := refFromURL(link.URL)
			if row.Epic != "" || ghDown {
				if ref != nil && refKey(*ref) != "" && link.URL != row.Epic {
					row.LinkedRefs = append(row.LinkedRefs, refKey(*ref))
				}
				continue
			}
			// An unresolved related URL might be the change's coordination
			// epic (specsync epic never writes a local link back to the
			// child, only a remote "## Related" edit — see epic.go). Confirm
			// via the epic's "type:epic" label rather than guessing from
			// text, since a sibling link renders identically.
			if ref == nil || ref.ID == "" || ref.Provider == "" {
				continue
			}
			cacheKey := ref.Provider + "#" + ref.ID
			isEpic, cached := epicLabel[cacheKey]
			if !cached {
				var err error
				isEpic, err = issueHasLabel(ctx, opts.run, repoFromKey(ref.Provider), ref.ID, "type:epic")
				if err != nil {
					ghDown = true
					addStatus("gh unavailable: issue state omitted")
					continue
				}
				epicLabel[cacheKey] = isEpic
			}
			if isEpic {
				row.Epic = link.URL
			} else if k := refKey(*ref); k != "" {
				row.LinkedRefs = append(row.LinkedRefs, k)
			}
		}

		rows = append(rows, row)
	}

	if opts.IncludeUnlinked && !scoped {
		for _, st := range stores {
			rows = append(rows, unlinkedRows(st, rows, repoOf(st.path), worktreesOf(st.path), requestsFor, enrich)...)
		}
	}

	if scoped && len(rows) == 0 {
		row := ChangeTopology{NoChange: true, Branch: curBranch, Worktree: curTop}
		if repo, err := opts.gitRemoteOrigin(ctx, curTop); err == nil {
			row.Repo = repo
		}
		enrich(&row)
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if (a.Slug == "") != (b.Slug == "") {
			return b.Slug == ""
		}
		if a.Slug == "" {
			return a.Repo+"\x00"+a.Branch < b.Repo+"\x00"+b.Branch
		}
		return a.Slug < b.Slug
	})

	return &Topology{Changes: rows, Status: status, RequestState: requestState}, nil
}

// fetchIssueState returns an issue's open/closed state via gh.
func fetchIssueState(ctx context.Context, run func(context.Context, ...string) (string, error), repo, id string) (string, error) {
	out, err := run(ctx, "issue", "view", id, "-R", repo, "--json", "state", "-q", ".state")
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(out)), nil
}

// issueHasLabel reports whether an issue carries the given label, via gh.
func issueHasLabel(ctx context.Context, run func(context.Context, ...string) (string, error), repo, id, label string) (bool, error) {
	out, err := run(ctx, "issue", "view", id, "-R", repo, "--json", "labels", "-q", ".labels[].name")
	if err != nil {
		return false, err
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == label {
			return true, nil
		}
	}
	return false, nil
}

// worktreeEntry is one entry from `git worktree list --porcelain`.
type worktreeEntry struct {
	path   string
	branch string // short branch name (e.g. "feat/159-x"); "" when detached or bare
}

// gitWorktreeListReal shells out to `git -C repoDir worktree list --porcelain`.
func gitWorktreeListReal(ctx context.Context, repoDir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", repoDir, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// gitRemoteOriginReal shells out to `git -C repoDir remote get-url origin`
// and parses it into "owner/name".
func gitRemoteOriginReal(ctx context.Context, repoDir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", repoDir, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", err
	}
	return parseGitRemoteURL(strings.TrimSpace(string(out))), nil
}

// listWorktrees parses `git worktree list --porcelain` output into entries.
// A bare repo or a worktree with a detached HEAD has no "branch" line and is
// returned with branch == "" — it simply can't match any change.
func listWorktrees(ctx context.Context, run func(context.Context, string) (string, error), repoDir string) ([]worktreeEntry, error) {
	out, err := run(ctx, repoDir)
	if err != nil {
		return nil, err
	}

	var entries []worktreeEntry
	var cur *worktreeEntry
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if cur != nil {
				entries = append(entries, *cur)
			}
			cur = &worktreeEntry{path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch "):
			if cur != nil {
				cur.branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
			}
		}
	}
	if cur != nil {
		entries = append(entries, *cur)
	}
	return entries, nil
}

// worktreeBranchPattern matches this repo's branch convention: feat/<issue>-<slug>.
var worktreeBranchPattern = regexp.MustCompile(`^feat/\d+-(.+)$`)

// matchWorktree finds the worktree for a change: first by branch name
// (feat/<n>-<slug>), then by directory basename, per agent-topology-command.
func matchWorktree(entries []worktreeEntry, slug string) (branch, path string, ok bool) {
	for _, e := range entries {
		if m := worktreeBranchPattern.FindStringSubmatch(e.branch); m != nil && m[1] == slug {
			return e.branch, e.path, true
		}
	}
	for _, e := range entries {
		if filepath.Base(e.path) == slug {
			return e.branch, e.path, true
		}
	}
	return "", "", false
}

// errReason returns the last non-empty line of an error: for a failed gh call
// that is gh's own message, without the echoed command line.
func errReason(err error) string {
	lines := strings.Split(strings.TrimSpace(err.Error()), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// rowLabel names a row in messages: its slug, or its branch for a branch with no change.
func rowLabel(r ChangeTopology) string {
	if r.Slug != "" {
		return r.Slug
	}
	return r.Branch
}

// gitCurrentReal returns the top level of the worktree containing dir and its
// current branch ("" when HEAD is detached).
func gitCurrentReal(ctx context.Context, dir string) (string, string, error) {
	top, err := gitIn(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", err
	}
	branch, err := gitIn(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return top, "", nil // a repo with no commits yet has no branch name
	}
	if branch == "HEAD" {
		branch = ""
	}
	return top, branch, nil
}

// refIssueID returns the issue number from a change's first ref, or "".
func refIssueID(refs map[string]Ref) string {
	if len(refs) == 0 {
		return ""
	}
	_, ref := firstRef(refs)
	return ref.ID
}

// issueInBranch matches a leading issue number: feat/186-name or 186-name.
var issueInBranch = regexp.MustCompile(`^(?:[^/]+/)?(\d+)-`)

// matchWorktreeByIssue finds the worktree whose branch starts with the issue
// number, comparing whole numbers (18 does not match 186).
func matchWorktreeByIssue(entries []worktreeEntry, issueID string) (branch, path string, ok bool) {
	if issueID == "" {
		return "", "", false
	}
	for _, e := range entries {
		if m := issueInBranch.FindStringSubmatch(e.branch); m != nil && m[1] == issueID {
			return e.branch, e.path, true
		}
	}
	return "", "", false
}

// unlinkedRows returns rows for the store's worktrees and open requests that
// no change claims: work with a branch and no change. The main worktree on a
// branch with no open request is not work and is left out.
func unlinkedRows(st storeEntry, existing []ChangeTopology, repo string, entries []worktreeEntry,
	requestsFor func(string) *ChangeRequestSet, enrich func(*ChangeTopology)) []ChangeTopology {
	claimedPath, claimedBranch := map[string]bool{}, map[string]bool{}
	for _, r := range existing {
		if r.Store != st.path {
			continue
		}
		if r.Worktree != "" {
			claimedPath[resolved(r.Worktree)] = true
		}
		if r.Branch != "" {
			claimedBranch[r.Branch] = true
		}
	}
	set := requestsFor(repo)
	hasOpen := func(branch string) bool {
		if set == nil || branch == "" {
			return false
		}
		cr := set.ForBranch(branch)
		return cr != nil && cr.State == "open"
	}

	var out []ChangeTopology
	seen := map[string]bool{}
	for _, e := range entries {
		if e.branch == "" {
			continue
		}
		seen[e.branch] = true
		if claimedPath[resolved(e.path)] || (e.branch != "" && claimedBranch[e.branch]) {
			continue
		}
		if resolved(e.path) == resolved(st.path) && !hasOpen(e.branch) {
			continue
		}
		row := ChangeTopology{NoChange: true, Store: st.path, Repo: repo, Branch: e.branch, Worktree: e.path}
		enrich(&row)
		out = append(out, row)
	}
	if set != nil {
		for _, cr := range set.Requests {
			if cr.State != "open" || cr.Fork || seen[cr.Branch] || claimedBranch[cr.Branch] {
				continue
			}
			seen[cr.Branch] = true
			row := ChangeTopology{NoChange: true, Store: st.path, Repo: repo, Branch: cr.Branch}
			enrich(&row)
			out = append(out, row)
		}
	}
	return out
}

// refKey names an issue reference as owner/repo#N, or "" when the provider is
// not a repository.
func refKey(r Ref) string {
	repo := repoFromKey(r.Provider)
	if repo == "" || r.ID == "" {
		return ""
	}
	return repo + "#" + r.ID
}
