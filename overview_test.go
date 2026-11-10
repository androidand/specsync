package specsync

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type epicWorkspace struct {
	root   string
	opts   OverviewOptions
	spikeW string
}

func writeChange(t *testing.T, clone, slug, repo, id, links string) {
	t.Helper()
	dir := filepath.Join(clone, "openspec", "changes", slug)
	mustWrite(t, filepath.Join(dir, "proposal.md"), "# "+slug+"\n")
	mustWrite(t, filepath.Join(dir, ".specsync", "refs.json"),
		`{"github:`+repo+`":{"provider":"github:`+repo+`","id":"`+id+`","url":"https://github.com/`+repo+`/issues/`+id+`"}}`)
	if links != "" {
		mustWrite(t, filepath.Join(dir, "links.md"), links)
	}
}

func newEpicWorkspace(t *testing.T) *epicWorkspace {
	t.Helper()
	root := t.TempDir()
	api, web, docs, worker, bare := filepath.Join(root, "api"), filepath.Join(root, "web"), filepath.Join(root, "docs"), filepath.Join(root, "worker"), filepath.Join(root, "bare")
	gitRepo(t, api, "https://github.com/acme/api.git")
	gitRepo(t, web, "https://github.com/acme/web.git")
	gitRepo(t, docs, "https://github.com/acme/docs.git")
	gitRepo(t, worker, "https://github.com/acme/worker.git")
	gitRepo(t, bare, "")

	epic := "## Related\n- https://github.com/acme/web/issues/1\n"
	writeChange(t, api, "scope-list", "acme/api", "12", epic+"\n## Blocked by\n- acme/web#2\n")
	writeChange(t, web, "company-scope", "acme/web", "2", epic)
	writeChange(t, web, "picker-ui", "acme/web", "5", epic)
	writeChange(t, docs, "write-guide", "acme/docs", "3", "")

	spike := filepath.Join(t.TempDir(), "spike")
	gitIn2(t, api, "worktree", "add", "-q", "-b", "spike-picker", spike)
	hotfix := filepath.Join(t.TempDir(), "hotfix")
	gitIn2(t, worker, "worktree", "add", "-q", "-b", "hotfix", hotfix)

	requests := map[string]*ChangeRequestSet{
		"acme/api": {Requests: []ChangeRequest{{Number: 40, State: "open", Branch: "spike-picker", Closes: []string{"12"}, Author: "me",
			ReviewDecision: "review_required", Checks: ChangeRequestChecks{Pass: 9}}}},
		"acme/web":    {Requests: []ChangeRequest{{Number: 41, State: "merged", Branch: "company-scope-work", Closes: []string{"2"}, Author: "me"}}},
		"acme/worker": {Requests: []ChangeRequest{{Number: 31, State: "open", Branch: "hotfix", Author: "someone"}}},
	}
	run := func(_ context.Context, args ...string) (string, error) {
		if len(args) > 2 && args[0] == "issue" && args[1] == "view" {
			for i, a := range args {
				if a == "--json" && args[i+1] == "state" {
					return "OPEN", nil
				}
				if a == "--json" && args[i+1] == "labels" {
					if args[2] == "1" {
						return "type:epic", nil
					}
					return "", nil
				}
			}
		}
		return "", errors.New("unexpected gh call: " + strings.Join(args, " "))
	}
	return &epicWorkspace{root: root, spikeW: spike, opts: OverviewOptions{
		Workspace: WorkspaceOptions{Roots: []string{root}},
		Topology: TopologyOptions{
			run: run,
			changeRequests: func(_ context.Context, repo string) (*ChangeRequestSet, error) {
				if set, ok := requests[repo]; ok {
					return set, nil
				}
				return &ChangeRequestSet{}, nil
			},
		},
		viewer: func(context.Context) (string, error) { return "me", nil },
	}}
}

func keysOf(rows []ChangeTopology) []string {
	var out []string
	for _, r := range rows {
		out = append(out, RowKey(r))
	}
	return out
}

func TestBuildOverview_GroupsByEpicThenNoEpicThenUnlinked(t *testing.T) {
	w := newEpicWorkspace(t)
	ov, err := BuildOverview(context.Background(), w.opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Epics) != 1 || ov.Epics[0].Epic != "acme/web#1" {
		t.Fatalf("epics = %+v", ov.Epics)
	}
	g := ov.Epics[0]
	if want := []string{"acme/api#12", "acme/web#2", "acme/web#5"}; !reflect.DeepEqual(keysOf(g.Items), want) || g.Total != 3 || g.Done != 1 {
		t.Fatalf("epic items = %v done %d of %d", keysOf(g.Items), g.Done, g.Total)
	}
	if want := []string{"acme/docs#3"}; !reflect.DeepEqual(keysOf(ov.NoEpic), want) {
		t.Fatalf("no-epic = %v", keysOf(ov.NoEpic))
	}
	var unlinked []string
	for _, r := range ov.Unlinked {
		unlinked = append(unlinked, r.Repo+":"+r.Branch)
		if !r.NoChange {
			t.Fatal("unlinked rows are marked NoChange")
		}
	}
	if want := []string{"acme/worker:hotfix"}; !reflect.DeepEqual(unlinked, want) {
		t.Fatalf("unlinked = %v", unlinked)
	}
	scope := g.Items[0]
	if scope.Branch != "spike-picker" || scope.Worktree == "" || scope.PR == nil || scope.PR.Number != 40 {
		t.Fatalf("a branch unrelated to the change must resolve through the closing request: %+v", scope)
	}
	var sawBare bool
	for _, s := range ov.Status {
		sawBare = sawBare || strings.Contains(s, "bare") && strings.Contains(s, "no origin")
	}
	if !sawBare || !ov.Incomplete {
		t.Fatalf("a clone with no origin is named and the result marked incomplete: %v", ov.Status)
	}
}

func TestBuildOverview_ScopesCombine(t *testing.T) {
	w := newEpicWorkspace(t)
	run := func(scope OverviewScope) *Overview {
		o := w.opts
		o.Scope = scope
		ov, err := BuildOverview(context.Background(), o)
		if err != nil {
			t.Fatal(err)
		}
		return ov
	}
	all := func(ov *Overview) []string {
		var keys []string
		for _, g := range ov.Epics {
			keys = append(keys, keysOf(g.Items)...)
		}
		return append(append(keys, keysOf(ov.NoEpic)...), keysOf(ov.Unlinked)...)
	}
	if got := all(run(OverviewScope{Repo: "web"})); !reflect.DeepEqual(got, []string{"acme/web#2", "acme/web#5"}) {
		t.Fatalf("repo: %v", got)
	}
	if got := all(run(OverviewScope{Epic: "#1", Repo: "acme/api"})); !reflect.DeepEqual(got, []string{"acme/api#12"}) {
		t.Fatalf("epic and repo combine: %v", got)
	}
	if got := all(run(OverviewScope{Unattached: true})); !reflect.DeepEqual(got, []string{"acme/docs#3", "acme/worker:hotfix"}) {
		t.Fatalf("unattached: %v", got)
	}
	if got := all(run(OverviewScope{Mine: true})); !reflect.DeepEqual(got, []string{"acme/api#12", "acme/web#2"}) {
		t.Fatalf("mine keeps my requests and local work without one: %v", got)
	}
	if got := all(run(OverviewScope{Stage: "active", Repo: "docs"})); !reflect.DeepEqual(got, []string{"acme/docs#3"}) {
		t.Fatalf("stage: %v", got)
	}
	ov := run(OverviewScope{Repo: "api", Attention: true})
	if want := []string{"repo api", "needs attention"}; !reflect.DeepEqual(ov.Scope, want) {
		t.Fatalf("the applied scope is reported: %v", ov.Scope)
	}
	for _, k := range all(ov) {
		if !strings.HasPrefix(k, "acme/api") {
			t.Fatalf("attention in one repo leaked %s", k)
		}
	}
}

func TestBuildOverview_ProviderDownMarksIncompleteButKeepsLocalFacts(t *testing.T) {
	w := newEpicWorkspace(t)
	w.opts.Topology.changeRequests = func(context.Context, string) (*ChangeRequestSet, error) {
		return nil, errors.New("provider unreachable")
	}
	ov, err := BuildOverview(context.Background(), w.opts)
	if err != nil {
		t.Fatal(err)
	}
	if !ov.Incomplete || ov.RequestState != "omitted" {
		t.Fatalf("incomplete=%v requestState=%q", ov.Incomplete, ov.RequestState)
	}
	var reasons int
	for _, s := range ov.Status {
		if strings.Contains(s, "provider unreachable") {
			reasons++
		}
	}
	if reasons == 0 {
		t.Fatalf("the reason must be given: %v", ov.Status)
	}
	if len(ov.Epics) == 0 || len(ov.NoEpic) == 0 {
		t.Fatalf("local rows are still shown: %+v", ov)
	}
}

func TestBuildOverview_LocalOnlyAndNoViewerStillWork(t *testing.T) {
	w := newEpicWorkspace(t)
	w.opts.Topology.LocalOnly = true
	w.opts.Scope = OverviewScope{Mine: true}
	w.opts.viewer = func(context.Context) (string, error) { return "", errors.New("no gh") }
	ov, err := BuildOverview(context.Background(), w.opts)
	if err != nil {
		t.Fatal(err)
	}
	var sawViewer bool
	for _, s := range ov.Status {
		sawViewer = sawViewer || strings.Contains(s, "viewer unknown")
	}
	if !sawViewer {
		t.Fatalf("status = %v", ov.Status)
	}
}

func TestRelatedTo_EpicSiblingsBlockersAndAlone(t *testing.T) {
	w := newEpicWorkspace(t)
	work, err := CollectWorkRows(context.Background(), w.opts)
	if err != nil {
		t.Fatal(err)
	}
	var cur ChangeTopology
	for _, r := range work.Rows {
		if RowKey(r) == "acme/api#12" {
			cur = r
		}
	}
	rel := RelatedTo(cur, work.Rows, nil)
	if rel == nil || rel.Epic == nil || rel.Epic.Key != "acme/web#1" {
		t.Fatalf("rel = %+v", rel)
	}
	if rel.Epic.Total != 3 || rel.Epic.Done != 1 {
		t.Fatalf("all three siblings listed with the current one among them: %+v", rel.Epic)
	}
	if len(rel.BlockedBy) != 1 || rel.BlockedBy[0].Key != "acme/web#2" || rel.BlockedBy[0].Row == nil {
		t.Fatalf("blocked by: %+v", rel.BlockedBy)
	}
	for _, r := range work.Rows {
		if RowKey(r) == "acme/docs#3" {
			if RelatedTo(r, work.Rows, nil) != nil {
				t.Fatal("work with no relations is shown alone")
			}
		}
	}
	missing := cur
	missing.BlockedBy = []string{"acme/ghost#9"}
	swapped := append([]ChangeTopology(nil), work.Rows...)
	for i := range swapped {
		if RowKey(swapped[i]) == "acme/api#12" {
			swapped[i] = missing
		}
	}
	if rel := RelatedTo(missing, swapped, []string{"provider unreachable"}); len(rel.BlockedBy) != 1 || rel.BlockedBy[0].Row != nil || rel.Incomplete[0] != "provider unreachable" {
		t.Fatalf("an item outside the workspace is kept by key and the incompleteness carried: %+v", rel)
	}
}
