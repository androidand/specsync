package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// TaskFlip records one task whose checkbox changed during reconcile, carrying
// the new (post-merge) checked state. Used to report what a sync changed.
type TaskFlip struct {
	Text    string
	Checked bool
}

// reconcileTaskState merges the issue's task-list checkbox state back into the
// local tasks.md before the change is pushed — the inbound half of two-way sync,
// implementing the "checkbox state is authoritative on the issue side" rule.
//
// The merge is a monotonic union: a matching task ends up checked if either side
// has it checked. This captures boxes ticked on the issue (the whole point)
// without ever reverting local progress that has not been pushed yet — the bug a
// naive "issue always wins" hits when the issue lags an un-pushed local edit.
// Un-checking via the issue is therefore not propagated; that is a deliberate v1
// limitation (a 3-way merge against a stored base state would be needed for it).
//
// Only "- [ ]" / "- [x]" lines are touched, matched by normalized text. Task
// wording, ordering, and every other line — including living-plan's [~]/[>]
// markers and any preserved proposal sections — are left exactly as authored, so
// the spec still wins the plan. Returns the resolved issue ref so the caller can
// reuse it for the push instead of resolving twice.
func reconcileTaskState(ctx context.Context, prov WorkProvider, c *Change, existing *Ref) (resolved *Ref, flips []TaskFlip, err error) {
	if strings.TrimSpace(c.TasksMarkdown) == "" {
		return existing, nil, nil
	}

	// Resolve the ref once, rebuilding it from the identity marker if the cache
	// lacks it. Both the state-source paths below and the caller's subsequent
	// push reuse it, so a marker lookup never happens twice.
	ref := existing
	if ref == nil {
		found, ferr := prov.Find(ctx, c.Slug)
		if ferr != nil {
			return existing, nil, ferr
		}
		ref = found
	}

	states, err := externalTaskStates(ctx, prov, c.Slug, ref)
	if err != nil {
		return ref, nil, err
	}
	if len(states) == 0 {
		return ref, nil, nil
	}

	merged, flips := mergeTaskState(c.TasksMarkdown, states)
	if len(flips) == 0 {
		return ref, nil, nil
	}
	c.TasksMarkdown = merged
	if err := os.WriteFile(filepath.Join(c.Dir, "tasks.md"), []byte(merged), 0o644); err != nil {
		return ref, nil, err
	}
	return ref, flips, nil
}

// externalTaskStates obtains task done-state from whichever capability the
// provider supports, returning it keyed by normalized task text so the shared
// mergeTaskState can consume it unchanged. A TaskStateReader (Beads: one bead
// per task, status is the state) is preferred; otherwise an IssueReader's body
// carries the rendered "## Tasks" checklist (GitHub) and parseIssueTaskStates
// reads it. A provider with neither capability — or a GitHub issue that does not
// exist yet (ref == nil) — yields no state, making reconcile a no-op. This is
// the single point where state acquisition differs across providers; the merge,
// the flip detection, and the tasks.md write are all shared below.
func externalTaskStates(ctx context.Context, prov WorkProvider, slug string, ref *Ref) (map[string]bool, error) {
	if tsr, ok := prov.(TaskStateReader); ok {
		return tsr.TaskStates(ctx, slug, ref)
	}
	reader, ok := prov.(IssueReader)
	if !ok || ref == nil {
		return nil, nil
	}
	item, err := reader.Get(ctx, ref.ID)
	if err != nil {
		return nil, err
	}
	return parseIssueTaskStates(item.Body), nil
}

// parseIssueTaskStates extracts the ## Tasks checkbox state from an issue body,
// keyed by normalized task text. It reuses splitBody so it sees exactly the
// managed Tasks section specsync renders; only [ ]/[x] lines are recorded.
func parseIssueTaskStates(body string) map[string]bool {
	_, tasks, _ := splitBody(body, "")
	states := map[string]bool{}
	for _, line := range strings.Split(tasks, "\n") {
		if text, checked, ok := parseTaskLine(line); ok {
			states[text] = checked
		}
	}
	return states
}

// mergeTaskState applies the union rule to local tasks markdown given the issue
// states, returning the merged markdown and the flips it made.
func mergeTaskState(local string, issue map[string]bool) (string, []TaskFlip) {
	lines := strings.Split(local, "\n")
	var flips []TaskFlip
	for i, line := range lines {
		text, checked, ok := parseTaskLine(line)
		if !ok {
			continue
		}
		issueChecked, present := issue[text]
		if !present {
			continue // task added locally, or wording changed — spec keeps its line
		}
		if union := checked || issueChecked; union != checked {
			lines[i] = setTaskChecked(line, union)
			flips = append(flips, TaskFlip{Text: text, Checked: union})
		}
	}
	return strings.Join(lines, "\n"), flips
}

// parseTaskLine parses a "- [ ] text" / "- [x] text" task line, returning the
// normalized text and checked state. ok is false for non-task lines and for
// other checkbox markers (e.g. living-plan's [~]/[>]), which stay untouched.
func parseTaskLine(line string) (text string, checked, ok bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "- [") || len(t) < 6 || t[4] != ']' {
		return "", false, false
	}
	switch t[3] {
	case ' ':
		checked = false
	case 'x', 'X':
		checked = true
	default:
		return "", false, false
	}
	return normalizeTaskText(t[5:]), checked, true
}

// setTaskChecked rewrites the checkbox mark of a task line in place, preserving
// indentation and the task text exactly.
func setTaskChecked(line string, checked bool) string {
	i := strings.Index(line, "- [")
	if i < 0 || i+3 >= len(line) {
		return line
	}
	b := []byte(line)
	if checked {
		b[i+3] = 'x'
	} else {
		b[i+3] = ' '
	}
	return string(b)
}

// normalizeTaskText collapses internal whitespace so trivially-reformatted task
// lines still match across the two sides.
func normalizeTaskText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
