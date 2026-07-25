# Make specsync compatible with GitHub Projects (boards)

## Why

specsync projects changes onto GitHub **Issues** only. It has no awareness of
GitHub **Projects (v2)** boards. Everything it does is `gh issue` (create / edit /
list / close) plus labels — verified in `github.go`:

- lifecycle is emitted purely as a label: `github.go:323` sets
  `"stage:" + item.Stage` (stage ∈ `active` / `archived`);
- there is **no assignee handling anywhere** in the codebase;
- there is **no ProjectV2 / `gh project` / GraphQL** usage at all.

For a team whose backlog **is** a GitHub Projects board, this breaks the mental
model. Reproduced with org Project #1 ("the backlog board"): specsync
created issue `org/widget-app#16`, and on the board it showed
`onBoard: false`, `status: null`, `assignees: []`. It carried only the
`stage:active` **label**. So the issue never appeared as active work on the board
— because on a Projects board "active" means *on the board* + a **Status** field
value + an **assignee**, none of which specsync sets. The `stage:active` label and
the board's `Status` field are two disconnected lifecycle models that never meet.

This is not a crash bug; it is a **compatibility gap**. specsync models lifecycle
as issue labels; GitHub Projects models it as board membership + a single-select
Status field + assignees. specsync should be able to project onto the board too,
and — crucially — **detect** whether an issue already belongs to the target board
so it neither misses it nor clobbers human curation.

The create-vs-existing distinction is the hard part:
- When specsync **creates** the issue, it owns the whole lifecycle and can eagerly
  add it to the board, set Status, and assign — cheaply and unambiguously.
- When specsync **syncs an existing** issue (including issues born on the board or
  pulled with `pull`), it must first *query* the board to learn whether the issue
  is already an item, then reconcile without overwriting a human-moved card or a
  human-set assignee.

## What Changes

- **Opt-in target project.** A new setting names the board to project onto — an
  org or user ProjectV2 by number, e.g. `-project org/6` (flag on `sync`
  and `pull`, mirroring `-repo`; also readable from a persisted config so it need
  not be retyped). **When unset, behavior is unchanged** — no board calls, exactly
  as today. This keeps the feature backward-compatible and off by default.
- **Board-membership detection.** specsync resolves the issue's node id and queries
  the target ProjectV2's items to determine whether the issue is already on the
  board. This "is it actually on the board / should it be?" check is the primitive
  everything else builds on, and makes all board operations idempotent.
- **Ensure-on-board on create and sync.** With a target project configured, a
  synced change's issue is added to the board if absent (never duplicated).
- **Stage → Status projection (configurable, schema-resolved).** specsync maps its
  stage to the board's single-select **Status** option and sets it via
  `updateProjectV2ItemFieldValue`. Because Status option names differ per board
  (this board uses `Ready for development` / `In progress` / `Done`), the mapping
  is configurable and the field + option ids are resolved from the project schema
  at runtime rather than hard-coded. A sensible default maps `active → In progress`
  (first non-terminal option) and `archived → Done` (terminal option).
- **Assignee projection.** specsync assigns the issue to the acting user (resolved
  via `gh api user`, reusing the identity approach specsync already trusts) or a
  configured assignee. Existing assignees are **not** overwritten.
- **Reconciliation without clobber (the existing-issue case).** On sync of an issue
  already on the board, specsync only sets Status/assignee when they are unset, or
  when the current value is one specsync itself last wrote; a human who moved the
  card or changed the assignee wins. specsync never *removes* a board item.
- **Dry-run parity.** `-dry-run` prints the board mutations it would make (add item,
  set Status option, set assignee) and performs no GraphQL writes, honoring the
  existing zero-API-call dry-run contract.

### Out of scope / explicitly deferred
- Making Projects the source of truth for stage — OpenSpec/`.status` stays
  authoritative; the board is a projection, like issues.
- Inbound reconciliation of board Status back into `tasks.md`/stage (a future
  counterpart to task-checkbox reconcile).
- Custom board fields beyond Status and assignees (priority, dates, iteration).
- Providers other than GitHub — Projects is GitHub-specific; Beads is unaffected.
- Auto-creating a board or Status options — specsync targets an existing board.

## Prior art: the org `backlog` MCP

The `backlog` MCP server (`internal-tools/backlog-mcp/src`) already drives this exact board
(org ProjectV2, node id `PVT_xxxxxxxxxxxxxxxxxxxxx`) and is the reference for the
mechanisms specsync should adopt:

- **Resolve number → node id, then address by node id.** The board is always
  `node(id: <projectId>) { ... on ProjectV2 { ... } }`. backlog bakes the id into
  `constants.ts`; specsync should instead resolve `owner/number` → node id once per
  run and cache it, since it spans repos/boards (backlog's own caveat: hardcoded ids
  + literal status strings don't generalize to multiple boards).
- **Schema discovery → name↔id maps.** One query for
  `fields { ... on ProjectV2SingleSelectField { id name options { id name } } }`,
  matched by field **name** (`"Status"`), built into bidirectional `byName`/`byId`
  maps and cached. This decouples human status labels from the volatile
  `PVTSSF_…`/option ids. Map a status name → option id with **fail-loud**
  validation (throw listing valid options), never a silent no-op.
- **Membership by node id.** backlog bulk-fetches the board and indexes by
  `repo#number`; specsync syncs one change at a time, so it can query the issue
  directly — `issue { projectItems(first:20) { nodes { id project { id } } } }` —
  and check for the target `projectId`. Absent → add; present → reconcile. This is
  the "is it actually on the board / should it be" primitive, and it makes both the
  create case (empty `projectItems`) and the existing case uniform.
- **Two node ids, two mutation families.** Field/board mutations use the **project
  item id** (`updateProjectV2ItemFieldValue` / `clearProjectV2ItemFieldValue`,
  `singleSelectOptionId`); add-to-board uses the **issue content id**
  (`addProjectV2ItemById(projectId, contentId)`); **assignees are issue-level**
  (`addAssigneesToAssignable(assignableId, assigneeIds)`), not a project field.
  These must not be conflated.
- **Identity.** Resolve the assignee login → user node id via `user(login){id}`;
  `"me"` = `viewer{login}`, preferring local `git config user.email` — the same
  identity anchor specsync already trusts.
- **Token scope.** ProjectV2 needs the **`project`** scope (plus `repo`,
  `read:org`) — a direct echo of the publish-scope problem in
  `stable-projection-ref-key`/#2: board writes will 403 without it, so specsync must
  fail with a clear scope message rather than a raw GraphQL error.

specsync stays `gh`-based rather than adding an Octokit dependency: these operations
run through `gh api graphql` (backlog uses Octokit directly; the operations are
identical).

## Capabilities

### New Capabilities
- `github-projects-projection` — with an opt-in target ProjectV2, specsync detects
  board membership, ensures the synced issue is on the board, maps its stage to the
  board Status field, and assigns it, idempotently and without clobbering human
  curation; unset target = no board behavior.

## Impact

- `github.go`: add ProjectV2 support via `gh api graphql` (query project id + Status
  field/option ids + membership by issue node id; `addProjectV2ItemById`,
  `updateProjectV2ItemFieldValue`) and assignee setting (`gh issue edit
  --add-assignee` or GraphQL). Expressed as a new type-asserted provider capability
  (e.g. `BoardProjector`) so the core `WorkProvider` contract stays minimal, like
  `IssueReader`/`IssueSearcher`.
- `sync.go` (`syncOne`) / `pull.go`: after the issue upsert, when a target project is
  configured, run detect → ensure-on-board → project Status/assignee through the new
  capability; skip entirely when unconfigured.
- `provider.go`: define the `BoardProjector` capability interface + the Stage→Status
  mapping type.
- `cmd/specsync/main.go`: `-project owner/number` flag on `sync`/`pull`, config
  fallback, and dry-run rendering of the board plan.
- Depends on the stabilized ref keying from `stable-projection-ref-key` (issue node
  id resolution reuses the repo-scoped provider). Stdlib-only; shells out to `gh`
  (now including `gh api graphql`).
