package specsync

import (
	"context"
	"sort"
	"strings"
)

// ChangeRequest is a pull or merge request as the work provider reports it.
// Every field comes from the provider; nothing is inferred.
type ChangeRequest struct {
	Number         int                 `json:"number"`
	URL            string              `json:"url"`
	State          string              `json:"state"` // open, closed, merged
	Draft          bool                `json:"draft"`
	Branch         string              `json:"branch"` // head branch
	Base           string              `json:"base,omitempty"`
	ReviewDecision string              `json:"reviewDecision,omitempty"` // approved, changes_requested, review_required
	Mergeable      string              `json:"mergeable,omitempty"`      // mergeable, conflicting, unknown
	Checks         ChangeRequestChecks `json:"checks"`
	Size           ChangeRequestSize   `json:"size"`
	ReviewStarted  bool                `json:"reviewStarted,omitempty"` // any review or comment exists
	Closes         []string            `json:"closes,omitempty"`        // issue numbers it closes
}

// ChangeRequestChecks summarises the request's checks.
type ChangeRequestChecks struct {
	Pass    int      `json:"pass"`
	Fail    int      `json:"fail"`
	Pending int      `json:"pending"`
	Failing []string `json:"failing,omitempty"` // names of failing checks, sorted
}

// ChangeRequestSize is the size the provider reports.
type ChangeRequestSize struct {
	Files     int `json:"files"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Commits   int `json:"commits,omitempty"` // not reported by the GitHub provider; see local.size
}

// ChangeRequestSet is one repo's listing. Truncated is true when the listing
// filled a whole page, so older requests may be missing.
type ChangeRequestSet struct {
	Requests  []ChangeRequest
	Truncated bool
}

// ChangeRequestSource is an optional provider capability: list a repo's
// change requests. A provider without it yields rows without a request block.
type ChangeRequestSource interface {
	ChangeRequests(ctx context.Context) (ChangeRequestSet, error)
}

// ForBranch returns the request whose head branch is branch: an open one if
// any, else the newest. nil when there is none.
func (s ChangeRequestSet) ForBranch(branch string) *ChangeRequest {
	var best *ChangeRequest
	for i := range s.Requests {
		c := &s.Requests[i]
		if c.Branch != branch {
			continue
		}
		switch {
		case best == nil:
			best = c
		case (c.State == "open") != (best.State == "open"):
			if c.State == "open" {
				best = c
			}
		case c.Number > best.Number:
			best = c
		}
	}
	if best == nil {
		return nil
	}
	cp := *best
	return &cp
}

// RowAttention derives a row's attention flags from its local state and
// request, in the documented vocabulary order. Either may be nil. Each flag is
// a deterministic predicate over the facts.
func RowAttention(ls *LocalState, cr *ChangeRequest) []string {
	var flags []string
	volatile := false
	if ls != nil {
		for _, f := range LocalAttention(*ls) {
			if f == "volatile-path" {
				volatile = true
				continue
			}
			flags = append(flags, f)
		}
	}
	open := cr != nil && cr.State == "open"
	if open && cr.Checks.Fail > 0 {
		flags = append(flags, "ci-failing")
	}
	if open && cr.ReviewDecision == "changes_requested" {
		flags = append(flags, "changes-requested")
	}
	if open && !cr.Draft && !cr.ReviewStarted && cr.ReviewDecision != "approved" {
		flags = append(flags, "review-missing")
	}
	if volatile {
		flags = append(flags, "volatile-path")
	}
	if cr != nil && cr.State == "merged" && ls != nil && ls.Modified+ls.Untracked == 0 {
		flags = append(flags, "cleanup")
	}
	if open && !cr.Draft && cr.ReviewDecision == "approved" && cr.Checks.Fail == 0 &&
		cr.Checks.Pending == 0 && cr.Mergeable == "mergeable" {
		flags = append(flags, "ready-to-merge")
	}
	return flags
}

func sortedUnique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func lower(s string) string { return strings.ToLower(s) }
