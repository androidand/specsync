package specsync

import (
	"context"
	"fmt"
	"strings"
)

// Options configures a sync run.
type Options struct {
	OpenSpecDir    string       // path to the openspec/ directory
	Provider       WorkProvider // target tracker
	Slug           string       // if set, only this change is synced
	DryRun         bool         // when true, never persist refs to the cache
	Reconcile      bool         // when true, merge issue checkbox state into tasks.md before pushing
	CloseCompleted bool         // when true, a change whose every task is checked projects as closed
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
}

// Sync projects every OpenSpec change into the provider, idempotently. It is
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
		ref, created, flips, err := syncOne(ctx, opts.Provider, c, opts.DryRun, opts.Reconcile, opts.CloseCompleted)
		if err != nil {
			return res, fmt.Errorf("sync %s: %w", c.Slug, err)
		}
		if created {
			res.Created++
		} else {
			res.Updated++
		}
		res.Items = append(res.Items, ItemResult{Slug: c.Slug, URL: ref.URL, Created: created, Flips: flips})
	}
	return res, nil
}

func syncOne(ctx context.Context, prov WorkProvider, c Change, dryRun, reconcile, closeCompleted bool) (ref Ref, created bool, flips []TaskFlip, err error) {
	refs, err := loadRefs(c.Dir)
	if err != nil {
		return Ref{}, false, nil, err
	}
	// Resolve by the canonical key, then fall back to the legacy bare "github"
	// key so an existing refs.json — written before the key was repo-qualified —
	// keeps updating its issue instead of creating a duplicate. saveRef below
	// persists under the canonical key, migrating the hit going forward.
	key := prov.Name()
	existing, hadRef := refs[key]
	if !hadRef && strings.HasPrefix(key, "github:") {
		existing, hadRef = refs["github"]
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
			return Ref{}, false, nil, rerr
		}
		existingPtr = resolved
		flips = f
	}
	refreshStage(&c)

	ref, err = prov.Push(ctx, WorkItemFor(c, closeCompleted), existingPtr)
	if err != nil {
		return Ref{}, false, nil, err
	}
	if dryRun {
		// A dry run must never mutate local state, or it poisons the cache with
		// a placeholder ref (e.g. issue #0) that breaks the next real run.
		return ref, !hadRef, nil, nil
	}
	if err := saveRef(c.Dir, prov.Name(), ref); err != nil {
		return Ref{}, false, nil, err
	}
	return ref, !hadRef, flips, nil
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
	return WorkItem{
		Slug:         c.Slug,
		Title:        c.Title,
		Body:         body,
		Stage:        c.Stage,
		Priority:     c.Priority,
		Closed:       c.Archived || (closeCompleted && c.Stage == StageComplete),
		ManageClosed: c.Archived || closeCompleted,
	}
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
