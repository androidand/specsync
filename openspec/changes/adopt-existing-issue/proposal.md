# Adopt an existing issue into an existing change

## Why

A change and its issue can both already exist, independently, with nothing
linking them. It happens whenever more than one person or agent works a feature:
someone files the tracker issue by hand while someone else writes the OpenSpec
change, and neither artifact knows about the other.

specsync has no way to say "these two are the same work". `pull` is issue-first
and overwrites the local change from the issue, which is wrong when the local
change is the richer artifact. `link` cross-links changes to each other, not a
change to an issue. So the only route today is to hand-edit the issue body to
add the `<!-- specsync:change=<slug> -->` marker and hope the next sync finds
it.

That route has a race, and the race is not rare. Marker discovery goes through
GitHub's code search index, which lags behind writes by seconds to minutes. A
sync run inside that window finds nothing and creates a duplicate issue —
precisely the outcome the marker was added to prevent. Recovering means closing
the duplicate, stripping its marker so the search stops matching it, and
repointing `.specsync/refs.json` by hand.

Observed on ExopenGitHub/FusionHub: issue #3691 was created by hand, a marker
was added to it, a sync seconds later created duplicate #3692, and a second
sync then wrote to the closed duplicate because the local ref cache still
pointed there. Two agents were meanwhile fixing the same problem from different
worktrees, one by seeding the ref cache and one by editing the body.

## What

**`specsync adopt -issue <n> [-change <slug>]`** — bind an existing change to an
existing issue. Writes `.specsync/refs.json` first, so the link holds
immediately and does not depend on search indexing, then adds the marker to the
issue body for durability and for other worktrees. Refuses when the change is
already bound to a different issue unless `-force` is given, and refuses when
the issue already carries a marker for a different change.

This is the missing third verb. `pull` creates a change from an issue, `sync`
projects a change onto an issue, `adopt` states that an existing pair belong
together.

**Duplicate detection on sync.** When marker search returns more than one
candidate issue, stop rather than pick. Report the candidates and point at
`adopt` to disambiguate. Today the resolver takes a first hit, which is how a
sync can silently start writing to the wrong issue of a pair.

**Refuse to write to a closed issue** unless `-force`. A closed issue is the
strongest available signal that a ref is stale, and writing to one produces a
sync that reports success while updating something nobody reads.

**Cache-first ordering when the cache was just written.** `adopt` marks the ref
as locally authoritative so the immediately following `sync` trusts it over a
search that has not caught up.

## Relationship to `spec-issue-linker`

That change (status: researched) defines a chained `Linker` that resolves a
change to an issue from branch name, marker, cache, or an external source. This
change is complementary and narrower: `Linker` infers a link that already
exists somewhere, `adopt` declares one that exists nowhere. Duplicate detection
and the closed-issue guard belong to whichever lands first; they are specified
here because the failure they prevent was observed here.

## Impact

- New command, `adopt`. No change to `sync` or `pull` semantics beyond the two
  guards.
- Both guards can reject flows that silently "worked" before, which is the
  point: they were reporting success while writing to the wrong issue.
- Needs an `agent-help` entry, since agents hit this more than humans — an
  orchestrator filing issues while a worker writes specs is the normal case.
