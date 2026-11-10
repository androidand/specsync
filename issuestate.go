package specsync

import "context"

// IssueState is the open or closed state of a change's issue. State is "open",
// "closed" or "unknown"; Reason is set for a closed issue when the provider
// reports one ("completed", "not planned", "duplicate").
type IssueState struct {
	State  string
	Reason string
}

// IssueStateReader is an optional, type-asserted provider capability: read the
// state of many issues in one call. A ref the provider cannot resolve is
// reported as "unknown"; providers without it leave every issue unknown.
type IssueStateReader interface {
	IssueStates(ctx context.Context, refs []Ref) (map[string]IssueState, error)
}

// IssueStateKey identifies one ref in an IssueStates result.
func IssueStateKey(r Ref) string { return r.Provider + "#" + r.ID }

// ChangeIssueRef returns the issue ref cached for a change, with Provider set
// to the cache key. ok is false when the change has no cached ref.
func ChangeIssueRef(changeDir string) (Ref, bool) {
	refs, err := LoadRefs(changeDir)
	if err != nil || len(refs) == 0 {
		return Ref{}, false
	}
	key, ref := firstRef(refs)
	if ref.ID == "" {
		return Ref{}, false
	}
	ref.Provider = key
	return ref, true
}

// ChangeDrift names the mismatch between a change and its issue, or "". It
// reports two mismatches and nothing else: "closed-unarchived" (the issue is
// closed, the change is not archived) and "complete-open" (the change has at
// least one task, every task is checked, the issue is open).
func ChangeDrift(c Change, st IssueState) string {
	switch st.State {
	case "closed":
		if !c.Archived {
			return "closed-unarchived"
		}
	case "open":
		if total, done := CountCheckboxes(c.TasksMarkdown); total > 0 && done == total {
			return "complete-open"
		}
	}
	return ""
}
