package spec

import (
	"fmt"
)

// BeadsSource loads changes from Beads format.
// Not yet implemented. This is a placeholder for Phase 7+ work.
// See: https://github.com/steveyegge/beads
type BeadsSource struct{}

func (s BeadsSource) Name() string {
	return "beads"
}

// LoadChanges is not yet implemented for Beads format.
func (s BeadsSource) LoadChanges(specDir string) ([]Change, error) {
	return nil, fmt.Errorf("Beads support not yet implemented")
}

// SaveChange is not yet implemented for Beads format.
func (s BeadsSource) SaveChange(change Change) error {
	return fmt.Errorf("Beads support not yet implemented")
}
