# Launch readiness — confident public promotion of specsync

## Why

specsync is already public-shaped: the repo dogfoods itself, the binary is on
GitHub Releases, and the npm wrapper `@androidand/specsync` is published at
v0.4.0. "Launch-ready" for this project therefore does not mean making it
public — it means being able to *promote* it to strangers with confidence:

- A stranger arriving from npm or GitHub can understand what it does, install
  it, and use every shipped subcommand from the README alone.
- The npm package is discoverable (keywords) and correctly attributed
  (author, bugs URL) so the registry page looks maintained.
- A secrets/private-info audit of tracked files **and git history** has been
  run and its results recorded, so promotion carries no leak risk.

An audit on 2026-07-04 found the repo close to ready. Tracked files contain no
secrets, no private paths (`/Users/...`), no LAN IPs, and no personal data;
`git log --all -S` spot checks for token prefixes (`ghp_`, `github_pat_`,
`sk-ant`), private paths, and LAN IPs found nothing in history either. LICENSE
(MIT), `.gitignore`, CI, release automation, and the built site are all in
place. The remaining gaps are documentation and metadata polish plus a few
owner-only decisions.

## What Changes

- **README rewrite for a stranger**: document all shipped subcommands
  (`scan`, `trace`, `link`, `release-plan`, `install-skill`, `pull`, `sync`)
  and the `-provider beads` option — today the README covers only push and
  `pull`. Add a requirements section (Go-free install, `gh` CLI needed and
  authenticated) and npm/CI badges.
- **npm package metadata**: add `keywords`, `author`, and `bugs` to
  `npm/package.json` so the registry page is discoverable and reportable.
- **CLI polish**: add a `-version`/`version` output to the binary (the npm
  wrapper knows its version; the binary itself cannot report one).
- **Owner decisions**: confirm GitHub repo visibility/description/topics,
  decide whether to claim the unscoped `specsync` npm name (currently
  unpublished — the scoped `@androidand/specsync` is what exists), and decide
  whether the mixed commit-author emails in history are acceptable (they are —
  no history rewrite is proposed or needed).

### Out of scope

- Any git history rewrite (nothing found that would justify one).
- New features, providers, or behavior changes.
- Homebrew tap (already tracked on the roadmap).

## Capabilities

### New Capabilities

- `launch-readiness` — the repo presents itself completely and safely to a
  first-time visitor: full CLI surface documented, package metadata complete,
  no sensitive data in tracked files or history.

## Impact

- Docs: `README.md` (rewrite of Usage; badges; requirements).
- Packaging: `npm/package.json` (metadata only — no behavior change).
- Code: `cmd/specsync/main.go` (a `version` subcommand / `-version` flag;
  wired into `.goreleaser.yaml` ldflags).
- No change to sync/pull/reconcile behavior; existing tests untouched.

## Audit notes (for the record)

- Tracked-file grep for `api key|token|password|secret|bearer` matched only
  documentation about *not* committing secrets, CI `${{ github.token }}`
  usage, and code comments — no findings.
- Grep for `/Users/`, `/home/`, `192.168.`, `10.0.`, personal emails — no
  matches in tracked files.
- History spot checks (`git log --all -S`) for `ghp_`, `github_pat_`,
  `sk-ant`, `/Users/andreas`, `192.168.` — no matches.
- Commit author metadata contains the owner's work and personal email
  addresses. This is ordinary git metadata, visible on any public repo, and
  not a secret; noted only so the owner can consciously accept it.
- The four copies of `SKILL.md` are byte-identical (kept in sync by
  `make sync-skill`) — no drift.
