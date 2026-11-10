package specsync

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeOrca stands in for `orca worktree create`: it creates a worktree like
// Orca does — on branch+suffix, tracking the base — and records its argv.
func fakeOrca(t *testing.T, suffix string) *[][]string {
	t.Helper()
	var calls [][]string
	prev := runOrcaFn
	runOrcaFn = func(ctx context.Context, dir string, argv []string) (string, error) {
		calls = append(calls, argv)
		var name, branch, base string
		for i := 0; i+1 < len(argv); i++ {
			switch argv[i] {
			case "--name":
				name = argv[i+1]
			case "--branch":
				branch = argv[i+1]
			case "--base-branch":
				base = argv[i+1]
			}
		}
		path := filepath.Join(filepath.Dir(dir), "orca-workspaces", name)
		return gitIn(ctx, dir, "worktree", "add", "--track", "-b", branch+suffix, path, base)
	}
	t.Cleanup(func() { runOrcaFn = prev })
	return &calls
}

func TestStartCreatesTheWorktreeThroughOrca(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "hook.log")
	clone := startRepo(t, "worktrees:\n  hooks: [\"echo $PWD >> "+marker+"\"]\n")
	if _, err := Trust(clone); err != nil {
		t.Fatal(err)
	}
	calls := fakeOrca(t, "")

	res, err := startIn(t, clone, StartOptions{Issue: "42", Orca: "/bin/orca", Agent: "claude", Prompt: "Implement it"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/bin/orca", "worktree", "create", "--name", "add-x", "--branch", "feat/42-add-x", "--base-branch", "origin/main", "--no-parent", "--issue", "42", "--agent", "claude", "--prompt", "Implement it"}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Fatalf("orca argv = %q", *calls)
	}
	if res.Launcher != "orca" || !res.Created || !strings.Contains(res.Path, "orca-workspaces") {
		t.Fatalf("%+v", res)
	}
	if b := gitFixture(t, res.Path, "rev-parse", "--abbrev-ref", "HEAD"); b != "feat/42-add-x" {
		t.Fatalf("branch = %q", b)
	}
	if log, _ := os.ReadFile(marker); !strings.Contains(string(log), "orca-workspaces") {
		t.Fatalf("the start hook must run in Orca's worktree: %q", log)
	}
	if got := gitFixture(t, clone, "config", "--get", "branch.feat/42-add-x.specsyncChange"); got != "add-x" {
		t.Fatalf("link = %q", got)
	}
	if out, err := gitIn(context.Background(), clone, "rev-parse", "--abbrev-ref", "feat/42-add-x@{upstream}"); err == nil {
		t.Fatalf("the branch Orca made must have no upstream, has %q", out)
	}
}

func TestStartRefusesWhenOrcaRenamedTheBranch(t *testing.T) {
	clone := startRepo(t, "")
	fakeOrca(t, "-2")
	_, err := startIn(t, clone, StartOptions{Issue: "42", Orca: "/bin/orca"})
	if err == nil || !strings.Contains(err.Error(), "no worktree on branch feat/42-add-x") {
		t.Fatalf("got %v", err)
	}
	if _, err := gitIn(context.Background(), clone, "config", "--get", "branch.feat/42-add-x.specsyncChange"); err == nil {
		t.Fatal("recorded a link for a branch that does not exist")
	}
}

func TestStartOrcaDryRunRunsNothing(t *testing.T) {
	clone := startRepo(t, "")
	calls := fakeOrca(t, "")
	res, err := startIn(t, clone, StartOptions{Orca: "/bin/orca", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 || res.Launcher != "orca" || len(res.Command) == 0 || res.Command[0] != "/bin/orca" {
		t.Fatalf("calls=%v res=%+v", *calls, res)
	}
	for _, a := range res.Command {
		if a == "--issue" {
			t.Fatalf("passed --issue for a change without one: %q", res.Command)
		}
	}
}

func TestStartWithoutOrcaNeverCallsIt(t *testing.T) {
	clone := startRepo(t, "")
	calls := fakeOrca(t, "")
	res, err := startIn(t, clone, StartOptions{Issue: "42"})
	if err != nil || res.Launcher != "git" || len(*calls) != 0 {
		t.Fatalf("err=%v launcher=%q calls=%v", err, res.Launcher, *calls)
	}
}

func TestWorktreeLauncherConfigIsChecked(t *testing.T) {
	w, issues := ParseWays([]byte("worktrees:\n  launcher: orca\n"))
	if len(issues) != 0 || w.Worktrees.Launcher != "orca" {
		t.Fatalf("launcher=%q issues=%v", w.Worktrees.Launcher, issues)
	}
	if _, issues := ParseWays([]byte("worktrees:\n  launcher: tmux\n")); len(issues) == 0 {
		t.Fatal("an unknown launcher must be reported when the config loads")
	}
}
