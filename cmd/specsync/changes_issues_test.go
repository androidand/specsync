package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeChange(t *testing.T, root, slug, tasks, refs string) {
	t.Helper()
	dir := filepath.Join(root, "openspec", "changes", slug)
	if err := os.MkdirAll(filepath.Join(dir, ".specsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"proposal.md": "# " + slug + "\n", "tasks.md": tasks}
	if refs != "" {
		files[filepath.Join(".specsync", "refs.json")] = refs
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func ghRef(id string) string {
	return `{"github:acme/app":{"provider":"github:acme/app","id":"` + id + `","url":"https://github.com/acme/app/issues/` + id + `"}}`
}

func issuesEnv(t *testing.T, graphql string) (env []string, calls string) {
	t.Helper()
	bin := t.TempDir()
	calls = filepath.Join(bin, "calls")
	stub := "#!/bin/sh\necho \"$@\" >> " + calls + "\ncat <<'EOF'\n" + graphql + "\nEOF\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	return append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH")), calls
}

func changesRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeChange(t, root, "all-done", "# Tasks\n- [x] a\n- [x] b\n", ghRef("1"))
	writeChange(t, root, "half-done", "# Tasks\n- [x] a\n- [ ] b\n", ghRef("2"))
	writeChange(t, root, "no-ref", "# Tasks\n- [ ] a\n", "")
	return root
}

// Refs are queried in table order: half-done (#2) is i0, all-done (#1) is i1.
const threeIssues = `{"data":{"i0":{"issue":{"state":"CLOSED","stateReason":"NOT_PLANNED"}},"i1":{"issue":{"state":"OPEN","stateReason":null}}}}`

func TestChangesWithoutIssuesFlagMakesNoCallAndKeepsOutput(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	root := changesRepo(t)
	env, calls := issuesEnv(t, threeIssues)
	out, err := runIn(t, bin, root, env, "changes")
	if err != nil {
		t.Fatalf("changes: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(calls); statErr == nil {
		t.Fatal("gh was called without -issues")
	}
	want := `STAGE          PRIORITY  TASKS       SLUG                          PROGRESS        TITLE
─────────────  ────────  ─────────  ────────────────────────────  ──────────────  ────────────────────────────────────────────────────────────
active        -         1/2         half-done                      in-progress     half-done
active        -         0/1         no-ref                         not-started     no-ref

complete      -         2/2         all-done                       complete        all-done
`
	if out != want {
		t.Fatalf("output changed:\n%s", out)
	}
}

func TestChangesIssuesShowsStateDriftAndUncheckedCount(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	root := changesRepo(t)
	env, calls := issuesEnv(t, threeIssues)
	out, err := runIn(t, bin, root, env, "changes", "-issues")
	if err != nil {
		t.Fatalf("changes -issues: %v\n%s", err, out)
	}
	for _, want := range []string{"#1", "open", "complete-open", "#2", "closed (not planned)", "closed-unarchived"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "1 change(s) have no cached issue and were not checked") {
		t.Errorf("no unchecked count:\n%s", out)
	}
	b, _ := os.ReadFile(calls)
	if n := strings.Count(string(b), "\n"); n != 1 {
		t.Errorf("want one gh call for all issues, got %d:\n%s", n, b)
	}
}

func TestChangesIssuesJSONFields(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	root := changesRepo(t)
	env, _ := issuesEnv(t, threeIssues)
	out, err := runIn(t, bin, root, env, "changes", "-issues", "-json")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{`"issue": 1`, `"issueState": "open"`, `"issueStateReason": "not planned"`, `"drift": "closed-unarchived"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json lacks %s:\n%s", want, out)
		}
	}
}

func TestChangesDriftListsOnlyDriftAndExitsNonZero(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	root := changesRepo(t)
	env, _ := issuesEnv(t, `{"data":{"i0":{"issue":{"state":"OPEN","stateReason":null}},"i1":{"issue":{"state":"OPEN","stateReason":null}}}}`)
	out, err := runIn(t, bin, root, env, "changes", "-drift")
	var ee *exec.ExitError
	if err == nil || !errors.As(err, &ee) || ee.ExitCode() != 1 {
		t.Fatalf("want exit 1, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "all-done") || strings.Contains(out, "half-done") {
		t.Errorf("-drift must list only the drifting change:\n%s", out)
	}
}

func TestChangesDriftCleanRepoPrintsNothingAndExitsZero(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	root := changesRepo(t)
	env, _ := issuesEnv(t, `{"data":{"i0":{"issue":{"state":"OPEN","stateReason":null}},"i1":{"issue":{"state":"OPEN","stateReason":null}}}}`)
	writeChange(t, root, "all-done", "# Tasks\n- [x] a\n- [ ] b\n", ghRef("1"))
	out, err := runIn(t, bin, root, env, "changes", "-drift")
	if err != nil || strings.TrimSpace(out) != "1 change(s) have no cached issue and were not checked" {
		t.Fatalf("clean repo: err=%v out=%q", err, out)
	}
}

func TestChangesDriftOnFreshCheckoutChecksNothingAndExitsZero(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	root := t.TempDir()
	writeChange(t, root, "a", "# Tasks\n- [x] a\n", "")
	env, calls := issuesEnv(t, `{}`)
	out, err := runIn(t, bin, root, env, "changes", "-drift")
	if err != nil {
		t.Fatalf("fresh checkout must exit zero: %v\n%s", err, out)
	}
	if !strings.Contains(out, "1 change(s) have no cached issue and were not checked") {
		t.Errorf("must say nothing was checked:\n%s", out)
	}
	if _, statErr := os.Stat(calls); statErr == nil {
		t.Error("no ref means no provider call")
	}
}
