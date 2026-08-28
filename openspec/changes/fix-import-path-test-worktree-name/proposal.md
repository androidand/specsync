# Fix TestImportPath failing outside a folder literally named "specsync"

## Why

`TestImportPath` (import_path_test.go) is meant to guard doc.go's stated
invariant — "the package lives at the module root, not under pkg/" — but it
actually asserted something narrower and wrong: that the resolved package
directory's *basename* ends in the literal string `"specsync"`
(`strings.HasSuffix(pkg.Dir, "specsync")`). That's true for the primary clone
(`/…/specsync`) but false for any git worktree named after its branch — this
repo's own convention (`specsync-<change>`, e.g. this session's
`specsync-design-notes`) — so the test fails in every worktree despite
nothing being wrong with the package's location.

## Proposed Changes

- Compare the resolved package directory (`build.Import`'s `pkg.Dir`)
  against the test binary's own working directory (`os.Getwd()`) instead of
  a literal folder-name substring. `go test` always sets the process's
  working directory to the package's source directory, so this checks the
  actual invariant — resolving the canonical import path lands back at
  wherever the package's files really are — regardless of what the
  containing checkout or worktree happens to be named on disk.
- The test still fails the way it's meant to: if the package were ever
  relocated under `pkg/` while this test file moved with it, `build.Import`
  either errors (no Go files left at the module root) or resolves to a
  different directory than the moved test's own `cwd` — both are still
  caught.

## Release Notes

Fixed `TestImportPath` failing in any git worktree not literally named
"specsync" (e.g. this repo's own `specsync-<change>` worktree convention) —
it now checks the actual module-root invariant instead of a folder-name
string match.
