package specsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func changeWith(slug, title, body string) Change {
	return Change{Slug: slug, Title: title, Body: body}
}

func TestBuildChangelogOneEntryPerChange(t *testing.T) {
	in := TraceInput{
		Changes: []ChangeRefs{
			{Change: changeWith("ref-key", "Stable ref keys", "# Stable ref keys"), IssueIDs: []string{"35"}},
		},
		Commits: []Commit{
			ParseCommit("a1", "an", "d", "fix: canonical cache key\n\nCloses #35"),
			ParseCommit("a2", "an", "d", "fix: marker upsert on pull (#35)"),
			ParseCommit("a3", "an", "d", "test: cover migration\n\nRefs #35"),
		},
	}
	cl := BuildChangelog(in, nil)
	if len(cl.Entries) != 1 {
		t.Fatalf("want 1 entry, got %d: %+v", len(cl.Entries), cl.Entries)
	}
	e := cl.Entries[0]
	if e.Slug != "ref-key" || len(e.IssueIDs) != 1 || e.IssueIDs[0] != "35" {
		t.Fatalf("entry not bound to change+issue: %+v", e)
	}
}

func TestReleaseNoteExtraction(t *testing.T) {
	body := "# Stable ref keys\n\n## Why\n\nStuff.\n\n## Release note\n\nSync no longer duplicates issues after pull.\n\n## What Changes\n\n- things\n"
	got := ReleaseNote(changeWith("s", "Stable ref keys", body))
	if got != "Sync no longer duplicates issues after pull." {
		t.Fatalf("release note = %q", got)
	}
	// Absent section falls back to the title.
	if got := ReleaseNote(changeWith("s", "Stable ref keys", "# Stable ref keys\n\n## Why\n\nStuff.")); got != "Stable ref keys" {
		t.Fatalf("fallback = %q", got)
	}
	// Empty section falls back too.
	if got := ReleaseNote(changeWith("s", "T", "# T\n\n## Release note\n\n## What Changes\n")); got != "T" {
		t.Fatalf("empty-section fallback = %q", got)
	}
}

func TestCategorizeFromDeltasAndCommits(t *testing.T) {
	feat := ParseCommit("h1", "a", "d", "feat: new thing (#1)")
	fix := ParseCommit("h2", "a", "d", "fix: broken thing (#1)")

	cases := []struct {
		name    string
		deltas  []OpenSpecDelta
		commits []Commit
		want    ChangelogCategory
	}{
		{"added delta wins", []OpenSpecDelta{{Operation: "ADDED", Spec: "s"}}, []Commit{fix}, CatAdded},
		{"removed outranks added", []OpenSpecDelta{{Operation: "ADDED"}, {Operation: "REMOVED"}}, nil, CatRemoved},
		{"modified is changed", []OpenSpecDelta{{Operation: "MODIFIED"}}, nil, CatChanged},
		{"all-fix commits", nil, []Commit{fix, fix}, CatFixed},
		{"any feat commit", nil, []Commit{fix, feat}, CatAdded},
		{"nothing conventional", nil, []Commit{ParseCommit("h3", "a", "d", "wip stuff #1")}, CatChanged},
	}
	for _, tc := range cases {
		if got := categorize(tc.deltas, tc.commits); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestLooseCommitsHonest(t *testing.T) {
	in := TraceInput{
		Commits: []Commit{
			ParseCommit("f1", "a", "d", "fix: broken flag parsing"),
			ParseCommit("c1", "a", "d", "chore: bump deps"),
			ParseCommit("n1", "a", "d", "random non conventional message"),
		},
	}
	cl := BuildChangelog(in, nil)
	if len(cl.Entries) != 1 || cl.Entries[0].Category != CatFixed {
		t.Fatalf("want one Fixed loose entry, got %+v", cl.Entries)
	}
	if cl.OmittedCommits != 1 {
		t.Fatalf("want 1 omitted plumbing commit, got %d", cl.OmittedCommits)
	}
}

func TestRenderChangelogSection(t *testing.T) {
	cl := Changelog{
		Entries: []ChangelogEntry{
			{Category: CatAdded, Text: "Board projection for GitHub Projects.", Slug: "boards", IssueIDs: []string{"37"}},
			{Category: CatFixed, Text: "Sync no longer duplicates issues.", Slug: "ref-key", IssueIDs: []string{"35"}},
			{Category: CatFixed, Text: "broken flag parsing", Hash: "abcdef1234567890"},
			{Category: CatChanged, Text: "New provider interface.", Breaking: true, Slug: "prov"},
		},
		OmittedCommits: 2,
	}
	out := RenderChangelogSection(cl, "0.6.0", "2026-07-13")

	for _, want := range []string{
		"## [0.6.0] - 2026-07-13",
		"### Added\n\n- Board projection for GitHub Projects. (#37)",
		"### Changed\n\n- **Breaking:** New provider interface.",
		"### Fixed\n\n- Sync no longer duplicates issues. (#35)\n- broken flag parsing (abcdef12)",
		"2 internal commit(s) omitted",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "### Removed") || strings.Contains(out, "### Security") {
		t.Errorf("empty categories must be omitted:\n%s", out)
	}
}

func TestApplyChangelogLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	section := func(v, text string) string {
		return RenderChangelogSection(Changelog{Entries: []ChangelogEntry{
			{Category: CatAdded, Text: text, Slug: "s", IssueIDs: []string{"1"}},
		}}, v, "2026-07-13")
	}

	// Create.
	if err := ApplyChangelog(path, "0.6.0", section("0.6.0", "First cut.")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(b), "# Changelog") || !strings.Contains(string(b), "First cut.") {
		t.Fatalf("create: %s", b)
	}

	// Replace same version idempotently.
	if err := ApplyChangelog(path, "0.6.0", section("0.6.0", "Second cut.")); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if strings.Contains(string(b), "First cut.") || strings.Count(string(b), "## [0.6.0]") != 1 {
		t.Fatalf("replace: %s", b)
	}

	// Prepend a newer version above the old one.
	if err := ApplyChangelog(path, "0.7.0", section("0.7.0", "Newer.")); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	s := string(b)
	if strings.Count(s, "## [") != 2 {
		t.Fatalf("prepend count: %s", s)
	}
	if strings.Index(s, "## [0.7.0]") > strings.Index(s, "## [0.6.0]") {
		t.Fatalf("0.7.0 must precede 0.6.0:\n%s", s)
	}
	// The replaced older section must survive intact.
	if !strings.Contains(s, "Second cut.") {
		t.Fatalf("older section lost:\n%s", s)
	}

	// Re-replacing the middle version keeps both neighbors byte-identical.
	before := s
	if err := ApplyChangelog(path, "0.7.0", section("0.7.0", "Newer.")); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if string(b) != before {
		t.Fatalf("idempotent re-apply changed bytes:\n--- before\n%s\n--- after\n%s", before, b)
	}
}

func TestApplyChangelogUnreleasedIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	section := RenderChangelogSection(Changelog{Entries: []ChangelogEntry{
		{Category: CatFixed, Text: "A fix.", Slug: "s", IssueIDs: []string{"9"}},
	}}, "", "2026-07-13")

	if strings.Contains(section, "2026-07-13") {
		t.Fatalf("Unreleased header must carry no date: %s", section)
	}
	for i := 0; i < 2; i++ {
		if err := ApplyChangelog(path, "", section); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(path)
	if got := strings.Count(string(b), "## [Unreleased]"); got != 1 {
		t.Fatalf("want exactly 1 Unreleased section after re-apply, got %d:\n%s", got, b)
	}
}

func TestApplyChangelogPrependWhenFileStartsWithSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	// A user-owned changelog with no "# Changelog" title line.
	if err := os.WriteFile(path, []byte("## [0.6.0] - 2026-07-01\n\n### Added\n\n- Old. (#1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	section := RenderChangelogSection(Changelog{Entries: []ChangelogEntry{
		{Category: CatAdded, Text: "New.", Slug: "s", IssueIDs: []string{"2"}},
	}}, "0.7.0", "2026-07-13")
	if err := ApplyChangelog(path, "0.7.0", section); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	s := string(b)
	if strings.Index(s, "## [0.7.0]") > strings.Index(s, "## [0.6.0]") {
		t.Fatalf("newest section must come first:\n%s", s)
	}
	if !strings.Contains(s, "Old. (#1)") {
		t.Fatalf("existing section lost:\n%s", s)
	}
}

func TestBuildChangelogSortsIssueIDs(t *testing.T) {
	in := TraceInput{
		Changes: []ChangeRefs{
			{Change: changeWith("multi", "Multi-repo change", "# Multi"), IssueIDs: []string{"12", "7"}},
		},
		Commits: []Commit{ParseCommit("a1", "an", "d", "feat: thing (#7)")},
	}
	cl := BuildChangelog(in, nil)
	if len(cl.Entries) != 1 {
		t.Fatalf("want 1 entry, got %+v", cl.Entries)
	}
	got := strings.Join(cl.Entries[0].IssueIDs, ",")
	if got != "7,12" {
		t.Fatalf("issue ids not deterministically sorted: %s", got)
	}
}
