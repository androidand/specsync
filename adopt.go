package specsync

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// AdoptOptions configures binding an existing local change to an existing
// tracker issue — the missing third verb alongside pull (issue → change) and
// sync (change → issue). adopt declares a link that exists nowhere; unlike
// pull it never overwrites the change, and unlike sync it never creates an
// issue.
type AdoptOptions struct {
	OpenSpecDir string       // path to the spec root (openspec/, beads/, etc.)
	Provider    WorkProvider // must implement IssueReader; IssueMarkerWriter used when present
	IssueID     string       // provider id of the issue to adopt (required)
	Slug        string       // change slug; resolved from the branch name when empty
	DryRun      bool         // when true, write nothing
	Force       bool         // rebind despite an existing, conflicting binding
}

// AdoptResult reports what adopt did (or would do on a dry run).
type AdoptResult struct {
	Slug        string
	Dir         string
	IssueID     string
	IssueURL    string
	MarkerAdded bool // whether the marker write happened (or would, on dry-run)
}

// adoptBranchPattern mirrors BranchResolver's default: issue-linked branches
// as "feat/<n>-<slug>" or "fix/<n>-<slug>".
var adoptBranchPattern = regexp.MustCompile(`^(?:feat|fix)/\d+-(.+)$`)

// resolveAdoptSlug derives the change slug from the current branch name, the
// same convention sync's branch-based Linker resolves issues from. adopt
// binds exactly one change, so — unlike sync's "-change omitted means every
// change" — an unresolvable branch is an error, not "adopt everything".
func resolveAdoptSlug() (string, error) {
	branch, err := currentBranch()
	if err != nil || branch == "" {
		return "", fmt.Errorf("-change is required (could not resolve a change from the current branch)")
	}
	m := adoptBranchPattern.FindStringSubmatch(branch)
	if m == nil {
		return "", fmt.Errorf("-change is required (branch %q doesn't match the issue-linked pattern feat/<n>-<slug> or fix/<n>-<slug>)", branch)
	}
	return m[1], nil
}

// Adopt binds an existing change to an existing issue: it writes
// .specsync/refs.json *before* touching the issue body, so the link holds
// immediately and does not depend on marker-search indexing catching up (the
// race that produced FusionHub's duplicate #3692). The marker is then added
// to the issue body for durability and for other worktrees/clones.
func Adopt(ctx context.Context, opts AdoptOptions) (AdoptResult, error) {
	if opts.Provider == nil {
		return AdoptResult{}, fmt.Errorf("provider is required")
	}
	issueID := strings.TrimSpace(opts.IssueID)
	if issueID == "" {
		return AdoptResult{}, fmt.Errorf("-issue is required")
	}
	reader, ok := opts.Provider.(IssueReader)
	if !ok {
		return AdoptResult{}, fmt.Errorf("provider %q cannot read issues", opts.Provider.Name())
	}

	slug := strings.TrimSpace(opts.Slug)
	if slug == "" {
		resolved, err := resolveAdoptSlug()
		if err != nil {
			return AdoptResult{}, err
		}
		slug = resolved
	}

	c, err := LoadChangeBySlug(opts.OpenSpecDir, slug)
	if err != nil {
		return AdoptResult{}, err
	}

	item, err := reader.Get(ctx, issueID)
	if err != nil {
		return AdoptResult{}, fmt.Errorf("fetch issue %s: %w", issueID, err)
	}

	// Refuse when the issue already carries another change's marker, unless
	// -force. Adopting it anyway would silently steal a binding that some
	// other change depends on.
	if owner := slugFromMarker(item.Body); owner != "" && owner != slug && !opts.Force {
		return AdoptResult{}, fmt.Errorf("issue %s already carries the marker for change %q; pass -force to rebind it to %q", item.URL, owner, slug)
	}

	// Refuse when the change is already bound to a different issue, unless
	// -force.
	key := opts.Provider.Name()
	refs, err := LoadRefs(c.Dir)
	if err != nil {
		return AdoptResult{}, err
	}
	if existing, ok := refs[key]; ok && existing.ID != item.ID && !opts.Force {
		return AdoptResult{}, fmt.Errorf("change %q is already bound to %s; pass -force to rebind it to %s", slug, existing.URL, item.URL)
	}

	res := AdoptResult{Slug: slug, Dir: c.Dir, IssueID: item.ID, IssueURL: item.URL}
	markerPresent := strings.Contains(item.Body, marker(slug))

	if opts.DryRun {
		res.MarkerAdded = !markerPresent
		return res, nil
	}

	// Write the ref cache first: the link holds immediately, independent of
	// whatever the marker search would find. Carry forward any prior merge
	// bases for this exact issue so a rebind onto the same issue (e.g. a
	// -force no-op) doesn't reset three-way-merge state.
	ref := Ref{Provider: key, ID: item.ID, URL: item.URL}
	if prior, ok := refs[key]; ok && prior.ID == item.ID {
		ref.Base, ref.BaseSHA = prior.Base, prior.BaseSHA
		ref.BaseClosed = prior.BaseClosed
	}
	if err := saveRef(c.Dir, key, ref); err != nil {
		return AdoptResult{}, err
	}

	if mw, ok := opts.Provider.(IssueMarkerWriter); ok {
		added, err := mw.EnsureMarker(ctx, item.ID, slug, item.Body)
		if err != nil {
			return AdoptResult{}, fmt.Errorf("persist identity marker: %w", err)
		}
		res.MarkerAdded = added
	}

	return res, nil
}
