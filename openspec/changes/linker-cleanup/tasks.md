# Tasks: Linker cleanup

## 1. LinkerResult with Source

- [x] 1.1 Add `LinkerResult` struct with `Ref` and `Source` fields
- [x] 1.2 Update `Linker` interface to return `*LinkerResult`

## 2. BranchResolver slug-matching

- [x] 2.1 Capture slug from branch name (`feat/42-slug` → slug = "slug")
- [x] 2.2 Verify slug matches `changeDir` when resolving for a specific change

## 3. Dead code removal

- [x] 3.1 Remove MarkerResolver
- [x] 3.2 Remove ExternalResolver
- [x] 3.3 Update tests to remove dead resolver tests

## 4. Pull simplification

- [x] 4.1 Pull uses BranchResolver directly (no Linker chain)
- [x] 4.2 Remove Linker field from PullOptions

## 5. BuildSyncLinker

- [x] 5.1 Only BranchResolver in sync linker (cache handled per-provider)

## 6. Tests

- [x] 6.1 All linker tests use `*LinkerResult`
- [x] 6.2 Full test suite passes
