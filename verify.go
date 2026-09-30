package specsync

import (
	"fmt"
	"strings"
)

// CheckStatus is the outcome of one verification check.
type CheckStatus string

const (
	CheckPass CheckStatus = "pass"
	CheckFail CheckStatus = "fail"
	CheckWarn CheckStatus = "warn"
)

// Check is a single verifiable statement about a change.
type Check struct {
	Status CheckStatus `json:"status"`
	Name   string      `json:"name"`
	Detail string      `json:"detail,omitempty"`
}

// VerifyReport is the result of verifying one change against its own spec.
type VerifyReport struct {
	Slug         string   `json:"slug"`
	Contracted   bool     `json:"contracted"`
	Requirements int      `json:"requirements"`
	Scenarios    int      `json:"scenarios"`
	Tasks        string   `json:"tasks"`
	Checks       []Check  `json:"checks"`
	Criteria     []string `json:"criteria,omitempty"`
}

// OK reports whether no check failed.
func (r VerifyReport) OK() bool {
	for _, c := range r.Checks {
		if c.Status == CheckFail {
			return false
		}
	}
	return true
}

// Verify walks a change's spec deltas and task list and reports whether the
// change is ready to archive. It checks the shape of the contract, not the
// code: what it produces is the acceptance checklist a reviewer works through.
//
// A change with no deltas is not a failure. Not every piece of work changes
// observable behaviour, and forcing a contract onto a dependency bump is how
// specs become the ceremony their critics say they are. Such a change is
// reported as uncontracted, and only its tasks are checked.
func Verify(c Change) VerifyReport {
	total, done := CountCheckboxes(c.TasksMarkdown)
	r := VerifyReport{
		Slug:         c.Slug,
		Contracted:   len(c.Deltas) > 0,
		Requirements: RequirementCount(c.Deltas),
		Scenarios:    ScenarioCount(c.Deltas),
		Tasks:        fmt.Sprintf("%d/%d", done, total),
	}

	if !r.Contracted {
		r.Checks = append(r.Checks, Check{
			Status: CheckWarn,
			Name:   "behaviour contract",
			Detail: "no specs/ delta: nothing accumulates into openspec/specs/ when this is archived. Correct for a fix or chore; add a delta if this changes observable behaviour",
		})
	} else {
		r.Checks = append(r.Checks, Check{
			Status: CheckPass,
			Name:   "behaviour contract",
			Detail: fmt.Sprintf("%d requirement(s), %d scenario(s) across %d capability(ies)", r.Requirements, r.Scenarios, len(c.Deltas)),
		})
	}

	for _, d := range c.Deltas {
		if d.Purpose == "" || strings.Contains(strings.ToUpper(d.Purpose), "TBD") {
			r.Checks = append(r.Checks, Check{
				Status: CheckWarn,
				Name:   "capability purpose: " + d.Capability,
				Detail: "Purpose is missing or still TBD; it is the paragraph a newcomer reads first",
			})
		}
		for _, req := range d.Requirements {
			if len(req.Scenarios) == 0 {
				r.Checks = append(r.Checks, Check{
					Status: CheckFail,
					Name:   "requirement without scenario: " + req.Name,
					Detail: "every requirement needs at least one #### Scenario: block, or it cannot be verified",
				})
			}
			if !req.Normative() {
				r.Checks = append(r.Checks, Check{
					Status: CheckFail,
					Name:   "requirement without SHALL/MUST: " + req.Name,
					Detail: "openspec validate reads the first line of the requirement body for normative language",
				})
			}
			for _, s := range req.Scenarios {
				if !hasOutcome(s) {
					r.Checks = append(r.Checks, Check{
						Status: CheckWarn,
						Name:   "scenario without an outcome: " + s.Name,
						Detail: "a scenario needs a THEN step to be checkable",
					})
				}
			}
		}
	}

	switch {
	case total == 0:
		r.Checks = append(r.Checks, Check{Status: CheckWarn, Name: "tasks", Detail: "no tasks recorded"})
	case done < total:
		r.Checks = append(r.Checks, Check{
			Status: CheckFail,
			Name:   "tasks",
			Detail: fmt.Sprintf("%d of %d task(s) still open", total-done, total),
		})
	default:
		r.Checks = append(r.Checks, Check{Status: CheckPass, Name: "tasks", Detail: r.Tasks + " complete"})
	}

	r.Criteria = AcceptanceCriteria(c.Deltas)
	return r
}

func hasOutcome(s Scenario) bool {
	for _, step := range s.Steps {
		if strings.Contains(strings.ToUpper(step), "THEN") {
			return true
		}
	}
	return false
}

// AcceptanceCriteria renders a change's scenarios as reviewer-facing lines,
// one per scenario, qualified by capability and requirement.
func AcceptanceCriteria(deltas []SpecDelta) []string {
	var out []string
	for _, d := range deltas {
		for _, req := range d.Requirements {
			for _, s := range req.Scenarios {
				out = append(out, fmt.Sprintf("%s — %s: %s", d.Capability, req.Name, s.Name))
			}
		}
	}
	return out
}

// VerificationSection renders the acceptance checklist projected into an
// issue body, so the contract a reviewer must check travels with the issue.
func VerificationSection(deltas []SpecDelta) string {
	if len(deltas) == 0 {
		return ""
	}
	var b strings.Builder
	for _, d := range deltas {
		fmt.Fprintf(&b, "### %s\n\n", d.Capability)
		for _, req := range d.Requirements {
			fmt.Fprintf(&b, "**%s** (%s)\n\n", req.Name, strings.ToLower(string(req.Op)))
			for _, s := range req.Scenarios {
				fmt.Fprintf(&b, "- [ ] %s\n", s.Name)
				for _, step := range s.Steps {
					fmt.Fprintf(&b, "  - %s\n", step)
				}
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
