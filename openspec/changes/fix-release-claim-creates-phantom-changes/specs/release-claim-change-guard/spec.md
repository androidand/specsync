# release and claim must operate on an existing change, or refuse

## ADDED Requirements

### Requirement: release and claim refuse a change that does not exist

`specsync release -change <slug>` and `specsync claim -change <slug>` SHALL load the
named change and refuse with a non-zero exit when it does not exist, without creating
or modifying any directory.

The change directory is built as `filepath.Join(abs, "changes", <slug>)` and written via
`SaveChangeMetadata`, which calls `os.MkdirAll` and therefore creates a missing
directory. A slug that was never a change — a typo, or a slug that exists only in a
mistyped `-openspec` — must therefore be rejected before that write, using the same
load-first guard as every other metadata-mutating subcommand.

A tool whose contract is to move an existing change between tracker states SHALL NOT
report success while inventing the change it claims to move.

#### Scenario: A typo'd change slug

- **WHEN** `specsync claim -change feat-loginn` runs against a repo that has no such
  change
- **THEN** the command exits non-zero, prints "change not found", and creates no
  `changes/feat-loginn/.specsync/metadata.json`

#### Scenario: release of an unclaimed slug

- **WHEN** `specsync release -change does-not-exist` runs
- **THEN** the command exits non-zero and writes nothing

### Requirement: release and claim reject an openspec path without a changes directory

`specsync release` and `specsync claim` SHALL refuse an `-openspec` argument that is not a
directory containing a `changes/` child.

The default `-openspec openspec` run from inside an `openspec/` directory resolves to
`openspec/openspec/changes/<slug>`, and that path is where the estate-wide phantom
`openspec/openspec/` artifact was created. The command must detect the missing
`changes/` before writing, so a misinvocation is refused rather than materialised.

#### Scenario: openspec resolves to a directory without changes

- **WHEN** `specsync release -openspec /tmp/nope -change feat-login` runs and
  `/tmp/nope/changes` does not exist
- **THEN** the command exits non-zero and creates no directory
