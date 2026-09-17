package specsync

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestPull_SameIssueRePullOverwritesInPlace(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{Number: 7, URL: "https://github.com/o/r/issues/7", Title: "Round trip", State: "open", Body: "# Round trip\n\nfirst body\n"}
	var calls [][]string
	prov := NewGitHubProviderFuncWithRepo("o/r", ghRunner(issue, &calls))

	first, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "7"})
	if err != nil {
		t.Fatalf("first Pull: %v", err)
	}

	issue.Body = "# Round trip\n\nupdated body\n"
	second, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "7"})
	if err != nil {
		t.Fatalf("second Pull (re-pull of the same issue): %v", err)
	}
	if second.Slug != first.Slug {
		t.Fatalf("re-pulling the same issue must reuse the same slug, got %q then %q", first.Slug, second.Slug)
	}
	if second.Dir != first.Dir {
		t.Fatalf("re-pulling the same issue must reuse the same directory, got %q then %q", first.Dir, second.Dir)
	}
}

func TestPull_DifferentIssueSameGeneratedSlugGetsSuffixed(t *testing.T) {
	dir := t.TempDir()
	issueA := fakeIssue{Number: 1, URL: "https://github.com/o/r/issues/1", Title: "Add thing", State: "open", Body: "# Add thing\n\nfirst\n"}
	var callsA [][]string
	provA := NewGitHubProviderFuncWithRepo("o/r", ghRunner(issueA, &callsA))
	first, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: provA, IssueID: "1"})
	if err != nil {
		t.Fatalf("first Pull: %v", err)
	}
	if first.Slug != "add-thing" {
		t.Fatalf("expected slug 'add-thing', got %q", first.Slug)
	}

	// A different issue whose title happens to slugify identically.
	issueB := fakeIssue{Number: 2, URL: "https://github.com/o/r/issues/2", Title: "Add thing", State: "open", Body: "# Add thing\n\nunrelated\n"}
	var callsB [][]string
	provB := NewGitHubProviderFuncWithRepo("o/r", ghRunner(issueB, &callsB))
	second, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: provB, IssueID: "2"})
	if err != nil {
		t.Fatalf("second Pull (colliding slug, different issue): %v", err)
	}
	if second.Slug == first.Slug {
		t.Fatalf("expected a suffixed slug distinct from %q, got the same", first.Slug)
	}
	if second.Slug != "add-thing-2" {
		t.Errorf("expected 'add-thing-2', got %q", second.Slug)
	}

	// Neither change's own content was clobbered.
	firstProposal := readFile(t, filepath.Join(dir, "changes", "add-thing", "proposal.md"))
	if !strings.Contains(firstProposal, "first") {
		t.Errorf("first change's proposal.md was overwritten: %s", firstProposal)
	}
}

func TestPull_ExplicitChangeCollisionWithDifferentIssueErrors(t *testing.T) {
	dir := t.TempDir()
	issueA := fakeIssue{Number: 1, URL: "https://github.com/o/r/issues/1", Title: "Add thing", State: "open", Body: "# Add thing\n\nbody\n"}
	var callsA [][]string
	provA := NewGitHubProviderFuncWithRepo("o/r", ghRunner(issueA, &callsA))
	if _, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: provA, IssueID: "1", Slug: "my-change"}); err != nil {
		t.Fatalf("first Pull: %v", err)
	}

	issueB := fakeIssue{Number: 2, URL: "https://github.com/o/r/issues/2", Title: "Something else", State: "open", Body: "# Something else\n\nbody\n"}
	var callsB [][]string
	provB := NewGitHubProviderFuncWithRepo("o/r", ghRunner(issueB, &callsB))
	_, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: provB, IssueID: "2", Slug: "my-change"})
	if err == nil {
		t.Fatal("expected an error pulling a different issue into an explicit, already-taken -change slug")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPull_ExplicitChangeCollisionWithSameIssueOverwrites(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{Number: 1, URL: "https://github.com/o/r/issues/1", Title: "Add thing", State: "open", Body: "# Add thing\n\nfirst\n"}
	var calls [][]string
	prov := NewGitHubProviderFuncWithRepo("o/r", ghRunner(issue, &calls))
	if _, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "1", Slug: "my-change"}); err != nil {
		t.Fatalf("first Pull: %v", err)
	}

	issue.Body = "# Add thing\n\nupdated\n"
	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "1", Slug: "my-change"})
	if err != nil {
		t.Fatalf("re-pull with explicit -change matching the same issue must succeed: %v", err)
	}
	if res.Slug != "my-change" {
		t.Errorf("expected slug unchanged at 'my-change', got %q", res.Slug)
	}
}

func TestPull_UnrelatedHandAuthoredDirectorySameSlugGetsSuffixed(t *testing.T) {
	dir := t.TempDir()
	// A hand-authored change with no ref cache at all, sharing the slug an
	// incoming issue would generate.
	mustWrite(t, filepath.Join(dir, "changes", "add-thing", "proposal.md"), "# Add thing (hand-authored)\n")

	issue := fakeIssue{Number: 9, URL: "https://github.com/o/r/issues/9", Title: "Add thing", State: "open", Body: "# Add thing\n\nfrom issue\n"}
	var calls [][]string
	prov := NewGitHubProviderFuncWithRepo("o/r", ghRunner(issue, &calls))
	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "9"})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if res.Slug != "add-thing-2" {
		t.Errorf("expected the incoming pull to get suffixed rather than overwrite the hand-authored change, got %q", res.Slug)
	}

	handAuthored := readFile(t, filepath.Join(dir, "changes", "add-thing", "proposal.md"))
	if !strings.Contains(handAuthored, "hand-authored") {
		t.Errorf("hand-authored change was overwritten: %s", handAuthored)
	}
}

func TestPull_DryRunPreviewsTheSuffixedSlugWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	issueA := fakeIssue{Number: 1, URL: "https://github.com/o/r/issues/1", Title: "Add thing", State: "open", Body: "# Add thing\n\nfirst\n"}
	var callsA [][]string
	provA := NewGitHubProviderFuncWithRepo("o/r", ghRunner(issueA, &callsA))
	if _, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: provA, IssueID: "1"}); err != nil {
		t.Fatalf("first Pull: %v", err)
	}

	issueB := fakeIssue{Number: 2, URL: "https://github.com/o/r/issues/2", Title: "Add thing", State: "open", Body: "# Add thing\n\nsecond\n"}
	var callsB [][]string
	provB := NewGitHubProviderFuncWithRepo("o/r", ghRunner(issueB, &callsB))
	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: provB, IssueID: "2", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run Pull: %v", err)
	}
	if res.Slug != "add-thing-2" {
		t.Errorf("expected dry-run to preview the suffixed slug, got %q", res.Slug)
	}
	if dirExists(filepath.Join(dir, "changes", "add-thing-2")) {
		t.Error("dry-run must not create the suffixed directory")
	}
}

func TestNextAvailableSlug_SkipsMultipleTakenSuffixes(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "changes", "x", "proposal.md"), "# X\n")
	mustWrite(t, filepath.Join(dir, "changes", "x-2", "proposal.md"), "# X2\n")
	mustWrite(t, filepath.Join(dir, "changes", "x-3", "proposal.md"), "# X3\n")

	got := nextAvailableSlug(dir, "x")
	if got != "x-4" {
		t.Errorf("nextAvailableSlug = %q, want x-4", got)
	}
}
