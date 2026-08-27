package specsync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bigDesignNotes is long enough on its own to push a normal issue body past
// GitHubBodyLimit, without needing to construct a body that size by hand.
func bigDesignNotes() string {
	return strings.Repeat("design decision paragraph. ", GitHubBodyLimit/20)
}

func TestGitHubPushDesignNotesOverflowsToComment(t *testing.T) {
	var calls [][]string
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "issue" && args[1] == "view":
			return `{"labels":[]}`, nil // no existing design comment yet
		case args[0] == "issue" && args[1] == "comment":
			return "https://github.com/o/r/issues/7#issuecomment-1", nil
		case args[0] == "issue" && args[1] == "edit":
			return "", nil
		default:
			return "", nil
		}
	}}

	notes := bigDesignNotes()
	_, err := p.Push(context.Background(), WorkItem{
		Slug: "my-change", Title: "T", Body: "B\n\n## Design notes\n\n" + notes,
		DesignNotes: notes, Stage: "planned",
	}, &Ref{Provider: "github", ID: "7"})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}

	comment := findCall(calls, "issue", "comment", "7")
	if comment == nil {
		t.Fatal("expected a design-notes comment to be posted")
	}
	commentBody := flagValue(comment, "--body")
	if !strings.Contains(commentBody, designCommentMarker("my-change")) {
		t.Errorf("comment missing its identity marker: %q", commentBody)
	}
	if !strings.Contains(commentBody, notes) {
		t.Errorf("comment missing design notes content")
	}

	edit := findCall(calls, "issue", "edit", "7")
	if edit == nil {
		t.Fatal("expected an issue edit")
	}
	editBody := flagValue(edit, "--body")
	if strings.Contains(editBody, notes) {
		t.Errorf("issue body should not inline the oversized design notes:\n%s", editBody)
	}
	if !strings.Contains(editBody, "issuecomment-1") {
		t.Errorf("issue body should link to the design notes comment, got:\n%s", editBody)
	}
}

func TestGitHubPushDesignNotesResyncUpdatesSameComment(t *testing.T) {
	var calls [][]string
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "issue" && args[1] == "view":
			return `{"labels":[],"comments":[{"id":"COMMENT_NODE_ID","url":"https://github.com/o/r/issues/7#issuecomment-1","body":"` +
				designCommentMarker("my-change") + `\n\nold content"}]}`, nil
		case args[0] == "api" && args[1] == "graphql":
			return `{"data":{"updateIssueComment":{"clientMutationId":null}}}`, nil
		case args[0] == "issue" && args[1] == "comment":
			t.Fatalf("should update the existing comment, not create a new one; args=%v", args)
			return "", nil
		default:
			return "", nil
		}
	}}

	notes := bigDesignNotes()
	_, err := p.Push(context.Background(), WorkItem{
		Slug: "my-change", Title: "T", Body: "B\n\n## Design notes\n\n" + notes,
		DesignNotes: notes, Stage: "planned",
	}, &Ref{Provider: "github", ID: "7"})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}

	if findCall(calls, "api", "graphql") == nil {
		t.Fatal("expected a graphql mutation to update the existing comment")
	}
}

func TestGitHubPushDesignNotesShrinkBackMarksStale(t *testing.T) {
	var calls [][]string
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "issue" && args[1] == "view":
			return `{"labels":[],"comments":[{"id":"COMMENT_NODE_ID","url":"https://github.com/o/r/issues/7#issuecomment-1","body":"` +
				designCommentMarker("my-change") + `\n\nold oversized content"}]}`, nil
		case args[0] == "api" && args[1] == "graphql":
			return `{"data":{}}`, nil
		default:
			return "", nil
		}
	}}

	small := "Now a short design note."
	_, err := p.Push(context.Background(), WorkItem{
		Slug: "my-change", Title: "T", Body: "B\n\n## Design notes\n\n" + small,
		DesignNotes: small, Stage: "planned",
	}, &Ref{Provider: "github", ID: "7"})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}

	edit := findCall(calls, "issue", "edit", "7")
	if edit == nil {
		t.Fatal("expected an issue edit")
	}
	if !strings.Contains(flagValue(edit, "--body"), small) {
		t.Errorf("issue body should inline the now-small design notes, got:\n%s", flagValue(edit, "--body"))
	}

	if findCall(calls, "api", "graphql") == nil {
		t.Fatal("expected a graphql mutation marking the old comment stale")
	}
	comment := findCall(calls, "issue", "comment")
	if comment != nil {
		t.Errorf("stale marking must not delete or recreate the comment via `issue comment`, got: %v", comment)
	}
}

func TestGitHubPushDesignNotesStaleIsIdempotent(t *testing.T) {
	var graphqlCalls int
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		switch {
		case args[0] == "issue" && args[1] == "view":
			return `{"labels":[],"comments":[{"id":"COMMENT_NODE_ID","url":"u","body":"` +
				designCommentMarker("my-change") + `\n\n_Design notes moved back into the issue body; this comment is no longer current._"}]}`, nil
		case args[0] == "api" && args[1] == "graphql":
			graphqlCalls++
			return `{"data":{}}`, nil
		default:
			return "", nil
		}
	}}

	small := "Still a short design note."
	_, err := p.Push(context.Background(), WorkItem{
		Slug: "my-change", Title: "T", Body: "B\n\n## Design notes\n\n" + small,
		DesignNotes: small, Stage: "planned",
	}, &Ref{Provider: "github", ID: "7"})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if graphqlCalls != 0 {
		t.Errorf("expected no further write to an already-stale comment, got %d graphql calls", graphqlCalls)
	}
}

func TestGitHubPushDesignNotesCreateNewIssueOverflow(t *testing.T) {
	var calls [][]string
	p := &GitHubProvider{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "issue" && args[1] == "list":
			return "[]", nil
		case args[0] == "issue" && args[1] == "create":
			return "https://github.com/o/r/issues/9", nil
		case args[0] == "issue" && args[1] == "view":
			return `{"labels":[]}`, nil
		case args[0] == "issue" && args[1] == "comment":
			return "https://github.com/o/r/issues/9#issuecomment-1", nil
		default:
			return "", nil
		}
	}}

	notes := bigDesignNotes()
	ref, err := p.Push(context.Background(), WorkItem{
		Slug: "my-change", Title: "T", Body: "B\n\n## Design notes\n\n" + notes,
		DesignNotes: notes, Stage: "planned",
	}, nil)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if ref.ID != "9" {
		t.Fatalf("ref.ID = %q, want 9", ref.ID)
	}

	if findCall(calls, "issue", "comment", "9") == nil {
		t.Fatal("expected a design-notes comment posted against the newly created issue")
	}
	edit := findCall(calls, "issue", "edit", "9")
	if edit == nil {
		t.Fatal("expected a follow-up edit with the real comment link")
	}
	if strings.Contains(flagValue(edit, "--body"), notes) {
		t.Errorf("follow-up edit should not inline the oversized design notes")
	}
	if !strings.Contains(flagValue(edit, "--body"), "issuecomment-1") {
		t.Errorf("follow-up edit should carry the real comment link")
	}
}

func TestPullRecoversDesignNotesFromOverflowComment(t *testing.T) {
	dir := t.TempDir()
	realNotes := "The real, lost-to-a-force-removed-worktree design decision.\n"
	issue := fakeIssue{
		Number: 12,
		URL:    "https://github.com/o/r/issues/12",
		Title:  "Recover design notes",
		State:  "open",
		Body: "# Recover design notes\n\nbody\n\n## Design notes\n\n" +
			"_Too large to inline — see [design notes comment](https://github.com/o/r/issues/12#issuecomment-1)._\n",
	}
	var calls [][]string
	prov := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case len(args) >= 2 && args[0] == "issue" && args[1] == "view":
			if contains(args, "--json") && jsonFields(args) == "comments" {
				b, _ := json.Marshal(map[string]any{
					"comments": []map[string]string{{
						"id":   "COMMENT_NODE_ID",
						"url":  "https://github.com/o/r/issues/12#issuecomment-1",
						"body": designCommentMarker("recover-design-notes") + "\n\n" + realNotes,
					}},
				})
				return string(b), nil
			}
			b, _ := json.Marshal(issue)
			return string(b), nil
		default:
			return "", nil
		}
	})

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "12"})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if isDesignNotesStub(res.DesignNotes) {
		t.Fatalf("pull should have recovered the real content, not the stub: %q", res.DesignNotes)
	}
	if !strings.Contains(res.DesignNotes, "lost-to-a-force-removed-worktree") {
		t.Fatalf("recovered design notes missing real content: %q", res.DesignNotes)
	}

	design, err := os.ReadFile(filepath.Join(dir, "changes", res.Slug, "design.md"))
	if err != nil {
		t.Fatalf("read design.md: %v", err)
	}
	if !strings.Contains(string(design), "lost-to-a-force-removed-worktree") {
		t.Fatalf("design.md missing recovered content: %s", design)
	}
	if strings.Contains(string(design), designNotesStubMarker) {
		t.Fatalf("design.md should not contain the stub text: %s", design)
	}
}

func TestEndToEndDesignNotesRegression(t *testing.T) {
	t.Run("small design.md syncs inline", func(t *testing.T) {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "changes", "small", "proposal.md"), "# Small\n\nbody\n")
		mustWrite(t, filepath.Join(dir, "changes", "small", "design.md"), "A short decision.\n")

		var calls [][]string
		prov := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
			calls = append(calls, args)
			switch {
			case args[0] == "issue" && args[1] == "list":
				return "[]", nil
			case args[0] == "issue" && args[1] == "create":
				return "https://github.com/o/r/issues/1", nil
			default:
				return "", nil
			}
		})
		if _, err := Sync(context.Background(), Options{OpenSpecDir: dir, Provider: prov, Slug: "small"}); err != nil {
			t.Fatalf("Sync: %v", err)
		}
		create := findCall(calls, "issue", "create")
		if create == nil || !strings.Contains(flagValue(create, "--body"), "A short decision") {
			t.Fatalf("expected design notes inlined in the created issue, calls: %v", calls)
		}
		if findCall(calls, "issue", "comment") != nil {
			t.Fatalf("a small design.md must not overflow to a comment, calls: %v", calls)
		}
	})

	t.Run("oversized design.md syncs via comment, then pull recovers it", func(t *testing.T) {
		dir := t.TempDir()
		notes := bigDesignNotes()
		mustWrite(t, filepath.Join(dir, "changes", "big", "proposal.md"), "# Big\n\nbody\n")
		mustWrite(t, filepath.Join(dir, "changes", "big", "design.md"), notes)

		const issueURL = "https://github.com/o/r/issues/2"
		var issueBody, commentBody string
		prov := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
			switch {
			case args[0] == "issue" && args[1] == "list":
				return "[]", nil
			case args[0] == "issue" && args[1] == "create":
				issueBody = flagValue(args, "--body")
				return issueURL, nil
			case args[0] == "issue" && args[1] == "edit":
				if b := flagValue(args, "--body"); b != "" {
					issueBody = b
				}
				return "", nil
			case args[0] == "issue" && args[1] == "comment":
				commentBody = flagValue(args, "--body")
				return issueURL + "#issuecomment-1", nil
			case args[0] == "issue" && args[1] == "view":
				if contains(args, "--json") && jsonFields(args) == "comments" {
					if commentBody == "" {
						return `{"comments":[]}`, nil
					}
					b, _ := json.Marshal(map[string]any{
						"comments": []map[string]string{{
							"id": "COMMENT_NODE_ID", "url": issueURL + "#issuecomment-1", "body": commentBody,
						}},
					})
					return string(b), nil
				}
				if jsonFields(args) == "labels,state" {
					return `{"labels":[],"state":"OPEN"}`, nil
				}
				// The full fetch pull's Get() makes.
				b, _ := json.Marshal(fakeIssue{Number: 2, URL: issueURL, Title: "Big", State: "open", Body: issueBody})
				return string(b), nil
			default:
				return "", nil
			}
		})
		if _, err := Sync(context.Background(), Options{OpenSpecDir: dir, Provider: prov, Slug: "big"}); err != nil {
			t.Fatalf("Sync: %v", err)
		}
		if strings.Contains(issueBody, notes) {
			t.Fatalf("oversized design.md must not be inlined in the issue body")
		}
		if !strings.Contains(commentBody, notes) {
			t.Fatalf("oversized design.md must be posted as a comment")
		}

		// Simulate the worktree being lost, then pull it back.
		if err := os.Remove(filepath.Join(dir, "changes", "big", "design.md")); err != nil {
			t.Fatalf("remove design.md: %v", err)
		}
		res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "2"})
		if err != nil {
			t.Fatalf("Pull: %v", err)
		}
		if !strings.Contains(res.DesignNotes, notes[:200]) {
			t.Fatalf("pull did not recover the oversized design notes from the comment")
		}
		recovered, err := os.ReadFile(filepath.Join(dir, "changes", "big", "design.md"))
		if err != nil {
			t.Fatalf("read recovered design.md: %v", err)
		}
		if !strings.Contains(string(recovered), notes[:200]) {
			t.Fatalf("recovered design.md missing content")
		}
	})
}
