package specsync

import (
	"context"
	"errors"
	"testing"
)

// findStub answers Find per-slug from a fixed map; never used for Push/Name in
// these tests. found[slug] absent means Find returns (nil, nil) — no evidence.
type findStub struct {
	found map[string]Ref
	err   error
}

func (f findStub) Name() string                                       { return "github" }
func (f findStub) Push(context.Context, WorkItem, *Ref) (Ref, error)  { return Ref{}, nil }
func (f findStub) Find(_ context.Context, slug string) (*Ref, error) {
	if f.err != nil {
		return nil, f.err
	}
	if r, ok := f.found[slug]; ok {
		return &r, nil
	}
	return nil, nil
}

func TestResolveLiveRefsFillsOnlyMissing(t *testing.T) {
	in := TraceInput{Changes: []ChangeRefs{
		{Change: Change{Slug: "cached"}, IssueIDs: []string{"1"}},
		{Change: Change{Slug: "uncached"}},
		{Change: Change{Slug: "no-evidence"}},
	}}
	resolver := findStub{found: map[string]Ref{
		"cached":   {Provider: "github", ID: "999"}, // must NOT overwrite the cached id
		"uncached": {Provider: "github", ID: "42"},
	}}

	if err := ResolveLiveRefs(context.Background(), &in, resolver); err != nil {
		t.Fatalf("ResolveLiveRefs: %v", err)
	}

	if got := in.Changes[0].IssueIDs; len(got) != 1 || got[0] != "1" {
		t.Fatalf("cached change's IssueIDs changed: %v", got)
	}
	if got := in.Changes[1].IssueIDs; len(got) != 1 || got[0] != "42" {
		t.Fatalf("uncached change not resolved: %v", got)
	}
	if got := in.Changes[2].IssueIDs; len(got) != 0 {
		t.Fatalf("no-evidence change fabricated a link: %v", got)
	}
}

func TestResolveLiveRefsPropagatesFindError(t *testing.T) {
	in := TraceInput{Changes: []ChangeRefs{{Change: Change{Slug: "boom"}}}}
	resolver := findStub{err: errors.New("network down")}

	if err := ResolveLiveRefs(context.Background(), &in, resolver); err == nil {
		t.Fatal("expected an error from a failing Find")
	}
}
