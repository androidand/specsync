package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// BuildWorkFileEdges derives work↔files edges for all PR and commit nodes in g
// by querying `gh pr view <number> --json files` for PRs and `git log` for
// commits. It routes through a GHRunner so it is mockable in tests.
//
// For each PR node in the graph, it:
//  1. Queries `gh pr view <number> --json files` to get changed files.
//  2. Adds a work↔files edge for each file.
//
// For each commit node in the graph, it:
//  1. Queries `git diff-tree --no-commit-id -r --name-only <hash>` to get
//     changed files.
//  2. Adds a work↔files edge for each file.
//
// Errors from gh/git are logged to stderr but do not stop the build — partial
// results are returned so the graph is still useful when gh/git is unavailable.
func (g *Graph) BuildWorkFileEdges(ctx context.Context, repo string, runner GHRunner) error {
	if runner == nil {
		runner = defaultGHRunner()
	}

	// Derive work↔files edges for PR nodes.
	if err := g.buildPRFileEdges(ctx, repo, runner); err != nil {
		fmt.Fprintf(os.Stderr, "warning: build PR file edges: %v\n", err)
	}

	// Derive work↔files edges for commit nodes.
	if err := g.buildCommitFileEdges(ctx, repo, runner); err != nil {
		fmt.Fprintf(os.Stderr, "warning: build commit file edges: %v\n", err)
	}

	return nil
}

// buildPRFileEdges derives work↔files edges for all PR nodes in g.
func (g *Graph) buildPRFileEdges(ctx context.Context, repo string, runner GHRunner) error {
	// Collect all PR nodes from the graph.
	var prNodes []Node
	for _, n := range g.Nodes() {
		if n.Kind == GraphKindPR {
			prNodes = append(prNodes, n)
		}
	}

	if len(prNodes) == 0 {
		return nil
	}

	for _, prNode := range prNodes {
		// Extract PR number from node ID.
		// PR node IDs are formatted as "pr:owner/repo:100".
		prNum := extractPRNum(prNode.ID)
		if prNum == "" {
			continue
		}

		// Query gh for changed files.
		prNumInt := atoiSafe(prNum)
		files, err := listPRFiles(ctx, runner, repo, prNumInt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: gh pr view files for PR #%d: %v\n", prNumInt, err)
			continue
		}

		for _, file := range files {
			// Add work↔files edge.
			g.AddEdge(Edge{
				Kind:   EdgeWorkFiles,
				From:   prNode.ID,
				To:     "file:" + file,
				Source: "gh pr view --json files",
			})
		}
	}

	return nil
}

// buildCommitFileEdges derives work↔files edges for all commit nodes in g.
func (g *Graph) buildCommitFileEdges(ctx context.Context, repo string, runner GHRunner) error {
	// Collect all commit nodes from the graph.
	var commitNodes []Node
	for _, n := range g.Nodes() {
		if n.Kind == GraphKindCommit {
			commitNodes = append(commitNodes, n)
		}
	}

	if len(commitNodes) == 0 {
		return nil
	}

	for _, commitNode := range commitNodes {
		commitHash := commitNode.Hash
		if commitHash == "" {
			continue
		}

		// Query git for changed files.
		files, err := listCommitFiles(ctx, runner, commitHash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: git diff-tree for commit %s: %v\n", commitHash[:8], err)
			continue
		}

		for _, file := range files {
			// Add work↔files edge.
			g.AddEdge(Edge{
				Kind:   EdgeWorkFiles,
				From:   graphNodeID(GraphKindCommit, commitHash),
				To:     "file:" + file,
				Source: "git diff-tree --name-only",
			})
		}
	}

	return nil
}

// extractPRNum extracts the PR number from a PR node ID.
// PR node IDs are formatted as "pr:owner/repo:100".
func extractPRNum(nodeID string) string {
	// Strip "pr:" prefix.
	if !strings.HasPrefix(nodeID, "pr:") {
		return ""
	}
	rest := strings.TrimPrefix(nodeID, "pr:")
	// The rest is "owner/repo:PRNum". Extract the last colon-separated segment.
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		return rest[i+1:]
	}
	return ""
}

// listPRFiles queries `gh pr view <number> --json files` to get changed files
// for the given PR. Returns an empty slice (not error) when no files are found.
func listPRFiles(ctx context.Context, runner GHRunner, repo string, prNum int) ([]string, error) {
	args := []string{"pr", "view", fmt.Sprintf("%d", prNum), "--json", "files"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}

	out, err := runner.Run(ctx, args...)
	if err != nil {
		return nil, err
	}

	var result struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parse gh pr view: %w", err)
	}

	var files []string
	for _, f := range result.Files {
		if f.Path != "" {
			files = append(files, f.Path)
		}
	}
	return files, nil
}

// listCommitFiles queries `git diff-tree --no-commit-id -r --name-only <hash>`
// to get changed files for the given commit. Returns an empty slice (not error)
// when no files are found.
func listCommitFiles(ctx context.Context, runner GHRunner, commitHash string) ([]string, error) {
	args := []string{"diff-tree", "--no-commit-id", "-r", "--name-only", commitHash}

	out, err := runner.Run(ctx, args...)
	if err != nil {
		return nil, err
	}

	if out == "" {
		return nil, nil
	}

	// Split by newline and trim whitespace.
	var files []string
	for _, line := range strings.Split(out, "\n") {
		file := strings.TrimSpace(line)
		if file != "" {
			files = append(files, file)
		}
	}
	return files, nil
}
