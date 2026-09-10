package specsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleDelta = `# credential-health

## Purpose
How the platform surfaces dead credentials.

## ADDED Requirements

### Requirement: Dead credentials pause one integration
The system SHALL pause only the affected integration.

#### Scenario: Definitive rejection pauses one
- **WHEN** a refresh is definitively rejected
- **THEN** that integration is paused
- **AND** its siblings keep running

#### Scenario: Transient failure does not pause
- **WHEN** a refresh times out
- **THEN** it is retried

## MODIFIED Requirements

### Requirement: Notification reaches the customer
Notification SHALL reach domain admins by email.

#### Scenario: Admins are emailed
- **WHEN** an integration is paused
- **THEN** admins receive an email
`

func TestParseSpecDeltaOperationsAndScenarios(t *testing.T) {
	d := parseSpecDelta(sampleDelta)
	if d.Purpose != "How the platform surfaces dead credentials." {
		t.Fatalf("purpose: %q", d.Purpose)
	}
	if len(d.Requirements) != 2 {
		t.Fatalf("want 2 requirements, got %d", len(d.Requirements))
	}
	first := d.Requirements[0]
	if first.Op != OpAdded {
		t.Fatalf("first op: %s", first.Op)
	}
	if len(first.Scenarios) != 2 {
		t.Fatalf("first scenarios: %d", len(first.Scenarios))
	}
	if got := first.Scenarios[0].Steps; len(got) != 3 {
		t.Fatalf("steps: %v", got)
	}
	if second := d.Requirements[1]; second.Op != OpModified {
		t.Fatalf("second op: %s", second.Op)
	}
}

func TestRequirementNormativeReadsFirstLineOnly(t *testing.T) {
	ok := Requirement{Text: "The system SHALL do it.\nMore prose."}
	if !ok.Normative() {
		t.Fatal("SHALL on the first line should be normative")
	}
	// openspec validate only reads the first body line; mirror that exactly.
	late := Requirement{Text: "When something happens,\nthe system SHALL do it."}
	if late.Normative() {
		t.Fatal("SHALL on a later line should not count")
	}
	none := Requirement{Text: "The system should probably do it."}
	if none.Normative() {
		t.Fatal("should is not normative")
	}
}

func TestLoadSpecDeltasAbsentSpecsIsNotAnError(t *testing.T) {
	deltas, err := LoadSpecDeltas(t.TempDir())
	if err != nil || deltas != nil {
		t.Fatalf("got %v, %v", deltas, err)
	}
}

func TestLoadSpecDeltasUsesDirectoryAsCapability(t *testing.T) {
	dir := t.TempDir()
	capDir := filepath.Join(dir, "specs", "credential-health")
	if err := os.MkdirAll(capDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capDir, "spec.md"), []byte(sampleDelta), 0o644); err != nil {
		t.Fatal(err)
	}
	deltas, err := LoadSpecDeltas(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 || deltas[0].Capability != "credential-health" {
		t.Fatalf("got %+v", deltas)
	}
	if RequirementCount(deltas) != 2 || ScenarioCount(deltas) != 3 {
		t.Fatalf("counts: %d reqs, %d scenarios", RequirementCount(deltas), ScenarioCount(deltas))
	}
}

func TestVerifyFlagsMissingScenarioAndNonNormative(t *testing.T) {
	c := Change{
		Slug: "x",
		Deltas: []SpecDelta{{
			Capability: "cap",
			Purpose:    "Real purpose.",
			Requirements: []Requirement{
				{Name: "no scenarios", Text: "It SHALL work."},
				{Name: "not normative", Text: "It should work.", Scenarios: []Scenario{{Name: "s", Steps: []string{"**THEN** ok"}}}},
			},
		}},
		TasksMarkdown: "- [x] done\n",
	}
	r := Verify(c)
	if r.OK() {
		t.Fatal("expected failure")
	}
	var missing, nonNormative bool
	for _, ch := range r.Checks {
		if ch.Status != CheckFail {
			continue
		}
		if ch.Name == "requirement without scenario: no scenarios" {
			missing = true
		}
		if ch.Name == "requirement without SHALL/MUST: not normative" {
			nonNormative = true
		}
	}
	if !missing || !nonNormative {
		t.Fatalf("checks: %+v", r.Checks)
	}
}

func TestVerifyUncontractedChangeWarnsButPasses(t *testing.T) {
	r := Verify(Change{Slug: "chore", TasksMarkdown: "- [x] bump dep\n"})
	if !r.OK() {
		t.Fatalf("a change with no delta should not fail: %+v", r.Checks)
	}
	if r.Contracted {
		t.Fatal("should be reported as uncontracted")
	}
}

func TestVerifyFailsOnOpenTasks(t *testing.T) {
	r := Verify(Change{Slug: "x", TasksMarkdown: "- [x] one\n- [ ] two\n"})
	if r.OK() {
		t.Fatal("open tasks should fail verification")
	}
}

func TestVerifyWarnsOnTBDPurpose(t *testing.T) {
	c := Change{Slug: "x", TasksMarkdown: "- [x] a\n", Deltas: []SpecDelta{{
		Capability:   "cap",
		Purpose:      "TBD - created by archiving change foo.",
		Requirements: []Requirement{{Name: "r", Text: "It SHALL work.", Scenarios: []Scenario{{Name: "s", Steps: []string{"**THEN** ok"}}}}},
	}}}
	r := Verify(c)
	var warned bool
	for _, ch := range r.Checks {
		if ch.Status == CheckWarn && ch.Name == "capability purpose: cap" {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("checks: %+v", r.Checks)
	}
}

func TestVerificationSectionRendersChecklist(t *testing.T) {
	deltas := []SpecDelta{{Capability: "cap", Requirements: []Requirement{
		{Name: "r", Op: OpAdded, Scenarios: []Scenario{{Name: "s", Steps: []string{"**WHEN** x", "**THEN** y"}}}},
	}}}
	got := VerificationSection(deltas)
	for _, want := range []string{"### cap", "**r** (added)", "- [ ] s", "  - **WHEN** x"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if VerificationSection(nil) != "" {
		t.Fatal("no deltas should render nothing")
	}
}
