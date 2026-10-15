package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

// openIssuesInArea finds open issues matching the topic that link to no change —
// neither carrying a specsync:change= marker nor bound to a change via the ref
// cache. Returns a note when issues could not be read (no topic, or gh absent),
// so scan degrades visibly rather than silently narrowing.
func openIssuesInArea(ctx context.Context, in specsync.TraceInput, topic string) (loose []specsync.FetchedItem, note string) {
	if topic == "" {
		return nil, "topic needed to search issues; path-only issue scan not yet supported"
	}
	var searcher specsync.IssueSearcher = specsync.NewGitHubProvider()
	found, err := searcher.SearchOpenIssues(ctx, topic)
	if err != nil {
		return nil, "issues not read: gh unavailable"
	}
	linked := map[string]bool{}
	for _, cr := range in.Changes {
		for _, id := range cr.IssueIDs {
			linked[id] = true
		}
	}
	for _, it := range found {
		if linked[it.ID] || strings.Contains(it.Body, "specsync:change=") {
			continue // already linked to a change
		}
		loose = append(loose, it)
	}
	return loose, ""
}

// resolve gathers and resolves a trace, failing on error.
func resolve(openspecDir string, scope specsync.Scope) specsync.Trace {
	in, err := specsync.GatherTrace(context.Background(), openspecDir, specsync.NewGitCommitSource(), scope)
	if err != nil {
		fail(err)
	}
	return specsync.ResolveTrace(in, scope)
}

func printTrace(tr specsync.Trace) {
	fmt.Println("Nodes")
	for _, n := range tr.Nodes {
		fmt.Printf("  [%s] %s — %s\n", n.Kind, n.ID, n.Label)
	}
	fmt.Println("Links")
	for _, l := range tr.Links {
		fmt.Printf("  %s → %s  (%s)\n", l.From, l.To, l.Provenance)
	}
	if len(tr.Gaps) > 0 {
		fmt.Println("Gaps")
		for _, g := range tr.Gaps {
			fmt.Printf("  %s: %s (%s)\n", g.Kind, g.Subject, g.Reason)
		}
	}
}

func emitJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fail(err)
	}
	fmt.Println(string(b))
}

func nodesOfKind(tr specsync.Trace, kind specsync.NodeKind) []specsync.TraceNode {
	var out []specsync.TraceNode
	for _, n := range tr.Nodes {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

// splitArea sorts scan args into path globs and a topic string. An arg is a path
// when it contains a separator or glob char or names an existing file; the rest
// join into the topic.
func splitArea(args []string) (paths []string, topic string) {
	var topicWords []string
	for _, a := range args {
		if looksLikePath(a) {
			paths = append(paths, a)
		} else {
			topicWords = append(topicWords, a)
		}
	}
	return paths, strings.TrimSpace(strings.Join(topicWords, " "))
}

func looksLikePath(a string) bool {
	if strings.ContainsAny(a, "/*?[") || strings.HasPrefix(a, ".") {
		return true
	}
	if _, err := os.Stat(a); err == nil {
		return true
	}
	return false
}

func quoteIf(topic string) []string {
	if topic == "" {
		return nil
	}
	return []string{fmt.Sprintf("%q", topic)}
}

func rangeLabel(since, until string) string {
	if until == "" {
		until = "HEAD"
	}
	if since == "" {
		since = latestTag()
		if since == "" {
			since = "start"
		}
	}
	return since + ".." + until
}

func latestTag() string {
	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func nextVersionSuffix(impact specsync.ImpactResult) string {
	tag := latestTag()
	if tag == "" {
		return ""
	}
	v, err := specsync.ParseVersion(tag)
	if err != nil {
		return ""
	}
	next := v.Bump(impact.Impact)
	if next.String() == v.String() {
		return ""
	}
	return fmt.Sprintf("   (%s → %s)", tag, "v"+next.String())
}

// openSpecStatus maps change slug → OpenSpec status, best-effort.
func openSpecStatus(ctx context.Context, o *specsync.OpenSpecCLI) map[string]specsync.OpenSpecChange {
	m := map[string]specsync.OpenSpecChange{}
	changes, err := o.Changes(ctx)
	if err != nil {
		return m
	}
	for _, c := range changes {
		m[c.Name] = c
	}
	return m
}

func completedShipped(shipped []specsync.TraceNode, status map[string]specsync.OpenSpecChange) []string {
	var out []string
	for _, n := range shipped {
		slug := strings.TrimPrefix(n.ID, "change:")
		if c, ok := status[slug]; ok && c.TotalTasks > 0 && c.CompletedTasks == c.TotalTasks {
			out = append(out, slug)
		}
	}
	return out
}

func archiveHygieneError(archiveCandidates []string, failOnArchiveCandidates bool) error {
	if !failOnArchiveCandidates || len(archiveCandidates) == 0 {
		return nil
	}
	return fmt.Errorf("release-plan: %d archive candidate(s) remain unarchived: %s", len(archiveCandidates), strings.Join(archiveCandidates, ", "))
}

func archiveCompletedChanges(openspecDir string, candidates []string) ([]string, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	archiveRoot := filepath.Join(openspecDir, "changes", "archive")
	if err := os.MkdirAll(archiveRoot, 0o755); err != nil {
		return nil, fmt.Errorf("archive-completed: create archive dir: %w", err)
	}
	archived := make([]string, 0, len(candidates))
	for _, slug := range candidates {
		src := filepath.Join(openspecDir, "changes", slug)
		dst := filepath.Join(archiveRoot, slug)
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return archived, fmt.Errorf("archive-completed: stat %s: %w", slug, err)
		}
		if _, err := os.Stat(dst); err == nil {
			return archived, fmt.Errorf("archive-completed: destination already exists for %s", slug)
		} else if !os.IsNotExist(err) {
			return archived, fmt.Errorf("archive-completed: check destination for %s: %w", slug, err)
		}
		if err := os.Rename(src, dst); err != nil {
			return archived, fmt.Errorf("archive-completed: move %s to archive: %w", slug, err)
		}
		archived = append(archived, slug)
	}
	return archived, nil
}

// resolvePRBodyRef finds the synced ref for a change. It checks the ref cache
// first, then falls back to the provider's Find.
func resolvePRBodyRef(ctx context.Context, prov specsync.WorkProvider, c *specsync.Change) (*specsync.Ref, error) {
	// Try ref cache first.
	refs, err := specsync.LoadRefs(c.Dir)
	if err == nil && len(refs) > 0 {
		key := prov.Name()
		if ref, ok := refs[key]; ok {
			return &ref, nil
		}
		// Legacy bare-"github" key.
		if ref, ok := refs["github"]; ok {
			return &ref, nil
		}
	}

	// Fall back to provider Find.
	found, err := prov.Find(ctx, c.Slug)
	if err != nil {
		return nil, err
	}
	return found, nil
}

// ensureNoDuplicateReference strips any existing Part of / Closes line from body
// so that running pr-body twice never stacks duplicates. It returns the cleaned
// body ready for the reference line to be prepended.
func ensureNoDuplicateReference(body string, allComplete bool, issueID string) string {
	lines := strings.Split(body, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if allComplete {
			// Skip existing "Closes #N" for the same issue.
			if strings.HasPrefix(trimmed, "Closes #"+issueID) ||
				strings.HasPrefix(trimmed, "- [x] Closes #"+issueID) {
				continue
			}
		} else {
			// Skip existing "Part of #N" for the same issue.
			if strings.HasPrefix(trimmed, "Part of #"+issueID) ||
				strings.HasPrefix(trimmed, "- [x] Part of #"+issueID) {
				continue
			}
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

// branchMatchesChange reports whether a PR head branch name belongs to a change.
// Matches exact slug or common prefixes like "feat/<slug>", "feature/<slug>".
func branchMatchesChange(branch, slug string) bool {
	if branch == slug {
		return true
	}
	// Strip common prefixes.
	for _, prefix := range []string{"feat/", "feature/", "fix/", "chore/", "refactor/"} {
		if strings.HasPrefix(branch, prefix) {
			rest := strings.TrimPrefix(branch, prefix)
			// Slug might be the first component before a slash.
			if idx := strings.Index(rest, "/"); idx >= 0 {
				rest = rest[:idx]
			}
			if rest == slug {
				return true
			}
		}
	}
	return false
}

// prBodyReferencesIssue reports whether body contains a reference to issue number.
// Matches patterns like "#42", "Closes #42", "Part of #42", "Refs #42", etc.
func prBodyReferencesIssue(body, issueNum string) bool {
	// Look for #N pattern where N matches the issue number.
	refPattern := "#" + issueNum
	return strings.Contains(body, refPattern)
}
