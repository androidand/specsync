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
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "7"})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// The ref cache must now bind the change to issue 7 so a push updates it.
	refs, err := loadRefs(res.Dir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	if refs["github"].ID != "7" {
		t.Fatalf("cached ref id = %q, want 7", refs["github"].ID)
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
