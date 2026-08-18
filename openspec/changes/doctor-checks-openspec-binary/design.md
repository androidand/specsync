## Context

`specsync` shells out to the `openspec` binary in three places
(`graph_delta.go`, `openspec.go`, `coordination.go`); one of them already does
`exec.LookPath("openspec")` and fails loudly if it's missing, but only at the
point of use, deep in a sync/scan command. `cmd/specsync/doctor.go` currently
only inspects specsync's own skill files (`doctorClaude`, `doctorInstall`) —
it has no concept of specsync's own runtime dependencies.

## Goals / Non-Goals

**Goals:**
- Surface a missing/unreachable `openspec` binary as an upfront `doctor`
  diagnostic, before it fails inside an unrelated command.
- Report the `openspec` binary's version when found, so version-skew issues
  (e.g. a schema the CLI doesn't support yet) are visible too.
- Keep the check read-only: `doctor` must not install, upgrade, or invoke
  `openspec` beyond `--version`.

**Non-Goals:**
- Auto-installing or bootstrapping `openspec` for the user.
- Bundling or forking `openspec`'s own Claude Code skill/command scaffolding
  (`openspec init`/`update` already owns that; specsync only needs the
  binary, not that UX layer).
- Checking every possible external dependency — this change adds one
  dependency check (`openspec`) with a shape that a future dependency could
  reuse, not a general plugin system.

## Decisions

- **Reuse the existing `exec.LookPath` pattern from `graph_delta.go`** rather
  than introducing a new dependency-detection abstraction. A single small
  helper — `checkOpenspecBinary() DependencyInfo` (no error return: "not
  found" and "version unparseable" are both reportable states, not failures
  the caller must handle) — lives in `doctor.go` and is called from both
  `doctorClaude` and `doctorInstall`. Alternative considered: a generic
  `[]DependencyCheck` table for future binaries — rejected as premature; one
  dependency doesn't justify the abstraction yet, and the field shapes below
  don't block adding one later.
- **Version via `openspec --version`, best-effort.** If the binary is found
  but `--version` fails or its output is unparseable, `DependencyInfo.Found`
  stays `true` with `Version` left empty, rather than erroring the whole
  `doctor` run — diagnostics should degrade gracefully, not crash. Observed
  today: `openspec --version` prints a bare version string (e.g. `1.5.0`,
  no `v` prefix, no surrounding text) to stdout, so parsing is a trim, not a
  regex — but the helper should not assume that stays true forever.
- **Additive JSON fields only.** Add a `Dependencies` field to `DoctorResult`
  (or nest under `Installation`) rather than repurposing existing fields, so
  existing consumers of `doctor --json` aren't broken.

## Risks / Trade-offs

- [`openspec --version` output format could change upstream and break
  parsing] → parse defensively (best-effort, never fatal) and fall back to
  "found, version unknown" rather than failing the check.
- [Scope creep toward a general dependency-checking framework] → explicitly
  scoped to `openspec` only in this change; documented as a Non-Goal above.
