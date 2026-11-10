package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func lintRepo(t *testing.T, proposal, config string) (dir string, env []string) {
	t.Helper()
	dir, env = firstUseEnv(t)
	if err := os.MkdirAll(filepath.Join(dir, "openspec", "changes", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if proposal != "" {
		if err := os.WriteFile(filepath.Join(dir, "openspec", "changes", "demo", "proposal.md"), []byte(proposal), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if config != "" {
		if err := os.WriteFile(filepath.Join(dir, "openspec", "specsync.yml"), []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, env
}

func TestLintTitleIsAdvisoryUnlessFail(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := lintRepo(t, "", "")
	out, err := runIn(t, bin, dir, env, "lint", "-title", "Add the thing")
	if err != nil || !strings.Contains(out, "conventional-commit") || !strings.Contains(out, "type(scope): subject") {
		t.Fatalf("advisory run: err=%v\n%s", err, out)
	}
	if _, err := runIn(t, bin, dir, env, "lint", "-title", "Add the thing", "-fail"); err == nil {
		t.Fatal("-fail must exit non-zero on a finding")
	}
	if out, err := runIn(t, bin, dir, env, "lint", "-title", "fix(sync): keep links out", "-fail"); err != nil {
		t.Fatalf("a clean title passes -fail: %v\n%s", err, out)
	}
}

func TestLintChangeReportsHeaderProblemsAndNeverEdits(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	proposal := "# Show where work stands\n\n## Why\n\nWe basically need it.\n\n## What this change does\n\n- One.\n\n## Release note\n\nShort.\n"
	dir, env := lintRepo(t, proposal, "")
	path := filepath.Join(dir, "openspec", "changes", "demo", "proposal.md")
	out, err := runIn(t, bin, dir, env, "lint", "-change", "demo")
	if err != nil {
		t.Fatalf("advisory: %v\n%s", err, out)
	}
	for _, want := range []string{"header-set", "header-length", "filler", "proposal.md:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `header "Release note"`) {
		t.Errorf("Release note is a header specsync needs:\n%s", out)
	}
	if _, err := runIn(t, bin, dir, env, "lint", "-change", "demo", "-fail"); err == nil {
		t.Fatal("-fail must exit non-zero")
	}
	if got, _ := os.ReadFile(path); string(got) != proposal {
		t.Fatal("lint must never edit text")
	}
}

func TestLintBodyFileUsesThePRBudget(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := lintRepo(t, "", "text: {pr_words: 10}\n")
	body := filepath.Join(dir, "body.md")
	if err := os.WriteFile(body, []byte("## Summary\n\n"+strings.Repeat("word ", 12)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runIn(t, bin, dir, env, "lint", "-body-file", body)
	if err != nil || !strings.Contains(out, "length-budget") || !strings.Contains(out, "12") {
		t.Fatalf("budget: err=%v\n%s", err, out)
	}
}

func TestLintPlainProfileFromConfig(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := lintRepo(t, "", "text: {profile: plain}\n")
	if out, err := runIn(t, bin, dir, env, "lint", "-title", "Add the thing", "-fail"); err != nil {
		t.Fatalf("plain profile: %v\n%s", err, out)
	}
}

func TestLintJSONAndNeedsAnInput(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := lintRepo(t, "", "")
	out, err := runIn(t, bin, dir, env, "lint", "-title", "Add the thing", "-json")
	if err != nil || !strings.Contains(out, `"rule": "conventional-commit"`) {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if out, err := runIn(t, bin, dir, env, "lint"); err == nil || !strings.Contains(out, "-change") {
		t.Fatalf("lint with nothing to check must say what to pass: %v\n%s", err, out)
	}
}

func TestLintPRReadsTitleAndBodyFromTheProvider(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := lintRepo(t, "", "")
	stub := t.TempDir()
	resp := filepath.Join(stub, "pr.json")
	if err := os.WriteFile(resp, []byte(`{"title":"Add the thing","body":"## Summary\n\nShort."}`), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \"$1 $2\" in\n  \"pr view\") cat " + resp + " ;;\n  *) echo '[]' ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(stub, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	env = append(env, "PATH="+stub+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := runIn(t, bin, dir, env, "lint", "-pr", "9")
	if err != nil || !strings.Contains(out, "conventional-commit") {
		t.Fatalf("pr: %v\n%s", err, out)
	}
}

func prBodyRepo(t *testing.T, template string) (dir string, env []string) {
	t.Helper()
	proposal := "# Show where work stands\n\n## Why\n\nWork is spread over many branches. Nobody sees it all.\n\n## What\n\n- `status` lists each change.\n- `overview` lists all work.\n"
	dir, env = lintRepo(t, proposal, "")
	if err := os.WriteFile(filepath.Join(dir, "openspec", "changes", "demo", "tasks.md"), []byte("- [x] a\n- [ ] b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if template != "" {
		if err := os.MkdirAll(filepath.Join(dir, ".github"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".github", "pull_request_template.md"), []byte(template), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stub := t.TempDir()
	script := "#!/bin/sh\ncase \"$1 $2\" in\n  \"issue list\") echo '[{\"number\":7,\"url\":\"https://github.com/o/r/issues/7\",\"body\":\"<!-- specsync:change=demo -->\"}]' ;;\n  *) echo '[]' ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(stub, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, append(env, "PATH="+stub+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestPRBodyTemplateRendersTheDefaultBody(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := prBodyRepo(t, "")
	out, err := runIn(t, bin, dir, env, "pr-body", "-change", "demo", "-repo", "o/r", "-template")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{"## Summary\n\nWork is spread over many branches.\n", "## Changes\n\n- `status` lists each change.\n", "## Verification\n\n1 of 2 tasks done.\n", "## Closes\n\nPart of #7\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "## Size") {
		t.Errorf("no base to measure against, so no Size section:\n%s", out)
	}
	if strings.Contains(out, "Nobody sees it all") {
		t.Errorf("only the first Why sentence:\n%s", out)
	}
}

func TestPRBodyTemplateUsesTheRepoTemplate(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := prBodyRepo(t, "## Description\n\n<!-- why -->\n\n## Checklist\n\n- [ ] docs updated\n")
	out, err := runIn(t, bin, dir, env, "pr-body", "-change", "demo", "-repo", "o/r", "-template")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "## Description\n\nWork is spread over many branches.") || !strings.Contains(out, "- [ ] docs updated") || strings.Contains(out, "<!-- why -->") {
		t.Fatalf("repo template sections:\n%s", out)
	}
	if !strings.Contains(out, "Part of #7") {
		t.Fatalf("the reference must still be present:\n%s", out)
	}
}

func TestPRBodyWithoutTemplateFlagIsUnchanged(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := prBodyRepo(t, "")
	out, err := runIn(t, bin, dir, env, "pr-body", "-change", "demo", "-repo", "o/r")
	if err != nil || strings.TrimSpace(out) != "Part of #7" {
		t.Fatalf("the plain output stays the reference line: %v\n%s", err, out)
	}
	if out, err := runIn(t, bin, dir, env, "pr-body", "-change", "demo", "-repo", "o/r", "-template", "-body-file", "x.md"); err == nil || !strings.Contains(out, "-template") {
		t.Fatalf("-template and -body-file conflict: %v\n%s", err, out)
	}
}

func TestDoctorConfigValidatesTheTextSection(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := lintRepo(t, "", "text:\n  profile: strict\n  pr_title: many\n")
	out, err := runIn(t, bin, dir, env, "doctor", "config")
	if err == nil || !strings.Contains(out, "profile must be conventional or plain") || !strings.Contains(out, "pr_title must be a whole number") {
		t.Fatalf("doctor config must report text problems with lines: err=%v\n%s", err, out)
	}
	dir, env = lintRepo(t, "", "text:\n  profile: plain\n  headers: [Non-goals]\n")
	if out, err := runIn(t, bin, dir, env, "doctor", "config"); err != nil || !strings.Contains(out, "config is valid") {
		t.Fatalf("a valid text section: err=%v\n%s", err, out)
	}
}

func lintExit(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return 0
}

func TestLintSetupErrorsExitTwoAndFindingsExitOne(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := lintRepo(t, "# T\n", "")
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"nothing to check", []string{"lint"}, 2},
		{"empty title", []string{"lint", "-title", ""}, 2},
		{"unknown change", []string{"lint", "-change", "nope"}, 2},
		{"bad kind", []string{"lint", "-title", "x", "-kind", "bug"}, 2},
		{"missing file", []string{"lint", "-body-file", "nope.md"}, 2},
		{"findings without -fail", []string{"lint", "-title", "Add the thing"}, 0},
		{"findings with -fail", []string{"lint", "-title", "Add the thing", "-fail"}, 1},
	}
	for _, c := range cases {
		out, err := runIn(t, bin, dir, env, c.args...)
		if got := lintExit(err); got != c.want {
			t.Errorf("%s: exit %d, want %d\n%s", c.name, got, c.want, out)
		}
	}
}

func TestLintReadsTheStandardFromTheRepoRootNotTheWorkingDirectory(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := lintRepo(t, "", "text: {profile: plain}\n")
	sub := filepath.Join(dir, "pkg", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := runIn(t, bin, sub, env, "lint", "-title", "Add the thing", "-fail"); err != nil {
		t.Fatalf("the plain profile at the repo root applies from a subdirectory: %v\n%s", err, out)
	}
}

func TestPRBodyTemplateRejectsATemplatePathOutsideTheRepo(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir, env := prBodyRepo(t, "")
	if err := os.WriteFile(filepath.Join(dir, "openspec", "specsync.yml"), []byte("text: {template: ../outside.md}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runIn(t, bin, dir, env, "pr-body", "-change", "demo", "-repo", "o/r", "-template")
	if err == nil || !strings.Contains(out, "inside the repository") {
		t.Fatalf("a template outside the repo must be refused: %v\n%s", err, out)
	}
}
