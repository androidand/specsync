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

## Why

OpenSpec keeps requirements out of chat history and in reviewable spec files.
But teams often want those specs to *also* live where the rest of the world
tracks work — a backlog, a board, a set of issues. Maintaining both by hand
means writing intent twice and reconciling it forever.

specsync removes the double-entry: write the spec once, and it appears (and
stays current) as an issue.

## Install

```bash
# Go
go install github.com/androidand/specsync/cmd/specsync@latest

# npm (after a tagged release is published)
npm i -g @androidand/specsync

# or grab a prebuilt binary from the Releases page
```

A Homebrew tap is on the roadmap. Releases are built for linux/darwin on amd64/arm64.

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
