package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGit answers the git queries start makes, and records every command.
type fakeGit struct {
	calls     []string
	worktrees string // `git worktree list --porcelain` output
	branches  map[string]bool
	onCreate  func(argv []string)
	failOn    string // command prefix that fails
	top       string // `git rev-parse --show-toplevel`; "" makes it fail
}

func (f *fakeGit) run(name string, args ...string) (string, error) {
	argv := append([]string{name}, args...)
	cmd := strings.Join(argv, " ")
	f.calls = append(f.calls, cmd)
	if f.failOn != "" && strings.HasPrefix(cmd, f.failOn) {
		return "boom", errors.New("exit status 1")
	}
	switch {
	case cmd == "git symbolic-ref --short refs/remotes/origin/HEAD":
		return "origin/main\n", nil
	case cmd == "git rev-parse --path-format=absolute --git-common-dir":
		return "/src/specsync/.git\n", nil
	case cmd == "git remote get-url origin":
		return "git@github.com:acme/specsync.git\n", nil
	case cmd == "git worktree list --porcelain":
		return f.worktrees, nil
	case strings.HasPrefix(cmd, "git rev-parse --verify --quiet refs/heads/"):
		if f.branches[strings.TrimPrefix(cmd, "git rev-parse --verify --quiet refs/heads/")] {
			return "", nil
		}
		return "", errors.New("exit status 1")
	case strings.HasPrefix(cmd, "git worktree add"), strings.Contains(cmd, "worktree create"):
		if f.onCreate != nil {
			f.onCreate(argv)
		}
		return "", nil
	case cmd == "git rev-parse --show-toplevel":
		if f.top == "" {
			return "", errors.New("not stubbed")
		}
		return f.top + "\n", nil
	case strings.HasPrefix(cmd, "git cat-file"):
		return "", errors.New("not on the remote") // the plan is never on origin here
	}
	return "", nil
}

func (f *fakeGit) ran(prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func startTestEnv(f *fakeGit, goos string, onPath map[string]string, envVars map[string]string) (startEnv, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return startEnv{
		run: f.run,
		lookPath: func(name string) (string, error) {
			if p, ok := onPath[name]; ok {
				return p, nil
			}
			return "", exec.ErrNotFound
		},
		getenv: func(k string) string { return envVars[k] },
		goos:   goos,
		out:    out,
	}, out
}

// newStartChange creates openspec/changes/<slug>, linked to issue when issue
// is not empty.
func newStartChange(t *testing.T, slug, issue string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "openspec")
	dir := filepath.Join(root, "changes", slug)
	if err := os.MkdirAll(filepath.Join(dir, ".specsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# Fix the login race\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if issue != "" {
		refs := `{"github:acme/specsync":{"provider":"github","id":"` + issue + `","url":"https://github.com/acme/specsync/issues/` + issue + `"}}`
		if err := os.WriteFile(filepath.Join(dir, ".specsync", "refs.json"), []byte(refs), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func stageOf(t *testing.T, openspecDir, slug string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(openspecDir, "changes", slug, ".specsync", "metadata.json"))
	if err != nil {
		return ""
	}
	return string(b)
}

func TestResolveOrca(t *testing.T) {
	tests := []struct {
		name   string
		goos   string
		path   map[string]string
		env    map[string]string
		expect string
	}{
		{"env wins", "darwin", map[string]string{"orca": "/bin/orca"}, map[string]string{"ORCA_CLI_COMMAND": "/opt/orca"}, "/opt/orca"},
		{"mac uses orca", "darwin", map[string]string{"orca": "/bin/orca"}, nil, "/bin/orca"},
		{"linux uses orca-ide", "linux", map[string]string{"orca-ide": "/bin/orca-ide", "orca": "/bin/orca"}, nil, "/bin/orca-ide"},
		{"linux with only the screen reader", "linux", map[string]string{"orca": "/usr/bin/orca"}, nil, ""},
		{"nothing installed", "darwin", nil, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, _ := startTestEnv(&fakeGit{}, tt.goos, tt.path, tt.env)
			if got := resolveOrca(env.getenv, env.lookPath, env.goos); got != tt.expect {
				t.Fatalf("resolveOrca = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestStartUsesOrcaWhenInstalled(t *testing.T) {
	root := newStartChange(t, "fix-login-race", "42")
	f := &fakeGit{top: filepath.Dir(root)}
	f.onCreate = func([]string) {
		f.worktrees = "worktree /src/orca/specsync-fix\nHEAD abc\nbranch refs/heads/feat/42-fix-login-race\n"
	}
	env, out := startTestEnv(f, "darwin", map[string]string{"orca": "/bin/orca"}, nil)

	res, err := startChange(env, startOptions{OpenSpecDir: root, Slug: "fix-login-race", Launcher: "auto", Agent: "claude", Prompt: "Implement it"})
	if err != nil {
		t.Fatal(err)
	}
	want := "/bin/orca worktree create --name fix-login-race --branch feat/42-fix-login-race --base-branch origin/main --issue 42 --no-parent --agent claude --prompt Implement it"
	if !f.ran(want) {
		t.Fatalf("orca not run as expected; calls:\n%s", strings.Join(f.calls, "\n"))
	}
	if !f.ran("git fetch origin main") {
		t.Fatal("did not fetch the default branch")
	}
	if res.Path != "/src/orca/specsync-fix" {
		t.Fatalf("path = %q, want the one Orca chose", res.Path)
	}
	if !strings.Contains(stageOf(t, root, "fix-login-race"), `"active"`) {
		t.Fatal("stage not set to active")
	}
	if !strings.Contains(out.String(), "warning: openspec/changes/fix-login-race is not on origin/main") {
		t.Fatalf("no warning that the new worktree lacks the plan:\n%s", out.String())
	}
}

func TestStartFallsBackToGitOnLinuxWithOnlyScreenReader(t *testing.T) {
	root := newStartChange(t, "fix-login-race", "42")
	f := &fakeGit{}
	env, _ := startTestEnv(f, "linux", map[string]string{"orca": "/usr/bin/orca"}, nil)

	if _, err := startChange(env, startOptions{OpenSpecDir: root, Slug: "fix-login-race", Launcher: "auto"}); err != nil {
		t.Fatal(err)
	}
	if f.ran("/usr/bin/orca") {
		t.Fatal("ran the GNOME screen reader")
	}
	if !f.ran("git worktree add -b feat/42-fix-login-race /src/worktrees/specsync-42 origin/main") {
		t.Fatalf("git launcher not run as expected; calls:\n%s", strings.Join(f.calls, "\n"))
	}
}

func TestStartGitLauncherReusesExistingBranch(t *testing.T) {
	root := newStartChange(t, "fix-login-race", "42")
	f := &fakeGit{branches: map[string]bool{"feat/42-fix-login-race": true}}
	env, _ := startTestEnv(f, "darwin", nil, nil)

	if _, err := startChange(env, startOptions{OpenSpecDir: root, Slug: "fix-login-race", Launcher: "git"}); err != nil {
		t.Fatal(err)
	}
	if !f.ran("git worktree add /src/worktrees/specsync-42 feat/42-fix-login-race") {
		t.Fatalf("did not check out the existing branch; calls:\n%s", strings.Join(f.calls, "\n"))
	}
}

func TestStartReusesExistingWorktree(t *testing.T) {
	root := newStartChange(t, "fix-login-race", "42")
	f := &fakeGit{top: filepath.Dir(root), worktrees: "worktree /src/specsync\nbranch refs/heads/main\n\nworktree /elsewhere/wt\nbranch refs/heads/feat/42-fix-login-race\n"}
	env, out := startTestEnv(f, "darwin", map[string]string{"orca": "/bin/orca"}, nil)

	res, err := startChange(env, startOptions{OpenSpecDir: root, Slug: "fix-login-race", Launcher: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if f.ran("/bin/orca") || f.ran("git worktree add") || f.ran("git fetch") {
		t.Fatalf("created something for an existing worktree; calls:\n%s", strings.Join(f.calls, "\n"))
	}
	if res.Path != "/elsewhere/wt" || !strings.Contains(out.String(), "already has a worktree at /elsewhere/wt") {
		t.Fatalf("did not report the existing worktree: %q", out.String())
	}
	if strings.Contains(out.String(), "warning:") {
		t.Fatalf("warned about the plan for a worktree that already exists:\n%s", out.String())
	}
}

func TestStartRefusesChangeWithoutIssue(t *testing.T) {
	root := newStartChange(t, "fix-login-race", "")
	f := &fakeGit{}
	env, _ := startTestEnv(f, "darwin", nil, nil)

	_, err := startChange(env, startOptions{OpenSpecDir: root, Slug: "fix-login-race", Launcher: "git"})
	if err == nil || !strings.Contains(err.Error(), "specsync -change fix-login-race") {
		t.Fatalf("err = %v, want a refusal naming the sync command", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("ran commands before refusing: %v", f.calls)
	}
	if stageOf(t, root, "fix-login-race") != "" {
		t.Fatal("wrote metadata for a refused start")
	}
}

func TestStartDryRunChangesNothing(t *testing.T) {
	root := newStartChange(t, "fix-login-race", "42")
	f := &fakeGit{}
	env, out := startTestEnv(f, "darwin", nil, nil)

	if _, err := startChange(env, startOptions{OpenSpecDir: root, Slug: "fix-login-race", Launcher: "git", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if f.ran("git fetch") || f.ran("git worktree add") {
		t.Fatalf("dry run mutated; calls:\n%s", strings.Join(f.calls, "\n"))
	}
	if stageOf(t, root, "fix-login-race") != "" {
		t.Fatal("dry run wrote metadata")
	}
	for _, want := range []string{"feat/42-fix-login-race", "/src/worktrees/specsync-42", "git worktree add -b"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("dry run output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestStartOrcaLauncherRequiresOrca(t *testing.T) {
	root := newStartChange(t, "fix-login-race", "42")
	env, _ := startTestEnv(&fakeGit{}, "linux", map[string]string{"orca": "/usr/bin/orca"}, nil)
	_, err := startChange(env, startOptions{OpenSpecDir: root, Slug: "fix-login-race", Launcher: "orca"})
	if err == nil || !strings.Contains(err.Error(), "screen reader") {
		t.Fatalf("err = %v, want Orca-not-found naming the screen reader", err)
	}
}

func TestWorktreeLayout(t *testing.T) {
	path, branch := worktreeLayout("/src/worktrees", "acme/specsync", "42", "fix-login-race")
	if path != "/src/worktrees/specsync-42" || branch != "feat/42-fix-login-race" {
		t.Fatalf("got %q %q", path, branch)
	}
	if _, branch := worktreeLayout("/w", "specsync", "7", ""); branch != "feat/7" {
		t.Fatalf("branch without slug = %q", branch)
	}
}

func TestResolveWorktreeDir(t *testing.T) {
	none := func(string) string { return "" }
	if got := resolveWorktreeDir("", none, "/src/specsync"); got != "/src/worktrees" {
		t.Fatalf("default = %q", got)
	}
	env := func(string) string { return "/tmp/wt" }
	if got := resolveWorktreeDir("", env, "/src/specsync"); got != "/tmp/wt" {
		t.Fatalf("env = %q", got)
	}
	if got := resolveWorktreeDir("/x", env, "/src/specsync"); got != "/x" {
		t.Fatalf("flag = %q", got)
	}
}

// TestStartEndToEnd runs the built binary against a real clone with the git
// launcher: the worktree must exist on feat/<issue>-<slug>, based on the
// remote default branch, and a second start must reuse it.
func TestStartEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	bin := filepath.Join(t.TempDir(), "specsync")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	base := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	origin := filepath.Join(base, "remote", "proj.git")
	git(base, "init", "-q", "--bare", "-b", "main", origin)
	repo := filepath.Join(base, "src", "proj")
	git(base, "clone", "-q", origin, repo)
	change := filepath.Join(repo, "openspec", "changes", "fix-login-race")
	if err := os.MkdirAll(change, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(change, "proposal.md"), []byte("# Fix the login race\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(repo, "add", "openspec")
	git(repo, "commit", "-q", "-m", "plan")
	git(repo, "push", "-q", "origin", "main")
	git(repo, "remote", "set-head", "origin", "--auto")
	// A local commit not on origin: the worktree must not include it.
	if err := os.WriteFile(filepath.Join(repo, "local.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(repo, "add", "local.txt")
	git(repo, "commit", "-q", "-m", "local only")
	if err := os.MkdirAll(filepath.Join(change, ".specsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(change, ".specsync", "refs.json"), []byte(`{"github":{"provider":"github","id":"42","url":"u"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	start := func() string {
		t.Helper()
		cmd := exec.Command(bin, "start", "-launcher", "git", "fix-login-race")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("start: %v\n%s", err, out)
		}
		return string(out)
	}
	out := start()

	wt := filepath.Join(base, "src", "worktrees", "proj-42")
	if got := git(wt, "symbolic-ref", "--short", "HEAD"); got != "feat/42-fix-login-race" {
		t.Fatalf("worktree branch = %q\n%s", got, out)
	}
	if git(wt, "rev-parse", "HEAD") != git(repo, "rev-parse", "origin/main") {
		t.Fatal("worktree is not based on origin/main")
	}
	if _, err := os.Stat(filepath.Join(wt, "local.txt")); err == nil {
		t.Fatal("worktree contains a commit that is only on the local branch")
	}
	if !strings.Contains(stageOf(t, filepath.Join(repo, "openspec"), "fix-login-race"), `"active"`) {
		t.Fatalf("stage not active\n%s", out)
	}
	if strings.Contains(out, "warning:") {
		t.Fatalf("warned although the plan is on origin/main:\n%s", out)
	}

	again := start()
	if !strings.Contains(again, "already has a worktree") {
		t.Fatalf("second start did not reuse the worktree:\n%s", again)
	}
}
