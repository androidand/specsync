package specsync

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const fullConfig = `board: acme/4
flow: pr-review
gates:
  - {name: lint, run: "make lint"}
  - {name: test, run: "make test"}
  - {name: e2e, run: "make e2e", optional: true}
worktrees: {dir: "../worktrees", branch: "{type}/{issue}-{slug}"}
text: {profile: conventional}
cleanup: {remote_branch: keep, archive: true}
sync: {strategy: merge, after_review: rebase, force: never}
`

func TestParseWaysFullConfig(t *testing.T) {
	w, issues := ParseWays([]byte(fullConfig))
	if len(issues) != 0 {
		t.Fatalf("issues: %+v", issues)
	}
	if w.Flow != "pr-review" || !reflect.DeepEqual(w.Steps, presets["pr-review"]) {
		t.Fatalf("flow = %q steps = %v", w.Flow, w.Steps)
	}
	if len(w.Gates) != 3 || w.Gates[2].Name != "e2e" || !w.Gates[2].Optional || w.Gates[0].Run != "make lint" {
		t.Fatalf("gates = %+v", w.Gates)
	}
	if w.Worktrees.Dir != "../worktrees" || w.Text.Profile != "conventional" || !w.Cleanup.Archive || w.Cleanup.RemoteBranch != "keep" {
		t.Fatalf("sections = %+v %+v %+v", w.Worktrees, w.Text, w.Cleanup)
	}
	if w.Sync != (SyncConfig{Strategy: "merge", AfterReview: "rebase", Force: "never"}) {
		t.Fatalf("sync = %+v", w.Sync)
	}
}

func TestParseWaysDefaultsWhenAbsent(t *testing.T) {
	w, issues := ParseWays(nil)
	if len(issues) != 0 || w.Flow != "pr-review" || !reflect.DeepEqual(w.Steps, presets["pr-review"]) {
		t.Fatalf("w = %+v issues = %v", w, issues)
	}
	if w.Sync != (SyncConfig{Strategy: "rebase", AfterReview: "merge", Force: "with-lease"}) {
		t.Fatalf("documented sync defaults, got %+v", w.Sync)
	}
	w, _ = ParseWays([]byte("board: acme/1\n"))
	if w.Flow != "pr-review" {
		t.Fatalf("flow = %q", w.Flow)
	}
}

func TestParseWaysPresetsAndCustomSteps(t *testing.T) {
	for _, name := range []string{"minimal", "pr-review", "strict"} {
		w, issues := ParseWays([]byte("flow: " + name + "\n"))
		if len(issues) != 0 || w.Flow != name || len(w.Steps) == 0 {
			t.Fatalf("%s: %+v %v", name, w, issues)
		}
	}
	w, issues := ParseWays([]byte("flow: [new, build, gate]\n"))
	if len(issues) != 0 || w.Flow != "" || !reflect.DeepEqual(w.Steps, []string{"new", "build", "gate"}) {
		t.Fatalf("custom: %+v %v", w, issues)
	}
}

func TestParseWaysIssues(t *testing.T) {
	cases := []struct {
		name, src string
		line      int
		want      string
	}{
		{"typo key", "gate:\n  - {name: a, run: b}\n", 1, `did you mean "gates"`},
		{"unknown key", "colour: red\n", 1, "unknown key"},
		{"unknown nested key", "sync:\n  stratgy: merge\n", 2, `did you mean "strategy"`},
		{"unknown preset", "flow: lenient\n", 1, "minimal, pr-review, strict"},
		{"unknown step", "flow: [new, deploy]\n", 1, `unknown step "deploy"`},
		{"flow wrong type", "flow:\n  a: b\n", 1, "preset name or a list"},
		{"gate without run", "gates:\n  - name: lint\n", 2, "has no run command"},
		{"gate without name", "gates:\n  - run: make\n", 2, "no name"},
		{"gates wrong type", "gates: make lint\n", 1, "must be a list"},
		{"optional not bool", "gates:\n  - {name: a, run: b, optional: maybe}\n", 2, "true or false"},
		{"bad strategy", "sync:\n  strategy: squash\n", 2, "rebase or merge"},
		{"bad force", "sync:\n  force: always\n", 2, "with-lease or never"},
		{"section wrong type", "worktrees: ../w\n", 1, "must be a map"},
		{"archive not bool", "cleanup:\n  archive: yes please\n", 2, "true or false"},
		{"unsupported yaml", "a: &x 1\n", 1, "anchors"},
		{"empty flow list", "flow: []\n", 1, "at least one step"},
		{"list root", "- a\n- b\n", 1, "map of keys"},
		{"scalar root", "just text\n", 1, "key: value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, issues := ParseWays([]byte(c.src))
			for _, i := range issues {
				if i.Line == c.line && strings.Contains(i.Msg, c.want) {
					return
				}
			}
			t.Fatalf("want line %d containing %q, got %+v", c.line, c.want, issues)
		})
	}
}

func TestParseWaysKeepsBoardAndIdeasRepoKnown(t *testing.T) {
	if _, issues := ParseWays([]byte("board: acme/1\nideas_repo: acme/ideas\n")); len(issues) != 0 {
		t.Fatalf("existing keys must stay valid: %+v", issues)
	}
}

func TestGateCommandsHonourOptional(t *testing.T) {
	w, _ := ParseWays([]byte(fullConfig))
	if got := w.GateCommands(false, nil); !reflect.DeepEqual(got, []string{"make lint", "make test"}) {
		t.Fatalf("default = %v", got)
	}
	if got := w.GateCommands(true, nil); len(got) != 3 {
		t.Fatalf("with optional = %v", got)
	}
	if got := w.GateCommands(false, []string{"e2e"}); !reflect.DeepEqual(got, []string{"make e2e"}) {
		t.Fatalf("by name = %v", got)
	}
}

func repoWithConfig(t *testing.T, cfg string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SpecSyncConfigPath), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPECSYNC_TRUST_FILE", filepath.Join(t.TempDir(), "trust.json"))
	return root
}

func TestTrustFreshCloneRefusesAndPrintsCommands(t *testing.T) {
	root := repoWithConfig(t, fullConfig)
	err := RequireTrust(root, []string{"make lint", "make test"})
	if err == nil {
		t.Fatal("an untrusted config must not run commands")
	}
	for _, want := range []string{"make lint", "make test", "specsync trust -yes"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
}

func TestTrustRecordsAndChangedConfigNeedsTrustAgain(t *testing.T) {
	root := repoWithConfig(t, fullConfig)
	if _, err := Trust(root); err != nil {
		t.Fatal(err)
	}
	if err := RequireTrust(root, []string{"make lint"}); err != nil {
		t.Fatalf("trusted config must pass: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, SpecSyncConfigPath), []byte(fullConfig+"# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RequireTrust(root, []string{"make lint"}); err == nil {
		t.Fatal("a changed config must need trust again")
	}
	if err := Untrust(root); err != nil {
		t.Fatal(err)
	}
}

func TestTrustIsPerRepository(t *testing.T) {
	a := repoWithConfig(t, fullConfig)
	b := t.TempDir()
	if err := os.MkdirAll(filepath.Join(b, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, SpecSyncConfigPath), []byte(fullConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Trust(a); err != nil {
		t.Fatal(err)
	}
	if err := RequireTrust(b, []string{"make lint"}); err == nil {
		t.Fatal("trusting one repo must not trust another with identical config")
	}
}

func TestRequireTrustNeedsNothingWhenNoCommands(t *testing.T) {
	root := repoWithConfig(t, "board: acme/1\n")
	if err := RequireTrust(root, nil); err != nil {
		t.Fatalf("no commands, nothing to trust: %v", err)
	}
}

func TestParseWaysReservedKeysAreAcceptedAndNoted(t *testing.T) {
	w, issues := ParseWays([]byte("worktrees: {dir: ../w, branch: x}\ntext: {profile: plain}\ncleanup: {remote_branch: keep, archive: true}\n"))
	if len(issues) != 0 {
		t.Fatalf("reserved keys are valid: %+v", issues)
	}
	got := map[string]bool{}
	for _, r := range w.Reserved {
		got[r.Key] = true
	}
	for _, k := range []string{"worktrees.dir", "worktrees.branch", "cleanup.remote_branch"} {
		if !got[k] {
			t.Errorf("%s must be reported as reserved: %+v", k, w.Reserved)
		}
	}
	if got["cleanup.archive"] {
		t.Error("cleanup.archive is used by next, not reserved")
	}
}
