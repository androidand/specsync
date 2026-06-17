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
