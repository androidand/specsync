package specsync

import (
	"context"
	"fmt"
	"strings"
)

// Options configures a sync run.
type Options struct {
	OpenSpecDir    string       // path to the spec root (openspec/, beads/, etc.)
	Provider       WorkProvider // target tracker
	Slug           string       // if set, only this change is synced
	DryRun         bool         // when true, never persist refs to the cache
	Reconcile      bool         // when true, merge issue checkbox state into tasks.md before pushing
	CloseCompleted bool         // when true, a change whose every task is checked projects as closed
	Project        BoardTarget  // optional GitHub Projects board; unset = no board operations
}

// Result reports what a sync run did.
type Result struct {
	Created int
	Updated int
	Items   []ItemResult
}

// ItemResult records the outcome for one change.
type ItemResult struct {
	Slug    string
	URL     string
	Created bool
	Flips   []TaskFlip // task states merged in from the issue (reconcile)
	// Board reports the board projection; BoardConfigured is false when no target
	// project was configured (in which case Board is zero and no board calls ran).
	BoardConfigured bool
	Board           BoardPlan
}

// Sync projects every change into the provider, idempotently. It is
// safe to run after any change moves through the funnel: existing projections
// are updated, new ones created.
func Sync(ctx context.Context, opts Options) (Result, error) {
	var res Result
	if opts.Provider == nil {
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
		ref, created, flips, plan, err := syncOne(ctx, opts.Provider, c, opts.DryRun, opts.Reconcile, opts.CloseCompleted, opts.Project)
		if err != nil {
			return res, fmt.Errorf("sync %s: %w", c.Slug, err)
		}
		if created {
			res.Created++
		} else {
			res.Updated++
		}
		res.Items = append(res.Items, ItemResult{
			Slug:            c.Slug,
			URL:             ref.URL,
			Created:         created,
			Flips:           flips,
			BoardConfigured: opts.Project.Configured(),
			Board:           plan,
		})
	}
	return res, nil
}

func syncOne(ctx context.Context, prov WorkProvider, c Change, dryRun, reconcile, closeCompleted bool, target BoardTarget) (ref Ref, created bool, flips []TaskFlip, plan BoardPlan, err error) {
	refs, err := loadRefs(c.Dir)
	if err != nil {
		return Ref{}, false, nil, BoardPlan{}, err
	}
	// Resolve by the canonical key, then fall back to the legacy bare "github"
	// key so an existing refs.json — written before the key was repo-qualified —
	// keeps updating its issue instead of creating a duplicate. The fallback only
	// applies when the cached URL points into the target repo: an unguarded hit
	// would make `-repo ownerB/repoB` edit whatever issue number the legacy entry
	// holds in the wrong repo. A rejected fallback degrades to no ref, and the
	// marker lookup in Push still rescues genuine matches. saveRef below persists
	// under the canonical key, migrating the hit going forward.
	key := prov.Name()
	existing, hadRef := refs[key]
	if !hadRef && strings.HasPrefix(key, "github:") {
		if legacy, ok := refs["github"]; ok && legacyRefMatchesRepo(legacy, strings.TrimPrefix(key, "github:")) {
			existing, hadRef = legacy, true
		}
	}
	var existingPtr *Ref
	if hadRef {
		existingPtr = &existing
	}

	// Inbound half of two-way sync: merge issue checkbox state into tasks.md
	// before rendering. Skipped on dry-run to honor the zero-API-call contract
	// (the dry-runner has no real issue to read). Reusing the resolved ref for
	// the push avoids a second marker lookup.
	if reconcile && !dryRun {
		resolved, f, rerr := reconcileTaskState(ctx, prov, &c, existingPtr)
		if rerr != nil {
			return Ref{}, false, nil, BoardPlan{}, rerr
		}
		existingPtr = resolved
		flips = f
	}
	if err := refreshState(&c); err != nil {
		return Ref{}, false, nil, BoardPlan{}, fmt.Errorf("refresh state after reconcile: %w", err)
	}

	item := WorkItemFor(c, closeCompleted)
	ref, err = prov.Push(ctx, item, existingPtr)
	if err != nil {
		return Ref{}, false, nil, BoardPlan{}, err
	}

	// Project onto the board only when a target is configured and the provider
	// supports it; otherwise no board call is made (backward-compatible). The
	// projector honors dryRun internally (zero board calls, plan only).
	// Before pushing, use three-way merge to detect human board edits and prevent clobbering.
	if target.Configured() {
		if bp, ok := prov.(BoardProjector); ok {
			// Load board state for three-way merge (human-move detection).
			boardState, err := LoadBoardState(c.Dir)
			if err != nil {
				return Ref{}, false, nil, BoardPlan{}, fmt.Errorf("load board state: %w", err)
			}

			// If we have a prior binding, use three-way merge to detect changes.
			bindingKey := fmt.Sprintf("%s:%d:%s", target.Owner, target.Number, prov.Name())
			if binding, ok := boardState.Bindings[bindingKey]; ok && !dryRun {
				// Get current remote state from board (perform board query).
				// We query just to get the current status, then decide via three-way merge.
				// For now, ProjectOntoBoard will still query; we just save state after.
				// Future: optimize to avoid double-query by passing remote state to projector.
				decision := threeWayMerge(item.Stage, binding.RemoteOptionIDBase, binding)
				if decision.Action == "report-remote-move" {
					// Human moved the card; respect it, don't clobber.
					plan = BoardPlan{
						ProjectID:     binding.ProjectID,
						StatusField:   "Status",
						StatusSkipped: decision.Reason,
					}
					// Skip the actual ProjectOntoBoard call; the decision stands.
				} else if decision.Action == "report-conflict" {
					// Both sides changed; report for manual review.
					plan = BoardPlan{
						ProjectID:     binding.ProjectID,
						StatusField:   "Status",
						StatusSkipped: decision.Reason,
					}
				} else {
					// "push-local" or "none": proceed with normal push.
					plan, err = bp.ProjectOntoBoard(ctx, target, ref, item, dryRun)
					if err != nil {
						return Ref{}, false, nil, BoardPlan{}, err
					}
				}
			} else {
				// First sync or dry-run: proceed normally.
				plan, err = bp.ProjectOntoBoard(ctx, target, ref, item, dryRun)
				if err != nil {
					return Ref{}, false, nil, BoardPlan{}, err
				}
			}
		}
	}

	if dryRun {
		// A dry run must never mutate local state, or it poisons the cache with
		// a placeholder ref (e.g. issue #0) that breaks the next real run.
		return ref, !hadRef, nil, plan, nil
	}
	if err := saveRef(c.Dir, prov.Name(), ref); err != nil {
		return Ref{}, false, nil, BoardPlan{}, err
	}

	// After successful board projection, save the binding for future three-way merge.
	if target.Configured() && plan.ProjectID != "" {
		if err := saveBoardBinding(c.Dir, target, prov.Name(), item.Stage, plan); err != nil {
			return Ref{}, false, nil, BoardPlan{}, fmt.Errorf("save board binding: %w", err)
		}
	}

	return ref, !hadRef, flips, plan, nil
}

// WorkItemFor renders a Change into the provider-agnostic WorkItem. tasks.md
// is folded in as a checklist; links.md becomes a ## Related section using
// "[owner/repo#N](url)" GitHub autolink format. When closeCompleted is set, a
// change in the complete stage (every task checked, not yet archived) also
// projects as closed, so finishing the last task can retire the issue.
func WorkItemFor(c Change, closeCompleted bool) WorkItem {
	body := c.Body
	if strings.TrimSpace(c.TasksMarkdown) != "" {
		body = body + "\n\n## Tasks\n\n" + c.TasksMarkdown
	}
	if len(c.Links) > 0 {
		var lines []string
		for _, ref := range c.Links {
			lines = append(lines, "- "+refLabel(ref))
		}
		if len(lines) > 0 {
			body = body + "\n\n## Related\n\n" + strings.Join(lines, "\n")
		}
	}
	priority := 0
	if c.Priority != nil {
		priority = *c.Priority
	}
	return WorkItem{
		Slug:         c.Slug,
		Title:        shortenTitle(c.Title, 80),
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
