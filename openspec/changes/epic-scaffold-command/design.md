## Context

Every piece the epic command needs already exists as a separate primitive:

- `classifyArg(arg, openspecDir, repo string) (linkEntry, error)` (`link.go`)
  already classifies an argument as a local change slug or an issue reference
  (`#N`, `owner/repo#N`, URL), resolving bare `#N` against `-repo`.
- `UpsertRelatedSection(body string, links []Ref) string` (`link.go`) already
  renders/replaces a managed `## Related` section idempotently.
- `GitHubProvider.Push(ctx, item WorkItem, existing *Ref) (Ref, error)`
  (`github.go`) already creates-or-edits an issue: it computes labels via
  `desiredLabels` (an explicit non-nil `item.Labels` bypasses the
  stage/priority default entirely), calls `EnsureLabels`, and — critically —
  works from `item.Slug` alone; nothing in `Push`, `Find`, or `marker()`
  requires `item.Slug` to be a real change directory. `runLink`'s reference
  path (`cmd/specsync/main.go`) already proves this: it calls
  `provider.Push(ctx, WorkItem{Slug: "", ...}, &existingRef)` to edit a bare
  issue that has no local spec at all.
- `Find(ctx, slug) (*Ref, error)` searches `specsync:change=<slug> in:body`
  and is driven purely by whatever string is passed as `slug` — it has no
  awareness of change directories either.

So "mint an epic and idempotently re-run" is not a new capability at the
provider layer — it is a new *caller* (`epic.go`) composing four things that
already work, plus one new label convention (`type:epic`) that the existing
label-delta machinery already handles correctly with zero changes (see
Decisions).

## Goals / Non-Goals

**Goals:**
- One command, `specsync epic <title> -repo owner/name -child ...`
  (repeatable `-child`), that creates or converges an epic issue and wires
  every child to it, cross-repo, idempotently.
- Reuse `classifyArg`, `UpsertRelatedSection`, and `Push`/`Find` completely
  unchanged — no new provider interface methods for the degraded (`##
  Related`) mode.
- Graceful degradation: when `epic-and-subissue-projection` has not landed
  (or the token lacks the GraphQL scope), children are wired via `##
  Related` in both directions, using the exact same upsert helper `link`
  already uses for issue-reference edits.
- Idempotent re-run: same title + same children converges (no duplicate
  epic, no duplicate `## Related` lines) rather than erroring or duplicating.

**Non-Goals:**
- Native sub-issue attachment (`addSubIssue` GraphQL mutation) — reserved for
  `epic-and-subissue-projection`; `epic.go` becomes its consumer once it
  lands, per the proposal's Out-of-scope section.
- Dependency direction (`blocked-by`/`blocks`) — reserved for
  `issue-dependency-sync`; only the flag name is reserved here.
- A general "standalone issue" abstraction in the provider layer — `epic.go`
  calls `Push`/`Find` directly with a chosen `Slug` convention (see below)
  rather than the provider gaining a `CreateStandaloneIssue` method. One
  caller doing this doesn't justify a new interface; `runLink`'s reference
  path already established the pattern of a slug-less/synthetic `WorkItem`.
- Non-GitHub providers — an epic is inherently a GitHub Issues concept today;
  `pluggable-providers` grows an equivalent when another provider needs one.

## Decisions

- **Epic identity reuses the existing marker/Find machinery via a slug
  namespace, not a new marker format.** `marker(slug)` renders
  `<!-- specsync:change=%s -->` unconditionally inside `Push`/`renderBody`,
  and `Find` searches for exactly that string. Rather than teaching
  `marker()` a second format (`specsync:epic=...`, as an earlier sketch of
  this proposal assumed), `epic.go` picks `Slug: "epic:" + normalizeTitle(title)`
  for the `WorkItem` it pushes. The embedded HTML comment then literally
  reads `<!-- specsync:change=epic:add-widgets -->` — cosmetically says
  "change" for an epic, which is a little odd, but it is an internal,
  never-rendered identity marker, and this way `Push`/`Find`/`EnsureMarker`
  need zero changes. Alternative considered: add an epic-specific marker
  format — rejected as unnecessary provider-layer churn for a purely
  internal string; revisit only if the "change" wording ever leaks somewhere
  user-visible.
- **No local ref cache for the epic.** A real OpenSpec change caches its ref
  in `.specsync/refs.json` so `Push` skips `Find`'s `gh issue list --search`
  round trip on every sync. An epic has no change directory (`proposal.md`'s
  own framing: "an epic is not a spec"), so `epic.go` always resolves via
  `Find` before deciding create-vs-edit. This costs one extra `gh` call per
  invocation, which is acceptable: `specsync epic` is an occasional,
  human-driven command, not part of the per-push `sync` hot path.
- **`type:epic` needs no changes to `desiredLabels`/`managedLabel`.**
  `epic.go` passes `WorkItem.Labels: []string{"specsync", "type:epic"}`
  explicitly, which makes `desiredLabels` return that slice verbatim
  (`item.Labels != nil` short-circuits the stage/priority default).
  `labelDelta`'s `add` side adds any desired-but-not-current label
  unconditionally (it does not gate `add` on `managedLabel`, only `remove`),
  so `type:epic` is added correctly on both create and idempotent re-run
  without touching `managedLabel`.
- **Child wiring branches by `classifyArg`'s existing `linkEntryKind`, reusing
  each kind's already-proven push path** — no new "how do I edit this thing"
  code:
  - `kindSlug`: sync that change first if it has no ref yet (same as
    `runLink`'s slug path: `specsync.Sync(ctx, Options{..., Slug: slug})`),
    then treat its resulting `Ref` as a child to relate.
  - `kindIssueRef`: fetch via `provider.(IssueReader).Get`, `Upsert...` into
    its body, `Push` the edited body back with `Slug: ""` and the fetched
    `Labels` preserved — the exact sequence `runLink`'s reference path already
    runs, extracted into a small shared helper (`pushRelatedEdit` or similar)
    so `link.go` and `epic.go` call one function instead of `epic.go`
    duplicating `runLink`'s ~15 lines.
- **Epic body is hand-assembled, not `WorkItemFor(Change)`.** `WorkItemFor`
  renders a change's proposal + tasks checklist; an epic has neither. Its
  body is a small roll-up template (title line + a bullet per child with its
  URL and current state) that `epic.go` regenerates in full on every push —
  simpler than trying to diff/preserve prior roll-up content, and safe
  because the whole body between the marker and end-of-file is
  specsync-owned for an epic (no human-authored prose to preserve, unlike a
  change's proposal).
- **`-repo` is optional, defaulting to the current repo's git remote.**
  Consistent with `relate`'s `-repo` ("target repo as owner/name (default:
  auto-detect from git remote)") and every other command's flag — the
  proposal's own usage line already brackets it as optional
  (`[-repo owner/name]`), so `epic.go` must not make it required in practice.
  Only bare `#N` children resolve against it; `owner/repo#N` and URL children
  ignore it entirely.
- **`-child` is repeatable via the existing `stringSlice` flag type**
  (`cmd/specsync/main.go`, already used for `-provider`) — no new flag
  plumbing needed, just `fs.Var(&children, "child", ...)`.
- **`--version` build-info (task 6) is a separate, independently-landable
  slice of this change**, unrelated to the `epic` command mechanically —
  it only shares a motivation (the `link-by-issue-reference` release-gap
  story). It is captured as its own capability (`release-gap-guard`, see
  specs) so it can be implemented and archived on its own schedule without
  blocking or being blocked by the `epic` command's own work.

## Risks / Trade-offs

- [No ref cache means every `specsync epic` re-run pays a `gh issue list
  --search` round trip] → acceptable for a human-driven, occasional command;
  revisit only if agents start invoking it in a hot loop.
- [Epic body is fully regenerated each push, so any manual edit a human makes
  directly to the epic issue body is silently overwritten] → document this
  explicitly in the command's help text and the skill; matches the existing
  precedent that a change's issue body is specsync-owned once synced.
- [Degraded mode (`## Related`) and full mode (native sub-issues, once
  `epic-and-subissue-projection` lands) render different relationship UI on
  GitHub] → `epic.go` type-asserts for the sub-issue capability the same way
  other optional provider capabilities are detected elsewhere in the
  codebase, and falls back automatically; no user-visible flag needed to pick
  a mode.
- [Bundling `release-gap-guard` into the same change as `epic-scaffold`
  risks conflating two unrelated review/ship timelines] → mitigated by
  giving each its own capability/spec file so either can be archived
  independently; if this proves confusing in practice, split them into two
  changes before implementation starts.
