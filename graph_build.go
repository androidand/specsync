package specsync

import (
	"fmt"
	"os"
	"path/filepath"
)

// BuildCore builds the asserted core edges for a single change: spec↔spec
// from links.md and spec↔issue from the ref cache. It populates g in place.
//
// Spec↔spec edges come from parseLinksMD (the same function LoadChange uses
// for the Links, BlockedBy, and Blocks fields). Each entry becomes a directed
// EdgeSpecSpec edge from this change to the linked change slug.
//
// Spec↔issue edges come from the ref cache (LoadRefs). Each ref becomes an
// EdgeSpecIssue edge from the change to the issue node.
//
// The change's slug is used as the spec node ID; issue nodes are keyed by
// their provider+id to stay unique across providers.
func (g *Graph) BuildCore(c Change, openspecDir string) error {
	// Add the spec node.
	g.AddNode(Node{
		Kind:  GraphKindSpec,
		ID:    graphNodeID(GraphKindSpec, c.Slug),
		Label: c.Title,
		Slug:  c.Slug,
		Stage: c.Stage,
	})

	// Spec↔spec edges from links.md.
	links, blockedBy, blocks := parseLinksMD(c.Dir, openspecDir)
	allRefs := make([]Ref, 0, len(links)+len(blockedBy)+len(blocks))
	allRefs = append(allRefs, links...)
	allRefs = append(allRefs, blockedBy...)
	allRefs = append(allRefs, blocks...)
	for _, ref := range allRefs {
		// Try to resolve the ref to a sibling slug.
		slug := resolveSlugFromRef(ref, openspecDir)
		if slug != "" {
			g.AddEdge(Edge{
				Kind:   EdgeSpecSpec,
				From:   graphNodeID(GraphKindSpec, c.Slug),
				To:     graphNodeID(GraphKindSpec, slug),
				Source: "links.md",
			})
			continue
		}
		// Unresolvable slug — skip (will resolve on next sync).
	}

	// Spec↔issue edges from ref cache.
	refs, err := LoadRefs(c.Dir)
	if err != nil {
		return fmt.Errorf("load refs for %s: %w", c.Slug, err)
	}
	for _, ref := range refs {
		issueID := ref.Provider + ":" + ref.ID
		g.AddNode(Node{
			Kind:     GraphKindIssue,
			ID:       graphNodeID(GraphKindIssue, issueID),
			Label:    ref.URL,
			IssueNum: ref.ID,
		})
		g.AddEdge(Edge{
			Kind:   EdgeSpecIssue,
			From:   graphNodeID(GraphKindSpec, c.Slug),
			To:     graphNodeID(GraphKindIssue, issueID),
			Source: "refs.json",
		})
	}

	return nil
}

// resolveSlugFromRef tries to extract a sibling change slug from a Ref.
// It returns "" when the ref points to an external issue or can't be resolved.
func resolveSlugFromRef(ref Ref, openspecDir string) string {
	if openspecDir == "" {
		return ""
	}
	// Full URL → extract slug from the URL path if it points to a local change.
	if ref.URL != "" {
		// Check if this URL matches any sibling's ref cache.
		entries, err := readDirNames(filepath.Join(openspecDir, "changes"))
		if err == nil {
			for _, slug := range entries {
				siblingRefs, err := LoadRefs(filepath.Join(openspecDir, "changes", slug))
				if err != nil {
					continue
				}
				for _, sr := range siblingRefs {
					if sr.URL == ref.URL {
						return slug
					}
				}
			}
		}
		// Try archived.
		entries, err = readDirNames(filepath.Join(openspecDir, "changes", "archive"))
		if err == nil {
			for _, slug := range entries {
				siblingRefs, err := LoadRefs(filepath.Join(openspecDir, "changes", "archive", slug))
				if err != nil {
					continue
				}
				for _, sr := range siblingRefs {
					if sr.URL == ref.URL {
						return slug
					}
				}
			}
		}
	}
	return ""
}

// readDirNames returns the directory entry names (sorted) under dir,
// or nil, nil if the directory does not exist.
func readDirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	// Simple insertion sort for deterministic order.
	for i := 1; i < len(names); i++ {
		key := names[i]
		j := i - 1
		for j >= 0 && names[j] > key {
			names[j+1] = names[j]
			j--
		}
		names[j+1] = key
	}
	return names, nil
}
