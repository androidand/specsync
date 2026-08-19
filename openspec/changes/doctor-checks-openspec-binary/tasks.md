## 1. Dependency check helper

- [x] 1.1 Add a `checkOpenspecBinary()` helper in `cmd/specsync/doctor.go` that
      wraps `exec.LookPath("openspec")` and, when found, best-effort runs
      `openspec --version` to extract a version string (never fatal on parse
      failure)
- [x] 1.2 Add a small result type (e.g. `DependencyInfo{Name, Found, Path,
      Version string}`) alongside the existing `InstallationInfo` types

## 2. Wire into `doctor claude`

- [x] 2.1 Call `checkOpenspecBinary()` from `doctorClaude` and attach the
      result to `DoctorResult` as an additive field (e.g. `Dependencies
      []DependencyInfo`)
- [x] 2.2 When missing, downgrade `Status` to at least `"warning"` and append
      a recommendation (`"Install with: openspec init"`) to
      `Recommendations`, consistent with existing skill-missing handling
- [x] 2.3 Update the human-readable `doctor claude` prose output to print the
      dependency section

## 3. Wire into `doctor install`

- [x] 3.1 Extend `doctorInstall`'s output with an "External Dependencies"
      section listing the `openspec` binary (found/path/version), alongside
      the existing per-agent skill-location list
- [x] 3.2 `doctorInstall` currently ignores its `asJSON bool` parameter
      entirely — there is no JSON branch today, unlike `doctorClaude`. Add
      one (mirroring `doctorClaude`'s `json.MarshalIndent` pattern) so
      `doctor install --json` emits structured output at all, including the
      new dependency-check fields. Pre-existing gap, closed incidentally by
      this change rather than left for a separate one, since we're already
      editing this function.

## 4. Tests

- [x] 4.1 Unit test `checkOpenspecBinary()` for: binary present with parseable
      version, binary present with unparseable/failing `--version`, binary
      absent (e.g. by temporarily emptying `$PATH` or stubbing `exec.LookPath`
      if the codebase already has a seam for that — check `graph_delta.go`'s
      existing `exec.LookPath("openspec")` usage/tests for the established
      pattern)
- [x] 4.2 Test that existing `doctor claude --json` / `doctor install --json`
      fields are unchanged (additive-only) to guard against breaking
      existing consumers

## 5. Docs

- [x] 5.1 Update `specsync agent-help doctor` (if `doctor` is registered
      there) to mention the new dependency check
- [x] 5.2 Note the change in the next release notes / CHANGELOG entry
