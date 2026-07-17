package specsync

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestShortenTitle pins the title-hygiene transform: parenthetical asides go,
// backtick markers go (their content stays — tracker titles don't render
// markdown), and anything that would clean down to nothing is left alone.
func TestShortenTitle(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		want        string
		wantChanged bool
	}{
		{
			"clean title passes through",
			"Portal 2026 Q3",
			"Portal 2026 Q3",
			false,
		},
		{
			"parenthetical scope stripped",
			"Design: multi-select flavor of the export fields schema (load → list fields → multi-create)",
			"Design: multi-select flavor of the export fields schema",
			true,
		},
		{
			"backtick markers removed, content kept",
			"Migrate to Postgres 17 `pgx/v6` driver (rewrite ~450 call sites)",
			"Migrate to Postgres 17 pgx/v6 driver",
			true,
		},
		{
			"backticked flag survives",
			"Fix `--dry-run` flag handling",
			"Fix --dry-run flag handling",
			true,
		},
		{
			"detail words are not specsync's to judge",
			"Add HTTP client",
			"Add HTTP client",
			false,
		},
		{
			"nested parentheticals",
			"Fix parser (see RFC (obsolete)) now",
			"Fix parser now",
			true,
		},
		{
			"unbalanced open paren left alone",
			"Fix smiley :( in parser",
			"Fix smiley :( in parser",
			false,
		},
		{
			"unbalanced close paren left alone",
			"Fix step 2) of the wizard",
			"Fix step 2) of the wizard",
			false,
		},
		{
			"unbalanced backtick keeps content",
			"Fix `parseFoo",
			"Fix parseFoo",
			true,
		},
		{
			"all-parenthetical title returned unchanged",
			"(everything in parens)",
			"(everything in parens)",
			false,
		},
		{
			"trailing punctuation trimmed",
			"Ship the release plan.",
			"Ship the release plan",
			true,
		},
		{
			"whitespace collapsed after stripping",
			"Fix cache  (stale entries)  eviction",
			"Fix cache eviction",
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := shortenTitle(tc.in)
			if got != tc.want || changed != tc.wantChanged {
				t.Fatalf("shortenTitle(%q) = (%q, %v), want (%q, %v)", tc.in, got, changed, tc.want, tc.wantChanged)
			}
		})
	}
}

// titleRecorder captures the WorkItems a Sync run pushes.
type titleRecorder struct {
	items []WorkItem
}

func (r *titleRecorder) Name() string { return "github" }
func (r *titleRecorder) Push(_ context.Context, item WorkItem, _ *Ref) (Ref, error) {
	r.items = append(r.items, item)
	return Ref{Provider: "github", ID: item.Slug}, nil
}
func (r *titleRecorder) Find(context.Context, string) (*Ref, error) { return nil, nil }

// TestSyncSurfacesTitleSuggestionWithoutMutating pins the outward half of
// title hygiene: sync pushes the proposal H1 verbatim — the title is the
// author's content — and reports a tighter variant in ItemResult so the
// author can fix it at the source.
func TestSyncSurfacesTitleSuggestionWithoutMutating(t *testing.T) {
	root := t.TempDir()
	verbose := "Migrate to Postgres 17 `pgx/v6` driver (rewrite ~450 call sites)"
	mustWrite(t, filepath.Join(root, "changes", "verbose", "proposal.md"), "# "+verbose+"\n")
	mustWrite(t, filepath.Join(root, "changes", "clean", "proposal.md"), "# Portal 2026 Q3\n")
	mustWrite(t, filepath.Join(root, "changes", "archive", "verbose-old", "proposal.md"), "# "+verbose+"\n")

	prov := &titleRecorder{}
	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	pushed := map[string]string{}
	for _, item := range prov.items {
		pushed[item.Slug] = item.Title
	}
	if pushed["verbose"] != verbose {
		t.Errorf("sync must push the H1 verbatim, pushed %q", pushed["verbose"])
	}

	suggestions := map[string]string{}
	for _, it := range res.Items {
		suggestions[it.Slug] = it.TitleSuggestion
	}
	if want := "Migrate to Postgres 17 pgx/v6 driver"; suggestions["verbose"] != want {
		t.Errorf("TitleSuggestion = %q, want %q", suggestions["verbose"], want)
	}
	if suggestions["clean"] != "" {
		t.Errorf("clean title must not get a suggestion, got %q", suggestions["clean"])
	}
	if suggestions["verbose-old"] != "" {
		t.Errorf("archived change must not get a suggestion (archive is immutable), got %q", suggestions["verbose-old"])
	}
}

// TestPullSurfacesTitleSuggestionWithoutMutating pins the inward half: pull
// writes the issue title verbatim into the proposal H1 — rewriting someone
// else's issue title is not specsync's call — and reports a tighter variant
// so the author can edit the H1 after pulling.
func TestPullSurfacesTitleSuggestionWithoutMutating(t *testing.T) {
	dir := t.TempDir()
	verbose := "Migrate to Postgres 17 `pgx/v6` driver (rewrite ~450 call sites)"
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(fakeIssue{
		Number: 7,
		URL:    "https://github.com/o/r/issues/7",
		Title:  verbose,
		Body:   "the body",
	}, &calls))

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: dir, Provider: prov, IssueID: "7", DryRun: true})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if want := "# " + verbose; !strings.HasPrefix(res.Proposal, want) {
		t.Errorf("pull must keep the issue title verbatim as H1, got proposal starting %q", res.Proposal[:min(len(res.Proposal), 80)])
	}
	if want := "Migrate to Postgres 17 pgx/v6 driver"; res.TitleSuggestion != want {
		t.Errorf("TitleSuggestion = %q, want %q", res.TitleSuggestion, want)
	}
}

// TestShortenTitleIdempotent pins the fixpoint property the title-hygiene
// spec requires: cleaning a cleaned title changes nothing. Without it,
// repeated pull/sync round-trips would erode a title word by word.
func TestShortenTitleIdempotent(t *testing.T) {
	inputs := []string{
		"Portal 2026 Q3",
		"Design: multi-select flavor of the export fields schema (load → list fields → multi-create)",
		"Migrate to Postgres 17 `pgx/v6` driver (rewrite ~450 call sites)",
		"Refactor auth client adapter",
		"Fix `parseFoo",
		"(everything in parens)",
		"Fix smiley :( in parser",
		"Ship the release plan.",
		"",
	}
	for _, in := range inputs {
		once, _ := shortenTitle(in)
		twice, changedAgain := shortenTitle(once)
		if twice != once || changedAgain {
			t.Errorf("shortenTitle not idempotent for %q: first pass %q, second pass %q (changed=%v)", in, once, twice, changedAgain)
		}
	}
}
