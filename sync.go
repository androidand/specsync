package specsync

import (
	"context"
	"fmt"
	"strings"
)

// Options configures a sync run.
type Options struct {
	OpenSpecDir    string        // path to the spec root (openspec/, beads/, etc.)
	Provider       WorkProvider  // target tracker (deprecated: use Providers)
	Providers      []WorkProvider // set of providers to fan-out to; Provider is used when Providers is nil
	Slug           string        // if set, only this change is synced
	DryRun         bool          // when true, never persist refs to the cache
	Reconcile      bool          // when true, merge issue checkbox state into tasks.md before pushing
	CloseCompleted bool          // when true, a change whose every task is checked projects as closed
	Project BoardTarget // optional GitHub Projects board; unset = no board operations
	Linker         Linker        // optional linker to resolve issue refs; nil = cache-only
}

// Result reports what a sync run did.
type Result struct {
	Created int
	Updated int
	Items   []ItemResult
}

// ProviderResult records the outcome for one provider on one change.
type ProviderResult struct {
	ProviderName string
	Slug         string
	URL          string
	Created      bool
	Flips        []TaskFlip
	Error        error
}

// ItemResult records the outcome for one change.
type ItemResult struct {
	Slug    string
	URL     string
	Created bool
	Flips   []TaskFlip // task states merged in from the issue (reconcile)
	// TitleSuggestion is a tighter variant of the change's title, set only
	// when shortenTitle would modify it. Sync always pushes the proposal H1
	// verbatim — the title is the author's content, not specsync's to edit —
	// but surfaces the suggestion so the author can shorten it at the source.
	TitleSuggestion string
	// Board reports the board projection; BoardConfigured is false when no target
	// project was configured (in which case Board is zero and no board calls ran).
	BoardConfigured bool
	Board           BoardPlan
	// Providers holds per-provider results when fan-out is used.
	Providers []ProviderResult
}

// Sync projects every change into the provider(s), idempotently. When
// Providers is set, it fans out to each provider independently (star model:
// OpenSpec ↔ each provider, never provider ↔ provider). When Providers is
// nil, Provider is used for backward compatibility.
func Sync(ctx context.Context, opts Options) (Result, error) {
	var res Result

	// Resolve provider set.
	var providers []WorkProvider
	if len(opts.Providers) > 0 {
		providers = opts.Providers
	} else if opts.Provider != nil {
		providers = []WorkProvider{opts.Provider}
	} else {
		return res, fmt.Errorf("provider is required")
	}

	changes, err := LoadChanges(opts.OpenSpecDir)
	if err != nil {
		return res, err
	}

	for _, c := range changes {
		if opts.Slug != "" && c.Slug != opts.Slug {
			continue
		}

		// Try the Linker first to resolve an issue ref for this change.
		// This is used when the change has no cache entry but can be linked
		// via branch name or other discovery strategy.
		var linkerResult *LinkerResult
		if opts.Linker != nil {
			result, err := opts.Linker.Resolve(ctx, c.Dir)
			if err != nil {
				return res, fmt.Errorf("linker resolve %s: %w", c.Slug, err)
			}
			if result != nil && result.Ref != nil {
				linkerResult = result
			}
		}

		var allFlips []TaskFlip
		var firstRef Ref
		var firstCreated bool
		providerResults := make([]ProviderResult, 0, len(providers))

		for _, prov := range providers {
			refs, err := loadRefs(c.Dir)
			if err != nil {
				providerResults = append(providerResults, ProviderResult{
					ProviderName: prov.Name(),
					Slug:         c.Slug,
					Error:        err,
				})
				continue
			}

			key := prov.Name()
			existing, hadRef := refs[key]
			if !hadRef && strings.HasPrefix(key, "github:") {
				if legacy, lok := refs["github"]; lok {
					repo := strings.TrimPrefix(key, "github:")
					if legacyRefMatchesRepo(legacy, repo) {
						existing, hadRef = legacy, true
					}
				}
			}
			var existingPtr *Ref
			if hadRef {
				existingPtr = &existing
			} else if linkerResult != nil && linkerResult.Ref.Provider == key {
				// Linker found a ref but the cache doesn't have one.
				// Only use it if the provider matches — a GitHub ref from
				// the Linker should not be used as a fallback for Beads.
				existingPtr = linkerResult.Ref
			}

		// For providers with an existing issue: reconcile inbound before
			// rendering, so the push carries the merged state.
			if opts.Reconcile && !opts.DryRun && existingPtr != nil {
				resolved, flips, rerr := reconcileTaskState(ctx, prov, &c, existingPtr)
				if rerr != nil {
					providerResults = append(providerResults, ProviderResult{
						ProviderName: prov.Name(),
						Slug:         c.Slug,
						Error:        rerr,
					})
					continue
				}
				existingPtr = resolved
				allFlips = append(allFlips, flips...)
			}

			if err := refreshState(&c); err != nil {
				return res, fmt.Errorf("refresh state: %w", err)
			}

			// Refuse to write to a fork's upstream parent without explicit -repo consent.
			if gp, ok := prov.(*GitHubProvider); ok {
				if refuse, reason, _ := ForkRefusal(ctx, gp.resolved); refuse {
					providerResults = append(providerResults, ProviderResult{
						ProviderName: prov.Name(),
						Slug:         c.Slug,
						Error:        fmt.Errorf("fork refusal: %s", reason),
					})
					continue
				}
			}

			item := WorkItemFor(c, opts.CloseCompleted)
			ref, perr := prov.Push(ctx, item, existingPtr)
			if perr != nil {
				providerResults = append(providerResults, ProviderResult{
					ProviderName: prov.Name(),
					Slug:         c.Slug,
					Error:        perr,
				})
				continue
			}

			created := !hadRef
			if firstRef.URL == "" {
				firstRef = ref
				firstCreated = created
			}

			if opts.DryRun {
				// A dry run must never mutate local state.
			} else {
				// Preserve base state from the reconciled ref so saveRef
				// doesn't overwrite it with a fresh provider ref.
				if existingPtr != nil && existingPtr.Base != "" && ref.Base == "" {
					ref.BaseSHA = existingPtr.BaseSHA
					ref.Base = existingPtr.Base
				}
				if err := saveRef(c.Dir, key, ref); err != nil {
					providerResults = append(providerResults, ProviderResult{
						ProviderName: prov.Name(),
						Slug:         c.Slug,
						Error:        err,
					})
					continue
				}
			}

			providerResults = append(providerResults, ProviderResult{
				ProviderName: prov.Name(),
				Slug:         c.Slug,
				URL:          ref.URL,
				Created:      created,
			})

			if created {
				res.Created++
			} else {
				res.Updated++
			}

			// For providers that just got created: reconcile inbound after the
			// push, because the issue now exists and may have state to merge
			// (e.g., Beads children already closed before the epic was created).
			// For providers with an existing ref: the pre-push reconcile above
			// already handled this (no double-reconcile).
			if created && opts.Reconcile && !opts.DryRun {
				_, flips, rerr := reconcileTaskState(ctx, prov, &c, &ref)
				if rerr != nil {
					continue
				}
				allFlips = append(allFlips, flips...)
				if err := refreshState(&c); err != nil {
					return res, fmt.Errorf("refresh state after post-push reconcile: %w", err)
				}
			}
		}

		// No suggestion for archived changes.
		var suggestion string
		if !c.Archived {
			suggestion = titleSuggestion(c.Title)
		}

		res.Items = append(res.Items, ItemResult{
			Slug:            c.Slug,
			URL:             firstRef.URL,
			Created:         firstCreated,
			Flips:           allFlips,
			TitleSuggestion: suggestion,
			BoardConfigured: opts.Project.Configured(),
			Providers:       providerResults,
		})
	}

	return res, nil
}

// WorkItemFor renders a Change into the provider-agnostic WorkItem. tasks.md
// is folded in as a checklist; links.md becomes a ## Related section using
// "[owner/repo#N](url)" GitHub autolink format. When closeCompleted is set, a
// change in the complete stage (every task checked, not yet archived) also
// projects as closed, so finishing the last task can retire the issue.
func WorkItemFor(c Change, closeCompleted bool) WorkItem {
	body := c.Body
	if strings.TrimSpace(c.OriginalAsk) != "" {
		body = body + "\n\n## Original ask\n\n" + c.OriginalAsk
	}
	if strings.TrimSpace(c.Discoveries) != "" {
		body = body + "\n\n## Discoveries\n\n" + c.Discoveries
	}
	if strings.TrimSpace(c.TasksMarkdown) != "" {
		body = body + "\n\n## Tasks\n\n" + c.TasksMarkdown
		// Plan changes footer
		tc := countTaskStates(c.TasksMarkdown)
		if tc.Total() > 0 {
			parts := []string{}
			if c.BaselineTasks != nil && tc.Total() > *c.BaselineTasks {
				added := tc.Total() - *c.BaselineTasks
				parts = append(parts, fmt.Sprintf("+%d added", added))
			}
			if tc.Done > 0 {
				parts = append(parts, fmt.Sprintf("%d done", tc.Done))
			}
			if tc.Dropped > 0 {
				parts = append(parts, fmt.Sprintf("%d dropped", tc.Dropped))
			}
			if tc.Moved > 0 {
				parts = append(parts, fmt.Sprintf("%d moved", tc.Moved))
			}
			if len(parts) > 0 {
				body = body + "\n\n## Plan changes\n\n" + strings.Join(parts, " · ")
			}
		}
	}
	if len(c.Links) > 0 {
		body = UpsertRelatedSection(body, c.Links)
	}
	priority := 0
	if c.Priority != nil {
		priority = *c.Priority
	}
	return WorkItem{
		Slug:         c.Slug,
		Title:        c.Title,
		Body:         body,
		Stage:        c.Stage,
		Priority:     priority,
		Closed:       c.Archived || (closeCompleted && c.Stage == StageComplete),
		ManageClosed: c.Archived || closeCompleted,
	}
}

// legacyRefMatchesRepo reports whether a legacy bare-"github" cache entry
// belongs to the given "owner/repo", by parsing the owner/repo out of its
// issue URL. Unparseable URLs never match: the entry may point anywhere, so
// it must not be edited under a repo-qualified key.
func legacyRefMatchesRepo(ref Ref, repo string) bool {
	r, ok := ghIssueRepo(ref.URL)
	return ok && strings.EqualFold(r, repo)
}

// ghIssueRepo extracts "owner/repo" from a GitHub issue URL. ok is false for
// anything that isn't a github.com URL with an owner, repo, and further path.
func ghIssueRepo(url string) (repo string, ok bool) {
	const prefix = "https://github.com/"
	if !strings.HasPrefix(url, prefix) {
		return "", false
	}
	parts := strings.SplitN(url[len(prefix):], "/", 3)
	if len(parts) < 3 {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

// refLabel returns "[owner/repo#N](url)" for GitHub issue URLs so GitHub
// renders them as rich cross-references. Falls back to bare URL otherwise.
func refLabel(ref Ref) string {
	const prefix = "https://github.com/"
	if strings.HasPrefix(ref.URL, prefix) {
		rest := ref.URL[len(prefix):]
		parts := strings.SplitN(rest, "/", 4)
		if len(parts) == 4 && parts[2] == "issues" {
			short := parts[0] + "/" + parts[1] + "#" + parts[3]
			return fmt.Sprintf("[%s](%s)", short, ref.URL)
		}
	}
	return ref.URL
}
