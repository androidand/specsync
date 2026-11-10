# Warn when a pull request is too large or mixes concerns

## Why

Review quality collapses when a change request is too large or when it mixes unrelated
work, and the problem is only noticed when a reviewer gives up. A real case: a single
request with 106 files, about 4,000 changed lines and 72 commits, containing a refactor,
an unrelated feature, tests, translations and the behaviour change that was actually
being shipped. No tool said so before review began.

The facts needed to say so early are already known locally: files, lines and commits
against the base are one `git diff --shortstat` away, and the *concerns* mixed in a
branch are visible in its commit messages when the project labels commits with a scope.
specsync already reports on work in flight and already verifies that change requests
reference their changes. It does not report size, so an oversized request is invisible
until it is expensive.

## What Changes

- **Size facts** come from git (`local.size`, see `status-pr-and-worktree-state`),
  with the provider's figures shown when it supplies them. No tracker is required.
- **Three attention flags**, added to the fixed vocabulary:
  - `oversized`: files, changed lines or commits exceed the budget;
  - `mixed-scopes`: the branch's commits carry more than one distinct conventional
    scope (`feat(a): ...`, `fix(b): ...`), listing the scopes. Asserted from commit
    messages only; branches that do not use scopes are never flagged.
  - `no-slice-plan`: the branch is oversized and its change declares no slices.
- **Budget** defaults are conservative numbers documented as advisory (files, changed
  lines, commits), overridable per change by a slice `budget:` line, per call by
  `-budget files=15,lines=400,commits=30`, and per environment by `SPECSYNC_BUDGET`.
- **`verify -size`** applies the same checks to a named branch or request and prints the
  reasons. It exits 0 by default; `-fail-on-oversized` makes it fail, for projects that
  want a CI gate.
- **`status`** shows the flags beside the row, so the signal appears without asking.

## Generality

- Works with any or no tracker, any language and any repository layout; it needs only
  git. The defaults are plain numbers a team can change; they are policy, not a claim
  about the right size.
- Scope detection is opt-in by convention: no scopes, no `mixed-scopes`.

## Non-goals

- It never blocks a merge unless the project asks for `-fail-on-oversized`.
- It does not judge quality, and it does not split anything. Splitting is
  `plan-changes-as-slices`.
- Generated files and lockfiles are counted as git reports them; excluding them is a
  budget option (`-exclude <glob>`), not an assumption.

## Impact

- A size and scope analyser over the existing `local` block; flags in the attention
  vocabulary; `verify -size`; budget parsing.
- Docs: README, `agent-help`, the skill, and `site/features.json`.

## Release note

`specsync status` and `specsync verify -size` now flag a branch that is too large or that
mixes unrelated concerns, before review starts.
