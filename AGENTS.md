# Agent Workflow

This repo uses OpenSpec as the planning layer and `specsync` as the tracker-sync
layer.

## Principles

- OpenSpec is the single source of planning truth.
- `specsync` is a tracker-agnostic sync engine: it projects an OpenSpec change
  into whatever tracker a project uses (GitHub by default; Beads and others
  behind the `WorkProvider` interface) and reconciles task state back. One source
  of truth, many projections — state never flows tracker-to-tracker.
- `specsync` only synchronizes. It is not a memory store (long-term memory, if a
  tracker offers it, is that tracker's concern — e.g. Beads' own `bd prime`
  session hook handles its memory; specsync never reads or writes it), not a
  control plane (it owns no orchestration), and it runs invoke-and-exit (no
  daemon, no background state).
- Trackers/issues are projections and the collaboration surface; `specsync` keeps
  them aligned with the spec.

## Rules

- MUST plan work in `openspec/changes/<slug>/` before broad implementation.
- MUST keep `proposal.md` and `tasks.md` accurate while working.
- MUST run `specsync` with `-dry-run` before mutating tracker state.
- MUST add or update tests for code behavior changes.
- MUST NOT commit `.beads/` artifacts.
- MUST NOT commit local `.specsync/` caches from change folders.

## Dogfooding (non-negotiable)

This repo's own backlog, changelog, and site are the public proof that specsync
works — not a marketing claim, a live artifact anyone can check. Treat a bad
changelog entry as a real bug in this project, same severity as a failing test.

- MUST NOT commit code changes without a linked `openspec/changes/<slug>/`
  change and its synced issue. A commit with no linked issue silently degrades
  `specsync changelog` from an authored release note to a raw
  `<commit description> (<hash>)` line — the exact failure mode that makes past
  releases here read like an unfiltered `git log`, not a product changelog.
- MUST reference the change's issue number in the commit message or PR (e.g.
  `(#42)`) so `specsync changelog` can bind the commit to its change. An
  unlinked but otherwise-fine commit is what produces the embarrassing
  fallback entries — this is the single most common way dogfooding quietly
  breaks.
- MUST run `specsync changelog -fail-on-unlinked-commits` before considering a
  change complete — CI runs this on every PR too (`.github/workflows/ci.yml`),
  so a commit missing an issue reference fails the build rather than silently
  degrading `CHANGELOG.md`. If it fails, either link the commit to its issue,
  or add a `## Release note` section to the change's `proposal.md` (see
  `ReleaseNote()` in `changelog.go` — it prefers that section, falling back to
  the proposal title only when absent).
- MUST update `site/features.json` (and `README.md` where relevant) in the
  same change when it adds or changes a user-facing capability. The site is
  not a follow-up task — it ships with the change, not after it.
- Title hygiene feeds this directly: `ReleaseNote()` falls back to the
  proposal's raw H1 when there's no release-note section, so an unclean title
  (parentheticals, backtick tool names, implementation detail) becomes the
  permanent changelog line, not just an ugly issue title. See
  `openspec/changes/title-hygiene-on-pull/`.

## Security

- This is a public repository.
- MUST NOT commit sensitive information, credentials, tokens, keys, personal or internal data
- MUST scrub examples, logs, and test fixtures for secrets before commit.
- When in doubt, treat data as sensitive and keep it out of git.

## Working Paths

- Spec-first path:
  1. Create/update change in `openspec/changes/<slug>/`.
  2. `specsync -dry-run -change <slug>`.
  3. `specsync -change <slug>`.

- Issue-first path:
  1. `specsync pull -issue <n> [-change <slug>]`.
  2. Refine `proposal.md` and `tasks.md`.
  3. `specsync -dry-run -change <slug>` then `specsync -change <slug>`.

## Completion Rule

- When work is complete, ensure tasks are checked, sync once more, and archive
  the completed OpenSpec change.

## Commit Messages

- Brief, concise, informative — describe the change and why.
- MUST NOT mention people or agents (no co-author trailers, no attributions).
- Conventional-commit prefixes (`feat:`, `fix:`, `build:`, `chore:`) are fine.

## Documentation Style

- Keep docs concise and practical.
- Avoid AI-bloated wording and repetition.
- Prefer direct instructions and concrete examples.
