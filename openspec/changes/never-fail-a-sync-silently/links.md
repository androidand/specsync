# Links

## Related

- `epic-and-subissue-projection` (this repo) — the structural answer to an oversized
  change. This change detects and refuses; that one provides the parent/child split
  the refusal should point at. Deliberately no overlap: nothing here reimplements
  splitting.
- `epic-scaffold-command` (this repo) — the front door for minting an epic and wiring
  its family. The remedy message in task 6 should name this command once it exists.

## Sequencing notes

Task 1 — non-zero exit on failure — is the actual bug and does not depend on either
epic change. Ship it first. The size-limit work (tasks 4–6) is the symptom that
exposed it and is worth noticeably less on its own: a pre-flight check that refuses
but still exits 0 has not fixed anything.

The remedy message (task 6) is the only part that wants the epic work in place. It
can ship pointing at manual splitting and be sharpened later.

## Provenance

Found 2026-08-14 while bulk-syncing 45 OpenSpec changes in
`androidand/llama-skein`. One change (`host-model-management-api`, 71,175 bytes)
failed with GitHub's `Body is too long` and was dropped; the run reported
`27 created, 16 updated` and exited 0. The change was made syncable by hand —
`tasks.md` went from 68,798 to 8,827 bytes by moving completion evidence for five
finished sections into a separate notes file — which is exactly the split the
refusal message should recommend.

A second directory in the same repo (`muse-glimmer-30b-gguf`) was skipped silently
for having no `proposal.md`, and had gone untracked for months as a result. That is
the case behind task 3: skips need reporting even when they are not failures.
