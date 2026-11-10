package specsync

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// OverviewScope narrows an overview. Fields combine: a row must satisfy all.
type OverviewScope struct {
	Epic       string // an issue URL, owner/repo#N or #N
	Repo       string // owner/name or just name
	Stage      string
	Mine       bool
	Attention  bool
	Unattached bool // work under no epic
}

// Describe lists the scope that applies, one phrase each, for printing.
func (s OverviewScope) Describe() []string {
	var out []string
	if s.Epic != "" {
		out = append(out, "epic "+s.Epic)
	}
	if s.Repo != "" {
		out = append(out, "repo "+s.Repo)
	}
	if s.Stage != "" {
		out = append(out, "stage "+s.Stage)
	}
	if s.Mine {
		out = append(out, "mine")
	}
	if s.Attention {
		out = append(out, "needs attention")
	}
	if s.Unattached {
		out = append(out, "under no epic")
	}
	return out
}

// OverviewOptions says where to look and what to keep. Topology is a template
// for each clone's join: its LocalOnly, Cache, RefreshRequests and Sync apply.
type OverviewOptions struct {
	Workspace WorkspaceOptions
	Topology  TopologyOptions
	Scope     OverviewScope

	// SessionsCmd is the caller's session provider command line; empty means none.
	SessionsCmd     string
	SessionsTimeout time.Duration

	viewer func(ctx context.Context) (string, error)
}

// WorkRows is every row of every discovered clone.
type WorkRows struct {
	Workspace    *Workspace       `json:"workspace"`
	Rows         []ChangeTopology `json:"rows"`
	Status       []string         `json:"status,omitempty"`
	RequestState string           `json:"requestState,omitempty"`
}

// CollectWorkRows discovers the workspace and joins each clone's changes,
// worktrees and requests, including work that has no change. It never writes.
func CollectWorkRows(ctx context.Context, opts OverviewOptions) (*WorkRows, error) {
	ws, err := DiscoverWorkspace(ctx, opts.Workspace)
	if err != nil {
		return nil, err
	}
	out := &WorkRows{Workspace: ws, Status: append([]string(nil), ws.Status...)}
	for _, clone := range ws.Clones {
		to := opts.Topology
		to.OpenSpecDir = filepath.Join(clone.Path, "openspec")
		to.NoCoordination = true
		to.IncludeUnlinked = true
		to.Slug, to.Dir, to.All = "", "", false
		topo, err := BuildTopology(ctx, to)
		if err != nil {
			out.Status = append(out.Status, "could not read "+clone.Path+": "+errReason(err))
			continue
		}
		out.Rows = append(out.Rows, topo.Changes...)
		for _, s := range topo.Status {
			if clone.Repo == "" && strings.HasPrefix(s, "no git remote in store") {
				continue
			}
			out.Status = append(out.Status, s)
		}
		if topo.RequestState == "omitted" || (out.RequestState == "" && topo.RequestState != "") {
			out.RequestState = topo.RequestState
		}
	}
	sessionsCmd := opts.SessionsCmd
	if opts.Topology.LocalOnly && strings.TrimSpace(sessionsCmd) != "" {
		out.Status = append(out.Status, SessionsSkippedOffline)
		sessionsCmd = ""
	}
	out.Status = append(out.Status, ApplySessions(ctx, out.Rows, sessionsCmd, opts.SessionsTimeout, true)...)
	out.Status = append(out.Status, BuildWorkGraph(out.Rows).Unresolved...)
	out.Status = uniqueStrings(out.Status)
	return out, nil
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// IncompleteReasons drops the status lines that only report an unresolved
// link, which say nothing is missing from a source.
func IncompleteReasons(status []string) []string {
	var out []string
	for _, s := range status {
		if !strings.HasPrefix(s, "unresolved link:") {
			out = append(out, s)
		}
	}
	return out
}

// EpicGroup is the work under one epic.
type EpicGroup struct {
	Epic  string           `json:"epic"`
	Items []ChangeTopology `json:"items"`
	Done  int              `json:"done"`
	Total int              `json:"total"`
}

// Overview is all work, grouped by epic, then work under no epic, then work
// with no change.
type Overview struct {
	Scope        []string         `json:"scope"`
	Roots        []string         `json:"roots"`
	Epics        []EpicGroup      `json:"epics"`
	NoEpic       []ChangeTopology `json:"noEpic"`
	Unlinked     []ChangeTopology `json:"unlinked"`
	Status       []string         `json:"status,omitempty"`
	Incomplete   bool             `json:"incomplete,omitempty"`
	RequestState string           `json:"requestState,omitempty"`
}

// BuildOverview lists all work in the workspace under the scope.
func BuildOverview(ctx context.Context, opts OverviewOptions) (*Overview, error) {
	work, err := CollectWorkRows(ctx, opts)
	if err != nil {
		return nil, err
	}
	ov := &Overview{Scope: opts.Scope.Describe(), Roots: work.Workspace.Roots, Status: work.Status, RequestState: work.RequestState,
		Epics: []EpicGroup{}, NoEpic: []ChangeTopology{}, Unlinked: []ChangeTopology{}}

	me := ""
	if opts.Scope.Mine {
		get := opts.viewer
		if get == nil {
			get = viewerLogin
		}
		if login, err := get(ctx); err == nil {
			me = login
		} else {
			ov.Status = append(ov.Status, "viewer unknown ("+errReason(err)+"): -mine keeps work that has a local worktree and no request")
		}
	}

	groups := map[string]*EpicGroup{}
	for _, row := range work.Rows {
		if !opts.Scope.keeps(row, me) {
			continue
		}
		switch {
		case row.NoChange:
			ov.Unlinked = append(ov.Unlinked, row)
		case row.Epic != "":
			key := epicKeyOf(row.Epic)
			g := groups[key]
			if g == nil {
				g = &EpicGroup{Epic: key}
				groups[key] = g
			}
			g.Items = append(g.Items, row)
			g.Total++
			if rowDone(row) {
				g.Done++
			}
		default:
			ov.NoEpic = append(ov.NoEpic, row)
		}
	}
	for _, g := range groups {
		sortRows(g.Items)
		ov.Epics = append(ov.Epics, *g)
	}
	sort.Slice(ov.Epics, func(i, j int) bool { return ov.Epics[i].Epic < ov.Epics[j].Epic })
	sortRows(ov.NoEpic)
	sortRows(ov.Unlinked)
	ov.Incomplete = len(IncompleteReasons(ov.Status)) > 0
	return ov, nil
}

func sortRows(rows []ChangeTopology) {
	sort.SliceStable(rows, func(i, j int) bool { return RowKey(rows[i]) < RowKey(rows[j]) })
}

func epicKeyOf(url string) string {
	if ref := refFromURL(url); ref != nil {
		if k := refKey(*ref); k != "" {
			return k
		}
	}
	return url
}

func (s OverviewScope) keeps(row ChangeTopology, me string) bool {
	if s.Repo != "" && row.Repo != s.Repo && !strings.HasSuffix(row.Repo, "/"+s.Repo) {
		return false
	}
	if s.Stage != "" && row.Stage != s.Stage {
		return false
	}
	if s.Attention && len(row.Attention) == 0 {
		return false
	}
	if s.Unattached && row.Epic != "" {
		return false
	}
	if s.Epic != "" && !epicMatches(s.Epic, row.Epic) {
		return false
	}
	if s.Mine {
		if row.PR != nil && me != "" {
			return row.PR.Author == "" || row.PR.Author == me
		}
		return row.PR == nil && row.Worktree != ""
	}
	return true
}

// epicMatches compares a requested epic (URL, owner/repo#N or #N) with a row's.
func epicMatches(want, have string) bool {
	if have == "" {
		return false
	}
	key := epicKeyOf(have)
	if strings.HasPrefix(want, "#") {
		return strings.HasSuffix(key, want)
	}
	return epicKeyOf(want) == key
}

func viewerLogin(ctx context.Context) (string, error) {
	out, err := runGH(ctx, "api", "user", "-q", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
