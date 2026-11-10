package specsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestScanDocRefsPrecision(t *testing.T) {
	text := strings.Join([]string{
		"A defaced wall and a decade of effaced facades.",
		"Fixed in `a1b2c3d` and commit 0123abcd4567.",
		"The word `deadbeef` has no digit, and 1234567 in prose is a number.",
		"Cached as `abcdef0123456789abcdef0123456789abcdef01`.",
		"```",
		"git show 9999999aaaa",
		"```",
		"See #12, owner/repo#45 and https://github.com/acme/app/pull/7#issuecomment-99.",
		"Not a ref: abc#34, &#38; and a heading # 5.",
	}, "\n")
	var got []string
	for _, r := range ScanDocRefs("doc.md", text) {
		got = append(got, r.Kind+":"+r.Text+":"+strconv.Itoa(r.Line))
	}
	want := []string{
		"commit:a1b2c3d:2", "commit:0123abcd4567:2",
		"commit:abcdef0123456789abcdef0123456789abcdef01:4",
		"request:#12:8", "request:owner/repo#45:8",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestScanDocRefsRequestParts(t *testing.T) {
	refs := ScanDocRefs("d.md", "Needs owner/repo#45 and #12.")
	if len(refs) != 2 || refs[0].Repo != "owner/repo" || refs[0].Number != 45 || refs[1].Repo != "" || refs[1].Number != 12 {
		t.Fatalf("got %+v", refs)
	}
}

func docRefRepo(t *testing.T) (dir, kept, orphan string) {
	t.Helper()
	dir = t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a")
	run("commit", "-q", "-m", "first change")
	kept = run("rev-parse", "HEAD")
	run("checkout", "-q", "-b", "scratch")
	if err := os.WriteFile(filepath.Join(dir, "b"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "b")
	run("commit", "-q", "-m", "discarded work")
	orphan = run("rev-parse", "HEAD")
	run("checkout", "-q", "main")
	run("branch", "-q", "-D", "scratch")
	return dir, kept, orphan
}

func TestCheckCommitsClassifiesResolvedStaleAndOrphaned(t *testing.T) {
	dir, kept, orphan := docRefRepo(t)
	refs := []DocRef{
		{Kind: "commit", Text: kept[:9], File: "a.md", Line: 1},
		{Kind: "commit", Text: "1234567abcd", File: "a.md", Line: 2},
		{Kind: "commit", Text: orphan[:9], File: "a.md", Line: 3},
	}
	got := CheckCommits(context.Background(), dir, refs, []string{"HEAD"})
	if got[0].Status != "ok" || got[1].Status != "stale" || got[2].Status != "orphaned" {
		t.Fatalf("got %+v", got)
	}
	if got[2].Subject != "discarded work" {
		t.Fatalf("orphaned must show the subject, got %q", got[2].Subject)
	}
	if got[1].Subject != "" {
		t.Fatalf("a stale hash has no subject, got %q", got[1].Subject)
	}
}

func TestDocRefReachableRefsIncludesChangeBranch(t *testing.T) {
	dir, _, _ := docRefRepo(t)
	if out, err := exec.Command("git", "-C", dir, "branch", "feat/12-my-change").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	refs := DocRefReachableRefs(context.Background(), dir, "my-change")
	joined := strings.Join(refs, " ")
	for _, want := range []string{"HEAD", "main", "feat/12-my-change"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, refs)
		}
	}
}

type stateProvider struct {
	WorkProvider
	states map[string]IssueState
	err    error
	asked  []Ref
}

func (s *stateProvider) IssueStates(_ context.Context, refs []Ref) (map[string]IssueState, error) {
	s.asked = append(s.asked, refs...)
	return s.states, s.err
}

func TestCheckRequestsResolvesBareAndQualifiedRefs(t *testing.T) {
	p := &stateProvider{states: map[string]IssueState{
		"github:acme/app#12":  {State: "open"},
		"github:other/lib#45": {State: "closed", Reason: "completed"},
	}}
	refs := []DocRef{
		{Kind: "request", Number: 12, Text: "#12"},
		{Kind: "request", Repo: "other/lib", Number: 45, Text: "other/lib#45"},
		{Kind: "request", Number: 99, Text: "#99"},
	}
	got, status := CheckRequests(context.Background(), p, "github:acme/app", refs)
	if status != "" {
		t.Fatalf("status = %q", status)
	}
	if got[0].State != "open" || got[1].State != "closed" || got[1].Reason != "completed" || got[2].State != "missing" {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckRequestsSkipsWhenTheProviderCannotAnswer(t *testing.T) {
	refs := []DocRef{{Kind: "request", Number: 1, Text: "#1"}}
	if got, status := CheckRequests(context.Background(), noStateProvider{}, "github:acme/app", refs); got != nil || !strings.Contains(status, "skipped") {
		t.Fatalf("got %v, %q", got, status)
	}
	p := &stateProvider{err: context.DeadlineExceeded}
	if got, status := CheckRequests(context.Background(), p, "github:acme/app", refs); got != nil || !strings.Contains(status, "skipped") {
		t.Fatalf("got %v, %q", got, status)
	}
}

type noStateProvider struct{ WorkProvider }

func TestScanDocRefsIgnoresAnchorsFilesAndNonCommitWords(t *testing.T) {
	text := strings.Join([]string{
		"Jump to [output](#2-output) or (#3b) or docs/guide.md#4 or path/file.md#5 or acme/app#6.",
		"Digits only `1234567` and `12345678901` are numbers; sha deadbeef has no digit.",
		"Landed in sha abc1234 and fixed in 0a1b2c3d, and in 99ab123 too.",
		"~~~",
		"```",
		"commit 5a5a5a5a5a",
		"~~~",
		"commit aaaa1111bbbb",
	}, "\n")
	var got []string
	for _, r := range ScanDocRefs("d.md", text) {
		got = append(got, r.Text)
	}
	want := []string{"acme/app#6", "abc1234", "0a1b2c3d", "aaaa1111bbbb"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestCheckCommitsIgnoresNonCommitObjectsAndKeepsStaleCodeSpans(t *testing.T) {
	dir, _, _ := docRefRepo(t)
	blob, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD:a").Output()
	if err != nil {
		t.Fatal(err)
	}
	b := strings.TrimSpace(string(blob))[:9]
	got := CheckCommits(context.Background(), dir, []DocRef{
		{Kind: "commit", Text: b, File: "a.md", Line: 1},
		{Kind: "commit", Text: "7654321fed", File: "a.md", Line: 2},
	}, []string{"HEAD"})
	if len(got) != 1 || got[0].Status != "stale" || got[0].Ref.Text != "7654321fed" {
		t.Fatalf("a blob is not a stale commit and a missing hash still is: %+v", got)
	}
}

func TestCheckCommitsTreatsACommitOnAnyBranchAsReachable(t *testing.T) {
	dir, _, _ := docRefRepo(t)
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("checkout", "-q", "-b", "other")
	if err := os.WriteFile(filepath.Join(dir, "c"), []byte("3"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "c")
	run("commit", "-q", "-m", "other branch work")
	sha := run("rev-parse", "HEAD")
	run("checkout", "-q", "main")
	got := CheckCommits(context.Background(), dir, []DocRef{{Kind: "commit", Text: sha[:9]}}, []string{"HEAD"})
	if got[0].Status != "ok" {
		t.Fatalf("a commit on another branch is not orphaned: %+v", got)
	}
}

func TestCheckRequestsDoesNotReportUnqueryableRefsAsMissing(t *testing.T) {
	p := &stateProvider{states: map[string]IssueState{"github:acme/app#3": {State: "unknown"}}}
	refs := []DocRef{
		{Kind: "request", Number: 0, Text: "#0"},
		{Kind: "request", Repo: "bad repo", Number: 2, Text: "bad repo#2"},
		{Kind: "request", Number: 3, Text: "#3"},
	}
	got, status := CheckRequests(context.Background(), p, "github:acme/app", refs)
	if len(got) != 1 || got[0].Ref.Number != 3 || got[0].State != "missing" {
		t.Fatalf("only the queried, not-found ref is missing: %+v", got)
	}
	if !strings.Contains(status, "2 reference") || len(p.asked) != 1 {
		t.Fatalf("unqueryable refs must be skipped, not sent: status=%q asked=%v", status, p.asked)
	}
}

func TestCheckRequestsWithAnUnresolvedRepoNeverSendsBareRefs(t *testing.T) {
	p := &stateProvider{states: map[string]IssueState{"github:acme/lib#4": {State: "open"}}}
	refs := []DocRef{{Kind: "request", Number: 12, Text: "#12"}, {Kind: "request", Repo: "acme/lib", Number: 4, Text: "acme/lib#4"}}
	got, status := CheckRequests(context.Background(), p, "github", refs)
	if len(p.asked) != 1 || p.asked[0].Provider != "github:acme/lib" {
		t.Fatalf("a bare #n must not be sent when the repo is unknown: %v", p.asked)
	}
	if len(got) != 1 || !strings.Contains(status, "skipped") {
		t.Fatalf("got %+v, %q", got, status)
	}
}
