# Tasks

- [x] Rewrite `TestImportPath` to compare `pkg.Dir` against `os.Getwd()`
      instead of a literal `"specsync"` basename suffix check.
      Validation: `go test ./...` passes with zero failures when run from a
      worktree whose folder is not literally named "specsync".
