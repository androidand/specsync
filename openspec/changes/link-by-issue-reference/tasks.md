# Tasks: link existing issues by reference

## Argument classification (core + `cmd/specsync`)
- [x] Classify each `link` argument as slug vs issue reference using the
      `resolveEntry` rules (URL / `owner/repo#N`), adding a bare `#N` / `N` form
      resolved against `-repo` or the auto-detected remote
- [x] Allow slugs and references to be mixed in one invocation
- [x] An unresolved reference (no such issue) is an error; never create an issue
      (classifyArg errors on unknown slug; reference path errors on Get failure)

## Shared `## Related` renderer (`sync.go`)
- [x] Extract `upsertRelatedSection(body string, links []Ref) string`: replace an
      existing `## Related` block (up to the next `##` or EOF) in place, else append
- [x] Refactor `WorkItemFor` to call it instead of the inline append (no behavior
      change for slug syncs; assert via existing tests)

## Reference link path (`link.go`)
- [x] For a reference argument: `IssueReader.Get` the issue, upsert `## Related`
      with the other arguments' URLs, push the edited body via the provider
- [x] Write no change directory and no `links.md` for reference arguments
- [x] Build one provider per distinct repo (`makeProvider`); edit each
      issue through the provider bound to its repo
- [x] Keep the slug path unchanged (still writes `links.md`, still syncs)

## Dry-run (`cmd/specsync`)
- [x] `link -dry-run` prints, per referenced issue, the issue-edit and the
      `## Related` block it would write; makes no GitHub calls

## Tests
- [x] Classification table: slug, `#N`, `owner/repo#N`, URL, mixed
- [x] `upsertRelatedSection`: insert into a body with none, replace an existing one,
      idempotent re-run, preserve trailing `##` sections
- [x] Reference link via fake provider: asserts the edited body and per-repo routing,
      and that no file is written
- [x] Mixed slug + reference: slug gets `links.md`, reference gets a body edit

## Boundaries & docs
- [x] Stdlib-only; `boundary_test.go` stays green
- [x] Update the specsync skill: `link` accepts issue references, not just slugs;
      show the cross-repo `owner/repo#N` example
