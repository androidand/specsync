# Design

## Where `targets` lives
A committed `specsync.yml` inside the change folder, holding a `targets` array in
the same provider-key format as `refs.json` (`"github:owner/name"`), so targets
and refs line up by key.

Rejected: `.specsync/metadata.json`, the obvious first choice and wrong. The whole
`.specsync/` directory is gitignored — `git ls-files` finds zero tracked files
under it — because it holds the projection cache, which must never be committed.
Targets have the opposite requirement: a store is shared, so every teammate's
clone must agree on where a change belongs. A target that does not survive a
clone is not a declaration, it is a local guess.

Note this also means `stage` is machine-local today, which contradicts the
promise that stages are committed metadata rather than external labels. Out of
scope here, but tracked: see `commit-change-stage`.

Rejected: a `targets:` key in `openspec/specsync.yml`. In store-only mode that file
is one-per-store, but targets are one-per-change.

Rejected: parsing `links.md`. It records relationships between changes, not the
change's own destination, and it is append-only prose.

## Resolution order
`repo.go` gains `RuleChangeTarget` between the explicit flag and gh's own detection:

    -repo flag  →  change targets  →  gh default  →  origin  →  upstream

A change with no targets resolves exactly as today, which keeps every repo-local
setup working untouched.

## Store root resolution
    -openspec flag  →  -store <id>  →  openspec/config.yaml `store:`  →  ./openspec

The registry is OpenSpec's own (`~/.openspec/`). specsync reads it rather than
keeping a second registry, and shells out to `openspec store list --json` so the
format stays OpenSpec's to change.

## Verify
specsync did not parse `specs/` deltas at all: `Change` carried proposal, tasks,
design and links, but no behaviour contract. That absence is why the Verify phase
was unreachable — there was nothing to verify against.

`Verify` reads a change's deltas and reports the acceptance checklist a reviewer
works through. A change with no deltas is reported as uncontracted and warns
rather than fails: forcing a contract onto a dependency bump is exactly how specs
become the ceremony their critics describe.

`verify` already existed as a release-traceability sweep. It keeps that behaviour
when unscoped, and verifies one change against its spec when given `-change`.

## Upstream first
`targets` answers a question OpenSpec leaves open. File it upstream before shipping
a private format, and match whatever shape is proposed so convergence is cheap. If
OpenSpec adopts an equivalent field, specsync reads that and treats its own as a
deprecated alias.
