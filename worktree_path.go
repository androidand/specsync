package specsync

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// worktreePathPlaceholders are the names a worktrees.path template may use.
var worktreePathPlaceholders = []string{"{dir}", "{repo}", "{slug}", "{issue}", "{name}"}

var placeholderRE = regexp.MustCompile(`\{[^{}]*\}`)

// worktreeName is the folder name for one change: <issue>-<slug>, or the slug alone without an
// issue, or the issue alone when the change is not known yet.
func worktreeName(issue, slug string) string {
	switch {
	case issue != "" && slug != "":
		return issue + "-" + slug
	case issue != "":
		return issue
	default:
		return slug
	}
}

// resolveWorktreeDir turns the configured folder into an absolute one. A relative folder is taken
// from the main checkout.
func resolveWorktreeDir(dir, root string) string {
	if dir == "" {
		dir = DefaultWorktreeDir
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return filepath.Clean(dir)
}

func expandWorktreePath(tpl, dir, repo, issue, slug, root string) string {
	p := strings.NewReplacer(
		"{dir}", dir, "{repo}", repo, "{slug}", slug, "{issue}", issue, "{name}", worktreeName(issue, slug),
	).Replace(tpl)
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return filepath.Clean(p)
}

// WorktreePath expands worktrees.path for one change. ok is false when no template is configured,
// so a caller keeps its own layout and nothing changes for a repo that never sets the key.
func WorktreePath(cfg WorktreeConfig, root, issue, slug string) (string, bool) {
	if cfg.Path == "" {
		return "", false
	}
	return expandWorktreePath(cfg.Path, resolveWorktreeDir(cfg.Dir, root), filepath.Base(root), issue, slug, root), true
}

// checkWorktreePath reports what is wrong with a template, in words for the config check.
func checkWorktreePath(tpl string) []string {
	var out []string
	for _, ph := range placeholderRE.FindAllString(tpl, -1) {
		known := false
		for _, k := range worktreePathPlaceholders {
			known = known || ph == k
		}
		if !known {
			out = append(out, fmt.Sprintf("unknown placeholder %s in worktrees.path (known: %s)", ph, strings.Join(worktreePathPlaceholders, " ")))
		}
	}
	if !strings.Contains(tpl, "{slug}") && !strings.Contains(tpl, "{name}") {
		out = append(out, "worktrees.path must contain {slug} or {name}; without one, every change would share one folder")
	}
	return out
}

// orcaWorkspaceName is the name Orca gets for the workspace. With a path template it is the last
// folder of the expanded path, so Orca's folder matches the template. Without one it is the slug.
func orcaWorkspaceName(cfg WorktreeConfig, issue, slug string) string {
	if cfg.Path == "" {
		return slug
	}
	return filepath.Base(expandWorktreePath(cfg.Path, "/dir", "repo", issue, slug, "/dir"))
}

// PullWorktreePlan says where `pull -worktree` puts the worktree and which branch it uses, from the
// same config `start` reads. flagDir and envDir (the -worktree-dir flag and $SPECSYNC_WORKTREE_DIR)
// win over worktrees.dir. Without worktrees.path the layout stays <dir>/<repoName>-<issue>.
func PullWorktreePlan(ways Ways, root, repoName, issue, change, flagDir, envDir string) (path, branch string) {
	cfg := ways.Worktrees
	switch {
	case flagDir != "":
		cfg.Dir = flagDir
	case envDir != "":
		cfg.Dir = envDir
	}
	branch = "feat/" + issue
	if change != "" {
		branch = BranchName(cfg.Branch, "feat", issue, change)
	}
	if p, ok := WorktreePath(cfg, root, issue, change); ok {
		return p, branch
	}
	return filepath.Join(resolveWorktreeDir(cfg.Dir, root), repoName+"-"+issue), branch
}

// MainCheckout is the root of the repository's main checkout, whichever of its worktrees dir is in.
func MainCheckout(dir string) string { return mainCheckout(dir) }
