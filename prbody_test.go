package specsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const demoProposal = `# Show where work stands

## Why

Work is spread over many branches. Nobody can see it all at once.

## What

- ` + "`status`" + ` lists each change with its branch.
- ` + "`overview`" + ` lists all work in a workspace.
- Flags mark work that needs attention.

## Review focus

- The attention rules in one table.

## Out of scope

- No rewriting.
`

func demoChange() Change {
	return Change{Slug: "demo", Title: "Show where work stands", Body: demoProposal, TasksMarkdown: "- [x] a\n- [x] b\n- [ ] c\n"}
}

func TestRenderPRBodyDefaultTemplate(t *testing.T) {
	out := RenderPRBody(PRBodyInput{Change: demoChange(), Reference: "Part of #12", Size: "3 files, +40 -2, 2 commits", Text: DefaultText()})
	want := []string{
		"## Summary\n\nWork is spread over many branches.\n",
		"## Changes\n\n- `status` lists each change with its branch.\n",
		"## Review focus\n\n- The attention rules in one table.\n",
		"## Size\n\n3 files, +40 -2, 2 commits\n",
		"## Verification\n\n2 of 3 tasks done.\n",
		"## Closes\n\nPart of #12\n",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
	if strings.Contains(out, "Out of scope") || strings.Contains(out, "Nobody can see") {
		t.Errorf("only the first Why sentence and the listed sections belong:\n%s", out)
	}
	if got := LintDocument("pr", out, DefaultText()); len(got) != 0 {
		t.Errorf("the generated body must pass lint: %+v", got)
	}
}

func TestRenderPRBodyLeavesOutEmptySections(t *testing.T) {
	c := demoChange()
	c.Body = "# T\n\n## Why\n\nOnly a reason.\n"
	c.TasksMarkdown = ""
	out := RenderPRBody(PRBodyInput{Change: c, Reference: "Closes #3", Text: DefaultText()})
	for _, gone := range []string{"## Changes", "## Review focus", "## Size", "## Verification"} {
		if strings.Contains(out, gone) {
			t.Errorf("%s has nothing to say and must be left out:\n%s", gone, out)
		}
	}
}

func TestRenderPRBodyKeepsSectionsWithinBudget(t *testing.T) {
	c := demoChange()
	var b strings.Builder
	b.WriteString("# T\n\n## Why\n\n" + strings.Repeat("word ", 60) + "end.\n\n## What\n\n")
	for i := 0; i < 20; i++ {
		b.WriteString("- " + strings.Repeat("item ", 8) + "\n")
	}
	c.Body = b.String()
	out := RenderPRBody(PRBodyInput{Change: c, Reference: "Part of #1", Text: DefaultText()})
	if n := sectionWords(out, "Summary"); n > 30 {
		t.Errorf("summary has %d words", n)
	}
	if n := sectionWords(out, "Changes"); n > 80 {
		t.Errorf("changes has %d words", n)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("a cut summary says so:\n%s", out)
	}
}

func sectionWords(body, name string) int {
	_, rest, ok := strings.Cut(body, "## "+name+"\n")
	if !ok {
		return 0
	}
	if i := strings.Index(rest, "\n## "); i >= 0 {
		rest = rest[:i]
	}
	n := 0
	for _, w := range strings.Fields(rest) {
		if w != "-" {
			n++
		}
	}
	return n
}

func TestRenderPRBodyUsesTheRepoTemplateSections(t *testing.T) {
	tpl := "## Description\n\n<!-- what and why -->\n\n## Testing\n\n- [ ] ran the tests\n\n## Checklist\n\n- [ ] docs updated\n\n## Closes\n"
	out := RenderPRBody(PRBodyInput{Change: demoChange(), Template: tpl, Reference: "Part of #12", Text: DefaultText()})
	for _, w := range []string{"## Description\n\nWork is spread over many branches.\n", "## Testing\n\n2 of 3 tasks done.\n", "## Checklist\n\n- [ ] docs updated\n", "## Closes\n\nPart of #12\n"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
	if strings.Contains(out, "what and why") {
		t.Errorf("a filled section replaces the template's placeholder:\n%s", out)
	}
}

func TestRenderPRBodyTemplateWithoutKnownSectionsGetsTheReference(t *testing.T) {
	out := RenderPRBody(PRBodyInput{Change: demoChange(), Template: "Describe your change.\n", Reference: "Closes #4", Text: DefaultText()})
	if !strings.HasPrefix(out, "Describe your change.\n") || !strings.HasSuffix(strings.TrimSpace(out), "Closes #4") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestFindPRTemplate(t *testing.T) {
	root := t.TempDir()
	if p, _, _ := FindPRTemplate(root, DefaultText()); p != "" {
		t.Fatalf("no template yet: %q", p)
	}
	gh := filepath.Join(root, ".github")
	_ = os.MkdirAll(gh, 0o755)
	_ = os.WriteFile(filepath.Join(gh, "pull_request_template.md"), []byte("## Why\n"), 0o644)
	if p, body, _ := FindPRTemplate(root, DefaultText()); p != ".github/pull_request_template.md" || body != "## Why\n" {
		t.Fatalf("got %q %q", p, body)
	}
	cfg := DefaultText()
	cfg.Template = "docs/pr.md"
	_ = os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "docs", "pr.md"), []byte("## What\n"), 0o644)
	if p, _, _ := FindPRTemplate(root, cfg); p != "docs/pr.md" {
		t.Fatalf("the configured path wins: %q", p)
	}
}

func TestRenderPRBodyAddsTheReferenceWhenTheTemplateHasNoClosesSection(t *testing.T) {
	out := RenderPRBody(PRBodyInput{Change: demoChange(), Template: "## Description\n\nx\n\n## Checklist\n\n- [ ] docs\n", Reference: "Part of #12", Text: DefaultText()})
	if !strings.HasSuffix(strings.TrimSpace(out), "Part of #12") || !strings.Contains(out, "- [ ] docs") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestRenderPRBodyReadsWhatChangesAndSubheadings(t *testing.T) {
	c := demoChange()
	c.Body = "# T\n\n## Why\n\nReason.\n\n## What Changes\n\n- First change.\n\n### Detail\n\n- Second change.\n\n## Impact\n\n- Not this.\n"
	out := RenderPRBody(PRBodyInput{Change: c, Reference: "Part of #1", Text: DefaultText()})
	if !strings.Contains(out, "- First change.") || !strings.Contains(out, "- Second change.") || strings.Contains(out, "Not this") {
		t.Fatalf("got:\n%s", out)
	}
	c.Body = "# T\n\n## Why\n\nReason.\n\n## Changes\n\n- Plain.\n"
	if out := RenderPRBody(PRBodyInput{Change: c, Reference: "Part of #1", Text: DefaultText()}); !strings.Contains(out, "- Plain.") {
		t.Fatalf("## Changes also works:\n%s", out)
	}
}

func TestRenderPRBodyTemplateIgnoresHeadingsInCodeAndComments(t *testing.T) {
	tpl := "## Notes\n\n```\n## Summary\nnot a section\n```\n\n<!--\n## Testing\n-->\n\n## Summary\n\nplaceholder\n\n## Description\n\nold\n"
	out := RenderPRBody(PRBodyInput{Change: demoChange(), Template: tpl, Reference: "Part of #1", Text: DefaultText()})
	if !strings.Contains(out, "```\n## Summary\nnot a section\n```") || !strings.Contains(out, "<!--\n## Testing\n-->") {
		t.Fatalf("code and comment content must survive:\n%s", out)
	}
	if strings.Count(out, "Work is spread over many branches.") != 1 || strings.Contains(out, "placeholder") {
		t.Fatalf("a kind is filled once, by its first heading:\n%s", out)
	}
	if !strings.Contains(out, "## Description\n\nold") {
		t.Fatalf("the second heading of the same kind keeps its template text:\n%s", out)
	}
}

func TestFindPRTemplateStaysInsideTheRepo(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	_ = os.WriteFile(filepath.Join(outside, "t.md"), []byte("## Why\n"), 0o644)
	_ = os.Symlink(outside, filepath.Join(root, "link"))
	for _, tpl := range []string{filepath.Join(outside, "t.md"), "../" + filepath.Base(outside) + "/t.md", "link/t.md"} {
		cfg := DefaultText()
		cfg.Template = tpl
		if _, _, err := FindPRTemplate(root, cfg); err == nil || !strings.Contains(err.Error(), "inside the repository") {
			t.Errorf("%q must be rejected, got %v", tpl, err)
		}
	}
}
