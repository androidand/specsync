package specsync

import (
	"context"
	"fmt"
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

	var status []string
	addStatus := func(format string, args ...any) {
		status = append(status, fmt.Sprintf(format, args...))
	}

	repoRoot := filepath.Dir(opts.OpenSpecDir)
	stores := []storeEntry{{path: repoRoot, openspecDir: opts.OpenSpecDir}}

	coord, cerr := opts.readCoordination(ctx)
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
	ghDown := false

	var rows []ChangeTopology
	for _, l := range all {
		c := l.change
		if opts.Slug != "" && c.Slug != opts.Slug {
			continue
		}

		row := ChangeTopology{
			Slug:  c.Slug,
			Title: c.Title,
			Stage: string(c.Stage),
			Store: l.store.path,
		}

		if repo, ok := repos[l.store.path]; ok {
			row.Repo = repo
		} else if !repoFailed[l.store.path] {
			repo, err := opts.gitRemoteOrigin(ctx, l.store.path)
			if err != nil || repo == "" {
				repoFailed[l.store.path] = true
				addStatus("no git remote in store %s", l.store.path)
			} else {
				repos[l.store.path] = repo
				row.Repo = repo
			}
		}

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

		entries, ok := worktrees[l.store.path]
		if !ok && !worktreeFailed[l.store.path] {
			es, err := listWorktrees(ctx, opts.gitWorktreeList, l.store.path)
			if err != nil {
				worktreeFailed[l.store.path] = true
				addStatus("git worktree list failed in %s: %v", l.store.path, err)
			} else {
				worktrees[l.store.path] = es
				entries = es
				ok = true
			}
		}
		if ok {
			if branch, path, matched := matchWorktree(entries, c.Slug); matched {
				row.Branch = branch
				row.Worktree = path
			}
		}

		for _, link := range c.Links {
			if slug, known := urlToSlug[link.URL]; known {
				row.Linked = append(row.Linked, slug)
				continue
			}
			if row.Epic != "" || ghDown {
				continue
			}
			// An unresolved related URL might be the change's coordination
			// epic (specsync epic never writes a local link back to the
			// child, only a remote "## Related" edit — see epic.go). Confirm
			// via the epic's "type:epic" label rather than guessing from
			// text, since a sibling link renders identically.
			ref := refFromURL(link.URL)
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
			}
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Slug < rows[j].Slug })

	return &Topology{Changes: rows, Status: status}, nil
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
