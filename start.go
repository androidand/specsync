package specsync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultWorktreeDir and DefaultBranchPattern apply when the config says nothing.
const (
	DefaultWorktreeDir   = "../worktrees"
	DefaultBranchPattern = "{type}/{issue}-{slug}"
)

// StartOptions describes one `specsync start`.
type StartOptions struct {
	RepoDir string // any directory of the repository
	// ConfigRoot is the directory whose openspec/specsync.yml Ways was read from;
	// trust is checked against that file, not against another checkout's copy.
	ConfigRoot string
	Ways       Ways
	Slug       string
	Issue      string // issue number, "" when unknown
	Type       string // branch type, default feat
	Base       string // start point for a new branch, default origin/HEAD, else HEAD
	// Sessions is what the session provider reported. SessionsChecked is false
	// when no provider is configured; a provider that failed sets SessionsErr.
	Sessions        []SessionEntry
	SessionsChecked bool
	SessionsErr     error
	DryRun          bool
}

// StartResult reports the worktree a start used or would create.
type StartResult struct {
	Path     string `json:"path"`
	Branch   string `json:"branch"`
	Existing bool   `json:"existing"`
	Created  bool   `json:"created"`
	Hooks    int    `json:"hooksRun"`
}

// BranchName expands a branch pattern. {issue} and the separator after it
// disappear when there is no issue.
func BranchName(pattern, typ, issue, slug string) string {
	if pattern == "" {
		pattern = DefaultBranchPattern
	}
	if typ == "" {
		typ = "feat"
	}
	if issue == "" {
		for _, sep := range []string{"-", "/", "_", "."} {
			pattern = strings.ReplaceAll(pattern, "{issue}"+sep, "")
		}
	}
	r := strings.NewReplacer("{type}", typ, "{issue}", issue, "{slug}", slug)
	return r.Replace(pattern)
}

var endedSession = map[string]bool{"ended": true, "exited": true, "closed": true, "archived": true, "stopped": true, "done": true, "finished": true}

func liveSessionIn(path string, entries []SessionEntry) *SessionEntry {
	root := resolved(absPath(path))
	for i, e := range entries {
		if endedSession[strings.ToLower(strings.TrimSpace(e.Status))] {
			continue
		}
		cwd := resolved(absPath(e.CWD))
		if cwd == root || strings.HasPrefix(cwd, root+string(filepath.Separator)) {
			return &entries[i]
		}
	}
	return nil
}

// StartWork finds or creates the worktree for a change using the repo's
// config: directory and branch pattern, then runs the start hooks in a new
// worktree and records which change the branch belongs to. It refuses when a
// live session sits in an existing worktree, and never runs hooks before the
// config is trusted.
func StartWork(ctx context.Context, o StartOptions) (StartResult, error) {
	var res StartResult
	root := mainCheckout(o.RepoDir)
	res.Branch = BranchName(o.Ways.Worktrees.Branch, o.Type, o.Issue, o.Slug)
	if _, err := gitIn(ctx, root, "check-ref-format", "--branch", res.Branch); err != nil {
		return res, fmt.Errorf("the branch pattern %q gives %q, which is not a valid branch name", o.Ways.Worktrees.Branch, res.Branch)
	}
	dir := o.Ways.Worktrees.Dir
	if dir == "" {
		dir = DefaultWorktreeDir
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	res.Path = filepath.Join(filepath.Clean(dir), filepath.Base(root)+"-"+o.Slug)

	existing, err := listWorktrees(ctx, gitWorktreeListReal, root)
	if err != nil {
		return res, err
	}
	for _, w := range existing {
		if w.branch == res.Branch || resolved(absPath(w.path)) == resolved(absPath(res.Path)) {
			res.Path, res.Branch, res.Existing = w.path, w.branch, true
			if o.SessionsChecked {
				if o.SessionsErr != nil {
					return res, fmt.Errorf("%s exists, but live sessions could not be checked (%v); nothing was changed", w.path, o.SessionsErr)
				}
				if s := liveSessionIn(w.path, o.Sessions); s != nil {
					return res, fmt.Errorf("a session is live in %s: %s %s; close it or resume it there instead", w.path, s.ID, s.Name)
				}
			}
			return res, nil
		}
	}

	hooks := o.Ways.Worktrees.Hooks
	configRoot := o.ConfigRoot
	if configRoot == "" {
		configRoot = root
	}
	if err := RequireTrust(configRoot, hooks); err != nil {
		return res, err
	}
	if o.DryRun {
		return res, nil
	}
	base := o.Base
	if base == "" {
		if b, err := DefaultBase(ctx, root); err == nil {
			base = b
		} else {
			base = "HEAD"
		}
	}
	if err := os.MkdirAll(filepath.Dir(res.Path), 0o755); err != nil {
		return res, err
	}
	if _, err := gitIn(ctx, root, "rev-parse", "--verify", "--quiet", "refs/heads/"+res.Branch); err == nil {
		_, err = gitIn(ctx, root, "worktree", "add", res.Path, res.Branch)
		if err != nil {
			return res, err
		}
	} else if _, err := gitIn(ctx, root, "worktree", "add", "--no-track", "-b", res.Branch, res.Path, base); err != nil {
		return res, err
	}
	res.Created = true
	if _, err := gitIn(ctx, root, "config", "branch."+res.Branch+".specsyncChange", o.Slug); err != nil {
		return res, err
	}
	for _, h := range hooks {
		cmd := exec.CommandContext(ctx, "sh", "-c", h)
		cmd.Dir = res.Path
		cmd.Env = append(os.Environ(), "SPECSYNC_WORKTREE="+res.Path, "SPECSYNC_BRANCH="+res.Branch, "SPECSYNC_CHANGE="+o.Slug, "SPECSYNC_ISSUE="+o.Issue)
		if out, err := cmd.CombinedOutput(); err != nil {
			return res, fmt.Errorf("start hook %q failed: %v: %s (the worktree %s was created; fix it or remove it with git worktree remove)", h, err, strings.TrimSpace(string(out)), res.Path)
		}
		res.Hooks++
	}
	return res, nil
}
