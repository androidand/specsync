package specsync

import (
	"context"
	"fmt"
	"strings"
)

// SliceProjector is an optional provider capability: nest one issue under
// another as a sub-issue, record that one issue is blocked by another, and read
// both back so a run can skip what already exists. A provider without it gets
// the slices listed in the change's issue body.
type SliceProjector interface {
	LinkSubIssue(ctx context.Context, parent, child Ref) error
	IsSubIssue(ctx context.Context, parent, child Ref) (bool, error)
	AddBlockedByRef(ctx context.Context, issue, blocker Ref) error
	BlockedByRefs(ctx context.Context, issue Ref) ([]Ref, error)
}

// SliceProjection reports what projecting a change's slices did, or on a dry
// run would do.
type SliceProjection struct {
	Actions  []string
	Children map[int]Ref
	// Failed names each step that did not complete; the next run retries them.
	Failed []string
	// Fallback is true when the provider cannot nest issues; the slices belong
	// in the change's issue body instead (see SlicesSection).
	Fallback bool
}

func sliceChildSlug(changeSlug string, n int) string {
	return fmt.Sprintf("%s:slice-%d", changeSlug, n)
}

// SlicesSection lists a plan's slices for an issue body, or "" without slices.
func SlicesSection(plan SlicePlan) string {
	if len(plan.Slices) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Slices\n\n")
	for _, s := range plan.Slices {
		fmt.Fprintf(&b, "- Slice %d: %s", s.N, s.Title)
		if s.Contract != "" {
			fmt.Fprintf(&b, " — %s", s.Contract)
		}
		if len(s.After) > 0 {
			fmt.Fprintf(&b, " (after %s)", JoinSliceNumbers(s.After))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// JoinSliceNumbers renders slice numbers as "1, 2".
func JoinSliceNumbers(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprint(n)
	}
	return strings.Join(parts, ", ")
}

// ProjectSlices projects a change's declared slices through the provider: one
// child issue per slice, each nested under the change's issue, and each `after`
// as a blocked-by edge. It heals on every run: a child is found by its slug
// marker, and a missing nesting or edge is added while an existing one is left
// alone, so a failed step is retried next time and a second run changes
// nothing. Every failure is collected and returned together. A change without
// slices projects nothing. specsync never adds a slice the author did not
// declare.
func ProjectSlices(ctx context.Context, c Change, parent Ref, prov WorkProvider, dryRun bool) (SliceProjection, error) {
	res := SliceProjection{Children: map[int]Ref{}}
	plan := ParseSlices(c.TasksMarkdown)
	if len(plan.Slices) == 0 {
		return res, nil
	}
	proj, ok := prov.(SliceProjector)
	if !ok {
		res.Fallback = true
		return res, nil
	}

	if dryRun {
		for _, s := range plan.Slices {
			res.Actions = append(res.Actions, fmt.Sprintf("slice %d: create or update child issue %q, sub-issue of %s", s.N, sliceChildTitle(c, s), parent.URL))
		}
		for _, s := range plan.Slices {
			for _, a := range s.After {
				res.Actions = append(res.Actions, fmt.Sprintf("slice %d: blocked by slice %d", s.N, a))
			}
		}
		return res, nil
	}

	fail := func(format string, args ...any) {
		res.Failed = append(res.Failed, fmt.Sprintf(format, args...))
	}
	for _, s := range plan.Slices {
		existing, err := prov.Find(ctx, sliceChildSlug(c.Slug, s.N))
		if err != nil {
			fail("slice %d: find child issue: %v", s.N, err)
			continue
		}
		ref, err := prov.Push(ctx, WorkItem{
			Slug:   sliceChildSlug(c.Slug, s.N),
			Title:  sliceChildTitle(c, s),
			Body:   sliceChildBody(c, s, parent),
			Labels: []string{"specsync", "type:slice"},
		}, existing)
		if err != nil {
			fail("slice %d: push child issue: %v", s.N, err)
			continue
		}
		res.Children[s.N] = ref
		verb := "updated"
		if existing == nil {
			verb = "created"
		}
		res.Actions = append(res.Actions, fmt.Sprintf("slice %d: %s %s", s.N, verb, ref.URL))
	}
	for _, s := range plan.Slices {
		child, ok := res.Children[s.N]
		if !ok {
			continue
		}
		nested, err := proj.IsSubIssue(ctx, parent, child)
		if err != nil {
			fail("slice %d: read whether it is nested under %s: %v", s.N, parent.URL, err)
		} else if !nested {
			if err := proj.LinkSubIssue(ctx, parent, child); err != nil {
				fail("slice %d: nest under %s: %v", s.N, parent.URL, err)
			} else {
				res.Actions = append(res.Actions, fmt.Sprintf("slice %d: nested under %s", s.N, parent.URL))
			}
		}
		if len(s.After) == 0 {
			continue
		}
		have, err := proj.BlockedByRefs(ctx, child)
		if err != nil {
			fail("slice %d: read blocked-by edges: %v", s.N, err)
			continue
		}
		for _, a := range s.After {
			blocker, ok := res.Children[a]
			if !ok {
				continue
			}
			if refsContain(have, blocker) {
				continue
			}
			if err := proj.AddBlockedByRef(ctx, child, blocker); err != nil {
				fail("slice %d: blocked by slice %d: %v", s.N, a, err)
				continue
			}
			res.Actions = append(res.Actions, fmt.Sprintf("slice %d: blocked by slice %d", s.N, a))
		}
	}
	if len(res.Failed) > 0 {
		return res, fmt.Errorf("%d slice projection step(s) failed: %s", len(res.Failed), strings.Join(res.Failed, "; "))
	}
	return res, nil
}

func refsContain(refs []Ref, want Ref) bool {
	for _, r := range refs {
		if (r.URL != "" && r.URL == want.URL) || (r.URL == "" && r.ID == want.ID) {
			return true
		}
	}
	return false
}

func sliceChildTitle(c Change, s Slice) string {
	return fmt.Sprintf("%s — slice %d: %s", c.Title, s.N, s.Title)
}

func sliceChildBody(c Change, s Slice, parent Ref) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Slice %d of %s", s.N, parent.URL)
	if s.Contract != "" {
		fmt.Fprintf(&b, "\n\nContract: %s", s.Contract)
	}
	if s.Budget != "" {
		fmt.Fprintf(&b, "\n\nBudget: %s", s.Budget)
	}
	if len(s.After) > 0 {
		fmt.Fprintf(&b, "\n\nAfter slice %s.", JoinSliceNumbers(s.After))
	}
	return b.String()
}
