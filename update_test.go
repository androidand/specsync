package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func advanceRemote(t *testing.T, origin, branch, file, content string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	gitFixture(t, filepath.Dir(other), "clone", "-q", "-b", branch, origin, other)
	commitFile(t, other, file, content)
	gitFixture(t, other, "push", "-q", "origin", branch)
}

func planAndApply(t *testing.T, clone string, ways Ways, apply, push bool) (UpdatePlan, UpdateResult, error) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	gitFixture(t, clone, "fetch", "-q")
	plan, err := PlanUpdate(context.Background(), clone, ways, false)
	if err != nil {
		t.Fatal(err)
	}
	if !apply {
		return plan, UpdateResult{}, nil
	}
	res, err := ApplyUpdate(context.Background(), clone, plan, push)
	return plan, res, err
}

func TestUpdatePlanOnlyChangesNothing(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	advanceRemote(t, origin, "main", "m.txt", "m")
	head := gitFixture(t, clone, "rev-parse", "HEAD")
	plan, _, _ := planAndApply(t, clone, Ways{}, false, false)
	if plan.Advice == nil || plan.Advice.Action != "rebase-onto-base" && plan.Advice.Action != "merge-base" {
		t.Fatalf("plan = %+v", plan)
	}
	if gitFixture(t, clone, "rev-parse", "HEAD") != head {
		t.Fatal("a plan must not move HEAD")
	}
}

func TestUpdateFastForwardsAnUpstreamThatMoved(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	advanceRemote(t, origin, "feature", "f.txt", "f")
	plan, res, err := planAndApply(t, clone, Ways{}, true, false)
	if err != nil || plan.Advice.Action != "fast-forward" || !res.Applied {
		t.Fatalf("%v %+v %+v", err, plan, res)
	}
	if _, statErr := os.Stat(filepath.Join(clone, "f.txt")); statErr != nil {
		t.Fatal("the upstream commit must be in the worktree")
	}
}

func TestUpdateRebasesOntoTheBaseAndPushesWithALeaseOnlyWhenAsked(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	commitFile(t, clone, "mine.txt", "mine")
	gitFixture(t, clone, "push", "-q")
	advanceRemote(t, origin, "main", "m.txt", "m")
	plan, res, err := planAndApply(t, clone, Ways{}, true, false)
	if err != nil || plan.Advice.Action != "rebase-onto-base" || !res.Applied || res.Pushed {
		t.Fatalf("%v %+v %+v", err, plan, res)
	}
	if n := gitFixture(t, clone, "rev-list", "--count", "origin/feature..HEAD"); n == "0" {
		t.Fatal("history was rewritten locally, so the branch must differ from the remote until -push")
	}
	if got := gitFixture(t, clone, "log", "--format=%s", "-3"); !strings.Contains(got, "add m.txt") {
		t.Fatalf("base commit missing: %q", got)
	}
}

func TestUpdatePushUsesALeaseAfterARebase(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	commitFile(t, clone, "mine.txt", "mine")
	gitFixture(t, clone, "push", "-q")
	advanceRemote(t, origin, "main", "m.txt", "m")
	plan, res, err := planAndApply(t, clone, Ways{}, true, true)
	if err != nil || !res.Pushed || !plan.Advice.ForcePush {
		t.Fatalf("%v %+v %+v", err, plan, res)
	}
	if n := gitFixture(t, clone, "rev-list", "--count", "origin/feature..HEAD"); n != "0" {
		t.Fatalf("the remote should match after the leased push, ahead=%s", n)
	}
}

func TestUpdateMergesSharedBranchesAndNeverRewritesThem(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	commitFile(t, clone, "mine.txt", "mine")
	gitFixture(t, clone, "-c", "user.name=Other", "-c", "user.email=other@example.com", "commit", "-q", "--allow-empty", "--author=Other <other@example.com>", "-m", "theirs")
	gitFixture(t, clone, "push", "-q")
	advanceRemote(t, origin, "main", "m.txt", "m")
	plan, res, err := planAndApply(t, clone, Ways{}, true, true)
	if err != nil || plan.Advice.Action != "merge-base" || plan.Advice.ForcePush || !res.Pushed {
		t.Fatalf("a shared branch is merged and pushed without force: %v %+v %+v", err, plan, res)
	}
	if gitFixture(t, clone, "rev-list", "--count", "--merges", "origin/main..HEAD") == "0" {
		t.Fatal("expected a merge commit")
	}
}

func TestUpdateHonoursTheMergeStrategyConfig(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	commitFile(t, clone, "mine.txt", "mine")
	advanceRemote(t, origin, "main", "m.txt", "m")
	plan, _, err := planAndApply(t, clone, Ways{Sync: SyncConfig{Strategy: "merge"}}, true, false)
	if err != nil || plan.Advice.Action != "merge-base" {
		t.Fatalf("%v %+v", err, plan)
	}
}

func TestUpdateRefusesADirtyTree(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	advanceRemote(t, origin, "main", "m.txt", "m")
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("dirty\n"), 0o644)
	head := gitFixture(t, clone, "rev-parse", "HEAD")
	_, _, err := planAndApply(t, clone, Ways{}, true, false)
	if err == nil || !strings.Contains(err.Error(), "commit or stash") {
		t.Fatalf("got %v", err)
	}
	if gitFixture(t, clone, "rev-parse", "HEAD") != head {
		t.Fatal("a dirty tree must not be touched")
	}
}

func TestUpdateStopsAtAConflictAndNamesTheFiles(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	commitFile(t, clone, "a.txt", "mine\n")
	advanceRemote(t, origin, "main", "a.txt", "theirs\n")
	plan, res, err := planAndApply(t, clone, Ways{}, true, true)
	if err == nil || len(res.Conflicts) != 1 || res.Conflicts[0] != "a.txt" || res.Pushed {
		t.Fatalf("%v %+v %+v", err, plan, res)
	}
	if !strings.Contains(err.Error(), "a.txt") || !strings.Contains(err.Error(), "--abort") {
		t.Fatalf("the error must name the files and how to back out: %v", err)
	}
}

func TestUpdateReportsNothingToDo(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	plan, _, _ := planAndApply(t, clone, Ways{}, false, false)
	if plan.Advice != nil {
		t.Fatalf("an up-to-date branch needs no steps: %+v", plan)
	}
}
