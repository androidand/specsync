package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// GHRunner is the interface for shelling out to `gh`. Graph delegates to it
// so the work-graph layer stays testable without network or auth.
type GHRunner interface {
	// Run executes gh with the given args and returns trimmed stdout.
	Run(ctx context.Context, args ...string) (string, error)
}

// ghAdapter adapts the existing runGH function to the GHRunner interface.
type ghAdapter struct{}

func (ghAdapter) Run(ctx context.Context, args ...string) (string, error) {
	return runGH(ctx, args...)
}

// defaultGHRunner returns the default GHRunner implementation (real gh CLI).
func defaultGHRunner() GHRunner {
	return ghAdapter{}
}

// BuildGHEdges derives issue↔PR↔commit edges for all issue nodes in g by
// querying `gh` for PRs linked to each issue, then the merge commit for each
// PR. It routes through a GHRunner so it is mockable in tests.
//
// For each issue node in the graph, it:
//  1. Extracts the issue number from the node ID.
//  2. Queries `gh pr list --issue <number>` to find linked PRs.
//  3. For each PR, adds a PR node and an issue↔PR edge.
//  4. For each PR, queries `gh pr view <number> --json mergeCommit` to get
//     the merge commit, adds a commit node and a PR↔commit edge.
//
// Errors from gh are logged to stderr but do not stop the build — partial
// results are returned so the graph is still useful when gh is unavailable.
func (g *Graph) BuildGHEdges(ctx context.Context, repo string, runner GHRunner) error {
	if runner == nil {
		runner = defaultGHRunner()
	}

	// Collect all issue nodes from the graph.
	var issueNodes []Node
	for _, n := range g.Nodes() {
		if n.Kind == GraphKindIssue {
			issueNodes = append(issueNodes, n)
		}
	}

	for _, issueNode := range issueNodes {
		issueNum := extractIssueNum(issueNode.ID)
		if issueNum == "" {
			continue
		}

		// Query gh for PRs linked to this issue.
		prs, err := listPRsForIssue(ctx, runner, repo, issueNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: gh pr list for issue %s: %v\n", issueNum, err)
			continue
		}

		for _, pr := range prs {
			prNodeID := graphNodeID(GraphKindPR, fmt.Sprintf("%s:%d", repo, pr.Number))
			g.AddNode(Node{
				Kind:     GraphKindPR,
				ID:       prNodeID,
				Label:    fmt.Sprintf("#%d: %s", pr.Number, pr.Title),
				IssueNum: fmt.Sprintf("%d", pr.Number),
			})

			// Add issue↔PR edge.
			g.AddEdge(Edge{
				Kind:   EdgeIssuePR,
				From:   issueNode.ID,
				To:     prNodeID,
				Source: "gh pr list --issue",
			})

			// Query gh for the merge commit.
			commit, err := getMergeCommit(ctx, runner, repo, pr.Number)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: gh pr view merge commit for PR #%d: %v\n", pr.Number, err)
				continue
			}
			if commit == "" {
				continue // PR not yet merged
			}

			commitNodeID := graphNodeID(GraphKindCommit, commit)
			g.AddNode(Node{
				Kind:       GraphKindCommit,
				ID:         commitNodeID,
				Label:      shortHash(commit) + " " + pr.Title,
				Hash:       commit,
				CommitDate: pr.MergedAt,
			})

			// Add PR↔commit edge.
			g.AddEdge(Edge{
				Kind:   EdgePRCommit,
				From:   prNodeID,
				To:     commitNodeID,
				Source: "gh pr view --json mergeCommit",
			})
		}
	}

	return nil
}

// prInfo holds minimal PR data from gh.
type prInfo struct {
	Number   int
	Title    string
	MergedAt string
}

// listPRsForIssue queries `gh api` to find PRs linked to the given issue.
// Returns an empty slice (not error) when no PRs are found.
func listPRsForIssue(ctx context.Context, runner GHRunner, repo, issueNum string) ([]prInfo, error) {
	// Use gh api to query the GitHub API for linked PRs.
	// The /issues/{number} endpoint includes a pull_requests field.
	args := []string{"api", fmt.Sprintf("repos/%s/issues/%s", repo, issueNum), "--jq", ".pull_requests // [] | .[] | {number: .number, title: .title, mergedAt: .merged_at}"}

	out, err := runner.Run(ctx, args...)
	if err != nil {
		return nil, err
	}

	if out == "" || out == "[]" {
		return nil, nil
	}

	var items []struct {
		Number   int    `json:"number"`
		Title    string `json:"title"`
		MergedAt string `json:"mergedAt"`
	}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("parse gh api: %w", err)
	}

	var prs []prInfo
	for _, item := range items {
		prs = append(prs, prInfo{
			Number:   item.Number,
			Title:    item.Title,
			MergedAt: item.MergedAt,
		})
	}
	return prs, nil
}

// getMergeCommit queries `gh pr view <number> --json mergeCommit` to get the
// merge commit SHA for the given PR. Returns "" if the PR is not merged.
func getMergeCommit(ctx context.Context, runner GHRunner, repo string, prNum int) (string, error) {
	args := []string{"pr", "view", fmt.Sprintf("%d", prNum), "--json", "mergeCommit"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}

	out, err := runner.Run(ctx, args...)
	if err != nil {
		return "", err
	}

	var result struct {
		MergeCommit struct {
			OID string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return "", fmt.Errorf("parse gh pr view: %w", err)
	}

	return result.MergeCommit.OID, nil
}

// extractIssueNum extracts the provider:issue portion from an issue node ID.
// Issue node IDs are formatted as "issue:github:owner/repo:42" or "issue:github:42".
func extractIssueNum(nodeID string) string {
	// Strip "issue:" prefix.
	if !strings.HasPrefix(nodeID, "issue:") {
		return ""
	}
	rest := strings.TrimPrefix(nodeID, "issue:")
	// The rest is "provider:issueNum" or "provider:owner/repo:issueNum".
	// Extract the last colon-separated segment.
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		return rest[i+1:]
	}
	return rest
}
