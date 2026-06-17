# Epic & sub-issue projection

## Why

Larger efforts span multiple repos and phases. A common pattern is one "epic"
issue that coordinates several sub-issues, where each sub-issue is the unit that
gets a branch, a worktree, and — once implementation starts — a focused spec.
specsync should understand this hierarchy so an epic stays a coordination shell
while each sub-issue maps one-to-one to an OpenSpec change.

## What

- Recognize an epic by a convention (a label/marker such as `type:epic`), not by
  having a spec of its own — an epic is a list/coordination issue.
- Map each sub-issue to one OpenSpec change (one change ⇄ one sub-issue).
- Keep the epic's body in sync with the set of sub-issues (checklist or links),
  while each sub-issue's body is driven by its change's proposal + tasks.

## Scope

- Epic detection convention + a `parent` link on a change.
- Sub-issue projection: child change ⇄ sub-issue.
- Epic body roll-up of its sub-issues.
