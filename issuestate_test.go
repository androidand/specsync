package specsync

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGitHubIssueStates_OneCallForManyRefs(t *testing.T) {
	var calls [][]string
	p := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		return `{"data":{"i0":{"issue":{"state":"OPEN","stateReason":null}},` +
			`"i1":{"issue":{"state":"CLOSED","stateReason":"NOT_PLANNED"}},` +
			`"i2":{"issue":{"state":"CLOSED","stateReason":"COMPLETED"}}}}`, nil
	})
	refs := []Ref{
		{Provider: "github:acme/app", ID: "1"},
		{Provider: "github:acme/app", ID: "2"},
		{Provider: "github:acme/api", ID: "3"},
	}
	got, err := p.IssueStates(context.Background(), refs)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("want one call for %d refs, got %d", len(refs), len(calls))
	}
	if !strings.Contains(strings.Join(calls[0], " "), `repository(owner:"acme",name:"api")`) {
		t.Errorf("query does not name the second repo: %v", calls[0])
	}
	want := map[string]IssueState{
		"github:acme/app#1": {State: "open"},
		"github:acme/app#2": {State: "closed", Reason: "not planned"},
		"github:acme/api#3": {State: "closed", Reason: "completed"},
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %+v, want %+v", k, got[k], w)
		}
	}
}

func TestGitHubIssueStates_MissingIssueIsUnknownForThatRefOnly(t *testing.T) {
	p := NewGitHubProviderFunc(func(context.Context, ...string) (string, error) {
		return "", errors.New(`gh api graphql: exit status 1
{"data":{"i0":{"issue":{"state":"OPEN","stateReason":null}},"i1":{"issue":null}},"errors":[{"type":"NOT_FOUND"}]}`)
	})
	got, err := p.IssueStates(context.Background(), []Ref{
		{Provider: "github:acme/app", ID: "1"},
		{Provider: "github:acme/app", ID: "999"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["github:acme/app#1"].State != "open" {
		t.Errorf("present issue = %+v", got["github:acme/app#1"])
	}
	if got["github:acme/app#999"].State != "unknown" {
		t.Errorf("missing issue = %+v, want unknown", got["github:acme/app#999"])
	}
}

func TestGitHubIssueStates_NonGitHubAndMalformedRefsAreUnknownWithoutACall(t *testing.T) {
	called := false
	p := NewGitHubProviderFunc(func(context.Context, ...string) (string, error) {
		called = true
		return "{}", nil
	})
	got, err := p.IssueStates(context.Background(), []Ref{
		{Provider: "beads", ID: "7"},
		{Provider: "github:acme/app", ID: "not-a-number"},
		{Provider: `github:acme/app"){x}`, ID: "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("no gh call is expected when no ref can be queried")
	}
	for k, v := range got {
		if v.State != "unknown" {
			t.Errorf("%s = %+v, want unknown", k, v)
		}
	}
}

func TestChangeDrift(t *testing.T) {
	done := "- [x] a\n- [x] b\n"
	partial := "- [x] a\n- [ ] b\n"
	cases := []struct {
		name string
		c    Change
		st   IssueState
		want string
	}{
		{"closed and still active", Change{TasksMarkdown: partial}, IssueState{State: "closed", Reason: "completed"}, "closed-unarchived"},
		{"closed and archived", Change{Archived: true, TasksMarkdown: done}, IssueState{State: "closed"}, ""},
		{"all checked, issue open", Change{TasksMarkdown: done}, IssueState{State: "open"}, "complete-open"},
		{"no tasks, issue open", Change{}, IssueState{State: "open"}, ""},
		{"unchecked tasks, issue open", Change{TasksMarkdown: partial}, IssueState{State: "open"}, ""},
		{"unknown never drifts", Change{TasksMarkdown: done}, IssueState{State: "unknown"}, ""},
		{"archived and all checked, issue open", Change{Archived: true, TasksMarkdown: done}, IssueState{State: "open"}, "complete-open"},
	}
	for _, c := range cases {
		if got := ChangeDrift(c.c, c.st); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
