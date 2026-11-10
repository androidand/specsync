package specsync

import (
	"context"
	"fmt"
	"strings"
)

// WorkspaceChangeOptions describes the workspace whose change is wanted.
type WorkspaceChangeOptions struct {
	OpenSpecDir string
	// Branch is the checked-out branch; "" when detached or unknown.
	Branch string
	// DefaultBranch never names a change.
	DefaultBranch string
	// LinkedIssue is the issue an orchestrator linked the workspace to, or "".
	LinkedIssue string
	// LinkedIssueFn, when LinkedIssue is empty, is asked for it — only if the
	// branch names no change, so the orchestrator is not queried needlessly.
	LinkedIssueFn func() string
	// Provider reads the linked issue. Needed only when LinkedIssue is set and
	// no local ref cache names it.
	Provider WorkProvider
	// PullMissing pulls the linked issue into a new change when no local
	// change is linked to it.
	PullMissing bool
	// DryRun reports what would be found without writing anything.
	DryRun bool
}

// WorkspaceChangeResult names the change and how it was found.
type WorkspaceChangeResult struct {
	Slug string // "" when the workspace names no change
	// Source is "branch", "ref-cache", "issue-marker" or "pulled".
	Source string
	// WouldPull is set on a dry run when the issue would be pulled.
	WouldPull bool
	// Issue is the linked issue consulted, if any.
	Issue string
}

// WorkspaceChange finds the change a workspace is for: from the branch, then
// from the workspace's linked issue — a local ref cache naming it, then the
// issue's identity marker naming a local change — and finally, with
// PullMissing, by pulling the issue into a new change. A workspace that names
// no work yields an empty Slug and no error.
//
// A fresh worktree has no ref caches (they are gitignored), so the marker
// step is what finds a committed change there; it binds the cache with Adopt
// rather than Pull, because Pull would create a suffixed copy and overwrite
// the proposal with the issue's rendering.
func WorkspaceChange(ctx context.Context, opts WorkspaceChangeOptions) (WorkspaceChangeResult, error) {
	if slug := ChangeForBranch(opts.OpenSpecDir, opts.Branch, opts.DefaultBranch); slug != "" {
		return WorkspaceChangeResult{Slug: slug, Source: "branch"}, nil
	}
	linked := opts.LinkedIssue
	if linked == "" && opts.LinkedIssueFn != nil {
		linked = opts.LinkedIssueFn()
	}
	issue := strings.TrimPrefix(strings.TrimSpace(linked), "#")
	if issue == "" {
		return WorkspaceChangeResult{}, nil
	}
	res, err := changeForLinkedIssue(ctx, opts, issue)
	res.Issue = issue
	return res, err
}

func changeForLinkedIssue(ctx context.Context, opts WorkspaceChangeOptions, issue string) (WorkspaceChangeResult, error) {
	slug, err := changeForIssueInCache(opts.OpenSpecDir, issue)
	if err != nil {
		return WorkspaceChangeResult{}, err
	}
	if slug != "" {
		return WorkspaceChangeResult{Slug: slug, Source: "ref-cache"}, nil
	}
	if opts.Provider == nil {
		return WorkspaceChangeResult{}, fmt.Errorf("issue %s: no provider to read it", issue)
	}
	reader, ok := opts.Provider.(IssueReader)
	if !ok {
		return WorkspaceChangeResult{}, fmt.Errorf("provider %q cannot read issues", opts.Provider.Name())
	}
	item, err := reader.Get(ctx, issue)
	if err != nil {
		return WorkspaceChangeResult{}, fmt.Errorf("read issue %s: %w", issue, err)
	}
	if slug := slugFromMarker(item.Body); slug != "" && isActiveChange(opts.OpenSpecDir, slug) {
		if !opts.DryRun {
			if _, err := Adopt(ctx, AdoptOptions{OpenSpecDir: opts.OpenSpecDir, Provider: opts.Provider, IssueID: issue, Slug: slug}); err != nil {
				return WorkspaceChangeResult{}, fmt.Errorf("bind %s to issue %s: %w", slug, issue, err)
			}
		}
		return WorkspaceChangeResult{Slug: slug, Source: "issue-marker"}, nil
	}
	if !opts.PullMissing {
		return WorkspaceChangeResult{}, nil
	}
	if opts.DryRun {
		return WorkspaceChangeResult{Source: "pulled", WouldPull: true}, nil
	}
	res, err := Pull(ctx, PullOptions{OpenSpecDir: opts.OpenSpecDir, Provider: opts.Provider, IssueID: issue})
	if err != nil {
		return WorkspaceChangeResult{}, fmt.Errorf("pull issue %s: %w", issue, err)
	}
	return WorkspaceChangeResult{Slug: res.Slug, Source: "pulled"}, nil
}

// changeForIssueInCache returns the active change whose GitHub ref cache
// names issue, or "".
func changeForIssueInCache(openspecDir, issue string) (string, error) {
	changes, err := LoadChanges(openspecDir)
	if err != nil {
		return "", err
	}
	for _, c := range changes {
		if c.Archived {
			continue
		}
		refs, err := LoadRefs(c.Dir)
		if err != nil {
			continue
		}
		for key, ref := range refs {
			if strings.HasPrefix(key, "github") && ref.ID == issue {
				return c.Slug, nil
			}
		}
	}
	return "", nil
}
