package specsync

import (
	"context"
	"errors"
	"fmt"
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
		return "", errors.New(`gh api graphql -f query={i0:repository(owner:"acme",name:"app"){issue(number:1){state stateReason}} i1:repository(owner:"acme",name:"app"){issue(number:999){state stateReason}} }: exit status 1
{"data":{"i0":{"issue":{"state":"OPEN","stateReason":null}},"i1":{"issue":null}},"errors":[{"type":"NOT_FOUND"}]}
gh: Could not resolve to an Issue with the number of 999.`)
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
		{"archived is never drift", Change{Archived: true, TasksMarkdown: done}, IssueState{State: "open"}, ""},
	}
	for _, c := range cases {
		if got := ChangeDrift(c.c, c.st); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func stubCountingAliases(calls *[]string) func(context.Context, ...string) (string, error) {
	return func(_ context.Context, args ...string) (string, error) {
		q := strings.Join(args, " ")
		*calls = append(*calls, q)
		n := strings.Count(q, "issue(number:")
		var parts []string
		for i := 0; i < n; i++ {
			parts = append(parts, fmt.Sprintf(`"i%d":{"issue":{"state":"OPEN","stateReason":null}}`, i))
		}
		return `{"data":{` + strings.Join(parts, ",") + `}}`, nil
	}
}

func TestGitHubIssueStates_BatchesOfFiftyAndNoDuplicates(t *testing.T) {
	var calls []string
	p := NewGitHubProviderFunc(stubCountingAliases(&calls))
	var refs []Ref
	for i := 1; i <= 120; i++ {
		refs = append(refs, Ref{Provider: "github:acme/app", ID: fmt.Sprint(i)})
	}
	refs = append(refs, refs[0], refs[1])
	got, err := p.IssueStates(context.Background(), refs)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 {
		t.Fatalf("120 distinct refs need 3 calls of at most 50, got %d", len(calls))
	}
	for _, q := range calls {
		if n := strings.Count(q, "issue(number:"); n > 50 {
			t.Errorf("a batch holds %d aliases", n)
		}
	}
	if len(got) != 120 || got["github:acme/app#120"].State != "open" {
		t.Errorf("got %d states, last = %+v", len(got), got["github:acme/app#120"])
	}
}
