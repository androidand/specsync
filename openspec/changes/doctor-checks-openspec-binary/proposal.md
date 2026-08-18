## Why

specsync hard-depends on the `openspec` CLI binary at runtime — it shells out to
it via `exec.LookPath`/`exec.Command` in `graph_delta.go`, `openspec.go`, and
`coordination.go`. But `specsync doctor` only ever checks whether specsync's
*own* skill is installed (`doctorClaude`, `doctorInstall`); it never checks
whether `openspec` itself is present. A user can install the specsync skill,
skip `openspec init`, and get a bare "executable file not found in $PATH"
failure deep inside a sync/pull/scan command instead of an upfront diagnostic
pointing them at the fix.

## What Changes

- `specsync doctor` (and `doctor claude`) additionally probe for the
  `openspec` binary via `exec.LookPath("openspec")` and report:
  - found/not found, and its resolved path when found
  - its reported version (`openspec --version`), when found
- When missing, the diagnostic surfaces as a `warning`/`error`-level issue
  (consistent with existing `DoctorResult.Status`/`Recommendations` shape)
  with a recommendation: `openspec init` (or the project's existing
  `openspec/` setup instructions).
- `doctor install` gains an "External Dependencies" section listing the
  `openspec` binary alongside the existing per-agent skill locations, so the
  full picture (skill + binary) is visible in one place.
- JSON output (`doctor claude --json`, `doctor install --json`) includes the
  new dependency-check fields so agents can act on it programmatically.

No changes to sync/pull/scan behavior itself — this is diagnostics only.

## Capabilities

### New Capabilities
- `doctor-dependency-checks`: `specsync doctor` verifies that runtime CLI
  dependencies (starting with `openspec`) are installed and reachable, and
  reports version/path or a fix recommendation when missing.

### Modified Capabilities
(none — no existing spec currently documents `doctor` behavior in
`openspec/specs/`)

## Impact

- `cmd/specsync/doctor.go`: extend `DoctorResult`/`InstallationInfo` (or add a
  small `DependencyInfo` struct) and `doctorClaude`/`doctorInstall` to run the
  new check.
- No new external dependencies; reuses `os/exec` already imported elsewhere
  in the codebase (`graph_delta.go` already does an inline
  `exec.LookPath("openspec")` check — no extractable helper exists yet, so
  this change adds the first one, sized to also fit that call site later
  if desired).
- No breaking changes to existing `doctor` output fields; new fields are
  additive.
