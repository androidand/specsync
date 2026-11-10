package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/androidand/specsync"
)

type refsProvider struct {
	specsync.WorkProvider
	states map[string]specsync.IssueState
}

func (p refsProvider) IssueStates(context.Context, []specsync.Ref) (map[string]specsync.IssueState, error) {
	return p.states, nil
}

type noLookupProvider struct{ specsync.WorkProvider }

func refsRepo(t *testing.T) (repo, kept, orphan string) {
	t.Helper()
	repo = t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	write := func(name, body string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("f", "1")
	git("add", "f")
	git("commit", "-q", "-m", "kept work")
	kept = git("rev-parse", "HEAD")
	git("checkout", "-q", "-b", "gone")
	write("g", "2")
	git("add", "g")
	git("commit", "-q", "-m", "discarded work")
	orphan = git("rev-parse", "HEAD")
	git("checkout", "-q", "main")
	git("branch", "-q", "-D", "gone")
	write("openspec/changes/my-change/proposal.md", "# My change\n\nFixed in `"+kept[:9]+"`, later `"+orphan[:9]+"`, and commit 1234567abcd.\nSee #12 and #99 and acme/lib#4.\n")
	write("openspec/changes/my-change/tasks.md", "- [ ] 1.1 x\n")
	return repo, kept, orphan
}

func runRefs(t *testing.T, repo string, p specsync.WorkProvider, paths ...string) (string, int, error) {
	t.Helper()
	var buf bytes.Buffer
	n, err := verifyRefs(context.Background(), &buf, verifyRefsOptions{
		OpenSpecDir: filepath.Join(repo, "openspec"), RepoDir: repo, Paths: paths, Provider: p, DefaultKey: "github:acme/app",
	})
	return buf.String(), n, err
}

func TestVerifyRefsReportsStaleOrphanedAndRequestFacts(t *testing.T) {
	repo, _, orphan := refsRepo(t)
	p := refsProvider{states: map[string]specsync.IssueState{
		"github:acme/app#12": {State: "open"},
		"github:acme/lib#4":  {State: "closed", Reason: "completed"},
	}}
	out, n, err := runRefs(t, repo, p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"proposal.md:3  stale  1234567abcd",
		"proposal.md:3  orphaned  " + orphan[:9] + "  \"discarded work\"",
		"proposal.md:4  #99  missing",
		"proposal.md:4  acme/lib#4  closed (completed)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "#12") {
		t.Errorf("an open request is not a finding:\n%s", out)
	}
	if n != 2 {
		t.Errorf("stale and orphaned count = %d, want 2", n)
	}
}

func TestVerifyRefsHashChecksRunWithoutAProviderLookup(t *testing.T) {
	repo, _, _ := refsRepo(t)
	out, n, err := runRefs(t, repo, noLookupProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "stale  1234567abcd") || n != 2 {
		t.Fatalf("hash checks must still run, n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "request checks skipped") {
		t.Fatalf("a status line must say request checks were skipped:\n%s", out)
	}
}

func TestVerifyRefsScansGivenPathsAndNeverModifiesOrSuggests(t *testing.T) {
	repo, kept, _ := refsRepo(t)
	doc := filepath.Join(repo, "notes.md")
	body := "Rewritten, was `deadbeef1` and kept `" + kept[:9] + "`.\n"
	if err := os.WriteFile(doc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(repo, "openspec/changes/my-change/proposal.md"))
	out, _, err := runRefs(t, repo, noLookupProvider{}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "notes.md:1  stale  deadbeef1") {
		t.Fatalf("got:\n%s", out)
	}
	for _, banned := range []string{"did you mean", "replace", "successor", "similar"} {
		if strings.Contains(strings.ToLower(out), banned) {
			t.Fatalf("a replacement must never be suggested: %q in\n%s", banned, out)
		}
	}
	if after, _ := os.ReadFile(doc); string(after) != body {
		t.Fatal("a document was modified")
	}
	if after, _ := os.ReadFile(filepath.Join(repo, "openspec/changes/my-change/proposal.md")); string(after) != string(before) {
		t.Fatal("a change document was modified")
	}
}

func TestVerifyRefsIgnoresArchivedChanges(t *testing.T) {
	repo, _, _ := refsRepo(t)
	arch := filepath.Join(repo, "openspec/changes/archive/2026-01-01-old")
	if err := os.MkdirAll(arch, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(arch, "proposal.md"), []byte("was `abcdef1234`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, err := runRefs(t, repo, noLookupProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "abcdef1234") {
		t.Fatalf("archived changes are history and must not be scanned:\n%s", out)
	}
}

func TestVerifyRefsExitStatusIsAdvisoryUnlessFailOnStale(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, _, _ := refsRepo(t)
	stub := t.TempDir()
	if err := os.WriteFile(filepath.Join(stub, "gh"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "PATH="+stub+string(os.PathListSeparator)+os.Getenv("PATH"), "HOME="+t.TempDir())

	out, err := runIn(t, bin, repo, env, "verify", "-refs")
	if err != nil || !strings.Contains(out, "stale  1234567abcd") {
		t.Fatalf("advisory run must exit 0 and report, got err=%v:\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "verify", "-refs", "-fail-on-stale")
	if err == nil || !strings.Contains(out, "stale  1234567abcd") {
		t.Fatalf("-fail-on-stale must exit non-zero, got err=%v:\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "verify", "-fail-on-stale")
	if err == nil || !strings.Contains(out, "needs -refs") {
		t.Fatalf("-fail-on-stale alone must be rejected, got err=%v:\n%s", err, out)
	}
}

func TestVerifyRefsAMissingPathIsAnError(t *testing.T) {
	repo, _, _ := refsRepo(t)
	if _, _, err := runRefs(t, repo, noLookupProvider{}, filepath.Join(repo, "nope.md")); err == nil {
		t.Fatal("a path that does not exist must be an error, not an empty scan")
	}
}

func TestVerifyRefsScansAFileOnceEvenWhenGivenInsideAChangeFolder(t *testing.T) {
	repo, _, _ := refsRepo(t)
	doc := filepath.Join(repo, "openspec/changes/my-change/proposal.md")
	out, _, err := runRefs(t, repo, noLookupProvider{}, doc)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "proposal.md:3  stale  1234567abcd"); n != 1 {
		t.Fatalf("stale hash reported %d times:\n%s", n, out)
	}
}

func TestVerifyRefsDoesNotClaimAllClearWhenRequestChecksWereSkipped(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "openspec/changes/c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "openspec/changes/c/proposal.md"), []byte("# C\n\nSee #5.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	out, _, err := runRefs(t, repo, noLookupProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "every commit and request reference resolves") {
		t.Fatalf("must not claim all clear when checks were skipped:\n%s", out)
	}
}
