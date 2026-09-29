package specsync

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DeltaOp is the operation a delta section performs on the accumulated spec.
type DeltaOp string

const (
	OpAdded    DeltaOp = "ADDED"
	OpModified DeltaOp = "MODIFIED"
	OpRemoved  DeltaOp = "REMOVED"
	OpRenamed  DeltaOp = "RENAMED"
)

// Scenario is a "#### Scenario:" block: a name and its WHEN/THEN/AND steps.
type Scenario struct {
	Name  string   `json:"name"`
	Steps []string `json:"steps,omitempty"`
}

// Requirement is a "### Requirement:" block within a delta section.
type Requirement struct {
	Name      string     `json:"name"`
	Op        DeltaOp    `json:"op"`
	Text      string     `json:"text,omitempty"`
	Scenarios []Scenario `json:"scenarios,omitempty"`
}

// Normative reports whether the requirement's first body line carries SHALL or
// MUST, which is what `openspec validate` enforces.
func (r Requirement) Normative() bool {
	for _, line := range strings.Split(r.Text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		return strings.Contains(line, "SHALL") || strings.Contains(line, "MUST")
	}
	return false
}

// SpecDelta is one capability's delta file inside a change's specs/ folder.
type SpecDelta struct {
	Capability   string        `json:"capability"`
	Path         string        `json:"path"`
	Purpose      string        `json:"purpose,omitempty"`
	Requirements []Requirement `json:"requirements,omitempty"`
}

// LoadSpecDeltas reads every specs/**/spec.md under a change folder. A change
// with no specs/ directory returns nil without error: not every change carries
// a behaviour contract.
func LoadSpecDeltas(changeDir string) ([]SpecDelta, error) {
	root := filepath.Join(changeDir, "specs")
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	var out []SpecDelta
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		capability := filepath.ToSlash(filepath.Dir(rel))
		if capability == "." {
			capability = strings.TrimSuffix(d.Name(), ".md")
		}
		delta := parseSpecDelta(string(data))
		delta.Capability = capability
		delta.Path = path
		out = append(out, delta)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Capability < out[j].Capability })
	return out, nil
}

func parseSpecDelta(src string) SpecDelta {
	var d SpecDelta
	var op DeltaOp
	var purpose []string
	inPurpose := false
	var req *Requirement
	var scn *Scenario

	flushScenario := func() {
		if req != nil && scn != nil {
			req.Scenarios = append(req.Scenarios, *scn)
		}
		scn = nil
	}
	flushRequirement := func() {
		flushScenario()
		if req != nil {
			req.Text = strings.TrimSpace(req.Text)
			d.Requirements = append(d.Requirements, *req)
		}
		req = nil
	}

	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "#### Scenario:"):
			flushScenario()
			if req != nil {
				scn = &Scenario{Name: strings.TrimSpace(strings.TrimPrefix(trimmed, "#### Scenario:"))}
			}
			inPurpose = false
		case strings.HasPrefix(trimmed, "### Requirement:"):
			flushRequirement()
			req = &Requirement{
				Name: strings.TrimSpace(strings.TrimPrefix(trimmed, "### Requirement:")),
				Op:   op,
			}
			inPurpose = false
		case strings.HasPrefix(trimmed, "## "):
			flushRequirement()
			heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			inPurpose = strings.EqualFold(heading, "Purpose")
			if parsed, ok := parseDeltaHeading(heading); ok {
				op = parsed
			}
		default:
			switch {
			case scn != nil:
				if strings.HasPrefix(trimmed, "-") {
					step := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
					if step != "" {
						scn.Steps = append(scn.Steps, step)
					}
				}
			case req != nil:
				req.Text += line + "\n"
			case inPurpose:
				purpose = append(purpose, line)
			}
		}
	}
	flushRequirement()
	d.Purpose = strings.TrimSpace(strings.Join(purpose, "\n"))
	return d
}

// parseDeltaHeading recognises "ADDED Requirements" and its siblings. The
// operation word may appear anywhere in the heading so that both
// "## ADDED Requirements" and "## Requirements (ADDED)" are understood.
func parseDeltaHeading(heading string) (DeltaOp, bool) {
	upper := strings.ToUpper(heading)
	if !strings.Contains(upper, "REQUIREMENT") {
		return "", false
	}
	for _, op := range []DeltaOp{OpAdded, OpModified, OpRemoved, OpRenamed} {
		if strings.Contains(upper, string(op)) {
			return op, true
		}
	}
	return "", false
}

// RequirementCount returns the total requirements across every delta.
func RequirementCount(deltas []SpecDelta) int {
	n := 0
	for _, d := range deltas {
		n += len(d.Requirements)
	}
	return n
}

// ScenarioCount returns the total scenarios across every delta.
func ScenarioCount(deltas []SpecDelta) int {
	n := 0
	for _, d := range deltas {
		for _, r := range d.Requirements {
			n += len(r.Scenarios)
		}
	}
	return n
}
