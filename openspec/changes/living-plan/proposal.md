# Living plan

The plan changes constantly while work happens: tasks get added, scrapped,
replaced, or moved elsewhere. Today `tasks.md` is a flat checklist, so this churn
is invisible — a reviewer sees only the current boxes, not that the plan evolved
or *why*. And capturing a mid-work discovery means stopping to decide where it
goes, which breaks flow, so discoveries get dropped.

This change makes the plan's evolution **legible and cheap to capture**, without
demanding up-front planning and without noisy diffs. It is the task-level analog
of OpenSpec's intent history: not "what the patch was" but "how the plan moved."

## Solution

**1. Discovery capture — a holding pen.** A `## Discoveries` section agents
append to mid-work without triaging:

```
specsync note -slug <slug> "auth refresh races with logout"
```

It is a managed section (rendered into the issue like `## Tasks` / `## Related`,
stripped on `pull` so it never pollutes `proposal.md`). Capture now, triage later
— promote a discovery to a task (in scope) or `specsync spinoff` it (out of
scope).

**2. Preserved original intent.** When a change starts from an issue, `pull`
seeds an `## Original ask` block in `proposal.md` from the first issue body.
specsync treats it as **read-only on sync** — never overwritten on push or
re-pull, only displayed — so the spec winning everything else can never quietly
erase the intent you originally agreed to. When intent genuinely shifts mid-work,
you append a dated *"Revised scope: … (was: …)"* note rather than mutating the
block. That is the intent-history anchor the two-way reconcile policy depends on.

**3. Churn-legible task states.** Extend the checkbox vocabulary so scrapped and
moved tasks stay visible instead of vanishing from git:

```
- [ ] todo
- [x] done
- [~] dropped: superseded by the new client
- [>] moved: rate-limit-bug
```

specsync renders the live checklist from `[ ]`/`[x]` only, and adds a compact
**Plan changes** footer to the issue — e.g. `+3 added · 2 done · 1 dropped · 1
moved` — so a reviewer sees the plan evolved and why, at a glance. Dropped and
moved lines never count against progress.

## Non-goals

- Full task database, dependencies, or `bd ready` semantics — that is Beads, via
  `beads-memory-bridge`. This change stays plain-markdown and render-only.
- Two-way reconcile of checkbox state edited on the issue — separate change
  (`two-way-reconcile`); this change only adds the `## Original ask` anchor it
  relies on.
- Deciding scope for a discovery — that is the human/agent's call at triage.
