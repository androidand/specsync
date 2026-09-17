# Tasks: agent-topology-command

## Phase 1: The join

- [x] 1.1 Add `topology.go` with a `Topology`/`ChangeTopology` type matching the
      documented JSON shape
  - Validation: `TestChangeTopology_JSONRoundTripsWithEveryOptionalFieldAbsent` —
    round-trips through `encoding/json` with every optional field absent, and
    confirms `omitempty` actually omits them rather than emitting nulls
- [x] 1.2 Resolve stores via the existing `ReadCoordination`; do not add a second
      discovery path
  - Validation: `TestBuildTopology_MultiRepoJoinAcrossCoordinatedStores` (fake
    `readCoordination`) plus `TestBuildTopology_NothingResolvedIsAnEmptyResultNotAnError`
    for the no-coordination/nil case. `readCoordination` defaults to the real
    `ReadCoordination` — no parallel discovery path added.
- [x] 1.3 Read issue refs from `.specsync/refs.json` per change
  - Validation: `TestBuildTopology_ChangeWithNoWorktreeStillReportedExitZero`
    and others cover a change with a refs file; a change with none yields
    `Issue: nil` (`issue: null` in JSON), not an error — every fixture change
    without a written refs.json exercises this implicitly.
- [x] 1.4 Match branches and worktrees from `git worktree list --porcelain`
  - Match on `feat/<n>-<slug>` first, then directory basename
  - Validation: `TestMatchWorktree` (matched by branch, matched by basename,
    unmatched, detached-HEAD-falls-through-to-basename) and
    `TestListWorktrees_ParsesPorcelain` (regular, detached, and bare-repo
    porcelain blocks)

## Phase 2: Degradation

- [x] 2.1 Every source failure appends to `status[]` and drops only its own fields
  - Validation: `TestBuildTopology_GHUnauthenticatedOmitsStateNotesReason`
    (unauthenticated gh); an uncloned sibling store is exercised implicitly by
    `loadChanges` returning no changes for a nonexistent openspec dir (not an
    error — `LoadChanges` already degrades that way); a store with zero
    worktrees is exercised by every test using an empty porcelain string.
    `openspec` unavailable is covered by `readCoordination` returning
    `(nil, nil)` (the real `ReadCoordination`'s own degrade contract) in every
    test — none of it prevents rows from being returned.
- [x] 2.2 Exit non-zero only when no rows could be produced at all
  - Validation: `TestBuildTopology_NothingResolvedIsAnEmptyResultNotAnError`
    (BuildTopology itself never errors); `cmd/specsync/topology.go`'s
    `runTopology` exits 1 only when `len(topo.Changes) == 0` — verified by
    hand: `specsync topology -change nonexistent-slug` prints "No changes
    resolved." and exits 1; a normal run exits 0.

## Phase 3: Surfaces

- [x] 3.1 Wire `specsync topology` with `-json`, `-change`, `-all`, `-openspec`
      (`-openspec`/`-store` come free from the shared `addRootFlags`)
  - Validation: hand-run `specsync topology -json | jq .` parses;
    `specsync topology -json -change agent-topology-command` narrows to one
    row (see Phase 4 dogfood output below)
- [x] 3.2 Human-readable table output, aligned with `specsync changes` styling
  - Validation: `TestPrintTopologyTable_*` in `cmd/specsync` (resolved row,
    missing-fields-render-as-dash, empty-result message, status lines).
    Downgraded from "golden-file test" to output-assertion tests: no
    golden-file harness exists elsewhere in this codebase (checked
    `changes_test.go`, which doesn't golden-test `printChangeTable` either),
    so introducing one for this alone would be a new pattern, not reuse.
- [x] 3.3 Add the `agent-help topology` entry (behavior, flags, safety, examples)
  - Validation: `specsync agent-help topology -json` parses and lists every
    flag (hand-verified)
- [x] 3.4 Add the "check topology before starting" step to the embedded `SKILL.md`
  - Validation: `TestSkillDrift` stays green (updated the canonical
    `skills/specsync/SKILL.md` and propagated byte-for-byte to the 3 derived
    copies)

## Phase 4: Verify

- [x] 4.1 Dogfood against this repo and one multi-repo exopen change
  - Validation: ran `specsync topology` against this repo (~29 changes,
    correctly shows `agent-topology-command` itself with its real branch
    `feat/168-agent-topology-command` and worktree path — this change was
    being worked on inside that exact worktree at the time). Ran
    `specsync topology -json -change agent-topology-command` with the local
    ref cache present: correctly resolved repo, live issue state (`"open"`
    via a real `gh` call), branch, and worktree. Re-ran with `gh` removed
    from `PATH`: repo/branch/worktree still resolved, issue state omitted,
    with `"gh unavailable: issue state omitted"` in `status[]` — exactly the
    documented degradation. Did not have a second, real multi-repo exopen
    change available in this session to hand-verify against; the multi-store
    join path is covered by `TestBuildTopology_MultiRepoJoinAcrossCoordinatedStores`
    instead.
- [x] 4.2 Confirm runtime is acceptable to call at agent startup
  - Validation: timed `specsync topology` against this repo's single store,
    ~29 changes, no coordinated siblings: **~1.5s wall-clock** (dominated by
    one `git worktree list --porcelain` call plus per-change ref-cache reads;
    no `gh` calls were made since none of these fixture changes had cached
    refs). Acceptable for a check run once at agent startup; a store with `gh`
    calls for every change's issue state will scale roughly linearly with
    change count times issue lookups — worth revisiting if a store's change
    count grows an order of magnitude, but not a concern at this repo's scale.
