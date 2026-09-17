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
