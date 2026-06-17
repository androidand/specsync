package specsync

import (
	"context"
	"fmt"
	"strings"
)

// Options configures a sync run.
type Options struct {
	OpenSpecDir string       // path to the openspec/ directory
	Provider    WorkProvider // target tracker
	Slug        string       // if set, only this change is synced
	DryRun      bool         // when true, never persist refs to the cache
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
		ref, created, err := syncOne(ctx, opts.Provider, c, opts.DryRun)
		if err != nil {
			return res, fmt.Errorf("sync %s: %w", c.Slug, err)
		}
		if created {
			res.Created++
		} else {
			res.Updated++
		}
		res.Items = append(res.Items, ItemResult{Slug: c.Slug, URL: ref.URL, Created: created})
	}
	return res, nil
}

func syncOne(ctx context.Context, prov WorkProvider, c Change, dryRun bool) (ref Ref, created bool, err error) {
	refs, err := loadRefs(c.Dir)
	if err != nil {
		return Ref{}, false, err
	}
	existing, hadRef := refs[prov.Name()]
	var existingPtr *Ref
	if hadRef {
		existingPtr = &existing
	}

	ref, err = prov.Push(ctx, workItemFor(c), existingPtr)
	if err != nil {
		return Ref{}, false, err
	}
	if dryRun {
		// A dry run must never mutate local state, or it poisons the cache with
		// a placeholder ref (e.g. issue #0) that breaks the next real run.
		return ref, !hadRef, nil
	}
	if err := saveRef(c.Dir, prov.Name(), ref); err != nil {
		return Ref{}, false, err
	}
	return ref, !hadRef, nil
}

// workItemFor renders a Change into the provider-agnostic WorkItem. tasks.md is
// folded into the body as a checklist so providers without sub-issues still
// show task progress.
func workItemFor(c Change) WorkItem {
	body := c.Body
	if strings.TrimSpace(c.TasksMarkdown) != "" {
		body = body + "\n\n## Tasks\n\n" + c.TasksMarkdown
	}
	return WorkItem{
		Slug:     c.Slug,
		Title:    c.Title,
		Body:     body,
		Stage:    c.Stage,
		Priority: c.Priority,
		Closed:   c.Archived,
	}
}
