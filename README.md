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
specsync -dry-run -change X    # preview a single change
specsync                     # create/update issues for every change
specsync -change X             # sync just one change
specsync -repo owner/name      # target a specific repo (see resolution order below)
specsync -openspec path/to/openspec   # point at a non-default openspec dir

**Repo resolution order**, first match wins:

1. `-repo owner/name` (explicit flag — overrides everything).
2. `gh repo set-default` — the repository configured for this checkout, since
   that is the user's stated intent.
3. The `origin` remote.

Every `gh` invocation carries an explicit `--repo`. When `origin` and `upstream`
name different repositories (fork divergence), specsync targets `origin` and
reports the divergence. It refuses to write to the upstream parent without an
explicit `-repo` — a fork user's internal planning content should not silently
appear on someone else's repository.
```

All subcommands, at a glance:

```bash
specsync [sync]          # project changes -> issues (default command)
specsync pull            # pull an issue into a local change
specsync scan            # what already exists in an area?
specsync trace           # print the raw spec<->commit<->issue link graph
specsync link            # cross-link two or more changes
specsync spinoff         # spawn emergent work as a linked sibling
specsync epic            # create a coordination issue and wire children
specsync release-plan    # shipped changes + advisory semver bump
specsync changelog       # Keep a Changelog section from shipped changes
specsync audit           # archived changes vs. merged PRs
specsync install-skill   # install the bundled agent skill
specsync version         # print the binary version
```

**Dry-run flags** — `sync`, `pull`, `link`, and `spinoff` support `-dry-run`. Beads can be
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
specsync -provider beads -change X     # project one change into the beads graph
```

### Pluggable spec sources

specsync supports multiple spec formats through the `SpecSource` interface.
The default is `openspec` (reads `openspec/changes/`), but the architecture
allows future formats like Beads or custom loaders.

```bash
specsync sync --spec openspec    # default: OpenSpec format
specsync sync --spec beads       # placeholder: Beads (not yet implemented)
```

To add a new spec format, implement the `SpecSource` interface:

```go
type SpecSource interface {
    Name() string
    LoadChanges(specDir string) ([]Change, error)
    SaveChange(change Change) error
}
```

The interface is source-agnostic — `Sync`, `Pull`, `Board`, and other logic
operate on `[]Change` regardless of where the data comes from. Adding a new
format requires only implementing the interface; no changes to core logic.

### Multi-provider sync (fan-out)

`-provider` is repeatable. When you pass multiple providers, specsync projects
each change into every provider independently — a **star topology**: OpenSpec
is the single source of truth, and each provider is a separate edge from that
center. There are no provider-to-provider edges; specsync never routes state
from one tracker to another.

```bash
specsync -provider github -provider beads    # fan-out to both
specsync -provider github -provider beads -dry-run   # preview
```

**Reconciliation** works per-provider: before each push, specsync reads the
current state from that provider's issue and merges any inbound changes into
`tasks.md` (monotonic union — a check from any provider is absorbed). After
pushing to a newly created issue, it reconciles once more to pick up any
pre-existing state (e.g., Beads children already closed before the epic was
created).

**Failure isolation**: if one provider fails, the other providers for the same
change still proceed. The error is reported per-provider in the result without
aborting the entire run.

**Ref coexistence**: each provider's ref is stored under its own key in
`refs.json` (e.g., `"github"` and `"beads"`), so they never collide.

### Provider contract

specsync uses a `WorkProvider` interface so the core engine is provider-agnostic.
Implementations must be **idempotent**: `Push` with an existing ref updates;
without one it creates (and should defend against duplicates via `Find`).

```go
type WorkProvider interface {
    Name() string                         // ref-cache key, e.g. "github"
    Push(ctx, item WorkItem, existing *Ref) (Ref, error)
    Find(ctx, slug string) (*Ref, error)  // locate existing projection by slug
}
```

**Optional capabilities** are detected via type assertion so a minimal provider
need not implement everything:

| Interface | Purpose | Used by |
|-----------|---------|---------|
| `IssueReader` | Read an existing item by ID | `pull`, reconcile, `mcp` |
| `IssueMarkerWriter` | Persist identity marker into item body | `pull` (rediscoverability), `mcp` |
| `TaskStateReader` | Report external task done-state | reconcile |
| `BoardProjector` | Project onto a GitHub Projects board | `pull` |
| `IssueSearcher` | Find open issues by free-text query | `scan` |
| `CommentCapable` | Post comments on items | `mcp` |
| `SubItemCapable` | Create sub-items under a parent | `mcp` |
| `CustomFieldCapable` | Read/write custom fields on items | `mcp` |
| `OpenSpecSource` | Read OpenSpec change metadata/deltas | `release-plan` |

**Available providers** (selectable via `-provider`):

- `github` — default, shells out to `gh` CLI (human-facing issues)
- `beads` — local agent graph via `bd` CLI (agent-facing)
- `mcp` — delegates to an external MCP server (see below)

To add a new provider, implement `WorkProvider` and register it in `makeProvider`
in `cmd/specsync/main.go`.

### The `mcp` provider

`-provider mcp` projects changes through an external
[MCP](https://modelcontextprotocol.io) server instead of `gh` — a Linear, Jira,
GitHub, or in-house work-management server, whatever your project already
talks to. It speaks the current protocol
([2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28),
stateless, per-request `_meta`) and falls back automatically to a legacy
(pre-2026-07-28, `initialize`-handshake) server when the modern probe fails,
per the spec's own backward-compatibility algorithm — you don't have to know
which era a given server speaks.

```bash
specsync -dry-run -provider mcp -change my-feature
specsync -provider mcp -change my-feature
specsync -provider mcp -mcp-config custom-path.json -change my-feature
```

Configure it via a committed `.specsync-mcp.json` at the repo root (default
path; override with `-mcp-config`). It is deliberately **not** under
`.specsync/`, which is entirely gitignored — this file has no secrets in it
and is meant to be shared:

```json
{
  "server": "linear",
  "tools": {
    "createIssue": "create_issue",
    "updateIssue": "update_issue",
    "find": "search_issues",
    "comment": "add_comment",
    "addSubItem": "add_sub_issue",
    "removeSubItem": "remove_sub_issue",
    "setCustomField": "set_field"
  }
}
```

- **`server`** (optional) names an entry in the project's own `.mcp.json` — the
  file Claude Code and other agent harnesses already use to declare MCP
  servers — and reuses its `command`/`args`/`env` (stdio) or `url`/`headers`
  (HTTP) instead of duplicating them. If the project already has a
  work-tracker MCP server configured for agent use, specsync piggybacks on it.
  Without `server`, set `transport` (`"stdio"` or `"http"`) plus
  `command`/`args` or `url` directly. `tokenEnv` names an environment variable
  holding a bearer token (never a literal secret in the committed file); any
  string field pulled from a `.mcp.json` entry also honors `${VAR}`
  expansion, matching that format's own convention.
- **`tools`** maps specsync's operations to the server's actual tool names.
  specsync discovers the server's tools (`tools/list`) and, for any operation
  without an explicit mapping, tries a conservative set of common naming
  variants (`create_issue`, `new_issue`, ...); if nothing matches confidently
  it fails loudly and lists every tool the server actually advertises —
  it never guesses silently.

**How specsync picks GitHub-via-`gh` vs. GitHub-via-MCP vs. GitLab-via-MCP vs.
anything else**: it doesn't — you do, entirely through config. `-provider
github` vs. `-provider mcp` is your choice of transport (the `gh` CLI vs. a
generic MCP client). *Within* `-provider mcp`, which actual tracker you're
talking to is 100% determined by what `.specsync-mcp.json`'s `command`/`args`
(stdio) or `url` (HTTP) point at — specsync has no built-in notion of "this is
GitHub" or "this is GitLab." Every server names its tools, arguments, and
result shapes differently, which is why the rest of this config exists:

- **`context`** — static arguments merged into every call. MCP is stateless
  (no "current repo" implied by the connection), so any repo/project-scoping
  the server's tools require has to come from here, e.g. `{"owner":"...",
  "repo":"..."}` for GitHub's tools.
- **`toolArgs`** — static, per-operation arguments merged in last (highest
  precedence). For servers that fold several specsync operations into one
  tool selected by a fixed parameter, e.g. GitHub's `issue_write` handles both
  create and update via `{"method":"create"}` / `{"method":"update"}`.
- **`findQuery`** — templates the query text sent to the `find` tool
  (`{marker}` = the literal identity comment, `{slug}` = the bare change
  slug). Defaults to the literal marker text; some search tools need the
  tracker's own query syntax instead (GitHub's wants
  `"specsync:change={slug} in:body"`, not a literal string match).
- **`idField`** / **`idFieldNumeric`** — the argument key (and, if needed,
  JSON-number typing) an existing item's identifier is sent under for
  updates/comments/etc. Defaults to a plain string `"id"`; GitHub wants a
  numeric `"issue_number"`.
- **`findIdField`** — the field read *from* a found item as its identifier,
  tried before the "id"/"number" default. Needed when a server's result
  shape carries more than one id-shaped field and the generic default picks
  the wrong one — GitHub's search results carry both an internal database
  `id` and the repo-scoped `number` actually used elsewhere; `id` is wrong.

**Tool contract.** Beyond the above, specsync can't infer argument/result
shapes from an arbitrary server's schema, so the mapped tools must follow
this convention: `createIssue`/`updateIssue` take `{title, body, stage,
priority?, labels?, closed?}` plus your `context`/`toolArgs`/`idField`
additions, and return `structuredContent` (or JSON in the first text content
block, or an object wrapping the item under a common key like `items`) with
at least an id and a url; `find` takes `{query}` and returns a list in the
same tolerant shapes, first match whose body (when returned) contains the
identity marker wins; `comment`/`setCustomField`/`addSubItem`/`removeSubItem`
take the obvious `{id, body}` / `{id, field, value}` / `{parentId, childId}`
shapes.

**Verified against a real server.** `-provider mcp` has been validated
end-to-end against the official [`github-mcp-server`](https://github.com/github/github-mcp-server)
(PAT auth, no OAuth needed for local/stdio use) — create, rediscover via
search, and update, compared directly against `-provider github` on the same
repo. Here's the config that worked:

```json
{
  "transport": "stdio",
  "command": "github-mcp-server",
  "args": ["stdio", "--toolsets", "issues"],
  "env": {"GITHUB_PERSONAL_ACCESS_TOKEN": "${GH_TOKEN}"},
  "context": {"owner": "your-org", "repo": "your-repo"},
  "findQuery": "specsync:change={slug} in:body",
  "findIdField": "number",
  "idField": "issue_number",
  "idFieldNumeric": true,
  "tools": {
    "createIssue": "issue_write",
    "updateIssue": "issue_write",
    "find": "search_issues"
  },
  "toolArgs": {
    "createIssue": {"method": "create"},
    "updateIssue": {"method": "update"}
  }
}
```

That validation pass also found a real, shared bug: `Find`-based duplicate
defense (both this provider and `GitHubProvider` look up an existing item by
marker before creating, since the local ref cache is deliberately never
committed — see [Lifecycle discipline](#lifecycle-discipline)) can race a
tracker's search-index propagation lag moments after a different,
cache-less run (e.g. a prior CI run) just created the same item. A short
bounded retry (~1.2s worst case, nothing on a cache hit or a genuine find)
now backs off before concluding "doesn't exist yet."

**Scope cuts** (revisit if they bite you): no renegotiation against a modern
server that rejects protocol version 2026-07-28 — it errors out naming what
the server supports instead. No support for Multi Round-Trip Requests /
elicitation (`resultType: "input_required"` surfaces as a clear error) — a
non-interactive CLI has no human to elicit input from. No legacy HTTP+SSE
transport (deprecated upstream); legacy fallback covers stdio's `initialize`
handshake and legacy-era plain-JSON-RPC-over-POST HTTP servers.

### Issue-first: pull an issue into a change

Work often starts on the tracker — someone files an issue first. `specsync pull`
reads that issue and scaffolds a local OpenSpec change from it, so you can plan
it as a spec and keep syncing:

```bash
specsync pull -issue 42              # issue 42 -> openspec/changes/<slug>/
specsync pull -issue 42 -dry-run     # read the issue, show what would be written
specsync pull -issue 42 -change my-feature   # override the derived slug
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

### `spinoff` — spawn emergent work as a linked sibling

When a discovery doesn't belong in the current change, spin it off into its own
scoped change instead of scope-creep:

```bash
specsync spinoff -from my-change -task 3 -kind bug       # from task 3
specsync spinoff -from my-change -text "fix X" -dry-run  # free text
```

Extracts text from the parent's task (or uses `-text`), scaffolds a new change
with a seeded `proposal.md`, marks the parent task as moved (`[>] moved: <child>`),
and links the two.

### `epic` — create a coordination issue and wire children

Mints a `type:epic` coordination issue and attaches all children as `## Related`
cross-references. Idempotent — re-running with the same title finds the existing
epic instead of duplicating. Children can be local change slugs (synced first if
needed), issue refs (`#N`, `owner/repo#N`), or full URLs.

```bash
specsync epic "Feature X: cross-repo widgets" \
  --repo androidand/planning \
  --child androidand/backend#12 \
  --child frontend-widget-view
```

Until `epic-and-subissue-projection` lands, children are wired as Related links
in both directions. Once that lands, children are attached as native GitHub
sub-issues and the epic body rolls up from `subIssuesSummary`.

### `idea` — capture an idea as a GitHub issue

Drop any thought, vague or elaborate, into a durable inbox. An idea is just a
GitHub issue labeled `stage:intake` — ordinary enough that teammates and
managers can see it, no spec workflow required to use it.

```bash
specsync idea "Use WebAssembly for the renderer"
echo "Long-form idea text that spans multiple paragraphs..." | specsync idea
specsync idea "Quick thought" --repo my-personal-ideas
```

Title is derived mechanically (first line / first sentence, truncated to 70
chars). Body is the verbatim text plus a capture timestamp. No AI in the capture
path — capture never waits, never fails on model errors.

Configure a default ideas repo for cwd-independent capture:

```yaml
# openspec/specsync.yml
ideas_repo: my-org/ideas
```

Or use the `SPECSYNC_IDEAS_REPO` environment variable.

### `ideas` — list open intake issues

See what you've captured that nobody has triaged yet:

```bash
specsync ideas                    # table output, oldest first
specsync ideas -json              # JSON for scripting
```

### Triage: pull → triage → act

An idea graduates by being pulled into a change (`specsync pull -issue <n>`),
which creates a local spec and flips the issue label from `stage:intake` to
`stage:active` on the next sync. Closing with a reason is a legitimate
disposition — a written "why not" closes the loop on an idea rather than
losing it.

### `trace` — the raw link graph

Prints the resolved trace graph — changes, commits, issues, and the links
between them — for debugging or scripting:

```bash
specsync trace -change my-feature      # scope to one change
specsync trace -since v0.3.0 -json     # commits since a tag, as JSON
```

**Non-goals** (parked for future changes):
- **`graph.json` export** — no consumer exists yet
- **Graphify / inferred edges** — code-symbol edges would pollute an asserted graph
- **Release-plan report** — a future query over this same graph

### `release-plan` — advisory follow-up report

Read-only report over a revision range (default: latest tag → `HEAD`): shipped
changes, loose ends, archive candidates, and an advisory semver bump. It
detects your release tool (e.g. goreleaser) and defers to it — the bump is
advice, not action:

```bash
specsync release-plan                  # since the latest tag
specsync release-plan -since v0.3.0 -json
specsync release-plan -fail-on-archive-candidates
specsync release-plan -archive-completed
```

For release hygiene, run `specsync release-plan -fail-on-archive-candidates`
in CI/release checks. It exits non-zero when shipped changes with fully
completed tasks are still unarchived in `openspec/changes/`.

`release-plan` remains read-only by default. `-archive-completed` is the
explicit mutation flag that moves shipped+complete changes into
`openspec/changes/archive/`.

### `archive` — close, label, and retire a completed change

`specsync archive` runs the full close-out lifecycle for one change: final
sync, close the issue, label it, then apply a retention policy to the local
folder.

```bash
specsync archive -change my-feature                    # move or prune, per policy
specsync archive -change my-feature -retain move        # force keep in git
specsync archive -change my-feature -retain prune       # force delete the local folder
specsync archive -change my-feature -force              # override unchecked-task refusal
specsync archive -change my-feature -dry-run            # print the plan, mutate nothing
```

Steps: **(1)** final push so the issue reflects current scope and task state;
**(2)** close the issue and add the `spec:archived` label; **(3)** apply
retention to `openspec/changes/<slug>/` — `move` relocates it to
`openspec/changes/archive/<slug>/` (kept in git), `prune` deletes it (the
closed issue is the record).

Retention policy resolves in order: `-retain` flag → `retain=` key in
`.specsync/config` → a significance heuristic (a change with a `significant`
marker file, a `design.md`, or more than 5 tasks defaults to `move`;
otherwise `prune`). Archiving refuses when tasks are unchecked unless
`-force` is passed.

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

### `audit` — did the PR actually land?

An archived change means the spec work is done — not that the PR merged. `specsync
audit` cross-references archived changes against GitHub PRs to find the gap:

```bash
specsync audit                              # table of archived changes vs. PR state
specsync audit -json                        # machine-readable, for CI
specsync audit -fail-on-unmerged            # exit non-zero if any unmerged
specsync audit -mark-shipped                # write shipped metadata for merged PRs
```

For each archived change, audit checks: open PR → "unmerged", merged PR →
"shipped", no PR → "orphaned". PR matching uses the specsync marker in the PR
body, branch name convention (`feat/<issue>-<slug>`), or PR title.

Use `-mark-shipped` to write `stage: shipped` to each confirmed change's
`.specsync/metadata.json`. This records the lifecycle step: `active → complete
→ archived → shipped`.

Project a synced change onto a GitHub Projects (v2) board — the issue is added
to the board, its Status follows the change's stage, and the acting user is
assigned:

```bash
specsync -project my-org/6                        # sync + project onto board 6
specsync -project my-org/6 -status-map "active=In Progress,archived=Done"
```

Board resolution order, first match wins:

1. `-project owner/number` (explicit flag — overrides everything).
2. Repository-local declaration: `openspec/specsync.yml` with `board: owner/number`.
3. **No board** — specsync makes zero board calls.

There is deliberately **no global board default** (no env var, no `~/.config`
setting). A shell-wide value that spans every repository is the exact mechanism
by which personal work reaches a work board. If the board's owner differs from
the resolved repository's owner, specsync refuses unless `-project` names it
explicitly.

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
- **Task states** — beyond `[ ]` (todo) and `[x]` (done), tasks.md supports
  `[~]` (dropped — superseded or removed) and `[>]` (moved — relocated to
  another change). Dropped and moved tasks are excluded from progress
  calculations; only todo and done are "live". A compact "Plan changes" footer
  in the issue body shows the breakdown: `+3 added · 2 done · 1 dropped · 1
  moved`.
- **Discoveries** — a `## Discoveries` section captures findings during
  implementation without requiring scope decisions. Use `specsync note -change
  <slug> "<text>"` to append a discovery. The section is rendered into the
  issue and stripped on pull, keeping local and tracker state in sync.
- **Original ask** — when you `specsync pull` an issue, the original body is
  saved as `original-ask.md` (write-once, never overwritten). It renders as
  `## Original ask` in the issue body, providing a stable reference to what was
  requested before scope evolved.
- **Design notes** — a `design.md` renders as `## Design notes` in the issue
  body, between `## Original ask` and `## Discoveries`, so decisions made
  while planning survive even if the local worktree is lost. Pull is
  write-once like `original-ask.md`: local `design.md` can be richer than
  what's synced (mid-edit, not yet pushed), so a pull never clobbers it. When
  design.md is large enough to push the body over GitHub's size limit, sync
  posts it as a linked issue comment instead of inlining it, and keeps that
  comment up to date on every re-sync; if design.md later shrinks back under
  the limit, sync moves it back into the body and marks the old comment
  stale rather than deleting it.
- **Collapsed by default** — a synced issue shows title and task checklist
  first: the Proposal, Original ask, Design notes, and Discoveries sections
  render inside native `<details>` blocks, collapsed in GitHub's UI. Nothing
  is removed — each is one click away — and pull recovers their content via
  an HTML-comment marker inside the block, independent of the visible
  label, so re-labeling a section doesn't break round-tripping. A design
  notes section that has overflowed to a linked comment (above) has no
  marker of its own — its content lives in the comment, not the body.
- **Stage** — derived automatically: `active` while any task is unchecked,
  `complete` once every task is checked (before archiving), and `archived`
  once the change moves under `changes/archive/`. Add `-close-completed` to
  keep the issue's open/closed state aligned too: completion closes it and
  new unchecked work reopens it. Write a richer stage name into
  `<change>/.status` to override the derived value; an explicit `complete`
  closes with the flag, while any other explicit stage remains open.
- **Managed labels are opt-in, off by default** — `specsync` and
  `stage:<stage>` are not added unless you pass `-labels`. Neither is read
  back by specsync itself: issue identity is the body marker, and stage for
  board users is the Projects Status field, so adding them unconditionally
  was pure noise in a repo where every issue is already specsync's.
  `priority:<n>` is unaffected — it's always added when a priority is set.
  Pass `-labels` for a mixed-source repo where native-label filtering
  ("show me only the specsync issues") is actually useful. Turning it off
  after having it on cleans up: the next sync removes `specsync`/`stage:*`
  from issues that already carry them, not just stops adding more.
- **Local cache** — projection ids live in a gitignored `<change>/.specsync/`
  directory, never in git.

## License

MIT — see [LICENSE](LICENSE).
