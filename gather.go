package specsync

import (
	"context"
	"fmt"
	"strings"
)

// GatherTrace assembles a TraceInput for a scope from the OpenSpec changes on
// disk and the commits the CommitSource yields. It is the I/O bridge between the
// pure resolver and the host CLIs; resolution itself stays in ResolveTrace.
//
// Commit range: a change scope reads full history (a change's commits may
// predate the last tag); a range scope uses since/until; an area scope filters
// by paths. src may be nil (commits omitted — the graph still shows changes).
func GatherTrace(ctx context.Context, openspecDir string, src CommitSource, scope Scope) (TraceInput, error) {
	changes, err := LoadChanges(openspecDir)
	if err != nil {
		return TraceInput{}, err
	}

	var in TraceInput
	for _, c := range changes {
		refs, err := loadRefs(c.Dir)
		if err != nil {
			return TraceInput{}, err
		}
		in.Changes = append(in.Changes, ChangeRefs{Change: c, IssueIDs: issueIDsFromRefs(refs)})
	}

	if src != nil {
		since, until := scope.Since, scope.Until
		if scope.isChange() {
			since, until = AllHistory, "HEAD" // a change's commits may predate the last tag
		}
		commits, err := src.Commits(ctx, since, until, scope.Paths)
		if err != nil {
			return TraceInput{}, err
		}
		in.Commits = commits
	}
	return in, nil
}

// issueIDsFromRefs extracts the provider issue numbers from a change's ref
// cache. A ref ID may be a bare number ("42") or already namespaced; the trailing
// number is what commit references match against.
func issueIDsFromRefs(refs map[string]Ref) []string {
	var ids []string
	seen := map[string]bool{}
	for _, r := range refs {
		id := issueIDFromRef(r)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func issueIDFromRef(r Ref) string {
	id := r.ID
	if i := strings.LastIndex(id, "#"); i >= 0 {
		id = id[i+1:]
	}
	return id
}

// ResolveLiveRefs fills in IssueIDs for changes that have none — no local ref
// cache — via the provider's Find, the same read-only identity-marker lookup
// that rebuilds a lost cache (see cache.go). This never touches disk: it's for
// a checkout that has no cache at all (e.g. CI) and would otherwise report
// every one of that change's commits as an unlinked gap. Changes that already
// have cached IssueIDs are left untouched — no live call is made for them.
func ResolveLiveRefs(ctx context.Context, in *TraceInput, resolver WorkProvider) error {
	for i := range in.Changes {
		if len(in.Changes[i].IssueIDs) > 0 {
			continue
		}
		slug := in.Changes[i].Change.Slug
		ref, err := resolver.Find(ctx, slug)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", slug, err)
		}
		if ref == nil {
			continue
		}
		if id := issueIDFromRef(*ref); id != "" {
			in.Changes[i].IssueIDs = []string{id}
		}
	}
	return nil
}
