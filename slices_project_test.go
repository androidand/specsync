package specsync

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type sliceProjProvider struct {
	*fakeEpicProvider
	links []string
	deps  []string
}

func (p *sliceProjProvider) LinkSubIssue(_ context.Context, parent, child Ref) error {
	p.links = append(p.links, parent.ID+">"+child.ID)
	return nil
}

func (p *sliceProjProvider) AddBlockedByRef(_ context.Context, issue, blocker Ref) error {
	p.deps = append(p.deps, issue.ID+"<-"+blocker.ID)
	return nil
}

const projectedTasks = "# Tasks\n\n## Slice 1: Parser\ncontract: parse\n- [ ] a\n\n## Slice 2: Commands\nafter: 1\ncontract: list\n- [ ] b\n\n## Slice 3: Docs\nafter: 1, 2\ncontract: docs\n- [ ] c\n"

func projChange(t *testing.T, tasks string) (Change, Ref, *sliceProjProvider) {
	t.Helper()
	p := &sliceProjProvider{fakeEpicProvider: newFakeEpicProvider("acme/app")}
	parent, err := p.Push(context.Background(), WorkItem{Slug: "big", Title: "Big", Body: "x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return Change{Slug: "big", Title: "Big", TasksMarkdown: tasks}, parent, p
}

func TestProjectSlices_ChildPerSliceAndBlockedByPerAfterAndIdempotent(t *testing.T) {
	c, parent, p := projChange(t, projectedTasks)
	creates := p.creates
	res, err := ProjectSlices(context.Background(), c, parent, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.creates-creates != 3 || len(res.Children) != 3 || res.Fallback {
		t.Fatalf("want one child per declared slice, created %d: %+v", p.creates-creates, res)
	}
	if len(p.links) != 3 {
		t.Fatalf("links = %v", p.links)
	}
	if len(p.deps) != 3 {
		t.Fatalf("one blocked-by per after (2<-1, 3<-1, 3<-2), got %v", p.deps)
	}
	again, err := ProjectSlices(context.Background(), c, parent, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.creates-creates != 3 || len(p.links) != 3 || len(p.deps) != 3 {
		t.Fatalf("a second run must change nothing: creates=%d links=%v deps=%v", p.creates-creates, p.links, p.deps)
	}
	if len(again.Children) != 3 {
		t.Fatalf("children = %+v", again.Children)
	}
}

func TestProjectSlices_DryRunListsActionsAndCreatesNothing(t *testing.T) {
	c, parent, p := projChange(t, projectedTasks)
	creates := p.creates
	res, err := ProjectSlices(context.Background(), c, parent, p, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.creates != creates || len(p.links) != 0 || len(p.deps) != 0 {
		t.Fatalf("dry run touched the tracker: creates=%d links=%v deps=%v", p.creates-creates, p.links, p.deps)
	}
	text := strings.Join(res.Actions, "\n")
	for _, want := range []string{"slice 1", "slice 2", "slice 3", "sub-issue of", "blocked by slice 1", "blocked by slice 2"} {
		if !strings.Contains(text, want) {
			t.Errorf("dry run lacks %q:\n%s", want, text)
		}
	}
}

func TestProjectSlices_NoSlicesDoesNothing(t *testing.T) {
	c, parent, p := projChange(t, "# Tasks\n- [ ] a\n")
	creates := p.creates
	res, err := ProjectSlices(context.Background(), c, parent, p, false)
	if err != nil || p.creates != creates || len(res.Children) != 0 || len(res.Actions) != 0 {
		t.Fatalf("a change without slices projects none: %+v %v", res, err)
	}
}

func TestProjectSlices_ProviderWithoutHierarchyFallsBack(t *testing.T) {
	c, parent, _ := projChange(t, projectedTasks)
	plain := newFakeEpicProvider("acme/app")
	res, err := ProjectSlices(context.Background(), c, parent, plain, false)
	if err != nil || !res.Fallback || len(res.Children) != 0 || plain.creates != 0 {
		t.Fatalf("no hierarchy support means fallback and no error: %+v %v", res, err)
	}
}

func TestSlicesSection_ListsOnlyDeclaredSlices(t *testing.T) {
	sec := SlicesSection(ParseSlices(projectedTasks))
	for _, want := range []string{"## Slices", "Slice 1: Parser", "parse", "Slice 3: Docs", "after 1, 2"} {
		if !strings.Contains(sec, want) {
			t.Errorf("section lacks %q:\n%s", want, sec)
		}
	}
	if SlicesSection(ParseSlices("# Tasks\n- [ ] a\n")) != "" {
		t.Fatal("no slices, no section")
	}
}

func TestSync_SlicesFlagProjectsOnlyOnRequest(t *testing.T) {
	newRoot := func() string {
		root := t.TempDir()
		mustWrite(t, root+"/changes/big/proposal.md", "# Big\n\nwhy\n")
		mustWrite(t, root+"/changes/big/tasks.md", projectedTasks)
		return root
	}

	off := &sliceProjProvider{fakeEpicProvider: newFakeEpicProvider("acme/app")}
	if _, err := Sync(context.Background(), Options{OpenSpecDir: newRoot(), Provider: off, Slug: "big"}); err != nil {
		t.Fatal(err)
	}
	if off.creates != 1 || len(off.links) != 0 {
		t.Fatalf("without Slices only the change's own issue exists: creates=%d links=%v", off.creates, off.links)
	}

	on := &sliceProjProvider{fakeEpicProvider: newFakeEpicProvider("acme/app")}
	res, err := Sync(context.Background(), Options{OpenSpecDir: newRoot(), Provider: on, Slug: "big", Slices: true})
	if err != nil {
		t.Fatal(err)
	}
	if on.creates != 4 || len(on.links) != 3 || len(on.deps) != 3 {
		t.Fatalf("with Slices: 1 change + 3 slices expected, creates=%d links=%v deps=%v", on.creates, on.links, on.deps)
	}
	if len(res.Items) != 1 || len(res.Items[0].Slices) == 0 {
		t.Fatalf("result should report the slice actions: %+v", res.Items)
	}

	plain := newFakeEpicProvider("acme/app")
	if _, err := Sync(context.Background(), Options{OpenSpecDir: newRoot(), Provider: plain, Slug: "big", Slices: true}); err != nil {
		t.Fatal(err)
	}
	if plain.creates != 1 {
		t.Fatalf("a provider without hierarchy gets no child issues, creates=%d", plain.creates)
	}
	body := ""
	for _, iss := range plain.issues {
		body = iss.body
	}
	if !strings.Contains(body, "## Slices") || !strings.Contains(body, fmt.Sprintf("Slice %d: Docs", 3)) {
		t.Fatalf("the slices are listed in the issue body instead:\n%s", body)
	}
}

func TestGitHubProvider_SliceProjectorUsesNodeIdsAndTheRightMutations(t *testing.T) {
	var calls []string
	fake := func(_ context.Context, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		switch {
		case strings.Contains(joined, "resolveNodeID") || strings.Contains(joined, "issue(number"):
			if strings.Contains(joined, "number=7") {
				return `{"data":{"repository":{"issue":{"id":"I_parent"}}}}`, nil
			}
			return `{"data":{"repository":{"issue":{"id":"I_child"}}}}`, nil
		}
		return `{"data":{}}`, nil
	}
	p := NewGitHubProviderFuncWithRepo("acme/app", fake)
	parent := Ref{ID: "7", URL: "https://github.com/acme/app/issues/7"}
	child := Ref{ID: "9", URL: "https://github.com/acme/app/issues/9"}
	if err := p.LinkSubIssue(context.Background(), parent, child); err != nil {
		t.Fatal(err)
	}
	if err := p.AddBlockedByRef(context.Background(), child, parent); err != nil {
		t.Fatal(err)
	}
	all := strings.Join(calls, "\n")
	for _, want := range []string{"addSubIssue", "issueId=I_parent", "subIssueId=I_child", "addBlockedBy", "issueId=I_child", "blockedById=I_parent"} {
		if !strings.Contains(all, want) {
			t.Errorf("calls lack %q:\n%s", want, all)
		}
	}
	var _ SliceProjector = p
}
