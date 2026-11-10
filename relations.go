package specsync

import (
	"sort"
	"strconv"
)

// WorkEdgeKind names an asserted relation between two work items.
type WorkEdgeKind string

const (
	WorkEdgeEpic      WorkEdgeKind = "epic"       // From is a child of epic To
	WorkEdgeBlockedBy WorkEdgeKind = "blocked-by" // From is blocked by To
	WorkEdgeLinked    WorkEdgeKind = "linked"     // two changes that link each other
	WorkEdgeCloses    WorkEdgeKind = "closes"     // request From closes issue To
	WorkEdgeWorktree  WorkEdgeKind = "worktree"   // branch From is checked out at path To
)

// WorkEdge is one asserted relation. Source says where the assertion came from:
// "provider", "git" or "specsync" (the change's own files).
type WorkEdge struct {
	From   string       `json:"from"`
	To     string       `json:"to"`
	Kind   WorkEdgeKind `json:"kind"`
	Source string       `json:"source"`
}

// WorkGraph holds only edges something asserted. Nothing is inferred from names,
// titles or similarity.
type WorkGraph struct {
	Edges []WorkEdge `json:"edges"`
	// Unresolved lists links that point at work not found among the rows.
	Unresolved []string `json:"unresolved,omitempty"`
}

// RowKey names a row's work item: its issue as owner/repo#N, else its change
// as owner/repo:slug, else its branch as owner/repo:branch.
func RowKey(r ChangeTopology) string {
	if r.Issue != nil {
		if k := refKey(Ref{Provider: r.Issue.Provider, ID: r.Issue.ID}); k != "" {
			return k
		}
	}
	if r.Slug != "" {
		return r.Repo + ":" + r.Slug
	}
	return r.Repo + ":" + r.Branch
}

// BuildWorkGraph derives the relations graph from rows. Epic and blocked-by edges
// come from the changes' own files; the epic is one the provider labels as an
// epic. Request-closes-issue edges come from the provider and branch-to-worktree
// edges from git.
func BuildWorkGraph(rows []ChangeTopology) *WorkGraph {
	g := &WorkGraph{}
	seen := map[WorkEdge]bool{}
	add := func(e WorkEdge) {
		if e.From == "" || e.To == "" || e.From == e.To || seen[e] {
			return
		}
		seen[e] = true
		g.Edges = append(g.Edges, e)
	}
	bySlug := map[string][]ChangeTopology{}
	known := map[string]bool{}
	for _, r := range rows {
		if r.Slug != "" {
			bySlug[r.Slug] = append(bySlug[r.Slug], r)
		}
		known[RowKey(r)] = true
	}
	link := func(from, to string) {
		if to < from {
			from, to = to, from
		}
		add(WorkEdge{From: from, To: to, Kind: WorkEdgeLinked, Source: "specsync"})
	}
	unresolved := map[string]bool{}
	note := func(from, to string) {
		msg := "unresolved link: " + from + " -> " + to + " (not found in the workspace)"
		if !unresolved[msg] {
			unresolved[msg] = true
			g.Unresolved = append(g.Unresolved, msg)
		}
	}
	for _, r := range rows {
		key := RowKey(r)
		if ref := refFromURL(r.Epic); ref != nil {
			add(WorkEdge{From: key, To: refKey(*ref), Kind: WorkEdgeEpic, Source: "specsync"})
		}
		for _, b := range r.BlockedBy {
			add(WorkEdge{From: key, To: b, Kind: WorkEdgeBlockedBy, Source: "specsync"})
		}
		for _, b := range r.Blocks {
			add(WorkEdge{From: b, To: key, Kind: WorkEdgeBlockedBy, Source: "specsync"})
		}
		for _, slug := range r.Linked {
			var pick []ChangeTopology
			for _, c := range bySlug[slug] {
				if RowKey(c) != key {
					pick = append(pick, c)
				}
			}
			if len(pick) > 1 {
				var same []ChangeTopology
				for _, c := range pick {
					if SameFolder(c.Store, r.Store) {
						same = append(same, c)
					}
				}
				pick = same
			}
			if len(pick) != 1 {
				note(key, slug)
				continue
			}
			link(key, RowKey(pick[0]))
		}
		for _, ref := range r.LinkedRefs {
			if known[ref] {
				link(key, ref)
			} else {
				note(key, ref)
			}
		}
		if r.PR != nil && r.Repo != "" {
			for _, n := range r.PR.Closes {
				add(WorkEdge{From: r.Repo + "!" + strconv.Itoa(r.PR.Number), To: r.Repo + "#" + n, Kind: WorkEdgeCloses, Source: "provider"})
			}
		}
		if r.Branch != "" && r.Worktree != "" {
			add(WorkEdge{From: r.Repo + ":" + r.Branch, To: r.Worktree, Kind: WorkEdgeWorktree, Source: "git"})
		}
	}
	sort.Strings(g.Unresolved)
	sort.Slice(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	return g
}

// Targets returns the To of every edge of the kind that starts at from.
func (g *WorkGraph) Targets(kind WorkEdgeKind, from string) []string {
	var out []string
	for _, e := range g.Edges {
		if e.Kind == kind && e.From == from {
			out = append(out, e.To)
		}
	}
	return out
}

// Sources returns the From of every edge of the kind that ends at to.
func (g *WorkGraph) Sources(kind WorkEdgeKind, to string) []string {
	var out []string
	for _, e := range g.Edges {
		if e.Kind == kind && e.To == to {
			out = append(out, e.From)
		}
	}
	return out
}

// RelatedItem is one work item related to the current work. Row is nil when the
// item is not in the discovered workspace: only its key is known.
type RelatedItem struct {
	Key string          `json:"key"`
	Row *ChangeTopology `json:"row,omitempty"`
}

// RelatedEpic is the epic above the current work, with every known child.
// Done and Total count the known children, not the epic's whole list.
type RelatedEpic struct {
	Key   string        `json:"key"`
	Above []string      `json:"above,omitempty"`
	Items []RelatedItem `json:"items"`
	Done  int           `json:"done"`
	Total int           `json:"total"`
}

// RelatedWork is the current work and everything asserted to relate to it.
type RelatedWork struct {
	Current    string        `json:"current"`
	Epic       *RelatedEpic  `json:"epic,omitempty"`
	BlockedBy  []RelatedItem `json:"blockedBy,omitempty"`
	Blocks     []RelatedItem `json:"blocks,omitempty"`
	Linked     []RelatedItem `json:"linked,omitempty"`
	Incomplete []string      `json:"incomplete,omitempty"`
}

// RelatedTo derives what relates to current from every known row. It returns
// nil when nothing is asserted, so the caller shows the work alone. incomplete
// carries the reasons a source was unavailable.
func RelatedTo(current ChangeTopology, all []ChangeTopology, incomplete []string) *RelatedWork {
	g := BuildWorkGraph(all)
	key := RowKey(current)
	rows := map[string]*ChangeTopology{}
	for i := range all {
		rows[RowKey(all[i])] = &all[i]
	}
	item := func(k string) RelatedItem { return RelatedItem{Key: k, Row: rows[k]} }
	items := func(keys []string) []RelatedItem {
		sort.Strings(keys)
		var out []RelatedItem
		for i, k := range keys {
			if i > 0 && keys[i-1] == k {
				continue
			}
			out = append(out, item(k))
		}
		return out
	}

	rel := &RelatedWork{Current: key, Incomplete: incomplete}
	rel.BlockedBy = items(g.Targets(WorkEdgeBlockedBy, key))
	rel.Blocks = items(g.Sources(WorkEdgeBlockedBy, key))
	rel.Linked = items(append(g.Targets(WorkEdgeLinked, key), g.Sources(WorkEdgeLinked, key)...))

	if epics := g.Targets(WorkEdgeEpic, key); len(epics) > 0 {
		epic := &RelatedEpic{Key: epics[0]}
		seen := map[string]bool{key: true, epic.Key: true}
		for top := epic.Key; ; {
			up := g.Targets(WorkEdgeEpic, top)
			if len(up) == 0 || seen[up[0]] {
				break
			}
			seen[up[0]] = true
			epic.Above = append(epic.Above, up[0])
			top = up[0]
		}
		epic.Items = items(g.Sources(WorkEdgeEpic, epic.Key))
		for _, it := range epic.Items {
			epic.Total++
			if it.Row != nil && rowDone(*it.Row) {
				epic.Done++
			}
		}
		rel.Epic = epic
	}
	if rel.Epic == nil && len(rel.BlockedBy) == 0 && len(rel.Blocks) == 0 && len(rel.Linked) == 0 {
		return nil
	}
	return rel
}

// rowDone reports whether a row's work is finished: its issue is closed, its
// request merged, or every task checked.
func rowDone(r ChangeTopology) bool {
	if r.Issue != nil && r.Issue.State == "closed" {
		return true
	}
	if r.PR != nil && r.PR.State == "merged" {
		return true
	}
	return r.Tasks != nil && r.Tasks.Total > 0 && r.Tasks.Done == r.Tasks.Total
}
