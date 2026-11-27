package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/androidand/specsync"
)

// startEnv is everything `start` reaches outside the process: commands, the
// environment and the platform. Tests replace it with fakes.
type startEnv struct {
	run      func(name string, args ...string) (string, error)
	lookPath func(name string) (string, error)
	getenv   func(name string) string
	goos     string
	out      io.Writer
}

func realStartEnv() startEnv {
	return startEnv{
		run: func(name string, args ...string) (string, error) {
			// Combined and captured: probes stay quiet, and a failing
			// command's output reaches the user through the returned error.
			out, err := exec.CommandContext(context.Background(), name, args...).CombinedOutput()
			return string(out), err
		},
		lookPath: exec.LookPath,
		getenv:   os.Getenv,
		goos:     runtime.GOOS,
		out:      os.Stdout,
	}
}

type startOptions struct {
	OpenSpecDir string
	Slug        string
	Launcher    string // auto, orca or git
	WorktreeDir string
	Agent       string
	Prompt      string
	DryRun      bool
}

// startResult says what start did or, on a dry run, would do.
type startResult struct {
	Issue    string
	Branch   string
	Base     string // remote default branch, e.g. "main"
	Path     string // worktree path; for Orca, the path Orca chose
	Launcher string // "orca", "git", or "existing" when a worktree was reused
	Command  []string
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	openspec, storeFlag := addRootFlags(fs)
	launcher := fs.String("launcher", "auto", "auto, orca or git: auto uses Orca when it is installed")
	worktreeDir := fs.String("worktree-dir", "", "worktree base directory for the git launcher (default: $SPECSYNC_WORKTREE_DIR or ../worktrees beside the main checkout)")
	agent := fs.String("agent", "", "Orca only: agent to launch in the new workspace (e.g. claude, codex)")
	prompt := fs.String("prompt", "", "Orca only: initial prompt for the agent")
	doSync := fs.Bool("sync", false, "sync the change to the tracker after starting it")
	dryRun := fs.Bool("dry-run", false, "print the branch, worktree path and launcher command without changing anything")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 1)
	if fs.NArg() != 1 {
		fail(fmt.Errorf("usage: specsync start [flags] <change>"))
	}
	root := resolveRoot(fs, openspec, storeFlag).Dir

	if _, err := startChange(realStartEnv(), startOptions{
		OpenSpecDir: root,
		Slug:        fs.Arg(0),
		Launcher:    *launcher,
		WorktreeDir: *worktreeDir,
		Agent:       *agent,
		Prompt:      *prompt,
		DryRun:      *dryRun,
	}); err != nil {
		fail(err)
	}
	if *dryRun {
		if *doSync {
			fmt.Printf("  would sync: specsync -change %s\n", fs.Arg(0))
		}
		return
	}
	if *doSync {
		runSync([]string{"-openspec", root, "-change", fs.Arg(0)})
	}
}

// startChange creates or reuses the change's worktree and marks it active.
func startChange(env startEnv, opts startOptions) (*startResult, error) {
	if err := validateSlug(opts.Slug); err != nil {
		return nil, err
	}
	change, err := specsync.LoadChangeBySlug(opts.OpenSpecDir, opts.Slug)
	if err != nil {
		return nil, fmt.Errorf("change not found: %s", opts.Slug)
	}
	if change.Archived {
		return nil, fmt.Errorf("change %s is archived", opts.Slug)
	}
	issue, err := issueForChange(change.Dir)
	if err != nil {
		return nil, err
	}
	if issue == "" {
		return nil, fmt.Errorf("change %s has no linked issue yet; the branch and changelog need one — run `specsync -dry-run -change %s`, then `specsync -change %s`", opts.Slug, opts.Slug, opts.Slug)
	}

	orca, err := chooseLauncher(env, opts.Launcher)
	if err != nil {
		return nil, err
	}

	base, err := remoteDefaultBranch(env)
	if err != nil {
		return nil, err
	}
	mainRoot, err := mainCheckoutRoot(env)
	if err != nil {
		return nil, err
	}
	repoURL, _ := env.run("git", "remote", "get-url", "origin")
	path, branch := worktreeLayout(
		resolveWorktreeDir(opts.WorktreeDir, env.getenv, mainRoot),
		repoFromRemoteURL(strings.TrimSpace(repoURL), filepath.Base(mainRoot)),
		issue, opts.Slug)

	res := &startResult{Issue: issue, Branch: branch, Base: base, Path: path}
	if existing := worktreeForBranch(env, branch); existing != "" {
		res.Path, res.Launcher = existing, "existing"
	} else if orca != "" {
		res.Launcher = "orca"
		res.Command = append([]string{orca, "worktree", "create",
			"--name", opts.Slug,
			"--branch", branch,
			"--base-branch", "origin/" + base,
			"--issue", issue,
			"--no-parent"}, orcaAgentArgs(opts)...)
	} else {
		res.Launcher = "git"
		if branchExists(env, branch) {
			res.Command = []string{"git", "worktree", "add", path, branch}
		} else {
			res.Command = []string{"git", "worktree", "add", "-b", branch, path, "origin/" + base}
		}
	}

	if opts.DryRun {
		fmt.Fprintf(env.out, "DRY RUN — start %s (issue #%s):\n", opts.Slug, issue)
		fmt.Fprintf(env.out, "  branch: %s (from origin/%s)\n", branch, base)
		if res.Launcher == "existing" {
			fmt.Fprintf(env.out, "  worktree: %s (exists; nothing to create)\n", res.Path)
		} else {
			if res.Launcher == "git" {
				fmt.Fprintf(env.out, "  worktree: %s\n", res.Path)
			}
			fmt.Fprintf(env.out, "  would run: git fetch origin %s\n", base)
			fmt.Fprintf(env.out, "  would run: %s\n", shellJoin(res.Command))
		}
		fmt.Fprintf(env.out, "  would set stage: active\n")
		return res, nil
	}

	if res.Launcher != "existing" {
		if out, err := env.run("git", "fetch", "origin", base); err != nil {
			return nil, fmt.Errorf("fetch origin %s: %w\n%s", base, err, out)
		}
		if out, err := env.run(res.Command[0], res.Command[1:]...); err != nil {
			return nil, fmt.Errorf("%s: %w\n%s", shellJoin(res.Command), err, out)
		}
		if res.Launcher == "orca" {
			res.Path = worktreeForBranch(env, branch) // Orca chooses the path
		}
	}

	if err := markActive(change); err != nil {
		return nil, err
	}

	switch {
	case res.Launcher == "existing":
		fmt.Fprintf(env.out, "start: %s already has a worktree at %s (branch %s)\n", opts.Slug, res.Path, branch)
	case res.Path != "":
		fmt.Fprintf(env.out, "start: %s → %s (branch %s, via %s)\n", opts.Slug, res.Path, branch, res.Launcher)
	default:
		fmt.Fprintf(env.out, "start: %s → Orca workspace on branch %s\n", opts.Slug, branch)
	}
	fmt.Fprintf(env.out, "set-stage: %s → active\n", opts.Slug)
	if res.Launcher == "existing" {
		return res, nil
	}
	if rel, ok := changeRelPath(env, change.Dir); ok && !onRemoteBase(env, base, rel) {
		fmt.Fprintf(env.out, "warning: %s is not on origin/%s yet, so the new worktree does not contain it — merge the plan first or commit it on %s\n", rel, base, branch)
	}
	return res, nil
}

func orcaAgentArgs(opts startOptions) []string {
	var args []string
	if opts.Agent != "" {
		args = append(args, "--agent", opts.Agent)
	}
	if opts.Prompt != "" {
		args = append(args, "--prompt", opts.Prompt)
	}
	return args
}

// chooseLauncher returns the Orca executable to use, or "" for git.
func chooseLauncher(env startEnv, launcher string) (string, error) {
	switch launcher {
	case "git":
		return "", nil
	case "auto", "":
		return resolveOrca(env), nil
	case "orca":
		if orca := resolveOrca(env); orca != "" {
			return orca, nil
		}
		if env.goos == "linux" {
			return "", errors.New("-launcher orca: Orca not found; set ORCA_CLI_COMMAND or put orca-ide on PATH (bare `orca` on Linux is the GNOME screen reader)")
		}
		return "", errors.New("-launcher orca: Orca not found; set ORCA_CLI_COMMAND or put orca on PATH")
	default:
		return "", fmt.Errorf("-launcher must be auto, orca or git; got %q", launcher)
	}
}

// resolveOrca finds Orca's CLI the way Orca's own guide prescribes:
// $ORCA_CLI_COMMAND, then orca-ide on Linux, then orca. On Linux a bare
// `orca` is the GNOME screen reader, so it is never used there.
func resolveOrca(env startEnv) string {
	if v := strings.TrimSpace(env.getenv("ORCA_CLI_COMMAND")); v != "" {
		return v
	}
	name := "orca"
	if env.goos == "linux" {
		name = "orca-ide"
	}
	if p, err := env.lookPath(name); err == nil {
		return p
	}
	return ""
}

// issueForChange returns the issue number recorded in the change's ref cache,
// or "" when the change has never been synced.
func issueForChange(changeDir string) (string, error) {
	refs, err := specsync.LoadRefs(changeDir)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(refs))
	for k := range refs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Prefer GitHub refs: the branch convention numbers GitHub issues.
	sort.SliceStable(keys, func(i, j int) bool {
		return strings.HasPrefix(keys[i], "github") && !strings.HasPrefix(keys[j], "github")
	})
	for _, k := range keys {
		if id := refs[k].ID; digitsRE.MatchString(id) {
			return id, nil
		}
	}
	return "", nil
}

var digitsRE = regexp.MustCompile(`^\d+$`)

func remoteDefaultBranch(env startEnv) (string, error) {
	out, err := env.run("git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil || strings.TrimSpace(out) == "" {
		return "", errors.New("cannot tell the remote default branch; run `git remote set-head origin --auto`")
	}
	return strings.TrimPrefix(strings.TrimSpace(out), "origin/"), nil
}

// mainCheckoutRoot returns the main checkout's directory, even when called
// from a linked worktree, so worktrees land beside the repo rather than
// inside another worktree.
func mainCheckoutRoot(env startEnv) (string, error) {
	out, err := env.run("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return filepath.Dir(strings.TrimSpace(out)), nil
}

// resolveWorktreeDir picks the worktree base directory: the flag, then
// $SPECSYNC_WORKTREE_DIR, then ../worktrees. A relative path is taken from
// the main checkout.
func resolveWorktreeDir(flagValue string, getenv func(string) string, mainRoot string) string {
	dir := flagValue
	if dir == "" {
		dir = getenv("SPECSYNC_WORKTREE_DIR")
	}
	if dir == "" {
		dir = "../worktrees"
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(mainRoot, dir)
	}
	return filepath.Clean(dir)
}

// worktreeLayout names a change's worktree and branch: <dir>/<repo>-<issue>
// on feat/<issue>-<slug>, where <repo> is the repository name without its
// owner. pull -worktree and start share it.
func worktreeLayout(dir, repo, issue, slug string) (path, branch string) {
	name := repo[strings.LastIndex(repo, "/")+1:]
	branch = "feat/" + issue
	if slug != "" {
		branch += "-" + slug
	}
	return filepath.Join(dir, name+"-"+issue), branch
}

// repoFromRemoteURL turns a GitHub remote URL into owner/name, falling back
// to fallback when the URL is empty.
func repoFromRemoteURL(url, fallback string) string {
	url = strings.TrimSuffix(strings.TrimSpace(url), ".git")
	for _, p := range []string{"git@github.com:", "ssh://git@github.com/", "https://github.com/"} {
		if strings.HasPrefix(url, p) {
			return strings.TrimPrefix(url, p)
		}
	}
	if url == "" {
		return fallback
	}
	return url
}

// worktreeForBranch returns the path of the worktree that has branch checked
// out, or "".
func worktreeForBranch(env startEnv, branch string) string {
	out, err := env.run("git", "worktree", "list", "--porcelain")
	if err != nil {
		return ""
	}
	var path string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			path = strings.TrimPrefix(line, "worktree ")
		case line == "branch refs/heads/"+branch:
			return path
		}
	}
	return ""
}

func branchExists(env startEnv, branch string) bool {
	_, err := env.run("git", "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// changeRelPath returns the change folder relative to the checkout's top.
func changeRelPath(env startEnv, changeDir string) (string, bool) {
	top, err := env.run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", false
	}
	topDir, err := filepath.EvalSymlinks(strings.TrimSpace(top))
	if err != nil {
		return "", false
	}
	dir, err := filepath.EvalSymlinks(changeDir)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(topDir, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func onRemoteBase(env startEnv, base, rel string) bool {
	_, err := env.run("git", "cat-file", "-e", "origin/"+base+":"+rel+"/proposal.md")
	return err == nil
}

func markActive(change *specsync.Change) error {
	meta := changeMetadata(change)
	active := specsync.Stage("active")
	meta.Stage = &active
	return specsync.SaveChangeMetadata(change.Dir, meta)
}
