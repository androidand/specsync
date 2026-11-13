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
	out := RenderStatusTable([]ChangeTopology{row}, "", 80)
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
