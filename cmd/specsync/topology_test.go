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
	for in, want := range map[string]bool{"on": true, "off": true, "": true, "OFF": false, "maybe": false} {
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
