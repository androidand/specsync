package spec

import "github.com/androidand/specsync"

// FileSpecSource implements specsync.SpecSource for the OpenSpec directory format.
type FileSpecSource struct{}

// Name returns "openspec".
func (s FileSpecSource) Name() string { return "openspec" }

// LoadChanges loads all changes from the OpenSpec directory.
func (s FileSpecSource) LoadChanges(specDir string) ([]specsync.Change, error) {
	return specsync.LoadChanges(specDir)
}

// SaveChange is a no-op; future implementations may write back to .specsync/metadata.json.
func (s FileSpecSource) SaveChange(_ specsync.Change) error { return nil }

// Ensure FileSpecSource implements specsync.SpecSource.
var _ specsync.SpecSource = FileSpecSource{}
