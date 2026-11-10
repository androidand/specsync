package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// defaultChangeRequestLimit bounds one gh listing. A full page is reported as
// possibly truncated.
//
// The page is 100 and the query leaves out `commits`: with it GitHub's GraphQL
// node limit rejects more than about 40 requests per page. The commit count
// comes from local git instead.
const defaultChangeRequestLimit = 100

type ghCheck struct {
	Typename   string `json:"__typename"`
	Name       string `json:"name"`
	Context    string `json:"context"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

type ghChangeRequest struct {
	Number         int    `json:"number"`
	URL            string `json:"url"`
	State          string `json:"state"`
	IsDraft        bool   `json:"isDraft"`
	HeadRefName    string `json:"headRefName"`
	HeadRefOid     string `json:"headRefOid"`
	CrossRepo      bool   `json:"isCrossRepository"`
	BaseRefName    string `json:"baseRefName"`
	ReviewDecision string `json:"reviewDecision"`
	Mergeable      string `json:"mergeable"`
	Additions      int    `json:"additions"`
	Deletions      int    `json:"deletions"`
	ChangedFiles   int    `json:"changedFiles"`
	Closing        []struct {
		Number int `json:"number"`
	} `json:"closingIssuesReferences"`
	Checks []ghCheck `json:"statusCheckRollup"`
}

// ChangeRequests lists the repo's pull requests, all states, with one gh call.
// When that page is full, older open requests may be missing, so the open ones
// are listed again and merged in.
func (p *GitHubProvider) ChangeRequests(ctx context.Context) (ChangeRequestSet, error) {
	limit := p.changeRequestLimit
	if limit == 0 {
		limit = defaultChangeRequestLimit
	}
	all, err := p.listChangeRequests(ctx, "all", limit)
	if err != nil {
		return ChangeRequestSet{}, err
	}
	set := ChangeRequestSet{Truncated: len(all) >= limit}
	have := map[int]bool{}
	for _, r := range all {
		have[r.Number] = true
		set.Requests = append(set.Requests, r.toChangeRequest())
	}
	if set.Truncated {
		if open, err := p.listChangeRequests(ctx, "open", limit); err == nil {
			for _, r := range open {
				if !have[r.Number] {
					set.Requests = append(set.Requests, r.toChangeRequest())
				}
			}
			set.Truncated = len(open) >= limit
		}
	}
	return set.withThreads(ctx, p), nil
}

// listChangeRequests runs one `gh pr list` for the given state.
func (p *GitHubProvider) listChangeRequests(ctx context.Context, state string, limit int) ([]ghChangeRequest, error) {
	args := append([]string{"pr", "list"}, p.repoFlag()...)
	args = append(args, "--state", state, "--limit", strconv.Itoa(limit), "--json",
		"number,url,state,isDraft,headRefName,headRefOid,isCrossRepository,baseRefName,reviewDecision,mergeable,"+
			"additions,deletions,changedFiles,closingIssuesReferences,statusCheckRollup")
	out, err := p.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var raw []ghChangeRequest
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parse gh pr list: %w", err)
	}
	return raw, nil
}

// withThreads fills in unresolved review thread counts for open requests.
// They are a nice-to-have: GraphQL only, and a failure leaves them unknown
// instead of failing the listing.
func (set ChangeRequestSet) withThreads(ctx context.Context, p *GitHubProvider) ChangeRequestSet {
	anyOpen := false
	for _, r := range set.Requests {
		anyOpen = anyOpen || r.State == "open"
	}
	if !anyOpen {
		return set
	}
	if counts, err := p.unresolvedThreads(ctx); err == nil {
		for i := range set.Requests {
			if n, ok := counts[set.Requests[i].Number]; ok && set.Requests[i].State == "open" {
				n := n
				set.Requests[i].UnresolvedThreads = &n
			}
		}
	}
	return set
}

const unresolvedThreadsQuery = `query($owner:String!,$name:String!){repository(owner:$owner,name:$name){` +
	`pullRequests(states:OPEN,first:50,orderBy:{field:UPDATED_AT,direction:DESC}){nodes{number ` +
	`reviewThreads(first:100){nodes{isResolved}}}}}}`

// unresolvedThreads returns the unresolved review thread count per open
// request number, from one GraphQL call (the CLI's listing has no thread data).
func (p *GitHubProvider) unresolvedThreads(ctx context.Context) (map[int]int, error) {
	repo, _ := p.resolveRepo(ctx)
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, fmt.Errorf("repo %q is not owner/name", repo)
	}
	out, err := p.run(ctx, "api", "graphql", "-f", "query="+unresolvedThreadsQuery, "-f", "owner="+owner, "-f", "name="+name)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Repository struct {
				PullRequests struct {
					Nodes []struct {
						Number        int `json:"number"`
						ReviewThreads struct {
							Nodes []struct {
								IsResolved bool `json:"isResolved"`
							} `json:"nodes"`
						} `json:"reviewThreads"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse graphql: %w", err)
	}
	counts := map[int]int{}
	for _, pr := range resp.Data.Repository.PullRequests.Nodes {
		n := 0
		for _, th := range pr.ReviewThreads.Nodes {
			if !th.IsResolved {
				n++
			}
		}
		counts[pr.Number] = n
	}
	return counts, nil
}

func (r ghChangeRequest) toChangeRequest() ChangeRequest {
	cr := ChangeRequest{
		Number:         r.Number,
		URL:            r.URL,
		State:          strings.ToLower(r.State),
		Draft:          r.IsDraft,
		Branch:         r.HeadRefName,
		HeadSHA:        r.HeadRefOid,
		Fork:           r.CrossRepo,
		Base:           r.BaseRefName,
		ReviewDecision: strings.ToLower(r.ReviewDecision),
		Mergeable:      strings.ToLower(r.Mergeable),
		Size:           ChangeRequestSize{Files: r.ChangedFiles, Additions: r.Additions, Deletions: r.Deletions},
	}
	for _, c := range r.Closing {
		cr.Closes = append(cr.Closes, strconv.Itoa(c.Number))
	}
	var failing []string
	for _, c := range r.Checks {
		name, outcome := c.outcome()
		switch outcome {
		case "pass":
			cr.Checks.Pass++
		case "fail":
			cr.Checks.Fail++
			failing = append(failing, name)
		default:
			cr.Checks.Pending++
		}
	}
	cr.Checks.Failing = sortedUnique(failing)
	return cr
}

// outcome maps a CheckRun or StatusContext to its name and pass, fail or pending.
func (c ghCheck) outcome() (name, outcome string) {
	if c.Typename == "StatusContext" {
		switch c.State {
		case "SUCCESS":
			return c.Context, "pass"
		case "FAILURE", "ERROR":
			return c.Context, "fail"
		}
		return c.Context, "pending"
	}
	if c.Status != "COMPLETED" {
		return c.Name, "pending"
	}
	switch c.Conclusion {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return c.Name, "pass"
	case "STALE", "ACTION_REQUIRED":
		return c.Name, "pending"
	}
	return c.Name, "fail"
}
