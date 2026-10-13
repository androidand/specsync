# Design: release/claim must not create a phantom change directory

## The defect

`runRelease` (cmd/specsync/main.go:2224) and `runClaim` (cmd/specsync/main.go:2177)
build the change path as `filepath.Join(abs, "changes", *change)` and hand it to
`specsync.SaveChangeMetadata(dir, …)`. `SaveChangeMetadata` calls
`os.MkdirAll(filepath.Join(dir, ".specsync"))` unconditionally, and `MkdirAll`
creates every missing ancestor. So:

- a typo'd `-change` slug, or
- an `-openspec` that resolves to a directory with no `changes/` child (the default
  relative `-openspec openspec` run from inside `openspec/` resolves to
  `openspec/openspec/changes/<slug>`)

produces a **new, empty change directory** plus a `metadata.json` inside it, and the
command exits 0 because `SaveChangeMetadata` returns no error when it is the one that
created the path. That is the 2026-10-03 estate-wide `openspec/openspec/` artifact.

## The fix — load before you write

`mutableChange(openspecDir, slug, allowArchived)` is the shared guard every
metadata-mutating subcommand already uses. It validates the slug, loads the change via
`LoadChangeBySlug`, and `fail`s ("change not found") when the change is absent.
`fail` calls `os.Exit(1)`, so the command stops before any write.

`runClaim` and `runRelease` now call `mutableChange(abs, *change, false)` **before**
`SaveChangeMetadata`. A bad slug now behaves like every other mutating command: exit 1,
no disk change.

## Boundary: the guard is at the caller, not the writer

`SaveChangeMetadata` stays a raw writer. It is called by paths that have *just created*
the directory themselves (set-stage/set-priority scaffolding) and by tests that write
into freshly-made dirs; for those, `MkdirAll` is the intended behaviour. The contract
"operate only on an existing change" belongs to the commands that *claim* to move an
existing change — `release`, `claim`, and the close-state paths that already use
`mutableChange`. Moving the check into `SaveChangeMetadata` would break those callers
and hide their real errors, so it stays out.

## Taxonomy of a bad invocation (post-fix)

| Invocation                                   | Result                                    |
|----------------------------------------------|-------------------------------------------|
| `-change <typo>` (no such change)            | `mutableChange` fails → exit 1, no write  |
| `-openspec <path without changes/>`          | `isChangesDir(changes/)` check → exit 1   |
| `-openspec <nonexistent path>`               | `isChangesDir(changes/)` check → exit 1   |
| `-change <real slug>`                        | proceeds; may still fail later at sync    |

The single guard (`isChangesDir`) catches both a missing `-openspec` and an
`-openspec` with no `changes/` child in one check: a valid `-openspec` is the
openspec *directory* itself (default `"openspec"`), so the check is that
`<openspec>/changes` exists and is a directory, not that `-openspec` is a file.
An earlier iteration used a `fileExists` predicate that wrongly rejected a valid
directory `-openspec`; `isChangesDir` on the `changes/` child is the correct
shape and refuses exactly the artifact-producing invocations.
