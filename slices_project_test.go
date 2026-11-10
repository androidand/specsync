package specsync

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type sliceProjProvider struct {
	*fakeEpicProvider
	links     []string
	deps      []string
	linkSet   map[string]bool
	depSet    map[string]bool
	failLinks int
	finds     int
}

func (p *sliceProjProvider) Find(ctx context.Context, slug string) (*Ref, error) {
	p.finds++
	return p.fakeEpicProvider.Find(ctx, slug)
}

func (p *sliceProjProvider) LinkSubIssue(_ context.Context, parent, child Ref) error {
	if p.failLinks > 0 {
		p.failLinks--
		return fmt.Errorf("link failed")
	}
	if p.linkSet == nil {
		p.linkSet = map[string]bool{}
	}
	p.linkSet[parent.ID+">"+child.ID] = true
	p.links = append(p.links, parent.ID+">"+child.ID)
	return nil
}

func (p *sliceProjProvider) IsSubIssue(_ context.Context, parent, child Ref) (bool, error) {
	return p.linkSet[parent.ID+">"+child.ID], nil
}

func (p *sliceProjProvider) AddBlockedByRef(_ context.Context, issue, blocker Ref) error {
	if p.depSet == nil {
		p.depSet = map[string]bool{}
	}
	p.depSet[issue.ID+"<-"+blocker.ID] = true
	p.deps = append(p.deps, issue.ID+"<-"+blocker.ID)
	return nil
}

func (p *sliceProjProvider) BlockedByRefs(_ context.Context, issue Ref) ([]Ref, error) {
	var out []Ref
	for k := range p.depSet {
		if parts := strings.SplitN(k, "<-", 2); parts[0] == issue.ID {
			out = append(out, Ref{ID: parts[1], URL: p.urlFor(parts[1])})
		}
	}
	return out, nil
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

func TestProjectSlices_SelfHealsAndReportsFailures(t *testing.T) {
	c, parent, p := projChange(t, projectedTasks)
	p.failLinks = 1
	creates := p.creates
	res, err := ProjectSlices(context.Background(), c, parent, p, false)
	if err == nil || len(res.Failed) != 1 || !strings.Contains(res.Failed[0], "slice 1") {
		t.Fatalf("a failed nesting must be reported, naming the slice: err=%v failed=%v", err, res.Failed)
	}
	if p.creates-creates != 3 {
		t.Fatalf("one failure must not stop the other slices: created %d", p.creates-creates)
	}
	res, err = ProjectSlices(context.Background(), c, parent, p, false)
	if err != nil || len(res.Failed) != 0 {
		t.Fatalf("the next run heals: err=%v failed=%v", err, res.Failed)
	}
	if p.creates-creates != 3 || len(p.linkSet) != 3 || len(p.depSet) != 3 {
		t.Fatalf("creates=%d links=%v deps=%v", p.creates-creates, p.linkSet, p.depSet)
	}
	if len(p.links) != 3 || len(p.deps) != 3 {
		t.Fatalf("an existing nesting or edge must not be added again: links=%v deps=%v", p.links, p.deps)
	}
}

func TestProjectSlices_AddsAMissingEdgeToAnExistingChild(t *testing.T) {
	c, parent, p := projChange(t, projectedTasks)
	if _, err := ProjectSlices(context.Background(), c, parent, p, false); err != nil {
		t.Fatal(err)
	}
	for k := range p.depSet {
		delete(p.depSet, k)
		break
	}
	if _, err := ProjectSlices(context.Background(), c, parent, p, false); err != nil {
		t.Fatal(err)
	}
	if len(p.depSet) != 3 {
		t.Fatalf("a removed edge must be restored on the next run: %v", p.depSet)
	}
}

func TestProjectSlices_LooksEachChildUpOnce(t *testing.T) {
	c, parent, p := projChange(t, projectedTasks)
	p.finds = 0
	if _, err := ProjectSlices(context.Background(), c, parent, p, false); err != nil {
		t.Fatal(err)
	}
	if p.finds != 3 {
		t.Fatalf("one lookup per slice, no retry backoff: %d", p.finds)
	}
}

func TestGitHubProvider_FindQuotesColonSlugsAndFallsBackToTheSliceLabelList(t *testing.T) {
	var searches []string
	fake := func(_ context.Context, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--label type:slice") {
			return `[{"number":12,"url":"https://github.com/acme/app/issues/12","body":"<!-- specsync:change=big:slice-1 -->\n\nx"}]`, nil
		}
		searches = append(searches, joined)
		return "[]", nil
	}
	p := NewGitHubProviderFuncWithRepo("acme/app", fake)
	ref, err := p.Find(context.Background(), "big:slice-1")
	if err != nil || ref == nil || ref.ID != "12" {
		t.Fatalf("a search miss must fall back to the label list: %+v %v", ref, err)
	}
	if len(searches) != 1 || !strings.Contains(searches[0], `"specsync:change=big:slice-1" in:body`) {
		t.Fatalf("the colon-qualified marker must be quoted: %v", searches)
	}
	if miss, err := p.Find(context.Background(), "big:slice-9"); err != nil || miss != nil {
		t.Fatalf("a marker found nowhere is nil: %+v %v", miss, err)
	}
}

func TestSliceMarkersAreNotChangeSlugs(t *testing.T) {
	body := "<!-- specsync:change=big:slice-2 -->\n\nSlice 2"
	if got := slugFromMarker(body); got != "" {
		t.Fatalf("slugFromMarker must reject a slice marker, got %q", got)
	}
	if got := slugFromMarker("<!-- specsync:change=big -->"); got != "big" {
		t.Fatalf("a change marker still resolves: %q", got)
	}
	slug, n, ok := SliceMarker(body)
	if !ok || slug != "big" || n != 2 {
		t.Fatalf("SliceMarker = %q %d %v", slug, n, ok)
	}
	if _, _, ok := SliceMarker("<!-- specsync:change=big -->"); ok {
		t.Fatal("a change marker is not a slice marker")
	}

	f := newFakeEpicProvider("acme/app")
	ref, err := f.Push(context.Background(), WorkItem{Slug: "big:slice-2", Title: "Big — slice 2: Commands", Body: "x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Pull(context.Background(), PullOptions{OpenSpecDir: t.TempDir(), Provider: f, IssueID: ref.ID})
	if err == nil || !strings.Contains(err.Error(), "slice 2 of change") || !strings.Contains(err.Error(), "big") {
		t.Fatalf("pull on a slice issue must say what it is: %v", err)
	}
	specs := t.TempDir()
	mustWrite(t, specs+"/changes/other/proposal.md", "# Other\n")
	_, err = Adopt(context.Background(), AdoptOptions{OpenSpecDir: specs, Provider: f, IssueID: ref.ID, Slug: "other"})
	if err == nil || !strings.Contains(err.Error(), "slice 2 of change") {
		t.Fatalf("adopt must refuse a slice issue: %v", err)
	}
}

func TestGitHubProvider_ReadsNestingAndBlockedByBackForSelfHealing(t *testing.T) {
	fake := func(_ context.Context, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "readParent"+"") || strings.Contains(joined, "parent {"):
			return `{"data":{"repository":{"issue":{"parent":{"url":"https://github.com/acme/app/issues/7"}}}}}`, nil
		case strings.Contains(joined, "issueDependenciesSummary"):
			return `{"data":{"repository":{"issue":{"issueDependenciesSummary":{"blockedBy":{"edges":[{"node":{"id":"I_1","databaseId":5,"url":"https://github.com/acme/app/issues/5"}}]},"blocking":{"edges":[]}}}}}}`, nil
		}
		return `{"data":{}}`, nil
	}
	p := NewGitHubProviderFuncWithRepo("acme/app", fake)
	parent := Ref{URL: "https://github.com/acme/app/issues/7"}
	child := Ref{ID: "9", URL: "https://github.com/acme/app/issues/9"}
	if ok, err := p.IsSubIssue(context.Background(), parent, child); err != nil || !ok {
		t.Fatalf("nested: %v %v", ok, err)
	}
	if ok, _ := p.IsSubIssue(context.Background(), Ref{URL: "https://github.com/acme/app/issues/8"}, child); ok {
		t.Fatal("a different parent is not this parent")
	}
	refs, err := p.BlockedByRefs(context.Background(), child)
	if err != nil || len(refs) != 1 || refs[0].URL != "https://github.com/acme/app/issues/5" {
		t.Fatalf("blocked by = %+v %v", refs, err)
	}
	var _ SliceProjector = p
}

func TestSync_SliceFailuresAreReportedPerItemSoTheCLIcanFail(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/changes/big/proposal.md", "# Big\n\nwhy\n")
	mustWrite(t, root+"/changes/big/tasks.md", projectedTasks)
	p := &sliceProjProvider{fakeEpicProvider: newFakeEpicProvider("acme/app"), failLinks: 1}
	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: p, Slug: "big", Slices: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || len(res.Items[0].SliceFailures) != 1 {
		t.Fatalf("a failed step must be listed on the item: %+v", res.Items)
	}
}
