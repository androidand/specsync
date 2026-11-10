package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func reviewRepo(t *testing.T, proposal string) (repo string, env []string) {
	t.Helper()
	repo = t.TempDir()
	env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		p := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	write("README.md", "r\n")
	write("openspec/changes/my-change/proposal.md", proposal)
	write("openspec/changes/my-change/tasks.md", "# Tasks\n\n- [ ] 1.1 Parse\n- [ ] 2.1 Render\n")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("checkout", "-q", "-b", "feat/my-change")
	write("src/auth/login.go", "1\n2\n")
	git("add", ".")
	git("commit", "-q", "-m", "feat: login (task 1.1)")
	write("src/render.go", "1\n")
	write("src/render_test.go", "1\n1\n")
	git("add", ".")
	git("commit", "-q", "-m", "feat: render")
	return repo, env
}

func TestReviewMapCommandMarkdownAndJSONShareOrdering(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := reviewRepo(t, "# My change\n\n## Review focus\n\n- `src/auth/**` — token handling\n")
	out, err := runIn(t, bin, repo, env, "review-map", "main..feat/my-change")
	if err != nil {
		t.Fatalf("review-map: %v\n%s", err, out)
	}
	for _, want := range []string{"Change: my-change", "- [ ] `src/auth/login.go` — token handling", "Task 1.1: Parse", "Task: unlinked", "| test | 1 | 2 | 0 |"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Index(out, "Needs human judgement") > strings.Index(out, "login (task 1.1)") {
		t.Fatalf("judgement must come first:\n%s", out)
	}
	js, err := runIn(t, bin, repo, env, "review-map", "-json", "main..feat/my-change")
	if err != nil {
		t.Fatalf("json: %v\n%s", err, js)
	}
	var m struct {
		Change  string `json:"change"`
		Commits []struct {
			Subject string `json:"subject"`
			Task    string `json:"task"`
		} `json:"commits"`
		Judgement []struct{ Path string } `json:"needsJudgement"`
	}
	if err := json.Unmarshal([]byte(js), &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, js)
	}
	if m.Change != "my-change" || len(m.Commits) != 2 || m.Commits[0].Task != "1.1" || m.Commits[1].Task != "" || len(m.Judgement) != 1 {
		t.Fatalf("json = %+v", m)
	}
}

func TestReviewMapCommandWithoutFocusMarksNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := reviewRepo(t, "# My change\n\n## Why\nx\n")
	out, err := runIn(t, bin, repo, env, "review-map", "-change", "my-change", "main..feat/my-change")
	if err != nil || !strings.Contains(out, "Nothing declared") || strings.Contains(out, "needs judgement") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestReviewMapEquivalentExitCodes(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := reviewRepo(t, "# My change\n")
	out, err := runIn(t, bin, repo, env, "review-map", "-equivalent", "feat/my-change", "feat/my-change")
	if err != nil || !strings.Contains(out, "identical") || !strings.Contains(out, "not history") {
		t.Fatalf("identical: err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "review-map", "-equivalent", "main", "feat/my-change")
	if err == nil || !strings.Contains(out, "- src/auth/login.go") || !strings.Contains(out, "- src/render.go") || !strings.Contains(out, "not history") {
		t.Fatalf("differs must exit non-zero and list paths: err=%v\n%s", err, out)
	}
}

func TestResolveReviewRange(t *testing.T) {
	stub := func(_ context.Context, _, ref string) (string, string, error) {
		return "origin/main", "origin/feat/" + ref, nil
	}
	ctx := context.Background()
	for _, c := range []struct{ target, base, wantBase, wantHead string }{
		{"123", "", "origin/main", "origin/feat/123"},
		{"https://github.com/acme/app/pull/9", "", "origin/main", "origin/feat/https://github.com/acme/app/pull/9"},
		{"a..b", "", "a", "b"},
		{"a...b", "", "a", "b"},
		{"feat/x", "develop", "develop", "feat/x"},
		{"", "develop", "develop", "HEAD"},
	} {
		base, head, err := resolveReviewRange(ctx, t.TempDir(), c.target, c.base, stub)
		if err != nil || base != c.wantBase || head != c.wantHead {
			t.Errorf("%q: got %q %q %v", c.target, base, head, err)
		}
	}
	if _, _, err := resolveReviewRange(ctx, t.TempDir(), "feat/x", "", stub); err == nil {
		t.Fatal("no origin/HEAD and no -base must be an error naming -base")
	}
}
