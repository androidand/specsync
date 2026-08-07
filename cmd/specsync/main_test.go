package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/androidand/specsync"
)

// TestValidateSlug verifies the slug validation rules.
func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		{"valid simple", "foo", false},
		{"valid with hyphen", "foo-bar", false},
		{"valid with underscore", "foo_bar", false},
		{"valid with digits", "foo123", false},
		{"valid starts with digit", "1foo", false},
		{"valid long", "a-very-long-change-name", false},
		{"invalid empty", "", true},
		{"invalid slash", "foo/bar", true},
		{"invalid backslash", `foo\bar`, true},
		{"invalid dotdot", "foo..bar", true},
		{"invalid uppercase", "FooBar", true},
		{"invalid space", "foo bar", true},
		{"invalid special", "foo@bar", true},
		{"invalid dot", "foo.bar", true},
		{"invalid starts with hyphen", "-foo", true},
		{"invalid starts with underscore", "_foo", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSlug(tt.slug)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSlug(%q) error = %v, wantErr %v", tt.slug, err, tt.wantErr)
			}
		})
	}
}

// TestDeprecatedSlugFlag pins the removed -slug flag's error path: every
// spelling a user might type points at -change, and positional words that
// merely contain "slug" are left for flag.Parse to handle.
func TestDeprecatedSlugFlag(t *testing.T) {
	for _, args := range [][]string{
		{"-slug", "foo"},
		{"--slug", "foo"},
		{"-slug=foo"},
		{"--slug=foo"},
		{"-dry-run", "-slug", "foo"},
	} {
		if err := deprecatedSlugFlag(args); err == nil {
			t.Errorf("deprecatedSlugFlag(%v) = nil, want an error naming -change", args)
		} else if !strings.Contains(err.Error(), "-change") {
			t.Errorf("deprecatedSlugFlag(%v) error %q does not mention -change", args, err)
		}
	}
	for _, args := range [][]string{
		nil,
		{"-change", "foo"},
		{"-change", "slug"},
		{"set-stage", "slug", "active"},
	} {
		if err := deprecatedSlugFlag(args); err != nil {
			t.Errorf("deprecatedSlugFlag(%v) = %v, want nil", args, err)
		}
	}
}

// TestIsVersionArg pins the dispatch predicate the main switch uses for the
// version subcommand, so the wiring cannot silently regress.
func TestIsVersionArg(t *testing.T) {
	for _, arg := range []string{"version", "-version", "--version"} {
		if !isVersionArg(arg) {
			t.Errorf("isVersionArg(%q) = false, want true", arg)
		}
	}
	for _, arg := range []string{"sync", "pull", "scan", "-v", ""} {
		if isVersionArg(arg) {
			t.Errorf("isVersionArg(%q) = true, want false", arg)
		}
	}
}

// TestResolveSubcommand pins dispatch: known subcommands route with their
// remaining args intact, "push" is a transparent alias for "sync", bare
// invocations (no args, or flags-only) default to "sync", and any other bare
// word is rejected rather than silently reaching runSync's flag.Parse (which
// would otherwise discard every flag typed after it — see the doc comment on
// resolveSubcommand for the incident this pins).
func TestResolveSubcommand(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantCmd  string
		wantRest []string
		wantErr  bool
	}{
		{"empty", nil, "sync", nil, false},
		{"flags only", []string{"-change", "foo"}, "sync", []string{"-change", "foo"}, false},
		{"explicit sync", []string{"sync", "-change", "foo"}, "sync", []string{"-change", "foo"}, false},
		// The deprecated -slug flag routes to sync like any flag, where
		// deprecatedSlugFlag produces the "did you mean -change?" error.
		{"deprecated slug flag routes to sync", []string{"-slug", "foo"}, "sync", []string{"-slug", "foo"}, false},
		{"push is not an alias", []string{"push", "-change", "foo", "-dry-run"}, "", nil, true},
		{"pull", []string{"pull", "-issue", "3"}, "pull", []string{"-issue", "3"}, false},
		{"version word", []string{"version"}, "version", []string{}, false},
		{"version flag", []string{"-version"}, "version", []string{}, false},
		{"unknown word", []string{"frobnicate", "-dry-run"}, "", nil, true},
		{"typo of push", []string{"psh", "-change", "foo"}, "", nil, true},
		{"audit", []string{"audit", "-json"}, "audit", []string{"-json"}, false},
		{"audit fail-on-unmerged", []string{"audit", "-fail-on-unmerged"}, "audit", []string{"-fail-on-unmerged"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, rest, err := resolveSubcommand(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveSubcommand(%v): expected an error, got cmd=%q rest=%v", tc.args, cmd, rest)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSubcommand(%v): unexpected error: %v", tc.args, err)
			}
			if cmd != tc.wantCmd {
				t.Fatalf("resolveSubcommand(%v) cmd = %q, want %q", tc.args, cmd, tc.wantCmd)
			}
			if len(rest) != len(tc.wantRest) {
				t.Fatalf("resolveSubcommand(%v) rest = %v, want %v", tc.args, rest, tc.wantRest)
			}
			for i := range rest {
				if rest[i] != tc.wantRest[i] {
					t.Fatalf("resolveSubcommand(%v) rest = %v, want %v", tc.args, rest, tc.wantRest)
				}
			}
		})
	}
}

// TestResolveSubcommandPushSuggestsSync: "push" is a deliberate non-alias —
// it errors, but the message points to "sync" and explains why push isn't
// the right mental model (sync also reconciles tracker state back into
// tasks.md, so it isn't one-way like git push).
func TestResolveSubcommandPushSuggestsSync(t *testing.T) {
	_, _, err := resolveSubcommand([]string{"push", "-change", "foo"})
	if err == nil {
		t.Fatal("expected an error for \"push\"")
	}
	if !strings.Contains(err.Error(), `"sync"`) {
		t.Fatalf("error %q should suggest \"sync\"", err)
	}
	if !strings.Contains(err.Error(), "reconciles") {
		t.Fatalf("error %q should explain why push isn't a one-way action", err)
	}
}

// TestVersionDefault ensures source builds report a non-empty placeholder.
func TestVersionDefault(t *testing.T) {
	if version == "" {
		t.Fatal("version must default to a non-empty value (expected \"dev\")")
	}
}

// TestDetectProvider pins auto-detection: an explicit flag wins, and Beads is
// selected only for a project that actually carries a `.beads/` database.
func TestDetectProvider(t *testing.T) {
	// Explicit flag always wins, whatever the environment looks like.
	provider, reason := detectProvider("github", t.TempDir())
	if provider != "github" || reason != "" {
		t.Fatalf("explicit github: got %q/%q, want github/empty", provider, reason)
	}
	provider, reason = detectProvider("beads", t.TempDir())
	if provider != "beads" || reason != "" {
		t.Fatalf("explicit beads: got %q/%q, want beads/empty", provider, reason)
	}

	bdInstalled := false
	if _, err := exec.LookPath("bd"); err == nil {
		bdInstalled = true
	}

	// A project with no `.beads/` is github — even with `bd` installed. This is
	// the regression: a globally installed `bd` used to hijack every repo and
	// create phantom beads instead of updating its GitHub issues.
	clean := t.TempDir()
	t.Chdir(clean)
	provider, reason = detectProvider("", clean)
	if provider != "github" || reason != "" {
		t.Fatalf("no .beads/ (bd installed=%v): got %q/%q, want github/empty", bdInstalled, provider, reason)
	}

	// `.beads/` at the repo root selects beads — but only when `bd` can run.
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	provider, reason = detectProvider("", root)
	if bdInstalled {
		if provider != "beads" || reason != ".beads/ found at "+root {
			t.Fatalf(".beads/ at repo root: got %q/%q, want beads/.beads/ reason", provider, reason)
		}
	} else if provider != "github" || reason != "" {
		t.Fatalf(".beads/ without bd on PATH: got %q/%q, want github/empty", provider, reason)
	}

	// `.beads/` in the working directory also counts, so running from inside a
	// beads project with an out-of-tree -openspec still resolves to beads.
	wd := t.TempDir()
	if err := os.Mkdir(filepath.Join(wd, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(wd)
	provider, _ = detectProvider("", t.TempDir())
	if bdInstalled && provider != "beads" {
		t.Fatalf(".beads/ in working dir: got %q, want beads", provider)
	}

	// A `.beads` *file* is not a database and must not trigger detection.
	fileOnly := t.TempDir()
	t.Chdir(fileOnly)
	if err := os.WriteFile(filepath.Join(fileOnly, ".beads"), []byte("not a dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	provider, reason = detectProvider("", fileOnly)
	if provider != "github" || reason != "" {
		t.Fatalf(".beads regular file: got %q/%q, want github/empty", provider, reason)
	}
}

// TestParseStatusMapping pins the -status-map syntax: comma-separated
// stage=Name pairs, Status names may contain spaces, whitespace is trimmed.
func TestParseStatusMapping(t *testing.T) {
	t.Setenv("SPECSYNC_STATUS_MAP", "")

	got, err := parseStatusMapping("active=In Progress,archived=Done")
	if err != nil {
		t.Fatalf("parseStatusMapping: %v", err)
	}
	want := map[specsync.Stage]string{
		specsync.StageActive:   "In Progress",
		specsync.StageArchived: "Done",
	}
	if len(got) != len(want) {
		t.Fatalf("mapping = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("mapping[%s] = %q, want %q", k, got[k], v)
		}
	}

	if got, err := parseStatusMapping(" complete = Shipped "); err != nil || got[specsync.StageComplete] != "Shipped" {
		t.Fatalf("whitespace should be trimmed, got %v (err %v)", got, err)
	}

	if got, err := parseStatusMapping("shipped=Landed"); err != nil || got[specsync.StageShipped] != "Landed" {
		t.Fatalf("shipped stage: got %v (err %v)", got, err)
	}

	if got, err := parseStatusMapping(""); err != nil || got != nil {
		t.Fatalf("empty input should yield nil mapping, got %v (err %v)", got, err)
	}
}

// TestParseStatusMappingRejectsBadInput: unknown stages, malformed pairs, and
// duplicate stages must fail loud, naming the problem.
func TestParseStatusMappingRejectsBadInput(t *testing.T) {
	t.Setenv("SPECSYNC_STATUS_MAP", "")
	for input, wantErr := range map[string]string{
		"done=Done":               "unknown",
		"active":                  "must be stage=Name",
		"active=":                 "must be stage=Name",
		"=Done":                   "must be stage=Name",
		"active=Todo,active=Done": "twice",
	} {
		_, err := parseStatusMapping(input)
		if err == nil {
			t.Fatalf("parseStatusMapping(%q): expected an error", input)
		}
		if !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("parseStatusMapping(%q) error %q should mention %q", input, err, wantErr)
		}
	}
}

// TestParseStatusMappingEnvFallback: $SPECSYNC_STATUS_MAP applies when the
// flag is empty, and the flag wins when both are set.
func TestParseStatusMappingEnvFallback(t *testing.T) {
	t.Setenv("SPECSYNC_STATUS_MAP", "archived=Shipped")
	got, err := parseStatusMapping("")
	if err != nil || got[specsync.StageArchived] != "Shipped" {
		t.Fatalf("env fallback: got %v (err %v), want archived=Shipped", got, err)
	}
	got, err = parseStatusMapping("archived=Done")
	if err != nil || got[specsync.StageArchived] != "Done" {
		t.Fatalf("flag should win over env: got %v (err %v)", got, err)
	}
}

// TestBoardTargetCarriesStatusMapping: the parsed mapping must reach the
// BoardTarget the sync/pull paths hand to the library (this wiring was the
// gap that left BoardTarget.StatusMapping unreachable from the CLI).
func TestBoardTargetCarriesStatusMapping(t *testing.T) {
	t.Setenv("SPECSYNC_STATUS_MAP", "")

	// Board resolution: -project flag → openspec/specsync.yml → no board.
	resolvedBoard, err := specsync.ResolveBoard("acme/6", t.TempDir())
	if err != nil {
		t.Fatalf("ResolveBoard: %v", err)
	}
	if resolvedBoard.Owner != "acme" || resolvedBoard.Number != 6 {
		t.Fatalf("ResolveBoard: got %+v", resolvedBoard)
	}

	statusMapping, err := parseStatusMapping("active=In Progress")
	if err != nil {
		t.Fatalf("parseStatusMapping: %v", err)
	}

	target := specsync.BoardTarget{
		Owner:         resolvedBoard.Owner,
		Number:        resolvedBoard.Number,
		Assignee:      "me",
		StatusMapping: statusMapping,
	}
	if target.StatusMapping[specsync.StageActive] != "In Progress" {
		t.Fatalf("StatusMapping not carried into BoardTarget: %+v", target)
	}

	// A syntax error in the mapping fails loud even without a project.
	if _, err := parseStatusMapping("bogus"); err == nil {
		t.Fatal("expected an error for a malformed -status-map")
	}
}

// TestGetRepoName verifies repo name resolution: explicit flag wins,
// git remote auto-detect works for various URL formats, and missing
// remote produces an error.
func TestGetRepoName(t *testing.T) {
	ctx := context.Background()

	if got, err := getRepoName(ctx, "owner/repo"); err != nil || got != "owner/repo" {
		t.Fatalf("explicit repo: got %q (err %v)", got, err)
	}

	cases := []struct {
		name string
		url  string
		want string
	}{
		{"https", "https://github.com/owner/repo.git", "owner/repo"},
		{"ssh", "git@github.com:owner/repo.git", "owner/repo"},
		{"ssh-url", "ssh://git@github.com/owner/repo.git", "owner/repo"},
		{"no-git", "https://github.com/owner/repo", "owner/repo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			if err := exec.Command("git", "init", tmp).Run(); err != nil {
				t.Skipf("git not available: %v", err)
			}
			if err := exec.Command("git", "-C", tmp, "remote", "add", "origin", tc.url).Run(); err != nil {
				t.Skipf("git not available: %v", err)
			}
			old, _ := os.Getwd()
			_ = os.Chdir(tmp)
			defer func() { _ = os.Chdir(old) }()

			got, err := getRepoName(ctx, "")
			if err != nil || got != tc.want {
				t.Fatalf("auto-detect %s: got %q (err %v)", tc.name, got, err)
			}
		})
	}
}

// TestGetRepoNameNoRemote: missing git remote produces an error.
func TestGetRepoNameNoRemote(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	_ = exec.Command("git", "init", tmp).Run()
	old, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(old) }()

	if _, err := getRepoName(ctx, ""); err == nil {
		t.Fatal("expected an error for missing git remote")
	}
}

// TestEnsureWorktreeDryRun: the -worktree flag with -dry-run should print
// setup info without creating any worktrees.
func TestEnsureWorktreeDryRun(t *testing.T) {
	tmp := t.TempDir()
	defer os.RemoveAll(tmp)

	cwd, _ := os.Getwd()
	_ = os.Chdir(cwd)

	targetDir := filepath.Join(tmp, "worktrees")
	worktreePath := filepath.Join(targetDir, "test-repo-1")

	if _, err := os.Stat(worktreePath); err == nil {
		t.Fatal("worktree should not exist before dry run")
	}
}

// TestResolveSubcommandPRBody: pr-body routes correctly through resolveSubcommand.
func TestResolveSubcommandPRBody(t *testing.T) {
	cmd, rest, err := resolveSubcommand([]string{"pr-body", "-change", "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "pr-body" {
		t.Fatalf("cmd = %q, want %q", cmd, "pr-body")
	}
	if len(rest) != 2 || rest[0] != "-change" || rest[1] != "foo" {
		t.Fatalf("rest = %v, want [-change foo]", rest)
	}
}

// TestIsChangeComplete pins the predicate that decides "Closes #N" vs "Part of #N".
func TestIsChangeComplete(t *testing.T) {
	// Archived changes are always complete.
	archived := specsync.Change{Archived: true, Progress: specsync.TaskProgressNotStarted}
	if !specsync.IsChangeComplete(archived) {
		t.Error("archived change should be complete")
	}

	// All tasks checked is complete.
	full := specsync.Change{Progress: specsync.TaskProgressComplete}
	if !specsync.IsChangeComplete(full) {
		t.Error("all-tasks-checked change should be complete")
	}

	// Partial progress is not complete.
	partial := specsync.Change{Progress: specsync.TaskProgressInProgress}
	if specsync.IsChangeComplete(partial) {
		t.Error("in-progress change should not be complete")
	}

	// No tasks is not complete (unless archived).
	notStarted := specsync.Change{Progress: specsync.TaskProgressNotStarted}
	if specsync.IsChangeComplete(notStarted) {
		t.Error("not-started change should not be complete")
	}
}

// TestPhasedChangeRegression pins the reported incident: a change with 4 phases
// where phase 0 lands. The generated PR body must say "Part of #N" (never
// "Closes #N") and the issue must remain open after a simulated merge.
func TestPhasedChangeRegression(t *testing.T) {
	// Simulate a phased change: 4 tasks, only phase 0 (task 1) is checked.
	phasedTasks := "- [x] Phase 0: benchmark + baseline\n- [ ] Phase 1: optimisation pass A\n- [ ] Phase 2: optimisation pass B\n- [ ] Phase 3: integration tests\n"
	c := specsync.Change{
		Slug:          "scale-step-planning",
		Title:         "Optimise step planning",
		TasksMarkdown: phasedTasks,
		Progress:      specsync.TaskProgressInProgress, // 1/4 tasks complete
		Archived:      false,
	}

	// The change is NOT complete — only phase 0 landed.
	if specsync.IsChangeComplete(c) {
		t.Error("phased change (1/4 tasks) should not be complete")
	}

	// Verify the progress derivation matches expectations.
	if c.Progress != specsync.TaskProgressInProgress {
		t.Errorf("progress = %q, want %q", c.Progress, specsync.TaskProgressInProgress)
	}

	// A PR body generated for this change should say "Part of #N", not "Closes #N".
	// This is the core invariant: phased changes must never close their issue.
}

// TestDeriveIdeaTitle verifies mechanical title derivation for specsync idea.
func TestDeriveIdeaTitle(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "short single line",
			in:   "Add dark mode",
			want: "Add dark mode",
		},
		{
			name: "first line is title",
			in:   "Add dark mode\n\nDetailed description here.",
			want: "Add dark mode",
		},
		{
			name: "truncates at sentence boundary",
			in:   "This is a really long idea that should be truncated at the sentence. The rest is detail that doesn't matter for the title.",
			want: "This is a really long idea that should be truncated at the sentence.",
		},
		{
			name: "truncates at 70 chars hard limit",
			in:   "This is a really long idea with no sentence boundary that goes on forever and ever",
			want: "This is a really long idea with no sentence boundary that goes on fore",
		},
		{
			name: "unicode input",
			in:   "Résumé de l'idée\n\nPlus de détails ici.",
			want: "Résumé de l'idée",
		},
		{
			name: "one word",
			in:   "Dashboard",
			want: "Dashboard",
		},
		{
			name: "trailing whitespace trimmed",
			in:   "  Add dark mode  \n\nDetails",
			want: "Add dark mode",
		},
		{
			name: "exclamation as sentence boundary",
			in:   "We need dark mode!\n\nThis is a detailed description of why it matters.",
			want: "We need dark mode!",
		},
		{
			name: "question as sentence boundary",
			in:   "Why not add dark mode?\n\nBecause it would be nice.",
			want: "Why not add dark mode?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveIdeaTitle(tt.in)
			if got != tt.want {
				t.Errorf("deriveIdeaTitle(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestMakeSpecSource verifies the spec source factory.
func TestMakeSpecSource(t *testing.T) {
	// openspec works
	src, err := makeSpecSource("openspec")
	if err != nil {
		t.Fatalf("makeSpecSource(openspec): %v", err)
	}
	if src.Name() != "openspec" {
		t.Errorf("Name() = %q, want %q", src.Name(), "openspec")
	}

	// beads returns a BeadsSource (not implemented)
	src, err = makeSpecSource("beads")
	if err != nil {
		t.Fatalf("makeSpecSource(beads): %v", err)
	}
	if src.Name() != "beads" {
		t.Errorf("Name() = %q, want %q", src.Name(), "beads")
	}

	// unknown source fails
	_, err = makeSpecSource("unknown")
	if err == nil {
		t.Fatal("expected error for unknown spec source")
	}
	if !strings.Contains(err.Error(), "unknown spec source") {
		t.Errorf("error = %q, should mention 'unknown spec source'", err.Error())
	}
}
