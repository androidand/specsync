package specsync

import (
	"context"
	"strings"
	"testing"
)

// TestGitHubPushRefusesOversizedBodyBeforeAnyCall pins #130: a body GitHub
// would reject is refused locally, with the facts a reader needs to act, and
// before any gh call is made.
func TestGitHubPushRefusesOversizedBodyBeforeAnyCall(t *testing.T) {
	var calls int
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		calls++
		return "", nil
	}}
	body := "- [ ] 1. small task\n\n" + wrapSection("Proposal", "proposal", strings.Repeat("x", GitHubBodyLimit+500))
	_, err := p.Push(context.Background(), WorkItem{Slug: "big-change", Title: "Big", Body: body}, nil)
	if err == nil {
		t.Fatal("Push accepted a body over the GitHub limit")
	}
	for _, want := range []string{"big-change", "65536", "proposal", "split"} {
		if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(want)) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if calls != 0 {
		t.Errorf("made %d gh calls before refusing", calls)
	}
}

func TestGitHubPushAllowsBodyAtTheLimit(t *testing.T) {
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "issue" && args[1] == "create" {
			return "https://github.com/o/r/issues/9", nil
		}
		return "[]", nil
	}}
	item := WorkItem{Slug: "ok", Title: "Ok"}
	item.Body = strings.Repeat("y", GitHubBodyLimit-len(p.renderBody(item)))
	if _, err := p.Push(context.Background(), item, nil); err != nil {
		t.Errorf("a body exactly at the limit was refused: %v", err)
	}
}
