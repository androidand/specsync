package main

import (
	"strings"
	"testing"

	specsync "github.com/androidand/specsync"
)

func TestPrintTopologyTable_ResolvedRow(t *testing.T) {
	topo := specsync.Topology{
		Changes: []specsync.ChangeTopology{
			{
				Slug:  "add-field-forms",
				Title: "Add field forms",
				Stage: "active",
				Repo:  "acme/portal",
				Issue: &specsync.TopologyIssue{
					Provider: "github:acme/portal",
					ID:       "4231",
					URL:      "https://github.com/acme/portal/issues/4231",
					State:    "open",
				},
				Branch:   "feat/4231-add-field-forms",
				Worktree: "/repo/worktrees/add-field-forms",
				Epic:     "https://github.com/acme/planning/issues/88",
				Linked:   []string{"add-field-forms-api"},
			},
		},
	}

	out := captureStdout(t, func() { printTopologyTable(topo) })

	for _, want := range []string{
		"add-field-forms", "active", "acme/portal", "4231 (open)",
		"feat/4231-add-field-forms", "/repo/worktrees/add-field-forms",
		"epic: https://github.com/acme/planning/issues/88",
		"linked: add-field-forms-api",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestPrintTopologyTable_MissingFieldsRenderAsDash(t *testing.T) {
	topo := specsync.Topology{
		Changes: []specsync.ChangeTopology{
			{Slug: "no-worktree-yet", Stage: "active"},
		},
	}

	out := captureStdout(t, func() { printTopologyTable(topo) })

	if !strings.Contains(out, "no-worktree-yet") {
		t.Fatalf("expected the slug in output, got:\n%s", out)
	}
	if strings.Contains(out, "epic:") || strings.Contains(out, "linked:") {
		t.Errorf("expected no epic/linked lines for a row with neither, got:\n%s", out)
	}
}

func TestPrintTopologyTable_EmptyPrintsMessageNotBlank(t *testing.T) {
	out := captureStdout(t, func() { printTopologyTable(specsync.Topology{}) })
	if !strings.Contains(out, "No changes resolved.") {
		t.Errorf("expected an explicit empty-result message, got:\n%s", out)
	}
}

func TestPrintTopologyTable_StatusLinesAlwaysPrinted(t *testing.T) {
	topo := specsync.Topology{
		Status: []string{"gh unavailable: issue state omitted"},
	}
	out := captureStdout(t, func() { printTopologyTable(topo) })
	if !strings.Contains(out, "status: gh unavailable: issue state omitted") {
		t.Errorf("expected the status line surfaced, got:\n%s", out)
	}
}

func TestPrintTopologyTable_AttentionLine(t *testing.T) {
	topo := specsync.Topology{Changes: []specsync.ChangeTopology{
		{Slug: "needs-push", Stage: "active", Attention: []string{"unpushed", "uncommitted"}},
		{Slug: "all-clear", Stage: "active"},
	}}
	out := captureStdout(t, func() { printTopologyTable(topo) })
	if !strings.Contains(out, "attention: unpushed, uncommitted") {
		t.Errorf("expected an attention line, got:\n%s", out)
	}
	if strings.Count(out, "attention:") != 1 {
		t.Errorf("a row without attention must not print the line, got:\n%s", out)
	}
}

func TestPrintTopologyTable_NamesOverlapAndConflictFiles(t *testing.T) {
	topo := specsync.Topology{Changes: []specsync.ChangeTopology{{
		Slug: "stale", Stage: "active",
		Attention: []string{"behind-base", "base-touches-your-files", "conflicts-with-base"},
		Local: &specsync.LocalState{
			BaseTouchesYourFiles: []string{"src/a.go", "src/b.go"},
			Conflicts:            []string{"src/b.go"},
		},
	}, {
		Slug: "old-git", Stage: "active",
		Local: &specsync.LocalState{ConflictCheckUnavailable: true},
	}}}
	out := captureStdout(t, func() { printTopologyTable(topo) })
	for _, want := range []string{
		"base touches your files: src/a.go, src/b.go",
		"would conflict in: src/b.go",
		"conflict check unavailable",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	}
}

func TestPrintTopologyTable_SyncSuggestionShowsCommandsAndForceNotice(t *testing.T) {
	topo := specsync.Topology{Changes: []specsync.ChangeTopology{{
		Slug: "stale", Stage: "active",
		Sync: &specsync.SyncAdvice{
			Action:    "rebase-onto-base",
			Summary:   "rebase onto origin/main; a force push with lease is needed",
			Commands:  []string{"git rebase origin/main", "git push --force-with-lease"},
			ForcePush: true,
		},
	}}}
	out := captureStdout(t, func() { printTopologyTable(topo) })
	for _, want := range []string{
		"suggest: rebase onto origin/main; a force push with lease is needed",
		"$ git rebase origin/main",
		"$ git push --force-with-lease",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	}
}

func TestPrintTopologyTable_RequestLine(t *testing.T) {
	topo := specsync.Topology{Changes: []specsync.ChangeTopology{{
		Slug: "has-pr", Stage: "active",
		PR: &specsync.ChangeRequest{
			Number: 7, State: "open", URL: "https://github.com/acme/portal/pull/7",
			ReviewDecision: "changes_requested", Mergeable: "conflicting",
			Checks: specsync.ChangeRequestChecks{Pass: 3, Fail: 2, Pending: 1, Failing: []string{"ci/legacy", "unit"}},
		},
	}, {
		Slug: "draft-pr", Stage: "active",
		PR: &specsync.ChangeRequest{Number: 9, State: "open", Draft: true},
	}}}
	out := captureStdout(t, func() { printTopologyTable(topo) })
	for _, want := range []string{
		"pr #7 open: checks 3 pass, 2 fail, 1 pending; review changes_requested; conflicting",
		"failing checks: ci/legacy, unit",
		"pr #9 open (draft)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	}
}

func TestStatusCmd_StateFlag(t *testing.T) {
	fs := flagSetFor(&statusCmd{})
	if fs.Lookup("state") == nil {
		t.Fatal("status must define -state")
	}
	for in, want := range map[string]bool{"on": true, "off": true, "fresh": true, "": true, "OFF": false, "maybe": false} {
		if got := validState(in); got != want {
			t.Errorf("validState(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRequestLine_UnresolvedThreads(t *testing.T) {
	three := 3
	got := requestLine(&specsync.ChangeRequest{Number: 4, State: "open", UnresolvedThreads: &three})
	if !strings.Contains(got, "3 unresolved threads") {
		t.Fatalf("got %q", got)
	}
	zero := 0
	if got := requestLine(&specsync.ChangeRequest{Number: 4, State: "open", UnresolvedThreads: &zero}); strings.Contains(got, "unresolved") {
		t.Fatalf("zero is not worth a mention: %q", got)
	}
}

func currentRow() specsync.ChangeTopology {
	return specsync.ChangeTopology{
		Slug: "add-thing", Stage: "active", Repo: "acme/portal",
		Issue:    &specsync.TopologyIssue{ID: "42", State: "open", URL: "https://github.com/acme/portal/issues/42"},
		Branch:   "feat/42-add-thing",
		Worktree: "/work/add-thing",
		Tasks:    &specsync.TaskCount{Done: 2, Total: 5},
		PR:       &specsync.ChangeRequest{Number: 7, State: "open", Checks: specsync.ChangeRequestChecks{Pass: 3}},
		Local: &specsync.LocalState{
			Modified: 2, Untracked: 1, Upstream: "origin/feat/42-add-thing", Ahead: 2,
			Base: "origin/main", BehindBase: 4,
			Size: &specsync.BranchSize{Files: 6, Additions: 100, Deletions: 7, Commits: 3},
		},
		Attention: []string{"unpushed", "uncommitted", "behind-base"},
		Sync:      &specsync.SyncAdvice{Summary: "commit or stash your changes first"},
	}
}

func TestPrintCurrentStatus_AttentionComesFirstWithCounts(t *testing.T) {
	out := captureStdout(t, func() { printCurrentStatus(currentRow(), true) })
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if !strings.HasPrefix(lines[0], "add-thing") || !strings.Contains(lines[0], "active") || !strings.Contains(lines[0], "2/5 tasks") {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "attention:") {
		t.Fatalf("attention must be the first thing after the header, got:\n%s", out)
	}
	for _, want := range []string{"unpushed (2 commits)", "uncommitted (3 files)", "behind-base (4 commits)"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("attention line %q missing %q", lines[1], want)
		}
	}
	for _, want := range []string{
		"issue:", "#42 (open)",
		"branch:", "feat/42-add-thing", "origin/feat/42-add-thing", "/work/add-thing",
		"pr #7 open",
		"6 files, +100 -7, 3 commits",
		"suggest: commit or stash your changes first",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	}
}

func TestPrintCurrentStatus_NoAttentionLineWhenClear(t *testing.T) {
	row := currentRow()
	row.Attention = nil
	row.Sync = nil
	out := captureStdout(t, func() { printCurrentStatus(row, true) })
	if strings.Contains(out, "attention:") || strings.Contains(out, "suggest:") {
		t.Errorf("a clear row prints neither, got:\n%s", out)
	}
}

func TestPrintCurrentStatus_BranchWithoutAChange(t *testing.T) {
	row := specsync.ChangeTopology{
		NoChange: true, Branch: "scratch/idea", Worktree: "/work/idea",
		PR:    &specsync.ChangeRequest{Number: 3, State: "open"},
		Local: &specsync.LocalState{Upstream: "origin/scratch/idea"},
	}
	out := captureStdout(t, func() { printCurrentStatus(row, true) })
	for _, want := range []string{"no change", "scratch/idea", "pr #3 open"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	}
}

func TestPrintCurrentStatus_NoUpstreamAndNoRequest(t *testing.T) {
	row := specsync.ChangeTopology{Slug: "x", Stage: "active", Branch: "feat/x", Worktree: "/w", Local: &specsync.LocalState{}}
	out := captureStdout(t, func() { printCurrentStatus(row, true) })
	if !strings.Contains(out, "not pushed") || !strings.Contains(out, "pr: none") {
		t.Errorf("expected 'not pushed' and 'pr: none' in:\n%s", out)
	}
}

func TestStatusCmd_EverythingFlag(t *testing.T) {
	if flagSetFor(&statusCmd{}).Lookup("everything") == nil {
		t.Fatal("status must define -everything")
	}
}

func TestPrintCurrentStatus_OfflineSaysNothingAboutRequests(t *testing.T) {
	row := specsync.ChangeTopology{Slug: "x", Stage: "active", Branch: "feat/x", Worktree: "/w", Local: &specsync.LocalState{Upstream: "origin/feat/x"}}
	out := captureStdout(t, func() { printCurrentStatus(row, false) })
	if strings.Contains(out, "pr:") {
		t.Errorf("with -state=off a missing request is unknown, not none:\n%s", out)
	}
}
