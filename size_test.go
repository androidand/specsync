package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseBudget(t *testing.T) {
	cases := []struct {
		in   string
		want Budget
		bad  bool
	}{
		{"files=15,lines=400,commits=30", Budget{Files: 15, Lines: 400, Commits: 30}, false},
		{"files<=5", Budget{Files: 5}, false},
		{" lines = 10 , commits<=2 ", Budget{Lines: 10, Commits: 2}, false},
		{"", Budget{}, false},
		{"files=x", Budget{}, true},
		{"words=3", Budget{}, true},
		{"files", Budget{}, true},
	}
	for _, c := range cases {
		got, err := ParseBudget(c.in)
		if (err != nil) != c.bad || (!c.bad && got != c.want) {
			t.Errorf("ParseBudget(%q) = %+v, %v; want %+v bad=%v", c.in, got, err, c.want, c.bad)
		}
	}
}

func TestResolveBudgetPrecedence(t *testing.T) {
	got, err := ResolveBudget("files=1", "files<=2,lines<=20", "files=3,lines=30,commits=300")
	if err != nil {
		t.Fatal(err)
	}
	want := Budget{Files: 1, Lines: 20, Commits: 300}
	if got != want {
		t.Fatalf("flag, then slice, then env per key: got %+v, want %+v", got, want)
	}
	def, err := ResolveBudget("", "", "")
	if err != nil || def != DefaultBudget {
		t.Fatalf("defaults: %+v, %v", def, err)
	}
	if _, err := ResolveBudget("bogus", "", ""); err == nil {
		t.Fatal("a malformed flag must be an error")
	}
}

func TestBudgetExceededNamesTheLimit(t *testing.T) {
	b := Budget{Files: 3, Lines: 100, Commits: 2}
	if got := b.Exceeded(&BranchSize{Files: 3, Additions: 60, Deletions: 40, Commits: 2}); len(got) != 0 {
		t.Fatalf("at the limit is within budget: %v", got)
	}
	got := b.Exceeded(&BranchSize{Files: 4, Additions: 90, Deletions: 20, Commits: 3})
	want := []string{"files 4 > 3", "lines 110 > 100", "commits 3 > 2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := (Budget{}).Exceeded(&BranchSize{Files: 999}); len(got) != 0 {
		t.Fatalf("a zero limit means no limit: %v", got)
	}
}

func TestCommitScopes(t *testing.T) {
	cases := []struct {
		name string
		subs []string
		want []string
	}{
		{"two scopes", []string{"feat(billing): a", "fix(onboarding): b", "feat(billing): c"}, []string{"billing", "onboarding"}},
		{"one scope", []string{"feat(a): x", "fix(a)!: y"}, []string{"a"}},
		{"none", []string{"feat: x", "fix: y", "plain subject"}, nil},
		{"malformed", []string{"feat(: x", "feat(a:b", "(a): x", "Feat(A): x", "feat(a) x"}, nil},
	}
	for _, c := range cases {
		if got := CommitScopes(c.subs); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func commitFiles(t *testing.T, dir, msg string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		gitFixture(t, dir, "add", name)
	}
	gitFixture(t, dir, "commit", "-q", "-m", msg)
}

func TestAnalyzeBranchCountsAgainstTheBaseAndHonoursExclude(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFiles(t, clone, "feat(billing): add", map[string]string{"src/a.go": "1\n2\n3\n", "src/b.go": "1\n"})
	commitFiles(t, clone, "fix(onboarding): lock", map[string]string{"go.sum": strings.Repeat("x\n", 50)})

	size, scopes, err := AnalyzeBranch(context.Background(), clone, "origin/main", "HEAD", nil)
	if err != nil {
		t.Fatal(err)
	}
	if size.Files < 3 || size.Commits < 2 || size.Additions < 54 {
		t.Fatalf("size = %+v", size)
	}
	if !reflect.DeepEqual(scopes, []string{"billing", "onboarding"}) {
		t.Fatalf("scopes = %v", scopes)
	}

	excluded, _, err := AnalyzeBranch(context.Background(), clone, "origin/main", "HEAD", []string{"go.sum"})
	if err != nil {
		t.Fatal(err)
	}
	if excluded.Files != size.Files-1 || excluded.Additions != size.Additions-50 {
		t.Fatalf("exclude go.sum: %+v vs %+v", excluded, size)
	}
	globbed, _, _ := AnalyzeBranch(context.Background(), clone, "origin/main", "HEAD", []string{"src/*.go"})
	if globbed.Files != size.Files-2 {
		t.Fatalf("exclude src/*.go: %+v vs %+v", globbed, size)
	}
}

func TestAnalyzeBranchWithinBudgetAddsNoFlag(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFiles(t, clone, "feat: tiny", map[string]string{"t.txt": "x\n"})
	ls, err := CollectLocalState(context.Background(), clone, LocalStateOptions{Budget: DefaultBudget})
	if err != nil {
		t.Fatal(err)
	}
	if len(ls.Exceeded) != 0 || len(ls.Scopes) != 0 {
		t.Fatalf("within budget: %+v", ls)
	}
	if got := RowAttention(ls, nil); hasFlag(got, "oversized") || hasFlag(got, "mixed-scopes") || hasFlag(got, "no-slice-plan") {
		t.Fatalf("attention = %v", got)
	}
}

func hasFlag(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestOversizedOverEachLimit(t *testing.T) {
	for _, c := range []struct {
		name   string
		budget Budget
		want   string
	}{
		{"files", Budget{Files: 1}, "files 2 > 1"},
		{"lines", Budget{Lines: 3}, "lines 4 > 3"},
		{"commits", Budget{Commits: 1}, "commits 2 > 1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			clone, _ := repoWithOrigin(t)
			commitFiles(t, clone, "feat: one", map[string]string{"x.txt": "1\n2\n"})
			commitFiles(t, clone, "feat: two", map[string]string{"y.txt": "1\n2\n"})
			ls, err := CollectLocalState(context.Background(), clone, LocalStateOptions{Budget: c.budget})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ls.Exceeded, []string{c.want}) {
				t.Fatalf("exceeded = %v, want %q", ls.Exceeded, c.want)
			}
			if !hasFlag(RowAttention(ls, nil), "oversized") {
				t.Fatalf("attention = %v", RowAttention(ls, nil))
			}
		})
	}
}

func TestMixedScopesFlagNeedsTwoScopes(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitFiles(t, clone, "feat(a): x", map[string]string{"x.txt": "1\n"})
	ls, _ := CollectLocalState(context.Background(), clone, LocalStateOptions{})
	if hasFlag(RowAttention(ls, nil), "mixed-scopes") {
		t.Fatalf("one scope must not flag: %v", ls.Scopes)
	}
	commitFiles(t, clone, "feat(b): y", map[string]string{"y.txt": "1\n"})
	ls, _ = CollectLocalState(context.Background(), clone, LocalStateOptions{})
	if !hasFlag(RowAttention(ls, nil), "mixed-scopes") || !reflect.DeepEqual(ls.Scopes, []string{"a", "b"}) {
		t.Fatalf("two scopes must flag: %v %v", ls.Scopes, RowAttention(ls, nil))
	}
}

func TestNoSlicePlanOnlyWhenOversizedAndUndeclared(t *testing.T) {
	over := &LocalState{Exceeded: []string{"lines 5 > 1"}, NoSlicePlan: true}
	if !hasFlag(RowAttention(over, nil), "no-slice-plan") {
		t.Fatal("oversized and no slices must flag")
	}
	if hasFlag(RowAttention(&LocalState{NoSlicePlan: true}, nil), "no-slice-plan") {
		t.Fatal("within budget must not flag")
	}
	if hasFlag(RowAttention(&LocalState{Exceeded: []string{"lines 5 > 1"}}, nil), "no-slice-plan") {
		t.Fatal("a change with slices must not flag")
	}
}

func TestSliceDeclarations(t *testing.T) {
	dir := t.TempDir()
	if HasSlices(dir) {
		t.Fatal("no files, no slices")
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte("# Tasks\n\n## Slice 1: parser\n- [ ] a\nbudget: files<=5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasSlices(dir) {
		t.Fatal("a Slice heading declares slices")
	}
	if got := SliceBudgetLine(dir); got != "files<=5" {
		t.Fatalf("budget line = %q", got)
	}
}

func TestSizeAnalysisNeverGeneratesSlices(t *testing.T) {
	dir := t.TempDir()
	before := fmt.Sprint(listDir(t, dir))
	_ = HasSlices(dir)
	_ = SliceBudgetLine(dir)
	if after := fmt.Sprint(listDir(t, dir)); after != before {
		t.Fatalf("analysis must not write files: %s -> %s", before, after)
	}
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
