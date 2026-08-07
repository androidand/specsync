package spec

import (
	"fmt"

	"github.com/androidand/specsync"
)

// BeadsSource is a placeholder for Beads spec format support.
// It implements specsync.SpecSource but returns "not implemented" for all operations.
//
// Beads format requirements (for future implementation):
// - Beads uses a .beads/ directory as a task graph database
// - Each bead represents a task with open/closed status
// - Tasks are linked via edges (parent/child, blocked-by, etc.)
// - Mapping to Change: each bead family maps to a Change, with
//   bead open/closed status mapping to task completion state
type BeadsSource struct{}

// Name returns "beads".
func (s BeadsSource) Name() string { return "beads" }

// LoadChanges returns an error indicating Beads support is not yet implemented.
func (s BeadsSource) LoadChanges(_ string) ([]specsync.Change, error) {
	return nil, fmt.Errorf("beads support not yet implemented")
}

// SaveChange returns an error indicating Beads support is not yet implemented.
func (s BeadsSource) SaveChange(_ specsync.Change) error {
	return fmt.Errorf("beads support not yet implemented")
}

// Ensure BeadsSource implements specsync.SpecSource.
var _ specsync.SpecSource = BeadsSource{}
