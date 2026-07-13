package specsync

import "context"

// WorkItem is the provider-agnostic projection of a Change. Providers render it
// into their own issue/card shape.
type WorkItem struct {
	Slug         string
	Title        string
	Body         string // proposal plus rendered tasks; the provider prepends identity
	Stage        Stage
	Priority     int
	Closed       bool // desired state when ManageClosed is true
	ManageClosed bool // provider must enforce the desired open/closed state
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

// IssueMarkerWriter is an optional, type-asserted provider capability: persist
// the identity marker into an existing item's body. `pull` uses it so a change
// linked to an existing issue stays rediscoverable via Find even if the local
// ref cache is lost. Providers that always embed the marker on write (or model
// identity differently) simply don't implement it.
type IssueMarkerWriter interface {
	// EnsureMarker upserts the identity marker for slug into item id's body,
	// given its current body. It reports whether a write occurred and is
	// idempotent: a body already carrying the marker triggers no write.
	EnsureMarker(ctx context.Context, id, slug, body string) (bool, error)
}

// TaskStateReader is an optional, type-asserted provider capability: report the
// external done-state of a change's tasks, keyed by normalized task text. It
// exists because not every provider models tasks as checkboxes inside one item
// body. The GitHub provider does (one issue, a "## Tasks" checklist), so it
// needs no TaskStateReader — reconcile reads its body via IssueReader and parses
// the checkboxes. Beads instead models one bead per task, so done-state lives in
// per-bead open/closed status; it implements TaskStateReader to surface that.
//
// reconcile prefers a TaskStateReader when present and otherwise falls back to
// the IssueReader+body-parse path, then feeds the resulting map through the same
// merge. existing is the resolved ref (may be nil); a nil/empty result means
// "no external state yet" and reconcile becomes a no-op.
type TaskStateReader interface {
	TaskStates(ctx context.Context, slug string, existing *Ref) (map[string]bool, error)
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

// BoardTarget names an opt-in GitHub ProjectV2 board to project a synced change
// onto, plus the knobs for that projection. A zero BoardTarget (empty Owner)
// means "no board" and MUST result in no board operations at all.
type BoardTarget struct {
	Owner  string // org or user login that owns the project
	Number int    // project number, e.g. 6 in ExopenGitHub/6

	// Assignee is the login to assign; "" or "me"/"@me" means the acting viewer.
	Assignee string

	// StatusMapping overrides the default stage->Status-name mapping per stage.
	// A stage present here is treated as an explicit configuration, so an unknown
	// name fails loud (no positional fallback). Absent stages use the defaults
	// (active/complete -> "In progress", archived -> "Done") with a positional
	// fallback to the board's first non-terminal / terminal option.
	StatusMapping map[Stage]string
}

// Configured reports whether a board target is set. Unset = no board behavior.
func (t BoardTarget) Configured() bool { return t.Owner != "" }

// BoardPlan describes what a board projection did (real run) or would do
// (dry-run). It is the render surface for -dry-run and the human-facing summary.
type BoardPlan struct {
	ProjectID      string
	AlreadyOnBoard bool
	AddedToBoard   bool // an item was (or would be) added via addProjectV2ItemById

	StatusField   string // the resolved Status field name ("Status")
	StatusName    string // the status name specsync set (or would set); "" = left alone
	CurrentStatus string // the board Status before specsync acted
	StatusSkipped string // reason the Status was left unchanged (human curation), if any

	AssigneeLogin string // the login specsync assigned (or would assign); "" = none
	AssignSkipped string // reason the assignee was left unchanged, if any
}

// BoardProjector is an optional, type-asserted provider capability: projecting a
// synced change's issue onto a GitHub ProjectV2 board — detecting membership,
// ensuring the issue is on the board, mapping its stage to the board Status
// field, and assigning it, all idempotently and without clobbering human
// curation. Providers that have no board concept simply don't implement it, and
// sync/pull skip board work entirely. When target is unconfigured this MUST be a
// no-op that issues no board calls.
type BoardProjector interface {
	// ProjectOntoBoard reconciles ref's issue with target. When dryRun is set it
	// performs only read queries and returns the plan it would apply, making no
	// mutation. item.Stage drives the Status mapping.
	ProjectOntoBoard(ctx context.Context, target BoardTarget, ref Ref, item WorkItem, dryRun bool) (BoardPlan, error)
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
