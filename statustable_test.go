package specsync

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func two(n int) *int { return &n }

func tableRow(issue string, title string, pr *ChangeRequest) ChangeTopology {
	return ChangeTopology{
		Slug: "s-" + issue, Title: title, Repo: "acme/portal", Branch: "feat/" + issue + "-x",
		Issue: &TopologyIssue{Provider: "github:acme/portal", ID: issue},
		PR:    pr,
	}
}

func TestStatusTableCells(t *testing.T) {
	open := func(mod func(*ChangeRequest)) *ChangeRequest {
		pr := &ChangeRequest{Number: 7, State: "open", ReviewersKnown: true, Mergeable: "mergeable", Checks: ChangeRequestChecks{Pass: 3},
			Size: ChangeRequestSize{Files: 4, Additions: 50, Deletions: 6}}
		if mod != nil {
			mod(pr)
		}
		return pr
	}
	cases := []struct {
		name string
		row  ChangeTopology
		want []string // Issue, What, PR, State, Checks, Review, Size
	}{
		{"approved", tableRow("42", "Add thing", open(func(p *ChangeRequest) {
			p.ReviewDecision = "approved"
			p.Reviews = []ChangeRequestReview{{"ann", "approved"}}
		})), []string{"#42", "Add thing", "#7", "Open", "all 3 pass", "Approved by ann", "4 files, +50 -6"}},
		{"requested", tableRow("42", "Add thing", open(func(p *ChangeRequest) {
			p.ReviewDecision = "review_required"
			p.ReviewRequests = []string{"bob", "team:core"}
		})), []string{"#42", "Add thing", "#7", "Open", "all 3 pass", "Needs review; requested from bob, team:core", "4 files, +50 -6"}},
		{"changes", tableRow("42", "Add thing", open(func(p *ChangeRequest) {
			p.ReviewDecision = "changes_requested"
			p.Reviews = []ChangeRequestReview{{"cy", "changes_requested"}}
			p.UnresolvedThreads = two(2)
		})), []string{"#42", "Add thing", "#7", "Open", "all 3 pass", "Changes requested by cy; 2 unresolved", "4 files, +50 -6"}},
		{"failing", tableRow("42", "Add thing", open(func(p *ChangeRequest) {
			p.Checks = ChangeRequestChecks{Pass: 2, Fail: 1, Pending: 1, Failing: []string{"types"}}
		})), []string{"#42", "Add thing", "#7", "Open", "2 pass, 1 failing (types), 1 pending", "No review", "4 files, +50 -6"}},
		{"no checks", tableRow("42", "Add thing", open(func(p *ChangeRequest) { p.Checks = ChangeRequestChecks{} })),
			[]string{"#42", "Add thing", "#7", "Open", "none", "No review", "4 files, +50 -6"}},
		{"approved without a review rule", tableRow("42", "Add thing", open(func(p *ChangeRequest) {
			p.Reviews = []ChangeRequestReview{{"ann", "approved"}}
		})), []string{"#42", "Add thing", "#7", "Open", "all 3 pass", "Approved by ann", "4 files, +50 -6"}},
		{"merged", tableRow("42", "Add thing", &ChangeRequest{Number: 7, State: "merged"}),
			[]string{"#42", "Add thing", "#7", "Merged", "–", "–", "–"}},
		{"closed issue", func() ChangeTopology {
			r := tableRow("42", "Add thing", &ChangeRequest{Number: 7, State: "merged", Closes: []string{"9"}})
			r.Issue.State = "CLOSED"
			return r
		}(), []string{"#42 closed + #9", "Add thing", "#7", "Merged", "–", "–", "–"}},
		{"open issue stays unmarked", func() ChangeTopology {
			r := tableRow("42", "Add thing", nil)
			r.Issue.State = "open"
			return r
		}(), []string{"#42", "Add thing", "Not opened", "Local", "–", "–", "–"}},
		{"draft", tableRow("42", "Add thing", &ChangeRequest{Number: 7, State: "open", Draft: true, ReviewersKnown: true}),
			[]string{"#42", "Add thing", "#7", "Draft", "none", "No review", "–"}},
		{"not opened", func() ChangeTopology {
			r := tableRow("42", "Add thing", nil)
			r.Local = &LocalState{Upstream: "origin/x", Ahead: 2, Size: &BranchSize{Files: 3, Additions: 10, Deletions: 1, Commits: 2}}
			return r
		}(), []string{"#42", "Add thing", "Not opened", "Local, 2 unpushed", "–", "–", "3 files, +10 -1"}},
		{"title from the request when there is no change", ChangeTopology{Repo: "acme/portal", Branch: "feat/9-y", NoChange: true,
			PR: &ChangeRequest{Number: 9, State: "open", ReviewersKnown: true, Title: "chore: bump", Closes: []string{"9", "8"}}},
			[]string{"#8 + #9", "chore: bump", "#9", "Open", "none", "No review", "–"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := StatusTableRows([]ChangeTopology{c.row}, "")[0]
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("cells\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestStatusTableSeveralIssuesOnePR(t *testing.T) {
	pr := &ChangeRequest{Number: 26, State: "open", Closes: []string{"4015", "3800"}}
	row := tableRow("4015", "Job output", pr)
	got := StatusTableRows([]ChangeTopology{row}, "")[0][0]
	if got != "#3800 + #4015" && got != "#4015 + #3800" {
		t.Errorf("issue cell = %q, want both issues", got)
	}
}

func TestStatusTableNamesRepoWhenRowsSpanRepos(t *testing.T) {
	a := tableRow("1", "A", nil)
	b := tableRow("2", "B", nil)
	b.Repo = "acme/api"
	b.Issue.Provider = "github:acme/api"
	cells := StatusTableRows([]ChangeTopology{a, b}, RowKey(a))
	if cells[0][0] != "▶ portal#1" || cells[1][0] != "api#2" {
		t.Errorf("issue cells = %q, %q", cells[0][0], cells[1][0])
	}
}

func TestRenderStatusTableWrapsToWidth(t *testing.T) {
	row := tableRow("42", strings.Repeat("long words ", 12), &ChangeRequest{Number: 7, State: "open"})
	out := RenderStatusTable([]ChangeTopology{row}, "", 80, false)
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if n := len([]rune(l)); n > 80 {
			t.Fatalf("line is %d wide, want at most 80:\n%s", n, out)
		}
	}
	if !strings.Contains(out, "┌") || !strings.Contains(out, "└") {
		t.Errorf("not a box table:\n%s", out)
	}
}

func TestRelatedRowsDedupesAndSkipsUnresolved(t *testing.T) {
	cur := tableRow("1", "cur", nil)
	other := tableRow("2", "other", nil)
	rel := &RelatedWork{
		Epic:   &RelatedEpic{Items: []RelatedItem{{Key: RowKey(cur), Row: &cur}, {Key: RowKey(other), Row: &other}}},
		Linked: []RelatedItem{{Key: RowKey(other), Row: &other}, {Key: "acme/x#9"}},
	}
	rows := RelatedRows(cur, rel)
	if len(rows) != 2 || rows[0].Title != "cur" || rows[1].Title != "other" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestChangeRequestReviewFieldsRoundTrip(t *testing.T) {
	in := ChangeRequest{Number: 7, State: "open", Title: "t", ReviewRequests: []string{"bob"},
		Reviews: []ChangeRequestReview{{Reviewer: "ann", State: "approved"}}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out ChangeRequest
	if err := json.Unmarshal(b, &out); err != nil || out.Reviews[0].Reviewer != "ann" || out.ReviewRequests[0] != "bob" || out.Title != "t" {
		t.Fatalf("round trip lost fields: %+v, %v", out, err)
	}
	// A cache written before these fields existed still loads, with none set.
	var old ChangeRequest
	if err := json.Unmarshal([]byte(`{"number":7,"url":"u","state":"open","branch":"b","checks":{"pass":1,"fail":0,"pending":0},"size":{"files":1,"additions":1,"deletions":0}}`), &old); err != nil {
		t.Fatal(err)
	}
	if len(old.Reviews) != 0 || len(old.ReviewRequests) != 0 || old.Title != "" {
		t.Errorf("old cache entry gained fields: %+v", old)
	}
}

func linkedRow() ChangeTopology {
	r := tableRow("42", "Add thing", &ChangeRequest{Number: 7, State: "open", URL: "https://github.com/acme/portal/pull/7",
		Closes: []string{"42", "9"}, ClosesRefs: []string{"acme/portal#42", "acme/portal#9"}})
	r.Issue.URL = "https://github.com/acme/portal/issues/42"
	return r
}

func TestStatusTableURLsNameIssueAndRequest(t *testing.T) {
	got := StatusTableURLs([]ChangeTopology{linkedRow()})
	want := []string{
		"issue #42  https://github.com/acme/portal/issues/42",
		"issue #9   https://github.com/acme/portal/issues/9",
		"PR #7      https://github.com/acme/portal/pull/7",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("urls\n got %q\nwant %q", got, want)
	}
}

func TestRenderStatusTableHyperlinksOnlyWhenAsked(t *testing.T) {
	rows := []ChangeTopology{linkedRow()}
	plain := RenderStatusTable(rows, "", 0, false)
	if strings.Contains(plain, "\x1b") {
		t.Errorf("plain table has escapes:\n%q", plain)
	}
	linked := RenderStatusTable(rows, "", 0, true)
	for _, want := range []string{
		"\x1b]8;;https://github.com/acme/portal/pull/7\x1b\\#7\x1b]8;;\x1b\\",
		"\x1b]8;;https://github.com/acme/portal/issues/42\x1b\\#42\x1b]8;;\x1b\\",
		"\x1b]8;;https://github.com/acme/portal/issues/9\x1b\\#9\x1b]8;;\x1b\\",
	} {
		if !strings.Contains(linked, want) {
			t.Errorf("missing link %q in\n%q", want, linked)
		}
	}
	// Escapes take no width: every line still draws the same box.
	strip := func(s string) string {
		for strings.Contains(s, "\x1b]8;;") {
			i := strings.Index(s, "\x1b]8;;")
			j := strings.Index(s[i:], "\x1b\\") + i + 2
			s = s[:i] + s[j:]
		}
		return s
	}
	if strip(linked) != plain {
		t.Errorf("hyperlinks changed the layout:\n%s\nvs\n%s", strip(linked), plain)
	}
}

func TestLinkifyLeavesLongerNumbersAlone(t *testing.T) {
	got := linkify("#7 #70", []tableLink{{Label: "#7", URL: "u"}})
	if strings.Count(got, "\x1b]8;;u") != 1 || !strings.HasSuffix(got, " #70") {
		t.Errorf("linkify = %q", got)
	}
}

const oscOpen = "\x1b]8;;"

// linkTargets returns, per column, the URLs hyperlinked in that column of the
// first row of a rendered table.
func linkTargets(t *testing.T, row ChangeTopology) map[int][]string {
	t.Helper()
	rows := []ChangeTopology{row}
	cells := StatusTableRows(rows, "")
	links := statusTableLinks(rows)
	out := map[int][]string{}
	for col := range cells[0] {
		text := linkify(cells[0][col], cellLinks(&links[0], col))
		for rest := text; ; {
			i := strings.Index(rest, oscOpen)
			if i < 0 {
				break
			}
			rest = rest[i+len(oscOpen):]
			j := strings.Index(rest, "\x1b\\")
			if url := rest[:j]; url != "" { // the closing escape has an empty target
				out[col] = append(out[col], url)
			}
			rest = rest[j:]
		}
	}
	return out
}

// Regression: an issue and a request with the same number must each link to
// their own page, and only in their own column.
func TestLinksStaySeparatePerCellWhenNumbersMatch(t *testing.T) {
	row := tableRow("9", "Fix #9 and #7", &ChangeRequest{Number: 9, State: "open", URL: "https://github.com/acme/portal/pull/9",
		Closes: []string{"9"}, ClosesRefs: []string{"acme/portal#9"}})
	row.Issue.URL = "https://github.com/acme/portal/issues/9"
	got := linkTargets(t, row)
	if len(got) != 2 || len(got[0]) != 1 || got[0][0] != "https://github.com/acme/portal/issues/9" ||
		len(got[2]) != 1 || got[2][0] != "https://github.com/acme/portal/pull/9" {
		t.Errorf("links per column = %v, want the issue page in Issue and the pull page in PR only", got)
	}
}

func TestLinkifyOnlyLinksWholeNumbers(t *testing.T) {
	for _, text := range []string{"portal#7", "x#7", "a7", "#7a", "#77", "##7"} {
		if got := linkify(text, []tableLink{{Label: "#7", URL: "u"}}); got != text {
			t.Errorf("linkify(%q) = %q, want it left alone", text, got)
		}
	}
	if got := linkify("▶ #7", []tableLink{{Label: "#7", URL: "u"}}); !strings.Contains(got, oscOpen+"u") {
		t.Errorf("a number after the marker is not linked: %q", got)
	}
}

// Regression: a closed issue in another repository is neither merged into nor
// hidden by an issue with the same number here, and it names its repository.
func TestCrossRepoClosedIssueKeepsItsRepo(t *testing.T) {
	row := tableRow("5", "Add thing", &ChangeRequest{Number: 7, State: "open", URL: "https://github.com/acme/portal/pull/7",
		Closes: []string{"5"}, ClosesRefs: []string{"acme/api#5"}})
	row.Issue.URL = "https://github.com/acme/portal/issues/5"
	if got := StatusTableRows([]ChangeTopology{row}, "")[0][0]; got != "#5 + api#5" {
		t.Errorf("issue cell = %q, want %q", got, "#5 + api#5")
	}
	got := StatusTableURLs([]ChangeTopology{row})
	want := []string{
		"issue #5      https://github.com/acme/portal/issues/5",
		"api issue #5  https://github.com/acme/api/issues/5",
		"PR #7         https://github.com/acme/portal/pull/7",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("urls\n got %q\nwant %q", got, want)
	}
	// Same issue named both ways is one issue.
	row.Issue = &TopologyIssue{ID: "5"}
	row.PR.ClosesRefs = []string{"acme/portal#5"}
	if got := StatusTableRows([]ChangeTopology{row}, "")[0][0]; got != "#5" {
		t.Errorf("same issue twice: cell = %q, want #5", got)
	}
}

// Regression: a request with no state (an old cache, another provider) renders.
func TestTableStateSurvivesAnEmptyRequestState(t *testing.T) {
	row := tableRow("1", "x", &ChangeRequest{Number: 1})
	if got := StatusTableRows([]ChangeTopology{row}, "")[0][3]; got != "Unknown" {
		t.Errorf("state cell = %q, want Unknown", got)
	}
}

// The response of reviewersQuery: the repository holds one aliased field per
// request. A user or a bot request carries login, a team name and slug;
// latestOpinionatedReviews holds each reviewer's latest approval, change
// request or dismissal, never a comment. A request not found is null.
func TestParseReviewers(t *testing.T) {
	const raw = `{"data":{"repository":{
	  "pr5":{"number":5,
	   "reviewRequests":{"totalCount":4,"nodes":[
	     {"requestedReviewer":{"__typename":"User","login":"bob"}},
	     {"requestedReviewer":{"__typename":"Bot","login":"copilot-pull-request-reviewer"}},
	     {"requestedReviewer":{"__typename":"Team","name":"Core Devs","slug":"core-devs"}},
	     {"requestedReviewer":{}}]},
	   "latestOpinionatedReviews":{"totalCount":4,"nodes":[
	     {"author":{"login":"zed"},"state":"CHANGES_REQUESTED"},
	     {"author":{"login":"ann"},"state":"APPROVED"},
	     {"author":{"login":"di"},"state":"DISMISSED"},
	     {"author":null,"state":"APPROVED"}]}},
	  "pr6":{"number":6,"reviewRequests":{"totalCount":0,"nodes":[]},"latestOpinionatedReviews":{"totalCount":0,"nodes":[]}},
	  "pr9":null
	}}}`
	got, err := parseReviewers(raw)
	if err != nil {
		t.Fatal(err)
	}
	five := got[5]
	if strings.Join(five.Requested, ",") != "bob,copilot-pull-request-reviewer,team:core-devs" {
		t.Errorf("requested = %v (a bot request must not be lost)", five.Requested)
	}
	want := []ChangeRequestReview{{"ann", "approved"}, {"di", "dismissed"}, {"zed", "changes_requested"}}
	if len(five.Reviews) != 3 || five.Reviews[0] != want[0] || five.Reviews[1] != want[1] || five.Reviews[2] != want[2] {
		t.Errorf("reviews = %v, want %v", five.Reviews, want)
	}
	if five.Incomplete {
		t.Error("complete lists were marked incomplete")
	}
	if r, ok := got[6]; !ok || len(r.Requested) != 0 || len(r.Reviews) != 0 || r.Incomplete {
		t.Errorf("request 6 = %+v, %v", r, ok)
	}
	if _, ok := got[9]; ok {
		t.Error("a request that was not found is in the result")
	}
	if _, err := parseReviewers("not json"); err == nil {
		t.Error("a bad response parsed")
	}
}

// A list the provider cut short at its page size is said to be incomplete.
func TestParseReviewersFlagsCutShortLists(t *testing.T) {
	const raw = `{"data":{"repository":{"pr1":{"number":1,
	  "reviewRequests":{"totalCount":31,"nodes":[{"requestedReviewer":{"login":"bob"}}]},
	  "latestOpinionatedReviews":{"totalCount":1,"nodes":[{"author":{"login":"ann"},"state":"APPROVED"}]}}}}}`
	got, _ := parseReviewers(raw)
	if !got[1].Incomplete {
		t.Errorf("31 requests with 1 returned was not flagged: %+v", got[1])
	}
}

func TestReviewersQueryNamesRequestsByNumberAndAsksForBots(t *testing.T) {
	q := reviewersQuery([]int{40, 7})
	for _, want := range []string{"pr40:pullRequest(number:40)", "pr7:pullRequest(number:7)", "... on Bot{login}", "... on Team{name slug}", "latestOpinionatedReviews", "totalCount"} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %q:\n%s", want, q)
		}
	}
	if strings.Contains(q, "pullRequests(") {
		t.Error("the query lists pull requests instead of naming the ones shown")
	}
}

func TestGitHubReviewersBatchesCalls(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		return `{"data":{"repository":{"pr1":{"number":1,"reviewRequests":{"totalCount":0,"nodes":[]},"latestOpinionatedReviews":{"totalCount":1,"nodes":[{"author":{"login":"ann"},"state":"APPROVED"}]}}}}}`, nil
	}
	p := NewGitHubProviderFuncWithRepo("acme/portal", run)
	got, err := p.Reviewers(context.Background(), []int{1})
	if err != nil || len(calls) != 1 || calls[0][0] != "api" || calls[0][1] != "graphql" || got[1].Reviews[0].Reviewer != "ann" {
		t.Errorf("calls %v, result %+v, err %v", calls, got, err)
	}
	calls = nil
	many := make([]int, 85)
	for i := range many {
		many[i] = i + 1
	}
	if _, err := p.Reviewers(context.Background(), many); err != nil || len(calls) != 3 {
		t.Errorf("85 requests took %d calls (err %v), want 3 batches of at most %d", len(calls), err, reviewersBatch)
	}
}

// The real cause is returned, not "repo is not owner/name".
func TestGitHubReviewersReturnsTheResolveError(t *testing.T) {
	run := func(_ context.Context, args ...string) (string, error) { return "", errors.New("no git remote here") }
	_, err := NewGitHubProviderFuncWithRepo("", run).Reviewers(context.Background(), []int{1})
	if err == nil || !strings.Contains(err.Error(), "no git remote here") {
		t.Errorf("error = %v, want the resolver's cause", err)
	}
}

type fakeReviewers struct {
	by    map[int]RequestReviewers
	err   error
	calls *[][]int
}

func (f fakeReviewers) Reviewers(_ context.Context, numbers []int) (map[int]RequestReviewers, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, numbers)
	}
	return f.by, f.err
}

type memReviewerCache struct {
	m    map[string]map[int]RequestReviewers
	gets int
}

func (c *memReviewerCache) Get(repo string) (map[int]RequestReviewers, bool) {
	c.gets++
	v, ok := c.m[repo]
	return v, ok
}
func (c *memReviewerCache) Put(repo string, by map[int]RequestReviewers) {
	if c.m == nil {
		c.m = map[string]map[int]RequestReviewers{}
	}
	c.m[repo] = by
}

// Reviewers are fetched once per repository that has an open request among the
// rows given, naming exactly those requests; the requests are updated in place.
func TestAttachReviewersFollowsTheRowsShown(t *testing.T) {
	open := func(n int) *ChangeRequest { return &ChangeRequest{Number: n, State: "open"} }
	a1, a2, b1, merged := open(1), open(2), open(1), &ChangeRequest{Number: 3, State: "merged"}
	rows := []ChangeTopology{
		{Repo: "acme/portal", PR: a1}, {Repo: "Acme/Portal", PR: a2}, {Repo: "acme/api", PR: b1},
		{Repo: "acme/web", PR: merged}, {Repo: "acme/none"}, {Repo: "", PR: open(9)}, {Repo: "acme/portal", PR: a1},
	}
	var mu sync.Mutex
	asked := map[string][][]int{}
	source := func(repo string) ReviewerSource {
		var calls [][]int
		return recordingSource{fakeReviewers{calls: &calls, by: map[int]RequestReviewers{
			1: {Requested: []string{"bob"}, Reviews: []ChangeRequestReview{{"ann", "approved"}}}, 2: {}}},
			func(n []int) {
				mu.Lock()
				asked[strings.ToLower(repo)] = append(asked[strings.ToLower(repo)], n)
				mu.Unlock()
			}}
	}
	if notes := AttachReviewers(context.Background(), rows, ReviewerOptions{Source: source}); len(notes) != 0 {
		t.Errorf("notes = %v", notes)
	}
	if len(asked) != 2 || len(asked["acme/portal"]) != 1 || len(asked["acme/api"]) != 1 {
		t.Errorf("provider calls = %v, want one each for acme/portal and acme/api", asked)
	}
	if got := asked["acme/portal"][0]; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("acme/portal was asked for %v, want exactly requests 1 and 2", got)
	}
	if !a1.ReviewersKnown || a1.ReviewRequests[0] != "bob" || a1.Reviews[0].Reviewer != "ann" || !b1.ReviewersKnown || !a2.ReviewersKnown {
		t.Errorf("open requests not filled: %+v %+v %+v", a1, a2, b1)
	}
	if merged.ReviewersKnown {
		t.Error("a merged request was filled")
	}
}

type recordingSource struct {
	inner fakeReviewers
	on    func([]int)
}

func (r recordingSource) Reviewers(ctx context.Context, n []int) (map[int]RequestReviewers, error) {
	r.on(n)
	return r.inner.Reviewers(ctx, n)
}

func TestAttachReviewersFailureClaimsNothing(t *testing.T) {
	pr := &ChangeRequest{Number: 1, State: "open"}
	notes := AttachReviewers(context.Background(), []ChangeTopology{{Repo: "acme/portal", PR: pr}},
		ReviewerOptions{Source: func(string) ReviewerSource { return fakeReviewers{err: errors.New("boom")} }})
	if pr.ReviewersKnown || len(notes) != 1 || !strings.Contains(notes[0], "reviewers unavailable for acme/portal") {
		t.Errorf("known=%v notes=%v", pr.ReviewersKnown, notes)
	}
}

// A request the provider did not return, and a list it cut short, are said.
func TestAttachReviewersNotesWhatIsMissingOrIncomplete(t *testing.T) {
	missing, cut, fine := &ChangeRequest{Number: 1, State: "open"}, &ChangeRequest{Number: 2, State: "open"}, &ChangeRequest{Number: 3, State: "open"}
	rows := []ChangeTopology{{Repo: "acme/portal", PR: missing}, {Repo: "acme/portal", PR: cut}, {Repo: "acme/portal", PR: fine}}
	by := map[int]RequestReviewers{2: {Incomplete: true}, 3: {}}
	notes := AttachReviewers(context.Background(), rows, ReviewerOptions{Source: func(string) ReviewerSource { return fakeReviewers{by: by} }})
	want := []string{
		"reviewers unavailable for acme/portal#1: the provider did not return it",
		"reviewers of acme/portal#2 may be incomplete: the provider returned only part of the list",
	}
	if strings.Join(notes, "\n") != strings.Join(want, "\n") {
		t.Errorf("notes\n got %q\nwant %q", notes, want)
	}
	if missing.ReviewersKnown || !cut.ReviewersKnown || !fine.ReviewersKnown {
		t.Errorf("known: missing=%v cut=%v fine=%v", missing.ReviewersKnown, cut.ReviewersKnown, fine.ReviewersKnown)
	}
}

// Offline asks nobody; on reuses what is cached; fresh refetches and stores.
func TestAttachReviewersFollowsTheStateRules(t *testing.T) {
	var calls [][]int
	source := func(string) ReviewerSource {
		return fakeReviewers{calls: &calls, by: map[int]RequestReviewers{1: {Requested: []string{"bob"}}}}
	}
	row := func() ([]ChangeTopology, *ChangeRequest) {
		pr := &ChangeRequest{Number: 1, State: "open"}
		return []ChangeTopology{{Repo: "acme/portal", PR: pr}}, pr
	}
	cache := &memReviewerCache{}

	rows, pr := row()
	AttachReviewers(context.Background(), rows, ReviewerOptions{Source: source, Cache: cache, Offline: true})
	if len(calls) != 0 || cache.gets != 0 || pr.ReviewersKnown {
		t.Errorf("offline: calls=%d cache reads=%d known=%v", len(calls), cache.gets, pr.ReviewersKnown)
	}

	rows, pr = row()
	AttachReviewers(context.Background(), rows, ReviewerOptions{Source: source, Cache: cache})
	if len(calls) != 1 || !pr.ReviewersKnown || cache.m["acme/portal"][1].Requested[0] != "bob" {
		t.Errorf("on, empty cache: calls=%d known=%v cache=%v", len(calls), pr.ReviewersKnown, cache.m)
	}

	rows, pr = row()
	AttachReviewers(context.Background(), rows, ReviewerOptions{Source: source, Cache: cache})
	if len(calls) != 1 || !pr.ReviewersKnown || pr.ReviewRequests[0] != "bob" {
		t.Errorf("on, warm cache: calls=%d known=%v", len(calls), pr.ReviewersKnown)
	}

	rows, _ = row()
	AttachReviewers(context.Background(), rows, ReviewerOptions{Source: source, Cache: cache, Refresh: true})
	if len(calls) != 2 {
		t.Errorf("fresh: calls=%d, want a refetch", len(calls))
	}

	// A cache entry that lacks a request shown is a miss.
	other := &ChangeRequest{Number: 7, State: "open"}
	AttachReviewers(context.Background(), []ChangeTopology{{Repo: "acme/portal", PR: other}}, ReviewerOptions{Source: source, Cache: cache})
	if len(calls) != 3 {
		t.Errorf("a cache entry without request 7 counted as a hit: calls=%d", len(calls))
	}
}

// Reviewer data lives in its own cache. The listing cache clears it on the way
// in and out, so an entry written by an older binary shows nothing nobody
// fetched in this run.
func TestListingCacheDropsReviewerFields(t *testing.T) {
	c := &FileRequestCache{Dir: t.TempDir(), TTL: time.Minute}
	c.Put("acme/portal", ChangeRequestSet{Requests: []ChangeRequest{{Number: 1, State: "open", ReviewersKnown: true,
		ReviewRequests: []string{"bob"}, Reviews: []ChangeRequestReview{{"ann", "approved"}}}}})
	got, ok := c.Get("acme/portal")
	if !ok || got.Requests[0].ReviewersKnown || len(got.Requests[0].Reviews) != 0 || len(got.Requests[0].ReviewRequests) != 0 {
		t.Errorf("listing cache kept reviewer fields: %+v", got.Requests)
	}
	// An entry an older binary wrote, with the fields in it.
	old := `{"fetchedAt":"` + time.Now().Format(time.RFC3339) + `","requests":[{"number":1,"state":"open","reviewersKnown":true,"reviewRequests":["bob"],"reviews":[{"reviewer":"ann","state":"approved"}],"checks":{"pass":0,"fail":0,"pending":0},"size":{"files":0,"additions":0,"deletions":0}}]}`
	if err := os.WriteFile(c.path("acme/old"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok = c.Get("acme/old")
	if !ok || got.Requests[0].ReviewersKnown || len(got.Requests[0].Reviews) != 0 {
		t.Errorf("an old entry's reviewers survived: %+v", got.Requests)
	}
}

func TestFileReviewerCacheHonoursTheTTL(t *testing.T) {
	now := time.Now()
	c := &FileReviewerCache{FileRequestCache{Dir: t.TempDir(), TTL: time.Minute, now: func() time.Time { return now }}}
	c.Put("acme/portal", map[int]RequestReviewers{1: {Requested: []string{"bob"}}})
	if got, ok := c.Get("acme/portal"); !ok || got[1].Requested[0] != "bob" {
		t.Errorf("fresh entry: %v, %v", got, ok)
	}
	now = now.Add(2 * time.Minute)
	if _, ok := c.Get("acme/portal"); ok {
		t.Error("an entry older than the TTL was served")
	}
	if _, ok := c.Get("acme/other"); ok {
		t.Error("a miss was a hit")
	}
}

// Reviewers that were not fetched are unknown, not absent.
func TestReviewCellIsUnknownWhenReviewersWereNotFetched(t *testing.T) {
	if got := tableReview(&ChangeRequest{Number: 1, State: "open"}); got != "Unknown" {
		t.Errorf("review cell = %q, want Unknown", got)
	}
	if got := tableReview(&ChangeRequest{Number: 1, State: "open", ReviewDecision: "approved"}); got != "Approved" {
		t.Errorf("review cell = %q, want Approved", got)
	}
}

// An issue tracked in another repository than the code's is that repository's
// issue, so a request that closes it is not a second issue.
func TestIssueInTrackerRepoIsNotCountedTwice(t *testing.T) {
	row := tableRow("9", "Fix the thing", &ChangeRequest{Number: 9, State: "open", URL: "https://github.com/acme/api/pull/9",
		Closes: []string{"9"}, ClosesRefs: []string{"acme/portal#9"}})
	row.Repo = "acme/api"
	row.Issue.Provider = "github:acme/portal"
	row.Issue.URL = "https://github.com/acme/portal/issues/9"
	if got := StatusTableRows([]ChangeTopology{row}, "")[0][0]; got != "portal#9" {
		t.Errorf("issue cell = %q, want portal#9", got)
	}
	want := []string{
		"portal issue #9  https://github.com/acme/portal/issues/9",
		"PR #9            https://github.com/acme/api/pull/9",
	}
	if got := StatusTableURLs([]ChangeTopology{row}); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("urls\n got %q\nwant %q", got, want)
	}
}

func TestStatusTableLocalStateWording(t *testing.T) {
	row := func(l *LocalState) ChangeTopology { r := tableRow("42", "x", nil); r.Local = l; return r }
	for want, l := range map[string]*LocalState{
		"Local, upstream gone": {Upstream: "origin/x", UpstreamGone: true},
		"Local, state unknown": {Upstream: "origin/x", StatusUnavailable: true},
		"Local, clean":         {Upstream: "origin/x"},
	} {
		if got := tableState(row(l)); got != want {
			t.Errorf("tableState = %q, want %q", got, want)
		}
	}
}

func TestStatusTableIssueRepoCaseAndControlChars(t *testing.T) {
	r := tableRow("42", "evil\n\x1b]0;pwned\x07title", &ChangeRequest{Number: 7, State: "open",
		URL: "https://github.com/Acme/Portal/pull/7", Closes: []string{"42"}, ClosesRefs: []string{"Acme/Portal#42"}})
	cells := StatusTableRows([]ChangeTopology{r}, "")[0]
	if cells[0] != "#42" {
		t.Errorf("issue cell = %q, want one #42 despite repository name case", cells[0])
	}
	if strings.ContainsAny(cells[1], "\n\x1b\x07") {
		t.Errorf("what cell keeps control characters: %q", cells[1])
	}
}

// A row whose request lookup did not happen is unknown, not "not opened".
func TestPRCellIsUnknownWhenTheLookupDidNotHappen(t *testing.T) {
	row := tableRow("1", "x", nil)
	if got := StatusTableRows([]ChangeTopology{row}, "")[0][2]; got != "Not opened" {
		t.Errorf("PR cell = %q, want Not opened", got)
	}
	row.RequestsUnknown = true
	if got := StatusTableRows([]ChangeTopology{row}, "")[0][2]; got != "Unknown" {
		t.Errorf("PR cell = %q, want Unknown", got)
	}
}

// ClosesRefs names a repository for only some of Closes; a number a reference
// already placed elsewhere is not placed again here.
func TestClosesWithoutRepoDoesNotDuplicateANamedOne(t *testing.T) {
	row := tableRow("1", "x", &ChangeRequest{Number: 3, State: "open", URL: "https://github.com/acme/portal/pull/3",
		Closes: []string{"5", "9"}, ClosesRefs: []string{"acme/other#5"}})
	if got := StatusTableRows([]ChangeTopology{row}, "")[0][0]; got != "#1 + other#5 + #9" {
		t.Errorf("issue cell = %q, want %q", got, "#1 + other#5 + #9")
	}
}

func TestDisplayWidthCountsColumns(t *testing.T) {
	for s, want := range map[string]int{
		"abc": 3, "日本語": 6, "✨ x": 4, "e\u0301": 1, "": 0,
		"☕": 2, "⏳": 2, "🩷": 2, "🫶": 2, "⚡": 2, "❌": 2, // wide symbols and the newer emoji blocks
		"☀": 1, "☀\ufe0f": 2, "❤\ufe0f": 2, // VS16 asks for the wide emoji form
		"\ufe0f": 0, "a\ufe0f": 1,
	} {
		if got := displayWidth(s); got != want {
			t.Errorf("displayWidth(%q) = %d, want %d", s, got, want)
		}
	}
}

// Wide characters take two columns, so a table with them still draws a box
// whose every line is the same width.
func TestGridStaysAlignedWithWideCharacters(t *testing.T) {
	row := tableRow("1", "✨ 日本語のタイトル feat: add the thing", &ChangeRequest{Number: 2, State: "open"})
	for _, width := range []int{0, 70} {
		out := RenderStatusTable([]ChangeTopology{row}, "", width, false)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		for _, l := range lines {
			if displayWidth(l) != displayWidth(lines[0]) {
				t.Fatalf("width %d: line is %d columns, first is %d:\n%s", width, displayWidth(l), displayWidth(lines[0]), out)
			}
		}
		if width > 0 && displayWidth(lines[0]) > width {
			t.Errorf("table is %d columns, want at most %d", displayWidth(lines[0]), width)
		}
	}
}

// Two links with the same label never nest, whatever order they come in.
func TestLinkifyNeverNestsLinks(t *testing.T) {
	got := linkify("#5", []tableLink{{Label: "#5", URL: "a"}, {Label: "#5", URL: "b"}})
	if strings.Count(got, oscOpen+"a") != 1 || strings.Contains(got, oscOpen+"b") {
		t.Errorf("linkify = %q, want one link, to a", got)
	}
}

func TestTableStateOfRowsWithoutARequest(t *testing.T) {
	for name, c := range map[string]struct {
		local LocalState
		want  string
	}{
		"status failed wins over everything": {LocalState{StatusUnavailable: true, Ahead: 2, Upstream: ""}, "Local, state unknown"},
		"upstream gone":                      {LocalState{Upstream: "origin/x", UpstreamGone: true}, "Local, upstream gone"},
		"uncommitted":                        {LocalState{Upstream: "origin/x", Modified: 1}, "Local, uncommitted"},
		"clean":                              {LocalState{Upstream: "origin/x"}, "Local, clean"},
	} {
		row := tableRow("1", "x", nil)
		row.Local = &c.local
		if got := tableState(row); got != c.want {
			t.Errorf("%s: state = %q, want %q", name, got, c.want)
		}
	}
}

func TestCellsDropControlCharacters(t *testing.T) {
	row := tableRow("1", "evil\x1b]0;owned\x07 title\nline two", &ChangeRequest{Number: 2, State: "open"})
	cell := StatusTableRows([]ChangeTopology{row}, "")[0][1]
	if strings.ContainsAny(cell, "\x1b\x07\n") || !strings.Contains(cell, "title line two") {
		t.Errorf("cell = %q", cell)
	}
}

func TestSafeURLRefusesAnythingBut(t *testing.T) {
	for url, ok := range map[string]bool{
		"https://github.com/a/b/pull/1": true, "http://h/p": true,
		"javascript:alert(1)": false, "https://h/\x1b]0;x\x07": false, "https://h/a b": false, "": false, "https://h/\x07": false,
	} {
		if got := safeURL(url) != ""; got != ok {
			t.Errorf("safeURL(%q) accepted = %v, want %v", url, got, ok)
		}
	}
	row := tableRow("1", "x", &ChangeRequest{Number: 2, State: "open", URL: "https://h/p\x1b\\evil"})
	if out := RenderStatusTable([]ChangeTopology{row}, "", 0, true); strings.Contains(out, "\x1b") {
		t.Errorf("an unsafe URL reached the output: %q", out)
	}
	if urls := StatusTableURLs([]ChangeTopology{row}); len(urls) != 0 {
		t.Errorf("an unsafe URL was listed: %q", urls)
	}
}

// Repository names are case-insensitive: the same issue spelt two ways is one
// issue, and rows of one repository do not look like several.
func TestRepositoryCaseDoesNotMatter(t *testing.T) {
	row := tableRow("42", "x", &ChangeRequest{Number: 7, State: "open", Closes: []string{"42"}, ClosesRefs: []string{"acme/portal#42"}})
	row.Repo = "Acme/Portal"
	row.Issue.Provider = "github:Acme/Portal"
	if got := StatusTableRows([]ChangeTopology{row}, "")[0][0]; got != "#42" {
		t.Errorf("issue cell = %q, want #42", got)
	}
	a, b := tableRow("1", "a", nil), tableRow("2", "b", nil)
	a.Repo, b.Repo = "Acme/Portal", "acme/portal"
	if manyRepos([]ChangeTopology{a, b}) {
		t.Error("one repository in two spellings counted as two")
	}
	set := ChangeRequestSet{Repo: "Acme/Portal", Requests: []ChangeRequest{{Number: 3, State: "open", Closes: []string{"9"}}, {Number: 4, State: "open", ClosesRefs: []string{"acme/portal#8"}}}}
	if cr := set.ForIssue("acme/portal", "9"); cr == nil || cr.Number != 3 {
		t.Errorf("ForIssue by bare number = %+v", cr)
	}
	if cr := set.ForIssue("Acme/Portal", "8"); cr == nil || cr.Number != 4 {
		t.Errorf("ForIssue by ref = %+v", cr)
	}
}

// me/api#3 and org/api#3 would both read "api#3": the labels name the owner.
func TestClashingShortRepoNamesStayDistinct(t *testing.T) {
	row := tableRow("3", "x", &ChangeRequest{Number: 7, State: "open", URL: "https://github.com/me/api/pull/7",
		Closes: []string{"3"}, ClosesRefs: []string{"org/api#3"}})
	row.Repo = "me/api"
	row.Issue.Provider = "github:me/api"
	row.Issue.URL = "https://github.com/me/api/issues/3"
	cell := StatusTableRows([]ChangeTopology{row}, "")[0][0]
	if cell != "me/api#3 + org/api#3" {
		t.Errorf("issue cell = %q, want both owners named: two repositories answer to api", cell)
	}
	// Two foreign repositories that share a short name clash with each other.
	row.PR.ClosesRefs = []string{"org/api#3", "x/api#3"}
	row.PR.Closes = []string{"3", "3"}
	if got := StatusTableRows([]ChangeTopology{row}, "")[0][0]; got != "me/api#3 + org/api#3 + x/api#3" {
		t.Errorf("issue cell = %q", got)
	}
	row.PR.ClosesRefs, row.PR.Closes = []string{"org/api#3"}, []string{"3"}
	linked := RenderStatusTable([]ChangeTopology{row}, "", 0, true)
	if strings.Count(linked, oscOpen+"https://github.com/me/api/issues/3") != 1 || strings.Count(linked, oscOpen+"https://github.com/org/api/issues/3") != 1 {
		t.Errorf("each issue should link once to its own page:\n%q", linked)
	}
}

func TestRelatedUnresolvedListsRowlessItems(t *testing.T) {
	cur := tableRow("1", "cur", nil)
	rel := &RelatedWork{Linked: []RelatedItem{{Key: "acme/x#9"}, {Key: RowKey(cur), Row: &cur}}, Blocks: []RelatedItem{{Key: "acme/x#9"}}}
	if got := rel.Unresolved(); len(got) != 1 || got[0] != "acme/x#9" {
		t.Errorf("unresolved = %v", got)
	}
	if (*RelatedWork)(nil).Unresolved() != nil {
		t.Error("nil related has unresolved items")
	}
}

// Which repositories answer to "api" is decided over the whole table, so rows
// that never share a row still tell me/api from org/api, in cells and in -urls.
func TestShortNameClashAcrossRowsNamesOwners(t *testing.T) {
	mk := func(repo, issueNum string) ChangeTopology {
		r := tableRow(issueNum, "t"+issueNum, &ChangeRequest{Number: 12, State: "open", URL: "https://github.com/" + repo + "/pull/12"})
		r.Repo = repo
		r.Issue.Provider = "github:" + repo
		r.Issue.URL = "https://github.com/" + repo + "/issues/" + issueNum
		return r
	}
	rows := []ChangeTopology{mk("me/api", "5"), mk("org/api", "5"), mk("me/web", "7")}
	cells := StatusTableRows(rows, "")
	if cells[0][0] != "me/api#5" || cells[1][0] != "org/api#5" || cells[2][0] != "web#7" {
		t.Errorf("issue cells = %q, %q, %q", cells[0][0], cells[1][0], cells[2][0])
	}
	got := StatusTableURLs(rows)
	want := []string{
		"me/api issue #5   https://github.com/me/api/issues/5",
		"me/api PR #12     https://github.com/me/api/pull/12",
		"org/api issue #5  https://github.com/org/api/issues/5",
		"org/api PR #12    https://github.com/org/api/pull/12",
		"web issue #7      https://github.com/me/web/issues/7",
		"web PR #12        https://github.com/me/web/pull/12",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("urls\n got %q\nwant %q", got, want)
	}
}

func TestStatusTableNotes(t *testing.T) {
	rows := []ChangeTopology{
		{Repo: "acme/portal", RequestsUnknown: true},
		{Repo: "acme/portal", PR: &ChangeRequest{Number: 1, State: "open"}},
		{Repo: "acme/api"},
	}
	status := []string{"request state omitted for acme/portal: boom"}
	rel := &RelatedWork{
		Incomplete: []string{
			"request state omitted for acme/portal: boom", // status already says it
			"request state omitted for acme/api: slow",
			"request state omitted for acme/elsewhere: nope", // no row of it is shown
			"clone /w/x has no origin",
		},
		Linked: []RelatedItem{{Key: "acme/api#4"}},
	}
	got := StatusTableNotes(rows, status, rel)
	want := []string{
		"status: pull requests were not looked up for 1 of 3 rows, shown as Unknown",
		"incomplete: request state omitted for acme/api: slow",
		"incomplete: clone /w/x has no origin",
		"status: acme/api#4 is related but was not found in the workspace, so it has no row",
		"status: request state omitted for acme/portal: boom",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("notes\n got %q\nwant %q", got, want)
	}
	if n := StatusTableNotes(rows[1:2], nil, nil); len(n) != 0 {
		t.Errorf("a table with nothing unknown has notes: %q", n)
	}
}

// Bidirectional controls reorder a line: a row could be made to read as
// something else. They are dropped; joiners that emoji need are kept.
func TestCellsDropBidiAndFormatCharacters(t *testing.T) {
	title := "safe\u202e\u2066reversed\u2069\u200f text\u00adend \U0001F468\u200d\U0001F469"
	row := tableRow("1", title, &ChangeRequest{Number: 2, State: "open"})
	cell := StatusTableRows([]ChangeTopology{row}, "")[0][1]
	for _, bad := range []string{"\u202e", "\u2066", "\u2069", "\u200f", "\u00ad"} {
		if strings.Contains(cell, bad) {
			t.Errorf("cell kept %U: %q", []rune(bad)[0], cell)
		}
	}
	if !strings.Contains(cell, "\U0001F468\u200d\U0001F469") {
		t.Errorf("the emoji joiner was dropped: %q", cell)
	}
}

func TestNoBranchIsNotLocal(t *testing.T) {
	r := ChangeTopology{Slug: "x", Title: "Not started"}
	if got := tableState(r); got != "No branch" {
		t.Errorf("state = %q, want No branch", got)
	}
	r.Branch = "feat/1-x"
	if got := tableState(r); got != "Local" {
		t.Errorf("a branch with no local state = %q, want Local", got)
	}
	r.Branch, r.Local = "", &LocalState{Upstream: "origin/x"}
	if got := tableState(r); got != "Local, clean" {
		t.Errorf("local state without a branch name = %q", got)
	}
}

// Only the reasons that really are about a repository are filtered by
// repository: a branch name with a slash in a reason is not one.
func TestNotesKeepReasonsThatOnlyLookLikeRepositories(t *testing.T) {
	rows := []ChangeTopology{{Repo: "acme/portal"}}
	rel := &RelatedWork{Incomplete: []string{
		"local state unavailable for feat/243-x: boom",
		"request state omitted for acme/elsewhere: nope",
		"reviewers unavailable for acme/elsewhere#4: the provider did not return it",
		"request listing for acme/elsewhere may be truncated; older requests can be missing",
		"request state omitted for Acme/Portal: shown repo, spelt differently",
	}}
	got := StatusTableNotes(rows, nil, rel)
	want := []string{
		"incomplete: local state unavailable for feat/243-x: boom",
		"incomplete: request state omitted for Acme/Portal: shown repo, spelt differently",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("notes\n got %q\nwant %q", got, want)
	}
}

func TestDisplayWidthOfFlagsJoinersAndColouredCircles(t *testing.T) {
	for s, want := range map[string]int{
		"🇸🇪": 2, "🇸": 1, // a flag is a pair of regional indicators: two columns, each alone one
		"🟠": 2, "🟫": 2, "🟰": 2, // 1F7E0-1F7EB and the heavy equals sign
		"👨\u200d👩": 2, "👨\u200d👩\u200d👧": 2, // a joined sequence is one glyph
	} {
		if got := displayWidth(s); got != want {
			t.Errorf("displayWidth(%q) = %d, want %d", s, got, want)
		}
	}
}

// A character is never cut from its variation selector, joiner or marks.
func TestWrapCellNeverSplitsACharacter(t *testing.T) {
	for name, word := range map[string]string{
		"vs16":   strings.Repeat("☀\ufe0f", 6),
		"zwj":    strings.Repeat("👨\u200d👩\u200d👧", 4),
		"marks":  strings.Repeat("e\u0301", 9),
		"flags":  strings.Repeat("🇸🇪", 5),
		"narrow": strings.Repeat("x", 25),
	} {
		lines := wrapCell(word, 5)
		if strings.Join(lines, "") != word {
			t.Errorf("%s: wrapping lost text: %q", name, lines)
		}
		for _, l := range lines {
			if displayWidth(l) > 5 {
				t.Errorf("%s: line %q is %d columns, want at most 5", name, l, displayWidth(l))
			}
			if strings.HasPrefix(l, "\ufe0f") || strings.HasPrefix(l, "\u200d") || strings.HasPrefix(l, "\u0301") {
				t.Errorf("%s: line %q starts with a combining character", name, l)
			}
		}
	}
}

// A table of many rows is laid out in one pass: the per-table facts are
// computed once, not per row.
func TestRenderStatusTableOfManyRowsIsFast(t *testing.T) {
	var rows []ChangeTopology
	for i := 0; i < 400; i++ {
		r := tableRow(strconv.Itoa(i+1), strings.Repeat("a long title ", 8), &ChangeRequest{Number: i + 1000, State: "open",
			URL: "https://github.com/acme/portal/pull/" + strconv.Itoa(i+1000), Closes: []string{"1", "2", "3"}})
		rows = append(rows, r)
	}
	start := time.Now()
	out := RenderStatusTable(rows, "", 120, true)
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("400 rows took %v", d)
	}
	if strings.Count(out, "\n") < 400 {
		t.Error("rows are missing")
	}
}

func TestClosedMarkerStaysOutsideTheLink(t *testing.T) {
	r := linkedRow()
	r.Issue.State = "closed"
	out := RenderStatusTable([]ChangeTopology{r}, "", 0, true)
	if !strings.Contains(out, "\x1b]8;;https://github.com/acme/portal/issues/42\x1b\\#42\x1b]8;;\x1b\\ closed") {
		t.Errorf("closed marker not right after the linked number:\n%q", out)
	}
}
