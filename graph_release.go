package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// GitRunner is the interface for shelling out to `git`. Graph delegates to it
// so the work-graph layer stays testable without network or auth.
type GitRunner interface {
	// Run executes git with the given args and returns trimmed stdout.
	Run(ctx context.Context, args ...string) (string, error)
}

// gitRunner adapts the existing runGit function to the GitRunner interface.
type gitRunner struct{}

func (gitRunner) Run(ctx context.Context, args ...string) (string, error) {
	return runGit(ctx, args...)
}

// defaultGitRunner returns the default GitRunner implementation (real git CLI).
func defaultGitRunner() GitRunner {
	return gitRunner{}
}

// BuildReleaseEdges derives commit↔release edges for all commit nodes in g by
// querying `gh release` to find releases and `git tag --contains` to verify
// containment. It routes gh through a GHRunner and git through a GitRunner so
// it is mockable in tests.
//
// For each commit node in the graph, it:
//  1. Queries `gh release list --json tagName` to get all releases.
//  2. For each release, checks if the commit is contained via `git tag --contains`.
//  3. Adds a release node and a commit↔release edge for each match.
//
// Errors from gh or git are logged to stderr but do not stop the build — partial
// results are returned so the graph is still useful when either CLI is unavailable.
func (g *Graph) BuildReleaseEdges(ctx context.Context, repo string, ghRunner GHRunner, gitRunner GitRunner) error {
	if ghRunner == nil {
		ghRunner = defaultGHRunner()
	}
	if gitRunner == nil {
		gitRunner = defaultGitRunner()
	}

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

	// Query gh for all releases.
	releases, err := listReleases(ctx, ghRunner, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: gh release list: %v\n", err)
		return nil // partial results
	}

	if len(releases) == 0 {
		return nil
	}

	// For each commit, check which releases contain it.
	for _, commitNode := range commitNodes {
		commitHash := commitNode.Hash
		if commitHash == "" {
			continue
		}

		for _, release := range releases {
			// Check if the commit is contained in this release.
			contained, err := isCommitInRelease(ctx, gitRunner, commitHash, release.TagName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: git tag --contains %s %s: %v\n", release.TagName, commitHash[:8], err)
				continue
			}
			if !contained {
				continue
			}

			releaseNodeID := graphNodeID(GraphKindRelease, release.TagName)
			g.AddNode(Node{
				Kind:    GraphKindRelease,
				ID:      releaseNodeID,
				Label:   release.TagName,
				TagName: release.TagName,
			})

			// Add commit↔release edge.
			g.AddEdge(Edge{
				Kind:   EdgeCommitRel,
				From:   graphNodeID(GraphKindCommit, commitHash),
				To:     releaseNodeID,
				Source: "git tag --contains",
			})
		}
	}

	return nil
}

// releaseInfo holds minimal release data from gh.
type releaseInfo struct {
	TagName string
}

// listReleases queries `gh release list` to get all releases for the repo.
// Returns an empty slice (not error) when no releases are found.
func listReleases(ctx context.Context, runner GHRunner, repo string) ([]releaseInfo, error) {
	args := []string{"release", "list", "--json", "tagName"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}

	out, err := runner.Run(ctx, args...)
	if err != nil {
		return nil, err
	}

	if out == "" || out == "[]" {
		return nil, nil
	}

	var items []struct {
		TagName string `json:"tagName"`
	}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("parse gh release list: %w", err)
	}

	var releases []releaseInfo
	for _, item := range items {
		if item.TagName != "" {
			releases = append(releases, releaseInfo{TagName: item.TagName})
		}
	}
	return releases, nil
}

// isCommitInRelease checks if a commit is contained in a release by querying
// `git tag --contains <commit> <tag>`. Returns false if the commit is not
// contained or if the check fails.
func isCommitInRelease(ctx context.Context, runner GitRunner, commitHash, tagName string) (bool, error) {
	out, err := runner.Run(ctx, "tag", "--contains", commitHash, tagName)
	if err != nil {
		return false, err
	}

	// If the tag name appears in the output, the commit is contained.
	return strings.Contains(out, tagName), nil
}
