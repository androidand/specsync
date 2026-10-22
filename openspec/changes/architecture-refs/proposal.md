# Architecture references in links.md

## Why

Architecture tools can plan work from an architecture model and hand it to
specsync as OpenSpec changes. ArchSync (desired vs observed architecture, with
LikeC4 as its model) does this: each generated change records which architecture
elements it covers, so the tool can later say "this element is planned in change
X, tracked as issue Y, and now built".

It records them in the change's own `links.md`, under `## Architecture`, as
entries of the form `archsync:<owner>/<repo>/<elementId>`. Today specsync
ignores such entries: they are neither URLs, shorthands, nor sibling slugs, so
`resolveEntry` returns nil. That is safe, but the link is invisible on the
specsync side: the issue does not show its architecture scope, and `relate`
cannot answer "which changes touch this element?".

Both tools hold the same rule for links: asserted, never inferred. An
`## Architecture` entry is authored in the change, so it fits the work graph's
asserted-edge model without new inference.

## What Changes

- `parseLinksMD` recognises entries of the form `<scheme>:<path>` under
  `## Architecture` as architecture refs (`scheme`, `repo`, `id`). They are never
  added to related links or dependencies. Any other entry under that heading keeps
  today's behaviour.
- The work graph gains a node kind `arch` and an asserted edge kind `spec-arch`
  with source `links.md`. `relate -change` and `trace -json` include them.
- `relate -arch <scheme:repo/id>` lists the changes (and their issues) that
  record that element.
- Sync renders an `## Architecture` section in the issue body listing the
  element ids, so the tracker shows a change's architecture scope.
- specsync does not call the architecture tool and does not check that an element
  exists. It records what the change asserts.

## Non-goals

- No architecture model in specsync, no drift, no rendering of diagrams.
- No round-trip of the issue's `## Architecture` section back into `links.md` on
  `pull` (possible later).
- No change to how entries outside `## Architecture` are read.

## Dependencies

- `fix-links-append-section`: without it, a related link appended to a file whose
  last section is `## Architecture` would land in that section.
