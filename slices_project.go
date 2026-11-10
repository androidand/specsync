package specsync

import (
	"context"
	"fmt"
	"strings"
)

// SliceProjector is an optional provider capability: nest one issue under
// another as a sub-issue, and record that one issue is blocked by another. A
// provider without it gets the slices listed in the change's issue body.
type SliceProjector interface {
	LinkSubIssue(ctx context.Context, parent, child Ref) error
	AddBlockedByRef(ctx context.Context, issue, blocker Ref) error
}

// SliceProjection reports what projecting a change's slices did, or on a dry
// run would do.
type SliceProjection struct {
	Actions  []string
	Children map[int]Ref
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
			fmt.Fprintf(&b, " (after %s)", joinSliceNumbers(s.After))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func joinSliceNumbers(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprint(n)
	}
	return strings.Join(parts, ", ")
}

// ProjectSlices projects a change's declared slices through the provider: one
// child issue per slice, each nested under the change's issue, and each `after`
// as a blocked-by edge. Children are found again by their slug marker, and a
// child is nested and wired only when it is created, so a second run changes
// nothing. A change without slices projects nothing. specsync never adds a
// slice the author did not declare.
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

	created := map[int]bool{}
	for _, s := range plan.Slices {
		slug := sliceChildSlug(c.Slug, s.N)
		existing, err := findWithRetry(ctx, func(ctx context.Context) (*Ref, error) { return prov.Find(ctx, slug) })
		if err != nil {
			return res, fmt.Errorf("find slice %d: %w", s.N, err)
		}
		ref, err := prov.Push(ctx, WorkItem{
			Slug:   slug,
			Title:  sliceChildTitle(c, s),
			Body:   sliceChildBody(c, s, parent),
			Labels: []string{"specsync", "type:slice"},
		}, existing)
		if err != nil {
			return res, fmt.Errorf("push slice %d: %w", s.N, err)
		}
		res.Children[s.N] = ref
		created[s.N] = existing == nil
		verb := "updated"
		if existing == nil {
			verb = "created"
		}
		res.Actions = append(res.Actions, fmt.Sprintf("slice %d: %s %s", s.N, verb, ref.URL))
		if existing == nil {
			if err := proj.LinkSubIssue(ctx, parent, ref); err != nil {
				return res, fmt.Errorf("nest slice %d under %s: %w", s.N, parent.URL, err)
			}
		}
	}
	for _, s := range plan.Slices {
		if !created[s.N] {
			continue
		}
		for _, a := range s.After {
			blocker, ok := res.Children[a]
			if !ok {
				continue
			}
			if err := proj.AddBlockedByRef(ctx, res.Children[s.N], blocker); err != nil {
				return res, fmt.Errorf("slice %d blocked by slice %d: %w", s.N, a, err)
			}
			res.Actions = append(res.Actions, fmt.Sprintf("slice %d: blocked by slice %d", s.N, a))
		}
	}
	return res, nil
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
		fmt.Fprintf(&b, "\n\nAfter slice %s.", joinSliceNumbers(s.After))
	}
	return b.String()
}
