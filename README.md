# specsync

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
no Go toolchain or build step. A Homebrew tap is on the roadmap.

## Usage

Run it from a repo that has an `openspec/` directory, with `gh` authenticated for
that repo's `origin`:

```bash
specsync -dry-run            # preview the gh commands + rendered issue bodies (safe)
specsync -dry-run -slug X    # preview a single change
specsync                     # create/update issues for every change
specsync -slug X             # sync just one change
specsync -openspec path/to/openspec   # point at a non-default openspec dir
```

**Always `-dry-run` first** in a new repo — it makes zero API calls and never
touches local state.

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
- **Stage** — OpenSpec has no native lifecycle beyond active/archived, which
  specsync derives from the folder location. Write a richer stage name into
  `<change>/.status` and it becomes a `stage:<name>` label.
- **Local cache** — projection ids live in a gitignored `<change>/.specsync/`
  directory, never in git.

## Roadmap

Tracked as OpenSpec changes in this repo's own `openspec/changes/` — dogfooding
the tool on itself:

- **Pluggable providers** — a `WorkProvider` interface so the same engine can
  target raw `gh`, an MCP work-management server, or self-hosted trackers
  (Vikunja, Plane, Forgejo) without changing the core.
- **Spec↔issue linker** — resolve the link from branch name / marker / cache /
  MCP relations, so both issue-first and spec-first flows work.
- **Epic & sub-issue projection** — model an epic issue whose sub-issues each
  become a focused spec.
- **Distribution** — Homebrew tap and an `npm` wrapper for one-line global
  install.

## License

MIT — see [LICENSE](LICENSE).
