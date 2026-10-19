package specsync

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func fakeGHJSON(out string, calls *[][]string) func(context.Context, ...string) (string, error) {
	return func(_ context.Context, args ...string) (string, error) {
		if calls != nil {
			*calls = append(*calls, args)
		}
		if len(args) > 0 && args[0] == "api" {
			return "", errors.New("graphql not stubbed")
		}
		return out, nil
	}
}

const threadsFixture = `{"data":{"repository":{"pullRequests":{"nodes":[
 {"number":7,"reviewThreads":{"nodes":[{"isResolved":false},{"isResolved":true},{"isResolved":false}]}},
 {"number":9,"reviewThreads":{"nodes":[{"isResolved":true}]}}
]}}}}`

const prListFixture = `[
 {"number": 7, "url": "https://github.com/acme/portal/pull/7", "state": "OPEN", "isDraft": false,
  "headRefName": "feat/4231-add-field-forms", "baseRefName": "main", "headRefOid": "abc111", "isCrossRepository": false,
  "reviewDecision": "APPROVED", "mergeable": "MERGEABLE",
  "additions": 120, "deletions": 8, "changedFiles": 5,
  "commits": [{}, {}, {}],
  "reviews": [{"state": "APPROVED"}], "comments": [],
  "closingIssuesReferences": [{"number": 4231}],
  "statusCheckRollup": [
    {"__typename": "CheckRun", "name": "build", "status": "COMPLETED", "conclusion": "SUCCESS"},
    {"__typename": "CheckRun", "name": "lint", "status": "COMPLETED", "conclusion": "SKIPPED"},
    {"__typename": "StatusContext", "context": "ci/legacy", "state": "SUCCESS"}
  ]},
 {"number": 9, "url": "https://github.com/acme/portal/pull/9", "state": "OPEN", "isDraft": true,
  "headRefName": "feat/9-failing", "baseRefName": "main", "isCrossRepository": true,
  "reviewDecision": "", "mergeable": "UNKNOWN",
  "additions": 1, "deletions": 0, "changedFiles": 1, "commits": [{}],
  "reviews": [], "comments": [{"body": "hm"}],
  "closingIssuesReferences": [],
  "statusCheckRollup": [
    {"__typename": "CheckRun", "name": "unit", "status": "COMPLETED", "conclusion": "FAILURE"},
    {"__typename": "CheckRun", "name": "e2e", "status": "IN_PROGRESS", "conclusion": ""},
    {"__typename": "StatusContext", "context": "ci/legacy", "state": "ERROR"},
    {"__typename": "StatusContext", "context": "ci/slow", "state": "PENDING"}
  ]},
 {"number": 3, "url": "https://github.com/acme/portal/pull/3", "state": "MERGED", "isDraft": false,
  "headRefName": "feat/3-old", "baseRefName": "main", "reviewDecision": "APPROVED", "mergeable": "UNKNOWN",
  "additions": 2, "deletions": 2, "changedFiles": 1, "commits": [{}], "reviews": [{}], "comments": [],
  "closingIssuesReferences": [], "statusCheckRollup": []}
]`

func TestGitHubChangeRequests_AskesOnceForAllStatesAndMapsEveryField(t *testing.T) {
	var calls [][]string
	p := NewGitHubProviderFuncWithRepo("acme/portal", fakeGHJSON(prListFixture, &calls))
	crs, err := p.ChangeRequests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	prCalls := 0
	for _, c := range calls {
		if c[0] == "pr" {
			prCalls++
		}
	}
	if prCalls != 1 {
		t.Fatalf("expected one pr listing per repo, got %d: %v", prCalls, calls)
	}
	joined := strings.Join(calls[0], " ")
	if strings.Contains(joined, "commits") {
		t.Errorf("commits is left out of the query: it pushes GitHub's GraphQL node limit down to ~40 requests per page: %s", joined)
	}
	for _, want := range []string{"pr list", "--repo acme/portal", "--state all", "--json", "statusCheckRollup", "--limit"} {
		if !strings.Contains(joined, want) {
			t.Errorf("gh args %q missing %q", joined, want)
		}
	}

	byNum := map[int]ChangeRequest{}
	for _, c := range crs.Requests {
		byNum[c.Number] = c
	}
	open := byNum[7]
	if open.State != "open" || open.Draft || open.Branch != "feat/4231-add-field-forms" || open.Base != "main" ||
		open.ReviewDecision != "approved" || open.Mergeable != "mergeable" ||
		open.URL != "https://github.com/acme/portal/pull/7" {
		t.Fatalf("open PR mapped wrong: %+v", open)
	}
	if open.Checks.Pass != 3 || open.Checks.Fail != 0 || open.Checks.Pending != 0 {
		t.Fatalf("checks = %+v, want 3 pass", open.Checks)
	}
	if want := (ChangeRequestSize{Files: 5, Additions: 120, Deletions: 8}); open.Size != want {
		t.Fatalf("size = %+v, want %+v", open.Size, want)
	}
	if !open.ReviewStarted || !reflect.DeepEqual(open.Closes, []string{"4231"}) {
		t.Fatalf("reviewStarted/closes = %v/%v", open.ReviewStarted, open.Closes)
	}

	failing := byNum[9]
	if !failing.Draft || failing.ReviewDecision != "" || failing.Mergeable != "unknown" {
		t.Fatalf("draft PR mapped wrong: %+v", failing)
	}
	if failing.Checks.Pass != 0 || failing.Checks.Fail != 2 || failing.Checks.Pending != 2 {
		t.Fatalf("checks = %+v, want 0 pass 2 fail 2 pending", failing.Checks)
	}
	if !reflect.DeepEqual(failing.Checks.Failing, []string{"ci/legacy", "unit"}) {
		t.Fatalf("failing names = %v, want sorted [ci/legacy unit]", failing.Checks.Failing)
	}
	if !failing.ReviewStarted {
		t.Fatal("a comment counts as review started")
	}
	if byNum[3].State != "merged" {
		t.Fatalf("merged PR state = %q", byNum[3].State)
	}
}

func TestGitHubChangeRequests_TruncationIsReported(t *testing.T) {
	p := NewGitHubProviderFuncWithRepo("acme/portal", fakeGHJSON(prListFixture, nil))
	p.changeRequestLimit = 3 // the fixture has exactly three
	crs, err := p.ChangeRequests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !crs.Truncated {
		t.Fatal("a full page means the listing may be truncated")
	}
	p.changeRequestLimit = 50
	if crs, _ = p.ChangeRequests(context.Background()); crs.Truncated {
		t.Fatal("a short page is complete")
	}
}

func TestGitHubChangeRequests_ErrorsPropagate(t *testing.T) {
	p := NewGitHubProviderFuncWithRepo("acme/portal", func(context.Context, ...string) (string, error) {
		return "", errors.New("gh: not logged in")
	})
	if _, err := p.ChangeRequests(context.Background()); err == nil {
		t.Fatal("expected the gh error")
	}
	p = NewGitHubProviderFuncWithRepo("acme/portal", fakeGHJSON("not json", nil))
	if _, err := p.ChangeRequests(context.Background()); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestChangeRequestsByBranch_PrefersOpenThenNewest(t *testing.T) {
	set := ChangeRequestSet{Requests: []ChangeRequest{
		{Number: 1, Branch: "feat/x", State: "closed"},
		{Number: 5, Branch: "feat/x", State: "merged"},
		{Number: 2, Branch: "feat/x", State: "open"},
		{Number: 4, Branch: "feat/y", State: "closed"},
		{Number: 6, Branch: "feat/y", State: "closed"},
	}}
	if got := set.ForBranch("feat/x"); got == nil || got.Number != 2 {
		t.Fatalf("open PR should win: %+v", got)
	}
	if got := set.ForBranch("feat/y"); got == nil || got.Number != 6 {
		t.Fatalf("newest should win among non-open: %+v", got)
	}
	if set.ForBranch("feat/none") != nil {
		t.Fatal("no PR for an unknown branch")
	}
}

func TestRowAttention_RequestFlags(t *testing.T) {
	clean := &LocalState{Upstream: "origin/x"}
	open := func(f func(*ChangeRequest)) *ChangeRequest {
		cr := &ChangeRequest{State: "open", ReviewDecision: "approved", Mergeable: "mergeable", ReviewStarted: true}
		if f != nil {
			f(cr)
		}
		return cr
	}
	cases := []struct {
		name string
		ls   *LocalState
		cr   *ChangeRequest
		want []string
	}{
		{"ready to merge", clean, open(nil), []string{"ready-to-merge"}},
		{"ci failing blocks ready", clean, open(func(c *ChangeRequest) { c.Checks.Fail = 1 }), []string{"ci-failing"}},
		{"pending checks block ready", clean, open(func(c *ChangeRequest) { c.Checks.Pending = 1 }), nil},
		{"changes requested", clean, open(func(c *ChangeRequest) { c.ReviewDecision = "changes_requested" }), []string{"changes-requested"}},
		{"review missing", clean, open(func(c *ChangeRequest) { c.ReviewDecision = ""; c.ReviewStarted = false }), []string{"review-missing"}},
		{"draft is not missing a review", clean, open(func(c *ChangeRequest) { c.ReviewDecision = ""; c.ReviewStarted = false; c.Draft = true }), nil},
		{"conflicting is not ready", clean, open(func(c *ChangeRequest) { c.Mergeable = "conflicting" }), nil},
		{"merged and clean means cleanup", clean, &ChangeRequest{State: "merged"}, []string{"cleanup"}},
		{"merged but dirty is not cleanup", &LocalState{Upstream: "origin/x", Modified: 1}, &ChangeRequest{State: "merged"}, []string{"uncommitted"}},
		{"merged with no worktree is not cleanup", nil, &ChangeRequest{State: "merged"}, nil},
		{"closed PR raises nothing", clean, &ChangeRequest{State: "closed", Checks: ChangeRequestChecks{Fail: 1}}, nil},
		{"order follows the vocabulary", &LocalState{Upstream: "origin/x", Ahead: 1, BehindBase: 1, VolatilePath: true},
			open(func(c *ChangeRequest) { c.Checks.Fail = 1 }), []string{"unpushed", "behind-base", "ci-failing", "volatile-path"}},
		{"no request, local flags only", &LocalState{Ahead: 2, Upstream: "origin/x"}, nil, []string{"unpushed"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RowAttention(c.ls, c.cr)
			if !reflect.DeepEqual(got, c.want) && !(len(got) == 0 && len(c.want) == 0) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestGitHubChangeRequests_UnresolvedThreadsFromGraphQL(t *testing.T) {
	var calls [][]string
	p := NewGitHubProviderFuncWithRepo("acme/portal", func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		if args[0] == "api" {
			return threadsFixture, nil
		}
		return prListFixture, nil
	})
	set, err := p.ChangeRequests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	by := map[int]ChangeRequest{}
	for _, c := range set.Requests {
		by[c.Number] = c
	}
	if got := by[7].UnresolvedThreads; got == nil || *got != 2 {
		t.Fatalf("PR 7 unresolved = %v, want 2", got)
	}
	if got := by[9].UnresolvedThreads; got == nil || *got != 0 {
		t.Fatalf("PR 9 unresolved = %v, want 0", got)
	}
	if by[3].UnresolvedThreads != nil {
		t.Fatal("a merged PR has no thread count")
	}
	var api []string
	for _, c := range calls {
		if c[0] == "api" {
			api = c
		}
	}
	joined := strings.Join(api, " ")
	for _, want := range []string{"api graphql", "owner=acme", "name=portal", "reviewThreads"} {
		if !strings.Contains(joined, want) {
			t.Errorf("graphql call %q missing %q", joined, want)
		}
	}
}

func TestGitHubChangeRequests_ThreadLookupFailureIsNotAnError(t *testing.T) {
	p := NewGitHubProviderFuncWithRepo("acme/portal", fakeGHJSON(prListFixture, nil))
	set, err := p.ChangeRequests(context.Background())
	if err != nil {
		t.Fatalf("a failed thread lookup must not fail the listing: %v", err)
	}
	for _, c := range set.Requests {
		if c.UnresolvedThreads != nil {
			t.Fatalf("unknown stays unknown, got %v for #%d", *c.UnresolvedThreads, c.Number)
		}
	}
}

func TestGitHubChangeRequests_NoOpenRequestsSkipsTheThreadLookup(t *testing.T) {
	var calls [][]string
	p := NewGitHubProviderFuncWithRepo("acme/portal", fakeGHJSON(`[{"number":3,"state":"MERGED","headRefName":"x"}]`, &calls))
	if _, err := p.ChangeRequests(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, c := range calls {
		if c[0] == "api" {
			t.Fatal("no open request, so no graphql call")
		}
	}
}

func TestRowAttention_UnresolvedComments(t *testing.T) {
	two := 2
	zero := 0
	open := func(n *int) *ChangeRequest {
		return &ChangeRequest{State: "open", ReviewStarted: true, UnresolvedThreads: n}
	}
	if got := RowAttention(nil, open(&two)); !reflect.DeepEqual(got, []string{"unresolved-comments"}) {
		t.Fatalf("got %v", got)
	}
	for _, n := range []*int{&zero, nil} {
		if got := RowAttention(nil, open(n)); len(got) != 0 {
			t.Fatalf("no unresolved threads raises nothing, got %v", got)
		}
	}
	// approved + clean + unresolved threads is not ready to merge
	cr := &ChangeRequest{State: "open", ReviewDecision: "approved", Mergeable: "mergeable", ReviewStarted: true, UnresolvedThreads: &two}
	for _, f := range RowAttention(nil, cr) {
		if f == "ready-to-merge" {
			t.Fatal("unresolved threads block ready-to-merge")
		}
	}
}

func TestGitHubChangeRequests_MapsHeadSHAAndForkFlag(t *testing.T) {
	p := NewGitHubProviderFuncWithRepo("acme/portal", fakeGHJSON(prListFixture, nil))
	set, err := p.ChangeRequests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	by := map[int]ChangeRequest{}
	for _, c := range set.Requests {
		by[c.Number] = c
	}
	if by[7].HeadSHA != "abc111" || by[7].Fork {
		t.Fatalf("PR 7 = %+v", by[7])
	}
	if !by[9].Fork {
		t.Fatalf("PR 9 is from a fork: %+v", by[9])
	}
}

func TestChangeRequestsByBranch_IgnoresRequestsFromForks(t *testing.T) {
	set := ChangeRequestSet{Requests: []ChangeRequest{
		{Number: 8, Branch: "fix", State: "open", Fork: true},
		{Number: 2, Branch: "fix", State: "merged"},
		{Number: 9, Branch: "main", State: "open", Fork: true},
	}}
	if got := set.ForBranch("fix"); got == nil || got.Number != 2 {
		t.Fatalf("a fork's PR must not win over the repo's own: %+v", got)
	}
	if got := set.ForBranch("main"); got != nil {
		t.Fatalf("a fork's head branch name says nothing about a local branch: %+v", got)
	}
}

func TestRowAttention_CleanupNeedsTheMergedCommit(t *testing.T) {
	merged := func(sha string) *ChangeRequest { return &ChangeRequest{State: "merged", HeadSHA: sha} }
	clean := func(head string) *LocalState { return &LocalState{Upstream: "origin/x", Head: head} }
	cases := []struct {
		name string
		ls   *LocalState
		cr   *ChangeRequest
		want bool
	}{
		{"same commit", clean("aaa"), merged("aaa"), true},
		{"new commits since the merge", clean("bbb"), merged("aaa"), false},
		{"request head unknown", clean("bbb"), merged(""), true},
		{"local head unknown", clean(""), merged("aaa"), true},
	}
	for _, c := range cases {
		got := false
		for _, f := range RowAttention(c.ls, c.cr) {
			got = got || f == "cleanup"
		}
		if got != c.want {
			t.Errorf("%s: cleanup = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGitHubChangeRequests_FullPageAlsoFetchesOpenRequests(t *testing.T) {
	const open = `[
 {"number": 7, "state": "OPEN", "headRefName": "feat/4231-add-field-forms"},
 {"number": 11, "state": "OPEN", "headRefName": "feat/old-but-open"}
]`
	var calls [][]string
	p := NewGitHubProviderFuncWithRepo("acme/portal", func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "api":
			return "", errors.New("no graphql")
		case strings.Contains(strings.Join(args, " "), "--state open"):
			return open, nil
		}
		return prListFixture, nil
	})
	p.changeRequestLimit = 3 // the first page is full, so older open requests may be missing
	set, err := p.ChangeRequests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	count := map[int]int{}
	for _, c := range set.Requests {
		count[c.Number]++
	}
	if count[11] != 1 || count[7] != 1 {
		t.Fatalf("expected #11 added and #7 not duplicated, got %v", count)
	}
	if set.Truncated {
		t.Fatal("the open listing was not full, so nothing open is missing")
	}

	// A full open listing is still truncated.
	p.changeRequestLimit = 2
	if set, _ = p.ChangeRequests(context.Background()); !set.Truncated {
		t.Fatal("a full open page may still miss open requests")
	}
}

func TestGitHubChangeRequests_ShortPageNeedsNoSecondListing(t *testing.T) {
	var calls [][]string
	p := NewGitHubProviderFuncWithRepo("acme/portal", fakeGHJSON(prListFixture, &calls))
	if _, err := p.ChangeRequests(context.Background()); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range calls {
		if c[0] == "pr" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("a short page is complete; got %d listings", n)
	}
}
