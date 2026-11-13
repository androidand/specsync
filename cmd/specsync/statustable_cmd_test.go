package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	specsync "github.com/androidand/specsync"
)

// A row whose pull request lookup did not happen says so under the table.
func TestStatusTableSaysWhenLookupsDidNotHappen(t *testing.T) {
	rows := []specsync.ChangeTopology{
		{Slug: "a", Title: "A", Repo: "acme/portal", RequestsUnknown: true},
		{Slug: "b", Title: "B", Repo: "acme/api"},
	}
	out := captureStdout(t, func() { printStatusTable(rows, "", nil, false, nil) })
	if !strings.Contains(out, "pull requests were not looked up for 1 of 2 rows") || !strings.Contains(out, "Unknown") || !strings.Contains(out, "Not opened") {
		t.Errorf("one row unknown, one not:\n%s", out)
	}
	out = captureStdout(t, func() { printStatusTable(rows[1:], "", nil, false, nil) })
	if strings.Contains(out, "not looked up") {
		t.Errorf("nothing unknown, yet a warning:\n%s", out)
	}
}

func TestUrlsNeedsTable(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	for _, cmd := range []string{"status", "overview"} {
		c := exec.Command(bin, cmd, "-urls")
		c.Dir = t.TempDir()
		out, err := c.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "-urls needs -table") {
			t.Errorf("%s -urls: err=%v, output %q; want a failure saying -urls needs -table", cmd, err, out)
		}
	}
}

// The table path must report what printRelated does: reasons a source was
// incomplete, and related work that resolved to no row.
func TestStatusTablePrintsRelatedWarnings(t *testing.T) {
	rows := []specsync.ChangeTopology{{Slug: "a", Title: "A", Repo: "acme/portal"}, {Slug: "b", Title: "B", Repo: "acme/api"}}
	rel := &specsync.RelatedWork{
		Incomplete: []string{"request state omitted for acme/api: boom"},
		Linked:     []specsync.RelatedItem{{Key: "acme/api#4"}},
	}
	out := captureStdout(t, func() { printStatusTable(rows, "", nil, false, rel) })
	for _, want := range []string{"incomplete: request state omitted for acme/api: boom", "acme/api#4 is related but was not found"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestTableAndJSONCannotBeCombined(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	for _, cmd := range []string{"status", "overview"} {
		c := exec.Command(bin, cmd, "-table", "-json")
		c.Dir = t.TempDir()
		out, err := c.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "-table and -json can't be combined") {
			t.Errorf("%s -table -json: err=%v, output %q", cmd, err, out)
		}
	}
}

func TestTableWidthOrder(t *testing.T) {
	asked := false
	term := func(n int) func() int { return func() int { asked = true; return n } }
	for _, c := range []struct {
		name, columns string
		terminal      int
		want          int
		asks          bool
	}{
		{"COLUMNS beats the terminal", "200", 90, 200, false},
		{"the terminal beats the default", "", 90, 90, true},
		{"not a terminal falls back", "", 0, 140, true},
		{"junk COLUMNS is ignored", "wide", 90, 90, true},
		{"zero COLUMNS is ignored", "0", 90, 90, true},
		{"negative COLUMNS is ignored", "-5", 0, 140, true},
	} {
		asked = false
		if got := pickTableWidth(c.columns, term(c.terminal)); got != c.want || asked != c.asks {
			t.Errorf("%s: width %d (terminal asked: %v), want %d (asked: %v)", c.name, got, asked, c.want, c.asks)
		}
	}
}

// status and overview share one gate: reviewers are fetched only when the output
// needs them and never offline.
func TestAttachReviewersGate(t *testing.T) {
	pr := &specsync.ChangeRequest{Number: 1, State: "open"}
	rows := []specsync.ChangeTopology{{Repo: "acme/portal", PR: pr}}
	for _, c := range []struct {
		wanted bool
		state  string
	}{{false, "on"}, {true, "off"}} {
		if notes := attachReviewers(context.Background(), rows, c.wanted, c.state); len(notes) != 0 || pr.ReviewersKnown {
			t.Errorf("wanted=%v state=%s: notes %v, known %v; nothing should have been asked", c.wanted, c.state, notes, pr.ReviewersKnown)
		}
	}
}
