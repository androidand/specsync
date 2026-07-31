package specsync

import (
	"testing"
)

func TestExtractRepoFromDir(t *testing.T) {
	tests := []struct {
		dir  string
		want string
	}{
		{"/Users/andreas/dev/brick-now/openspec", "brick-now"},
		{"/Users/andreas/dev/tengil/openspec/changes/foo", "tengil"},
		{"/home/user/project/openspec", "project"},
		{"/openspec", ""},          // edge case: no parent
		{"openspec", "openspec"},   // bare openspec — no parent, returns full dir
		{"openspec/changes/foo", "openspec/changes/foo"}, // openspec at start, no parent, returns full dir
	}
	for _, tt := range tests {
		if got := extractRepoFromDir(tt.dir); got != tt.want {
			t.Errorf("extractRepoFromDir(%q) = %q, want %q", tt.dir, got, tt.want)
		}
	}
}

func TestLooksLikeFilePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"src/foo.ts", true},
		{"lib/bar.js", true},
		{"test/baz.tsx", true},
		{"./foo.go", true},
		{"a/b/c/d.md", true},
		{"http://example.com/foo", false},
		{"https://example.com/foo", false},
		{"example.com", false},
		{"no-slash", false},
		{"no-dot", false},
		{"a/b/.hidden", true},
		{"a/b/c.verylongext", false},
		{"a/b/c.d", true},
		{"a/b/c.svelte", true},
		{"a/b/c.astro", true},
	}
	for _, tt := range tests {
		if got := looksLikeFilePath(tt.path); got != tt.want {
			t.Errorf("looksLikeFilePath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestExtractFilePaths(t *testing.T) {
	tests := []struct {
		text string
		want []string
	}{
		{
			"Edit src/main.ts and test/main.test.ts",
			[]string{"src/main.ts", "test/main.test.ts"},
		},
		{
			"Add lib/utils.ts\nAlso update pkg/index.js",
			[]string{"lib/utils.ts", "pkg/index.js"},
		},
		{
			"See https://example.com/foo",
			[]string{},
		},
		{
			"No file paths here at all.",
			[]string{},
		},
		{
			"Edit `src/main.ts` to add logging",
			[]string{"src/main.ts"},
		},
		{
			"Also update `lib/utils.go` and `pkg/config.yaml`",
			[]string{"lib/utils.go", "pkg/config.yaml"},
		},
		{
			"See [src/main.ts](https://example.com) for details",
			[]string{},
		},
		{
			"Edit (src/main.ts) to add logging",
			[]string{"src/main.ts"},
		},
		{
			"Update \"lib/utils.go\" and (pkg/config.yaml)",
			[]string{"lib/utils.go", "pkg/config.yaml"},
		},
		{
			"Fix 'src/components/Button.tsx' and {lib/format.ts}",
			[]string{"src/components/Button.tsx", "lib/format.ts"},
		},
		{
			"Update ./src/main.ts and ../lib/utils.js",
			[]string{"./src/main.ts", "../lib/utils.js"},
		},
		{
			"See src/main.ts#L42 for context",
			[]string{"src/main.ts"},
		},
		{
			"Check src/main.ts?line=42 for details",
			[]string{"src/main.ts"},
		},
		{
			"See <a href=\"src/main.ts\">link</a> for context",
			[]string{"src/main.ts"},
		},
		{
			"src/main.ts#L42?line=42 with fragment and query",
			[]string{"src/main.ts"},
		},
	}
	for _, tt := range tests {
		got := extractFilePaths(tt.text)
		if len(got) != len(tt.want) {
			t.Errorf("extractFilePaths(%q) = %v (len %d), want %v (len %d)",
				tt.text, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i, w := range tt.want {
			if got[i] != w {
				t.Errorf("extractFilePaths(%q)[%d] = %q, want %q", tt.text, i, got[i], w)
			}
		}
	}
}

func TestContainsPath(t *testing.T) {
	tests := []struct {
		paths  []string
		target string
		want   bool
	}{
		{[]string{"src/foo.ts", "lib/bar.js"}, "src/foo.ts", true},
		{[]string{"src/foo.ts"}, "lib/bar.js", false},
		{[]string{"a/b/c.ts"}, "b/c.ts", true},     // suffix match
		{[]string{"a.ts"}, "a.ts", true},             // exact match
		{[]string{"a/b/c.ts"}, "a/b", true},          // directory prefix match
		{[]string{"src/main/index.ts", "lib/utils.go"}, "src/main", true}, // directory prefix
		{[]string{"src/main.ts"}, "src", true},       // single component prefix
		{[]string{}, "foo.ts", false},                // empty list
	}
	for _, tt := range tests {
		if got := containsPath(tt.paths, tt.target); got != tt.want {
			t.Errorf("containsPath(%v, %q) = %v, want %v", tt.paths, tt.target, got, tt.want)
		}
	}
}

func TestPathCorrelates(t *testing.T) {
	c1 := Change{Slug: "a", Body: "Edit src/main.ts and lib/utils.go"}
	c2 := Change{Slug: "b", Body: "Also touch src/main.ts and test/unit.ts"}
	c3 := Change{Slug: "c", Body: "Unrelated work in docs/"}

	if !pathCorrelates(c1, c2, nil) {
		t.Error("expected c1 and c2 to correlate by path src/main.ts")
	}
	if pathCorrelates(c1, c3, nil) {
		t.Error("c1 and c3 should not correlate")
	}

	// Scope path match
	if pathCorrelates(c1, c3, []string{"docs/"}) {
		t.Error("c1 and c3 should not correlate even with docs/ scope path")
	}

	// Scope path in title should also match
	c4 := Change{Slug: "d", Title: "Fix docs/ navigation", Body: "some other work"}
	if !pathCorrelates(c3, c4, []string{"docs/"}) {
		t.Error("c3 and c4 should correlate by docs/ scope path (in title)")
	}

	// Scope path in tasks markdown should also match
	c5 := Change{Slug: "e", Body: "implement feature", TasksMarkdown: "- Update docs/readme.md"}
	if !pathCorrelates(c3, c5, []string{"docs/"}) {
		t.Error("c3 and c5 should correlate by docs/ scope path (in tasks)")
	}

	// File path in title should also match (not just scope path)
	c6 := Change{Slug: "f", Title: "Fix src/main.ts rendering", Body: "some other work"}
	if !pathCorrelates(c1, c6, nil) {
		t.Error("c1 and c6 should correlate by file path src/main.ts (in title)")
	}

	// File path in one change's tasks and another's title
	c7 := Change{Slug: "g", Title: "Refactor lib/utils.go", Body: "misc changes"}
	c8 := Change{Slug: "h", Body: "implement feature", TasksMarkdown: "- Update lib/utils.go"}
	if !pathCorrelates(c7, c8, nil) {
		t.Error("c7 and c8 should correlate by file path lib/utils.go (title vs tasks)")
	}
}

func TestCrossRepoCorrelationTopic(t *testing.T) {
	changeRefs := []ChangeRefs{
		{Change: Change{Dir: "/repo-a/openspec", Slug: "compose-setup", Title: "Compose setup", Body: "docker compose"}, IssueIDs: []string{"1"}},
		{Change: Change{Dir: "/repo-b/openspec", Slug: "compose-config", Title: "Compose config", Body: "compose configuration"}, IssueIDs: []string{"2"}},
		{Change: Change{Dir: "/repo-a/openspec", Slug: "unrelated", Title: "Something else", Body: "build pipeline changes"}, IssueIDs: []string{"3"}},
	}

	scope := Scope{Topic: "compose"}
	rels := CrossRepoCorrelation(changeRefs, scope)

	// compose-setup should relate to compose-config (topic correlation)
	relsA, ok := rels["compose-setup"]
	if !ok || len(relsA) != 1 {
		t.Fatalf("compose-setup should have 1 cross-repo relationship, got %d", len(relsA))
	}
	if relsA[0].RelatedChange.Slug != "compose-config" {
		t.Errorf("expected compose-config, got %s", relsA[0].RelatedChange.Slug)
	}
	if relsA[0].Provenance != Provenance("topic-correlation") {
		t.Errorf("expected topic-correlation, got %s", relsA[0].Provenance)
	}

	// compose-config should relate to compose-setup
	relsB, ok := rels["compose-config"]
	if !ok || len(relsB) != 1 {
		t.Fatalf("compose-config should have 1 cross-repo relationship, got %d", len(relsB))
	}

	// unrelated should have no relationships
	if relsU, ok := rels["unrelated"]; ok && len(relsU) > 0 {
		t.Errorf("unrelated should have no cross-repo relationships, got %d", len(relsU))
	}
}

func TestCrossRepoCorrelationLinksMD(t *testing.T) {
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec",
				Slug:  "feature-x",
				Title: "Feature X",
				Body:  "implement feature x",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change: Change{
				Dir:      "/repo-b/openspec",
				Slug:     "feature-y",
				Title:    "Feature Y",
				Body:     "implement feature y",
			},
			IssueIDs: []string{"42"}, // issue 42 is bound to feature-y
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["feature-x"]
	if !ok || len(relsA) != 1 {
		t.Fatalf("feature-x should have 1 cross-repo relationship (links-md), got %d", len(relsA))
	}
	if relsA[0].RelatedChange.Slug != "feature-y" {
		t.Errorf("expected feature-y, got %s", relsA[0].RelatedChange.Slug)
	}
	if relsA[0].Provenance != ProvLinksMD {
		t.Errorf("expected links-md, got %s", relsA[0].Provenance)
	}
}

func TestCrossRepoCorrelationLinksMDNoDups(t *testing.T) {
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec",
				Slug:  "feature-x",
				Title: "Feature X",
				Body:  "implement feature x",
				// Two links to the same issue
				Links: []Ref{
					{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"},
					{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/pull/42"},
				},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "feature-y", Title: "Feature Y", Body: "implement feature y"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["feature-x"]
	if !ok || len(relsA) != 1 {
		t.Fatalf("feature-x should have 1 cross-repo relationship (deduped), got %d", len(relsA))
	}
	if relsA[0].RelatedChange.Slug != "feature-y" {
		t.Errorf("expected feature-y, got %s", relsA[0].RelatedChange.Slug)
	}
}

func TestCrossRepoCorrelationSameRepoExcluded(t *testing.T) {
	changeRefs := []ChangeRefs{
		{Change: Change{Dir: "/repo-a/openspec", Slug: "a1", Title: "A1", Body: "compose"}, IssueIDs: []string{}},
		{Change: Change{Dir: "/repo-a/openspec", Slug: "a2", Title: "A2", Body: "compose"}, IssueIDs: []string{}},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{Topic: "compose"})

	// Both changes are in the same repo — no cross-repo relationships
	for slug, rels := range rels {
		for _, rel := range rels {
			if extractRepoFromDir(rel.RelatedChange.Dir) == "repo-a" {
				t.Errorf("same-repo change %s correlated with %s — should be excluded", slug, rel.RelatedChange.Slug)
			}
		}
	}
}

func TestCrossRepoCorrelationLinksMDSameRepoExcluded(t *testing.T) {
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-a/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-a/openspec", Slug: "a2"},
			IssueIDs: []string{"42"}, // same issue bound to same repo
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	// a1's link references issue 42, which is bound to a2 in the same repo — should NOT correlate
	if relsA, ok := rels["a1"]; ok {
		for _, rel := range relsA {
			if extractRepoFromDir(rel.RelatedChange.Dir) == "repo-a" {
				t.Errorf("same-repo change a1 correlated with %s via links.md — should be excluded", rel.RelatedChange.Slug)
			}
		}
	}
}

func TestCrossRepoCorrelationLinksMDMultiTarget(t *testing.T) {
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1"},
			IssueIDs: []string{"42"}, // same issue bound to repo-b
		},
		{
			Change:   Change{Dir: "/repo-c/openspec", Slug: "c1"},
			IssueIDs: []string{"42"}, // same issue also bound to repo-c
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	// a1 should correlate with both b1 and c1
	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 2 {
		t.Fatalf("a1 should have 2 cross-repo relationships, got %d", len(relsA))
	}

	// Collect correlated slugs
	correlated := make(map[string]bool)
	for _, rel := range relsA {
		correlated[rel.RelatedChange.Slug] = true
	}
	if !correlated["b1"] || !correlated["c1"] {
		t.Errorf("a1 should correlate with both b1 and c1, got %v", correlated)
	}
}

func TestCrossRepoCorrelationMultipleProvenance(t *testing.T) {
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1", Title: "Compose setup", Body: "edit src/main.ts",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1", Title: "Compose config", Body: "update src/main.ts"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{Topic: "compose"})

	// a1 and b1 are correlated by topic, path, and links.md
	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 3 {
		t.Fatalf("a1 should have 3 cross-repo relationships (topic, path, links), got %d", len(relsA))
	}

	provenances := make(map[string]bool)
	for _, rel := range relsA {
		provenances[string(rel.Provenance)] = true
	}
	for _, p := range []string{"topic-correlation", "path-correlation", "links-md"} {
		if !provenances[p] {
			t.Errorf("a1 should have %s provenance, got %v", p, provenances)
		}
	}
}

func TestCrossRepoCorrelationMultipleLinks(t *testing.T) {
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1",
				Links: []Ref{
					{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"},
					{Provider: "github", ID: "99", URL: "https://github.com/owner/repo-b/issues/99"},
				},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1"},
			IssueIDs: []string{"42"},
		},
		{
			Change:   Change{Dir: "/repo-c/openspec", Slug: "c1"},
			IssueIDs: []string{"99"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 2 {
		t.Fatalf("a1 should have 2 cross-repo relationships (two links), got %d", len(relsA))
	}

	correlated := make(map[string]bool)
	for _, rel := range relsA {
		correlated[rel.RelatedChange.Slug] = true
	}
	if !correlated["b1"] || !correlated["c1"] {
		t.Errorf("a1 should correlate with b1 and c1, got %v", correlated)
	}
}

func TestCrossRepoCorrelationLinksMDSingleDirection(t *testing.T) {
	// A has a link to issue 42, B has issue 42 bound, B has no links.
	// A → B should be correlated by links.md, but B → A should NOT be correlated by links.md.
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 1 {
		t.Fatalf("a1 should have 1 cross-repo relationship (links-md), got %d", len(relsA))
	}
	if relsA[0].Provenance != ProvLinksMD {
		t.Errorf("expected links-md, got %s", relsA[0].Provenance)
	}

	relsB, ok := rels["b1"]
	if ok && len(relsB) > 0 {
		for _, rel := range relsB {
			if rel.Provenance == ProvLinksMD {
				t.Errorf("b1 should NOT have links-md provenance (b1 has no links), got %v", rel.Provenance)
			}
		}
	}
}

func TestCrossRepoCorrelationLinksMDUnboundIssue(t *testing.T) {
	// A has a link to issue 42, but no change has issue 42 bound.
	// A should have no links.md correlations.
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1"},
			IssueIDs: []string{}, // no issue bound
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if ok && len(relsA) > 0 {
		t.Errorf("a1 should have no links-md correlations (issue unbound), got %d", len(relsA))
	}
}

func TestCrossRepoCorrelationLinksMDEmptyRefID(t *testing.T) {
	// A has a link with an empty ID, which should be skipped.
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1",
				Links: []Ref{{Provider: "github", ID: "", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1"},
			IssueIDs: []string{},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if ok && len(relsA) > 0 {
		t.Errorf("a1 should have no links-md correlations (empty ref ID), got %d", len(relsA))
	}
}

func TestCrossRepoCorrelationLinksMDBareID(t *testing.T) {
	// A has a link with bare ID "42", B has "42" bound.
	// A should correlate with B via links.md.
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 1 {
		t.Fatalf("a1 should have 1 cross-repo relationship (links-md), got %d", len(relsA))
	}
	if relsA[0].Provenance != ProvLinksMD {
		t.Errorf("expected links-md, got %s", relsA[0].Provenance)
	}
}

func TestCrossRepoCorrelationLinksMDDuplicateRefs(t *testing.T) {
	// A has two links to the same issue 42 (duplicate), B has 42 bound.
	// A should have only 1 correlation with B (deduplicated).
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1",
				Links: []Ref{
					{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"},
					{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"},
				},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 1 {
		t.Fatalf("a1 should have 1 cross-repo relationship (deduplicated), got %d", len(relsA))
	}
}

func TestCrossRepoCorrelationLinksMDWithTopicConflict(t *testing.T) {
	// A has a link to issue 42, B has 42 bound, AND they share topic "compose".
	// A should correlate with B by both links.md and topic.
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1", Title: "Compose setup",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1", Title: "Compose config"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{Topic: "compose"})

	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 2 {
		t.Fatalf("a1 should have 2 cross-repo relationships (links-md and topic), got %d", len(relsA))
	}

	provenances := make(map[string]bool)
	for _, rel := range relsA {
		provenances[string(rel.Provenance)] = true
	}
	if !provenances[string(ProvLinksMD)] || !provenances[string(ProvTopicCorrelation)] {
		t.Errorf("expected both links-md and topic-correlation, got %v", provenances)
	}
}

func TestCrossRepoCorrelationLinksMDWithPathConflict(t *testing.T) {
	// A has a link to issue 42, B has 42 bound, AND they share path "src/main.ts".
	// A should correlate with B by both links.md and path.
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1", Body: "edit src/main.ts",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1", Body: "update src/main.ts"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 2 {
		t.Fatalf("a1 should have 2 cross-repo relationships (links-md and path), got %d", len(relsA))
	}

	provenances := make(map[string]bool)
	for _, rel := range relsA {
		provenances[string(rel.Provenance)] = true
	}
	if !provenances[string(ProvLinksMD)] || !provenances[string(ProvPathCorrelation)] {
		t.Errorf("expected both links-md and path-correlation, got %v", provenances)
	}
}

func TestCrossRepoCorrelationLinksMDSameRepoWithTopic(t *testing.T) {
	// A and B are in the same repo, share topic "compose".
	// A should NOT correlate with B via topic (same repo).
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1", Title: "Compose setup",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-a/openspec", Slug: "b1", Title: "Compose config"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{Topic: "compose"})

	relsA, ok := rels["a1"]
	if ok && len(relsA) > 0 {
		t.Errorf("a1 should have no cross-repo relationships (same repo), got %d", len(relsA))
	}
}

func TestCrossRepoCorrelationLinksMDSameRepoWithPath(t *testing.T) {
	// A and B are in the same repo, share path "src/main.ts".
	// A should NOT correlate with B via path (same repo).
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1", Body: "edit src/main.ts",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-a/openspec", Slug: "b1", Body: "update src/main.ts"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if ok && len(relsA) > 0 {
		t.Errorf("a1 should have no cross-repo relationships (same repo), got %d", len(relsA))
	}
}

func TestCrossRepoCorrelationLinksMDSameRepoWithBoth(t *testing.T) {
	// A and B are in the same repo, share topic "compose" and path "src/main.ts".
	// A should NOT correlate with B at all (same repo).
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1", Title: "Compose setup", Body: "edit src/main.ts",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-a/openspec", Slug: "b1", Title: "Compose config", Body: "update src/main.ts"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{Topic: "compose"})

	relsA, ok := rels["a1"]
	if ok && len(relsA) > 0 {
		t.Errorf("a1 should have no cross-repo relationships (same repo), got %d", len(relsA))
	}
}

func TestCrossRepoCorrelationLinksMDMultipleSameRepo(t *testing.T) {
	// A is in repo-a, B and C are in the same repo, but different from A.
	// A should correlate with both B and C.
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1", Title: "Compose setup", Body: "edit src/main.ts",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1", Title: "Compose config", Body: "update src/main.ts"},
			IssueIDs: []string{"42"},
		},
		{
			Change:   Change{Dir: "/repo-c/openspec", Slug: "c1", Title: "Compose deployment", Body: "update src/main.ts"},
			IssueIDs: []string{},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{Topic: "compose"})

	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 5 {
		t.Fatalf("a1 should have 5 cross-repo relationships (b1: links+topic+path, c1: topic+path), got %d", len(relsA))
	}

	correlated := make(map[string]bool)
	for _, rel := range relsA {
		correlated[rel.RelatedChange.Slug] = true
	}
	if !correlated["b1"] || !correlated["c1"] {
		t.Errorf("a1 should correlate with b1 and c1, got %v", correlated)
	}
}

func TestCrossRepoCorrelationLinksMDWithProviderPrefix(t *testing.T) {
	// A has a link with provider-prefixed ID "github/42", B has "42" bound.
	// A should correlate with B via links.md.
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1",
				Links: []Ref{{Provider: "github", ID: "github/42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-b/openspec", Slug: "b1"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 1 {
		t.Fatalf("a1 should have 1 cross-repo relationship (links-md), got %d", len(relsA))
	}
	if relsA[0].Provenance != ProvLinksMD {
		t.Errorf("expected links-md, got %s", relsA[0].Provenance)
	}
}

func TestCrossRepoCorrelationLinksMDSameRepoWithPathMatch(t *testing.T) {
	// A and B are in the same repo, share path "src/main.ts".
	// A should NOT correlate with B via path (same repo).
	changeRefs := []ChangeRefs{
		{
			Change: Change{
				Dir:   "/repo-a/openspec", Slug: "a1", Body: "edit src/main.ts",
				Links: []Ref{{Provider: "github", ID: "42", URL: "https://github.com/owner/repo-b/issues/42"}},
			},
			IssueIDs: []string{},
		},
		{
			Change:   Change{Dir: "/repo-a/openspec", Slug: "b1", Body: "update src/main.ts"},
			IssueIDs: []string{"42"},
		},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	relsA, ok := rels["a1"]
	if ok && len(relsA) > 0 {
		t.Errorf("a1 should have no cross-repo relationships (same repo), got %d", len(relsA))
	}
}

func TestCrossRepoCorrelationPath(t *testing.T) {
	changeRefs := []ChangeRefs{
		{Change: Change{Dir: "/repo-a/openspec", Slug: "a1", Title: "A1", Body: "edit src/main.ts"}, IssueIDs: nil},
		{Change: Change{Dir: "/repo-b/openspec", Slug: "b1", Title: "B1", Body: "update src/main.ts"}, IssueIDs: nil},
		{Change: Change{Dir: "/repo-b/openspec", Slug: "b2", Title: "B2", Body: "fix docs/readme.md"}, IssueIDs: nil},
	}

	rels := CrossRepoCorrelation(changeRefs, Scope{})

	// a1 and b1 share path src/main.ts
	relsA, ok := rels["a1"]
	if !ok || len(relsA) != 1 {
		t.Fatalf("a1 should have 1 cross-repo relationship (path), got %d", len(relsA))
	}
	if relsA[0].RelatedChange.Slug != "b1" {
		t.Errorf("expected b1, got %s", relsA[0].RelatedChange.Slug)
	}
	if relsA[0].Provenance != Provenance("path-correlation") {
		t.Errorf("expected path-correlation, got %s", relsA[0].Provenance)
	}

	// b1 should correlate back to a1
	if _, ok := rels["b1"]; !ok {
		t.Fatal("b1 should have cross-repo relationships")
	}
}
