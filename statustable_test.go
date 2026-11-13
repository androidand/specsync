package specsync

import (
	"encoding/json"
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
		pr := &ChangeRequest{Number: 7, State: "open", Mergeable: "mergeable", Checks: ChangeRequestChecks{Pass: 3},
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
		{"draft", tableRow("42", "Add thing", &ChangeRequest{Number: 7, State: "open", Draft: true}),
			[]string{"#42", "Add thing", "#7", "Draft", "none", "No review", "–"}},
		{"not opened", func() ChangeTopology {
			r := tableRow("42", "Add thing", nil)
			r.Local = &LocalState{Upstream: "origin/x", Ahead: 2, Size: &BranchSize{Files: 3, Additions: 10, Deletions: 1, Commits: 2}}
			return r
		}(), []string{"#42", "Add thing", "Not opened", "Local, 2 unpushed", "–", "–", "3 files, +10 -1"}},
		{"title from the request when there is no change", ChangeTopology{Repo: "acme/portal", Branch: "feat/9-y", NoChange: true,
			PR: &ChangeRequest{Number: 9, State: "open", Title: "chore: bump", Closes: []string{"9", "8"}}},
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

// The shapes below are what `gh pr list --json reviewRequests,latestReviews`
// returns: a user request carries login, a team request carries name and slug.
func TestToChangeRequestReadsReviewersFromGhJSON(t *testing.T) {
	const raw = `{
	  "number": 5, "url": "https://github.com/acme/portal/pull/5", "title": "T", "state": "OPEN",
	  "reviewRequests": [
	    {"__typename": "User", "login": "bob"},
	    {"__typename": "Team", "name": "Core Devs", "slug": "core-devs"}
	  ],
	  "latestReviews": [
	    {"author": {"login": "zed"}, "state": "COMMENTED"},
	    {"author": {"login": "ann"}, "state": "APPROVED"},
	    {"author": {"login": "cy"}, "state": "DISMISSED"},
	    {"author": {"login": ""}, "state": "APPROVED"}
	  ]
	}`
	var gh ghChangeRequest
	if err := json.Unmarshal([]byte(raw), &gh); err != nil {
		t.Fatal(err)
	}
	cr := gh.toChangeRequest()
	if strings.Join(cr.ReviewRequests, ",") != "bob,team:core-devs" {
		t.Errorf("review requests = %v", cr.ReviewRequests)
	}
	want := []ChangeRequestReview{{"ann", "approved"}, {"cy", "dismissed"}, {"zed", "commented"}}
	if len(cr.Reviews) != 3 || cr.Reviews[0] != want[0] || cr.Reviews[1] != want[1] || cr.Reviews[2] != want[2] {
		t.Errorf("reviews = %v, want %v", cr.Reviews, want)
	}
	if cr.Title != "T" {
		t.Errorf("title = %q", cr.Title)
	}
	// Commented and dismissed reviews decide nothing.
	cr.State = "open"
	cr.Reviews = cr.Reviews[1:]
	if got := tableReview(&cr); got != "No review; requested from bob, team:core-devs" && got != "requested from bob, team:core-devs" {
		t.Errorf("review cell = %q", got)
	}
}
