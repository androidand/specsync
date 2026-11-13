package specsync

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
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
			got := StatusTableRows([]ChangeTopology{c.row}, "", true)[0]
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("cells\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestStatusTableSeveralIssuesOnePR(t *testing.T) {
	pr := &ChangeRequest{Number: 26, State: "open", Closes: []string{"4015", "3800"}}
	row := tableRow("4015", "Job output", pr)
	got := StatusTableRows([]ChangeTopology{row}, "", true)[0][0]
	if got != "#3800 + #4015" && got != "#4015 + #3800" {
		t.Errorf("issue cell = %q, want both issues", got)
	}
}

func TestStatusTableNamesRepoWhenRowsSpanRepos(t *testing.T) {
	a := tableRow("1", "A", nil)
	b := tableRow("2", "B", nil)
	b.Repo = "acme/api"
	b.Issue.Provider = "github:acme/api"
	cells := StatusTableRows([]ChangeTopology{a, b}, RowKey(a), true)
	if cells[0][0] != "▶ portal#1" || cells[1][0] != "api#2" {
		t.Errorf("issue cells = %q, %q", cells[0][0], cells[1][0])
	}
}

func TestRenderStatusTableWrapsToWidth(t *testing.T) {
	row := tableRow("42", strings.Repeat("long words ", 12), &ChangeRequest{Number: 7, State: "open"})
	out := RenderStatusTable([]ChangeTopology{row}, "", 80, false, true)
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
	plain := RenderStatusTable(rows, "", 0, false, true)
	if strings.Contains(plain, "\x1b") {
		t.Errorf("plain table has escapes:\n%q", plain)
	}
	linked := RenderStatusTable(rows, "", 0, true, true)
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
	cells := StatusTableRows(rows, "", true)
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
	if got := StatusTableRows([]ChangeTopology{row}, "", true)[0][0]; got != "#5 + api#5" {
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
	if got := StatusTableRows([]ChangeTopology{row}, "", true)[0][0]; got != "#5" {
		t.Errorf("same issue twice: cell = %q, want #5", got)
	}
}

// Regression: a request with no state (an old cache, another provider) renders.
func TestTableStateSurvivesAnEmptyRequestState(t *testing.T) {
	row := tableRow("1", "x", &ChangeRequest{Number: 1})
	if got := StatusTableRows([]ChangeTopology{row}, "", true)[0][3]; got != "Unknown" {
		t.Errorf("state cell = %q, want Unknown", got)
	}
}

// The shapes below are what `gh pr list --json reviewRequests,reviews` returns:
// a user request carries login, a team request carries name and slug. A review
// is a comment, an approval, a change request or a dismissal; only the last
// three say what the reviewer thinks, and a later comment changes none of them.
func TestReviewersStandingFromGhJSON(t *testing.T) {
	const raw = `{
	  "number": 5,
	  "reviewRequests": [
	    {"__typename": "User", "login": "bob"},
	    {"__typename": "Team", "name": "Core Devs", "slug": "core-devs"}
	  ],
	  "reviews": [
	    {"author": {"login": "ann"}, "state": "APPROVED", "submittedAt": "2026-11-10T10:00:00Z"},
	    {"author": {"login": "ann"}, "state": "COMMENTED", "submittedAt": "2026-11-11T10:00:00Z"},
	    {"author": {"login": "cy"}, "state": "CHANGES_REQUESTED", "submittedAt": "2026-11-10T10:00:00Z"},
	    {"author": {"login": "cy"}, "state": "APPROVED", "submittedAt": "2026-11-12T10:00:00Z"},
	    {"author": {"login": "di"}, "state": "APPROVED", "submittedAt": "2026-11-10T10:00:00Z"},
	    {"author": {"login": "di"}, "state": "DISMISSED", "submittedAt": "2026-11-11T10:00:00Z"},
	    {"author": {"login": "zed"}, "state": "COMMENTED", "submittedAt": "2026-11-10T10:00:00Z"},
	    {"author": {"login": ""}, "state": "APPROVED", "submittedAt": "2026-11-10T10:00:00Z"}
	  ]
	}`
	var gh ghReviewers
	if err := json.Unmarshal([]byte(raw), &gh); err != nil {
		t.Fatal(err)
	}
	asked, reviews := gh.standing()
	if strings.Join(asked, ",") != "bob,team:core-devs" {
		t.Errorf("review requests = %v", asked)
	}
	want := []ChangeRequestReview{{"ann", "approved"}, {"cy", "approved"}, {"di", "dismissed"}}
	if len(reviews) != 3 || reviews[0] != want[0] || reviews[1] != want[1] || reviews[2] != want[2] {
		t.Errorf("reviews = %v, want %v", reviews, want)
	}
}

func TestWithReviewersFillsOpenRequestsOnly(t *testing.T) {
	run := func(_ context.Context, args ...string) (string, error) {
		if !strings.Contains(strings.Join(args, " "), "--state open") || !strings.Contains(strings.Join(args, " "), "number,reviewRequests,reviews") {
			t.Errorf("unexpected gh call %v", args)
		}
		return `[{"number":1,"reviewRequests":[{"login":"bob"}],"reviews":[{"author":{"login":"ann"},"state":"APPROVED","submittedAt":"2026-11-10T10:00:00Z"}]}]`, nil
	}
	set := ChangeRequestSet{Requests: []ChangeRequest{{Number: 1, State: "open"}, {Number: 2, State: "merged"}}}
	got := NewGitHubProviderFuncWithRepo("acme/portal", run).WithReviewers(context.Background(), set)
	if !got.Reviewers || !got.Requests[0].ReviewersKnown || got.Requests[0].Reviews[0].Reviewer != "ann" || got.Requests[0].ReviewRequests[0] != "bob" {
		t.Errorf("open request not filled: %+v", got)
	}
	if got.Requests[1].ReviewersKnown || set.Requests[0].ReviewersKnown {
		t.Errorf("a merged request was filled or the input was changed: %+v / %+v", got.Requests[1], set.Requests[0])
	}
	// A failed call claims nothing.
	failing := func(context.Context, ...string) (string, error) { return "", errors.New("boom") }
	if got := NewGitHubProviderFuncWithRepo("acme/portal", failing).WithReviewers(context.Background(), set); got.Reviewers || got.Requests[0].ReviewersKnown {
		t.Errorf("a failed lookup marked reviewers known: %+v", got)
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
	if got := StatusTableRows([]ChangeTopology{row}, "", true)[0][0]; got != "portal#9" {
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
	cells := StatusTableRows([]ChangeTopology{r}, "", true)[0]
	if cells[0] != "#42" {
		t.Errorf("issue cell = %q, want one #42 despite repository name case", cells[0])
	}
	if strings.ContainsAny(cells[1], "\n\x1b\x07") {
		t.Errorf("what cell keeps control characters: %q", cells[1])
	}
}

// Without request lookups an absent request is unknown, not "not opened".
func TestPRCellIsUnknownWhenRequestsWereNotLooked(t *testing.T) {
	row := tableRow("1", "x", nil)
	if got := StatusTableRows([]ChangeTopology{row}, "", false)[0][2]; got != "Unknown" {
		t.Errorf("PR cell = %q, want Unknown", got)
	}
	if got := StatusTableRows([]ChangeTopology{row}, "", true)[0][2]; got != "Not opened" {
		t.Errorf("PR cell = %q, want Not opened", got)
	}
}

// ClosesRefs names a repository for only some of Closes; a number a reference
// already placed elsewhere is not placed again here.
func TestClosesWithoutRepoDoesNotDuplicateANamedOne(t *testing.T) {
	row := tableRow("1", "x", &ChangeRequest{Number: 3, State: "open", URL: "https://github.com/acme/portal/pull/3",
		Closes: []string{"5", "9"}, ClosesRefs: []string{"acme/other#5"}})
	if got := StatusTableRows([]ChangeTopology{row}, "", true)[0][0]; got != "#1 + other#5 + #9" {
		t.Errorf("issue cell = %q, want %q", got, "#1 + other#5 + #9")
	}
}

func TestDisplayWidthCountsColumns(t *testing.T) {
	for s, want := range map[string]int{"abc": 3, "日本語": 6, "✨ x": 4, "e\u0301": 1, "": 0} {
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
		out := RenderStatusTable([]ChangeTopology{row}, "", width, false, true)
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
	cell := StatusTableRows([]ChangeTopology{row}, "", true)[0][1]
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
	if out := RenderStatusTable([]ChangeTopology{row}, "", 0, true, true); strings.Contains(out, "\x1b") {
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
	if got := StatusTableRows([]ChangeTopology{row}, "", true)[0][0]; got != "#42" {
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
	cell := StatusTableRows([]ChangeTopology{row}, "", true)[0][0]
	if cell != "#3 + org/api#3" {
		t.Errorf("issue cell = %q, want the other repository's owner named", cell)
	}
	// Two foreign repositories that share a short name clash with each other.
	row.PR.ClosesRefs = []string{"org/api#3", "x/api#3"}
	row.PR.Closes = []string{"3", "3"}
	if got := StatusTableRows([]ChangeTopology{row}, "", true)[0][0]; got != "#3 + org/api#3 + x/api#3" {
		t.Errorf("issue cell = %q", got)
	}
	row.PR.ClosesRefs, row.PR.Closes = []string{"org/api#3"}, []string{"3"}
	linked := RenderStatusTable([]ChangeTopology{row}, "", 0, true, true)
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
