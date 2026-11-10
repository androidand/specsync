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
	keyBySlug := map[string]string{}
	for _, r := range rows {
		if r.Slug != "" {
			keyBySlug[r.Store+"\x00"+r.Slug] = RowKey(r)
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
			if other, ok := keyBySlug[r.Store+"\x00"+slug]; ok {
				a, b := key, other
				if b < a {
					a, b = b, a
				}
				add(WorkEdge{From: a, To: b, Kind: WorkEdgeLinked, Source: "specsync"})
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
