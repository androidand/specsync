package specsync

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// GateOptions selects and runs the configured gates.
type GateOptions struct {
	Dir      string // worktree the gates run in
	Root     string // repository root holding the config, for trust
	Ways     Ways
	Only     []string // run exactly these gates; never stamps
	With     []string // optional gates to include
	Continue bool     // keep running after a failure
	Out      io.Writer
}

// GateResult is one gate's outcome. Status is passed, failed or skipped.
type GateResult struct {
	Name     string        `json:"name"`
	Run      string        `json:"run"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"durationNs,omitempty"`
}

// GateReport is the outcome of a run. Stamped is true when the run left a
// stamp naming the current commit.
type GateReport struct {
	Results []GateResult `json:"gates"`
	Passed  bool         `json:"passed"`
	Stamped bool         `json:"stamped"`
	Head    string       `json:"head,omitempty"`
	Note    string       `json:"note,omitempty"`
}

func gateStampPath(ctx context.Context, dir string) (string, error) {
	path, err := gitIn(ctx, dir, "rev-parse", "--git-path", GateStampFile)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	return path, nil
}

func selectGates(w Ways, only, with []string) ([]Gate, error) {
	byName := map[string]Gate{}
	for _, g := range w.Gates {
		byName[g.Name] = g
	}
	for _, n := range only {
		if _, ok := byName[n]; !ok {
			return nil, fmt.Errorf("no gate named %q (configured: %s)", n, gateNames(w))
		}
	}
	for _, n := range with {
		g, ok := byName[n]
		if !ok || !g.Optional {
			return nil, fmt.Errorf("-with names %q, which is not an optional gate (optional: %s)", n, optionalGateNames(w))
		}
	}
	var out []Gate
	for _, g := range w.Gates {
		switch {
		case len(only) > 0:
			if !slices.Contains(only, g.Name) {
				continue
			}
		case g.Optional && !slices.Contains(with, g.Name):
			continue
		}
		out = append(out, g)
	}
	return out, nil
}

func gateNames(w Ways) string {
	var n []string
	for _, g := range w.Gates {
		n = append(n, g.Name)
	}
	return strings.Join(n, ", ")
}

func optionalGateNames(w Ways) string {
	var n []string
	for _, g := range w.Gates {
		if g.Optional {
			n = append(n, g.Name)
		}
	}
	if len(n) == 0 {
		return "none"
	}
	return strings.Join(n, ", ")
}

// PlanGates returns "name: command" for every gate RunGates would run.
func PlanGates(o GateOptions) ([]string, error) {
	if len(o.Ways.Gates) == 0 {
		return nil, fmt.Errorf("no gates are configured: add a gates: list to %s", SpecSyncConfigPath)
	}
	gates, err := selectGates(o.Ways, o.Only, o.With)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(gates))
	for i, g := range gates {
		out[i] = g.Name + ": " + g.Run
	}
	return out, nil
}

// RunGates runs the selected gates in order in Dir. The commands come from the
// repository's config, so nothing runs until the config is trusted. A full
// run (no Only) that passes on a clean tree writes a stamp naming HEAD; any
// other run removes an existing stamp, so the stamp only ever says "the
// configured gates passed on exactly this commit".
func RunGates(ctx context.Context, o GateOptions) (GateReport, error) {
	var rep GateReport
	if len(o.Ways.Gates) == 0 {
		return rep, fmt.Errorf("no gates are configured: add a gates: list to %s", SpecSyncConfigPath)
	}
	gates, err := selectGates(o.Ways, o.Only, o.With)
	if err != nil {
		return rep, err
	}
	if len(gates) == 0 {
		return rep, fmt.Errorf("no required gates are configured; name optional ones with -with or -only")
	}
	cmds := make([]string, len(gates))
	for i, g := range gates {
		cmds[i] = g.Run
	}
	if err := RequireTrust(o.Root, cmds); err != nil {
		return rep, err
	}
	top, err := gitIn(ctx, o.Dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return rep, err
	}
	o.Dir = top
	head, err := gitIn(ctx, o.Dir, "rev-parse", "HEAD")
	if err != nil {
		return rep, err
	}
	rep.Head = head
	stamp, err := gateStampPath(ctx, o.Dir)
	if err != nil {
		return rep, err
	}
	_ = os.Remove(stamp)

	out := o.Out
	if out == nil {
		out = io.Discard
	}
	failed := false
	for _, g := range gates {
		res := GateResult{Name: g.Name, Run: g.Run}
		if failed && !o.Continue {
			res.Status = "skipped"
			rep.Results = append(rep.Results, res)
			continue
		}
		fmt.Fprintf(out, "== gate %s: %s\n", g.Name, g.Run)
		start := time.Now()
		cmd := exec.CommandContext(ctx, "sh", "-c", g.Run)
		cmd.Dir = o.Dir
		cmd.Stdout, cmd.Stderr = out, out
		if err := cmd.Run(); err != nil {
			res.Status = "failed"
			failed = true
		} else {
			res.Status = "passed"
		}
		res.Duration = time.Since(start)
		rep.Results = append(rep.Results, res)
	}
	rep.Passed = !failed
	switch {
	case failed:
	case len(o.Only) > 0:
		rep.Note = "ran only the named gates, so no stamp was written"
	default:
		if st, err := gitIn(ctx, o.Dir, "status", "--porcelain", "--untracked-files=normal"); err != nil || st != "" {
			rep.Note = "the working tree has uncommitted changes, so the gates did not test the commit and no stamp was written"
			break
		}
		if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
			return rep, err
		}
		if err := os.WriteFile(stamp, []byte(head+"\n"), 0o644); err != nil {
			return rep, err
		}
		rep.Stamped = true
	}
	return rep, nil
}
