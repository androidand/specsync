package specsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// WorkspaceEnv names the environment variable that lists workspace roots.
const WorkspaceEnv = "SPECSYNC_ROOTS"

const workspaceDepth = 2

// WorkspaceOptions says where to look. Roots come from Roots, else
// SPECSYNC_ROOTS, else the folder that holds the current clone.
type WorkspaceOptions struct {
	Roots []string
	Cwd   string

	// VolatileDirs are the temporary directories; nil means the system defaults.
	VolatileDirs []string

	getenv          func(string) string
	gitCommonDir    func(ctx context.Context, dir string) (string, error)
	gitWorktreeList func(ctx context.Context, repoDir string) (string, error)
	gitRemoteOrigin func(ctx context.Context, repoDir string) (string, error)
}

// WorkspaceWorktree is one worktree of a clone.
type WorkspaceWorktree struct {
	Path     string `json:"path"`
	Branch   string `json:"branch,omitempty"`
	Volatile bool   `json:"volatile,omitempty"`
}

// Clone is a main clone found under a root. Repo is the tracker repository
// mapped from its origin URL, empty when there is no origin.
type Clone struct {
	Path      string              `json:"path"`
	Repo      string              `json:"repo,omitempty"`
	HasSpecs  bool                `json:"hasSpecs,omitempty"`
	Worktrees []WorkspaceWorktree `json:"worktrees,omitempty"`
}

// Workspace is what discovery found. Status lines name what could not be
// mapped or listed; the rest is still returned.
type Workspace struct {
	Roots  []string `json:"roots"`
	Clones []Clone  `json:"clones"`
	Status []string `json:"status,omitempty"`
}

// DiscoverWorkspace finds main clones under the roots to a depth of two,
// lists their worktrees wherever they are, and maps each clone to a tracker
// repository. It only reads: nothing is written and no path is remembered.
func DiscoverWorkspace(ctx context.Context, opts WorkspaceOptions) (*Workspace, error) {
	if opts.getenv == nil {
		opts.getenv = os.Getenv
	}
	if opts.gitCommonDir == nil {
		opts.gitCommonDir = gitCommonDirReal
	}
	if opts.gitWorktreeList == nil {
		opts.gitWorktreeList = gitWorktreeListReal
	}
	if opts.gitRemoteOrigin == nil {
		opts.gitRemoteOrigin = gitRemoteOriginReal
	}
	if opts.Cwd == "" {
		opts.Cwd, _ = os.Getwd()
	}
	volatile := opts.VolatileDirs
	if volatile == nil {
		volatile = defaultVolatileDirs()
	}

	ws := &Workspace{}
	roots := opts.Roots
	if len(roots) == 0 {
		for _, r := range filepath.SplitList(opts.getenv(WorkspaceEnv)) {
			if r = strings.TrimSpace(r); r != "" {
				roots = append(roots, r)
			}
		}
	}
	if len(roots) == 0 {
		roots = []string{defaultRoot(ctx, opts)}
	}
	seenRoot := map[string]bool{}
	for _, r := range roots {
		abs, err := filepath.Abs(r)
		if err != nil || seenRoot[resolved(abs)] {
			continue
		}
		seenRoot[resolved(abs)] = true
		ws.Roots = append(ws.Roots, abs)
	}

	seen := map[string]bool{}
	for _, root := range ws.Roots {
		for _, path := range findClones(root, workspaceDepth) {
			real := resolved(path)
			if seen[real] {
				continue
			}
			seen[real] = true
			clone := Clone{Path: path}
			if info, err := os.Stat(filepath.Join(path, "openspec")); err == nil && info.IsDir() {
				clone.HasSpecs = true
			}
			if repo, err := opts.gitRemoteOrigin(ctx, path); err != nil || repo == "" {
				ws.Status = append(ws.Status, "clone "+path+" has no origin, so it is not mapped to a tracker repository")
			} else {
				clone.Repo = repo
			}
			entries, err := listWorktrees(ctx, opts.gitWorktreeList, path)
			if err != nil {
				ws.Status = append(ws.Status, "git worktree list failed in "+path+": "+errReason(err))
			}
			listed := map[string]bool{}
			for _, e := range entries {
				rp := resolved(e.path)
				if listed[rp] {
					continue
				}
				listed[rp] = true
				clone.Worktrees = append(clone.Worktrees, WorkspaceWorktree{Path: e.path, Branch: e.branch, Volatile: isVolatilePath(e.path, volatile)})
			}
			ws.Clones = append(ws.Clones, clone)
		}
	}
	sort.Slice(ws.Clones, func(i, j int) bool { return ws.Clones[i].Path < ws.Clones[j].Path })
	return ws, nil
}

// defaultRoot is the folder that holds the current clone's main checkout, or
// the current directory outside a git repository.
func defaultRoot(ctx context.Context, opts WorkspaceOptions) string {
	common, err := opts.gitCommonDir(ctx, opts.Cwd)
	if err != nil || common == "" {
		return opts.Cwd
	}
	return filepath.Dir(filepath.Dir(common))
}

// findClones returns the main clones at or under root, to depth levels below
// it. A main clone has a .git directory; a linked worktree has a .git file and
// is found through its clone instead. It does not descend into a clone.
func findClones(root string, depth int) []string {
	if info, err := os.Stat(filepath.Join(root, ".git")); err == nil && info.IsDir() {
		return []string{root}
	}
	if depth == 0 {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		out = append(out, findClones(filepath.Join(root, name), depth-1)...)
	}
	return out
}

func gitCommonDirReal(ctx context.Context, dir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
