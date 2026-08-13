# A sync that fails must fail loudly

## Why

A bulk sync of 45 OpenSpec changes reported success and silently dropped one of them.

Reproduced 2026-08-14 in `androidand/llama-skein`:

```
$ specsync -change host-model-management-api
32 done --label specsync --label stage:active: exit status 1
GraphQL: Body is too long (maximum is 65536 characters) (createIssue)

specsync: 0 created, 0 updated
$ echo $?
0
```

**Exit code 0.** Summary line `0 created, 0 updated` — indistinguishable from a
no-op. No failure count, no non-zero status, and in the bulk run the GraphQL error
scrolled past inside 40 lines of normal output while the final line read
`specsync: 27 created, 16 updated`. The change simply never reached the tracker, and
nothing said so.

Three defects compound here, and the third is the one that matters:

1. **No pre-flight size check.** specsync builds the body and hands it to GitHub,
   which rejects it at 65,536 characters. specsync knows the body length before it
   sends anything. A 71,175-byte body is knowable locally, instantly.
2. **No actionable message.** "Body is too long" is GitHub's error, not specsync's.
   It names no change, no file, no remedy, and does not say which part of the body
   (proposal? tasks? discoveries?) is oversized.
3. **Failure is not surfaced as failure.** This is the real bug. The other two are
   about one error class; this one means *any* per-change error in a bulk run can
   vanish. A tool whose job is "keep the tracker in sync with the specs" reporting
   success while the tracker is out of sync is failing at its only job — and CI
   gating on `specsync` would go green.

The size limit is the symptom that exposed it. The silent-failure behaviour is the
bug.

## What Changes

- **Non-zero exit when any change fails to sync.** A bulk run that syncs 44 of 45
  exits non-zero. This is the load-bearing change; without it nothing else is
  enforceable in CI.
- **A failure count in the summary.** `specsync: 27 created, 16 updated, 1 failed`,
  with the failed slugs listed at the end where they are readable — not buried
  mid-stream next to the change that happened to be adjacent.
- **Pre-flight body-size validation.** Measure the rendered body before sending;
  refuse locally with a message naming the change, the byte count, the limit, and the
  largest contributing section. `specsync -dry-run` and `openspec validate` should
  both be able to surface an oversized change before anyone attempts a write.
- **A stated remedy, not just a rejection.** Point at splitting: an oversized change
  is nearly always one whose `tasks.md` has accumulated completion evidence. The
  epic/sub-issue machinery already being built (`epic-and-subissue-projection`,
  `epic-scaffold-command`) is the structural answer, and this change should hand off
  to it rather than reinvent splitting.

## Capabilities

### Modified Capabilities

- `stable-projection-identity`: sync failures are reported and exit non-zero;
  oversized bodies are caught before the write.

## Non-Goals

- **Not** automatic splitting. Deciding where a change divides is a judgement about
  the work, not a mechanical operation, and a tool that silently restructured
  someone's spec would be worse than one that refuses. Detect, explain, refuse —
  the operator or `epic-scaffold-command` splits.
- **Not** truncating the body to fit. A tracker issue that silently omits half the
  tasks is a worse failure than no issue at all, because it looks correct.
- **Not** a general retry or resilience layer. This is about *reporting* what
  happened, not recovering from it.
- **Not** re-implementing epic/sub-issue projection. That work exists; this change
  routes to it.

## Open Questions

- **Should an oversized change be a hard refusal or a warning that still attempts?**
  Refusing is honest but blocks a sync that might have succeeded if GitHub's limit
  ever changes. Leaning refusal, since the limit is a documented API constraint and
  attempting a doomed write wastes a round trip and produces GitHub's unhelpful error
  instead of ours.
- **Where does the size check belong** — `sync` only, or also `openspec validate`
  via a specsync lint? Catching it at authoring time is much cheaper than at sync
  time, but it means specsync knowledge leaking into a validate step it does not own.
- **Does `-dry-run` currently render the full body?** If so the check is nearly free
  there. If dry-run short-circuits before rendering, it would need to render to
  measure — which is arguably what dry-run should do anyway.
- **What counts as "failed" for the exit code?** A change skipped because it has no
  `proposal.md` (e.g. a notes-only directory) is arguably not a failure. Silent
  skips have their own risk, but conflating them with real errors would make the
  exit code noisy enough to be ignored.

## Impact

- The sync driver's per-change error handling and its summary line.
- Exit-code semantics for `specsync sync` — a behavioural change for any CI already
  invoking it, which is the point, but worth a release note.
- A new pre-flight validation step with the byte budget and section attribution.
- Hand-off to `epic-and-subissue-projection` / `epic-scaffold-command` for the split.
