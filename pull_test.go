package specsync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeIssue is the canned data a faked `gh issue view` returns.
type fakeIssue struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	Labels []any  `json:"labels"`
}

// ghRunner records calls and answers `issue view` with the given issue plus the
// minimal responses Push needs, so a pull and a follow-up push can be exercised
// against one fake.
func ghRunner(issue fakeIssue, calls *[][]string) func(context.Context, ...string) (string, error) {
	return func(_ context.Context, args ...string) (string, error) {
		*calls = append(*calls, args)
		switch {
		case len(args) >= 2 && args[0] == "issue" && args[1] == "view":
			// labels query during a label reconcile asks only for labels.
			if contains(args, "--json") && jsonFields(args) == "labels" {
				return `{"labels":[]}`, nil
			}
			b, _ := json.Marshal(issue)
			return string(b), nil
		case len(args) >= 2 && args[0] == "issue" && args[1] == "list":
			return "[]", nil
		case len(args) >= 2 && args[0] == "issue" && args[1] == "create":
			return issue.URL, nil
		default:
			return "", nil
		}
	}
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func jsonFields(args []string) string {
	for i, a := range args {
		if a == "--json" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestPullCreatesChangeWithTasks(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{
		Number: 7,
		URL:    "https://github.com/o/r/issues/7",
		Title:  "Add CSV export",
		State:  "open",
		Body: "<!-- specsync:change=add-csv-export -->\n\n# Add CSV export\n\n" +
			"## Why\nUsers want their data.\n\n## Tasks\n\n- [ ] 1.1 build it\n- [ ] 1.2 test it\n",
	}
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{
		OpenSpecDir: dir,
		Provider:    prov,
		IssueID:     "7",
	})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if res.Slug != "add-csv-export" {
		t.Fatalf("slug from marker = %q, want add-csv-export", res.Slug)
	}

	proposal := readFile(t, filepath.Join(dir, "changes", "add-csv-export", "proposal.md"))
	if !strings.HasPrefix(proposal, "# Add CSV export") {
		t.Fatalf("proposal should open with H1, got:\n%s", proposal)
	}
	if strings.Contains(proposal, "specsync:change=") {
		t.Fatalf("proposal should not retain the identity marker:\n%s", proposal)
	}
	if strings.Contains(proposal, "## Tasks") {
		t.Fatalf("proposal should not contain the Tasks section:\n%s", proposal)
	}
	tasks := readFile(t, filepath.Join(dir, "changes", "add-csv-export", "tasks.md"))
	if !strings.Contains(tasks, "1.1 build it") || !strings.Contains(tasks, "1.2 test it") {
		t.Fatalf("tasks.md missing checklist:\n%s", tasks)
	}
}

func TestPullWithoutTasksWritesProposalOnly(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{
		Number: 4083,
		URL:    "https://github.com/o/r/issues/4083",
		Title:  "Streamlined modals for integration onboarding",
		State:  "open",
		Body:   "Figma design\nhttps://example.com/figma\n",
	}
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "4083"})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if res.Slug != "streamlined-modals-for-integration-onboarding" {
		t.Fatalf("slug = %q", res.Slug)
	}
	if _, err := os.Stat(filepath.Join(dir, "changes", res.Slug, "tasks.md")); !os.IsNotExist(err) {
		t.Fatalf("tasks.md should not exist for a body without a Tasks section")
	}
	proposal := readFile(t, filepath.Join(dir, "changes", res.Slug, "proposal.md"))
	if !strings.HasPrefix(proposal, "# Streamlined modals for integration onboarding") {
		t.Fatalf("proposal should be prefixed with an H1 title:\n%s", proposal)
	}
	if !strings.Contains(proposal, "Figma design") {
		t.Fatalf("proposal should retain the issue body:\n%s", proposal)
	}
}

func TestPullLinksIssueForRoundTrip(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{
		Number: 7,
		URL:    "https://github.com/o/r/issues/7",
		Title:  "Round trip",
		State:  "open",
		Body:   "# Round trip\n\nbody\n",
	}
	var calls [][]string
	prov := NewGitHubProviderFuncWithRepo("test/repo", ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "7"})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// The ref cache must now bind the change to issue 7 so a push updates it.
	refs, err := LoadRefs(res.Dir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	if refs["github:test/repo"].ID != "7" {
		t.Fatalf("cached ref id = %q, want 7", refs["github:test/repo"].ID)
	}

	// A follow-up sync of that change must edit issue 7, never create.
	calls = nil
	if _, err := Sync(context.Background(), Options{OpenSpecDir: dir, Provider: prov, Slug: res.Slug}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	var edited, created bool
	for _, c := range calls {
		if len(c) >= 3 && c[0] == "issue" && c[1] == "edit" && c[2] == "7" {
			edited = true
		}
		if len(c) >= 2 && c[0] == "issue" && c[1] == "create" {
			created = true
		}
	}
	if !edited {
		t.Fatalf("expected an `issue edit 7`, calls: %v", calls)
	}
	if created {
		t.Fatalf("round-trip must not create a duplicate issue, calls: %v", calls)
	}
}

func TestPullDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{Number: 7, URL: "u", Title: "T", State: "open", Body: "# T\n\nbody\n## Tasks\n\n- [ ] 1.1 x\n"}
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "7", DryRun: true})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if res.Proposal == "" {
		t.Fatalf("dry run should still render a proposal preview")
	}
	if _, err := os.Stat(filepath.Join(dir, "changes")); !os.IsNotExist(err) {
		t.Fatalf("dry run must not create any change folder")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestWorkItemForAddedCount(t *testing.T) {
	baseline := 2
	c := Change{
		Slug:          "test",
		Body:          "# Test\n\nBody.\n",
		TasksMarkdown: "- [x] one\n- [ ] two\n- [ ] three\n",
		BaselineTasks: &baseline,
	}
	wi := WorkItemFor(c, false)
	if !strings.Contains(wi.Body, "+1 added") {
		t.Errorf("expected '+1 added' in body, got:\n%s", wi.Body)
	}
}

func TestWorkItemForNoAddedAtBaseline(t *testing.T) {
	added := 3
	c := Change{
		Slug:          "test",
		Body:          "# Test\n\nBody.\n",
		TasksMarkdown: "- [x] one\n- [ ] two\n- [ ] three\n",
		BaselineTasks: &added,
	}
	wi := WorkItemFor(c, false)
	if strings.Contains(wi.Body, "added") {
		t.Errorf("expected no 'added' in body, got:\n%s", wi.Body)
	}
}

func TestWorkItemForNoAddedNoBaseline(t *testing.T) {
	c := Change{
		Slug:          "test",
		Body:          "# Test\n\nBody.\n",
		TasksMarkdown: "- [x] one\n- [ ] two\n- [ ] three\n",
	}
	wi := WorkItemFor(c, false)
	if strings.Contains(wi.Body, "added") {
		t.Errorf("expected no 'added' in body, got:\n%s", wi.Body)
	}
}

func TestOriginalAskRoundTrip(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{
		Number: 42,
		URL:    "https://github.com/o/r/issues/42",
		Title:  "Round trip",
		State:  "open",
		Body:   "# Round trip\n\nOriginal body text.\n\n## Tasks\n\n- [ ] one\n",
	}
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	// First pull seeds original-ask.md.
	res, err := Pull(context.Background(), PullOptions{
		OpenSpecDir: dir,
		Provider:    prov,
		IssueID:     "42",
	})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// Sync renders it back into the issue.
	calls = nil
	if _, err := Sync(context.Background(), Options{OpenSpecDir: dir, Provider: prov, Slug: res.Slug}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Re-pull should strip the managed sections and leave original-ask.md unchanged.
	calls = nil
	_, err = Pull(context.Background(), PullOptions{
		OpenSpecDir: dir,
		Provider:    prov,
		IssueID:     "42",
	})
	if err != nil {
		t.Fatalf("re-Pull: %v", err)
	}

	// Verify original-ask.md still exists and is unchanged.
	askPath := filepath.Join(dir, "changes", res.Slug, "original-ask.md")
	ask, err := os.ReadFile(askPath)
	if err != nil {
		t.Fatalf("read original-ask.md: %v", err)
	}
	if !strings.Contains(string(ask), "Original body text") {
		t.Fatalf("original-ask.md lost content: %s", ask)
	}
}

func TestWorkItemForRendersOriginalAsk(t *testing.T) {
	c := Change{
		Slug:        "test",
		Body:        "# Test\n\nBody.\n",
		OriginalAsk: "This was the original request.",
	}
	wi := WorkItemFor(c, false)
	if !strings.Contains(wi.Body, "## Original ask") {
		t.Errorf("expected '## Original ask' in body, got:\n%s", wi.Body)
	}
	if !strings.Contains(wi.Body, "This was the original request") {
		t.Errorf("expected original ask content in body, got:\n%s", wi.Body)
	}
}

func TestWorkItemForRendersDiscoveries(t *testing.T) {
	c := Change{
		Slug:        "test",
		Body:        "# Test\n\nBody.\n",
		Discoveries: "Found a bug in the auth flow.",
	}
	wi := WorkItemFor(c, false)
	if !strings.Contains(wi.Body, "## Discoveries") {
		t.Errorf("expected '## Discoveries' in body, got:\n%s", wi.Body)
	}
	if !strings.Contains(wi.Body, "Found a bug in the auth flow") {
		t.Errorf("expected discoveries content in body, got:\n%s", wi.Body)
	}
}

func TestWorkItemForSectionOrder(t *testing.T) {
	c := Change{
		Slug:          "test",
		Body:          "# Test\n\nBody.\n",
		OriginalAsk:   "Original ask.",
		DesignNotes:   "Design decision.",
		Discoveries:   "Discovery.",
		TasksMarkdown: "- [ ] one\n",
		Links:         []Ref{{ID: "1", URL: "https://example.com/o/r/1"}},
	}
	wi := WorkItemFor(c, false)

	oa := strings.Index(wi.Body, "## Original ask")
	dn := strings.Index(wi.Body, "## Design notes")
	di := strings.Index(wi.Body, "## Discoveries")
	ti := strings.Index(wi.Body, "## Tasks")
	ri := strings.Index(wi.Body, "## Related")

	if oa >= dn || dn >= di || di >= ti || ti >= ri {
		t.Errorf("sections not in order (Original ask < Design notes < Discoveries < Tasks < Related):\n%s", wi.Body)
	}
}

func TestWorkItemForRendersDesignNotes(t *testing.T) {
	c := Change{
		Slug:        "test",
		Body:        "# Test\n\nBody.\n",
		DesignNotes: "Picked the picker pattern over a modal.",
	}
	wi := WorkItemFor(c, false)
	if !strings.Contains(wi.Body, "## Design notes") {
		t.Errorf("expected '## Design notes' in body, got:\n%s", wi.Body)
	}
	if !strings.Contains(wi.Body, "Picked the picker pattern over a modal") {
		t.Errorf("expected design notes content in body, got:\n%s", wi.Body)
	}
	if wi.DesignNotes != c.DesignNotes {
		t.Errorf("WorkItem.DesignNotes = %q, want %q", wi.DesignNotes, c.DesignNotes)
	}
}

func TestWorkItemForOmitsDesignNotesWhenEmpty(t *testing.T) {
	c := Change{Slug: "test", Body: "# Test\n\nBody.\n"}
	wi := WorkItemFor(c, false)
	if strings.Contains(wi.Body, "## Design notes") {
		t.Errorf("expected no '## Design notes' section, got:\n%s", wi.Body)
	}
}

func TestPullSavesOriginalAsk(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{
		Number: 99,
		URL:    "https://github.com/o/r/issues/99",
		Title:  "Feature",
		State:  "open",
		Body:   "# Feature\n\nThe original request.\n\n## Tasks\n\n- [ ] one\n",
	}
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{
		OpenSpecDir: dir,
		Provider:    prov,
		IssueID:     "99",
	})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	askPath := filepath.Join(dir, "changes", res.Slug, "original-ask.md")
	ask, err := os.ReadFile(askPath)
	if err != nil {
		t.Fatalf("read original-ask.md: %v", err)
	}
	if !strings.Contains(string(ask), "The original request") {
		t.Fatalf("original-ask.md missing content: %s", ask)
	}
}

// TestDesignNotesWriteOnceOnPull: design.md is write-once like
// original-ask.md, not overwritten-every-pull like discoveries.md.
func TestDesignNotesWriteOnceOnPull(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{
		Number: 42,
		URL:    "https://github.com/o/r/issues/42",
		Title:  "Design write-once",
		State:  "open",
		Body:   "# Design write-once\n\nbody\n\n## Design notes\n\nSynced decision.\n",
	}
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "42"})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	designPath := filepath.Join(dir, "changes", res.Slug, "design.md")
	design, err := os.ReadFile(designPath)
	if err != nil {
		t.Fatalf("read design.md: %v", err)
	}
	if !strings.Contains(string(design), "Synced decision") {
		t.Fatalf("design.md missing synced content: %s", design)
	}

	richer := "Synced decision, plus a locally-written follow-up not yet pushed.\n"
	if err := os.WriteFile(designPath, []byte(richer), 0o644); err != nil {
		t.Fatalf("write richer design.md: %v", err)
	}

	if _, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "42"}); err != nil {
		t.Fatalf("re-Pull: %v", err)
	}
	design, err = os.ReadFile(designPath)
	if err != nil {
		t.Fatalf("read design.md after re-pull: %v", err)
	}
	if string(design) != richer {
		t.Fatalf("re-pull clobbered local design.md: got %q, want %q", design, richer)
	}
}

func TestPullSavesDesignNotes(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{
		Number: 99,
		URL:    "https://github.com/o/r/issues/99",
		Title:  "Feature",
		State:  "open",
		Body:   "# Feature\n\nbody\n\n## Design notes\n\nPicked approach A.\n\n## Tasks\n\n- [ ] one\n",
	}
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "99"})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	designPath := filepath.Join(dir, "changes", res.Slug, "design.md")
	design, err := os.ReadFile(designPath)
	if err != nil {
		t.Fatalf("read design.md: %v", err)
	}
	if !strings.Contains(string(design), "Picked approach A") {
		t.Fatalf("design.md missing content: %s", design)
	}
}

// TestPullNoDesignNotesWritesNoDesignFile: pull must not invent a design.md
// for a change whose issue has no "## Design notes" section.
func TestPullNoDesignNotesWritesNoDesignFile(t *testing.T) {
	dir := t.TempDir()
	issue := fakeIssue{
		Number: 100,
		URL:    "https://github.com/o/r/issues/100",
		Title:  "No design notes",
		State:  "open",
		Body:   "# No design notes\n\nbody\n",
	}
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "100"})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "changes", res.Slug, "design.md")); !os.IsNotExist(err) {
		t.Fatalf("design.md should not exist when the issue has no Design notes section")
	}
}

func TestPull_RequiresIssueForNonGithub(t *testing.T) {
	dir := t.TempDir()
	_, err := Pull(context.Background(), PullOptions{
		OpenSpecDir: dir,
		Provider:    &stubNonGithubProvider{},
		IssueID:     "",
	})
	if err == nil {
		t.Fatal("expected error when no issue for non-github provider")
	}
	if !strings.Contains(err.Error(), "issue id is required for non-github providers") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type stubNonGithubProvider struct{}

func (s *stubNonGithubProvider) Name() string                                         { return "beads" }
func (s *stubNonGithubProvider) Find(context.Context, string) (*Ref, error)           { return nil, nil }
func (s *stubNonGithubProvider) Push(context.Context, WorkItem, *Ref) (Ref, error)    { return Ref{}, nil }
func (s *stubNonGithubProvider) Get(context.Context, string) (FetchedItem, error)     { return FetchedItem{}, nil }

func TestPull_CannotResolveFromBranch(t *testing.T) {
	dir := t.TempDir()
	// On a branch that doesn't match the pattern, Pull should error.
	_, err := Pull(context.Background(), PullOptions{
		OpenSpecDir: dir,
		Provider:    NewGitHubProviderFuncWithRepo("o/r", func(context.Context, ...string) (string, error) { return "", nil }),
		IssueID:     "",
	})
	if err == nil {
		t.Fatal("expected error when branch name doesn't match")
	}
	if !strings.Contains(err.Error(), "could not resolve issue from branch name") {
		t.Fatalf("unexpected error: %v", err)
	}
}
