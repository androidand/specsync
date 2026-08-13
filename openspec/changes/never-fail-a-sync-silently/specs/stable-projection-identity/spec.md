# A sync that fails must fail loudly

## ADDED Requirements

### Requirement: A failed sync exits non-zero and is counted

`specsync sync` SHALL exit non-zero when any change fails to sync, and its summary
SHALL report a failure count alongside the created and updated counts.

Failed changes SHALL be listed at the end of the run. In a bulk run an inline error
is separated from the summary by every subsequent change's output — observed
2026-08-14, where a GraphQL failure appeared 40 lines above a closing line reading
`specsync: 27 created, 16 updated`, and the change silently never reached the tracker
while the command exited 0.

A tool whose contract is "the tracker matches the specs" SHALL NOT report success
when the tracker does not match the specs. CI gating on `specsync` SHALL be able to
detect a dropped change.

The taxonomy of failed / skipped / no-op SHALL be explicit, and which of them affect
the exit code SHALL be stated rather than implied. A directory with no `proposal.md`
is skipped rather than failed, but skips SHALL still be reported — one such directory
sat untracked for months precisely because the skip was silent.

#### Scenario: One change of many fails

- **WHEN** a bulk sync writes 44 changes successfully and one fails
- **THEN** the command exits non-zero, the summary reports one failure, and the
  failed slug is listed at the end of the run

#### Scenario: All changes sync

- **WHEN** every change syncs successfully
- **THEN** the command exits zero and reports no failures

#### Scenario: A directory is skipped, not failed

- **WHEN** a change directory has no `proposal.md` and is skipped
- **THEN** it is reported as skipped, distinctly from a failure

### Requirement: An oversized body is refused before the write

`specsync` SHALL measure the rendered issue body and refuse a change whose body
exceeds the tracker's limit, before attempting the write.

The refusal SHALL name the change, the rendered byte count, the limit, and the
largest contributing section, so the operator can act without reading the source or
GitHub's error. GitHub's own message — `Body is too long (maximum is 65536
characters)` — names none of these.

The refusal SHALL state a remedy. An oversized change is nearly always one whose
`tasks.md` has accumulated completion evidence under finished sections; the message
SHALL point at the epic/sub-issue split rather than leaving the operator to invent
one.

specsync SHALL NOT truncate a body to fit. An issue that silently omits half its
tasks is worse than no issue, because it looks correct.

specsync SHALL NOT split a change automatically. Where a change divides is a
judgement about the work, and a tool that restructured someone's spec unasked would
be a worse failure than one that refuses.

#### Scenario: Body exceeds the limit

- **WHEN** a change renders to 71,175 bytes against a 65,536-byte limit
- **THEN** it is refused locally with the change name, both byte figures, and the
  largest contributing file named, and no network call is made

#### Scenario: Body is within the limit

- **WHEN** a change renders within the limit
- **THEN** it syncs normally with no size diagnostics

#### Scenario: Oversized change after splitting

- **WHEN** a previously oversized change is split so each part fits
- **THEN** each part syncs cleanly

#### Scenario: Truncation is never a remedy

- **WHEN** a body exceeds the limit
- **THEN** the change is refused rather than sent with content removed
