package specsync

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestCategoryOfUsesPathsOnly(t *testing.T) {
	cases := map[string]string{
		"src/auth/login.go":              "source",
		"src/auth/login_test.go":         "test",
		"tests/e2e/flow.ts":              "test",
		"web/app.spec.tsx":               "test",
		"go.sum":                         "generated",
		"web/package-lock.json":          "generated",
		"api/user.pb.go":                 "generated",
		"vendor/lib/x.go":                "generated",
		"README.md":                      "docs",
		"docs/guide/intro.png":           "docs",
		"openspec/changes/x/proposal.md": "docs",
		".github/workflows/ci.yml":       "config",
		"go.mod":                         "config",
		"site/features.json":             "config",
		"Makefile":                       "config",
		"assets/logo.png":                "other",
		"scripts/build.sh":               "source",
	}
	for p, want := range cases {
		if got := CategoryOf(p); got != want {
			t.Errorf("CategoryOf(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestParseReviewFocus(t *testing.T) {
	doc := "# T\n\n## Why\nx\n\n## Review focus\n\n- `src/auth/**` — token handling\n- internal/pay/*.go: money math\n- `db/**`\nprose line\n\n## Impact\n- `ignored/**` — not focus\n"
	want := []ReviewFocus{
		{Glob: "src/auth/**", Reason: "token handling"},
		{Glob: "internal/pay/*.go", Reason: "money math"},
		{Glob: "db/**"},
	}
	if got := ParseReviewFocus(doc); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got := ParseReviewFocus("# T\n\n## Why\nx\n"); got != nil {
		t.Fatalf("no section means nothing declared, got %+v", got)
	}
}

func TestLinkTaskOnlyByExplicitID(t *testing.T) {
	tasks := ParseTaskIDs("# Tasks\n\n## 1. A\n\n- [x] 1.1 Parse\n- [ ] 2.1 Render output\n")
	if len(tasks) != 2 || tasks[1].ID != "2.1" || tasks[1].Text != "Render output" {
		t.Fatalf("tasks = %+v", tasks)
	}
	if got := LinkTask("feat: render (task 2.1)", tasks); got != "2.1" {
		t.Fatalf("explicit id: got %q", got)
	}
	if got := LinkTask("feat: render output for the parse step", tasks); got != "" {
		t.Fatalf("similarity must never link, got %q", got)
	}
	if got := LinkTask("task 9.9", tasks); got != "" {
		t.Fatalf("an id the change does not have must not link, got %q", got)
	}
}

func reviewFixture(t *testing.T) (clone string) {
	t.Helper()
	clone, _ = repoWithOrigin(t)
	commitFiles(t, clone, "feat: parse (task 1.1)", map[string]string{"src/auth/login.go": "1\n2\n3\n", "src/util.go": "1\n"})
	commitFiles(t, clone, "test: cover it", map[string]string{"src/auth/login_test.go": "1\n2\n"})
	commitFiles(t, clone, "chore: deps", map[string]string{"go.sum": strings.Repeat("x\n", 10), "README.md": "doc\n"})
	return clone
}

func TestBuildReviewMapOrdersSizesAndTotals(t *testing.T) {
	clone := reviewFixture(t)
	tasks := ParseTaskIDs("- [ ] 1.1 Parse the input\n")
	m, err := BuildReviewMap(context.Background(), clone, "origin/main", "HEAD", nil, tasks)
	if err != nil {
		t.Fatal(err)
	}
	subjects := []string{}
	for _, c := range m.Commits {
		subjects = append(subjects, c.Subject)
	}
	if want := []string{"feat: parse (task 1.1)", "test: cover it", "chore: deps"}; !reflect.DeepEqual(subjects, want) {
		t.Fatalf("commit order = %v", subjects)
	}
	first := m.Commits[0]
	if first.Task != "1.1" || first.TaskText != "Parse the input" || m.Commits[1].Task != "" {
		t.Fatalf("task links: %+v / %+v", first, m.Commits[1])
	}
	if want := []MapFile{
		{Path: "src/auth/login.go", Category: "source", Additions: 3},
		{Path: "src/util.go", Category: "source", Additions: 1},
	}; !reflect.DeepEqual(first.Files, want) {
		t.Fatalf("files = %+v", first.Files)
	}
	got := map[string]CategoryTotal{}
	for _, tt := range m.Totals {
		got[tt.Category] = tt
	}
	for cat, want := range map[string]CategoryTotal{
		"source":    {Category: "source", Files: 2, Additions: 4},
		"test":      {Category: "test", Files: 1, Additions: 2},
		"docs":      {Category: "docs", Files: 1, Additions: 1},
		"generated": {Category: "generated", Files: 1, Additions: 10},
		"config":    {Category: "config"},
		"other":     {Category: "other"},
	} {
		if got[cat] != want {
			t.Errorf("total %s = %+v, want %+v", cat, got[cat], want)
		}
	}
	if len(m.Judgement) != 0 {
		t.Fatalf("no focus declared, nothing may be marked: %+v", m.Judgement)
	}
	for i := 1; i < len(m.Totals); i++ {
		if m.Totals[i-1].Category == m.Totals[i].Category {
			t.Fatal("duplicate category")
		}
	}
}

func TestBuildReviewMapMarksOnlyDeclaredFocus(t *testing.T) {
	clone := reviewFixture(t)
	focus := []ReviewFocus{{Glob: "src/auth/**", Reason: "token handling"}}
	m, err := BuildReviewMap(context.Background(), clone, "origin/main", "HEAD", focus, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []JudgementFile{
		{Path: "src/auth/login.go", Reason: "token handling", Glob: "src/auth/**"},
		{Path: "src/auth/login_test.go", Reason: "token handling", Glob: "src/auth/**"},
	}
	if !reflect.DeepEqual(m.Judgement, want) {
		t.Fatalf("judgement = %+v", m.Judgement)
	}
	md := m.Markdown()
	if !strings.Contains(md, "- [ ] `src/auth/login.go` — token handling") || strings.Index(md, "Needs human judgement") > strings.Index(md, "Commits in order") {
		t.Fatalf("judgement must come first:\n%s", md)
	}
	if !strings.Contains(md, "| generated | 1 | 10 | 0 |") || !strings.Contains(md, "Task: unlinked") {
		t.Fatalf("markdown:\n%s", md)
	}
}

func TestTreesEqualAfterSquashAndOnDifference(t *testing.T) {
	clone := reviewFixture(t)
	gitFixture(t, clone, "branch", "original")
	gitFixture(t, clone, "reset", "-q", "--soft", "origin/main")
	gitFixture(t, clone, "commit", "-q", "-m", "squashed")
	same, paths, err := TreesEqual(context.Background(), clone, "original", "HEAD")
	if err != nil || !same || paths != nil {
		t.Fatalf("squash keeps content: same=%v paths=%v err=%v", same, paths, err)
	}
	commitFiles(t, clone, "fix: extra", map[string]string{"src/new.go": "n\n", "src/util.go": "changed\n"})
	same, paths, err = TreesEqual(context.Background(), clone, "original", "HEAD")
	if err != nil || same || !reflect.DeepEqual(paths, []string{"src/new.go", "src/util.go"}) {
		t.Fatalf("differing pair: same=%v paths=%v err=%v", same, paths, err)
	}
}

func TestReviewMapLeavesTheRepoUntouched(t *testing.T) {
	clone := reviewFixture(t)
	snapshot := func() string {
		cfg, _ := os.ReadFile(filepath.Join(clone, ".git", "config"))
		return strings.Join([]string{
			gitFixture(t, clone, "status", "--porcelain=v1", "-uall"),
			gitFixture(t, clone, "for-each-ref"),
			gitFixture(t, clone, "ls-files", "-s"),
			gitFixture(t, clone, "rev-parse", "HEAD"),
			string(cfg),
		}, "\n--\n")
	}
	before := snapshot()
	if _, err := BuildReviewMap(context.Background(), clone, "origin/main", "HEAD", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := TreesEqual(context.Background(), clone, "origin/main", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(); after != before {
		t.Fatalf("worktree, index, refs or config changed:\nbefore: %s\nafter: %s", before, after)
	}
}

func TestParseReviewFocusKeepsHyphensInGlobs(t *testing.T) {
	doc := "## Review focus\n\n- cmd/specsync/review-map.go: parses git\n- internal/a-b/*.go - careful here\n- `x-y/**` \u2014 backticked\n- plain-glob\n"
	want := []ReviewFocus{
		{Glob: "cmd/specsync/review-map.go", Reason: "parses git"},
		{Glob: "internal/a-b/*.go", Reason: "careful here"},
		{Glob: "x-y/**", Reason: "backticked"},
		{Glob: "plain-glob"},
	}
	if got := ParseReviewFocus(doc); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCategoryOfAnchorsTestAndDocsRules(t *testing.T) {
	cases := map[string]string{
		"src/latest.java":      "source",
		"src/FooTest.java":     "test",
		"src/FooTests.cs":      "test",
		"src/contests.cs":      "source",
		"app/protest/x.ts":     "source",
		"docs/gen.go":          "source",
		"docs/guide/intro.png": "docs",
		"docs/notes.txt":       "docs",
		"pkg/docs/x.go":        "source",
		"x/__tests__/a.js":     "test",
		"src/attest.go":        "source",
	}
	for p, want := range cases {
		if got := CategoryOf(p); got != want {
			t.Errorf("CategoryOf(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestTreesEqualRejectsOptionLikeAndUnknownRevisions(t *testing.T) {
	clone := reviewFixture(t)
	out := filepath.Join(t.TempDir(), "x")
	if _, _, err := TreesEqual(context.Background(), clone, "--output="+out, "HEAD"); err == nil {
		t.Fatal("an option-like revision must be rejected")
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("git was run with an attacker-chosen option")
	}
	if _, _, err := TreesEqual(context.Background(), clone, "HEAD", "no-such-rev"); err == nil {
		t.Fatal("an unknown revision must be an error, not a difference")
	}
}

func TestBuildReviewMapSkipsMergesAndSaysSo(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFiles(t, clone, "feat: one", map[string]string{"one.go": "1\n"})
	gitFixture(t, clone, "checkout", "-q", "-b", "side", "origin/main")
	commitFiles(t, clone, "feat: side", map[string]string{"side.go": "1\n"})
	gitFixture(t, clone, "checkout", "-q", "feature")
	gitFixture(t, clone, "merge", "-q", "--no-ff", "-m", "Merge side", "side")
	m, err := BuildReviewMap(context.Background(), clone, "origin/main", "HEAD", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	subjects := []string{}
	files := 0
	for _, c := range m.Commits {
		subjects = append(subjects, c.Subject)
		files += len(c.Files)
	}
	sort.Strings(subjects)
	if !reflect.DeepEqual(subjects, []string{"feat: one", "feat: side"}) || m.MergesSkipped != 1 || files != 2 {
		t.Fatalf("subjects=%v merges=%d files=%d", subjects, m.MergesSkipped, files)
	}
	if !strings.Contains(m.Markdown(), "1 merge commit skipped") {
		t.Fatalf("markdown must say merges were skipped:\n%s", m.Markdown())
	}
}

func TestBuildReviewMapParsesAwkwardMessagesAndPaths(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFiles(t, clone, "feat: tab\there\n\nbody line 1\nbody task 2.1\n", map[string]string{"dir with space/a b.go": "1\n", "caf\u00e9.go": "1\n"})
	m, err := BuildReviewMap(context.Background(), clone, "origin/main", "HEAD", nil, []TaskRef{{ID: "2.1", Text: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Commits) != 1 || m.Commits[0].Subject != "feat: tab\there" || m.Commits[0].Task != "2.1" {
		t.Fatalf("commit = %+v", m.Commits)
	}
	got := []string{}
	for _, f := range m.Commits[0].Files {
		got = append(got, f.Path)
	}
	if !reflect.DeepEqual(got, []string{"caf\u00e9.go", "dir with space/a b.go"}) {
		t.Fatalf("files = %v", got)
	}
}
