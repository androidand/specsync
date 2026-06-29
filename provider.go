package specsync

import "context"

// WorkItem is the provider-agnostic projection of a Change. Providers render it
// into their own issue/card shape.
type WorkItem struct {
	Slug     string
	Title    string
	Body     string // proposal plus rendered tasks; the provider prepends identity
	Stage    Stage
	Priority int
	Closed   bool // archived changes project as closed items
}

// Ref is the disposable binding between a Change and its projection in one
// provider. It is cached locally (never committed) and is always rebuildable
// from the identity marker the provider writes into the item body.
type Ref struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`  // provider-internal id / number
	URL      string `json:"url"` // human-facing link
}

// WorkProvider projects WorkItems outward. Implementations must be idempotent:
// Push with an existing ref updates the projection; without one it creates (and
// should defend against duplicates via Find).
type WorkProvider interface {
	// Name identifies the provider, e.g. "github". Used as the ref-cache key.
	Name() string

	// Push creates or updates the projection and returns its ref.
	Push(ctx context.Context, item WorkItem, existing *Ref) (Ref, error)

	// Find locates an existing projection by slug (via the identity marker),
	// returning (nil, nil) when none exists. Used to rebuild a lost cache.
	Find(ctx context.Context, slug string) (*Ref, error)
}

// FetchedItem is an existing provider item read back so it can be projected into
// a local OpenSpec change. It is the inbound counterpart of WorkItem.
type FetchedItem struct {
	ID     string
	URL    string
	Title  string
	Body   string
	Closed bool
	Labels []string
}

// IssueReader is an optional provider capability: reading an existing item by
// its provider id. Providers that support the issue-first pull flow implement
// it; the core detects it via type assertion so the minimal WorkProvider
// contract stays small.
type IssueReader interface {
	// Get fetches an existing item by its provider id (e.g. issue number).
	Get(ctx context.Context, id string) (FetchedItem, error)
}

// CommitSource yields the commits reachable in a revision range, parsed as
// Conventional Commits. It is an optional, type-asserted capability (like
// IssueReader): the Git implementation shells out to `git log`. An empty since
// defaults to the most recent reachable tag; an empty until defaults to HEAD.
// When paths is non-empty, only commits touching those path globs are returned
// (the area-scope path filter).
type CommitSource interface {
	Commits(ctx context.Context, since, until string, paths []string) ([]Commit, error)
}

// OpenSpecDelta is one requirement delta of an OpenSpec change, as reported by
// `openspec show --json --deltas-only`. Operation is ADDED, MODIFIED, or REMOVED.
type OpenSpecDelta struct {
	Spec        string
	Operation   string
	Requirement string
}

// OpenSpecChange is the metadata specsync reads from the openspec CLI for a
// change. Status is OpenSpec's task-derived status (e.g. in-progress, complete),
// distinct from specsync's own .status convention.
type OpenSpecChange struct {
	Name           string
	Status         string
	CompletedTasks int
	TotalTasks     int
	Deltas         []OpenSpecDelta
}

// IssueSearcher is an optional, type-asserted provider capability: finding open
// issues by a free-text query. `scan` uses it to surface in-area issues that
// link to no change. Providers that cannot search simply don't implement it, and
// scan degrades to omitting the issues section.
type IssueSearcher interface {
	SearchOpenIssues(ctx context.Context, query string) ([]FetchedItem, error)
}

// OpenSpecSource reads OpenSpec change metadata, completion status, and
// requirement deltas via the `openspec` CLI's JSON output. specsync defers to
// OpenSpec for the spec model rather than re-parsing markdown; this is the
// optional, type-asserted capability that provides it.
type OpenSpecSource interface {
	// Changes lists known changes with status (no deltas — cheap, called once).
	Changes(ctx context.Context) ([]OpenSpecChange, error)
	// Deltas returns the requirement deltas for one change (called per in-scope change).
	Deltas(ctx context.Context, change string) ([]OpenSpecDelta, error)
	// HasBaseline reports whether any accepted spec exists yet; when false, all
	// deltas are necessarily ADDED.
	HasBaseline(ctx context.Context) (bool, error)
}
