package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func providerScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "provider.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAttachSessions_LongestPrefixAndMostRecentWin(t *testing.T) {
	wt := t.TempDir()
	inner := filepath.Join(wt, "pkg", "x")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	rows := []ChangeTopology{
		{Slug: "a", Worktree: wt},
		{Slug: "b", Worktree: other},
		{Slug: "c"},
	}
	unmatched := AttachSessions(rows, []SessionEntry{
		{CWD: inner, ID: "old", UpdatedAt: "2026-10-13T09:00:00Z", Resume: "agent --resume old"},
		{CWD: wt, ID: "new", UpdatedAt: "2026-10-13T10:00:00Z", Resume: "agent --resume new"},
		{CWD: filepath.Join(t.TempDir(), "nowhere"), ID: "lost"},
	})
	if unmatched != 1 {
		t.Fatalf("unmatched = %d, want 1", unmatched)
	}
	if len(rows[0].Sessions) != 2 || rows[0].Sessions[0].ID != "new" {
		t.Fatalf("sessions = %+v", rows[0].Sessions)
	}
	if rows[0].Resume != "agent --resume new" {
		t.Fatalf("resume = %q", rows[0].Resume)
	}
	if len(rows[1].Sessions) != 0 || rows[1].Resume != "cd "+other {
		t.Fatalf("row b = %+v", rows[1])
	}
	if rows[2].Resume != "" || rows[2].Sessions != nil {
		t.Fatalf("a row without a worktree gets nothing: %+v", rows[2])
	}
}

func TestAttachSessions_DoesNotMatchASiblingFolderWithTheSamePrefix(t *testing.T) {
	base := t.TempDir()
	wt := filepath.Join(base, "work")
	sibling := filepath.Join(base, "work-two")
	for _, d := range []string{wt, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	rows := []ChangeTopology{{Slug: "a", Worktree: wt}}
	if n := AttachSessions(rows, []SessionEntry{{CWD: sibling, ID: "s"}}); n != 1 || len(rows[0].Sessions) != 0 {
		t.Fatalf("a sibling folder must not attach: unmatched=%d rows=%+v", n, rows[0])
	}
}

func TestAttachSessions_NoProviderLeavesNoSessionFields(t *testing.T) {
	wt := t.TempDir()
	rows := []ChangeTopology{{Slug: "a", Worktree: wt}}
	AttachSessions(rows, nil)
	if rows[0].Sessions != nil || rows[0].Resume != "cd "+wt {
		t.Fatalf("row = %+v", rows[0])
	}
}

func TestAttachSessions_SessionWithoutResumeFallsBackToCd(t *testing.T) {
	wt := t.TempDir()
	rows := []ChangeTopology{{Slug: "a", Worktree: wt}}
	AttachSessions(rows, []SessionEntry{{CWD: wt, ID: "s", Name: "api-scope"}})
	if len(rows[0].Sessions) != 1 || rows[0].Resume != "cd "+wt {
		t.Fatalf("row = %+v", rows[0])
	}
}

func TestLoadSessions_ReadsTheContractAndIgnoresUnknownFields(t *testing.T) {
	cmd := providerScript(t, `cat <<'JSON'
[{"cwd":"/w/a","id":"s-1","name":"n","status":"idle","updatedAt":"2026-10-13T09:12:00Z","resume":"agent --resume s-1","extra":{"x":1}},
 {"id":"no-cwd"}]
JSON`)
	got, err := LoadSessions(context.Background(), cmd, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "s-1" || got[0].Resume != "agent --resume s-1" {
		t.Fatalf("entries = %+v (an entry without cwd must be dropped)", got)
	}
}

func TestLoadSessions_FailureModes(t *testing.T) {
	cases := []struct {
		name, cmd, want string
		timeout         time.Duration
	}{
		{"missing command", "definitely-not-a-command-xyz", "exit", 5 * time.Second},
		{"non-zero exit", providerScript(t, "echo '[]'; exit 3"), "exit", 5 * time.Second},
		{"invalid json", providerScript(t, "echo 'not json'"), "JSON", 5 * time.Second},
		{"not an array", providerScript(t, `echo '{"cwd":"/x"}'`), "JSON", 5 * time.Second},
		{"timeout", providerScript(t, "sleep 5; echo '[]'"), "timed out", 150 * time.Millisecond},
		{"oversized output", providerScript(t, "yes '[' | head -c 3000000"), "larger than", 5 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start := time.Now()
			got, err := LoadSessions(context.Background(), c.cmd, c.timeout)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
			if got != nil {
				t.Fatalf("entries = %+v, want none", got)
			}
			if time.Since(start) > 3*time.Second {
				t.Fatalf("took %s; the timeout must bound the run", time.Since(start))
			}
		})
	}
}

func TestLoadSessions_EmptyCommandRunsNothing(t *testing.T) {
	got, err := LoadSessions(context.Background(), "  ", time.Second)
	if err != nil || got != nil {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestLastNote(t *testing.T) {
	dir := t.TempDir()
	if got := LastNote(dir); got != "" {
		t.Fatalf("absent file: %q", got)
	}
	file := filepath.Join(dir, "discoveries.md")
	if err := os.WriteFile(file, []byte("\n  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LastNote(dir); got != "" {
		t.Fatalf("empty file: %q", got)
	}
	if err := os.WriteFile(file, []byte("- first\n- found the edge case  \n\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LastNote(dir); got != "- found the edge case" {
		t.Fatalf("last note = %q", got)
	}
}

func TestSessionsAreNotWrittenAnywhere(t *testing.T) {
	wt := t.TempDir()
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	cmd := providerScript(t, `echo '[{"cwd":"`+wt+`","id":"s"}]'`)
	entries, err := LoadSessions(context.Background(), cmd, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	AttachSessions([]ChangeTopology{{Slug: "a", Worktree: wt}}, entries)
	for _, dir := range []string{wt, cache} {
		if es, _ := os.ReadDir(dir); len(es) != 0 {
			t.Fatalf("%s must stay empty, found %d entries", dir, len(es))
		}
	}
}
