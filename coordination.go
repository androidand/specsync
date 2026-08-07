// Package specsync reads OpenSpec coordination data (references, worksets)
// without duplicating it. It shells out to the openspec CLI and degrades
// cleanly when the binary is absent, older, or returns unexpected output.
package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Coordination holds the resolved OpenSpec context: root repo and any
// referenced sibling stores with their local paths.
type Coordination struct {
	Root    StoreEntry  `json:"root"`
	Members []StoreEntry `json:"members"`
	Status  []string    `json:"status"`
}

// StoreEntry describes one OpenSpec store (repo) with its local path.
type StoreEntry struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	Role   string `json:"role"`
}

// Workset holds the named folder sets from openspec workset list.
type Workset struct {
	Worksets []WorksetEntry `json:"worksets"`
	Status   []string       `json:"status"`
}

// WorksetEntry is one named folder in a workset.
type WorksetEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ReadCoordination runs "openspec context --json" and parses the output.
// Returns nil (not error) when the binary is absent or the output is
// unexpected — the feature degrades cleanly in that case.
func ReadCoordination(ctx context.Context) (*Coordination, error) {
	out, err := exec.CommandContext(ctx, "openspec", "context", "--json").Output()
	if err != nil {
		if isNotFoundOrExit1(err) {
			return nil, nil // degrade: binary absent or too old
		}
		return nil, fmt.Errorf("openspec context: %w", err)
	}

	var coord Coordination
	if err := json.Unmarshal(out, &coord); err != nil {
		return nil, fmt.Errorf("parse openspec context: %w", err)
	}

	// Version-guard: if root.path is empty, the binary is likely too old
	// to support this output shape.
	if coord.Root.Path == "" {
		return nil, nil
	}

	return &coord, nil
}

// ReadWorksets runs "openspec workset list --json" and parses the output.
// Returns nil (not error) when the binary is absent or the output is
// unexpected — the feature degrades cleanly in that case.
func ReadWorksets(ctx context.Context) (*Workset, error) {
	out, err := exec.CommandContext(ctx, "openspec", "workset", "list", "--json").Output()
	if err != nil {
		if isNotFoundOrExit1(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("openspec workset list: %w", err)
	}

	var ws Workset
	if err := json.Unmarshal(out, &ws); err != nil {
		return nil, fmt.Errorf("parse openspec workset: %w", err)
	}

	return &ws, nil
}

// ReferencedSiblings returns the non-root stores from coordination,
// i.e. the sibling repos this project depends on.
func (c *Coordination) ReferencedSiblings() []StoreEntry {
	return c.Members
}

// SuggestBlockedBy returns a "## Blocked by" suggestion for a change when the
// referenced sibling has active changes that could be dependencies. It only
// suggests — it never auto-creates the tracker edge.
func (c *Coordination) SuggestBlockedBy(changeSlug, siblingSlug string) string {
	if len(c.Members) == 0 {
		return ""
	}
	return fmt.Sprintf("## Blocked by\n- [%s] %s", siblingSlug, changeSlug)
}

// isNotFoundOrExit1 returns true when the error indicates the openspec
// binary is not found or exited with status 1 (too old to support --json).
func isNotFoundOrExit1(err error) bool {
	if execErr, ok := err.(*exec.ExitError); ok {
		return execErr.ExitCode() == 1
	}
	return strings.Contains(err.Error(), "executable file not found")
}
