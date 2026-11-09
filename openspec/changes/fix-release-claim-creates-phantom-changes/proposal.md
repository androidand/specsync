# release/claim must not create a phantom change directory

## Why

A `specsync release -change <slug>` or `specsync claim -change <slug>` whose slug is
wrong (a typo, or a slug that never existed) silently creates a directory that should
never exist, and reports success.

Reproduced 2026-10-03: ~40 repositories across the estate picked up an untracked
`openspec/openspec/` directory, all created within about a second
(09:59:07–08). The artifact is:

```
openspec/openspec/changes/<slug>/.specsync/metadata.json
{"version":1,"stage":"backlog"}
```

The chain is:

- `runRelease` (cmd/specsync/main.go:2224) and `runClaim` (cmd/specsync/main.go:2166)
  build the change directory with `filepath.Join(abs, "changes", *change)` and call
  `specsync.SaveChangeMetadata(dir, …)`.
- `SaveChangeMetadata` (change.go:534) does `os.MkdirAll(filepath.Join(dir, ".specsync"))`
  unconditionally. `MkdirAll` creates every missing ancestor.
- If `-change` is a typo, or `-openspec` resolves to a directory that has no `changes/`
  (the default relative `-openspec openspec` run from inside `openspec/` resolves to
  `openspec/openspec/changes/<slug>`), the target change directory does not exist, and
  the metadata write creates it. The command then exits 0.

Two things go wrong at once:

1. **The command writes into a path it never validated.** `SaveChangeMetadata` has no
   knowledge of whether the change exists; it is a raw directory writer. The callers
   treat "save the metadata" as if it were "update the change", but they never loaded
   the change first.
2. **A wrong slug is indistinguishable from a successful update.** Both commands call
   `fail(err)` only if `SaveChangeMetadata` returns an error. A typo produces no error
   — it produces a new directory — so the runner is told the item is locked/released
   when nothing was.

The installed binary on PATH does not even ship `release`/`claim` (they are being
re-added as uncommitted work in this tree), so the exact process that created the
2026-10-03 artifacts is not the one a fresh install runs. But the code path is live and
the same, and any future invocation with a bad slug repeats the artifact.

## What Changes

- `runClaim` and `runRelease` load the change with the existing guard
  `mutableChange(openspecDir, slug, false)` **before** calling `SaveChangeMetadata`.
  `mutableChange` already validates the slug, loads the change via
  `LoadChangeBySlug`, and `fail`s with "change not found" when it is absent — so a
  typo now exits 1 and changes nothing on disk, exactly like every other
  metadata-mutating subcommand.
- `release` and `claim` reject an `-openspec` that is not a directory with a
  `changes/` child. The default `-openspec openspec` run from inside `openspec/` is the
  case that produced the artifacts (it resolved to `openspec/openspec/`); the command
  should refuse that shape rather than writing into it.

## Impact

- Two functions in `cmd/specsync/main.go` gain a load-before-write step and an
  `-openspec` existence check.
- `SaveChangeMetadata` is unchanged: it is a raw writer, and other callers
  (set-stage/priority, tests) legitimately write into paths they have just created or
  that already exist. The fix belongs at the callers that claim to operate on an
  existing change.
- No tracker, CLI summary, or other command changes.

## Release note

`specsync release` and `specsync claim` now refuse a mistyped change name or a wrong `-openspec` path instead of silently creating a phantom change directory.
