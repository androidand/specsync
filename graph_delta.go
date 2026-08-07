package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BuildDeltaAnnotations tags spec nodes in g with requirement-delta summaries
// (ADDED/MODIFIED/REMOVED counts) when the `openspec` CLI is on PATH. When
// openspec is absent, the method is a no-op — the graph is still useful
// without delta annotations.
//
// For each spec node in the graph, it:
//  1. Checks if `openspec` is on PATH.
//  2. Runs `openspec show --json --deltas-only <slug>` to get deltas.
//  3. Counts ADDED, MODIFIED, REMOVED deltas.
//  4. Tags the spec node with the counts.
//
// Errors from openspec are logged to stderr but do not stop the build — partial
// results are returned so the graph is still useful when openspec is unavailable.
func (g *Graph) BuildDeltaAnnotations(ctx context.Context) error {
	// Check if openspec is on PATH.
	if _, err := exec.LookPath("openspec"); err != nil {
		return nil // openspec not available — no-op
	}

	// Collect all spec nodes from the graph.
	var specNodes []Node
	for _, n := range g.Nodes() {
		if n.Kind == GraphKindSpec {
			specNodes = append(specNodes, n)
		}
	}

	if len(specNodes) == 0 {
		return nil
	}

	for _, specNode := range specNodes {
		slug := specNode.Slug
		if slug == "" {
			continue
		}

		// Run openspec show --json --deltas-only <slug>.
		deltas, err := listDeltas(ctx, slug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: openspec show deltas for %s: %v\n", slug, err)
			continue
		}

		// Count deltas by operation.
		added, modified, removed := countDeltas(deltas)

		// Only tag the spec node if there are actual deltas.
		if added+modified+removed > 0 {
			if n, ok := g.NodeByID(specNode.ID); ok {
				n.Label = fmt.Sprintf("%s [+%d ~%d -%d]", specNode.Label, added, modified, removed)
				g.nodes[specNode.ID] = n
			}
		}
	}

	return nil
}

// deltaSummary holds the count of deltas by operation.
type deltaSummary struct {
	Added    int
	Modified int
	Removed  int
}

// listDeltas runs `openspec show --json --deltas-only <slug>` to get deltas
// for the given change. Returns an empty slice (not error) when no deltas are
// found or when openspec is not available.
func listDeltas(ctx context.Context, slug string) ([]OpenSpecDelta, error) {
	args := []string{"show", "--json", "--deltas-only", slug}

	out, err := exec.CommandContext(ctx, "openspec", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("openspec %s: %w\n%s", strings.Join(args, " "), err, out)
	}

	if out == nil || strings.TrimSpace(string(out)) == "" {
		return nil, nil
	}

	var result struct {
		Deltas []OpenSpecDelta `json:"deltas"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse openspec show: %w", err)
	}

	return result.Deltas, nil
}

// countDeltas counts deltas by operation (ADDED, MODIFIED, REMOVED).
func countDeltas(deltas []OpenSpecDelta) (added, modified, removed int) {
	for _, d := range deltas {
		switch d.Operation {
		case "ADDED":
			added++
		case "MODIFIED":
			modified++
		case "REMOVED":
			removed++
		}
	}
	return added, modified, removed
}
