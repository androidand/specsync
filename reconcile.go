package specsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

	// Try 3-way merge if base state is available; fall back to 2-way union.
	merged, flips, used3way, err := reconcileThreeWay(ctx, c, states, ref)
	if err != nil {
		merged, flips = mergeTaskState(c.TasksMarkdown, states)
		used3way = false
	} else if !used3way {
		merged, flips = mergeTaskState(c.TasksMarkdown, states)
	}

	// Save base state regardless of whether flips occurred, so next sync can
	// do a proper 3-way merge.
	ref.BaseSHA = currentTaskSHA(c)
	ref.Base = c.TasksMarkdown

	if len(flips) > 0 {
		c.TasksMarkdown = merged
		if err := os.WriteFile(filepath.Join(c.Dir, "tasks.md"), []byte(c.TasksMarkdown), 0o644); err != nil {
			return ref, nil, err
		}
	}

	if err := saveBaseState(c.Dir, ref); err != nil {
		return ref, nil, fmt.Errorf("save base state: %w", err)
	}
	return ref, flips, nil
}

// reconcileThreeWay attempts a 3-way merge using the base state from the ref.
// If base state is available, it performs a 3-way merge. Returns the merged
// markdown, flips, whether 3-way was used, and any error.
func reconcileThreeWay(ctx context.Context, c *Change, issue map[string]bool, ref *Ref) (string, []TaskFlip, bool, error) {
	if ref.Base == "" {
		return "", nil, false, nil
	}

	baseStates := parseTaskStates(ref.Base)

	lines := strings.Split(c.TasksMarkdown, "\n")
	var flips []TaskFlip
	for i, line := range lines {
		text, checked, ok := parseTaskLine(line)
		if !ok {
			continue
		}

		issueChecked, issuePresent := issue[text]
		baseChecked, basePresent := baseStates[text]

		if !issuePresent {
			continue
		}

		// 3-way merge: only apply issue changes relative to base.
		if !basePresent {
			continue // task was not in base — skip
		}

		// If issue checked what was unchecked in base, propagate.
		if issueChecked && !baseChecked {
			if !checked {
				lines[i] = setTaskChecked(line, true)
				flips = append(flips, TaskFlip{Text: text, Checked: true})
			}
		}
		// If issue unchecked what was checked in base, propagate.
		if !issueChecked && baseChecked {
			if checked {
				lines[i] = setTaskChecked(line, false)
				flips = append(flips, TaskFlip{Text: text, Checked: false})
			}
		}
	}

	return strings.Join(lines, "\n"), flips, true, nil
}

// currentTaskSHA returns the SHA-256 of the current tasks.md content.
func currentTaskSHA(c *Change) string {
	h := sha256.Sum256([]byte(c.TasksMarkdown))
	return hex.EncodeToString(h[:])
}

// saveBaseState persists the base tasks.md content to the ref cache.
func saveBaseState(changeDir string, ref *Ref) error {
	refs, err := loadRefs(changeDir)
	if err != nil {
		return err
	}
	r, ok := refs[ref.Provider]
	if ok {
		r.BaseSHA = ref.BaseSHA
		r.Base = ref.Base
		refs[ref.Provider] = r
	} else {
		refs[ref.Provider] = Ref{
			Provider: ref.Provider,
			ID:       ref.ID,
			URL:      ref.URL,
			BaseSHA:  ref.BaseSHA,
			Base:     ref.Base,
		}
	}
	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0o755); err != nil {
		return fmt.Errorf("create .specsync: %w", err)
	}
	b, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ref cache: %w", err)
	}
	if err := os.WriteFile(refCachePath(changeDir), append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write ref cache: %w", err)
	}
	return nil
}

// parseTaskStates extracts task checkbox state from tasks markdown.
func parseTaskStates(md string) map[string]bool {
	states := map[string]bool{}
	for _, line := range strings.Split(md, "\n") {
		if text, checked, ok := parseTaskLine(line); ok {
			states[text] = checked
		}
	}
	return states
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
