package specsync

import (
	"reflect"
	"testing"
)

func TestParseCommitHeader(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		wantType string
		scope    string
		breaking bool
		desc     string
		ok       bool
	}{
		{"feat with scope and bang", "feat(ui)!: split the integration modal", "feat", "ui", true, "split the integration modal", true},
		{"fix minimal", "fix: correct off-by-one in slug", "fix", "", false, "correct off-by-one in slug", true},
		{"scope with slash", "refactor(core/sync): rename Ref key", "refactor", "core/sync", false, "rename Ref key", true},
		{"merge commit not conventional", "Merge branch 'main'", "", "", false, "Merge branch 'main'", false},
		{"freeform not conventional", "wip stuff", "", "", false, "wip stuff", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ParseCommit("abc123", "Dev", "2026-06-29", tt.msg)
			if c.ConventionalOK != tt.ok {
				t.Fatalf("ConventionalOK = %v, want %v", c.ConventionalOK, tt.ok)
			}
			if c.Type != tt.wantType || c.Scope != tt.scope || c.Breaking != tt.breaking || c.Description != tt.desc {
				t.Fatalf("got {type:%q scope:%q breaking:%v desc:%q}, want {type:%q scope:%q breaking:%v desc:%q}",
					c.Type, c.Scope, c.Breaking, c.Description, tt.wantType, tt.scope, tt.breaking, tt.desc)
			}
			if c.Raw != tt.msg {
				t.Fatalf("Raw not preserved: %q", c.Raw)
			}
		})
	}
}

func TestParseCommitBreakingFooter(t *testing.T) {
	msg := "refactor: rename Ref key\n\nSome body.\n\nBREAKING CHANGE: cache keys are now namespaced"
	c := ParseCommit("h", "", "", msg)
	if !c.Breaking {
		t.Fatalf("expected breaking from footer")
	}
	if c.BreakingFooter != "cache keys are now namespaced" {
		t.Fatalf("BreakingFooter = %q", c.BreakingFooter)
	}
}

func TestParseCommitRefs(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		wantIssues []string
		wantPRs    []string
	}{
		{"closing footer", "fix: thing\n\nCloses #42", []string{"#42"}, nil},
		{"trailing PR squash", "feat(ui): split modal (#51)", nil, []string{"#51"}},
		{"cross-repo issue", "fix: shared client\n\nFixes owner/repo#7", []string{"owner/repo#7"}, nil},
		{"pr and issue", "feat: bulk import (#51)\n\nCloses #42", []string{"#42"}, []string{"#51"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ParseCommit("h", "", "", tt.msg)
			if !reflect.DeepEqual(c.IssueRefs, tt.wantIssues) {
				t.Fatalf("IssueRefs = %v, want %v", c.IssueRefs, tt.wantIssues)
			}
			if !reflect.DeepEqual(c.PRRefs, tt.wantPRs) {
				t.Fatalf("PRRefs = %v, want %v", c.PRRefs, tt.wantPRs)
			}
		})
	}
}
