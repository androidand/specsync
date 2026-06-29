package specsync

import "testing"

func TestParseAndBumpVersion(t *testing.T) {
	v, err := ParseVersion("v1.4.1-beta+exp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Major != 1 || v.Minor != 4 || v.Patch != 1 || v.Pre != "beta" || v.Build != "exp" {
		t.Fatalf("parsed wrong: %+v", v)
	}
	if got := v.Bump(ImpactMinor).String(); got != "1.5.0" {
		t.Fatalf("minor bump = %q, want 1.5.0", got)
	}
	if got := v.Bump(ImpactMajor).String(); got != "2.0.0" {
		t.Fatalf("major bump = %q, want 2.0.0", got)
	}
	if got := v.Bump(ImpactPatch).String(); got != "1.4.2" {
		t.Fatalf("patch bump = %q, want 1.4.2", got)
	}
	if got := v.Bump(ImpactNone).String(); got != "1.4.1-beta+exp" {
		t.Fatalf("none bump should be unchanged, got %q", got)
	}
}

func TestParseVersionRejectsJunk(t *testing.T) {
	for _, s := range []string{"", "1.2", "x.y.z", "1.2.3.4"} {
		if _, err := ParseVersion(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

func TestInferImpactMaxAcrossSignals(t *testing.T) {
	commits := []Commit{
		ParseCommit("h1", "", "", "fix: small thing"),
		ParseCommit("h2", "", "", "feat: new thing"),
	}
	r := InferImpact(commits, nil, true, nil)
	if r.Impact != ImpactMinor {
		t.Fatalf("feat+fix should be minor, got %s", r.Impact)
	}
}

func TestInferImpactBreakingWins(t *testing.T) {
	commits := []Commit{
		ParseCommit("h1", "", "", "fix: a"),
		ParseCommit("h2", "", "", "refactor!: drop X"),
	}
	r := InferImpact(commits, nil, true, nil)
	if r.Impact != ImpactMajor {
		t.Fatalf("breaking should be major, got %s", r.Impact)
	}
}

func TestInferImpactSpecDeltaDrivesMajor(t *testing.T) {
	commits := []Commit{ParseCommit("h1", "", "", "refactor: rename")}
	deltas := []OpenSpecDelta{{Spec: "trace-model", Operation: "REMOVED", Requirement: "R"}}
	r := InferImpact(commits, deltas, true, nil)
	if r.Impact != ImpactMajor {
		t.Fatalf("a refactor that REMOVED a requirement should be major, got %s", r.Impact)
	}
}

func TestInferImpactPreBaselineCapsAtMinor(t *testing.T) {
	// Without a baseline, a REMOVED can't occur; an ADDED caps the spec signal at minor.
	deltas := []OpenSpecDelta{{Spec: "x", Operation: "ADDED", Requirement: "R"}}
	r := InferImpact(nil, deltas, false, nil)
	if r.Impact != ImpactMinor {
		t.Fatalf("pre-baseline ADDED should be minor, got %s", r.Impact)
	}
	// A spurious REMOVED pre-baseline contributes nothing.
	r2 := InferImpact(nil, []OpenSpecDelta{{Spec: "x", Operation: "REMOVED"}}, false, nil)
	if r2.Impact != ImpactNone {
		t.Fatalf("pre-baseline REMOVED should contribute none, got %s", r2.Impact)
	}
}

func TestInferImpactChoreOnlyIsNone(t *testing.T) {
	commits := []Commit{
		ParseCommit("h1", "", "", "chore: deps"),
		ParseCommit("h2", "", "", "docs: readme"),
		ParseCommit("h3", "", "", "Merge branch 'main'"),
	}
	r := InferImpact(commits, nil, true, nil)
	if r.Impact != ImpactNone {
		t.Fatalf("chore/docs/merge should be none, got %s", r.Impact)
	}
}
