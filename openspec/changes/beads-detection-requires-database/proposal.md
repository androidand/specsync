# Beads auto-detection requires a Beads database

## Why

`beads-autodetect` made `bd` on PATH sufficient to auto-select the Beads
provider. On a developer machine `bd` is installed once, globally — so that rule
makes *every* repo look like a Beads project.

The consequence is not a cosmetic default. Reported from actual usage: a plain
`specsync sync` in a GitHub-tracked repo auto-selected Beads because `bd` was on
PATH — even though `bd list` reported no database — and would have created
phantom duplicate items instead of updating the issues specsync itself had
already created. The correct target was only reached by passing
`-provider github` explicitly. A default that quietly sends work to the wrong
tracker is worse than no default.

`bd` being installed says something about the machine. It says nothing about the
project. The project-level signal is a Beads database: a `.beads/` directory.

## What Changes

- **BREAKING** (to the auto-detection rule, not to any flag): `bd` on PATH alone
  no longer selects Beads. Auto-detection selects Beads only when a `.beads/`
  directory is present — at the resolved repo root or in the working directory —
  *and* `bd` is on PATH. A project carrying `.beads/` on a machine without the
  binary falls back to github rather than failing on the first shell-out.
- Detection is anchored to the repo root derived from `-openspec`, not just the
  process working directory, so running specsync from a subdirectory resolves the
  same way.
- A `.beads` regular file is not a database and does not trigger detection.
- When auto-detection lands on a non-default provider, specsync prints the
  provider and the reason on real runs, not only under `-dry-run`. Where every
  item in a run is about to land is worth one line of output.
- `-provider beads` still selects Beads unconditionally; explicit always wins.

## Impact

- Affected code: `cmd/specsync/main.go` (`detectProvider`, its `runSync` call
  site).
- Affected specs: `beads` — replaces the "Detect bd on PATH" requirement.
- Users who relied on `bd`-on-PATH auto-selection in a project with no `.beads/`
  now need `-provider beads`. That path could only ever have been driving a Beads
  database outside the project, which auto-detection was never able to locate.
