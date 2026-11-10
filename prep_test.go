package specsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func prepFor(t *testing.T, clone string, o PrepOptions) PrepReport {
	t.Helper()
	o.Dir = clone
	rep, err := PreparePR(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func checkOf(rep PrepReport, name string) PrepCheck {
	for _, c := range rep.Checks {
		if c.Name == name {
			return c
		}
	}
	return PrepCheck{}
}

func stampGates(t *testing.T, clone string) {
	t.Helper()
	path, err := gateStampPath(context.Background(), clone)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(gitFixture(t, clone, "rev-parse", "HEAD")+"\n"), 0o644)
}

var withGates = Ways{Gates: []Gate{{Name: "t", Run: "true"}}}

func TestPrepPassesWhenEverythingHolds(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFile(t, clone, "x.txt", "x")
	stampGates(t, clone)
	change := &Change{TasksMarkdown: "- [x] 1.1 a\n- [x] 1.2 b\n"}
	rep := prepFor(t, clone, PrepOptions{Ways: withGates, Change: change})
	if !rep.Passed {
		t.Fatalf("%+v", rep)
	}
	for _, n := range []string{"tree", "tasks", "fresh", "gates", "size"} {
		if checkOf(rep, n).Status != "pass" {
			t.Errorf("%s: %+v", n, checkOf(rep, n))
		}
	}
}

func TestPrepNamesTheGatesStepWhenNoValidStamp(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFile(t, clone, "x.txt", "x")
	rep := prepFor(t, clone, PrepOptions{Ways: withGates})
	c := checkOf(rep, "gates")
	if rep.Passed || c.Status != "fail" || !strings.Contains(c.Detail, "specsync gates") {
		t.Fatalf("%+v", rep)
	}
	stampGates(t, clone)
	commitFile(t, clone, "y.txt", "y")
	if c := checkOf(prepFor(t, clone, PrepOptions{Ways: withGates}), "gates"); c.Status != "fail" {
		t.Fatalf("a stamp for an older commit is stale: %+v", c)
	}
}

func TestPrepFailsOnOpenTasksDirtyTreeBehindBaseAndOversize(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	commitFile(t, clone, "x.txt", "x")
	stampGates(t, clone)
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("dirty\n"), 0o644)
	advanceRemote(t, origin, "main", "m.txt", "m")
	gitFixture(t, clone, "fetch", "-q")
	rep := prepFor(t, clone, PrepOptions{Ways: withGates, Change: &Change{TasksMarkdown: "- [x] a\n- [ ] b\n"}, BudgetFlag: "files=0,commits=0,lines=0"})
	for name, want := range map[string]string{"tree": "fail", "tasks": "fail", "fresh": "fail", "size": "pass"} {
		if got := checkOf(rep, name).Status; got != want {
			t.Errorf("%s = %s, want %s: %+v", name, got, want, checkOf(rep, name))
		}
	}
	if !strings.Contains(checkOf(rep, "tasks").Detail, "1 of 2") {
		t.Errorf("tasks detail = %q", checkOf(rep, "tasks").Detail)
	}
}

func TestPrepSizeOverBudgetFailsAndNoGatesOrChangeAreSkipped(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	for _, f := range []string{"1.txt", "2.txt", "3.txt"} {
		commitFile(t, clone, f, f)
	}
	rep := prepFor(t, clone, PrepOptions{BudgetFlag: "files=2"})
	if c := checkOf(rep, "size"); c.Status != "fail" || !strings.Contains(c.Detail, "files 3 > 2") {
		t.Fatalf("%+v", c)
	}
	if checkOf(rep, "gates").Status != "skipped" || checkOf(rep, "tasks").Status != "skipped" {
		t.Fatalf("no gates and no change are skipped, not failed: %+v", rep)
	}
}

func TestPrepAFailedSizeAnalysisIsAFailedCheckNotASkip(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFile(t, clone, "x.txt", "x")
	rep := prepFor(t, clone, PrepOptions{analyze: func(context.Context, string, string, string, []string) (*BranchSize, []string, error) {
		return nil, nil, errors.New("git exploded")
	}})
	c := checkOf(rep, "size")
	if c.Status != "fail" || !strings.Contains(c.Detail, "could not measure") || !strings.Contains(c.Detail, "git exploded") || c.Waivable {
		t.Fatalf("%+v", c)
	}
	if rep.Passed {
		t.Fatal("an unmeasurable branch is not ready")
	}
}

func TestPrepOnlyARealOverBudgetResultIsWaivable(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFile(t, clone, "1.txt", "1")
	commitFile(t, clone, "2.txt", "2")
	if c := checkOf(prepFor(t, clone, PrepOptions{BudgetFlag: "commits=1"}), "size"); c.Status != "fail" || !c.Waivable {
		t.Fatalf("over budget is waivable: %+v", c)
	}
	if c := checkOf(prepFor(t, clone, PrepOptions{BudgetFlag: "commits=abc"}), "size"); c.Status != "fail" || c.Waivable {
		t.Fatalf("a budget that does not parse is never waivable: %+v", c)
	}
}

func TestPrepMeasuresAgainstTheBaseGivenAndMarksAnUnmeasurableBaseAsBlocking(t *testing.T) {
	clone, origin := repoWithOrigin(t)
	commitFile(t, clone, "x.txt", "x")
	advanceRemote(t, origin, "main", "m.txt", "m")
	gitFixture(t, clone, "fetch", "-q")
	if c := checkOf(prepFor(t, clone, PrepOptions{Base: "origin/main"}), "fresh"); c.Status != "fail" || !strings.Contains(c.Detail, "origin/main") {
		t.Fatalf("freshness must use the base the pull request will use: %+v", c)
	}
	gitFixture(t, clone, "remote", "set-head", "origin", "-d")
	rep := prepFor(t, clone, PrepOptions{})
	for _, name := range []string{"fresh", "size"} {
		if c := checkOf(rep, name); c.Status != "skipped" || !c.Blocking {
			t.Errorf("%s with no measurable base must be skipped and blocking: %+v", name, c)
		}
	}
	if c := checkOf(rep, "gates"); c.Status != "skipped" || c.Blocking {
		t.Errorf("no gates configured is fine: %+v", c)
	}
	if c := checkOf(rep, "tasks"); c.Status != "skipped" || c.Blocking {
		t.Errorf("no matched change is fine: %+v", c)
	}
}
