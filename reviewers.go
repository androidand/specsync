package specsync

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// DefaultReviewerSource is the reviewer source for a repository: the GitHub
// provider, which asks gh.
func DefaultReviewerSource(repo string) ReviewerSource {
	return NewGitHubProviderFuncWithRepo(repo, runGH)
}

// AttachReviewers fills in who is asked to review each open request in rows,
// and each reviewer's verdict, with one provider call per repository that has
// such a request. Only the rows given are looked at, so the cost follows what
// is shown. The requests are updated in place. A repository whose call fails
// leaves its requests with ReviewersKnown false, and a status line says so.
func AttachReviewers(ctx context.Context, rows []ChangeTopology, source func(repo string) ReviewerSource) []string {
	if source == nil {
		source = DefaultReviewerSource
	}
	byRepo := map[string][]*ChangeRequest{}
	var repos []string
	seen := map[*ChangeRequest]bool{}
	for _, r := range rows {
		if r.PR == nil || r.PR.State != "open" || r.Repo == "" || seen[r.PR] {
			continue
		}
		seen[r.PR] = true
		key := strings.ToLower(r.Repo)
		if _, ok := byRepo[key]; !ok {
			repos = append(repos, r.Repo)
		}
		byRepo[key] = append(byRepo[key], r.PR)
	}
	type result struct {
		by  map[int]RequestReviewers
		err error
	}
	results := make([]result, len(repos))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			src := source(repo)
			if src == nil {
				results[i].err = fmt.Errorf("no reviewer source")
				return
			}
			results[i].by, results[i].err = src.Reviewers(ctx)
		}()
	}
	wg.Wait()
	var status []string
	for i, repo := range repos {
		if results[i].err != nil {
			status = append(status, fmt.Sprintf("reviewers unavailable for %s: %s", repo, errReason(results[i].err)))
			continue
		}
		for _, cr := range byRepo[strings.ToLower(repo)] {
			if rr, ok := results[i].by[cr.Number]; ok {
				cr.ReviewersKnown = true
				cr.ReviewRequests, cr.Reviews = rr.Requested, rr.Reviews
			}
		}
	}
	return status
}
