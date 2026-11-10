package specsync

import "testing"

func TestPRBodyReferencesIssue(t *testing.T) {
	tests := []struct {
		name, body, issue string
		want              bool
	}{
		{"simple reference", "This PR implements the feature\nRefs #42", "42", true},
		{"closes reference", "Closes #42\nAll tasks done", "42", true},
		{"part of reference", "Part of #42\nPhase 1 of 3", "42", true},
		{"different issue", "Refs #43\nNot related to anything", "42", false},
		{"no reference", "Just a regular PR body\nNo issue references", "42", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := PRBodyReferencesIssue(tc.body, tc.issue); got != tc.want {
				t.Errorf("PRBodyReferencesIssue(%q, %q) = %v, want %v", tc.body, tc.issue, got, tc.want)
			}
		})
	}
}

func TestForIssuePrefersOpenThenNewest(t *testing.T) {
	set := ChangeRequestSet{Requests: []ChangeRequest{
		{Number: 3, State: "merged", Branch: "a", Closes: []string{"9"}},
		{Number: 5, State: "closed", Branch: "b", Closes: []string{"9"}},
		{Number: 4, State: "open", Branch: "c", Closes: []string{"9"}},
		{Number: 6, State: "open", Branch: "d", Closes: []string{"10"}},
		{Number: 7, State: "open", Branch: "e", Closes: []string{"9"}, Fork: true},
	}}
	set.Repo = "acme/api"
	if got := set.ForIssue("acme/api", "9"); got == nil || got.Number != 4 {
		t.Fatalf("want the open request 4, got %+v", got)
	}
	set.Requests[2].State = "merged"
	if got := set.ForIssue("acme/api", "9"); got == nil || got.Number != 5 {
		t.Fatalf("want the newest non-open request 5, got %+v", got)
	}
	if set.ForIssue("acme/api", "11") != nil {
		t.Fatal("no request closes issue 11")
	}
}
