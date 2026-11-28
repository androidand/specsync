package specsync

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DefaultReviewerSource is the reviewer source for a repository: the GitHub
// provider, which asks gh.
func DefaultReviewerSource(repo string) ReviewerSource {
	return NewGitHubProviderFuncWithRepo(repo, runGH)
}

// ReviewerOptions says how reviewers are fetched. The offline and refresh rules
// are the ones of the request listing: Offline skips the provider and the cache,
// Refresh skips reading the cache but still stores the fresh data.
type ReviewerOptions struct {
	Source  func(repo string) ReviewerSource // nil: DefaultReviewerSource
	Cache   ReviewerCache                    // nil: no caching
	Refresh bool
	Offline bool
}

// AttachReviewers fills in who is asked to review each open request in rows,
// and each reviewer's verdict, with one provider call per repository that has
// such a request, naming exactly the requests shown. The requests are updated
// in place. It says in the returned notes what it could not do: a repository
// whose call failed, a request the provider did not return, and lists the
// provider cut short. What it could not fill stays ReviewersKnown false.
func AttachReviewers(ctx context.Context, rows []ChangeTopology, opts ReviewerOptions) []string {
	if opts.Offline {
		return nil
	}
	source := opts.Source
	if source == nil {
		source = DefaultReviewerSource
	}
	byRepo := map[string]map[int][]*ChangeRequest{} // repo key -> number -> requests
	spelling := map[string]string{}
	var repos []string
	seen := map[*ChangeRequest]bool{}
	for _, r := range rows {
		if r.PR == nil || r.PR.State != "open" || r.Repo == "" || seen[r.PR] {
			continue
		}
		seen[r.PR] = true
		key := strings.ToLower(r.Repo)
		if byRepo[key] == nil {
			byRepo[key] = map[int][]*ChangeRequest{}
			spelling[key] = r.Repo
			repos = append(repos, key)
		}
		byRepo[key][r.PR.Number] = append(byRepo[key][r.PR.Number], r.PR)
	}
	sort.Strings(repos)
	type result struct {
		by  map[int]RequestReviewers
		err error
	}
	results := make([]result, len(repos))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, key := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			repo, wanted := spelling[key], byRepo[key]
			if opts.Cache != nil && !opts.Refresh {
				if cached, ok := opts.Cache.Get(repo); ok && hasAll(cached, wanted) {
					results[i].by = cached
					return
				}
			}
			src := source(repo)
			if src == nil {
				results[i].err = fmt.Errorf("no reviewer source")
				return
			}
			numbers := make([]int, 0, len(wanted))
			for n := range wanted {
				numbers = append(numbers, n)
			}
			sort.Ints(numbers)
			results[i].by, results[i].err = src.Reviewers(ctx, numbers)
			if results[i].err == nil && opts.Cache != nil {
				opts.Cache.Put(repo, results[i].by)
			}
		}()
	}
	wg.Wait()
	var notes []string
	for i, key := range repos {
		repo := spelling[key]
		if results[i].err != nil {
			notes = append(notes, fmt.Sprintf("reviewers unavailable for %s: %s", repo, errReason(results[i].err)))
			continue
		}
		var numbers []int
		for n := range byRepo[key] {
			numbers = append(numbers, n)
		}
		sort.Ints(numbers)
		for _, n := range numbers {
			rr, ok := results[i].by[n]
			if !ok {
				notes = append(notes, fmt.Sprintf("reviewers unavailable for %s#%d: the provider did not return it", repo, n))
				continue
			}
			if rr.Incomplete {
				notes = append(notes, fmt.Sprintf("reviewers of %s#%d may be incomplete: the provider returned only part of the list", repo, n))
			}
			for _, cr := range byRepo[key][n] {
				cr.ReviewersKnown = true
				cr.ReviewRequests, cr.Reviews = rr.Requested, rr.Reviews
			}
		}
	}
	return notes
}

// hasAll reports whether cached holds an entry for every wanted request.
func hasAll(cached map[int]RequestReviewers, wanted map[int][]*ChangeRequest) bool {
	for n := range wanted {
		if _, ok := cached[n]; !ok {
			return false
		}
	}
	return true
}
