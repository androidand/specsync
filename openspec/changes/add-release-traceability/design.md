# Design: release traceability (the follow-up report)

Builds on `add-change-traceability-model/design.md` (shared architecture,
constraints, and philosophy). Covers only what is specific to the report.

## What the report is for

The headline is not "release management" — it is **follow-up that doesn't
require discipline you don't have**. One command, run at the end of a work
session or before a release, that reconstructs what happened and surfaces what's
loose:

```
Follow-up  (v0.2.1..HEAD)

Shipped
  cross-repo-linked-issues   #6    PR #51   4 commits
  living-plan                #2    PR #57   2 commits

Loose ends
  PR #53 (feat: bulk import) links to no OpenSpec change
  3 commits on main reference no change or issue

Archive candidates  (all tasks done)
  cross-repo-linked-issues

Advisory bump   minor   (v0.2.1 → v0.3.0)
Why
  feat(ui): split integration modal
  change living-plan added a requirement

Release path (detected)
  tool:      goreleaser + manual git tags  → owns bump, tag, publish
  changelog: GitHub-generated release notes → owns changelog
  specsync defers to these; the bump above is advisory only
```

The "loose ends" and "archive candidates" sections are the point — the follow-up
a human would otherwise reconstruct by hand and therefore never does. They come
straight from the trace graph's reported gaps and from the existing
task-completion signal (`tasks.md` / `.status`).

## Advisory bump: signals → recommendation

`ReleaseImpact` is `none | patch | minor | major` plus `Reasons []string`,
computed as the **maximum** impact across signals so any breaking signal wins:

| Signal | Source | Contributes |
|---|---|---|
| Commit `type` | `git log` (parsed) | `feat`→minor, `fix`→patch, others→none |
| `!` marker or `BREAKING CHANGE:` footer | `git log` (parsed) | major |
| OpenSpec requirement delta | `openspec show --json --deltas-only` | `REMOVED`→major; `ADDED`→minor; `MODIFIED`→patch |

The **spec-delta signal is the whole reason this lives in specsync** rather than
in a commit-only release tool: a change whose delta `REMOVED` a requirement is a
major even if its commits were all `refactor`. Crucially, that delta comes from
the `openspec` CLI (verified: `show --json --deltas-only` returns the `operation`
per delta) — specsync defers to OpenSpec for the spec model, it does not re-parse
markdown. When `openspec` is unavailable the signal is reported missing, never
guessed. Reasons are human strings shown verbatim — the *why* matters as much as
the bump. No config overrides yet (deferred); the default mapping is fixed until
a real need earns configuration.

## Release impact is a join with git history, not a call at HEAD

This is the subtlety that makes or breaks the feature. `openspec show
--deltas-only` returns the deltas in the **current working tree**, for changes
still active under `changes/`. A release signal is **historical**: "what
requirements changed between v1.3 and v1.4." Those two are not the same query.

So release impact over a range is **OpenSpec deltas ⨯ git history**:

1. From the `CommitSource`, walk the range `[since, until]` and find the changes
   that were **completed or archived** in it (an archive folds a change's deltas
   into the baseline `openspec/specs/` and removes it from `changes/`; that
   archive is a commit in the range).
2. For each such change, obtain its requirement deltas:
   - **still active** (the common near-term case — unreleased work since the last
     tag) → read directly via `openspec show <change> --json --deltas-only`;
   - **already archived within the range** → its deltas are no longer in
     `changes/`, so reconstruct them from the change's spec files at the git ref
     where it still existed (or from the baseline diff the archive produced).
3. Combine those deltas with the commit signals (the maximum-impact rule).

The near-term path — "advise the bump for everything since the last tag" — is the
easy 80%: the changes are still active, one `openspec show` each. The historical
path (a range spanning archives) is where the real work is, and the design owns it
rather than pretending `openspec` at HEAD is the whole source. The foundation
supplies both halves (`OpenSpecSource`, `CommitSource`); this join is the feature.

## No accepted baseline yet: deltas are all ADDED

Until a project has archived its first change, `openspec/specs/` is empty (verified:
`openspec list --specs` returns "No specs found" for this repo today). With no
baseline to diff against, **every requirement delta is `ADDED`** — there is no
`MODIFIED` or `REMOVED` possible. Resolution:

- Pre-first-baseline, the spec-delta signal can contribute at most `minor` (all
  `ADDED`); a `major` in that state can only come from a commit breaking marker,
  never from spec deltas. The report says so rather than implying a missing major.
- Post-first-baseline, `MODIFIED`→patch and `REMOVED`→major become reachable as
  changes diff against the accreted baseline.

This is the same truth viewed twice: deltas are relative to a baseline that
accretes through `archive` over git history — which is exactly why the signal is a
git join, not a snapshot.

## SemVer, hand-rolled and minimal

`semver.go`: parse `MAJOR.MINOR.PATCH[-pre][+build]`, compare, apply a bump.
No ranges, no constraint solving — only "what's the next version." Hand-rolled
for stdlib-only. Pre-release/build metadata preserved on read, dropped on a
normal bump.

## Release-tool detection: probe, report, defer (light)

Filesystem-only; never imports or runs the tool. Each detector returns a small
descriptor (name, detected, evidence paths, responsibilities owned). Kept
deliberately light — detect the common tools, otherwise report "custom" or
"none." The point is not an adapter framework; it is one honest line in the
report that keeps specsync visibly in its lane:

| Tool | Evidence | Note |
|---|---|---|
| release-please | `release-please-config.json`, manifest | |
| changesets | `.changeset/config.json` | conceptually aligned with OpenSpec (records intent) |
| release-it | `.release-it.*`, `package.json#release-it` | |
| semantic-release | `.releaserc*`, `package.json#release` | |
| standard-version | `.versionrc*` / dep | legacy/ad-hoc, not recommended |
| custom / none | `scripts.release`, Makefile target, or nothing | bump stays advisory |

For this repo the detector reports goreleaser + manual tags (custom) and
GitHub-generated notes — a useful self-test that specsync defers correctly.

## Commands and the one opt-in mutation

- `specsync trace [--change <slug>] [--since <ref>] [--until <ref>] [--json]` —
  the raw resolved graph, for scripting/debugging.
- `specsync release-plan [--since <tag>] [--until <ref>] [--json] [--apply]` —
  the human report above. **Read-only without `--apply`.** `--apply` performs only
  specsync-owned spec actions the report listed (archive a completed change) and
  still never bumps, tags, publishes, or edits tracker issues. This matches the
  existing tool's posture: a clear, named flag gates the only side effect.

## Monorepo

When packages are configured later, the report computes a per-package advisory
bump from the artifacts whose paths match each package. A single-package repo
(the no-config default, and this repo) prints one unlabeled block.

## Relationship to the living-middle changes

This report *reads* and *surfaces*; it does not reconcile. Resolving drift between
a spec and what was built is `two-way-reconcile`'s job; capturing and promoting
loose discoveries is `living-plan` + `emergent-work-spinoff`. A natural future
seam: a loose end the report surfaces ("PR with no spec") could be handed to
`specsync spinoff` to scaffold the missing change — assistance, never
enforcement. Out of scope here; noted so the boundary is deliberate.
