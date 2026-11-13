package specsync

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateStatusTableGolden = flag.Bool("update-status-table", false, "rewrite testdata/status-table.golden")

// goldenRows is one row per state the table has to tell apart.
func goldenRows() []ChangeTopology {
	pr := func(n int, mod func(*ChangeRequest)) *ChangeRequest {
		c := &ChangeRequest{Number: n, State: "open", Mergeable: "mergeable", Checks: ChangeRequestChecks{Pass: 18},
			Size: ChangeRequestSize{Files: 4, Additions: 50, Deletions: 6}}
		mod(c)
		return c
	}
	row := func(id, title string, c *ChangeRequest) ChangeTopology { return tableRow(id, title, c) }
	return []ChangeTopology{
		row("1", "Approved and green", pr(11, func(c *ChangeRequest) {
			c.ReviewDecision = "approved"
			c.Reviews = []ChangeRequestReview{{"ann", "approved"}}
		})),
		row("2", "Waiting for a reviewer", pr(12, func(c *ChangeRequest) {
			c.ReviewDecision = "review_required"
			c.ReviewRequests = []string{"bob", "team:core"}
		})),
		row("3", "Changes requested", pr(13, func(c *ChangeRequest) {
			c.ReviewDecision = "changes_requested"
			c.Reviews = []ChangeRequestReview{{"cy", "changes_requested"}}
			c.UnresolvedThreads = two(2)
		})),
		row("4", "A check fails", pr(14, func(c *ChangeRequest) {
			c.Checks = ChangeRequestChecks{Pass: 16, Fail: 1, Failing: []string{"types"}}
		})),
		row("5", "Checks still running", pr(15, func(c *ChangeRequest) {
			c.Checks = ChangeRequestChecks{Pass: 17, Pending: 1}
		})),
		row("6", "No checks at all", pr(16, func(c *ChangeRequest) { c.Checks = ChangeRequestChecks{} })),
		func() ChangeTopology {
			r := row("7", "Not opened yet", nil)
			r.Local = &BranchSize0
			return r
		}(),
		row("8", "Already merged", &ChangeRequest{Number: 18, State: "merged", Size: ChangeRequestSize{Files: 1, Additions: 3}}),
	}
}

// BranchSize0 is the local state of a branch with two unpushed commits.
var BranchSize0 = LocalState{Upstream: "origin/x", Ahead: 2, Size: &BranchSize{Files: 3, Additions: 10, Deletions: 1, Commits: 2}}

func TestStatusTableGolden(t *testing.T) {
	got := RenderStatusTable(goldenRows(), "", 0, false)
	path := filepath.Join("testdata", "status-table.golden")
	if *updateStatusTableGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update-status-table to create it)", err)
	}
	if string(want) != got {
		t.Errorf("table changed; if deliberate: go test . -run TestStatusTableGolden -update-status-table\n got:\n%s\nwant:\n%s", got, want)
	}
}
