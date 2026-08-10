# Proposal: Correct the documented Go import path and guard it

## Why

`openspec/changes/archive/2026-07-25-specsync-go-library` is archived with **all
nine tasks checked**, including:

> - [x] Create `pkg/specsync/` directory and move all root `.go` files into it
> - [x] Update `cmd/specsync/main.go` to import `pkg/specsync` instead of root
> - [x] Move all `*_test.go` files to `pkg/specsync/`

None of that happened. There is no `pkg/` directory; the package is at the
module root (`module github.com/androidand/specsync`, `audit.go`, `board.go`,
`provider.go` … all top-level). The change shipped a working library — just not
at the path its own record claims.

This is not a cosmetic docs bug. The archived record is the reference a consumer
reads, and one already has: skein's `specsync-library-integration` proposal
instructs implementers to import
`github.com/androidand/specsync/pkg/specsync`, which does not resolve. Its
`tasks.md` happens to use the correct root path, so the two disagree inside one
change. An archived task list that asserts a filesystem layout the repository
does not have will keep producing broken imports.

There is also nothing in the test suite that would notice. `boundary_test.go`
enforces stdlib-only, but no test asserts where the package lives or what it
exports, so the drift was silent and stayed silent.

## What

- **Correct the archived record** so it describes what shipped: a library at the
  module root, not at `pkg/specsync/`. Archived changes are the project's
  history; a false one is worse than none.
- **State the canonical import path once**, in the package documentation, so
  consumers have a single authoritative answer.
- **Guard it with a test** that fails if the package moves or the documented
  path stops resolving — the same reason the stdlib boundary has a test rather
  than a convention.
- **Flag the downstream consumer.** skein's proposal prose needs the same
  correction; that fix belongs in skein's repo, linked from here.

## Scope

**In scope**
- The archived `specsync-go-library` task/proposal text.
- Package-level documentation of the import path.
- A test guarding package location and the exported surface consumers rely on.

**Not in scope**
- Actually moving the package to `pkg/specsync/`. Root is a legitimate Go layout,
  it is what shipped, and consumers already build against it — relocating now
  would break them to satisfy a task list that was wrong in the first place.
- Editing skein's repository from here.

## Non-Goals

- Not a redesign of the public API. This records the surface, it does not change
  it.
- Not a general audit of every archived change, though the same question is
  worth asking elsewhere.

## Risks

- **Rewriting history can hide it.** Mitigated by correcting the archived tasks
  with an explicit note that the move was specced and deliberately not done,
  rather than silently deleting the claims.
- **A guard test can ossify the layout.** Accepted: if the package is ever moved
  on purpose, updating one test is the correct cost, and it forces the
  documentation to move with it.
