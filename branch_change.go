package specsync

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
)

// issuePrefixedSegmentRE matches a branch segment of the form "<issue>-<slug>",
// as in this repo's feat/<issue>-<slug> convention.
var issuePrefixedSegmentRE = regexp.MustCompile(`^\d+-(.+)$`)

// ChangeForBranch returns the slug of the active change that branch names, or
// "" when it names none. Only the branch's last path segment counts, so both
// feat/<issue>-<slug> and an orchestrator's <prefix>/<slug> resolve. The
// default branch never resolves, whatever changes exist.
func ChangeForBranch(openspecDir, branch, defaultBranch string) string {
	if branch == "" || branch == "HEAD" || branch == defaultBranch {
		return ""
	}
	segment := branch[strings.LastIndex(branch, "/")+1:]
	var candidates []string
	if m := issuePrefixedSegmentRE.FindStringSubmatch(segment); m != nil {
		candidates = append(candidates, m[1])
	}
	candidates = append(candidates, segment)
	for _, slug := range candidates {
		if isActiveChange(openspecDir, slug) {
			return slug
		}
	}
	return ""
}

func isActiveChange(openspecDir, slug string) bool {
	if slug == "" || slug == "archive" || strings.Contains(slug, "..") {
		return false
	}
	c, err := LoadChange(filepath.Join(openspecDir, "changes", slug), false, openspecDir)
	return err == nil && c != nil
}

// defaultBranchFn returns the remote default branch's short name, or "" when
// it cannot be determined. A variable so tests can override it.
var defaultBranchFn = func() string {
	out, err := runGit(context.Background(), "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(out), "origin/")
}

// ChangeFromCurrentBranch resolves the active change named by the checked-out
// branch. It returns the slug and the branch, or "" for the slug when the
// branch names no active change (including the default branch and a detached
// HEAD).
func ChangeFromCurrentBranch(openspecDir string) (slug, branch string) {
	branch = checkedOutBranchFn()
	if branch == "" {
		return "", ""
	}
	return ChangeForBranch(openspecDir, branch, defaultBranchFn()), branch
}

// checkedOutBranchFn returns the checked-out branch, or "" on a detached HEAD.
// symbolic-ref, unlike rev-parse, also works on a branch with no commits yet.
var checkedOutBranchFn = func() string {
	out, err := runGit(context.Background(), "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
