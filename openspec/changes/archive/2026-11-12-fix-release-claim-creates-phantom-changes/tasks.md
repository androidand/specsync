- [x] 1. Load the change before writing metadata in `runClaim` and `runRelease`.
       Each command SHALL call `mutableChange(openspecDir, slug, false)` (the shared
       load-first guard that fails "change not found") before `SaveChangeMetadata`, so a
       typo slug or an `-openspec` with no matching change exits non-zero and writes
       nothing.
       Validation: a Go test that runs `runClaim`/`runRelease` against a fixture repo
       with a nonexistent change slug asserts `os.Exit(1)`-level failure (fail() calls
       os.Exit(1), so drive it through a subprocess or assert the guard path via the
       exported `mutableChange`) and asserts no `changes/<slug>/.specsync/metadata.json`
       is created. The test must fail today: today both commands create the file and
       return nil.

- [x] 2. Reject an `-openspec` that is not a directory containing a `changes/` child.
       Both commands already resolve `*openspec` to an absolute path; add a check that
       `<abs>/changes` is an existing directory and `fail` otherwise, so the default
       `-openspec openspec` run from inside `openspec/` (which resolves to
       `openspec/openspec/`) is refused instead of written into.
       Validation: run `release`/`claim` with `-openspec <nonexistent>/changes` and
       assert a non-zero exit and no directory created.

- [x] 3. Record in `design.md` the taxonomy of what a bad invocation now does (refuse,
       exit 1, no write) and note that `SaveChangeMetadata` stays a raw writer — the
       guard lives at the "operate on an existing change" callers, not in the writer.
       Validation: `design.md` states the boundary explicitly.

- [x] 4. Pin the invariant on the commands that remain. `claim` and `release` no longer
       exist, so tasks 1 and 2 hold by removal; `set-stage`, `set-priority` and `note`
       are the surviving slug-mutating commands.
       Validation: `TestMutatingCommandsRefuseUnknownChange` runs each against an
       unknown slug, asserts a non-zero exit and that no change directory is created;
       it goes red when `mutableChange` stops failing on a missing change.
