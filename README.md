# specsync

[![npm](https://img.shields.io/npm/v/%40androidand%2Fspecsync)](https://www.npmjs.com/package/@androidand/specsync)
[![CI](https://img.shields.io/github/actions/workflow/status/androidand/specsync/ci.yml?branch=main&label=CI)](https://github.com/androidand/specsync/actions/workflows/ci.yml)
[![license](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Make your OpenSpec changes and your issue tracker the same thing.**

`specsync` projects [OpenSpec](https://openspec.dev) changes into an external work
tracker (GitHub Issues today; more providers planned) and keeps them in sync. An
OpenSpec change and a tracker issue describe the same work in two forms — the
rich, local, spec-driven form and the shareable, durable, human-facing form.
specsync renders one into the other, idempotently.

It is a single self-contained binary that depends only on the Go standard
library, so it runs in any OpenSpec project regardless of that project's
language.

> **This repo dogfoods itself.** Every issue here is generated from an OpenSpec
> change in [`openspec/changes/`](openspec/changes) by specsync — via the
> [`Sync specs → issues`](.github/workflows/sync.yml) workflow. The backlog you
> see *is* the spec set: open any issue to read its proposal and live task
> checklist. That workflow is also a copy-pasteable reference for keeping your
> own repo's specs and issues in sync.

## Why

OpenSpec keeps requirements out of chat history and in reviewable spec files.
But teams often want those specs to *also* live where the rest of the world
tracks work — a backlog, a board, a set of issues. Maintaining both by hand
means writing intent twice and reconciling it forever.

specsync removes the double-entry: write the spec once, and it appears (and
stays current) as an issue.

## Install

```bash
# npm (recommended) — installs the prebuilt binary for your platform
npm i -g @androidand/specsync

# Go
go install github.com/androidand/specsync/cmd/specsync@latest

# or grab a prebuilt binary from the Releases page
```

The npm package is a thin wrapper: its postinstall downloads the matching
prebuilt binary (linux/darwin, amd64/arm64) from the GitHub release, so there is
no Go toolchain or build step.

### Requirements

- **No Go toolchain** for the npm or prebuilt-binary installs — the binary is
  self-contained (Go stdlib only).
- **`gh` CLI, authenticated** — the default GitHub provider shells out to `gh`;
  check with `gh auth status`.
- **Node >= 16** — only for the npm wrapper's install shim.
- **Platforms**: linux and macOS (darwin) on amd64/arm64. No Windows binary
  today.

The npm installer verifies the downloaded archive against the release SHA-256
checksums before extracting it. On a supported platform, download, checksum, or
extraction failures fail the npm installation instead of leaving a successful
but unusable `specsync` command. Unsupported platforms should use the Go install
command or download a compatible release binary directly.

## Usage

Run it from a repo that has an `openspec/` directory (no OpenSpec yet? see
[openspec.dev](https://openspec.dev) — `openspec init` scaffolds one), with
`gh` authenticated for that repo's `origin` (`gh auth login` if `gh auth
status` fails).

**Always `-dry-run` first** in a new repo — it makes zero API calls and never
touches local state:

```bash
specsync -dry-run            # preview the gh commands + rendered issue bodies (safe)
specsync -dry-run -slug X    # preview a single change
specsync                     # create/update issues for every change
specsync -slug X             # sync just one change
specsync -openspec path/to/openspec   # point at a non-default openspec dir
```

All subcommands, at a glance:

```bash
specsync [sync]          # project changes -> issues (default command)
specsync pull            # pull an issue into a local change
specsync scan            # what already exists in an area?
specsync trace           # print the raw spec<->commit<->issue link graph
specsync link            # cross-link two or more changes
specsync release-plan    # shipped changes + advisory semver bump
specsync changelog       # Keep a Changelog section from shipped changes
specsync install-skill   # install the bundled agent skill
specsync version         # print the binary version
```

**Dry-run flags** — `sync`, `pull`, and `link` support `-dry-run`. Beads can be
previewed through `specsync -dry-run -provider beads`. `scan`, `trace`,
`release-plan`, and `changelog` (unless `-apply`) are read-only and do not take
a dry-run flag.

Flags come **before** positional arguments (standard Go flag parsing):
`specsync scan -json cmd/ auth`, not `specsync scan cmd/ auth -json`.

### Choosing a provider: `-provider beads`

The default provider is `github` — human-facing issues via the `gh` CLI. Pass
`-provider beads` to project the same changes into a local
[Beads](https://github.com/steveyegge/beads) graph via the `bd` CLI instead
(agent-facing; ignores `-repo`):

```bash
specsync -dry-run -provider beads    # preview the bd commands
specsync -provider beads -slug X     # project one change into the beads graph
```

### Issue-first: pull an issue into a change

Work often starts on the tracker — someone files an issue first. `specsync pull`
reads that issue and scaffolds a local OpenSpec change from it, so you can plan
it as a spec and keep syncing:

```bash
specsync pull -issue 42              # issue 42 -> openspec/changes/<slug>/
specsync pull -issue 42 -dry-run     # read the issue, show what would be written
specsync pull -issue 42 -slug my-feature   # override the derived slug
```

`pull` writes `proposal.md` (from the issue body, titled by the issue) and
`tasks.md` (from a `## Tasks` checklist when present), and links the change to
the issue so a later `specsync` push updates that same issue. A dry run reads the
issue but writes nothing.

### `scan` — what already exists here?

Run before planning new work. Give it an area — one or more paths and/or a
free-text topic (required) — and it lists related OpenSpec changes, open issues
with no linked change, and recent commits touching that area:

```bash
specsync scan cmd/ auth          # area = the cmd/ path + the topic "auth"
specsync scan -json pkg/api      # machine-readable, for a planning agent
```

### `link` — cross-link changes

Records each change's issue URL in the others' `links.md` and re-syncs them so
a `## Related` section appears in every linked issue:

```bash
specsync link -dry-run slug-a slug-b   # preview links.md + Related sections
specsync link slug-a slug-b            # write links and update both issues
```

### `trace` — the raw link graph

Prints the resolved trace graph — changes, commits, issues, and the links
between them — for debugging or scripting:

```bash
specsync trace -change my-feature      # scope to one change
specsync trace -since v0.3.0 -json     # commits since a tag, as JSON
```

### `release-plan` — advisory follow-up report

Read-only report over a revision range (default: latest tag → `HEAD`): shipped
changes, loose ends, archive candidates, and an advisory semver bump. It
detects your release tool (e.g. goreleaser) and defers to it — the bump is
advice, not action:

```bash
specsync release-plan                  # since the latest tag
specsync release-plan -since v0.3.0 -json
specsync release-plan -fail-on-archive-candidates
```

For release hygiene, run `specsync release-plan -fail-on-archive-candidates`
in CI/release checks. It exits non-zero when shipped changes with fully
completed tasks are still unarchived in `openspec/changes/`.

### `changelog` — a changelog generated from your specs, not your commits

Commit-log changelogs are noise: `chore`, `wip`, squash messages. `specsync
changelog` builds a [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
section from what actually shipped — one entry per OpenSpec change in the
revision range, in plain language, grouped into Added/Changed/Fixed/Removed/
Security:

```bash
specsync changelog                     # preview the section for latest tag -> HEAD
specsync changelog -release-notes      # bare body, for `goreleaser --release-notes`
specsync changelog -apply              # write/replace the section in CHANGELOG.md
```

Entry text comes from an optional `## Release note` section in the change's
`proposal.md` — written at planning time, reviewed in the issue like everything
else — falling back to the proposal title. The category comes from OpenSpec
requirement deltas (added/changed/removed) and linked commit types; a change
with no deltas and all-`fix` commits lands under Fixed, any `feat` under Added.
Commits that link to no change still surface honestly (a loose `feat`/`fix`
is included; plumbing commits like `chore`/`docs`/`ci` are counted, not
silently dropped). `-apply` is idempotent — re-running replaces that version's
section in place — and defers to a release tool that already owns the
changelog (release-please, changesets, …) unless you pass `-force`.

### Projects boards: `-project owner/number`

Project a synced change onto a GitHub Projects (v2) board — the issue is added
to the board, its Status follows the change's stage, and the acting user is
assigned:

```bash
specsync -project my-org/6                        # sync + project onto board 6
specsync -project my-org/6 -status-map "active=In Progress,archived=Done"
SPECSYNC_PROJECT=my-org/6 specsync                 # env var, so it need not be retyped
```

Unset (the default), specsync makes zero board calls — completely
backward-compatible. Status option names resolve case-insensitively against
the board's own schema (never hard-coded ids), so a stock "Todo / In Progress /
Done" board works out of the box; `-status-map` (or `$SPECSYNC_STATUS_MAP`)
overrides the stage→Status names explicitly and fails loud on an unknown name.
specsync never clobbers a Status or assignee it didn't set itself, and
`-dry-run` previews the board plan with zero GraphQL calls.

### `install-skill` — install the agent skill

Installs the bundled specsync `SKILL.md` into agent skill directories so
coding agents know how to drive the tool:

```bash
specsync install-skill --all           # every known agent directory
specsync install-skill --claude-code   # or: --codex --opencode --copilot --agents
```

The `--agents` flag installs the generic agentskills.io-compatible `.agents`
copy. OpenCode has its own `--opencode` destination.

### `version`

`specsync version` (also `-version` / `--version`) prints the binary version.
Release builds stamp the real version; source builds print `dev`.

## OpenSpec Workflow (Teams)

OpenSpec has become the go-to planning layer for many developers. `specsync`
extends that model for teams working on:

- large codebases
- multi-repo planning
- customization and integrations
- better collaboration

### Lifecycle discipline

Treat OpenSpec as an active planning lifecycle, not a one-off document dump:

1. `propose` — define intent in `openspec/changes/<slug>/proposal.md`
2. `tasks` — define execution in `tasks.md`
3. `apply` — implement and check off tasks
4. `sync` — project and reconcile with tracker issues via `specsync`

Both paths are first-class:

- spec-first: author local change, then run `specsync`
- issue-first: start from issue, run `specsync pull`, then continue with `specsync`

### `.status` and stage labels

OpenSpec natively gives active/archived lifecycle via folder location.
Optionally add `<change>/.status` for richer workflow stages. `specsync` maps
that value to a `stage:<name>` label on the projected issue.

### Check-in policy (intentional and contextual)

`specsync` supports two valid team patterns:

- **tracked OpenSpec** (like this repo): keep `openspec/` in git for public
  dogfooding and auditability.
- **local OpenSpec** (common in enterprise monorepos): keep OpenSpec/Beads as
  local planning artifacts and sync the durable collaboration surface to issues.

The tool is intentionally neutral: it reduces noise and friction either way, by
keeping issue tracking and spec planning synchronized.

### OpenSpec CLI usage boundary

`specsync` keeps file-based parsing as the baseline so it works even when the
OpenSpec CLI is unavailable. Teams can optionally run OpenSpec CLI checks
locally or in CI when they want stricter lifecycle validation.

## How it works

```
openspec/changes/<slug>/          ->  Change      (proposal.md, tasks.md, .status)
Change                            ->  WorkItem    (title, body, stage, labels)
WorkItem                          ->  issue       (via a pluggable provider)
```

- **Identity** — each issue body carries an `<!-- specsync:change=<slug> -->`
  marker. That marker is the durable link; the issue number is only cached
  locally. Lose the cache and specsync rediscovers the issue by its marker.
- **Idempotent** — running again *updates* the same issue; it never duplicates.
- **Body** — `proposal.md` becomes the issue body; `tasks.md` is rendered as a
  task-list checklist so the tracker shows progress.
- **Two-way task state** — a normal sync also merges checkbox state *back* from
  the issue into `tasks.md` (a box ticked on GitHub sticks), then pushes the
  merged result. The merge is a monotonic union — a task is done if either side
  marked it done — so a teammate's tick on the issue is captured without ever
  reverting un-pushed local progress. Spec still wins task *wording* and order;
  only the checkbox flips. Disable with `-reconcile=false`. Dry runs never read
  or write, so reconcile applies only on real syncs.
- **Stage** — each issue gets a `stage:<stage>` label, derived automatically:
  `active` while any task is unchecked, `complete` once every task is checked
  (before archiving), and `archived` once the change moves under
  `changes/archive/`. So finishing the last task flips the issue out of
  `stage:active` on the next sync — no manual bookkeeping. Add `-close-completed`
  to keep the issue's open/closed state aligned too: completion closes it and
  new unchecked work reopens it. Write a richer stage name into
  `<change>/.status` to override the derived value; an explicit `complete`
  closes with the flag, while any other explicit stage remains open.
- **Local cache** — projection ids live in a gitignored `<change>/.specsync/`
  directory, never in git.

## License

MIT — see [LICENSE](LICENSE).
