package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const resumeSlug = "resume-demo"

func resumeRepo(t *testing.T) (repo string, env []string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), resumeSlug)
	if err := os.MkdirAll(filepath.Join(repo, "openspec", "changes", resumeSlug), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	cdir := filepath.Join(repo, "openspec", "changes", resumeSlug)
	for name, body := range map[string]string{
		"proposal.md":    "# Resume demo\n\nWhy.\n",
		"tasks.md":       "- [x] one\n- [ ] two\n",
		"discoveries.md": "- first\n- left off at the parser\n",
	} {
		if err := os.WriteFile(filepath.Join(cdir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env = append(os.Environ(), "HOME="+t.TempDir(), "XDG_CACHE_HOME="+t.TempDir(), "SPECSYNC_DEFAULT_STORE=", "SPECSYNC_SESSIONS_CMD=")
	return repo, env
}

func providerFor(t *testing.T, repo string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "provider.sh")
	body := `#!/bin/sh
echo '[{"cwd":"` + filepath.Join(repo, "sub") + `","id":"s-9","name":"api","updatedAt":"2026-10-13T09:00:00Z","resume":"agent --resume s-9"},{"cwd":"/nowhere","id":"lost"}]'
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

type statusJSON struct {
	Changes []struct {
		Slug     string `json:"slug"`
		LastNote string `json:"lastNote"`
		Resume   string `json:"resume"`
		Sessions []struct {
			ID string `json:"id"`
		} `json:"sessions"`
	} `json:"changes"`
	Status []string `json:"status"`
}

func runStatusJSON(t *testing.T, repo string, env []string, extra ...string) statusJSON {
	t.Helper()
	bin := buildSpecsyncBinary(t)
	args := append([]string{"status", "-everything", "-json"}, extra...)
	out, err := runIn(t, bin, repo, env, args...)
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	var got statusJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(got.Changes) != 1 {
		t.Fatalf("rows = %d\n%s", len(got.Changes), out)
	}
	return got
}

func TestStatusShowsLastNoteAndDefaultResumeWithoutAProvider(t *testing.T) {
	repo, env := resumeRepo(t)
	got := runStatusJSON(t, repo, env)
	row := got.Changes[0]
	if row.LastNote != "- left off at the parser" {
		t.Fatalf("lastNote = %q", row.LastNote)
	}
	if row.Resume != "cd "+repo && !strings.HasSuffix(row.Resume, resumeSlug) {
		t.Fatalf("resume = %q", row.Resume)
	}
	if len(row.Sessions) != 0 {
		t.Fatalf("no provider configured, yet sessions = %+v", row.Sessions)
	}
}

func TestStatusJoinsProviderSessionsFromFlagAndEnvironment(t *testing.T) {
	repo, env := resumeRepo(t)
	provider := providerFor(t, repo)

	got := runStatusJSON(t, repo, env, "-sessions-cmd", provider)
	row := got.Changes[0]
	if row.Resume != "agent --resume s-9" || len(row.Sessions) != 1 || row.Sessions[0].ID != "s-9" {
		t.Fatalf("flag: row = %+v", row)
	}

	got = runStatusJSON(t, repo, append(env, "SPECSYNC_SESSIONS_CMD="+provider))
	if got.Changes[0].Resume != "agent --resume s-9" {
		t.Fatalf("environment: row = %+v", got.Changes[0])
	}

	got = runStatusJSON(t, repo, append(env, "SPECSYNC_SESSIONS_CMD="+provider), "-sessions-cmd=")
	if len(got.Changes[0].Sessions) != 0 {
		t.Fatalf("an empty flag must disable the provider for one run: %+v", got.Changes[0])
	}
}

func TestStatusReportsUnmatchedSessionsAndProviderFailureSoftly(t *testing.T) {
	repo, env := resumeRepo(t)
	got := runStatusJSON(t, repo, env, "-sessions-cmd", providerFor(t, repo))
	if strings.Contains(strings.Join(got.Status, "\n"), "matched no work") {
		t.Fatalf("a view of one store cannot say a session matched no work: %v", got.Status)
	}

	got = runStatusJSON(t, repo, env, "-sessions-cmd", "exit 3")
	if !strings.Contains(strings.Join(got.Status, "\n"), "session provider failed") {
		t.Fatalf("status = %v", got.Status)
	}
	if got.Changes[0].LastNote == "" || len(got.Changes[0].Sessions) != 0 {
		t.Fatalf("a failed provider must leave the rest unchanged: %+v", got.Changes[0])
	}

	got = runStatusJSON(t, repo, env, "-sessions-cmd", "sleep 5", "-sessions-timeout", "200ms")
	if !strings.Contains(strings.Join(got.Status, "\n"), "timed out") {
		t.Fatalf("status = %v", got.Status)
	}
}

func TestStatusIgnoresAProviderNamedInsideTheRepository(t *testing.T) {
	repo, env := resumeRepo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	script := "touch " + marker
	for _, f := range []string{
		filepath.Join(repo, "openspec", "specsync.yml"),
		filepath.Join(repo, ".specsync.yml"),
		filepath.Join(repo, "openspec", "changes", resumeSlug, "specsync.yml"),
	} {
		if err := os.WriteFile(f, []byte("sessions-cmd: "+script+"\nsessions_cmd: "+script+"\nSPECSYNC_SESSIONS_CMD: "+script+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runStatusJSON(t, repo, env)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a file inside the repository must never cause a command to run")
	}
}

func TestStatusWithAProviderWritesNothing(t *testing.T) {
	repo, env := resumeRepo(t)
	cache := ""
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "XDG_CACHE_HOME="); ok {
			cache = v
		}
	}
	before := gitStatus(t, repo)
	runStatusJSON(t, repo, env, "-sessions-cmd", providerFor(t, repo))
	if after := gitStatus(t, repo); after != before {
		t.Fatalf("worktree changed:\n%s\n--\n%s", before, after)
	}
	if es, _ := os.ReadDir(cache); len(es) != 0 {
		t.Fatalf("the cache dir must stay empty with -state=off, found %d entries", len(es))
	}
}

func TestStatusTableShowsTheResumeHintOnItsOwnLine(t *testing.T) {
	repo, env := resumeRepo(t)
	bin := buildSpecsyncBinary(t)
	out, err := runIn(t, bin, repo, env, "status", "-everything", "-sessions-cmd", providerFor(t, repo))
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	if strings.Contains(out, "RESUME") {
		t.Fatalf("the table keeps its old columns:\n%s", out)
	}
	if !strings.Contains(out, "WORKTREE\n") {
		t.Fatalf("the header must still end at WORKTREE:\n%s", out)
	}
	if !strings.Contains(out, "↳ resume: agent --resume s-9") {
		t.Fatalf("table lacks the resume line:\n%s", out)
	}
	if !strings.Contains(out, "note: - left off at the parser") {
		t.Fatalf("table lacks the last note:\n%s", out)
	}
}

func TestOverviewShowsResumeAndNoteFromTheProvider(t *testing.T) {
	repo, env := resumeRepo(t)
	bin := buildSpecsyncBinary(t)
	out, err := runIn(t, bin, repo, env, "overview", "-root", filepath.Dir(repo), "-json", "-sessions-cmd", providerFor(t, repo))
	if err != nil {
		t.Fatalf("overview: %v\n%s", err, out)
	}
	var ov struct {
		Status []string `json:"status"`
		NoEpic []struct {
			Slug     string `json:"slug"`
			Resume   string `json:"resume"`
			LastNote string `json:"lastNote"`
		} `json:"noEpic"`
	}
	if err := json.Unmarshal([]byte(out), &ov); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if !strings.Contains(strings.Join(ov.Status, "\n"), "1 session matched no work") {
		t.Fatalf("an overview covers the workspace, so an unmatched session is reported: %v", ov.Status)
	}
	if len(ov.NoEpic) != 1 || ov.NoEpic[0].Resume != "agent --resume s-9" || ov.NoEpic[0].LastNote == "" {
		t.Fatalf("overview rows = %+v\n%s", ov.NoEpic, out)
	}
}

func TestStatusStateOffDoesNotRunTheProvider(t *testing.T) {
	repo, env := resumeRepo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	provider := filepath.Join(t.TempDir(), "provider.sh")
	if err := os.WriteFile(provider, []byte("#!/bin/sh\ntouch "+marker+"\necho '[]'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := runStatusJSON(t, repo, env, "-state=off", "-sessions-cmd", provider)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("-state=off is offline and must not run the session provider")
	}
	if !strings.Contains(strings.Join(got.Status, "\n"), "skipped with -state=off") {
		t.Fatalf("status = %v", got.Status)
	}
	bin := buildSpecsyncBinary(t)
	if out, err := runIn(t, bin, repo, env, "overview", "-root", filepath.Dir(repo), "-state=off", "-sessions-cmd", provider); err != nil {
		t.Fatalf("overview: %v\n%s", err, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("overview -state=off must not run the provider either")
	}
}
