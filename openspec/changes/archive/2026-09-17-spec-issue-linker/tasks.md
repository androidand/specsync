# Tasks: Spec ↔ issue linker

## 1. Linker interface

- [x] 1.1 Define `Linker` interface in a new `linker.go`: `Resolve(ctx, changeDir) (*Ref, error)`
- [x] 1.2 Define chained resolver: tries each resolver in order, first hit wins

## 2. Resolvers

- [x] 2.1 Branch-name resolver: configurable pattern (e.g. `feat/(\d+)-.*` → issue `#1`)
- [x] 2.2 Marker resolver: parse `<!-- specsync:change=<slug> -->` from issue body via provider
- [x] 2.3 Cache resolver: read `.specsync/refs.json` (existing behavior, refactored behind Linker)
- [x] 2.4 External resolver: optional hook for MCP or other external relation sources

## 3. Integration

- [x] 3.1 Wire Linker into sync engine ahead of provider `Find`
- [x] 3.2 Use Linker in `pull` for issue-first flow (resolve branch → issue)
- [x] 3.3 Support spec-first creation when no link resolves (existing behavior)

## 4. Verification

- [x] 4.1 Tests: branch-name pattern matching, marker parsing, cache lookup, chain order
- [x] 4.2 Integration test: sync resolves issue via branch name without explicit ref
- [x] 4.3 Update the specsync skill: how Linker resolves issues from branches/markers
