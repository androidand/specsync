package spec

import (
	"fmt"

	"github.com/androidand/specsync"
)

// OpenSpecSource loads changes from OpenSpec format (openspec/changes/ directory).
// This is the default and primary SpecSource implementation.
// It delegates to the root package's specsync.LoadChanges() to avoid duplicate file-reading logic.
type OpenSpecSource struct{}

func (s OpenSpecSource) Name() string {
	return "openspec"
}

// LoadChanges loads all changes from openspec/changes, including archived.
// Delegates to specsync.LoadChanges() in the root package to use the canonical
// implementation and avoid maintenance hazards from duplicate code.
func (s OpenSpecSource) LoadChanges(specDir string) (interface{}, error) {
	// Call the canonical implementation in the root package
	changes, err := specsync.LoadChanges(specDir)
	if err != nil {
		return nil, fmt.Errorf("loadchanges from openspec: %w", err)
	}
	return changes, nil
}
