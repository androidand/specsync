package specsync

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GitCommitSource reads commit history via the host `git` CLI. Like the GitHub
// provider it holds no library dependency — everything is shelled out, keeping
// the package stdlib-only and easy to fake in tests by swapping run.
type GitCommitSource struct {
	run func(ctx context.Context, args ...string) (string, error)
}

// NewGitCommitSource returns a source driven by the real `git` binary.
func NewGitCommitSource() *GitCommitSource {
	return &GitCommitSource{run: runGit}
}

// NewGitCommitSourceFunc returns a source driven by the given runner instead of
// the real `git` binary. Used in tests.
func NewGitCommitSourceFunc(run func(ctx context.Context, args ...string) (string, error)) *GitCommitSource {
	return &GitCommitSource{run: run}
}

func runGit(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// Field/record separators chosen to be vanishingly unlikely in commit text, so a
// single `git log` call parses unambiguously without per-commit spawns.
const (
	gitFieldSep  = "\x1f" // unit separator
	gitRecordSep = "\x1e" // record separator
)

// AllHistory, passed as `since`, means "walk the full history" — distinct from
// an empty since (which defaults to the latest tag). Used by change-scope gather.
const AllHistory = "\x00all"

// Commits returns the parsed commits in (since, until]. An empty since defaults
// to the most recent reachable tag, falling back to the full history when the
// repository has no tags; an empty until defaults to HEAD. When paths is
// non-empty, the log is restricted to commits touching those path globs.
func (g *GitCommitSource) Commits(ctx context.Context, since, until string, paths []string) ([]Commit, error) {
	if until == "" {
		until = "HEAD"
	}
	switch since {
	case AllHistory:
		since = "" // full history: no lower bound
	case "":
		tag, err := g.latestTag(ctx)
		if err != nil {
			return nil, err
		}
		since = tag // "" when no tag → full history below
	}

	revRange := until
	if since != "" {
		revRange = since + ".." + until
	}

	format := strings.Join([]string{"%H", "%an", "%aI", "%B"}, gitFieldSep) + gitRecordSep
	args := []string{"log", "--pretty=format:" + format, revRange}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	out, err := g.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseGitLog(out), nil
}

// latestTag returns the most recent reachable tag, or "" when none exists.
func (g *GitCommitSource) latestTag(ctx context.Context) (string, error) {
	out, err := g.run(ctx, "describe", "--tags", "--abbrev=0")
	if err != nil {
		// No tags is not an error: fall back to full history.
		if strings.Contains(err.Error(), "No names found") ||
			strings.Contains(err.Error(), "No tags can describe") ||
			strings.Contains(err.Error(), "cannot describe") {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

func parseGitLog(out string) []Commit {
	var commits []Commit
	for _, rec := range strings.Split(out, gitRecordSep) {
		rec = strings.Trim(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		fields := strings.SplitN(rec, gitFieldSep, 4)
		if len(fields) < 4 {
			continue
		}
		commits = append(commits, ParseCommit(
			strings.TrimSpace(fields[0]),
			strings.TrimSpace(fields[1]),
			strings.TrimSpace(fields[2]),
			fields[3],
		))
	}
	return commits
}
