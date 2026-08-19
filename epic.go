package specsync

import (
	"context"
	"fmt"
	"strings"
)

// EpicOptions configures a specsync epic run: mint (or converge onto) a
// coordination epic issue and wire every child to it.
type EpicOptions struct {
	OpenSpecDir string
	Title       string
	Repo        string   // resolves bare "#N" children; unrelated to where a slug child's own issue lives
	Children    []string // raw --child arguments: local change slugs or issue references

	// DryRun must mirror the CLI's --dry-run: it is threaded into the Sync
	// call made for an unsynced slug child so that path never persists a ref
	// to disk on a dry run, exactly like every other dry-run path in this
	// codebase. It does not affect EpicProvider/ChildProviderFor directly —
	// the caller is responsible for substituting dry-run (fake-gh) providers
	// there, the same way runSync/runArchive do.
	DryRun bool

	// EpicProvider projects the epic issue itself, already targeting Repo (or
	// its auto-detected default) — built by the caller the same way every
	// other subcommand builds its GitHub provider.
	EpicProvider WorkProvider

	// ChildProviderFor returns the WorkProvider for a child's repo: "" for a
	// local slug child (synced against the caller's own default/auto-detected
	// repo), or "owner/name" for a cross-repo issue reference. May be called
	// more than once for the same repo; callers that shell out to resolve a
	// provider may want to cache by repo themselves.
	ChildProviderFor func(repo string) WorkProvider
}

// EpicChildResult describes one child after being wired to the epic.
type EpicChildResult struct {
	Arg    string // the raw --child argument, for reporting
	Kind   string // "slug" or "issue"
	Slug   string // set for slug children
	Repo   string // "owner/name", resolved
	Ref    Ref
	Synced bool // true if a slug child had no ref yet and was synced by this run
}

// EpicResult reports what an epic run did.
type EpicResult struct {
	Ref      Ref
	Created  bool
	Children []EpicChildResult
}

// SubIssueAttacher is an optional, type-asserted provider capability for
// attaching native GitHub sub-issues to a parent issue. It is reserved for
// epic-and-subissue-projection: once a provider implements it, Epic attaches
// children as native sub-issues instead of degrading to a managed
// "## Related" cross-link. No provider implements it yet, so Epic always
// takes the degraded path today — this seam exists so that capability can
// slot in later without restructuring the caller.
type SubIssueAttacher interface {
	AttachSubIssue(ctx context.Context, parent, child Ref) error
}

// Epic creates or converges a `type:epic` coordination issue and wires every
// child (a local change slug or an existing issue reference, possibly
// cross-repo) to it. Identity is keyed by an "epic:"-prefixed slug namespace
// derived from the normalized title, so re-running with the same title
// converges onto the same issue via the existing Push/Find/marker machinery
// (see design.md) instead of creating a duplicate.
func Epic(ctx context.Context, opts EpicOptions) (*EpicResult, error) {
	if strings.TrimSpace(opts.Title) == "" {
		return nil, fmt.Errorf("epic: title is required")
	}
	if opts.EpicProvider == nil {
		return nil, fmt.Errorf("epic: EpicProvider is required")
	}
	if opts.ChildProviderFor == nil {
		return nil, fmt.Errorf("epic: ChildProviderFor is required")
	}

	epicSlug := "epic:" + slugify(opts.Title)

	// Classify and resolve every child before touching the epic issue, so a
	// bad --child argument fails loud before any GitHub write happens.
	children := make([]EpicChildResult, 0, len(opts.Children))
	for _, arg := range opts.Children {
		e, synced, err := classifyChild(ctx, arg, opts.OpenSpecDir, opts.Repo, opts.DryRun, opts.ChildProviderFor)
		if err != nil {
			return nil, fmt.Errorf("--child %q: %w", arg, err)
		}
		kind := "issue"
		if e.kind == kindSlug {
			kind = "slug"
		}
		children = append(children, EpicChildResult{
			Arg:    arg,
			Kind:   kind,
			Slug:   e.slug,
			Repo:   repoFromKey(e.ref.Provider),
			Ref:    e.ref,
			Synced: synced,
		})
	}

	childRefs := make([]Ref, 0, len(children))
	for _, c := range children {
		childRefs = append(childRefs, c.Ref)
	}

	// Mint or converge the epic issue via the unchanged Push/Find/marker
	// machinery: the "epic:"-prefixed slug namespace does the work. There is
	// no local ref cache for an epic (it has no change directory), so this
	// always resolves via Find rather than a refs.json read.
	epicRef, err := findWithRetry(ctx, func(ctx context.Context) (*Ref, error) {
		return opts.EpicProvider.Find(ctx, epicSlug)
	})
	if err != nil {
		return nil, fmt.Errorf("find existing epic: %w", err)
	}

	body := UpsertRelatedSection(epicBaseBody(opts.Title), childRefs)
	pushedRef, err := opts.EpicProvider.Push(ctx, WorkItem{
		Slug:   epicSlug,
		Title:  opts.Title,
		Body:   body,
		Labels: []string{"specsync", "type:epic"},
	}, epicRef)
	if err != nil {
		return nil, fmt.Errorf("push epic issue: %w", err)
	}

	result := &EpicResult{Ref: pushedRef, Created: epicRef == nil, Children: children}

	// Wire each child back to the epic: native sub-issues once a provider
	// implements SubIssueAttacher (epic-and-subissue-projection), degrading
	// to a managed "## Related" backlink otherwise.
	for i, c := range children {
		childProvider := opts.ChildProviderFor(c.Repo)
		if attacher, ok := childProvider.(SubIssueAttacher); ok {
			if err := attacher.AttachSubIssue(ctx, pushedRef, c.Ref); err != nil {
				return nil, fmt.Errorf("attach sub-issue %s: %w", c.Ref.URL, err)
			}
			continue
		}
		updated, err := PushRelatedEdit(ctx, childProvider, c.Ref, c.Slug, []Ref{pushedRef})
		if err != nil {
			return nil, fmt.Errorf("wire child %s to epic: %w", c.Ref.URL, err)
		}
		result.Children[i].Ref = updated
	}

	return result, nil
}

// classifyChild classifies one --child argument. A local change slug with no
// synced ref yet is synced first (via Sync, honoring dryRun) instead of
// erroring the way classifyArg does on its own — the automatic counterpart of
// classifyArg's "has no synced ref" error, which link.go's runLink instead
// surfaces to the user directly. providerFor("") builds the provider for the
// caller's own default/auto-detected repo: repo (the epic's own --repo) only
// governs bare "#N" resolution and is unrelated to where a local slug's own
// issue lives.
func classifyChild(ctx context.Context, arg, openspecDir, repo string, dryRun bool, providerFor func(string) WorkProvider) (linkEntry, bool, error) {
	c, err := LoadChangeBySlug(openspecDir, arg)
	if err != nil {
		// Not a local slug — classify as an issue reference (or fail loud).
		e, err := classifyArg(arg, openspecDir, repo)
		return e, false, err
	}

	refs, err := LoadRefs(c.Dir)
	if err != nil {
		return linkEntry{}, false, err
	}
	if len(refs) > 0 {
		e, err := classifyArg(arg, openspecDir, repo)
		return e, false, err
	}

	// No ref yet: sync it first, then treat the resulting ref as the child to
	// relate — matching runLink's slug path (specsync.Sync), except automatic
	// instead of requiring the operator to sync it themselves beforehand.
	prov := providerFor("")
	syncRes, err := Sync(ctx, Options{
		OpenSpecDir: openspecDir,
		Provider:    prov,
		Slug:        c.Slug,
		DryRun:      dryRun,
	})
	if err != nil {
		return linkEntry{}, false, fmt.Errorf("sync %q before attaching to epic: %w", arg, err)
	}
	if len(syncRes.Items) == 0 || syncRes.Items[0].URL == "" {
		return linkEntry{}, false, fmt.Errorf("sync %q produced no issue", arg)
	}
	url := syncRes.Items[0].URL
	ref := Ref{Provider: prov.Name(), ID: numberFromURL(url), URL: url}
	return linkEntry{kind: kindSlug, ref: ref, slug: c.Slug, slugDir: c.Dir}, true, nil
}

// epicBaseBody is the fixed prose portion of an epic's roll-up body — the
// part before the managed "## Related" section that lists its children.
// There is no proposal.md for an epic (it is not a spec); this whole body is
// specsync-owned and regenerated in full on every `specsync epic` run, so a
// manual edit to it is overwritten on the next run.
func epicBaseBody(title string) string {
	return fmt.Sprintf(
		"Coordination epic for **%s**, created by `specsync epic`.\n\n"+
			"This body is regenerated on every run — manual edits are overwritten. "+
			"Children are tracked in the Related section below.",
		title,
	)
}

// stripLeadingMarker removes a leading specsync identity-marker line (and the
// blank line after it), if present. Push always prepends a fresh marker via
// renderBody, so a body fetched via IssueReader.Get — which may already carry
// one from a prior push — must have it stripped first, or the marker
// duplicates a little more on every re-run.
func stripLeadingMarker(body string) string {
	trimmed := strings.TrimLeft(body, "\n")
	if !strings.HasPrefix(trimmed, "<!-- specsync:change=") {
		return body
	}
	idx := strings.Index(trimmed, "-->")
	if idx < 0 {
		return body
	}
	return strings.TrimLeft(trimmed[idx+len("-->"):], "\n")
}

// PushRelatedEdit fetches an existing issue by ref, upserts a managed
// "## Related" section listing others, and pushes the edited body back — the
// reference-edit sequence shared by `link`'s issue-reference path and
// `epic`'s child wiring, so neither duplicates the other's ~15 lines. slug is
// the real change slug when ref is a change's own issue (preserving its
// identity marker), or "" for a bare issue reference with no local change.
func PushRelatedEdit(ctx context.Context, provider WorkProvider, ref Ref, slug string, others []Ref) (Ref, error) {
	reader, ok := provider.(IssueReader)
	if !ok {
		return Ref{}, fmt.Errorf("provider %T does not support reading issues", provider)
	}
	item, err := reader.Get(ctx, ref.ID)
	if err != nil {
		return Ref{}, fmt.Errorf("fetch issue %s: %w", ref.URL, err)
	}
	edited := UpsertRelatedSection(stripLeadingMarker(item.Body), others)
	workItem := WorkItem{
		Slug:   slug,
		Title:  item.Title,
		Body:   edited,
		Labels: item.Labels,
	}
	return provider.Push(ctx, workItem, &ref)
}
