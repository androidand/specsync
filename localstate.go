package specsync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// LocalState is what git alone says about one worktree: uncommitted work,
// how far the branch is from its upstream and from its base, and how big it
// is. Every field is read from git; nothing is inferred. See
// openspec/changes/status-pr-and-worktree-state.
type LocalState struct {
	Modified     int         `json:"modified"`
	Untracked    int         `json:"untracked"`
	Upstream     string      `json:"upstream,omitempty"`
	Ahead        int         `json:"ahead"`
	Behind       int         `json:"behind"`
	Base         string      `json:"base,omitempty"`
	BehindBase   int         `json:"behindBase"`
	Size         *BranchSize `json:"size,omitempty"`
	LastCommit   string      `json:"lastCommit,omitempty"` // RFC 3339
	VolatilePath bool        `json:"volatilePath,omitempty"`
}

// BranchSize is the branch's size against its merge base with the base.
type BranchSize struct {
	Files     int `json:"files"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Commits   int `json:"commits"`
}

// LocalStateOptions configures CollectLocalState.
type LocalStateOptions struct {
	// Base overrides base-branch resolution (for example a pull request's
	// base). Empty means origin/HEAD, else the base is omitted.
	Base string
	// VolatileDirs are the directories treated as temporary. nil means the
	// system defaults; an empty non-nil slice means none.
	VolatileDirs []string
}

// CollectLocalState reads the state of the worktree at dir. It only reads:
// no fetch, no index refresh, no checkout. An error means dir is not a git
// worktree; any narrower failure omits just that field.
func CollectLocalState(ctx context.Context, dir string, opts LocalStateOptions) (*LocalState, error) {
	if _, err := gitIn(ctx, dir, "rev-parse", "--git-dir"); err != nil {
		return nil, err
	}
	ls := &LocalState{}

	if out, err := gitIn(ctx, dir, "--no-optional-locks", "status", "--porcelain=v1"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			switch {
			case line == "":
			case strings.HasPrefix(line, "??"):
				ls.Untracked++
			default:
				ls.Modified++
			}
		}
	}

	if up, err := gitIn(ctx, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		ls.Upstream = strings.TrimSpace(up)
		if out, err := gitIn(ctx, dir, "rev-list", "--left-right", "--count", "HEAD...@{u}"); err == nil {
			ls.Ahead, ls.Behind = twoInts(out)
		}
	}

	base := opts.Base
	if base == "" {
		if out, err := gitIn(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
			base = strings.TrimSpace(out)
		}
	}
	if base != "" {
		if out, err := gitIn(ctx, dir, "rev-list", "--count", "HEAD.."+base); err == nil {
			ls.Base = base
			ls.BehindBase, _ = strconv.Atoi(strings.TrimSpace(out))
			ls.Size = branchSize(ctx, dir, base)
		}
	}

	if out, err := gitIn(ctx, dir, "log", "-1", "--format=%cI"); err == nil {
		ls.LastCommit = strings.TrimSpace(out)
	}

	dirs := opts.VolatileDirs
	if dirs == nil {
		dirs = defaultVolatileDirs()
	}
	ls.VolatilePath = isVolatilePath(dir, dirs)
	return ls, nil
}

func branchSize(ctx context.Context, dir, base string) *BranchSize {
	size := &BranchSize{}
	out, err := gitIn(ctx, dir, "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return nil
	}
	size.Commits, _ = strconv.Atoi(strings.TrimSpace(out))
	diff, err := gitIn(ctx, dir, "diff", "--numstat", base+"...HEAD")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(diff, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 3 {
			continue
		}
		size.Files++
		a, _ := strconv.Atoi(f[0]) // "-" for binary files counts as 0
		d, _ := strconv.Atoi(f[1])
		size.Additions += a
		size.Deletions += d
	}
	return size
}

func twoInts(s string) (int, int) {
	f := strings.Fields(s)
	if len(f) != 2 {
		return 0, 0
	}
	a, _ := strconv.Atoi(f[0])
	b, _ := strconv.Atoi(f[1])
	return a, b
}

// gitIn runs git in dir and returns stdout without the trailing newline.
func gitIn(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func defaultVolatileDirs() []string {
	return []string{os.TempDir(), "/tmp", "/private/tmp", "/var/tmp"}
}

// isVolatilePath reports whether path is inside one of dirs, after resolving
// symlinks on both sides.
func isVolatilePath(path string, dirs []string) bool {
	path = resolved(path)
	for _, d := range dirs {
		d = resolved(d)
		if d != "" && (path == d || strings.HasPrefix(path, d+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

// resolved evaluates symlinks through the deepest existing ancestor of p, so
// a path that does not exist yet still compares equal to its real location.
func resolved(p string) string {
	p = filepath.Clean(p)
	rest := ""
	for cur := p; ; {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// LocalAttention derives the attention flags that git facts alone decide, in
// the documented vocabulary order. Each flag is a predicate over the state.
func LocalAttention(ls LocalState) []string {
	var flags []string
	add := func(cond bool, flag string) {
		if cond {
			flags = append(flags, flag)
		}
	}
	add(ls.Ahead > 0, "unpushed")
	add(ls.Modified+ls.Untracked > 0, "uncommitted")
	add(ls.Upstream == "", "no-upstream")
	add(ls.Behind > 0 && ls.Ahead == 0, "behind-upstream")
	add(ls.Behind > 0 && ls.Ahead > 0, "diverged")
	add(ls.BehindBase > 0, "behind-base")
	add(ls.VolatilePath, "volatile-path")
	return flags
}
