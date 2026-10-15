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
	Number         int               `json:"number"`
	URL            string            `json:"url"`
	State          string            `json:"state"`
	IsDraft        bool              `json:"isDraft"`
	HeadRefName    string            `json:"headRefName"`
	BaseRefName    string            `json:"baseRefName"`
	ReviewDecision string            `json:"reviewDecision"`
	Mergeable      string            `json:"mergeable"`
	Additions      int               `json:"additions"`
	Deletions      int               `json:"deletions"`
	ChangedFiles   int               `json:"changedFiles"`
	Reviews        []json.RawMessage `json:"reviews"`
	Comments       []json.RawMessage `json:"comments"`
	Closing        []struct {
		Number int `json:"number"`
	} `json:"closingIssuesReferences"`
	Checks []ghCheck `json:"statusCheckRollup"`
}

// ChangeRequests lists the repo's pull requests, all states, with one gh call.
func (p *GitHubProvider) ChangeRequests(ctx context.Context) (ChangeRequestSet, error) {
	limit := p.changeRequestLimit
	if limit == 0 {
		limit = defaultChangeRequestLimit
	}
	args := append([]string{"pr", "list"}, p.repoFlag()...)
	args = append(args, "--state", "all", "--limit", strconv.Itoa(limit), "--json",
		"number,url,state,isDraft,headRefName,baseRefName,reviewDecision,mergeable,"+
			"additions,deletions,changedFiles,reviews,comments,closingIssuesReferences,statusCheckRollup")
	out, err := p.run(ctx, args...)
	if err != nil {
		return ChangeRequestSet{}, err
	}
	var raw []ghChangeRequest
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return ChangeRequestSet{}, fmt.Errorf("parse gh pr list: %w", err)
	}
	set := ChangeRequestSet{Truncated: len(raw) >= limit}
	anyOpen := false
	for _, r := range raw {
		cr := r.toChangeRequest()
		anyOpen = anyOpen || cr.State == "open"
		set.Requests = append(set.Requests, cr)
	}
	if anyOpen {
		// Thread counts are a nice-to-have: GraphQL only, and a failure leaves
		// them unknown instead of failing the listing.
		if counts, err := p.unresolvedThreads(ctx); err == nil {
			for i := range set.Requests {
				if n, ok := counts[set.Requests[i].Number]; ok && set.Requests[i].State == "open" {
					n := n
					set.Requests[i].UnresolvedThreads = &n
				}
			}
		}
	}
	return set, nil
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
	out, err := p.run(ctx, "api", "graphql", "-f", "query="+unresolvedThreadsQuery, "-F", "owner="+owner, "-F", "name="+name)
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
		State:          lower(r.State),
		Draft:          r.IsDraft,
		Branch:         r.HeadRefName,
		Base:           r.BaseRefName,
		ReviewDecision: lower(r.ReviewDecision),
		Mergeable:      lower(r.Mergeable),
		Size:           ChangeRequestSize{Files: r.ChangedFiles, Additions: r.Additions, Deletions: r.Deletions},
		ReviewStarted:  len(r.Reviews)+len(r.Comments) > 0,
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
	}
	return c.Name, "fail"
}
