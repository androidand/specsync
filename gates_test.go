package specsync

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var gateLogDir string

func gateRepo(t *testing.T, config string) string {
	t.Helper()
	gateLogDir = t.TempDir()
	config = strings.ReplaceAll(config, "LOG/", gateLogDir+"/")
	dir := t.TempDir()
	gitFixture(t, dir, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, SpecSyncConfigPath), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, dir, "add", "openspec")
	commitFile(t, dir, "a.txt", "a")
	t.Setenv(TrustFileEnv, filepath.Join(t.TempDir(), "trust.json"))
	return dir
}

func runGatesIn(t *testing.T, dir string, o GateOptions) (GateReport, string, error) {
	t.Helper()
	w, _ := LoadWays(dir)
	var out bytes.Buffer
	o.Dir, o.Root, o.Ways, o.Out = dir, dir, w, &out
	rep, err := RunGates(context.Background(), o)
	return rep, out.String(), err
}

const gateConfig = `gates:
  - {name: one, run: "echo ran-one >> LOG/order.log"}
  - {name: two, run: "echo ran-two >> LOG/order.log"}
  - {name: slow, run: "echo ran-slow >> LOG/order.log", optional: true}
`

func TestGatesRefuseUntrustedConfigAndRunNothing(t *testing.T) {
	dir := gateRepo(t, gateConfig)
	_, _, err := runGatesIn(t, dir, GateOptions{})
	if err == nil || !strings.Contains(err.Error(), "specsync trust") {
		t.Fatalf("want a trust error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(gateLogDir, "order.log")); statErr == nil {
		t.Fatal("no gate may run before the config is trusted")
	}
}

func TestGatesRunInOrderSkipOptionalAndStampTheHead(t *testing.T) {
	dir := gateRepo(t, gateConfig)
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	rep, _, err := runGatesIn(t, dir, GateOptions{})
	if err != nil || !rep.Passed {
		t.Fatalf("passed=%v err=%v", rep.Passed, err)
	}
	log, _ := os.ReadFile(filepath.Join(gateLogDir, "order.log"))
	if string(log) != "ran-one\nran-two\n" {
		t.Fatalf("order/optional wrong: %q", log)
	}
	head := gitFixture(t, dir, "rev-parse", "HEAD")
	if !rep.Stamped || !GatesPassedAt(context.Background(), dir, head) {
		t.Fatal("a full passing run must stamp the head")
	}
}

func TestGatesWithIncludesAnOptionalGateAndOnlyNeverStamps(t *testing.T) {
	dir := gateRepo(t, gateConfig)
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	if rep, _, err := runGatesIn(t, dir, GateOptions{With: []string{"slow"}}); err != nil || len(rep.Results) != 3 {
		t.Fatalf("with: %v %+v", err, rep)
	}
	os.Remove(filepath.Join(gateLogDir, "order.log"))
	gitFixture(t, dir, "commit", "-q", "--allow-empty", "-m", "next")
	rep, _, err := runGatesIn(t, dir, GateOptions{Only: []string{"two"}})
	if err != nil || len(rep.Results) != 1 || rep.Stamped {
		t.Fatalf("a partial run must not stamp: %v %+v", err, rep)
	}
	if GatesPassedAt(context.Background(), dir, gitFixture(t, dir, "rev-parse", "HEAD")) {
		t.Fatal("a partial run must not leave a valid stamp")
	}
}

func TestGatesStopAtFirstFailureUnlessContinuedAndFailureRemovesTheStamp(t *testing.T) {
	dir := gateRepo(t, "gates:\n  - {name: bad, run: \"exit 3\"}\n  - {name: after, run: \"echo x >> LOG/after.log\"}\n")
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	head := gitFixture(t, dir, "rev-parse", "HEAD")
	rep, _, err := runGatesIn(t, dir, GateOptions{})
	if err != nil || rep.Passed || len(rep.Results) != 2 || rep.Results[1].Status != "skipped" {
		t.Fatalf("stop at first failure: %v %+v", err, rep)
	}
	if _, statErr := os.Stat(filepath.Join(gateLogDir, "after.log")); statErr == nil {
		t.Fatal("the gate after a failure must not run")
	}
	rep, _, _ = runGatesIn(t, dir, GateOptions{Continue: true})
	if rep.Passed || rep.Results[1].Status != "passed" {
		t.Fatalf("continue must run the rest: %+v", rep)
	}
	if GatesPassedAt(context.Background(), dir, head) {
		t.Fatal("a failed run must not leave a stamp")
	}
}

func TestGateStampGoesStaleOnANewCommitAndOnADirtyTree(t *testing.T) {
	dir := gateRepo(t, "gates:\n  - {name: ok, run: \"true\"}\n")
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	rep, _, _ := runGatesIn(t, dir, GateOptions{})
	if !rep.Stamped {
		t.Fatal("expected a stamp")
	}
	commitFile(t, dir, "b.txt", "b")
	if GatesPassedAt(context.Background(), dir, gitFixture(t, dir, "rev-parse", "HEAD")) {
		t.Fatal("the stamp must be stale after a new commit")
	}
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644)
	rep, _, _ = runGatesIn(t, dir, GateOptions{})
	if !rep.Passed || rep.Stamped || rep.Note == "" {
		t.Fatalf("a dirty tree must pass without stamping, with a note: %+v", rep)
	}
}

func TestGatesRejectUnknownNames(t *testing.T) {
	dir := gateRepo(t, gateConfig)
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runGatesIn(t, dir, GateOptions{Only: []string{"nope"}}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("got %v", err)
	}
	if _, _, err := runGatesIn(t, dir, GateOptions{With: []string{"one"}}); err == nil {
		t.Fatal("-with must name an optional gate")
	}
}

func TestGatesWithNothingConfiguredSaysSo(t *testing.T) {
	dir := gateRepo(t, "flow: minimal\n")
	if _, _, err := runGatesIn(t, dir, GateOptions{}); err == nil || !strings.Contains(err.Error(), "no gates") {
		t.Fatalf("got %v", err)
	}
}

func TestGatesRunAtTheRepoTopAndStampFromASubdirectory(t *testing.T) {
	dir := gateRepo(t, "gates:\n  - {name: where, run: \"pwd > LOG/where.log\"}\n")
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "pkg", "x")
	os.MkdirAll(sub, 0o755)
	w, _ := LoadWays(dir)
	var out bytes.Buffer
	rep, err := RunGates(context.Background(), GateOptions{Dir: sub, Root: dir, Ways: w, Out: &out})
	if err != nil || !rep.Passed || !rep.Stamped {
		t.Fatalf("%v %+v", err, rep)
	}
	where, _ := os.ReadFile(filepath.Join(gateLogDir, "where.log"))
	if resolved(absPath(strings.TrimSpace(string(where)))) != resolved(absPath(dir)) {
		t.Fatalf("gates must run at the repo top, ran in %q", where)
	}
	if !GatesPassedAt(context.Background(), sub, gitFixture(t, dir, "rev-parse", "HEAD")) {
		t.Fatal("the stamp must be valid when read from the subdirectory too")
	}
}

func gateLog(t *testing.T) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(gateLogDir, "order.log"))
	return string(b)
}

func TestGatesIfStaleRunsNothingWhenTheStampNamesHeadOnACleanTree(t *testing.T) {
	dir := gateRepo(t, gateConfig)
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	if rep, _, _ := runGatesIn(t, dir, GateOptions{}); !rep.Stamped {
		t.Fatal("expected a stamp")
	}
	before := gateLog(t)
	// A skip needs no trust: nothing is run.
	t.Setenv(TrustFileEnv, filepath.Join(t.TempDir(), "other-trust.json"))
	rep, _, err := runGatesIn(t, dir, GateOptions{IfStale: true})
	if err != nil || !rep.Passed || !rep.Skipped {
		t.Fatalf("a fresh stamp must skip: %v %+v", err, rep)
	}
	if gateLog(t) != before {
		t.Fatalf("no gate may run when the stamp is fresh: %q", gateLog(t))
	}
}

func TestGatesIfStaleRunsAfterANewCommit(t *testing.T) {
	dir := gateRepo(t, gateConfig)
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	runGatesIn(t, dir, GateOptions{})
	commitFile(t, dir, "b.txt", "b")
	before := gateLog(t)
	rep, _, err := runGatesIn(t, dir, GateOptions{IfStale: true})
	if err != nil || !rep.Passed || rep.Skipped || !rep.Stamped {
		t.Fatalf("a new commit must run the gates and stamp: %v %+v", err, rep)
	}
	if gateLog(t) == before {
		t.Fatal("the gates must have run")
	}
}

func TestGatesIfStaleRunsOnADirtyTreeEvenWithAStampOnHead(t *testing.T) {
	dir := gateRepo(t, gateConfig)
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	runGatesIn(t, dir, GateOptions{})
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644)
	before := gateLog(t)
	rep, _, err := runGatesIn(t, dir, GateOptions{IfStale: true})
	if err != nil || rep.Skipped {
		t.Fatalf("a dirty tree must not skip: %v %+v", err, rep)
	}
	if gateLog(t) == before {
		t.Fatal("the gates must have run on the dirty tree")
	}
}

func TestGatesIfStaleWithOnlyIsRefusedAndRunsNothing(t *testing.T) {
	dir := gateRepo(t, gateConfig)
	if _, err := Trust(dir); err != nil {
		t.Fatal(err)
	}
	_, _, err := runGatesIn(t, dir, GateOptions{IfStale: true, Only: []string{"one"}})
	if err == nil || !strings.Contains(err.Error(), "-only") {
		t.Fatalf("got %v", err)
	}
	if gateLog(t) != "" {
		t.Fatal("nothing may run")
	}
}
