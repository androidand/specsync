package specsync

import (
	"sort"
	"strings"
)

// Provenance records how a link between two trace nodes was discovered. It is
// reported so a human can see *why* two artifacts are connected — links are
// asserted from real evidence, never inferred.
type Provenance string

const (
	ProvMarker       Provenance = "marker"        // specsync:change= marker in an issue body
	ProvBranch       Provenance = "branch"        // issue-linked branch name
	ProvCommitFooter Provenance = "commit-footer" // a reference in a commit message
	ProvPRBody       Provenance = "pr-body"       // a reference read from a PR body via gh
	ProvRefCache     Provenance = "ref-cache"     // the local refs.json binding
	ProvLinksMD      Provenance = "links-md"      // a links.md entry
)

// NodeKind is the type of a trace node.
type NodeKind string

const (
	NodeChange   NodeKind = "change"
	NodeWorkItem NodeKind = "issue"
	NodePR       NodeKind = "pr"
	NodeCommit   NodeKind = "commit"
)

// TraceNode is one artifact in the graph. ID is unique across kinds (it is
// kind-prefixed); Label is human-facing.
type TraceNode struct {
	Kind  NodeKind
	ID    string
	Label string
}

// TraceLink is a directed edge with the provenance that established it.
type TraceLink struct {
	From       string
	To         string
	Provenance Provenance
}

// Gap is an unresolved relationship — reported, never fabricated into a link.
type Gap struct {
	Kind    string // e.g. "unlinked-commit"
	Subject string // the artifact with no resolved link
	Reason  string
}

// Scope selects what a trace covers: a single change, a revision range, or an
// area (paths and/or a topic). One resolver serves all three so inbound
// (planning) and outbound (release) consumers share it.
type Scope struct {
	Change string   // single-change scope
	Since  string   // revision range
	Until  string   // revision range
	Paths  []string // area: path globs (commits already filtered to these by the caller)
	Topic  string   // area: case-insensitive substring
}

func (s Scope) isChange() bool { return s.Change != "" }
func (s Scope) isArea() bool   { return len(s.Paths) > 0 || s.Topic != "" }

// ChangeRefs pairs a change with the provider issue ids bound to it (from the
// ref cache), so the resolver can link commits that reference those issues.
type ChangeRefs struct {
	Change   Change
	IssueIDs []string // provider issue numbers, e.g. "42"
}

// TraceInput is everything the resolver needs, already gathered from disk and
// the source CLIs. Keeping resolution pure over this input makes it
// deterministic and unit-testable without I/O.
type TraceInput struct {
	Changes []ChangeRefs
	Commits []Commit
}

// Trace is the resolved graph.
type Trace struct {
	Nodes []TraceNode
	Links []TraceLink
	Gaps  []Gap
}

// ResolveTrace builds the trace for a scope from gathered input. It links
// commits to changes by matching commit issue/PR references against each
// change's bound issue ids, adds work-item and PR nodes from that evidence, and
// reports commits that link to no change as gaps. Output is deterministically
// ordered so repeated runs and --json diffs are byte-identical.
func ResolveTrace(in TraceInput, scope Scope) Trace {
	b := &traceBuilder{}

	// Decide which changes are in scope for output. Inclusion has a direct
	// criterion per scope; commit linking (below) adds more for range/path scopes:
	//   - change scope  → the named change only
	//   - topic area     → topic matches only (commits never auto-include)
	//   - range / paths  → driven entirely by commit links below
	included := map[string]bool{}
	topicOnly := scope.Topic != "" && len(scope.Paths) == 0
	for _, cr := range in.Changes {
		switch {
		case scope.isChange():
			if cr.Change.Slug == scope.Change {
				included[cr.Change.Slug] = true
			}
		case scope.Topic != "":
			if topicMatches(cr.Change, scope.Topic) {
				included[cr.Change.Slug] = true
			}
		}
	}

	// Index issue id -> change slug for commit linking, across ALL input changes
	// (a commit may link a change into an area scope even without a topic match).
	issueToChange := map[string]string{}
	changeByIssue := map[string][]string{}
	for _, cr := range in.Changes {
		for _, id := range cr.IssueIDs {
			issueToChange[id] = cr.Change.Slug
			changeByIssue[cr.Change.Slug] = append(changeByIssue[cr.Change.Slug], id)
		}
	}

	// Link commits to changes (verb) via referenced issue numbers.
	for _, c := range in.Commits {
		linkedSlug := ""
		for _, ref := range append(append([]string{}, c.IssueRefs...), c.PRRefs...) {
			num := refNumber(ref)
			if slug, ok := issueToChange[num]; ok {
				linkedSlug = slug
				break
			}
		}
		if linkedSlug == "" {
			// Unlinked contributor — reported, never invented into a link.
			if c.ConventionalOK {
				b.addGap(Gap{Kind: "unlinked-commit", Subject: shortHash(c.Hash), Reason: "references no issue/change"})
			}
			continue
		}
		switch {
		case scope.isChange():
			// Single-change view: ignore commits linking any other change.
			if linkedSlug != scope.Change {
				continue
			}
		case topicOnly:
			// Topic-only area: show commits of topic-matched changes, but a commit
			// never pulls in a change that didn't match the topic.
			if !included[linkedSlug] {
				continue
			}
		default:
			// Range, or area with paths (caller pre-filtered commits to the paths):
			// a linked commit includes its change.
			included[linkedSlug] = true
		}
		b.addNode(TraceNode{Kind: NodeCommit, ID: nodeID(NodeCommit, c.Hash), Label: commitLabel(c)})
		b.addLink(TraceLink{From: nodeID(NodeChange, linkedSlug), To: nodeID(NodeCommit, c.Hash), Provenance: ProvCommitFooter})
		for _, pr := range c.PRRefs {
			b.addNode(TraceNode{Kind: NodePR, ID: nodeID(NodePR, refNumber(pr)), Label: pr})
			b.addLink(TraceLink{From: nodeID(NodeCommit, c.Hash), To: nodeID(NodePR, refNumber(pr)), Provenance: ProvCommitFooter})
		}
	}

	// Emit change and work-item nodes for every included change.
	for _, cr := range in.Changes {
		if !included[cr.Change.Slug] {
			continue
		}
		b.addNode(TraceNode{Kind: NodeChange, ID: nodeID(NodeChange, cr.Change.Slug), Label: cr.Change.Title})
		for _, id := range cr.IssueIDs {
			b.addNode(TraceNode{Kind: NodeWorkItem, ID: nodeID(NodeWorkItem, id), Label: "#" + id})
			b.addLink(TraceLink{From: nodeID(NodeChange, cr.Change.Slug), To: nodeID(NodeWorkItem, id), Provenance: ProvRefCache})
		}
	}

	// Drop commit/PR nodes and links that dangle off an excluded change.
	b.prune(included, scope)
	b.sortAll()
	return Trace{Nodes: b.nodes, Links: b.links, Gaps: b.gaps}
}

type traceBuilder struct {
	nodes   []TraceNode
	links   []TraceLink
	gaps    []Gap
	nodeSet map[string]bool
	linkSet map[string]bool
	gapSet  map[string]bool
}

func (b *traceBuilder) addNode(n TraceNode) {
	if b.nodeSet == nil {
		b.nodeSet = map[string]bool{}
	}
	if b.nodeSet[n.ID] {
		return
	}
	b.nodeSet[n.ID] = true
	b.nodes = append(b.nodes, n)
}

func (b *traceBuilder) addLink(l TraceLink) {
	if b.linkSet == nil {
		b.linkSet = map[string]bool{}
	}
	key := l.From + "|" + l.To + "|" + string(l.Provenance)
	if b.linkSet[key] {
		return
	}
	b.linkSet[key] = true
	b.links = append(b.links, l)
}

func (b *traceBuilder) addGap(g Gap) {
	if b.gapSet == nil {
		b.gapSet = map[string]bool{}
	}
	key := g.Kind + "|" + g.Subject
	if b.gapSet[key] {
		return
	}
	b.gapSet[key] = true
	b.gaps = append(b.gaps, g)
}

// prune removes change nodes (and their dangling links) for changes that ended
// up excluded after commit linking — keeps the graph to the resolved scope.
func (b *traceBuilder) prune(included map[string]bool, scope Scope) {
	if !scope.isArea() && !scope.isChange() {
		return // range scope: keep everything resolved
	}
	keep := func(id string) bool {
		if strings.HasPrefix(id, string(NodeChange)+":") {
			return included[strings.TrimPrefix(id, string(NodeChange)+":")]
		}
		return true
	}
	var nodes []TraceNode
	for _, n := range b.nodes {
		if keep(n.ID) {
			nodes = append(nodes, n)
		}
	}
	b.nodes = nodes
	var links []TraceLink
	for _, l := range b.links {
		if keep(l.From) && keep(l.To) {
			links = append(links, l)
		}
	}
	b.links = links
}

func (b *traceBuilder) sortAll() {
	kindOrder := map[NodeKind]int{NodeChange: 0, NodeWorkItem: 1, NodePR: 2, NodeCommit: 3}
	sort.SliceStable(b.nodes, func(i, j int) bool {
		if kindOrder[b.nodes[i].Kind] != kindOrder[b.nodes[j].Kind] {
			return kindOrder[b.nodes[i].Kind] < kindOrder[b.nodes[j].Kind]
		}
		return b.nodes[i].ID < b.nodes[j].ID
	})
	sort.SliceStable(b.links, func(i, j int) bool {
		if b.links[i].From != b.links[j].From {
			return b.links[i].From < b.links[j].From
		}
		if b.links[i].To != b.links[j].To {
			return b.links[i].To < b.links[j].To
		}
		return b.links[i].Provenance < b.links[j].Provenance
	})
	sort.SliceStable(b.gaps, func(i, j int) bool {
		if b.gaps[i].Kind != b.gaps[j].Kind {
			return b.gaps[i].Kind < b.gaps[j].Kind
		}
		return b.gaps[i].Subject < b.gaps[j].Subject
	})
}

func nodeID(kind NodeKind, raw string) string { return string(kind) + ":" + raw }

// refNumber returns the trailing issue/PR number from "#42" or "owner/repo#7".
func refNumber(ref string) string {
	if i := strings.LastIndex(ref, "#"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

func commitLabel(c Commit) string {
	if c.ConventionalOK && c.Type != "" {
		return shortHash(c.Hash) + " " + c.Type + ": " + c.Description
	}
	return shortHash(c.Hash) + " " + c.Description
}

func topicMatches(c Change, topic string) bool {
	t := strings.ToLower(topic)
	return strings.Contains(strings.ToLower(c.Title), t) ||
		strings.Contains(strings.ToLower(c.Body), t)
}
