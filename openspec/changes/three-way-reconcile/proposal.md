# 3-way reconcile and stable task IDs

Two-way reconcile (v1) uses monotonic union: checked always wins, unchecked never propagates from the issue. Two limitations remain:

1. **Un-checking a task on the issue does not propagate** — because v1 has no stored base state, it can't distinguish "human unchecked" from "was already unchecked". A 3-way merge with a stored base state would solve this.
2. **Wording changes in `tasks.md` break text matching** — if the spec author rewrites a task line, the match fails and state from the issue can't co-apply. A stable task ID (e.g., hash of original text) would decouple matching from wording.

This change addresses both by storing a base snapshot per task and using a stable key for matching.

## Out of scope

- Real-time sync (specsync is invoke-and-exit)
- Cross-provider state merging (each provider reconciles independently against OpenSpec)
