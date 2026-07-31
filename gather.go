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

// GatherTraceMulti assembles a TraceInput from multiple OpenSpec directories.
// Each directory is loaded independently and merged into a single TraceInput.
// This enables cross-repo scanning where changes from different repositories
// can be correlated by topic, explicit links.md references, or path patterns.
//
// src may be nil (commits omitted — the graph still shows changes).
// When multiple directories are given, commits are not gathered (each repo
// would need its own git context), so src is ignored for multi-repo scans.
func GatherTraceMulti(ctx context.Context, openspecDirs []string, src CommitSource, scope Scope) (TraceInput, error) {
	var in TraceInput

	for _, dir := range openspecDirs {
		changes, err := LoadChanges(dir)
		if err != nil {
			return TraceInput{}, err
		}

		for _, c := range changes {
			refs, err := loadRefs(c.Dir)
			if err != nil {
				return TraceInput{}, err
			}
			in.Changes = append(in.Changes, ChangeRefs{Change: c, IssueIDs: issueIDsFromRefs(refs)})
		}
	}

	// For multi-repo scans, commits are not gathered from the source
	// because each repo would need its own git context. The caller can
	// provide commits separately if needed.
	_ = src

	return in, nil
}

// CrossRepoCorrelation finds related changes across multiple OpenSpec directories.
// It returns a map of change slug to the list of related changes from other repos,
// along with the provenance (why they're related).
func CrossRepoCorrelation(changeRefs []ChangeRefs, scope Scope) map[string][]ChangeRelationship {
	relMap := make(map[string][]ChangeRelationship)

	// Build index of issueID -> ChangeRefs for links.md correlation.
	issueToChangeRefs := make(map[string][]ChangeRefs)
	for _, cr := range changeRefs {
		for _, id := range cr.IssueIDs {
			issueToChangeRefs[id] = append(issueToChangeRefs[id], cr)
		}
	}

	// Topic correlation
	if scope.Topic != "" {
		for _, cr1 := range changeRefs {
			for _, cr2 := range changeRefs {
				if cr1.Change.Slug == cr2.Change.Slug {
					continue
				}
				if extractRepoFromDir(cr1.Change.Dir) == extractRepoFromDir(cr2.Change.Dir) {
					continue // same repo
				}
				if topicMatches(cr1.Change, scope.Topic) && topicMatches(cr2.Change, scope.Topic) {
					relMap[cr1.Change.Slug] = append(relMap[cr1.Change.Slug], ChangeRelationship{
						RelatedChange: cr2.Change,
						Provenance:    ProvTopicCorrelation,
					})
				}
			}
		}
	}

	// Path correlation: always runs — pathCorrelates checks scope paths first,
	// then falls back to extracting file paths from change bodies and tasks.
	for _, cr1 := range changeRefs {
		for _, cr2 := range changeRefs {
			if cr1.Change.Slug == cr2.Change.Slug {
				continue
			}
			if extractRepoFromDir(cr1.Change.Dir) == extractRepoFromDir(cr2.Change.Dir) {
				continue // same repo
			}
			if pathCorrelates(cr1.Change, cr2.Change, scope.Paths) {
				relMap[cr1.Change.Slug] = append(relMap[cr1.Change.Slug], ChangeRelationship{
					RelatedChange: cr2.Change,
					Provenance:    ProvPathCorrelation,
				})
			}
		}
	}

	// Links.md correlation: if change A's links.md references an issue ID that
	// is also bound to change B in a different repo, they're related.
	seenLinks := make(map[string]bool)
	for _, cr1 := range changeRefs {
		for _, ref := range cr1.Change.Links {
			id := issueIDFromRef(ref)
			if id == "" {
				continue
			}
			for _, cr2 := range issueToChangeRefs[id] {
				if cr1.Change.Slug == cr2.Change.Slug {
					continue
				}
				if extractRepoFromDir(cr1.Change.Dir) == extractRepoFromDir(cr2.Change.Dir) {
					continue // same repo
				}
				key := cr1.Change.Slug + "\x00" + cr2.Change.Slug
				if seenLinks[key] {
					continue // already correlated
				}
				seenLinks[key] = true
				relMap[cr1.Change.Slug] = append(relMap[cr1.Change.Slug], ChangeRelationship{
					RelatedChange: cr2.Change,
					Provenance:    ProvLinksMD,
				})
			}
		}
	}

	return relMap
}

// extractRepoFromDir returns the repo name from a change directory path.
func extractRepoFromDir(dir string) string {
	// The openspec directory is typically under <repo>/openspec/
	// Extract the repo name from the path
	parts := strings.Split(dir, "/")
	for i := len(parts) - 1; i > 0; i-- {
		if parts[i] == "openspec" {
			return parts[i-1]
		}
	}
	return dir
}

// pathCorrelates returns true if two changes touch similar paths.
func pathCorrelates(c1, c2 Change, paths []string) bool {
	// If both changes mention the same scope paths in their title, body, or tasks, correlate.
	for _, p := range paths {
		c1Has := strings.Contains(c1.Body, p) || strings.Contains(c1.Title, p) || strings.Contains(c1.TasksMarkdown, p)
		c2Has := strings.Contains(c2.Body, p) || strings.Contains(c2.Title, p) || strings.Contains(c2.TasksMarkdown, p)
		if c1Has && c2Has {
			return true
		}
	}

	// Extract file paths from each change's title, body, and tasks, then compare.
	pt1 := extractFilePaths(c1.Title)
	p1 := extractFilePaths(c1.Body)
	pt2 := extractFilePaths(c2.Title)
	p2 := extractFilePaths(c2.Body)
	t1 := extractFilePaths(c1.TasksMarkdown)
	t2 := extractFilePaths(c2.TasksMarkdown)
	for _, p := range append(pt1, p1...) {
		if containsPath(append(pt2, p2...), p) || containsPath(t2, p) {
			return true
		}
	}
	for _, p := range t1 {
		if containsPath(append(pt2, p2...), p) || containsPath(t2, p) {
			return true
		}
	}
	return false
}

// extractFilePaths returns strings that look like relative file paths from text.
// It matches patterns like "src/foo.ts", "lib/bar.js", "test/baz.tsx", etc.
func extractFilePaths(text string) []string {
	var paths []string
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		for i := 0; i < len(line); {
			slashIdx := strings.Index(line[i:], "/")
			if slashIdx < 0 {
				break
			}
			slashPos := i + slashIdx
			// Go back to start of directory word
			start := slashPos
			for start > 0 && line[start-1] != ' ' && line[start-1] != '\t' && line[start-1] != '`' && line[start-1] != '[' && line[start-1] != '(' && line[start-1] != '"' && line[start-1] != '{' && line[start-1] != '<' && line[start-1] != '#' && line[start-1] != '\u0027' && line[start-1] != '?' {
				start--
			}
			// Find the end of the path
			end := slashPos
			for end < len(line) && line[end] != ' ' && line[end] != '\t' && line[end] != ')' && line[end] != '>' && line[end] != '"' && line[end] != '`' && line[end] != ']' && line[end] != '\u0027' && line[end] != '}' && line[end] != '#' && line[end] != '?' {
				end++
			}
			if end > start && strings.Contains(line[start:end], ".") {
				path := line[start:end]
				// Skip paths inside markdown links
				if start > 0 && line[start-1] == '[' {
					i = end
					continue
				}
				if len(path) > 3 && !seen[path] && looksLikeFilePath(path) {
					seen[path] = true
					paths = append(paths, path)
				}
			}
			if end > start {
				i = end
			} else {
				i = slashPos + 1
			}
		}
	}
	return paths
}

// looksLikeFilePath returns true if the path looks like a real file path
// (has a directory component and a file extension, not a URL or domain).
func looksLikeFilePath(p string) bool {
	if !strings.Contains(p, "/") || !strings.Contains(p, ".") {
		return false
	}
	// Not a URL
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return false
	}
	// Not a domain (no top-level domain pattern)
	ext := p[strings.LastIndex(p, ".")+1:]
	if len(ext) > 6 {
		return false
	}
	return true
}

// containsPath returns true if any path in paths ends with or equals target,
// or if target is a directory prefix of a path (e.g., target "src/main" matches
// path "src/main.ts").
func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target || strings.HasSuffix(p, "/"+target) {
			return true
		}
		if strings.HasPrefix(p, target+"/") {
			return true
		}
	}
	return false
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
	if r.Provider != "" && strings.HasPrefix(id, r.Provider+"/") {
		id = strings.TrimPrefix(id, r.Provider+"/")
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
