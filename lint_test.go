package specsync

import (
	"strings"
	"testing"
)

func rules(fs []LintFinding) string {
	var r []string
	for _, f := range fs {
		r = append(r, f.Rule)
	}
	return strings.Join(r, ",")
}

func TestLintTitleLengthAndConventionalCommits(t *testing.T) {
	cfg := DefaultText()
	long := "feat(sync): " + strings.Repeat("a", 70)
	if got := rules(LintTitle("pr", long, cfg)); got != "title-length" {
		t.Fatalf("long title: %q", got)
	}
	f := LintTitle("pr", "Add the thing", cfg)
	if rules(f) != "conventional-commit" || !strings.Contains(f[0].Suggestion, "type(scope): subject") {
		t.Fatalf("no prefix: %+v", f)
	}
	if got := LintTitle("pr", "fix(sync): keep links out of sections", cfg); len(got) != 0 {
		t.Fatalf("clean title: %+v", got)
	}
}

func TestLintTitlePlainProfileDoesNotRequireConventionalCommits(t *testing.T) {
	cfg := DefaultText()
	cfg.Profile = "plain"
	if got := LintTitle("pr", "Add the thing", cfg); len(got) != 0 {
		t.Fatalf("plain: %+v", got)
	}
	if got := rules(LintTitle("pr", strings.Repeat("a", 80), cfg)); got != "title-length" {
		t.Fatalf("length still applies: %q", got)
	}
}

func TestLintIssueTitle(t *testing.T) {
	cfg := DefaultText()
	if got := rules(LintTitle("issue", "Show Where Your Work Stands", cfg)); got != "sentence-case" {
		t.Fatalf("title case: %q", got)
	}
	if got := rules(LintTitle("issue", "Fix the sync in pull.go", cfg)); got != "no-file-names" {
		t.Fatalf("file name: %q", got)
	}
	f := LintTitle("issue", "Show where work stands (see the long note about every detail of the status output for people)", cfg)
	if rules(f) != "title-length" || f[0].Suggestion == "" {
		t.Fatalf("long issue title: %+v", f)
	}
	if got := LintTitle("issue", "Show where your work stands", cfg); len(got) != 0 {
		t.Fatalf("clean: %+v", got)
	}
}

func TestLintHeadersSpecsyncNeedsAreAllowedAndSlicesExempt(t *testing.T) {
	doc := "# T\n\n## Release note\n\nShort.\n\n## Slice 2: Start and new things in the flow\n\nShort.\n"
	if got := LintDocument("issue", doc, DefaultText()); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestLintHeaderRules(t *testing.T) {
	cfg := DefaultText()
	got := rules(LintDocument("issue", "# T\n\n## Background\n\nShort.\n\n## What this change does\n\nShort.\n", cfg))
	if got != "header-set,header-length,header-set" && got != "header-set,header-set,header-length" {
		t.Fatalf("headers: %q", got)
	}
	var b strings.Builder
	b.WriteString("# T\n\n")
	for i := 0; i < 8; i++ {
		b.WriteString("## Why\n\nShort.\n\n")
	}
	if got := LintDocument("issue", b.String(), cfg); !strings.Contains(rules(got), "header-count") {
		t.Fatalf("count: %+v", got)
	}
}

func TestLintSentencesParagraphsAndFiller(t *testing.T) {
	cfg := DefaultText()
	long := strings.TrimSpace(strings.Repeat("word ", 25)) + "."
	if got := rules(LintDocument("issue", "# T\n\n## Why\n\n"+long+"\n", cfg)); got != "sentence-length" {
		t.Fatalf("sentence: %q", got)
	}
	para := "# T\n\n## Why\n\nOne.\nTwo.\nThree.\nFour.\nFive.\n"
	if got := rules(LintDocument("issue", para, cfg)); got != "paragraph-length" {
		t.Fatalf("paragraph: %q", got)
	}
	f := LintDocument("issue", "# T\n\n## Why\n\nWe basically fix it.\n", cfg)
	if rules(f) != "filler" || !strings.Contains(f[0].Message, "basically") {
		t.Fatalf("filler: %+v", f)
	}
	if got := LintDocument("issue", "# T\n\n## Why\n\n- One.\n- Two.\n- Three.\n- Four.\n- Five.\n", cfg); len(got) != 0 {
		t.Fatalf("bullets are not a paragraph: %+v", got)
	}
}

func TestLintWordBudgetExcludesLinksCodeAndDetails(t *testing.T) {
	cfg := DefaultText()
	cfg.PRWords = 10
	body := "## Summary\n\n" + strings.Repeat("word ", 8) + "[a long link text here](https://example.com/x) https://example.com/y\n\n" +
		"```\n" + strings.Repeat("code ", 50) + "\n```\n\n<details>\n" + strings.Repeat("hidden ", 50) + "\n</details>\n"
	if got := LintDocument("pr", body, cfg); len(got) != 0 {
		t.Fatalf("links, code and details must not count: %+v", got)
	}
	body = "## Summary\n\n" + strings.Repeat("word ", 12) + "\n"
	f := LintDocument("pr", body, cfg)
	if rules(f) != "length-budget" || !strings.Contains(f[0].Message, "12") || !strings.Contains(f[0].Message, "10") {
		t.Fatalf("budget: %+v", f)
	}
}

func TestLintFindingsCarryTheLine(t *testing.T) {
	f := LintDocument("issue", "# T\n\n## Why\n\nWe basically fix it.\n", DefaultText())
	if len(f) != 1 || f[0].Line != 5 {
		t.Fatalf("line: %+v", f)
	}
}

func TestLintNeverChangesItsInput(t *testing.T) {
	doc := "# T\n\n## Background\n\nWe basically fix it.\n"
	before := doc
	_ = LintDocument("issue", doc, DefaultText())
	if doc != before {
		t.Fatal("lint must not modify text")
	}
}

func TestLintOneLineDetailsDoesNotHideTheRest(t *testing.T) {
	doc := "# T\n\n<details>short</details>\n\n## Why\n\nWe basically fix it.\n"
	if got := rules(LintDocument("issue", doc, DefaultText())); got != "filler" {
		t.Fatalf("text after a one-line details block must still be linted: %q", got)
	}
}

func TestLintOneLineFenceAndInlineComment(t *testing.T) {
	doc := "# T\n\n```go x := 1```\n\nWe basically fix it. <!-- a note -->\n"
	if got := rules(LintDocument("issue", doc, DefaultText())); got != "filler" {
		t.Fatalf("got %q", got)
	}
	doc = "# T\n\nBefore <!-- start\nof a long comment -->\nWe basically fix it.\n"
	if got := rules(LintDocument("issue", doc, DefaultText())); got != "filler" {
		t.Fatalf("a comment that starts mid-line: %q", got)
	}
}

func TestLintListItemContinuationBelongsToTheItem(t *testing.T) {
	long := strings.Repeat("word ", 12)
	doc := "# T\n\n## Why\n\n- " + long + "\n  " + long + "end.\n"
	if got := rules(LintDocument("issue", doc, DefaultText())); got != "sentence-length" {
		t.Fatalf("a wrapped bullet is one sentence: %q", got)
	}
	doc = "# T\n\n## Why\n\n- One.\n  Two.\n  Three.\n  Four.\n  Five.\n"
	if got := rules(LintDocument("issue", doc, DefaultText())); got != "paragraph-length" {
		t.Fatalf("a list item longer than the paragraph limit: %q", got)
	}
}

func TestLintTitleKnowsProductNames(t *testing.T) {
	cfg := DefaultText()
	for _, ok := range []string{"Add Next.js support", "Migrate to Node.js 22", "Chart.js axes", "Vue.js build"} {
		if got := LintTitle("issue", ok, cfg); len(got) != 0 {
			t.Errorf("%q: %+v", ok, got)
		}
	}
	for _, bad := range []string{"Fix pull.go", "Fix src/app.js", "Update README.md"} {
		if rules(LintTitle("issue", bad, cfg)) != "no-file-names" {
			t.Errorf("%q should be flagged", bad)
		}
	}
}

func TestLintHeadingKeepsHashesThatAreText(t *testing.T) {
	cfg := DefaultText()
	cfg.Headers = append(cfg.Headers, "C#", "Fix #12")
	if got := LintDocument("issue", "# T\n\n## C#\n\nShort.\n\n## Fix #12\n\nShort.\n\n## Why ##\n\nShort.\n", cfg); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}
