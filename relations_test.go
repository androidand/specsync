package specsync

import (
	"reflect"
	"testing"
)

func issueRow(repo, slug, id string) ChangeTopology {
	return ChangeTopology{Slug: slug, Title: "Scope the list", Store: "/s/" + repo, Repo: repo,
		Issue: &TopologyIssue{Provider: "github:" + repo, ID: id}}
}

func TestBuildWorkGraph_AssertedEdgesOnly(t *testing.T) {
	a := issueRow("acme/api", "scope-list", "12")
	a.Epic = "https://github.com/acme/web/issues/1"
	a.BlockedBy = []string{"acme/web#2"}
	a.Blocks = []string{"acme/docs#3"}
	a.Linked = []string{"sibling"}
	a.Branch, a.Worktree = "spike", "/w/spike"
	a.PR = &ChangeRequest{Number: 40, Closes: []string{"12"}}
	b := issueRow("acme/api", "sibling", "13")
	b.Linked = []string{"scope-list"}

	g := BuildWorkGraph([]ChangeTopology{a, b})
	want := []WorkEdge{
		{"acme/api!40", "acme/api#12", WorkEdgeCloses, "provider"},
		{"acme/api#12", "acme/web#1", WorkEdgeEpic, "specsync"},
		{"acme/api#12", "acme/web#2", WorkEdgeBlockedBy, "specsync"},
		{"acme/docs#3", "acme/api#12", WorkEdgeBlockedBy, "specsync"},
		{"acme/api#12", "acme/api#13", WorkEdgeLinked, "specsync"},
		{"acme/api:spike", "/w/spike", WorkEdgeWorktree, "git"},
	}
	if len(g.Edges) != len(want) {
		t.Fatalf("edges = %+v", g.Edges)
	}
	have := map[WorkEdge]bool{}
	for _, e := range g.Edges {
		have[e] = true
	}
	for _, e := range want {
		if !have[e] {
			t.Errorf("missing edge %+v in %+v", e, g.Edges)
		}
	}
	if got := g.Targets(WorkEdgeBlockedBy, "acme/api#12"); !reflect.DeepEqual(got, []string{"acme/web#2"}) {
		t.Fatalf("targets = %v", got)
	}
	if got := g.Sources(WorkEdgeBlockedBy, "acme/api#12"); !reflect.DeepEqual(got, []string{"acme/docs#3"}) {
		t.Fatalf("sources = %v", got)
	}
}

func TestBuildWorkGraph_SimilarTitlesCreateNoEdge(t *testing.T) {
	a := issueRow("acme/api", "scope-list", "12")
	b := issueRow("acme/web", "scope-list-ui", "5")
	b.Title = a.Title
	if g := BuildWorkGraph([]ChangeTopology{a, b}); len(g.Edges) != 0 {
		t.Fatalf("no asserted link means no edge, got %+v", g.Edges)
	}
}

func TestRowKey(t *testing.T) {
	if k := RowKey(issueRow("acme/api", "x", "12")); k != "acme/api#12" {
		t.Fatal(k)
	}
	if k := RowKey(ChangeTopology{Repo: "acme/api", Slug: "x"}); k != "acme/api:x" {
		t.Fatal(k)
	}
	if k := RowKey(ChangeTopology{Repo: "acme/api", NoChange: true, Branch: "spike"}); k != "acme/api:spike" {
		t.Fatal(k)
	}
}
