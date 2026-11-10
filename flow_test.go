package specsync

import (
	"strings"
	"testing"
)

func zero() *int { return new(int) }

func approvedPR() *ChangeRequest {
	return &ChangeRequest{Number: 7, State: "open", ReviewDecision: "approved", Mergeable: "mergeable",
		Checks: ChangeRequestChecks{Pass: 3}, UnresolvedThreads: zero()}
}

func baseFacts() FlowFacts {
	return FlowFacts{HasChange: true, Slug: "add-forms", IssueID: "42", Branch: "feat/42-add-forms",
		Worktree: "/w/add-forms", TasksDone: 9, TasksTotal: 9, Head: "abc", GatesPassed: true, Trusted: true}
}

func current(t *testing.T, w Ways, f FlowFacts) StepState {
	t.Helper()
	pos := w.Position(f)
	if pos.Current == nil {
		t.Fatalf("no current step: %+v", pos)
	}
	return *pos.Current
}

func defaultWays() Ways {
	w, _ := ParseWays([]byte("gates:\n  - {name: lint, run: make lint}\n  - {name: test, run: make test}\n  - {name: e2e, run: make e2e, optional: true}\n"))
	return w
}

func TestFlowNoChangeSuggestsStartingWork(t *testing.T) {
	s := current(t, defaultWays(), FlowFacts{})
	if s.Step != "new" || !strings.Contains(s.Command, "specsync pull -issue") {
		t.Fatalf("%+v", s)
	}
}

func TestFlowChangeWithoutBranchSuggestsAWorktree(t *testing.T) {
	f := baseFacts()
	f.Branch, f.Worktree = "", ""
	s := current(t, defaultWays(), f)
	if s.Step != "start" || !strings.Contains(s.Command, "-worktree") {
		t.Fatalf("%+v", s)
	}
}

func TestFlowOpenTasksIsBuild(t *testing.T) {
	f := baseFacts()
	f.TasksDone = 7
	s := current(t, defaultWays(), f)
	if s.Step != "build" || !strings.Contains(s.Reason, "7/9") {
		t.Fatalf("%+v", s)
	}
}

func TestFlowGatesNotRunNamesTheGateCommands(t *testing.T) {
	f := baseFacts()
	f.GatesPassed = false
	s := current(t, defaultWays(), f)
	if s.Step != "gate" || s.Command != "make lint && make test" {
		t.Fatalf("optional gates stay out by default: %+v", s)
	}
	if !strings.Contains(s.Reason, "not run") {
		t.Fatalf("reason = %q", s.Reason)
	}
}

func TestFlowGatesUntrustedSaysSo(t *testing.T) {
	f := baseFacts()
	f.GatesPassed, f.Trusted = false, false
	s := current(t, defaultWays(), f)
	if !strings.Contains(s.Reason, "specsync trust") {
		t.Fatalf("an untrusted config must point at trust: %+v", s)
	}
}

func TestFlowDirtyTreeInvalidatesGates(t *testing.T) {
	f := baseFacts()
	f.Dirty = true
	if s := current(t, defaultWays(), f); s.Step != "gate" {
		t.Fatalf("uncommitted changes mean the stamp does not cover the work: %+v", s)
	}
}

func TestFlowNoGatesConfigured(t *testing.T) {
	f := baseFacts()
	f.GatesPassed = false
	w, _ := ParseWays(nil)
	s := current(t, w, f)
	if s.Step != "gate" || s.Command != "" || !strings.Contains(s.Reason, "no gates configured") {
		t.Fatalf("%+v", s)
	}
}

func TestFlowStrictRunsOptionalGates(t *testing.T) {
	f := baseFacts()
	f.GatesPassed = false
	w, _ := ParseWays([]byte("flow: strict\ngates:\n  - {name: t, run: make test}\n  - {name: e, run: make e2e, optional: true}\n"))
	if s := current(t, w, f); s.Command != "make test && make e2e" {
		t.Fatalf("%+v", s)
	}
}

func TestFlowNoPullRequestIsReview(t *testing.T) {
	s := current(t, defaultWays(), baseFacts())
	if s.Step != "review" || !strings.Contains(s.Command, "gh pr create") {
		t.Fatalf("%+v", s)
	}
}

func TestFlowDraftAndFailingAndWaiting(t *testing.T) {
	f := baseFacts()
	f.PR = approvedPR()
	f.PR.Draft = true
	if s := current(t, defaultWays(), f); s.Step != "review" || !strings.Contains(s.Command, "gh pr ready 7") {
		t.Fatalf("draft: %+v", s)
	}
	f.PR = approvedPR()
	f.PR.Checks = ChangeRequestChecks{Fail: 1, Failing: []string{"build"}}
	if s := current(t, defaultWays(), f); s.Step != "review" || !strings.Contains(s.Reason, "build") {
		t.Fatalf("failing: %+v", s)
	}
	f.PR = approvedPR()
	f.PR.ReviewDecision = "review_required"
	if s := current(t, defaultWays(), f); s.Step != "review" || !strings.Contains(s.Reason, "awaiting review") {
		t.Fatalf("waiting: %+v", s)
	}
}

func TestFlowApprovedGreenIsMerge(t *testing.T) {
	f := baseFacts()
	f.PR = approvedPR()
	s := current(t, defaultWays(), f)
	if s.Step != "merge" || s.Command != "gh pr merge 7" {
		t.Fatalf("%+v", s)
	}
}

func TestFlowStrictNeedsResolvedThreads(t *testing.T) {
	w, _ := ParseWays([]byte("flow: strict\n"))
	f := baseFacts()
	f.PR = approvedPR()
	n := 2
	f.PR.UnresolvedThreads = &n
	if s := current(t, w, f); s.Step != "review" || !strings.Contains(s.Reason, "unresolved") {
		t.Fatalf("%+v", s)
	}
	f.PR.UnresolvedThreads = nil
	if s := current(t, w, f); s.Step != "review" {
		t.Fatalf("an unknown thread count is not resolved: %+v", s)
	}
}

func TestFlowMergedWithWorktreeIsCleanup(t *testing.T) {
	f := baseFacts()
	f.PR = &ChangeRequest{Number: 7, State: "merged"}
	s := current(t, defaultWays(), f)
	if s.Step != "cleanup" || !strings.Contains(s.Command, "git worktree remove /w/add-forms") {
		t.Fatalf("%+v", s)
	}
	w, _ := ParseWays([]byte("cleanup: {archive: true}\n"))
	if s := current(t, w, f); !strings.Contains(s.Command, "specsync archive -change add-forms") {
		t.Fatalf("archive configured: %+v", s)
	}
}

func TestFlowMergedAndCleanIsComplete(t *testing.T) {
	f := baseFacts()
	f.Worktree = ""
	f.PR = &ChangeRequest{Number: 7, State: "merged"}
	pos := defaultWays().Position(f)
	if pos.Current != nil || !pos.Complete {
		t.Fatalf("%+v", pos)
	}
}

func TestFlowMinimalSkipsPullRequestSteps(t *testing.T) {
	w, _ := ParseWays([]byte("flow: minimal\n"))
	pos := w.Position(baseFacts())
	if !pos.Complete || len(pos.Steps) != 3 {
		t.Fatalf("%+v", pos)
	}
}

func TestFlowPositionMarksEarlierStepsDone(t *testing.T) {
	f := baseFacts()
	f.PR = approvedPR()
	f.PR.ReviewDecision = "review_required"
	pos := defaultWays().Position(f)
	want := map[string]bool{"new": true, "start": true, "build": true, "gate": true, "review": false, "merge": false, "cleanup": false}
	for _, s := range pos.Steps {
		if s.Done != want[s.Step] {
			t.Fatalf("step %s done=%v: %+v", s.Step, s.Done, pos.Steps)
		}
	}
	if pos.Current == nil || pos.Current.Step != "review" {
		t.Fatalf("%+v", pos.Current)
	}
}

func TestFlowFactsFromRow(t *testing.T) {
	row := ChangeTopology{Slug: "add-forms", Issue: &TopologyIssue{ID: "42"}, Branch: "feat/42-x", Worktree: "/w/x",
		Tasks: &TaskCount{Done: 2, Total: 5}, Local: &LocalState{Head: "abc", Modified: 1}}
	f := FactsFromRow(&row)
	if !f.HasChange || f.Slug != "add-forms" || f.IssueID != "42" || f.TasksTotal != 5 || !f.Dirty || f.Head != "abc" {
		t.Fatalf("%+v", f)
	}
	if f := FactsFromRow(&ChangeTopology{NoChange: true, Branch: "feat/x"}); f.HasChange {
		t.Fatalf("a branch that belongs to no change: %+v", f)
	}
	if f := FactsFromRow(nil); f.HasChange {
		t.Fatalf("%+v", f)
	}
}
