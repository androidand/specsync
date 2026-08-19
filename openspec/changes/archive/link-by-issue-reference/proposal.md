# Link existing issues by reference, without scaffolding specs

## Why

`specsync link` today accepts only local change **slugs**, and each must already
be synced to an issue (`link.go` errors with "has no synced ref" otherwise). So
to cross-reference two issues that already exist — especially across repos, e.g.
`billing-service#3489 ↔ dashboard#4084` — you must first `pull` each into a local OpenSpec
change just to add a `## Related` line, littering two repos with specs you never
wanted. The visible outcome is one cross-reference; the cost is full spec
scaffolding on both sides.

This is friction in exactly the cross-repo case `cross-repo-linked-issues` set out
to serve. The link mechanism is right; its only entry point is too narrow.

## What Changes

- **`specsync link` accepts issue references, not just slugs.** An argument may be
  a slug (as today), a bare `#N` (the `-repo`/auto-detected repo), `owner/repo#N`,
  or a full issue URL. Slugs and references may be mixed in one invocation.
- **A reference argument is linked directly on GitHub, with no local spec.**
  specsync fetches the issue, upserts a managed `## Related` section pointing at
  the other linked issues, and pushes the edited body — no change directory, no
  `links.md`. The cross-reference lives only in the issue bodies, which is the
  whole point.
- **The `## Related` section is managed idempotently in any body.** A shared
  renderer upserts the section (replace-in-place, never append-duplicate) whether
  the body comes from a rendered spec (`WorkItemFor`) or a fetched raw issue, so
  re-running is safe and the two paths can't drift in format.
- **Cross-repo targeting is per reference.** Each `owner/repo#N` or URL resolves to
  its own repo; specsync edits each issue through a provider bound to that repo,
  so one `link` call spans repos.

### Out of scope / explicitly deferred
- Creating issues — `link` only cross-references issues that already exist
- Any local artifact for pure-reference links — no change dir, no `links.md`
  (slug arguments still write `links.md` exactly as today)
- Providers other than GitHub — the reference forms are GitHub-shaped; other
  providers gain an equivalent when they exist (`pluggable-providers`)
- Removing a link — this change only adds/updates `## Related`; unlinking is a
  later, separately-earned addition

## Capabilities

### New Capabilities
- `issue-reference-linking` — `link` accepts issue references (`#N`,
  `owner/repo#N`, URL) and cross-references existing issues directly on the
  tracker, with no local spec, idempotently and across repos.

## Impact

- `link.go` / `cmd/specsync` (`runLink`): classify each argument as slug vs issue
  reference; for references, resolve the issue via the existing `IssueReader.Get`
  and push an edited body; keep the slug path unchanged.
- `sync.go`: factor the `## Related` rendering in `WorkItemFor` into a shared
  upsert helper reused by the reference path; no behavior change for slug syncs.
- Builds on shipped pieces: `-repo` + `github:owner/repo` ref keys, `resolveEntry`
  (URL/shorthand classification), `IssueReader.Get`, and the `[owner/repo#N](url)`
  autolink label. Extends `cross-repo-linked-issues`; depends on no other in-flight
  change. Stays stdlib-only and shells out to `gh`.
