package spec

// SpecSource defines how to load and save changes from a spec format.
// Implementations can support OpenSpec, Beads, GitLab Wiki, ADRs, or custom formats.
// All SpecSource implementations work with the root package's specsync.Change type,
// ensuring a single canonical representation across all spec formats.
type SpecSource interface {
	// Name returns the spec source identifier (e.g., "openspec", "beads").
	// Used for logging, error messages, and CLI flags.
	Name() string

	// LoadChanges loads all changes from the spec directory root using the root package's
	// specsync.Change type. Returns an empty slice (not an error) if no changes are found.
	// Errors are returned only for I/O failures or malformed specs, not missing specs.
	// Note: Implementation uses github.com/androidand/specsync.LoadChange internally.
	LoadChanges(specDir string) (interface{}, error) // Returns []specsync.Change
}

// Note: pkg/spec does not define its own Change type.
// All implementations use the root package's specsync.Change type (defined in change.go).
// This ensures:
// 1. No duplicate Change definitions
// 2. Single source of truth for change representation
// 3. No type conversion friction when wiring spec sources together
