# PR ↔ issue traceability: reference on open, close on completion

## Why

A user merged a PR produced by the specsync workflow and its tracker issue
stayed open, with nothing in the PR pointing back at the issue. Reviewing the
merge later, there is no link in either direction: the PR body never names the
issue, and the issue never learns a PR landed.

specsync today handles one half of this and none of the other:

- **Closing exists**: `WorkItemFor` (sync.go:334) computes
  `Closed: c.Archived || (closeCompleted && c.Stage == StageComplete)` behind
  the `-close-completed` flag, so an issue closes when its change reaches
  complete/archived.
- **PR creation does not exist at all.** Nothing in the repo creates a pull
  request or writes an issue reference into a PR body — `pull.go` is the
  opposite direction (issue → local change). So users hand-write
  `gh pr create`, and the reference is left to memory. It gets forgotten.

## The trap this must avoid

The obvious fix — put `Closes #N` in every PR body — is **wrong for this
workflow**, and shipping it would make things worse than the current gap.

In specsync, **one OpenSpec change = one issue = many PRs**, because a change
is phased. Two live examples from the reporting user's repo:

- `scale-step-planning` (#387): PR #388 landed **Phase 0 only** (benchmark +
  baseline). Phases 1–4 — the actual optimisation — are untouched.
- `fix-empty-ghost-steps` (#389): PR #390 landed **the spec only**. All
  implementation remains.

Had those PRs said `Closes #387` / `Closes #389`, merging would have closed
issues whose work is barely started, silently dropping the remaining phases
from the tracker. The issue represents the *change*, not the *commit*.

So: **reference on open, close on completion.** GitHub's `Refs #N` /
`Part of #N` creates the backlink without triggering auto-close; the existing
stage-derived `-close-completed` remains the only thing that closes an issue.

## What Changes

- **`specsync pr-body -change <slug>`**: emit a PR body fragment containing
  the correct reference line for the change's tracker item — `Part of #N` by
  default, `Closes #N` only when the change's remaining tasks would all be
  complete after this PR. Composable with `gh pr create --body-file -`.
- **Reference-line injection**: when a PR body is supplied to specsync, ensure
  exactly one canonical reference line exists (idempotent — re-running does
  not stack duplicates).
- **`specsync verify` gate**: warn when an open PR whose branch matches a
  change id carries no reference to that change's issue. This is what would
  have caught the reported case at review time rather than after merge.
- **Docs**: state the invariant plainly in SKILL.md and WORKFLOW.md, because
  the failure mode is a human forgetting, and the skill file is what agents
  read: *PRs reference, completion closes.*

## Impact

- New subcommand + verify check; no change to existing sync/close semantics.
- Multi-provider: the reference syntax differs per provider (GitHub
  `Refs #N`, GitLab `Related to #N`), so the line must come from the provider
  layer rather than being hardcoded.

## Out of scope

- Creating PRs. specsync should emit the body and let `gh`/`glab` own PR
  creation; wrapping those is a larger surface than this problem needs.
