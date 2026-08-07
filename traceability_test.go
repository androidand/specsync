package specsync

import (
	"testing"
)

func TestTasksComplete(t *testing.T) {
	tests := []struct {
		name     string
		tasks    string
		expected bool
	}{
		{
			name:     "empty",
			tasks:    "",
			expected: false,
		},
		{
			name:     "no tasks",
			tasks:    "Some prose\nNo checkboxes here",
			expected: false,
		},
		{
			name:     "all done",
			tasks:    "- [x] Task one\n- [x] Task two",
			expected: true,
		},
		{
			name:     "partially done",
			tasks:    "- [x] Task one\n- [ ] Task two",
			expected: false,
		},
		{
			name:     "none done",
			tasks:    "- [ ] Task one\n- [ ] Task two",
			expected: false,
		},
		{
			name:     "dropped tasks excluded",
			tasks:    "- [x] Task one\n- [~] Dropped task",
			expected: true,
		},
		{
			name:     "moved tasks excluded",
			tasks:    "- [x] Task one\n- [>] Moved task",
			expected: true,
		},
		{
			name:     "only dropped",
			tasks:    "- [~] Dropped task",
			expected: false,
		},
		{
			name:     "only moved",
			tasks:    "- [>] Moved task",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TasksComplete(tc.tasks)
			if got != tc.expected {
				t.Errorf("TasksComplete(%q) = %v, want %v", tc.tasks, got, tc.expected)
			}
		})
	}
}

func TestGitHubProvider_ReferenceLine(t *testing.T) {
	p := NewGitHubProvider()

	tests := []struct {
		name            string
		ref             Ref
		allComplete     bool
		want            string
	}{
		{
			name:        "part of incomplete",
			ref:         Ref{ID: "42", URL: "https://github.com/owner/repo/issues/42"},
			allComplete: false,
			want:        "Part of #42",
		},
		{
			name:        "closes complete",
			ref:         Ref{ID: "42", URL: "https://github.com/owner/repo/issues/42"},
			allComplete: true,
			want:        "Closes #42",
		},
		{
			name:        "empty id",
			ref:         Ref{ID: "", URL: ""},
			allComplete: false,
			want:        "",
		},
		{
			name:        "empty id complete",
			ref:         Ref{ID: "", URL: ""},
			allComplete: true,
			want:        "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := p.ReferenceLine(tc.ref, tc.allComplete)
			if got != tc.want {
				t.Errorf("ReferenceLine(%v, %v) = %q, want %q", tc.ref, tc.allComplete, got, tc.want)
			}
		})
	}
}

// TestPhasedChangeReference tests the core invariant: a partially complete
// change must get "Part of #N", never "Closes #N". This is the regression
// test for the reported incident where PRs said Closes #N but only Phase 0
// of 4 had landed.
func TestPhasedChangeReference(t *testing.T) {
	p := NewGitHubProvider()

	// Phase 0 of 4: only the first task is done.
	phase0Tasks := `- [x] Phase 0: benchmark + baseline
- [ ] Phase 1: initial optimization
- [ ] Phase 2: advanced optimization
- [ ] Phase 3: integration testing
- [ ] Phase 4: documentation`

	// All phases complete.
	allCompleteTasks := `- [x] Phase 0: benchmark + baseline
- [x] Phase 1: initial optimization
- [x] Phase 2: advanced optimization
- [x] Phase 3: integration testing
- [x] Phase 4: documentation`

	ref := Ref{ID: "387", URL: "https://github.com/owner/repo/issues/387"}

	// Phase 0: should say "Part of", NOT "Closes".
	partOf := p.ReferenceLine(ref, TasksComplete(phase0Tasks))
	if partOf != "Part of #387" {
		t.Errorf("Phase 0 reference = %q, want %q", partOf, "Part of #387")
	}

	// All complete: should say "Closes".
	closes := p.ReferenceLine(ref, TasksComplete(allCompleteTasks))
	if closes != "Closes #387" {
		t.Errorf("All complete reference = %q, want %q", closes, "Closes #387")
	}
}
