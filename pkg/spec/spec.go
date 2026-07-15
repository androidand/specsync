package spec

// SpecSource defines how to load and save changes from a spec format.
// Implementations can support OpenSpec, Beads, GitLab Wiki, ADRs, or custom formats.
type SpecSource interface {
	// Name returns the spec source identifier (e.g., "openspec", "beads").
	// Used for logging, error messages, and CLI flags.
	Name() string

	// LoadChanges loads all changes from the spec directory root.
	// Returns an empty slice (not an error) if no changes are found.
	// Errors are returned only for I/O failures or malformed specs, not missing specs.
	LoadChanges(specDir string) ([]Change, error)

	// SaveChange persists a change to disk.
	// Used for metadata updates, state changes, and future multi-directional sync.
	// Not required for read-only operations.
	SaveChange(change Change) error
}

// Change represents a single change/spec/ticket.
// This is the currency of specsync: every operation works with []Change.
type Change struct {
	Slug          string          // unique identifier within spec source (e.g., "feature-x")
	Dir           string          // full path to change directory
	Title         string          // from proposal/spec title
	Body          string          // from proposal/spec body
	TasksMarkdown string          // from tasks.md or equivalent
	Archived      bool            // true if in archive/ or marked archived
	Created       int64           // Unix timestamp when created
	Modified      int64           // Unix timestamp when last modified
	Metadata      *ChangeMetadata // workflow state: stage, priority (committed)
}

// ChangeMetadata represents workflow state (stage, priority) for a change.
// Stored in .specsync/metadata.json (OpenSpec) or equivalent.
type ChangeMetadata struct {
	Version  int    `json:"version"`
	Stage    string `json:"stage,omitempty"`      // backlog, active, blocked, in-review, complete, archived
	Priority *int   `json:"priority,omitempty"`   // 1-100, nil = default (0)
}
