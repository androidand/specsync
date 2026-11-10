package specsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func gitRepo(t *testing.T, dir, origin string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmds := [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"}}
	if origin != "" {
		cmds = append(cmds, []string{"remote", "add", "origin", origin})
	}
	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitIn2(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDiscoverWorkspace_FindsClonesWorktreesAndMapsOrigins(t *testing.T) {
	root := t.TempDir()
	api := filepath.Join(root, "api")
	web := filepath.Join(root, "group", "web")
	noOrigin := filepath.Join(root, "scratch")
	gitRepo(t, api, "https://github.com/acme/api.git")
	gitRepo(t, web, "git@github.com:acme/web.git")
	gitRepo(t, noOrigin, "")
	deep := filepath.Join(root, "a", "b", "c", "toodeep")
	gitRepo(t, deep, "https://github.com/acme/deep.git")
	if err := os.MkdirAll(filepath.Join(api, "openspec", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "api-wt")
	gitIn2(t, api, "worktree", "add", "-q", "-b", "feat/x", outside)

	ws, err := DiscoverWorkspace(context.Background(), WorkspaceOptions{Roots: []string{root}, VolatileDirs: []string{filepath.Dir(outside)}})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	byPath := map[string]Clone{}
	for _, c := range ws.Clones {
		paths = append(paths, c.Path)
		byPath[c.Path] = c
	}
	if want := []string{api, web, noOrigin}; !reflect.DeepEqual(paths, sortedCopy(want)) {
		t.Fatalf("clones = %v, want %v (depth 2, no linked worktrees as clones)", paths, want)
	}
	if byPath[api].Repo != "acme/api" || byPath[web].Repo != "acme/web" || byPath[noOrigin].Repo != "" {
		t.Fatalf("origins mapped wrongly: %+v", byPath)
	}
	if !byPath[api].HasSpecs || byPath[web].HasSpecs {
		t.Fatalf("hasSpecs wrong: %+v", byPath)
	}
	var found bool
	for _, w := range byPath[api].Worktrees {
		if resolved(w.Path) == resolved(outside) {
			found = true
			if !w.Volatile || w.Branch != "feat/x" {
				t.Fatalf("worktree outside the roots in a temp folder must be listed and volatile: %+v", w)
			}
		}
	}
	if !found {
		t.Fatalf("worktree outside the roots is missing: %+v", byPath[api].Worktrees)
	}
	if len(ws.Status) != 1 || !strings.Contains(ws.Status[0], noOrigin) {
		t.Fatalf("a clone with no origin gets one status line, got %v", ws.Status)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestDiscoverWorkspace_RootOrder(t *testing.T) {
	parent := t.TempDir()
	envRoot := t.TempDir()
	flagRoot := t.TempDir()
	gitRepo(t, filepath.Join(parent, "cur"), "https://github.com/acme/cur.git")
	gitRepo(t, filepath.Join(parent, "sib"), "https://github.com/acme/sib.git")
	gitRepo(t, filepath.Join(envRoot, "viaenv"), "https://github.com/acme/viaenv.git")
	gitRepo(t, filepath.Join(flagRoot, "viaflag"), "https://github.com/acme/viaflag.git")
	getenv := func(string) string { return envRoot }

	repos := func(o WorkspaceOptions) []string {
		ws, err := DiscoverWorkspace(context.Background(), o)
		if err != nil {
			t.Fatal(err)
		}
		var r []string
		for _, c := range ws.Clones {
			r = append(r, c.Repo)
		}
		return r
	}
	if got := repos(WorkspaceOptions{Roots: []string{flagRoot}, getenv: getenv}); !reflect.DeepEqual(got, []string{"acme/viaflag"}) {
		t.Fatalf("-root wins over the environment: %v", got)
	}
	if got := repos(WorkspaceOptions{getenv: getenv}); !reflect.DeepEqual(got, []string{"acme/viaenv"}) {
		t.Fatalf("the environment wins over the default: %v", got)
	}
	if got := repos(WorkspaceOptions{Cwd: filepath.Join(parent, "cur"), getenv: func(string) string { return "" }}); !reflect.DeepEqual(got, []string{"acme/cur", "acme/sib"}) {
		t.Fatalf("the default root is the folder holding the clone: %v", got)
	}
}

func TestDiscoverWorkspace_DefaultRootFromALinkedWorktree(t *testing.T) {
	parent := t.TempDir()
	main := filepath.Join(parent, "api")
	gitRepo(t, main, "https://github.com/acme/api.git")
	gitRepo(t, filepath.Join(parent, "web"), "https://github.com/acme/web.git")
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn2(t, main, "worktree", "add", "-q", "-b", "feat/y", wt)
	ws, err := DiscoverWorkspace(context.Background(), WorkspaceOptions{Cwd: wt, getenv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Clones) != 2 {
		t.Fatalf("running in a linked worktree must still use the main clone's parent folder: %+v", ws.Roots)
	}
}
