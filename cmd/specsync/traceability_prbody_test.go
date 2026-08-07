package main

import (
	"testing"
)

func TestEnsureNoDuplicateReference(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		allComplete bool
		issueID     string
		want        string
	}{
		{
			name:        "no duplicate to remove",
			body:        "This is my PR body\nWith multiple lines",
			allComplete: false,
			issueID:     "42",
			want:        "This is my PR body\nWith multiple lines",
		},
		{
			name:        "remove part of line",
			body:        "Part of #42\nThis is my PR body",
			allComplete: false,
			issueID:     "42",
			want:        "This is my PR body",
		},
		{
			name:        "remove closes line",
			body:        "Closes #42\nThis is my PR body",
			allComplete: true,
			issueID:     "42",
			want:        "This is my PR body",
		},
		{
			name:        "different issue not removed",
			body:        "Part of #43\n\nThis is my PR body",
			allComplete: false,
			issueID:     "42",
			want:        "Part of #43\n\nThis is my PR body",
		},
		{
			name:        "checkbox variant removed",
			body:        "- [x] Part of #42\nThis is my PR body",
			allComplete: false,
			issueID:     "42",
			want:        "This is my PR body",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ensureNoDuplicateReference(tc.body, tc.allComplete, tc.issueID)
			if got != tc.want {
				t.Errorf("ensureNoDuplicateReference(%q, %v, %q) = %q, want %q", tc.body, tc.allComplete, tc.issueID, got, tc.want)
			}
		})
	}
}

func TestBranchMatchesChange(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		slug   string
		want   bool
	}{
		{
			name:   "exact match",
			branch: "pr-issue-traceability",
			slug:   "pr-issue-traceability",
			want:   true,
		},
		{
			name:   "feat prefix",
			branch: "feat/pr-issue-traceability",
			slug:   "pr-issue-traceability",
			want:   true,
		},
		{
			name:   "feature prefix",
			branch: "feature/pr-issue-traceability",
			slug:   "pr-issue-traceability",
			want:   true,
		},
		{
			name:   "fix prefix",
			branch: "fix/pr-issue-traceability",
			slug:   "pr-issue-traceability",
			want:   true,
		},
		{
			name:   "chore prefix",
			branch: "chore/pr-issue-traceability",
			slug:   "pr-issue-traceability",
			want:   true,
		},
		{
			name:   "refactor prefix",
			branch: "refactor/pr-issue-traceability",
			slug:   "pr-issue-traceability",
			want:   true,
		},
		{
			name:   "nested branch with hyphen",
			branch: "feat/42-pr-issue-traceability",
			slug:   "42-pr-issue-traceability",
			want:   true,
		},
		{
			name:   "no match different slug",
			branch: "feat/other-change",
			slug:   "pr-issue-traceability",
			want:   false,
		},
		{
			name:   "no match partial",
			branch: "feat/pr-issue",
			slug:   "pr-issue-traceability",
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := branchMatchesChange(tc.branch, tc.slug)
			if got != tc.want {
				t.Errorf("branchMatchesChange(%q, %q) = %v, want %v", tc.branch, tc.slug, got, tc.want)
			}
		})
	}
}

func TestPRBodyReferencesIssue(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		issueNum  string
		want      bool
	}{
		{
			name:      "simple reference",
			body:      "This PR implements the feature\nRefs #42",
			issueNum:  "42",
			want:      true,
		},
		{
			name:      "closes reference",
			body:      "Closes #42\nAll tasks done",
			issueNum:  "42",
			want:      true,
		},
		{
			name:      "part of reference",
			body:      "Part of #42\nPhase 1 of 3",
			issueNum:  "42",
			want:      true,
		},
		{
			name:      "different issue",
			body:      "Refs #43\nNot related to anything",
			issueNum:  "42",
			want:      false,
		},
		{
			name:      "no reference",
			body:      "Just a regular PR body\nNo issue references",
			issueNum:  "42",
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := prBodyReferencesIssue(tc.body, tc.issueNum)
			if got != tc.want {
				t.Errorf("prBodyReferencesIssue(%q, %q) = %v, want %v", tc.body, tc.issueNum, got, tc.want)
			}
		})
	}
}
