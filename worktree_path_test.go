package specsync

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreePathTemplateNestsByRepoAndPrefixesTheIssue(t *testing.T) {
	cfg := WorktreeConfig{Dir: "../wt", Path: "{dir}/{repo}/{name}"}
	got, ok := WorktreePath(cfg, "/work/app", "7", "fix-it")
	if !ok || got != "/work/wt/app/7-fix-it" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	got, _ = WorktreePath(cfg, "/work/app", "", "fix-it")
	if got != "/work/wt/app/fix-it" {
		t.Fatalf("without an issue the name is the slug: %q", got)
	}
}

func TestWorktreePathWithoutATemplateSaysSoSoCallersKeepTheirLayout(t *testing.T) {
	if got, ok := WorktreePath(WorktreeConfig{Dir: "../wt"}, "/work/app", "7", "fix-it"); ok || got != "" {
		t.Fatalf("no template must report ok=false, got %q ok=%v", got, ok)
	}
}

func TestWorktreePathDirDefaultsToTheSiblingFolderAndMayBeAbsolute(t *testing.T) {
	got, _ := WorktreePath(WorktreeConfig{Path: "{dir}/{repo}/{name}"}, "/work/app", "7", "x")
	if got != "/work/worktrees/app/7-x" {
		t.Fatalf("default dir is %s: %q", DefaultWorktreeDir, got)
	}
	got, _ = WorktreePath(WorktreeConfig{Dir: "/srv/trees", Path: "{dir}/{repo}/{name}"}, "/work/app", "7", "x")
	if got != "/srv/trees/app/7-x" {
		t.Fatalf("absolute dir: %q", got)
	}
}

func TestWorktreePathTemplateIsCheckedWhenTheConfigLoads(t *testing.T) {
	cases := map[string]string{
		"worktrees:\n  path: \"{dir}/{repo}\"\n":         "share one folder",
		"worktrees:\n  path: \"{dir}/{repo}/{nme}\"\n":   "{nme}",
		"worktrees:\n  path: \"{dir}/{repo}/{name}\"\n":  "",
		"worktrees:\n  path: \"{dir}/{repo}-{slug}\"\n":  "",
		"worktrees:\n  path: \"{dir}/{repo}/{issue}\"\n": "share one folder",
	}
	for cfg, want := range cases {
		_, issues := ParseWays([]byte(cfg))
		var msgs []string
		for _, i := range issues {
			msgs = append(msgs, i.Msg)
		}
		all := strings.Join(msgs, "; ")
		if want == "" && len(issues) > 0 {
			t.Errorf("%q should be valid, got %s", cfg, all)
		}
		if want != "" && !strings.Contains(all, want) {
			t.Errorf("%q: want a finding containing %q, got %q", cfg, want, all)
		}
	}
}

func TestStartUsesTheWorktreePathTemplate(t *testing.T) {
	clone := startRepo(t, "worktrees:\n  dir: ../wt\n  path: \"{dir}/{repo}/{name}\"\n")
	res, err := startIn(t, clone, StartOptions{Issue: "42"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(clone), "wt", filepath.Base(clone), "42-add-x")
	if res.Path != want {
		t.Fatalf("path=%q want %q", res.Path, want)
	}
	if again, err := startIn(t, clone, StartOptions{Issue: "42"}); err != nil || !again.Existing || resolved(absPath(again.Path)) != resolved(absPath(want)) {
		t.Fatalf("a second start must find the same worktree: %v %+v", err, again)
	}
}

func TestStartWithoutATemplateKeepsTheRepoDashSlugLayout(t *testing.T) {
	clone := startRepo(t, "worktrees:\n  dir: ../wt\n")
	res, err := startIn(t, clone, StartOptions{Issue: "42"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(filepath.Dir(clone), "wt", filepath.Base(clone)+"-add-x"); res.Path != want {
		t.Fatalf("path=%q want %q", res.Path, want)
	}
}

func TestOrcaWorkspaceNameFollowsTheTemplateOnlyWhenOneIsSet(t *testing.T) {
	with := orcaCreateArgs(StartOptions{Orca: "orca", Slug: "add-x", Issue: "7", Ways: Ways{Worktrees: WorktreeConfig{Path: "{dir}/{repo}/{name}"}}}, "feat/7-add-x", "origin/main")
	without := orcaCreateArgs(StartOptions{Orca: "orca", Slug: "add-x", Issue: "7"}, "feat/7-add-x", "origin/main")
	if got := argAfter(with, "--name"); got != "7-add-x" {
		t.Fatalf("with a template the name is %q", got)
	}
	if got := argAfter(without, "--name"); got != "add-x" {
		t.Fatalf("without one the name stays the slug, got %q", got)
	}
}

func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestPullWorktreePlanReadsTheSameConfigAsStart(t *testing.T) {
	clone := startRepo(t, "worktrees:\n  dir: ../wt\n  path: \"{dir}/{repo}/{name}\"\n  branch: \"{type}/{issue}-{slug}\"\n")
	w, _ := LoadWays(clone)
	path, branch := PullWorktreePlan(w, clone, "app", "42", "add-x", "", "")
	if want := filepath.Join(filepath.Dir(clone), "wt", filepath.Base(clone), "42-add-x"); path != want || branch != "feat/42-add-x" {
		t.Fatalf("path=%q branch=%q", path, branch)
	}
	// What pull plans is what start then finds.
	res, err := startIn(t, clone, StartOptions{Issue: "42"})
	if err != nil || res.Path != path || res.Branch != branch {
		t.Fatalf("start must plan the same worktree: %v path=%q branch=%q", err, res.Path, res.Branch)
	}
}

func TestPullWorktreePlanWithoutATemplateKeepsItsOwnLayoutAndOverrides(t *testing.T) {
	root := "/work/app"
	path, branch := PullWorktreePlan(Ways{}, root, "app", "42", "add-x", "", "")
	if path != "/work/worktrees/app-42" || branch != "feat/42-add-x" {
		t.Fatalf("legacy layout: path=%q branch=%q", path, branch)
	}
	if path, _ := PullWorktreePlan(Ways{}, root, "app", "42", "", "", ""); path != "/work/worktrees/app-42" {
		t.Fatalf("no change name: %q", path)
	}
	if _, branch := PullWorktreePlan(Ways{}, root, "app", "42", "", "", ""); branch != "feat/42" {
		t.Fatalf("no change name keeps feat/<issue>: %q", branch)
	}
	cfg := Ways{Worktrees: WorktreeConfig{Dir: "../cfg", Path: "{dir}/{repo}/{name}"}}
	if path, _ := PullWorktreePlan(cfg, root, "app", "42", "add-x", "/flag", "/env"); path != "/flag/app/42-add-x" {
		t.Fatalf("the flag wins over env and config: %q", path)
	}
	if path, _ := PullWorktreePlan(cfg, root, "app", "42", "add-x", "", "/env"); path != "/env/app/42-add-x" {
		t.Fatalf("env wins over config: %q", path)
	}
}
