# Pull a GitHub issue into a local OpenSpec change

## Why

specsync only flows one way today: an OpenSpec change becomes an issue. But work
often starts on the tracker side — a human or an orchestrator files an issue
first, sometimes nearly empty, then wants to plan it as a spec. Without a way to
seed a local change from an existing issue, that planning happens off-spec and
the issue↔change link is invented by hand.

This is the missing reverse direction, and it is the first capability needed to
adopt specsync in a real multi-repo workflow: turn an existing issue into the
starting point of a spec.

## What Changes

- Add an **issue-first pull**: given a provider issue id, specsync materializes a
  local `openspec/changes/<slug>/` from that issue (`proposal.md` from the body,
  `tasks.md` from a `## Tasks` checklist when present).
- Add an optional, type-asserted provider capability **`IssueReader`** so the
  core can read an existing item without bloating the minimal `WorkProvider`
  contract. Implement it for the GitHub provider via `gh issue view`.
- Establish the round-trip link on pull: derive (or reuse) the change slug, strip
  the identity marker out of the proposal body, and cache the ref so a later
  `push` updates the same issue instead of creating a duplicate.
- Add a `pull` subcommand to the CLI with `-dry-run` parity.

This change keeps the engine provider-agnostic and the GitHub specifics behind
the provider — the interface work that later admits other trackers, while we
deliberately ship **GitHub only** now.

### Out of scope
- bidirectional conflict resolution (a later change)
- branch-based link resolution (the `spec-issue-linker` change)
- epic and sub-issue projection
- non-GitHub providers — this slice is deliberately GitHub-only

## Capabilities

### New Capabilities
- `issue-pull` — materialize a local OpenSpec change from an existing tracker issue.

### Modified Capabilities
- `work-provider` — gains the optional `IssueReader` read capability.

## Impact

- New code: `pull.go`, `IssueReader` on `provider.go`, `Get` on the GitHub
  provider, a `pull` subcommand in `cmd/specsync`.
- No change to the existing push path; `Sync` and its tests are untouched.
- Enables the first real downstream use: seeding a portal onboarding spec from an
  existing GitHub issue.
