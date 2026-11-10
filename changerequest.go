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
	Author         string              `json:"author,omitempty"`
	Base           string              `json:"base,omitempty"`
	ReviewDecision string              `json:"reviewDecision,omitempty"` // approved, changes_requested, review_required
	Mergeable      string              `json:"mergeable,omitempty"`      // mergeable, conflicting, unknown
	Checks         ChangeRequestChecks `json:"checks"`
	Size           ChangeRequestSize   `json:"size"`
	Closes         []string            `json:"closes,omitempty"` // issue numbers it closes
	// HeadSHA is the commit the request's head branch pointed at.
	HeadSHA string `json:"headSha,omitempty"`
	// Fork is true when the request comes from another repository, so its head
	// branch name says nothing about branches in this one.
	Fork bool `json:"fork,omitempty"`
	// UnresolvedThreads is the count of unresolved review threads on an open
	// request. nil means the provider did not say, which is not the same as 0.
	UnresolvedThreads *int `json:"unresolvedThreads,omitempty"`
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
		if c.Branch != branch || c.Fork {
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

// attentionRules is the one table of attention flags, in the documented
// vocabulary order. Each rule is a deterministic predicate over the row's
// local state and request; either may be nil. See the status-state spec.
var attentionRules = []struct {
	flag string
	when func(ls *LocalState, cr *ChangeRequest) bool
}{
	{"unpushed", func(ls *LocalState, _ *ChangeRequest) bool { return ls != nil && ls.Ahead > 0 }},
	{"uncommitted", func(ls *LocalState, _ *ChangeRequest) bool { return ls != nil && ls.Modified+ls.Untracked > 0 }},
	{"no-upstream", func(ls *LocalState, _ *ChangeRequest) bool { return ls != nil && ls.Upstream == "" }},
	{"behind-upstream", func(ls *LocalState, _ *ChangeRequest) bool {
		return ls != nil && ls.UpstreamBehind > 0 && ls.Ahead == 0
	}},
	{"diverged", func(ls *LocalState, _ *ChangeRequest) bool { return ls != nil && ls.UpstreamBehind > 0 && ls.Ahead > 0 }},
	{"behind-base", func(ls *LocalState, _ *ChangeRequest) bool { return ls != nil && ls.BehindBase > 0 }},
	{"base-touches-your-files", func(ls *LocalState, _ *ChangeRequest) bool { return ls != nil && len(ls.BaseTouchesYourFiles) > 0 }},
	{"conflicts-with-base", func(ls *LocalState, _ *ChangeRequest) bool { return ls != nil && len(ls.Conflicts) > 0 }},
	{"ci-failing", func(_ *LocalState, cr *ChangeRequest) bool { return isOpen(cr) && cr.Checks.Fail > 0 }},
	{"changes-requested", func(_ *LocalState, cr *ChangeRequest) bool {
		return isOpen(cr) && cr.ReviewDecision == "changes_requested"
	}},
	{"unresolved-comments", func(_ *LocalState, cr *ChangeRequest) bool {
		return isOpen(cr) && cr.UnresolvedThreads != nil && *cr.UnresolvedThreads > 0
	}},
	{"volatile-path", func(ls *LocalState, _ *ChangeRequest) bool { return ls != nil && ls.VolatilePath }},
	// cleanup: the request is merged, the tree is clean, and no commit was made
	// since the merged one (a reused branch name must not read as finished).
	{"cleanup", func(ls *LocalState, cr *ChangeRequest) bool {
		if cr == nil || cr.State != "merged" || ls == nil || ls.Modified+ls.Untracked != 0 {
			return false
		}
		return cr.HeadSHA == "" || ls.Head == "" || cr.HeadSHA == ls.Head
	}},
	{"ready-to-merge", func(_ *LocalState, cr *ChangeRequest) bool {
		return isOpen(cr) && !cr.Draft && cr.ReviewDecision == "approved" && cr.Checks.Fail == 0 &&
			cr.Checks.Pending == 0 && cr.Mergeable == "mergeable" &&
			cr.UnresolvedThreads != nil && *cr.UnresolvedThreads == 0
	}},
}

func isOpen(cr *ChangeRequest) bool { return cr != nil && cr.State == "open" }

// RowAttention derives a row's attention flags from the rule table, in
// vocabulary order. Either argument may be nil.
func RowAttention(ls *LocalState, cr *ChangeRequest) []string {
	var flags []string
	for _, r := range attentionRules {
		if r.when(ls, cr) {
			flags = append(flags, r.flag)
		}
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

// ForIssue returns the request that closes the issue, as the provider asserts:
// an open one if any, else the newest. Requests from forks are ignored.
func (s ChangeRequestSet) ForIssue(issue string) *ChangeRequest {
	var best *ChangeRequest
	for i := range s.Requests {
		c := &s.Requests[i]
		if c.Fork || !containsString(c.Closes, issue) {
			continue
		}
		switch {
		case best == nil:
			best = c
		case c.State == "open" && best.State != "open":
			best = c
		case (c.State == "open") == (best.State == "open") && c.Number > best.Number:
			best = c
		}
	}
	return best
}

// PRBodyReferencesIssue reports whether body contains a reference to the issue
// number, such as "#42", "Closes #42" or "Part of #42".
func PRBodyReferencesIssue(body, issueNum string) bool {
	return strings.Contains(body, "#"+issueNum)
}
