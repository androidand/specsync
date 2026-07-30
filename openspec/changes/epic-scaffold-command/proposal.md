# Epic scaffold command: mint the epic and wire the family in one step

## Why

The cross-repo stack is planned as clean layers — and, as of 2026-07-30, partly
shipped: `cross-repo-linked-issues` (archived) gave `link` + `-repo`;
`link-by-issue-reference` (16/16) lets one `link` call span repos with no local
scaffolding, proven live today by cross-linking
`androidand/brick-now#217 ↔ androidand/tengil#70 ↔ androidand/tengil#35`.
`epic-and-subissue-projection` will project `## Parent` edges onto native GitHub
sub-issues, and `issue-dependency-sync` will add direction.

But the stack has no *front door*. The maintainer's canonical scenario —

> **"We want feature X. It needs Y added to the backend repo and Z to use it in
> the frontend repo."**

— still requires the operator to hand-craft the epic: create a coordination
issue somewhere, label it `type:epic` by hand, then (once sub-issue projection
exists) edit each child's `links.md` or attach sub-issues in the GitHub UI.
`epic-and-subissue-projection` deliberately defines epics *by convention* and
does not create them; `link-by-issue-reference` explicitly defers issue
creation. Both deferrals are individually right — which leaves the composed
workflow owned by nobody. This was nearly lost once already: the maintainer
believed the cross-repo capability had been deleted, because the pieces are
scattered across five changes and the shipped part wasn't even in the published
npm package (the 16/16 `link-by-issue-reference` exists only in git — `npm`
still serves 0.9.1 without it).

## What Changes

- **`specsync epic <title> [--repo owner/name] [--child <slug|owner/repo#N|url>]...`**
  One command that:
  1. creates the epic as a coordination issue (`type:epic` label, no local
     change directory — an epic is not a spec) in the target repo;
  2. attaches every `--child` — a local change slug (synced first if needed,
     honoring the existing `-repo` behavior) or an existing issue reference in
     any repo — as a **native GitHub sub-issue** once `epic-and-subissue-
     projection` lands, and as a managed `## Related` cross-reference until
     then (graceful degradation, same body-upsert helper);
  3. prints the epic URL and the child mapping, idempotently re-runnable —
     re-invoking with the same title+children converges instead of duplicating.
- **Epic body is a roll-up, not a spec**: children listed with live state
  (from `subIssuesSummary` when available; from synced checkbox stage until
  then). No proposal.md is scaffolded for the epic itself.
- **Release-gap guard**: `specsync --version` gains the git describe / build
  info needed to tell "repo dev build" from "published release", and the
  release checklist grows one line: a capability shipped 16/16 MUST be
  published before its change is archived — the gap that hid
  `link-by-issue-reference` from every installed copy for two weeks.

## Acceptance — the maintainer's scenario, verbatim

From any repo:

```
specsync epic "Feature X: cross-repo widgets" --repo androidand/planning \
  --child androidand/backend#12 \
  --child frontend-widget-view        # local slug in this repo's openspec tree
```

produces: one `type:epic` issue in `androidand/planning`; backend#12 and the
frontend change's issue attached (sub-issues when projection exists, Related
until then); each child body pointing back at the epic; a second identical
invocation changing nothing. With `issue-dependency-sync` landed, adding
`--blocked-by androidand/backend#12` on a child records real direction — that
flag belongs to that change and is only *reserved* here.

## Out of scope

- Sub-issue projection mechanics and `## Parent` reconciliation
  (`epic-and-subissue-projection` owns them; this command becomes their
  consumer).
- Dependency direction (`issue-dependency-sync`).
- Cross-repo *local* discovery — worktree/workset awareness
  (`openspec-references-coordination`).
- Non-GitHub providers (`pluggable-providers`).

## Impact

- New `epic.go` + `epic` subcommand in `cmd/specsync/main.go`; reuses
  `NewGitHubProviderWithRepo`, `resolveEntry` classification, and the shared
  `## Related` upsert from `link-by-issue-reference`.
- `releasetool.go` / release checklist: the publish-before-archive rule.
- Depends on published `link-by-issue-reference` (task 1 is the npm release).
