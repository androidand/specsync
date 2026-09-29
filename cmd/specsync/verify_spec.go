package main

import (
	"encoding/json"
	"fmt"
	"os"

	specsync "github.com/androidand/specsync"
)

// runVerifySpec implements OpenSpec's Verify phase: it walks a change's spec
// deltas and reports the acceptance checklist a reviewer works through before
// the change is archived. Without this step the contract written during
// Propose is never read again, which is how a spec becomes pure ceremony.
func runVerifySpec(root specsync.ResolvedRoot, slug string, asJSON, checklist bool) {
	changes, err := specsync.LoadChanges(root.Dir)
	if err != nil {
		fail(err)
	}
	var target *specsync.Change
	for i := range changes {
		if changes[i].Slug == slug {
			target = &changes[i]
			break
		}
	}
	if target == nil {
		fail(fmt.Errorf("no change %q under %s", slug, root.Dir))
	}

	report := specsync.Verify(*target)

	if checklist {
		section := specsync.VerificationSection(target.Deltas)
		if section == "" {
			fail(fmt.Errorf("change %q has no specs/ delta, so there is no acceptance checklist to render", slug))
		}
		fmt.Println(section)
		return
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fail(err)
		}
		if !report.OK() {
			os.Exit(1)
		}
		return
	}

	fmt.Printf("verify %s\n", report.Slug)
	if report.Contracted {
		fmt.Printf("  %d requirement(s), %d scenario(s), tasks %s\n\n", report.Requirements, report.Scenarios, report.Tasks)
	} else {
		fmt.Printf("  no behaviour contract, tasks %s\n\n", report.Tasks)
	}
	for _, c := range report.Checks {
		fmt.Printf("  %s %s\n", checkGlyph(c.Status), c.Name)
		if c.Detail != "" {
			fmt.Printf("      %s\n", c.Detail)
		}
	}
	if len(report.Criteria) > 0 {
		fmt.Printf("\n  acceptance criteria\n")
		for _, line := range report.Criteria {
			fmt.Printf("    - [ ] %s\n", line)
		}
	}
	if !report.OK() {
		fmt.Fprintln(os.Stderr, "\nverify failed")
		os.Exit(1)
	}
}

func checkGlyph(s specsync.CheckStatus) string {
	switch s {
	case specsync.CheckPass:
		return "✓"
	case specsync.CheckFail:
		return "✗"
	default:
		return "!"
	}
}
