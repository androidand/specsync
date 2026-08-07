package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

// runRelate builds the work graph for a change and prints the connected slice.
// It is read-only — no tracker writes, no refs.json/links.md mutation.
// The command only reads from disk (change folders, refs.json, links.md) and
// queries gh/git for edge data. It never writes to disk or the tracker.
func runRelate(args []string) {
	fs := flag.NewFlagSet("relate", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	change := fs.String("change", "", "change slug to relate")
	path := fs.String("path", "", "file path to find related changes (alternative to -change)")
	repo := fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	fs.Parse(args)

	if *change == "" && *path == "" {
		fail(fmt.Errorf("relate: need -change <slug> or -path <file>"))
	}
	if *change != "" && *path != "" {
		fail(fmt.Errorf("relate: specify only one of -change or -path, not both"))
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}
	repoRoot := filepath.Dir(abs)

	// Resolve repo: -repo flag → gh repo view → empty.
	ctx := context.Background()
	if *repo == "" {
		if out, err := exec.CommandContext(ctx, "gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner").CombinedOutput(); err == nil {
			*repo = strings.TrimSpace(string(out))
		}
	}

	// Build the graph.
	g := specsync.NewGraph()

	if *change != "" {
		// Load the change and build its core edges.
		c, err := specsync.LoadChangeBySlug(abs, *change)
		if err != nil {
			fail(err)
		}
		if err := g.BuildCore(*c, abs); err != nil {
			fail(err)
		}
	} else {
		// Find changes that touch the given file path.
		changes, err := findChangesForPath(repoRoot, *path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: find changes for path: %v\n", err)
		}
		for _, c := range changes {
			if err := g.BuildCore(c, abs); err != nil {
				fmt.Fprintf(os.Stderr, "warning: build core for %s: %v\n", c.Slug, err)
			}
		}
	}

	// Derive gh-based edges.
	if err := g.BuildGHEdges(ctx, *repo, nil); err != nil {
		fmt.Fprintf(os.Stderr, "warning: build gh edges: %v\n", err)
	}
	if err := g.BuildReleaseEdges(ctx, *repo, nil, nil); err != nil {
		fmt.Fprintf(os.Stderr, "warning: build release edges: %v\n", err)
	}
	if err := g.BuildWorkFileEdges(ctx, *repo, nil); err != nil {
		fmt.Fprintf(os.Stderr, "warning: build work-file edges: %v\n", err)
	}

	// Print the graph.
	printGraph(g)
}

// findChangesForPath finds all changes that touch the given file path using
// git log to resolve actual file-to-change mapping. It returns changes whose
// proposal.md, tasks.md, or original-ask.md are in the same change folder as
// a file touched by the given path.
func findChangesForPath(repoRoot, targetPath string) ([]specsync.Change, error) {
	// Use git log to find commits touching the target path.
	ctx := context.Background()
	commitArgs := []string{"log", "--pretty=format:%H", "--diff-filter=ACMR", "--", targetPath}
	out, err := exec.CommandContext(ctx, "git", commitArgs...).CombinedOutput()
	if err != nil {
		// Git command failed (e.g., not a git repo) — fall back to no results.
		return nil, nil
	}

	// Collect unique change directories from commit messages.
	changeDirs := map[string]bool{}
	for _, hash := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}
		// Look for specsync issue references in the commit message body.
		bodyOut, err := exec.CommandContext(ctx, "git", "log", "-1", "--pretty=format:%B", hash).CombinedOutput()
		if err != nil {
			continue
		}
		body := string(bodyOut)
		// Extract issue numbers from body (e.g., "#42", "owner/repo#42").
		for _, ref := range extractIssueRefs(body) {
			// Try to resolve the issue to a change slug via gh.
			slug, err := resolveIssueToSlug(repoRoot, ref)
			if err != nil {
				continue
			}
			if slug != "" {
				changeDirs[filepath.Join(repoRoot, "openspec", "changes", slug)] = true
			}
		}
	}

	// Load changes for each resolved directory.
	var matches []specsync.Change
	for dir := range changeDirs {
		c, err := specsync.LoadChange(dir, false, filepath.Join(repoRoot, "openspec"))
		if err != nil {
			continue
		}
		if c != nil {
			matches = append(matches, *c)
		}
	}

	// Sort for deterministic output.
	sortChangesBySlug(matches)
	return matches, nil
}

// extractIssueRefs extracts issue references from commit message body text.
// It matches "#N" and "owner/repo#N" patterns.
func extractIssueRefs(body string) []string {
	var refs []string
	seen := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		for i := 0; i < len(line); i++ {
			if line[i] == '#' {
				// Try to match "owner/repo#N" by looking backward for alnum/slash/hyphen chars.
				if i > 0 && (isAlnum(line[i-1]) || line[i-1] == '/' || line[i-1] == '-') {
					start := i - 1
					for start > 0 && (isAlnum(line[start-1]) || line[start-1] == '/' || line[start-1] == '-') {
						start--
					}
					// Extract the number after #.
					num := ""
					for j := i + 1; j < len(line) && isDigit(line[j]); j++ {
						num += string(line[j])
					}
					if num != "" {
						ref := line[start:i+1] + num
						if !seen[ref] {
							seen[ref] = true
							refs = append(refs, ref)
						}
					}
				} else if i == 0 || !isAlnum(line[i-1]) {
					// Match "#N" pattern (bare number).
					num := ""
					for j := i + 1; j < len(line) && isDigit(line[j]); j++ {
						num += string(line[j])
					}
					if num != "" {
						ref := "#" + num
						if !seen[ref] {
							seen[ref] = true
							refs = append(refs, ref)
						}
					}
				}
			}
		}
	}
	return refs
}

func isAlnum(r byte) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isDigit(r byte) bool {
	return r >= '0' && r <= '9'
}

// resolveIssueToSlug queries gh to find the change slug associated with an issue.
func resolveIssueToSlug(repoRoot, issueRef string) (string, error) {
	issueNum := strings.TrimPrefix(issueRef, "#")
	repo := ""
	if idx := strings.LastIndex(issueRef, "#"); idx > 0 {
		repo = issueRef[:idx]
		issueNum = issueRef[idx+1:]
	}

	ctx := context.Background()
	// Query gh to find PRs linked to this issue.
	args := []string{"api", fmt.Sprintf("repos/%s/issues/%s", repo, issueNum), "--jq", ".pull_requests // [] | .[] | {number: .number}"}
	out, err := exec.CommandContext(ctx, "gh", args...).CombinedOutput()
	if err != nil {
		return "", err
	}

	// Parse PR numbers and check if any branch names match the change slug pattern.
	var prs []struct{ Number int }
	if err := json.Unmarshal(out, &prs); err != nil {
		return "", nil
	}

	for _, pr := range prs {
		// Check PR branch name for change slug.
		branchArgs := []string{"api", fmt.Sprintf("repos/%s/pulls/%d", repo, pr.Number), "--jq", ".head.ref"}
		branchOut, err := exec.CommandContext(ctx, "gh", branchArgs...).CombinedOutput()
		if err != nil {
			continue
		}
		branch := strings.TrimSpace(string(branchOut))
		// Extract slug from branch name (e.g., "feat/42-my-change" → "my-change").
		if slug := extractSlugFromBranch(branch); slug != "" {
			return slug, nil
		}
	}

	return "", nil
}

// extractSlugFromBranch extracts a change slug from a branch name.
// Branch names follow the pattern "feat/<issue>-<slug>" or "fix/<issue>-<slug>".
func extractSlugFromBranch(branch string) string {
	// Remove prefix like "feat/", "fix/", "bugfix/".
	for _, prefix := range []string{"feat/", "fix/", "bugfix/", "feature/", "hotfix/"} {
		if strings.HasPrefix(branch, prefix) {
			branch = strings.TrimPrefix(branch, prefix)
			break
		}
	}
	// Strip leading issue number and hyphen (only if the first segment is numeric).
	if idx := strings.Index(branch, "-"); idx > 0 {
		first := branch[:idx]
		isNum := true
		for _, r := range first {
			if r < '0' || r > '9' {
				isNum = false
				break
			}
		}
		if isNum {
			return branch[idx+1:]
		}
	}
	return branch
}

// sortChangesBySlug sorts changes by slug for deterministic output.
func sortChangesBySlug(changes []specsync.Change) {
	for i := 1; i < len(changes); i++ {
		key := changes[i]
		j := i - 1
		for j >= 0 && changes[j].Slug > key.Slug {
			changes[j+1] = changes[j]
			j--
		}
		changes[j+1] = key
	}
}

// printGraph prints the work graph in a human-readable format.
func printGraph(g *specsync.Graph) {
	nodes := g.Nodes()
	edges := g.Edges()

	if len(nodes) == 0 {
		fmt.Println("No nodes in graph.")
		return
	}

	// Print nodes grouped by kind.
	fmt.Println("Nodes:")
	var currentKind specsync.GraphKind
	for _, n := range nodes {
		if n.Kind != currentKind {
			currentKind = n.Kind
			fmt.Printf("\n  [%s]\n", currentKind)
		}
		fmt.Printf("    %s  %s\n", n.ID, n.Label)
	}

	// Print edges.
	if len(edges) > 0 {
		fmt.Println("\nEdges:")
		for _, e := range edges {
			fmt.Printf("  %s  %s -> %s  (%s)\n", e.Kind, e.From, e.To, e.Source)
		}
	}
}
