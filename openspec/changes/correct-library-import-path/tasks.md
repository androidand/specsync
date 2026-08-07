# Tasks

> Evidence: `openspec/changes/archive/2026-07-25-specsync-go-library/tasks.md`
> has 9/9 checked, including "move all root `.go` files into `pkg/specsync/`".
> `pkg/` does not exist; `go.mod` declares `module github.com/androidand/specsync`
> and `audit.go`, `board.go`, `provider.go` are at the root. skein's
> `specsync-library-integration` proposal already instructs implementers to
> import the non-existent `pkg/specsync` path.

- [x] 1 Correct the archived `specsync-go-library` tasks and proposal to describe
  the root-package layout that shipped, with an explicit note that the `pkg/`
  move was specced and not adopted. Do not silently delete the claims — the
  reason matters more than the tidiness.
  - Files: `openspec/changes/archive/2026-07-25-specsync-go-library/{tasks,proposal}.md`
  - Validation: `grep -r "pkg/specsync" openspec/` returns only the note
    explaining the layout was not adopted
- [x] 2 State the canonical import path in package documentation, once.
  - File: `doc.go`
  - Validation: `doc.go` names `github.com/androidand/specsync` as the import
    path and no other path is documented anywhere
- [x] 3 Add a guard test asserting the package location and the exported symbols
  consumers embed (`Sync`, `Pull`, `Link`, `Spinoff`, `LoadChange`,
  `WorkItemFor`, the `WorkProvider` interface). It sits next to
  `boundary_test.go`, which guards the stdlib-only rule for the same reason: a
  convention nothing checks is a convention that drifts.
  - File: `import_path_test.go`
  - Validation: `go test ./...` passes; moving the package or renaming an
    exported symbol fails the test
- [x] 4 Link the downstream fix. skein's `specsync-library-integration` proposal
  prose must change to the root path; its `tasks.md` is already correct, so the
  two disagree inside one change.
  - File: `openspec/changes/correct-library-import-path/links.md`
  - Validation: `links.md` references the skein change or its issue, and
    `specsync link` renders the cross-repo `## Related` block in both repos
