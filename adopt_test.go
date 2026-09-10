package specsync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdoptWritesRefAndMarker covers the happy path: adopt writes
// .specsync/refs.json and adds the identity marker to the issue body.
func TestAdoptWritesRefAndMarker(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "my-change", "proposal.md"), "# My Change\n")

	var editArgs []string
	body := "the original body"
	p := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		switch {
		case args[0] == "issue" && args[1] == "view":
			return `{"number":42,"url":"https://github.com/o/r/issues/42","title":"T","body":"` + body + `","state":"OPEN"}`, nil
		case args[0] == "issue" && args[1] == "edit":
			editArgs = args
			return "", nil
		default:
			return "", nil
		}
	})

	res, err := Adopt(context.Background(), AdoptOptions{
		OpenSpecDir: root,
		Provider:    p,
		IssueID:     "42",
		Slug:        "my-change",
	})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if res.Slug != "my-change" || res.IssueID != "42" || res.IssueURL != "https://github.com/o/r/issues/42" {
		t.Fatalf("unexpected result: %#v", res)
	}
	if !res.MarkerAdded {
		t.Error("expected MarkerAdded true")
	}
	if editArgs == nil {
		t.Fatal("expected an issue edit call adding the marker")
	}
	if got := flagValue(editArgs, "--body"); !strings.Contains(got, "specsync:change=my-change") || !strings.Contains(got, body) {
		t.Errorf("edited body = %q, want marker + preserved content", got)
	}

	refs, err := LoadRefs(filepath.Join(root, "changes", "my-change"))
	if err != nil {
		t.Fatalf("LoadRefs: %v", err)
	}
	ref, ok := refs["github:o/r"]
	if !ok || ref.ID != "42" {
		t.Fatalf("refs = %#v, want github:o/r -> issue 42", refs)
	}
}

// TestAdoptDryRunWritesNothing: a dry run must not touch the ref cache or
// the issue body.
func TestAdoptDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "my-change", "proposal.md"), "# My Change\n")

	var wroteIssue bool
	p := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		switch {
		case args[0] == "issue" && args[1] == "view":
			return `{"number":42,"url":"https://github.com/o/r/issues/42","title":"T","body":"orig","state":"OPEN"}`, nil
		case args[0] == "issue" && args[1] == "edit":
			wroteIssue = true
			return "", nil
		default:
			return "", nil
		}
	})

	res, err := Adopt(context.Background(), AdoptOptions{
		OpenSpecDir: root, Provider: p, IssueID: "42", Slug: "my-change", DryRun: true,
	})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if !res.MarkerAdded {
		t.Error("expected MarkerAdded true (preview) on dry run")
	}
	if wroteIssue {
		t.Error("dry run must not edit the issue")
	}
	cdir := filepath.Join(root, "changes", "my-change")
	if _, err := os.Stat(refCachePath(cdir)); !os.IsNotExist(err) {
		t.Error("dry run must not write .specsync/refs.json")
	}
}

// TestAdoptRejectsConflictingChangeBinding: the change already points at a
// different issue; -force is required to rebind.
func TestAdoptRejectsConflictingChangeBinding(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "my-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# My Change\n")
	if err := saveRef(cdir, "github:o/r", Ref{Provider: "github:o/r", ID: "10", URL: "https://github.com/o/r/issues/10"}); err != nil {
		t.Fatalf("saveRef: %v", err)
	}

	p := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "view" {
			return `{"number":42,"url":"https://github.com/o/r/issues/42","title":"T","body":"orig","state":"OPEN"}`, nil
		}
		return "", nil
	})

	if _, err := Adopt(context.Background(), AdoptOptions{
		OpenSpecDir: root, Provider: p, IssueID: "42", Slug: "my-change",
	}); err == nil {
		t.Fatal("expected refusal to rebind an already-bound change without -force")
	}

	res, err := Adopt(context.Background(), AdoptOptions{
		OpenSpecDir: root, Provider: p, IssueID: "42", Slug: "my-change", Force: true,
	})
	if err != nil {
		t.Fatalf("Adopt with Force: %v", err)
	}
	if res.IssueID != "42" {
		t.Errorf("IssueID = %q, want 42 after forced rebind", res.IssueID)
	}
}

// TestAdoptRejectsConflictingIssueMarker: the issue already carries a marker
// for a different change; -force is required to steal it.
func TestAdoptRejectsConflictingIssueMarker(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "my-change", "proposal.md"), "# My Change\n")

	p := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		if args[0] == "issue" && args[1] == "view" {
			return `{"number":42,"url":"https://github.com/o/r/issues/42","title":"T","body":"<!-- specsync:change=other-change -->\n\nbody","state":"OPEN"}`, nil
		}
		return "", nil
	})

	_, err := Adopt(context.Background(), AdoptOptions{
		OpenSpecDir: root, Provider: p, IssueID: "42", Slug: "my-change",
	})
	if err == nil || !strings.Contains(err.Error(), "other-change") {
		t.Fatalf("err = %v, want refusal naming the conflicting change", err)
	}
}

// TestAdoptRegressionRace reproduces the observed FusionHub failure: a
// change is adopted onto an issue whose marker has not yet reached the
// tracker's search index. A later sync — which would previously find
// nothing via marker search and create a duplicate — must instead find the
// ref cache adopt wrote and update the same issue, never creating a second
// one, regardless of what marker search reports.
func TestAdoptRegressionRace(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "my-change")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# My Change\n")

	issueBody := "hand-created body"
	var createCalled bool
	viewResponse := func() string {
		b, _ := json.Marshal(map[string]any{
			"number": 42, "url": "https://github.com/o/r/issues/42",
			"title": "T", "body": issueBody, "state": "OPEN", "labels": []string{},
		})
		return string(b)
	}
	run := func(_ context.Context, args ...string) (string, error) {
		switch {
		case args[0] == "issue" && args[1] == "view":
			return viewResponse(), nil
		case args[0] == "issue" && args[1] == "edit":
			// Simulate the marker landing in the body for future Get calls,
			// but the search index (below) stays stale for this run.
			issueBody = "<!-- specsync:change=my-change -->\n\n" + issueBody
			return "", nil
		case args[0] == "issue" && args[1] == "list":
			// The race: search hasn't indexed the marker yet.
			return "[]", nil
		case args[0] == "issue" && args[1] == "create":
			createCalled = true
			return "https://github.com/o/r/issues/999", nil
		default:
			return "", nil
		}
	}
	p := NewGitHubProviderFuncWithRepo("o/r", run)

	if _, err := Adopt(context.Background(), AdoptOptions{
		OpenSpecDir: root, Provider: p, IssueID: "42", Slug: "my-change",
	}); err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	// A sync run immediately after, while marker search is still lagging.
	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: p})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if createCalled {
		t.Fatal("sync created a duplicate issue instead of using the ref adopt wrote")
	}
	if len(res.Items) != 1 || res.Items[0].URL != "https://github.com/o/r/issues/42" {
		t.Fatalf("Sync result = %#v, want update of issue 42", res.Items)
	}
	if res.Created != 0 || res.Updated != 1 {
		t.Fatalf("Created=%d Updated=%d, want 0/1 (update, not create)", res.Created, res.Updated)
	}
}
