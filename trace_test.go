package specsync

import (
	"strings"
	"testing"
)

func sampleInput() TraceInput {
	return TraceInput{
		Changes: []ChangeRefs{
			{Change: Change{Slug: "modal-split", Title: "Split the integration modal", Body: "work on the modal"}, IssueIDs: []string{"6"}},
			{Change: Change{Slug: "csv-export", Title: "Add CSV export", Body: "export data"}, IssueIDs: []string{"7"}},
		},
		Commits: []Commit{
			ParseCommit("aaaaaaaa", "Dev", "2026-06-01T10:00:00Z", "feat(ui): split modal (#51)\n\nCloses #6"),
			ParseCommit("bbbbbbbb", "Dev", "2026-06-02T10:00:00Z", "fix: tidy\n\nCloses #7"),
			ParseCommit("cccccccc", "Dev", "2026-06-03T10:00:00Z", "feat: orphan work with no link"),
		},
	}
}

func hasNode(tr Trace, id string) bool {
	for _, n := range tr.Nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}

func hasLink(tr Trace, from, to string, p Provenance) bool {
	for _, l := range tr.Links {
		if l.From == from && l.To == to && l.Provenance == p {
			return true
		}
	}
	return false
}

func TestResolveRangeLinksCommitsAndReportsGap(t *testing.T) {
	tr := ResolveTrace(sampleInput(), Scope{Since: "v0.1.0", Until: "HEAD"})

	if !hasNode(tr, "change:modal-split") || !hasNode(tr, "commit:aaaaaaaa") {
		t.Fatalf("expected change and commit nodes, got %+v", tr.Nodes)
	}
	if !hasLink(tr, "change:modal-split", "commit:aaaaaaaa", ProvCommitFooter) {
		t.Fatalf("expected change->commit link, got %+v", tr.Links)
	}
	if !hasLink(tr, "change:modal-split", "issue:6", ProvRefCache) {
		t.Fatalf("expected change->issue link, got %+v", tr.Links)
	}
	if !hasNode(tr, "pr:51") {
		t.Fatalf("expected PR node from trailing (#51)")
	}
	// The orphan feat commit must be a reported gap, not a fabricated link.
	var foundGap bool
	for _, g := range tr.Gaps {
		if g.Kind == "unlinked-commit" && g.Subject == "cccccccc" {
			foundGap = true
		}
	}
	if !foundGap {
		t.Fatalf("expected unlinked-commit gap for cccccccc, gaps: %+v", tr.Gaps)
	}
}

func TestResolveChangeScopeIsolates(t *testing.T) {
	tr := ResolveTrace(sampleInput(), Scope{Change: "modal-split"})
	if hasNode(tr, "change:csv-export") {
		t.Fatalf("change scope must not include other changes")
	}
	if !hasNode(tr, "change:modal-split") || !hasNode(tr, "commit:aaaaaaaa") {
		t.Fatalf("change scope must include the change and its commit")
	}
}

func TestResolveAreaTopicMatch(t *testing.T) {
	tr := ResolveTrace(sampleInput(), Scope{Topic: "modal"})
	if !hasNode(tr, "change:modal-split") {
		t.Fatalf("topic 'modal' should match modal-split")
	}
	if hasNode(tr, "change:csv-export") {
		t.Fatalf("topic 'modal' should not match csv-export")
	}
}

func TestResolveDeterministicOrder(t *testing.T) {
	a := ResolveTrace(sampleInput(), Scope{Since: "v0", Until: "HEAD"})
	b := ResolveTrace(sampleInput(), Scope{Since: "v0", Until: "HEAD"})
	if len(a.Nodes) != len(b.Nodes) {
		t.Fatalf("node count differs")
	}
	for i := range a.Nodes {
		if a.Nodes[i].ID != b.Nodes[i].ID {
			t.Fatalf("node order not stable at %d: %q vs %q", i, a.Nodes[i].ID, b.Nodes[i].ID)
		}
	}
	// Nodes must be grouped by kind: changes precede commits.
	var firstCommit, lastChange int = -1, -1
	for i, n := range a.Nodes {
		if n.Kind == NodeChange {
			lastChange = i
		}
		if n.Kind == NodeCommit && firstCommit == -1 {
			firstCommit = i
		}
	}
	if firstCommit != -1 && lastChange != -1 && firstCommit < lastChange {
		t.Fatalf("expected change nodes before commit nodes")
	}
	_ = strings.TrimSpace
}
