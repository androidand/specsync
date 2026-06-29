package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// minOpenSpecVersion is the lowest `openspec` whose JSON contract specsync was
// verified against. Checked once per run; below it, the trace features report a
// clear version error rather than misparsing output.
const minOpenSpecVersion = "1.4.0"

// OpenSpecCLI sources OpenSpec data by shelling out to the `openspec` binary's
// JSON output. It defers to OpenSpec for the spec model (deltas, status) rather
// than re-parsing markdown. Results are memoized so the Node CLI is spawned a
// bounded number of times: `list` once, `show` at most once per change.
type OpenSpecCLI struct {
	run func(ctx context.Context, args ...string) (string, error)

	changes      []OpenSpecChange // memoized list
	changesValid bool
	deltaCache   map[string][]OpenSpecDelta
	versionOK    *bool // nil until checked
}

// NewOpenSpecCLI returns a source driven by the real `openspec` binary.
func NewOpenSpecCLI() *OpenSpecCLI {
	return &OpenSpecCLI{run: runOpenSpec, deltaCache: map[string][]OpenSpecDelta{}}
}

// NewOpenSpecCLIFunc returns a source driven by the given runner. Used in tests.
func NewOpenSpecCLIFunc(run func(ctx context.Context, args ...string) (string, error)) *OpenSpecCLI {
	return &OpenSpecCLI{run: run, deltaCache: map[string][]OpenSpecDelta{}}
}

func runOpenSpec(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "openspec", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("openspec %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// ensureVersion checks `openspec --version` once against the pinned minimum.
func (o *OpenSpecCLI) ensureVersion(ctx context.Context) error {
	if o.versionOK != nil {
		if *o.versionOK {
			return nil
		}
		return fmt.Errorf("openspec is older than the required %s", minOpenSpecVersion)
	}
	out, err := o.run(ctx, "--version")
	ok := err == nil && versionAtLeast(strings.TrimSpace(out), minOpenSpecVersion)
	o.versionOK = &ok
	if !ok {
		return fmt.Errorf("openspec %q is older than the required %s (or unavailable)", strings.TrimSpace(out), minOpenSpecVersion)
	}
	return nil
}

// Changes lists changes with status, memoized. Parses tolerantly: unknown JSON
// fields are ignored, so a newer openspec that adds fields does not break us.
func (o *OpenSpecCLI) Changes(ctx context.Context) ([]OpenSpecChange, error) {
	if o.changesValid {
		return o.changes, nil
	}
	if err := o.ensureVersion(ctx); err != nil {
		return nil, err
	}
	out, err := o.run(ctx, "list", "--json")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Changes []struct {
			Name           string `json:"name"`
			Status         string `json:"status"`
			CompletedTasks int    `json:"completedTasks"`
			TotalTasks     int    `json:"totalTasks"`
		} `json:"changes"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return nil, fmt.Errorf("parse openspec list: %w", err)
	}
	o.changes = make([]OpenSpecChange, 0, len(payload.Changes))
	for _, c := range payload.Changes {
		o.changes = append(o.changes, OpenSpecChange{
			Name:           c.Name,
			Status:         c.Status,
			CompletedTasks: c.CompletedTasks,
			TotalTasks:     c.TotalTasks,
		})
	}
	o.changesValid = true
	return o.changes, nil
}

// Deltas returns the requirement deltas for one change, memoized per change.
func (o *OpenSpecCLI) Deltas(ctx context.Context, change string) ([]OpenSpecDelta, error) {
	if d, ok := o.deltaCache[change]; ok {
		return d, nil
	}
	if err := o.ensureVersion(ctx); err != nil {
		return nil, err
	}
	out, err := o.run(ctx, "show", change, "--json", "--deltas-only")
	if err != nil {
		return nil, err
	}
	// openspec may emit a leading "Warning:" line before the JSON object.
	if i := strings.Index(out, "{"); i > 0 {
		out = out[i:]
	}
	var payload struct {
		Deltas []struct {
			Spec        string `json:"spec"`
			Operation   string `json:"operation"`
			Requirement struct {
				Text string `json:"text"`
			} `json:"requirement"`
		} `json:"deltas"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return nil, fmt.Errorf("parse openspec show %s: %w", change, err)
	}
	deltas := make([]OpenSpecDelta, 0, len(payload.Deltas))
	for _, d := range payload.Deltas {
		deltas = append(deltas, OpenSpecDelta{
			Spec:        d.Spec,
			Operation:   d.Operation,
			Requirement: d.Requirement.Text,
		})
	}
	o.deltaCache[change] = deltas
	return deltas, nil
}

// HasBaseline reports whether any accepted spec exists. Before the first
// archive, `openspec list --specs` reports none, so every delta is necessarily
// ADDED.
func (o *OpenSpecCLI) HasBaseline(ctx context.Context) (bool, error) {
	if err := o.ensureVersion(ctx); err != nil {
		return false, err
	}
	out, err := o.run(ctx, "list", "--specs", "--json")
	if err != nil {
		// A non-JSON "No specs found" message is the pre-baseline state.
		if strings.Contains(strings.ToLower(err.Error()), "no specs") {
			return false, nil
		}
		return false, err
	}
	if i := strings.Index(out, "{"); i > 0 {
		out = out[i:]
	}
	var payload struct {
		Specs []json.RawMessage `json:"specs"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		// Tolerate the textual "No specs found." output.
		if strings.Contains(strings.ToLower(out), "no specs") {
			return false, nil
		}
		return false, fmt.Errorf("parse openspec list --specs: %w", err)
	}
	return len(payload.Specs) > 0, nil
}

// versionAtLeast reports whether semver-ish got >= want, comparing the numeric
// MAJOR.MINOR.PATCH components. Tolerant of a leading "v" and trailing metadata.
func versionAtLeast(got, want string) bool {
	gp := splitVersionInts(got)
	wp := splitVersionInts(want)
	for i := 0; i < 3; i++ {
		if gp[i] != wp[i] {
			return gp[i] > wp[i]
		}
	}
	return true
}

func splitVersionInts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	// Drop any pre-release/build metadata.
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		out[i] = atoiSafe(part)
	}
	return out
}
