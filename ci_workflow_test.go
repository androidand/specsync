package specsync

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestCIWorkflowEnforcesTheDogfoodingGate pins #119: the workflow must run the
// unlinked-commit gate on full history, and every step must sit at one indent.
func TestCIWorkflowEnforcesTheDogfoodingGate(t *testing.T) {
	b, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)
	if !strings.Contains(ci, "changelog -resolve-refs -fail-on-unlinked-commits") {
		t.Error("ci.yml does not run the changelog gate with -fail-on-unlinked-commits")
	}
	if !strings.Contains(ci, "fetch-depth: 0") {
		t.Error("ci.yml does not fetch full history; the changelog range needs tags")
	}
	indents := map[string]bool{}
	for _, l := range strings.Split(ci, "\n") {
		if m := regexp.MustCompile(`^( +)- (name|run|uses):`).FindStringSubmatch(l); m != nil {
			indents[m[1]] = true
		}
	}
	if len(indents) != 1 {
		t.Errorf("steps are indented inconsistently: %v", indents)
	}
}
